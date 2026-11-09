package main

import (
	"bpimon/internal/alert"
	"bpimon/internal/bot"
	"bpimon/internal/buildinfo"
	"bpimon/internal/config"
	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
	"bpimon/internal/update"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	githubRepo          = "Kovalenkoyo81/bpimon"
	updateCheckInterval = 24 * time.Hour
)

// Set via -ldflags at build time.
var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	configPath := flag.String("config", "config.yaml", "path to config file")
	statePath := flag.String("state", "", "path to alert state file (optional)")
	flag.Parse()
	if *showVersion {
		fmt.Printf("bpimon %s (built %s, %s/%s)\n", version, buildDate, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	log.Init()
	cfg := config.Load(*configPath)

	log.Info.Println("Starting Banana Bot...")

	var ntf notifier.AlertSender
	var botInstance *bot.Bot

	if cfg.Telegram.Enabled {
		botInstance = connectTelegram(cfg.Telegram.Token, cfg.Telegram.ChatID)
		ntf = botInstance
	} else {
		ntf = notifier.Dummy{}
	}

	providers := monitor.NewProviders(cfg.Devices.Smart, cfg.Devices.MMC, cfg.Devices.Docker)

	updates := update.NewGitHub(githubRepo)
	handler := bot.Handler{
		Providers: providers,
		Admins:    cfg.Telegram.Admins,
		Version:   &bot.VersionReporter{Build: buildinfo.New(version, buildDate), Updates: updates},
	}
	handler.InitPermissions()

	alerts := alert.FromProviders(providers, cfg.Thresholds)

	manager := alert.NewManager(
		alerts,
		ntf,
		time.Duration(cfg.Thresholds.IntervalMin)*time.Minute,
		time.Duration(cfg.Thresholds.CooldownMin)*time.Minute,
	)
	handler.Silencer = manager

	if *statePath != "" {
		manager.SetStatePath(*statePath)
	}

	callbacks := bot.CallbackHandler{Resolver: manager, IsAdmin: handler.IsAdmin}

	if botInstance != nil {
		watcher := &update.Watcher{Source: updates, Current: version, Notifier: botInstance, Interval: updateCheckInterval}
		watcher.MarkAnnounced(handler.SendStartupMessage(botInstance))
		go watcher.Run(context.Background())
		handler.SendInitialStatus(botInstance)
		handler.SendCommandsMenu(botInstance)

		go func() {
			for {
				for update := range botInstance.UpdatesChan() {
					if cq := update.CallbackQuery; cq != nil {
						handleCallback(botInstance, &callbacks, cq, cfg.Telegram.ChatID)
						continue
					}
					if update.Message == nil {
						continue
					}
					if update.Message.Chat.ID != cfg.Telegram.ChatID {
						continue
					}
					response := handler.Handle(update.Message.Text, update.Message.From.ID, botInstance)
					if response != "" {
						log.Info.Printf("response len=%d val=%q", len(response), response)
						_ = botInstance.Send(response)
					}
				}
				log.Warn.Println("Telegram updates channel closed — reconnecting")
				botInstance = connectTelegram(cfg.Telegram.Token, cfg.Telegram.ChatID)
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	managerDone := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(managerDone)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info.Println("Shutting down...")
	cancel()
	// Let the manager save alert state before the process exits.
	select {
	case <-managerDone:
	case <-time.After(5 * time.Second):
		log.Warn.Println("alert manager did not stop in 5s")
	}
}

// handleCallback processes an inline-button press from the configured chat.
func handleCallback(b *bot.Bot, h *bot.CallbackHandler, cq *tgbotapi.CallbackQuery, chatID int64) {
	log.Info.Printf("callback received: data=%q from=%d", cq.Data, cq.From.ID)
	if cq.Message == nil || cq.Message.Chat.ID != chatID {
		log.Warn.Printf("callback ignored: not from the configured chat")
		if err := b.AnswerCallback(cq.ID, ""); err != nil {
			log.Warn.Printf("answer callback: %v", err)
		}
		return
	}
	who := cq.From.FirstName
	if cq.From.UserName != "" {
		who = "@" + cq.From.UserName
	}
	res := h.Handle(cq.Data, cq.From.ID, who)
	log.Info.Printf("callback %q from %d: %s", cq.Data, cq.From.ID, res.Toast)
	if err := b.AnswerCallback(cq.ID, res.Toast); err != nil {
		log.Warn.Printf("answer callback: %v", err)
	}
	if res.Note != "" {
		if err := b.EditText(cq.Message.MessageID, cq.Message.Text+"\n\n"+res.Note); err != nil {
			log.Warn.Printf("edit message: %v", err)
		}
	}
}

// connectTelegram retries bot.New with linear backoff until the connection succeeds.
// This handles the case where DNS is not yet available on system startup.
func connectTelegram(token string, chatID int64) *bot.Bot {
	for attempt := 1; ; attempt++ {
		b, err := bot.New(token, chatID)
		if err == nil {
			return b
		}
		delay := time.Duration(attempt) * 5 * time.Second
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		msg := "connection failed"
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			msg = urlErr.Err.Error()
		}
		log.Warn.Printf("Telegram unavailable (attempt %d): %s — retry in %s", attempt, msg, delay)
		time.Sleep(delay)
	}
}

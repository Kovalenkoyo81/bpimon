package main

import (
	"bpimon/internal/alert"
	"bpimon/internal/bot"
	"bpimon/internal/config"
	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
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

	handler := bot.Handler{
		Providers: providers,
		Admins:    cfg.Telegram.Admins,
	}
	handler.InitPermissions()

	if botInstance != nil {
		handler.SendRebootAlert(botInstance)
		handler.SendInitialStatus(botInstance)
		handler.SendCommandsMenu(botInstance)

		go func() {
			for {
				for update := range botInstance.UpdatesChan() {
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

	ctx, cancel := context.WithCancel(context.Background())
	go manager.Run(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info.Println("Shutting down...")
	cancel()
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

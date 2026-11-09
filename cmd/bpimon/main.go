package main

import (
	"bpimon/internal/alert"
	"bpimon/internal/bot"
	"bpimon/internal/buildinfo"
	"bpimon/internal/config"
	"bpimon/internal/log"
	"bpimon/internal/monitor"
	"bpimon/internal/notifier"
	"bpimon/internal/system"
	"bpimon/internal/update"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var ntf notifier.AlertSender = notifier.Dummy{}
	var tg *bot.Bot
	if cfg.Telegram.Enabled {
		var err error
		if tg, err = bot.Connect(ctx, cfg.Telegram.Token, cfg.Telegram.ChatID); err != nil {
			log.Info.Println("Shutting down before Telegram connected")
			return
		}
		ntf = tg
	}

	providers := monitor.NewProviders(cfg.Devices.Smart, cfg.Devices.MMC, cfg.Devices.Docker)

	manager := alert.NewManager(
		alert.FromProviders(providers, cfg.Thresholds),
		ntf,
		time.Duration(cfg.Thresholds.IntervalMin)*time.Minute,
		time.Duration(cfg.Thresholds.CooldownMin)*time.Minute,
	)
	if *statePath != "" {
		manager.SetStatePath(*statePath)
	}

	updates := update.NewGitHub(githubRepo)
	handler := bot.NewHandler(bot.HandlerConfig{
		Providers: providers,
		Admins:    cfg.Telegram.Admins,
		Silencer:  manager,
		Version:   &bot.VersionReporter{Build: buildinfo.New(version, buildDate), Updates: updates},
		System:    system.Actions{},
	})

	if tg != nil {
		watcher := &update.Watcher{Source: updates, Current: version, Notifier: tg, Interval: updateCheckInterval}
		watcher.MarkAnnounced(handler.SendStartupMessage(tg))
		handler.SendInitialStatus(tg)
		handler.SendCommandsMenu(tg)

		callbacks := &bot.CallbackHandler{Resolver: manager, IsAdmin: handler.IsAdmin}
		dispatcher := bot.NewDispatcher(tg, handler, callbacks, cfg.Telegram.ChatID)
		go watcher.Run(ctx)
		go dispatcher.Run(ctx, tg.UpdatesChan())
		defer tg.StopUpdates()
	}

	managerDone := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(managerDone)
	}()

	<-ctx.Done()
	log.Info.Println("Shutting down...")
	// Let the manager save alert state before the process exits.
	select {
	case <-managerDone:
	case <-time.After(5 * time.Second):
		log.Warn.Println("alert manager did not stop in 5s")
	}
}

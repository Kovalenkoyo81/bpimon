package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Kovalenkoyo81/bpimon/internal/alert"
	"github.com/Kovalenkoyo81/bpimon/internal/bot"
	"github.com/Kovalenkoyo81/bpimon/internal/buildinfo"
	"github.com/Kovalenkoyo81/bpimon/internal/config"
	"github.com/Kovalenkoyo81/bpimon/internal/monitor"
	"github.com/Kovalenkoyo81/bpimon/internal/notifier"
	"github.com/Kovalenkoyo81/bpimon/internal/system"
	"github.com/Kovalenkoyo81/bpimon/internal/update"
)

const (
	githubRepo          = "Kovalenkoyo81/bpimon"
	updateCheckInterval = 24 * time.Hour
)

// logLevel is Info, or Debug when BPIMON_DEBUG is set.
func logLevel() slog.Level {
	if os.Getenv("BPIMON_DEBUG") != "" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

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

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()})))
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	slog.Info("starting bpimon", "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var ntf notifier.AlertSender = notifier.Dummy{}
	var tg *bot.Bot
	if cfg.Telegram.Enabled {
		if tg, err = bot.Connect(ctx, cfg.Telegram.Token, cfg.Telegram.ChatID); err != nil {
			slog.Info("shutting down before Telegram connected")
			return
		}
		ntf = tg
	}

	providers, err := monitor.NewProviders(cfg.Devices.Smart, cfg.Devices.MMC, cfg.Devices.Docker)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

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
	slog.Info("shutting down")
	// Let the manager save alert state before the process exits.
	select {
	case <-managerDone:
	case <-time.After(5 * time.Second):
		slog.Warn("alert manager did not stop in 5s")
	}
}

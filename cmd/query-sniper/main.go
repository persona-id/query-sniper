package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/persona-id/query-sniper/internal/configuration"
	"github.com/persona-id/query-sniper/internal/sniper"
)

func main() {
	settings, err := configuration.Configure()
	if err != nil {
		slog.Error("error configuring the application, cannot continue", slog.Any("err", err))

		os.Exit(1)
	}

	// setup the slog logger and set it to the default logger for the application.
	configuration.SetupLogger(settings)

	// log the build info; mostly useful for debugging.
	configuration.LogBuildInfo()

	// setup the context and signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP)

	go handleSignals(ctx, cancel, sigChan)

	sniper.Run(ctx, settings)
}

// handleSignals processes OS signals in a separate goroutine.
// It cancels the context on shutdown signals (SIGINT, SIGTERM) and logs other signals.
func handleSignals(_ context.Context, cancel context.CancelFunc, sigChan <-chan os.Signal) {
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("received shutdown signal, initiating graceful shutdown", slog.String("signal", sig.String()))
			cancel()

			return

		case syscall.SIGUSR1, syscall.SIGUSR2:
			slog.Info("received SIGUSR signal", slog.String("signal", sig.String()))
			// TODO: implement SIGUSR handler functionality

		case syscall.SIGHUP:
			slog.Info("received SIGHUP signal", slog.String("signal", sig.String()))
			// TODO: implement SIGHUP handler functionality (e.g., reload config)

		default:
			slog.Warn("received unhandled signal", slog.String("signal", sig.String()))
		}
	}
}

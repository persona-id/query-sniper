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
		slog.Error("Error configuring the application, cannot continue", slog.Any("err", err))

		os.Exit(1)
	}

	// setup the slog logger and set it to the default logger for the application.
	configuration.SetupLogger(settings)
	configuration.LogBuildInfo()

	// setup the context and signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP)

	go handleSignals(cancel, sigChan)

	sniper.Run(ctx, settings)
}

// handleSignals processes OS signals in a separate goroutine.
// It cancels the context on shutdown signals (SIGINT, SIGTERM) and logs
// other signals (SIGUSR1, SIGUSR2, SIGHUP).
func handleSignals(cancel context.CancelFunc, sigChan <-chan os.Signal) {
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			slog.Info("Received shutdown signal, initiating graceful shutdown", slog.String("signal", sig.String()))
			cancel()

			return

		case syscall.SIGUSR1, syscall.SIGUSR2:
			slog.Info("Received SIGUSR signal", slog.String("signal", sig.String()))
			// TODO: implement SIGUSR{1,2} handler functionality.

		case syscall.SIGHUP:
			slog.Info("Received SIGHUP signal", slog.String("signal", sig.String()))
			// TODO: implement SIGHUP handler functionality (e.g., reload config).

		default:
			slog.Warn("Received unhandled signal", slog.String("signal", sig.String()))
		}
	}
}

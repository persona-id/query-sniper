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
		slog.Error("Error in Configure()", slog.Any("err", err))
		os.Exit(1)
	}

	setupLogger(settings.LogLevel)

	ctx, cancelFunc := context.WithCancel(context.Background())

	go sniper.Run(ctx, settings)

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan

	slog.Info("QuerySniper received TERM signal, shutting down")

	cancelFunc()

	slog.Info("QuerySniper shut down")
}

func setupLogger(level string) {
	var logLevel slog.Level

	switch level {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "INFO":
		logLevel = slog.LevelInfo
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, opts)

	logger := slog.New(handler)

	slog.SetDefault(logger)
}

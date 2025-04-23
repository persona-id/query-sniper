package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/persona-id/query-sniper/internal/configuration"
	"github.com/persona-id/query-sniper/internal/sniper"
)

func main() {
	settings, err := configuration.Configure()
	if err != nil {
		slog.Error("Error in Configure()", slog.Any("err", err))
		os.Exit(1)
	}

	setupLogger(settings)

	ctx, cancelFunc := context.WithCancel(context.Background())

	go sniper.Run(ctx, settings)

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)
	<-termChan

	slog.Info("QuerySniper received TERM signal, shutting down")

	cancelFunc()

	slog.Info("QuerySniper shut down")
}

func setupLogger(settings *configuration.Config) {
	var logLevel slog.Level

	switch settings.LogLevel {
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

	handler := tint.NewHandler(os.Stdout, &tint.Options{
		AddSource:   false,
		Level:       logLevel,
		TimeFormat:  time.RFC3339,
		NoColor:     false,
		ReplaceAttr: nil,
	})

	if settings.LogFormat == "JSON" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource:   false,
			Level:       logLevel,
			ReplaceAttr: nil,
		})
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)
}

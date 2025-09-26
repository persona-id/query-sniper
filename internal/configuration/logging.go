package configuration

import (
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

// SetupLogger sets up the slog logger as the default logger.
// Uses settings.log.* to configure aspects of the logger handler.
func SetupLogger(settings *Config) {
	levelMap := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"ERROR": slog.LevelError,
		"FATAL": LevelFatal,
		"INFO":  slog.LevelInfo,
		"TRACE": LevelTrace,
		"WARN":  slog.LevelWarn,
	}

	level, exists := levelMap[strings.ToUpper(settings.Log.Level)]
	if !exists {
		level = slog.LevelInfo // default fallback level
	}

	var handler slog.Handler

	LevelNames := map[slog.Leveler]string{
		LevelTrace: "TRACE",
		LevelFatal: "FATAL",
	}

	if strings.ToUpper(settings.Log.Format) == "JSON" { //nolint:nestif
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: settings.Log.IncludeCaller,
			Level:     level,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.LevelKey {
					if logLevel, ok := a.Value.Any().(slog.Level); ok {
						levelLabel, exists := LevelNames[logLevel]
						if !exists {
							levelLabel = logLevel.String()
						}

						a.Value = slog.StringValue(levelLabel)
					}
				}

				return a
			},
		})
	} else {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			AddSource: settings.Log.IncludeCaller,
			Level:     level,
			NoColor:   false,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.LevelKey {
					if len(groups) == 0 {
						level, ok := a.Value.Any().(slog.Level)
						if ok {
							// format the trace and fatal levels to be more readable.
							switch level { //nolint:exhaustive
							case LevelTrace:
								return tint.Attr(13, slog.String(a.Key, "TRC")) //nolint:mnd

							case LevelFatal:
								return tint.Attr(1, slog.String(a.Key, "FAL"))

							default:
								// don't touch the other levels
								return a
							}
						}
					}

					if logLevel, ok := a.Value.Any().(slog.Level); ok {
						levelLabel, exists := LevelNames[logLevel]
						if !exists {
							levelLabel = logLevel.String()
						}

						a.Value = slog.StringValue(levelLabel)
					}
				}

				return a
			},

			TimeFormat: time.RFC3339,
		})
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)
}

// LogBuildInfo logs debug information about the service, namely configuration values
// and build info.
func LogBuildInfo() {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		// Something happened, but we don't know what, so just log the error and return.
		slog.Error("failed to read build info")

		return
	}

	// Parse build info into key-value pairs for structured logging.
	buildArgs := []any{}

	// Add Go version, path, and module info
	buildArgs = append(buildArgs, "go", buildInfo.GoVersion)
	buildArgs = append(buildArgs, "path", buildInfo.Path)
	buildArgs = append(buildArgs, "mod", buildInfo.Main.Path+" "+buildInfo.Main.Version)
	buildArgs = append(buildArgs, "sum", buildInfo.Main.Sum)

	// Add build settings (but we're skipping deps because it's a lot of noise)
	for _, biSettings := range buildInfo.Settings {
		if strings.HasPrefix(biSettings.Key, "build") ||
			strings.HasPrefix(biSettings.Key, "CGO_") ||
			strings.HasPrefix(biSettings.Key, "GO") {
			if biSettings.Value != "" {
				buildArgs = append(buildArgs, biSettings.Key, biSettings.Value)
			}
		}
	}

	slog.Info("build info", buildArgs...)
}

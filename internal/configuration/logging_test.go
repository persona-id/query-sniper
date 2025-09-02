package configuration

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lmittmann/tint"
)

//nolint:gocognit,maintidx,paralleltest,cyclop,gocyclo
func TestSetupLogger(t *testing.T) {
	originalLogger := slog.Default()

	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	tests := []struct {
		config         *Config
		name           string
		testLogMessage string
		expectLevel    slog.Level
		expectJSON     bool
		expectCaller   bool
	}{
		{
			name: "JSON format with DEBUG level and caller",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "DEBUG",
					IncludeCaller: true,
				},
			},
			expectLevel:    slog.LevelDebug,
			expectJSON:     true,
			expectCaller:   true,
			testLogMessage: "test debug message",
		},
		{
			name: "JSON format with INFO level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "json", // lowercase to test case insensitivity
					Level:         "INFO",
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelInfo,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test info message",
		},
		{
			name: "JSON format with WARN level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "WARN",
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelWarn,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test warn message",
		},
		{
			name: "JSON format with ERROR level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "ERROR",
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelError,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test error message",
		},
		{
			name: "TEXT format with INFO level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "TEXT",
					Level:         "INFO",
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelInfo,
			expectJSON:     false,
			expectCaller:   false,
			testLogMessage: "test text message",
		},
		{
			name: "TEXT format with caller enabled",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "CONSOLE", // non-JSON format
					Level:         "INFO",
					IncludeCaller: true,
				},
			},
			expectLevel:    slog.LevelInfo,
			expectJSON:     false,
			expectCaller:   true,
			testLogMessage: "test console message with caller",
		},
		{
			name: "invalid level defaults to INFO",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "INVALID",
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelInfo,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test invalid level fallback",
		},
		{
			name: "case insensitive level matching",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "debug", // lowercase
					IncludeCaller: false,
				},
			},
			expectLevel:    slog.LevelDebug,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test lowercase level",
		},
		{
			name: "JSON format with TRACE level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "TRACE",
					IncludeCaller: false,
				},
			},
			expectLevel:    LevelTrace,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test trace message",
		},
		{
			name: "TEXT format with TRACE level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "TEXT",
					Level:         "TRACE",
					IncludeCaller: true,
				},
			},
			expectLevel:    LevelTrace,
			expectJSON:     false,
			expectCaller:   true,
			testLogMessage: "test trace text message",
		},
		{
			name: "JSON format with FATAL level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "JSON",
					Level:         "FATAL",
					IncludeCaller: false,
				},
			},
			expectLevel:    LevelFatal,
			expectJSON:     true,
			expectCaller:   false,
			testLogMessage: "test fatal message",
		},
		{
			name: "TEXT format with FATAL level",
			config: &Config{
				Log: struct {
					Format        string `mapstructure:"format"`
					Level         string `mapstructure:"level"`
					IncludeCaller bool   `mapstructure:"include_caller"`
				}{
					Format:        "CONSOLE",
					Level:         "FATAL",
					IncludeCaller: true,
				},
			},
			expectLevel:    LevelFatal,
			expectJSON:     false,
			expectCaller:   true,
			testLogMessage: "test fatal text message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			SetupLogger(tt.config)

			var testHandler slog.Handler

			levelNames := map[slog.Leveler]string{
				LevelTrace: "TRACE",
				LevelFatal: "FATAL",
			}

			if tt.expectJSON { //nolint:nestif
				testHandler = slog.NewJSONHandler(&buf, &slog.HandlerOptions{
					AddSource: tt.expectCaller,
					Level:     tt.expectLevel,
					ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
						if a.Key == slog.LevelKey {
							if logLevel, ok := a.Value.Any().(slog.Level); ok {
								if levelLabel, exists := levelNames[logLevel]; exists {
									a.Value = slog.StringValue(levelLabel)
								}
							}
						}

						return a
					},
				})
			} else {
				testHandler = tint.NewHandler(&buf, &tint.Options{
					AddSource:  tt.expectCaller,
					Level:      tt.expectLevel,
					NoColor:    true,
					TimeFormat: time.RFC3339,
					ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
						if a.Key == slog.LevelKey {
							if logLevel, ok := a.Value.Any().(slog.Level); ok {
								if levelLabel, exists := levelNames[logLevel]; exists {
									a.Value = slog.StringValue(levelLabel)
								}
							}
						}

						return a
					},
				})
			}

			testLogger := slog.New(testHandler)
			slog.SetDefault(testLogger)

			testLogger.Log(context.TODO(), LevelTrace, tt.testLogMessage+" trace")
			slog.Debug(tt.testLogMessage + " debug")
			slog.Info(tt.testLogMessage + " info")
			slog.Warn(tt.testLogMessage + " warn")
			slog.Error(tt.testLogMessage + " error")
			testLogger.Log(context.TODO(), LevelFatal, tt.testLogMessage+" fatal")

			logOutput := buf.String()

			if logOutput == "" && tt.expectLevel <= LevelFatal {
				t.Error("SetupLogger() resulted in no log output")

				return
			}

			switch tt.expectLevel {
			case LevelTrace:
				if !strings.Contains(logOutput, "trace") {
					t.Error("Trace level should be logged when level is TRACE")
				}

				fallthrough

			case slog.LevelDebug:
				if tt.expectLevel <= slog.LevelDebug && !strings.Contains(logOutput, "debug") {
					t.Error("Debug level should be logged when level is DEBUG or lower")
				}

				fallthrough

			case slog.LevelInfo:
				if tt.expectLevel <= slog.LevelInfo && !strings.Contains(logOutput, "info") {
					t.Error("Info level should be logged when level is INFO or lower")
				}

				fallthrough

			case slog.LevelWarn:
				if tt.expectLevel <= slog.LevelWarn && !strings.Contains(logOutput, "warn") {
					t.Error("Warn level should be logged when level is WARN or lower")
				}

				fallthrough

			case slog.LevelError:
				if tt.expectLevel <= slog.LevelError && !strings.Contains(logOutput, "error") {
					t.Error("Error level should be logged when level is ERROR or lower")
				}

				fallthrough

			case LevelFatal:
				if tt.expectLevel <= LevelFatal && !strings.Contains(logOutput, "fatal") {
					t.Error("Fatal level should be logged when level is FATAL or lower")
				}
			}

			if tt.expectJSON {
				if !strings.Contains(logOutput, `"msg"`) {
					t.Error("JSON format should contain msg field")
				}

				if !strings.Contains(logOutput, `"level"`) {
					t.Error("JSON format should contain level field")
				}
			} else if strings.Contains(logOutput, `"msg":`) {
				t.Error("TEXT format should not contain JSON-style msg field")
			}

			if tt.expectCaller && tt.expectJSON {
				if !strings.Contains(logOutput, `"source"`) {
					t.Error("Caller information should be included when IncludeCaller is true")
				}
			}

			if tt.expectLevel > slog.LevelDebug && strings.Contains(logOutput, "debug") {
				t.Errorf("Debug messages should be filtered out when level is %v", tt.expectLevel)
			}

			// Verify custom level names are displayed correctly
			if tt.expectLevel == LevelTrace && strings.Contains(logOutput, "trace") {
				if tt.expectJSON {
					if !strings.Contains(logOutput, `"level":"TRACE"`) {
						t.Error("JSON output should contain TRACE level name")
					}
				} else if !strings.Contains(logOutput, "TRACE") {
					t.Error("TEXT output should contain TRACE level name")
				}
			}

			if tt.expectLevel == LevelFatal && strings.Contains(logOutput, "fatal") {
				if tt.expectJSON {
					if !strings.Contains(logOutput, `"level":"FATAL"`) {
						t.Error("JSON output should contain FATAL level name")
					}
				} else if !strings.Contains(logOutput, "FATAL") {
					t.Error("TEXT output should contain FATAL level name")
				}
			}
		})
	}
}

func TestLogBuildInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	originalLogger := slog.Default()

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	LogBuildInfo()

	logOutput := buf.String()

	if logOutput == "" {
		t.Error("LogBuildInfo() produced no log output")
	}

	if !strings.Contains(logOutput, `"msg":"build info"`) {
		t.Error("LogBuildInfo() did not log expected 'build info' message")
	}

	expectedFields := []string{
		`"go"`,   // Go version
		`"path"`, // Module path
		`"mod"`,  // Module info
	}

	for _, field := range expectedFields {
		if !strings.Contains(logOutput, field) {
			t.Errorf("LogBuildInfo() output missing expected field %s", field)
		}
	}

	if !strings.Contains(logOutput, `"level":"INFO"`) {
		t.Error("LogBuildInfo() did not log at INFO level")
	}
}

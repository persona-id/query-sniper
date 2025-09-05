//nolint:paralleltest,gocognit
package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

func TestHandleSignals(t *testing.T) {
	tests := []struct {
		signal            os.Signal
		name              string
		expectLogContains string
		expectContextDone bool
		expectReturn      bool
	}{
		{
			name:              "SIGINT cancels context and returns",
			signal:            syscall.SIGINT,
			expectContextDone: true,
			expectLogContains: "Received shutdown signal, initiating graceful shutdown",
			expectReturn:      true,
		},
		{
			name:              "SIGTERM cancels context and returns",
			signal:            syscall.SIGTERM,
			expectContextDone: true,
			expectLogContains: "Received shutdown signal, initiating graceful shutdown",
			expectReturn:      true,
		},
		{
			name:              "SIGUSR1 logs but does not cancel context",
			signal:            syscall.SIGUSR1,
			expectContextDone: false,
			expectLogContains: "Received SIGUSR signal",
			expectReturn:      false,
		},
		{
			name:              "SIGUSR2 logs but does not cancel context",
			signal:            syscall.SIGUSR2,
			expectContextDone: false,
			expectLogContains: "Received SIGUSR signal",
			expectReturn:      false,
		},
		{
			name:              "SIGHUP logs but does not cancel context",
			signal:            syscall.SIGHUP,
			expectContextDone: false,
			expectLogContains: "Received SIGHUP signal",
			expectReturn:      false,
		},
		{
			name:              "unhandled signal logs warning",
			signal:            syscall.SIGPIPE,
			expectContextDone: false,
			expectLogContains: "Received unhandled signal",
			expectReturn:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				t.Helper()

				var buf safeBuffer

				logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				}))
				slog.SetDefault(logger)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				sigChan := make(chan os.Signal, 1)

				done := make(chan bool, 1)

				go func() {
					handleSignals(cancel, sigChan)

					done <- true
				}()

				sigChan <- tt.signal

				if tt.expectReturn {
					select {
					case <-done:
					case <-time.After(100 * time.Millisecond):
						t.Error("handleSignals should have returned after shutdown signal")
					}
				} else {
					select {
					case <-done:
						t.Error("handleSignals should not have returned for non-shutdown signal")

					case <-time.After(50 * time.Millisecond):
					}
				}

				if tt.expectContextDone {
					select {
					case <-ctx.Done():
					case <-time.After(100 * time.Millisecond):
						t.Error("context should have been cancelled")
					}
				} else {
					select {
					case <-ctx.Done():
						t.Error("context should not have been cancelled")

					case <-time.After(50 * time.Millisecond):
					}
				}

				logOutput := buf.String()
				if logOutput == "" {
					t.Error("expected log output but got none")
				}

				if tt.expectLogContains != "" && !containsString(logOutput, tt.expectLogContains) {
					t.Errorf("expected log to contain %q, but got: %s", tt.expectLogContains, logOutput)
				}

				if !containsString(logOutput, tt.signal.String()) {
					t.Errorf("expected log to contain signal name %q, but got: %s", tt.signal.String(), logOutput)
				}

				if !tt.expectReturn {
					close(sigChan)
					<-done
				}
			})
		})
	}
}

func TestHandleSignalsChannelClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Helper()

		var buf safeBuffer

		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		slog.SetDefault(logger)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)

		done := make(chan bool, 1)

		go func() {
			handleSignals(cancel, sigChan)

			done <- true
		}()

		close(sigChan)

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Error("handleSignals should have returned when signal channel was closed")
		}

		select {
		case <-ctx.Done():
			t.Error("context should not have been cancelled when channel was closed")
		case <-time.After(50 * time.Millisecond):
		}
	})
}

func TestHandleSignalsMultipleShutdownSignals(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Helper()

		var buf safeBuffer

		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		slog.SetDefault(logger)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 2)

		done := make(chan bool, 1)

		go func() {
			handleSignals(cancel, sigChan)

			done <- true
		}()

		sigChan <- syscall.SIGINT

		sigChan <- syscall.SIGTERM

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Error("handleSignals should have returned after first shutdown signal")
		}

		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
			t.Error("context should have been cancelled")
		}

		logOutput := buf.String()
		if !containsString(logOutput, "Received shutdown signal, initiating graceful shutdown") {
			t.Errorf("expected log to contain shutdown message, but got: %s", logOutput)
		}
	})
}

type safeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (sb *safeBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	n, err := sb.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("safeBuffer write failed: %w", err)
	}

	return n, nil
}

func (sb *safeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	return sb.buf.String()
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true

		for j := range len(substr) {
			if s[i+j] != substr[j] {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// FIXME? maybe
//
//nolint:gochecknoglobals
var (
	meter                metric.Meter
	QueriesKilledCounter metric.Int64Counter
)

// InitMetrics initializes OpenTelemetry metrics with OTLP exporter (for Datadog compatibility).
func InitMetrics(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"), // TODO: get from build info
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// This can export to Datadog via OTLP endpoint or local OTel collector
	otlpEndpoint := getOTLPEndpoint()

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithHeaders(getOTLPHeaders()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(30*time.Second), //nolint:mnd
		)),
	)

	otel.SetMeterProvider(provider)

	meter = otel.Meter("github.com/persona-id/query-sniper")

	err = initCounters()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize counters: %w", err)
	}

	slog.Info("OpenTelemetry metrics initialized",
		slog.String("endpoint", otlpEndpoint),
		slog.String("service_name", serviceName),
	)

	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}

	return shutdown, nil
}

// initCounters initializes all metric counters.
// Currently only supports the queries_killed_total counter.
func initCounters() error {
	var err error

	QueriesKilledCounter, err = meter.Int64Counter(
		"query_sniper.queries_killed_total",
		metric.WithDescription("Total number of queries killed by query-sniper"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return fmt.Errorf("failed to create queries_killed counter: %w", err)
	}

	return nil
}

// getOTLPEndpoint returns the OTLP endpoint URL
// Defaults to local collector, can be overridden via env var.
func getOTLPEndpoint() string {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	return "http://localhost:4318"
}

// getOTLPHeaders returns headers for OTLP exporter
// Useful for Datadog API key if sending directly to Datadog.
func getOTLPHeaders() map[string]string {
	headers := make(map[string]string)

	// If sending directly to Datadog, add API key
	if apiKey := os.Getenv("DD_API_KEY"); apiKey != "" {
		headers["DD-API-KEY"] = apiKey
	}

	// Support generic auth headers
	if authHeader := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); authHeader != "" {
		// Simple parsing of key=value pairs separated by commas
		// For production, consider using a proper parser
		slog.Debug("Using custom OTLP headers", slog.String("headers", authHeader))
	}

	return headers
}

// RecordQueryKilled increments the queries killed counter with attributes.
func RecordQueryKilled(ctx context.Context, database, reason, command string, duration int) {
	QueriesKilledCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("database", database),
			attribute.String("reason", reason),
			attribute.String("command", command),
		),
	)

	slog.Debug("recorded query_killed metric",
		slog.String("database", database),
		slog.String("reason", reason),
		slog.String("command", command),
		slog.Int("duration", duration),
	)
}

# OpenTelemetry Integration

This document describes the OpenTelemetry metrics integration added to Query Sniper.

## Overview

Query Sniper now emits OpenTelemetry metrics that can be consumed by any OTLP-compatible backend, including Datadog, Prometheus, and other observability platforms.

## Metrics

### `query_sniper.queries_killed_total`

**Type**: Counter
**Description**: Total number of queries killed by the sniper
**Unit**: `{query}`

**Attributes**:
- `database`: The database name where the query was killed
- `reason`: The reason for killing the query (`query_timeout`, `long_running_query`)
- `command`: The MySQL command type (`Query`, `Execute`, etc.)

## Configuration

### Environment Variables

The following environment variables control OpenTelemetry behavior:

#### OTLP Endpoint Configuration

```bash
# OTLP endpoint (defaults to http://localhost:4318)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Or metrics-specific endpoint
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318
```

#### Datadog Direct Integration

```bash
# For direct Datadog ingestion
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.datadoghq.com
DD_API_KEY=your_datadog_api_key
```

#### Custom Headers

```bash
# For custom authentication
OTEL_EXPORTER_OTLP_HEADERS="api-key=your-key,another-header=value"
```

## Deployment Patterns

### 1. OpenTelemetry Collector (Recommended)

Deploy an OpenTelemetry Collector alongside your application to receive metrics and forward them to multiple backends:

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  datadog:
    api:
      key: ${DD_API_KEY}
      site: datadoghq.com

service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [datadog]
```

### 2. Direct Datadog Ingestion

Set environment variables to send metrics directly to Datadog:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://api.datadoghq.com
export DD_API_KEY=your_datadog_api_key
```

### 3. Kubernetes Deployment

Use the OpenTelemetry Collector as a DaemonSet or sidecar:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: query-sniper
spec:
  template:
    spec:
      containers:
      - name: query-sniper
        image: query-sniper:latest
        env:
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "http://otel-collector:4318"
```

## Development

### Local Testing

1. Run a local OpenTelemetry Collector:

```bash
docker run --rm -p 4317:4317 -p 4318:4318 \
  otel/opentelemetry-collector-contrib:latest
```

2. Run Query Sniper with default settings (will use localhost:4318)

### Disable Metrics

If you need to disable metrics, the application will continue running normally even if the OTLP endpoint is unreachable. Check logs for initialization warnings.

## Troubleshooting

### Common Issues

1. **Connection Refused**: Check that your OTLP endpoint is reachable
2. **Authentication Errors**: Verify API keys and headers are correctly set
3. **No Metrics Appearing**: Check collector logs and verify pipeline configuration

### Debug Logging

Enable debug logging to see metric recording events:

```bash
export LOG_LEVEL=debug
```

This will show debug messages when metrics are recorded:

```
DEBUG recorded query_killed metric database=prod reason=query_timeout command=Query duration=45
```

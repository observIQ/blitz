# Metrics Documentation

This document describes the metrics exposed by the Bindplane Loader application.

## Overview

Bindplane Loader exposes Prometheus-compatible metrics via an HTTP endpoint. The metrics provide insights into the application's performance, including log generation rates, output throughput, error rates, and worker activity.

## Metrics Endpoint

The metrics are exposed on the following endpoint:

```
http://localhost:9100/metrics
```

### Scraping Metrics

To scrape metrics from the Bindplane Loader, configure your Prometheus server to scrape the metrics endpoint:

```yaml
scrape_configs:
  - job_name: 'bindplane-loader'
    static_configs:
      - targets: ['localhost:9100']
    scrape_interval: 15s
```

### Example: Fetching Metrics with curl

```bash
curl http://localhost:9100/metrics
```

This will return metrics in Prometheus format, for example:

```
# HELP bindplane_loader_generator_logs_generated_total Total number of logs generated
# TYPE bindplane_loader_generator_logs_generated_total counter
bindplane_loader_generator_logs_generated_total{component="generator_json"} 1500

# HELP bindplane_loader_tcp_logs_received_total Number of logs received from the write channel
# TYPE bindplane_loader_tcp_logs_received_total counter
bindplane_loader_tcp_logs_received_total{component="output_tcp"} 1500

# HELP bindplane_loader_tcp_workers_active Number of active worker goroutines
# TYPE bindplane_loader_tcp_workers_active gauge
bindplane_loader_tcp_workers_active{component="output_tcp"} 4
```

## Available Metrics

### Generator Metrics

#### `bindplane_loader_generator_logs_generated_total`
- **Type**: Counter
- **Description**: Total number of logs generated
- **Labels**:
  - `component`: Always `generator_json`

#### `bindplane_loader_generator_workers_active`
- **Type**: Gauge
- **Description**: Number of active worker goroutines
- **Labels**:
  - `component`: Always `generator_json`

#### `bindplane_loader_generator_write_errors_total`
- **Type**: Counter
- **Description**: Total number of write errors
- **Labels**:
  - `component`: Always `generator_json`
  - `error_type`: Either `unknown` or `timeout`

### TCP Output Metrics

#### `bindplane_loader_tcp_logs_received_total`
- **Type**: Counter
- **Description**: Number of logs received from the write channel
- **Labels**:
  - `component`: Always `output_tcp`

#### `bindplane_loader_tcp_workers_active`
- **Type**: Gauge
- **Description**: Number of active worker goroutines
- **Labels**:
  - `component`: Always `output_tcp`

#### `bindplane_loader_tcp_log_rate_total`
- **Type**: Counter (Float64)
- **Description**: Rate at which logs are successfully sent to the configured host
- **Labels**:
  - `component`: Always `output_tcp`

#### `bindplane_loader_tcp_request_size_bytes`
- **Type**: Histogram
- **Description**: Size of requests in bytes
- **Labels**:
  - `component`: Always `output_tcp`

#### `bindplane_loader_tcp_request_latency`
- **Type**: Histogram
- **Description**: Request latency in seconds
- **Labels**:
  - `component`: Always `output_tcp`

#### `bindplane_loader_tcp_send_errors_total`
- **Type**: Counter
- **Description**: Total number of send errors
- **Labels**:
  - `component`: Always `output_tcp`
  - `error_type`: Either `unknown` or `timeout`

### UDP Output Metrics

#### `bindplane_loader_udp_logs_received_total`
- **Type**: Counter
- **Description**: Number of logs received from the write channel
- **Labels**:
  - `component`: Always `output_udp`

#### `bindplane_loader_udp_workers_active`
- **Type**: Gauge
- **Description**: Number of active worker goroutines
- **Labels**:
  - `component`: Always `output_udp`

#### `bindplane_loader_udp_log_rate_total`
- **Type**: Counter (Float64)
- **Description**: Rate at which logs are successfully sent to the configured host
- **Labels**:
  - `component`: Always `output_udp`

#### `bindplane_loader_udp_request_size_bytes`
- **Type**: Histogram
- **Description**: Size of requests in bytes
- **Labels**:
  - `component`: Always `output_udp`

#### `bindplane_loader_udp_send_errors_total`
- **Type**: Counter
- **Description**: Total number of send errors
- **Labels**:
  - `component`: Always `output_udp`
  - `error_type`: Either `unknown` or `timeout`

## Metric Labels

### Component Labels

All metrics include a `component` label that identifies the source component:

- `generator_json`: Metrics from the JSON log generator
- `output_tcp`: Metrics from the TCP output
- `output_udp`: Metrics from the UDP output

### Error Type Labels

Error metrics include an `error_type` label with the following values:

- `unknown`: Generic errors that don't fit other categories
- `timeout`: Errors caused by operation timeouts

## Example Queries

### Log Generation Rate
```promql
rate(bindplane_loader_generator_logs_generated_total[5m])
```

### Active Workers by Component
```promql
bindplane_loader_generator_workers_active or bindplane_loader_tcp_workers_active or bindplane_loader_udp_workers_active
```

### Error Rate by Component
```promql
rate(bindplane_loader_generator_write_errors_total[5m]) + rate(bindplane_loader_tcp_send_errors_total[5m]) + rate(bindplane_loader_udp_send_errors_total[5m])
```

### Request Latency (TCP only)
```promql
histogram_quantile(0.95, rate(bindplane_loader_tcp_request_latency_bucket[5m]))
```

### Request Size Distribution
```promql
histogram_quantile(0.50, rate(bindplane_loader_tcp_request_size_bytes_bucket[5m]))
histogram_quantile(0.95, rate(bindplane_loader_tcp_request_size_bytes_bucket[5m]))
```

## Monitoring Recommendations

### Key Metrics to Monitor

1. **Log Generation Rate**: Monitor `bindplane_loader_generator_logs_generated_total` to ensure logs are being generated at expected rates
2. **Worker Health**: Monitor `bindplane_loader_*_workers_active` to ensure workers are running
3. **Error Rates**: Monitor error counters to detect issues early
4. **Throughput**: Monitor `bindplane_loader_*_log_rate_total` to track output performance
5. **Latency**: Monitor `bindplane_loader_tcp_request_latency` for TCP output performance

### Alerting Examples

```yaml
# Alert if no logs are being generated
- alert: NoLogGeneration
  expr: rate(bindplane_loader_generator_logs_generated_total[5m]) == 0
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "No logs are being generated"

# Alert if error rate is high
- alert: HighErrorRate
  expr: rate(bindplane_loader_generator_write_errors_total[5m]) + rate(bindplane_loader_tcp_send_errors_total[5m]) + rate(bindplane_loader_udp_send_errors_total[5m]) > 0.1
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "High error rate detected"

# Alert if workers are down
- alert: WorkersDown
  expr: bindplane_loader_generator_workers_active == 0 or bindplane_loader_tcp_workers_active == 0 or bindplane_loader_udp_workers_active == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "One or more worker pools are down"
```

## Troubleshooting

### Metrics Not Appearing

1. **Check if the metrics server is running**: Verify the application is running and the metrics endpoint is accessible
2. **Verify port 9100**: Ensure port 9100 is not blocked by firewall rules
3. **Check application logs**: Look for any errors related to metrics initialization

### High Error Rates

1. **Check network connectivity**: Verify TCP/UDP connections to target hosts
2. **Review timeout settings**: Check if timeouts are too aggressive
3. **Monitor resource usage**: Ensure sufficient CPU and memory resources

### Performance Issues

1. **Monitor worker counts**: Ensure adequate worker goroutines are running
2. **Check request latency**: Monitor `bindplane_loader_tcp_request_latency` for TCP performance
3. **Review request sizes**: Monitor `bindplane_loader_*_request_size_bytes` for optimal payload sizes

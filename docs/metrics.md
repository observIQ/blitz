# Metrics Documentation

This document describes the metrics exposed by the Blitz application.

## Overview

Blitz exposes Prometheus-compatible metrics via an HTTP endpoint. The metrics provide insights into the application's performance, including log generation rates, output throughput, error rates, worker activity, and channel utilization.

## Metrics Endpoint

The metrics are exposed on the following endpoint:

```
http://localhost:9100/metrics
```

### Example: Fetching Metrics with curl

```bash
curl http://localhost:9100/metrics
```

This will return metrics in Prometheus format, for example:

```
# HELP blitz_generator_logs_generated_total Total number of logs generated
# TYPE blitz_generator_logs_generated_total counter
blitz_generator_logs_generated_total{component="generator_json"} 1500

# HELP blitz_tcp_logs_received_total Number of logs received from the write channel
# TYPE blitz_tcp_logs_received_total counter
blitz_tcp_logs_received_total{component="output_tcp"} 1500

# HELP blitz_tcp_workers_active Number of active worker goroutines
# TYPE blitz_tcp_workers_active gauge
blitz_tcp_workers_active{component="output_tcp"} 4

# HELP blitz_tcp_channel_size Current size of the data channel
# TYPE blitz_tcp_channel_size gauge
blitz_tcp_channel_size{component="output_tcp"} 25
```

## Available Metrics

Blitz exposes metrics for each component (generators and outputs). For detailed information about metrics for specific components, see the individual component documentation in `docs/`.

### Metric Labels

All metrics include a `component` label that identifies the source component (e.g., `generator_json`, `output_tcp`, `output_udp`). Error metrics also include an `error_type` label with values such as `unknown` or `timeout`.
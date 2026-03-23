# HEC Output

The HEC output sends logs to Splunk via the HTTP Event Collector (HEC) protocol. It supports batching, TLS encryption, and Splunk's indexer acknowledgement (ACK) paradigm for at-least-once delivery.

## Data Mutation

Each log record is wrapped in a HEC JSON envelope with metadata fields (`time`, `host`, `source`, `sourcetype`, `index`). The event body can be sent as the raw message string or as a parsed JSON object, controlled by the `eventFormat` configuration.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `hec` to use this output. |
| `output.hec.host` | `--output-hec-host` | `BLITZ_OUTPUT_HEC_HOST` | `""` | HEC target host (IP address or hostname) |
| `output.hec.port` | `--output-hec-port` | `BLITZ_OUTPUT_HEC_PORT` | `8088` | HEC target port |
| `output.hec.token` | `--output-hec-token` | `BLITZ_OUTPUT_HEC_TOKEN` | `""` | HEC authentication token |
| `output.hec.workers` | `--output-hec-workers` | `BLITZ_OUTPUT_HEC_WORKERS` | `1` | Number of HEC output workers. Each worker has its own HEC channel. |
| `output.hec.batchSize` | `--output-hec-batchsize` | `BLITZ_OUTPUT_HEC_BATCHSIZE` | `100` | Maximum events per batch |
| `output.hec.batchTimeout` | `--output-hec-batchtimeout` | `BLITZ_OUTPUT_HEC_BATCHTIMEOUT` | `5s` | Maximum time before flushing a partial batch |
| `output.hec.eventFormat` | `--output-hec-eventformat` | `BLITZ_OUTPUT_HEC_EVENTFORMAT` | `raw` | Event body format: `raw` (message string) or `parsed` (structured JSON via ParseFunc) |

### Indexer Acknowledgement (ACK)

When enabled, the HEC output tracks each batch's acknowledgement status. Unacknowledged batches are resent after the ACK timeout, up to the configured retry limit.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.hec.enableAck` | `--output-hec-enableack` | `BLITZ_OUTPUT_HEC_ENABLEACK` | `true` | Enable Splunk indexer acknowledgement |
| `output.hec.ackPollInterval` | `--output-hec-ackpollinterval` | `BLITZ_OUTPUT_HEC_ACKPOLLINTERVAL` | `10s` | How often to poll for ACK status |
| `output.hec.ackTimeout` | `--output-hec-acktimeout` | `BLITZ_OUTPUT_HEC_ACKTIMEOUT` | `5m` | Time to wait for ACK before resending |
| `output.hec.maxRetries` | `--output-hec-maxretries` | `BLITZ_OUTPUT_HEC_MAXRETRIES` | `3` | Maximum resend attempts per batch before dropping |

### Event Metadata

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.hec.source` | `--output-hec-source` | `BLITZ_OUTPUT_HEC_SOURCE` | `blitz` | Source metadata for HEC events |
| `output.hec.sourceType` | `--output-hec-sourcetype` | `BLITZ_OUTPUT_HEC_SOURCETYPE` | `_json` | Sourcetype metadata for HEC events |
| `output.hec.index` | `--output-hec-index` | `BLITZ_OUTPUT_HEC_INDEX` | `""` | Target index (empty = token's default index) |

### TLS Configuration

TLS is enabled by default for HEC connections. To disable TLS (e.g., for local development), set `enableTLS` to `false`.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.hec.enableTLS` | `--output-hec-enable-tls` | `BLITZ_OUTPUT_HEC_ENABLE_TLS` | `true` | Enable TLS for HEC connections |
| `output.hec.tls.cert` | `--output-hec-tls-cert` | `BLITZ_OUTPUT_HEC_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.hec.tls.key` | `--output-hec-tls-key` | `BLITZ_OUTPUT_HEC_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.hec.tls.ca` | `--output-hec-tls-ca` | `BLITZ_OUTPUT_HEC_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format) |
| `output.hec.tls.skipVerify` | `--output-hec-tls-skip-verify` | `BLITZ_OUTPUT_HEC_TLS_SKIP_VERIFY` | `false` | Skip TLS certificate verification (not recommended for production) |
| `output.hec.tls.minVersion` | `--output-hec-tls-min-version` | `BLITZ_OUTPUT_HEC_TLS_MIN_VERSION` | `1.2` | Minimum TLS version: `1.2` or `1.3` |

## Example Configuration

### Basic HEC Output

```yaml
output:
  type: hec
  hec:
    host: splunk.example.com
    port: 8088
    token: your-hec-token
```

### HEC with ACK and Custom Metadata

```yaml
output:
  type: hec
  hec:
    host: splunk.example.com
    port: 8088
    token: your-hec-token
    workers: 4
    batchSize: 200
    batchTimeout: 3s
    eventFormat: parsed
    enableAck: true
    ackPollInterval: 10s
    ackTimeout: 5m
    maxRetries: 5
    source: my-application
    sourceType: app_logs
    index: main
```

### HEC without ACK (Splunk Cloud Platform)

```yaml
output:
  type: hec
  hec:
    host: inputs.splunkcloud.com
    port: 443
    token: your-hec-token
    enableAck: false
```

### Environment Variable Configuration

```bash
blitz \
  --output-type hec \
  --output-hec-host splunk.example.com \
  --output-hec-port 8088 \
  --output-hec-token your-hec-token \
  --generator-type json
```

Or via environment variables:

```bash
export BLITZ_OUTPUT_TYPE=hec
export BLITZ_OUTPUT_HEC_HOST=splunk.example.com
export BLITZ_OUTPUT_HEC_PORT=8088
export BLITZ_OUTPUT_HEC_TOKEN=your-hec-token
export BLITZ_GENERATOR_TYPE=json
blitz
```

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `blitz.hec.logs.received` | Counter | Logs received from the write channel |
| `blitz.hec.workers.active` | Gauge | Active worker goroutines |
| `blitz.hec.log.rate` | Counter | Successfully sent events |
| `blitz.hec.request.size.bytes` | Histogram | HTTP request body size |
| `blitz.hec.request.latency` | Histogram | HTTP POST latency (seconds) |
| `blitz.hec.send.errors` | Counter | Send errors (attribute: `error_type`) |
| `blitz.hec.batch.size` | Histogram | Events per batch |
| `blitz.hec.ack.pending` | Gauge | ackIds awaiting confirmation |
| `blitz.hec.ack.confirmed` | Counter | ackIds confirmed by Splunk |
| `blitz.hec.ack.expired` | Counter | ackIds that timed out (triggered resend) |
| `blitz.hec.ack.retried` | Counter | Batches resent after ACK timeout |
| `blitz.hec.ack.dropped` | Counter | Batches dropped after max retries |
| `blitz.hec.ack.poll.latency` | Histogram | ACK poll request latency (seconds) |

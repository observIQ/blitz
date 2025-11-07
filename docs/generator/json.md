# JSON Generator

The JSON generator creates structured JSON log entries with configurable fields. Two log types are supported: default logs with standard fields, and PII logs with personally identifiable information fields suitable for testing PII detection and redaction systems.

## Example Logs

### Default Log Type

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "environment": "production",
  "location": "us-east1",
  "message": "User authentication failed for user_id=12345, ip_address=192.168.1.100, reason=invalid_password, attempt_count=3, timestamp=2024-01-15T10:30:45Z, session_id=abc123def456, user_agent=Mozilla/5.0, location=us-east-1, service=auth-service"
}
```

### PII Log Type

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "message": "Customer service request completed",
  "user_id": "01234567-89abcdef-01234567-89abcdef",
  "iban": "US123456789012345678901234",
  "phone": "+1-555-123-4567",
  "ssn": "123-45-6789",
  "event": "processed transaction",
  "action": "approved loan application",
  "status": "successful",
  "type": "info",
  "detail": "Transaction completed successfully"
}
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `json` to use this generator. |
| `generator.json.workers` | `--generator-json-workers` | `BLITZ_GENERATOR_JSON_WORKERS` | `1` | Number of JSON generator workers (must be ≥ 1) |
| `generator.json.rate` | `--generator-json-rate` | `BLITZ_GENERATOR_JSON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |
| `generator.json.type` | `--generator-json-type` | `BLITZ_GENERATOR_JSON_TYPE` | `default` | Type of log to generate. Valid values: `default`, `pii` |

## Example Configuration

```yaml
generator:
  type: json
  json:
    workers: 2
    rate: 500ms
    type: default
```

### PII Log Type Example

```yaml
generator:
  type: json
  json:
    workers: 2
    rate: 500ms
    type: pii
```

## Metrics

The JSON generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_json`.


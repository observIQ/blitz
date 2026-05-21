# Palo Alto Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Palo Alto generator generates realistic Palo Alto firewall syslog entries in the standard Palo Alto log format. These logs are suitable for testing Palo Alto firewall log processing and analysis systems.

## Example Logs

```
1,2024/01/15 10:30:45,001234567890,SYSTEM,threat,2049,2024/01/15 10:30:45,192.0.2.10,10.0.0.1,0.0.0.0,0.0.0.0,rule1,,,inbound,vsys1,trust,untrust,ethernet1/1,ethernet1/2,Forward,1,2024/01/15 10:30:45,123456,1,62873,0,0,0x0,tcp,allow,2049,2024/01/15 10:30:45,10,any,0,1234567890,0x8000000000000000,192.168.1.0-192.168.1.255,United States,0,1,0,aged-out,0,0,0,0,,N/A,0,0,0,0,,,from-policy,,,0,,0,,N/A,unknown,AppThreat-0000,0x0,0x0000000000000000
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `palo-alto` to use this generator. |
| `generator.paloAlto.workers` | `--generator-paloalto-workers` | `BLITZ_GENERATOR_PALOALTO_WORKERS` | `1` | Number of Palo Alto generator workers (must be ≥ 1) |
| `generator.paloAlto.rate` | `--generator-paloalto-rate` | `BLITZ_GENERATOR_PALOALTO_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: palo-alto
  paloAlto:
    workers: 2
    rate: 500ms
```

## Metrics

The Palo Alto generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_paloalto`.


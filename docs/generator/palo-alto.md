# Palo Alto Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Palo Alto generator generates realistic PAN-OS firewall syslog entries in the standard comma-separated field format. It emits all 13 log types the Bindplane Palo Alto blueprints parse, each at its full PAN-OS 11.0 field width.

## Log Types

Every emitted record is one of these types, selected at random per record. The field count is the number of comma-separated fields in the record body (field 1 is `FUTURE_USE`), validated against Palo Alto's public PAN-OS 11.0 "Syslog Field Descriptions".

| Log Type | Fields |
|----------|-------:|
| TRAFFIC | 115 |
| THREAT | 121 |
| SYSTEM | 26 |
| CONFIG | 28 |
| AUTHENTICATION | 47 |
| CORRELATION | 22 |
| DECRYPTION | 106 |
| GLOBALPROTECT | 50 |
| GTP | 94 |
| HIP-MATCH | 32 |
| IPTAG | 27 |
| SCTP | 65 |
| USERID | 37 |

Field order is validated against Palo Alto's public PAN-OS 11.0 "Syslog Field Descriptions" and enforced by tests (`generator/paloalto/logs.go` holds the ordered field-name list per type; `logs_test.go` asserts count and named-position order). The authoritative page is unified across 11.0 and later; for TRAFFIC the page explicitly marks Flow Type, AI Traffic, AI Forward Error, K8S Cluster ID (11.1+) and Adv DevID (12.1.2+), which are excluded. Counts and full field order were extracted from the rendered spec pages (headless Chrome) and are enforced by tests. DECRYPTION 106 and THREAT 121 exclude fields the pages mark 11.1+/12.1.2+ (Cluster Name, Flow Type, AI/K8S/Adv DevID); GTP 94 has none.

## Example Log (SYSTEM)

```
Jan 15 10:30:45 1,2024/01/15 10:30:45,001234567890,SYSTEM,general,,2024/01/15 10:30:43,vsys1,42,general,,,general,informational,Config installed,1234567,0x0,0,0,0,0,vsys1,PA-VM,,,2024-01-15T10:30:45.000-05:00
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `palo-alto` to use this generator. |
| `generator.palo-alto.workers` | `--generator-palo-alto-workers` | `BLITZ_GENERATOR_PALO_ALTO_WORKERS` | `1` | Number of Palo Alto generator workers (must be ≥ 1) |
| `generator.palo-alto.rate` | `--generator-palo-alto-rate` | `BLITZ_GENERATOR_PALO_ALTO_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: palo-alto
  palo-alto:
    workers: 2
    rate: 500ms
```

## Metrics

The Palo Alto generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_paloalto`.


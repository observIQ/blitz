# Host Metrics Generator

The Host Metrics generator produces synthetic host-level metric data that mimics the OpenTelemetry Host Metrics receiver. It generates metrics for CPU, memory, disk, network, filesystem, load, paging, and processes.

## Telemetry Type

This generator produces **metrics** (not logs). It must be paired with an output that supports metrics, such as `otlp-grpc` or `stdout`.

## Scrapers

The generator includes 8 scrapers, each producing metrics for a specific subsystem:

| Scraper      | Metrics Produced                                           |
|-------------|-----------------------------------------------------------|
| `cpu`        | `system.cpu.time`, `system.cpu.utilization`               |
| `memory`     | `system.memory.usage`, `system.memory.utilization`        |
| `disk`       | `system.disk.io`, `system.disk.operations`, `system.disk.operation_time`, `system.disk.io_time` |
| `network`    | `system.network.io`, `system.network.packets`, `system.network.errors`, `system.network.dropped` |
| `filesystem` | `system.filesystem.usage`, `system.filesystem.utilization` |
| `load`       | `system.cpu.load_average.1m`, `system.cpu.load_average.5m`, `system.cpu.load_average.15m` |
| `paging`     | `system.paging.usage`, `system.paging.utilization`, `system.paging.operations`, `system.paging.faults` |
| `processes`  | `system.processes.count`                                   |

## Configuration

| YAML Path                          | Flag Name                          | Environment Variable                    | Default   | Description                                                          |
|------------------------------------|------------------------------------|-----------------------------------------|-----------|----------------------------------------------------------------------|
| `generator.type`                   | `--generator-type`                 | `BLITZ_GENERATOR_TYPE`                  | `nop`     | Generator type. Set to `hostmetrics` to use this generator.          |
| `generator.hostmetrics.workers`    | `--generator-hostmetrics-workers`  | `BLITZ_GENERATOR_HOSTMETRICS_WORKERS`   | `1`       | Number of worker goroutines.                                         |
| `generator.hostmetrics.rate`       | `--generator-hostmetrics-rate`     | `BLITZ_GENERATOR_HOSTMETRICS_RATE`      | `1s`      | Scrape interval for host metrics.                                    |
| `generator.hostmetrics.os`         | `--generator-hostmetrics-os`       | `BLITZ_GENERATOR_HOSTMETRICS_OS`        | `linux`   | Simulated operating system. One of: `linux`, `windows`.              |
| `generator.hostmetrics.hostname`   | `--generator-hostmetrics-hostname` | `BLITZ_GENERATOR_HOSTMETRICS_HOSTNAME`  | (random)  | Simulated hostname. If empty, a random hostname is generated.        |
| `generator.hostmetrics.scrapers`   | `--generator-hostmetrics-scrapers` | `BLITZ_GENERATOR_HOSTMETRICS_SCRAPERS`  | (all)     | Scrapers to enable. If empty, all scrapers are enabled.              |

## Example Configuration

```yaml
generator:
  type: hostmetrics
  hostmetrics:
    workers: 1
    rate: 1s
    os: linux
    scrapers:
      - cpu
      - memory
      - disk
output:
  type: otlp-grpc
  otlpGrpc:
    host: localhost
    port: 4317
```

## Example CLI Usage

```bash
blitz --generator-type hostmetrics --output-type stdout --generator-hostmetrics-rate 1s
```

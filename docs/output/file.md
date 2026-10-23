# File Output

The File output writes telemetry to files with automatic rotation and compression. This is useful for local storage and testing file-based processing systems.

## Data Mutation

The File output does not mutate data; it writes log records as-is to the specified file, with each record on a new line.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `file` to use this output. |
| `output.file.path` | `--output-file-path` | `BLITZ_OUTPUT_FILE_PATH` | `""` | Destination file path (required when using file output) |
| `output.file.workers` | `--output-file-workers` | `BLITZ_OUTPUT_FILE_WORKERS` | `1` | Number of File output workers (must be ≥ 0) |
| `output.file.rotation.maxSizeMB` | `--output-file-rotation-maxsizemb` | `BLITZ_OUTPUT_FILE_ROTATION_MAXSIZEMB` | `100` | Maximum size in MB before rotation |
| `output.file.rotation.maxBackups` | `--output-file-rotation-maxbackups` | `BLITZ_OUTPUT_FILE_ROTATION_MAXBACKUPS` | `7` | Maximum number of backups to retain |
| `output.file.rotation.maxAgeDays` | `--output-file-rotation-maxagedays` | `BLITZ_OUTPUT_FILE_ROTATION_MAXAGEDAYS` | `30` | Maximum age in days to retain backups |
| `output.file.rotation.compress` | `--output-file-rotation-compress` | `BLITZ_OUTPUT_FILE_ROTATION_COMPRESS` | `true` | Compress rotated files |
| `output.file.rotation.localTime` | `--output-file-rotation-localtime` | `BLITZ_OUTPUT_FILE_ROTATION_LOCALTIME` | `false` | Use local time for backup timestamps |

## Example Configuration

```yaml
output:
  type: file
  file:
    path: /var/log/blitz/output.log
    workers: 2
    rotation:
      maxSizeMB: 100
      maxBackups: 7
      maxAgeDays: 30
      compress: true
      localTime: false
```

## Metrics

The File output records the shared output metrics:

- **`blitz.output.entries_received`** (Counter): Entries received by the output
- **`blitz.output.active_workers`** (Gauge): Number of active worker goroutines
- **`blitz.output.entry_rate`** (Float64 Counter): Rate at which entries are sent
- **`blitz.output.request_size`** (Histogram): Size of requests in bytes
- **`blitz.output.send_errors`** (Counter): Total number of send errors, by `telemetry_type`
- **`blitz.output.queue_size`** (Observable Gauge): Current number of entries in the queue

All metrics carry an `output_type` attribute set to `file`. See [Metrics](../metrics.md) for the exported names.


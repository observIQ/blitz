# File Output

The File output writes logs to files with automatic rotation and compression. This is useful for local log storage and testing file-based log processing systems.

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

The File output exposes the following metrics:

- **`blitz_file_logs_received_total`** (Counter): Number of logs received from the write channel
- **`blitz_file_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_file_log_rate_total`** (Counter, Float64): Rate at which logs are successfully written to file
- **`blitz_file_request_size_bytes`** (Histogram): Size of write requests in bytes
- **`blitz_file_write_errors_total`** (Counter): Total number of file write errors, labeled by `error_type`
- **`blitz_file_channel_size`** (Gauge): Current size of the data channel

All metrics include a `component` label set to `output_file`.


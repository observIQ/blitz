# Stdout Output

The stdout output writes all generated logs to standard output (stdout). This is useful for debugging and testing.

**Note:** The stdout output may not be suitable for piping to another process, as stdout is shared with the main blitz logger. Both application logs and generated log data will be written to stdout, which can make it difficult to separate them when piping.

## Buffering

Log records are written into an in-process buffer (`bufio.Writer` wrapping `os.Stdout`) and flushed either when the buffer fills or every `flushInterval`. This significantly reduces syscall overhead when running with many generator workers.

On graceful shutdown (SIGINT, SIGTERM), buffered records are flushed before exit. Records remaining in the buffer during a hard kill (SIGKILL) or crash are lost.

**Trade-off:** shorter intervals reduce write latency at the cost of more frequent syscalls; longer intervals maximize throughput.

## Data Mutation

The stdout output does not mutate data; it writes log records as-is to standard output, with each record on a new line.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `stdout` to use this output. |
| `output.stdout.flushInterval` | `--output-stdout-flushinterval` | `BLITZ_OUTPUT_STDOUT_FLUSHINTERVAL` | `100ms` | How often to flush buffered log records to stdout. |

## Example Configuration

```yaml
output:
  type: stdout
  stdout:
    flushInterval: 100ms
```

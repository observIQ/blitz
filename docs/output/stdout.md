# Stdout Output

The stdout output writes all generated logs to standard output (stdout). This is useful for debugging and testing.

**Note:** The stdout output may not be suitable for piping to another process, as stdout is shared with the main blitz logger. Both application logs and generated log data will be written to stdout, which can make it difficult to separate them when piping.

## Data Mutation

The stdout output does not mutate data; it writes log records as-is to standard output, with each record on a new line.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `stdout` to use this output. |

**No additional configuration options are required for the stdout output.**

## Example Configuration

```yaml
output:
  type: stdout
```


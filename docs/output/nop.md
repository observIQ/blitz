# NOP Output

The NOP (No Operation) output performs no work and discards all data. It's useful for testing the application infrastructure without actually sending data to external destinations.

## Data Mutation

The NOP output does not mutate data; it simply discards all received log records.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `nop` to use this output. |

**No additional configuration options are required for the NOP output.**

## Example Configuration

```yaml
output:
  type: nop
```


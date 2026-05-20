# NOP Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The NOP (No Operation) generator performs no work and generates no data. It's useful for testing the application infrastructure without generating actual log data.

## Example Logs

The NOP generator does not produce any log output.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `nop` to use this generator. |

**No additional configuration options are required for the NOP generator.**

## Example Configuration

```yaml
generator:
  type: nop
```


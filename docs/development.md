# Development

## Understanding the Codebase

Before contributing to blitz, we recommend reading the [Architecture Overview](/docs/architecture.md) to understand the application structure, components, and data flow. This will help you understand how different parts of the system interact and where to make changes.

## Requirements

You should have Go 1.25+ installed.

## Local Build

```bash
make build
```

## Testing

```bash
make test
```

## Adding a Data Library

To add a new data library, create a directory under `data_library/` with your log files:

```bash
mkdir -p data_library/my-dataset
# Add your log files to data_library/my-dataset/
```

The `filegen` generator automatically discovers and reads files from data library directories. Use it with:

```bash
./blitz --generator-type=filegen --generator-filegen-source=data_library/my-dataset --output-type=stdout
```

The generator reads files line-by-line and supports timestamp directives (e.g., `%Y-%m-%dT%H:%M:%SZ`) for dynamic timestamp generation.

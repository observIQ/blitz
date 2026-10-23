# Development

## Understanding the Codebase

Before contributing to blitz, we recommend reading the [Architecture Overview](architecture.md) to understand the application structure, components, and data flow. This will help you understand how different parts of the system interact and where to make changes.

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

To add a new data library package, create a directory under `generator/filegen/embeddedlibrary/data_library/` with your log files:

```bash
mkdir -p generator/filegen/embeddedlibrary/data_library/my-dataset
# Add your log files to generator/filegen/embeddedlibrary/data_library/my-dataset/
```

Use it with:

```bash
./blitz --generator-type=filegen --generator-filegen-source=my-dataset --output-type=stdout
```

The generator supports timestamp directives (e.g., `%Y-%m-%dT%H:%M:%SZ`) for dynamic timestamp generation.

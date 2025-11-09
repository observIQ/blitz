# PostgreSQL Log Format Generator

The PostgreSQL generator creates logs in PostgreSQL's standard log format. This format matches PostgreSQL's default `log_line_prefix` configuration (`'%t [%p]: user=%u,db=%d,app=%a,client=%h '`) and includes timestamp, process ID, user, database, application name, client address, severity level, and log message. Query logs are included in the same format when `log_statement = 'all'` is configured in PostgreSQL.

## Description

The PostgreSQL log format follows the specification: `<timestamp> [<process_id>]: user=<user>,db=<database>,app=<application>,client=<client_address> <severity>:  <message>`. This format is the standard PostgreSQL log format used when logging is enabled via `logging_collector = on` and `log_statement = 'all'` in `postgresql.conf`. Query logs are included in the same log file as other PostgreSQL logs.

## Example Logs

```
2024-01-15 10:23:45.123 UTC [12345]: user=postgres,db=mydb,app=psql,client=127.0.0.1 LOG:  statement: SELECT * FROM users WHERE id = $1
2024-01-15 10:23:46.456 UTC [12346]: user=app_user,db=appdb,app=application,client=192.168.1.100 LOG:  statement: INSERT INTO orders (user_id, total) VALUES ($1, $2)
2024-01-15 10:23:47.789 UTC [12347]: user=admin,db=postgres,app=pgAdmin,client=10.0.0.5 LOG:  duration: 12.345 ms
2024-01-15 10:23:48.012 UTC [12348]: user=readonly_user,db=analytics,app=worker,client=172.16.0.10 ERROR:  relation "nonexistent" does not exist
2024-01-15 10:23:49.234 UTC [12349]: user=postgres,db=postgres,app=-,client=127.0.0.1 LOG:  connection received: host=127.0.0.1 port=54321
2024-01-15 10:23:50.567 UTC [12350]: user=backup_user,db=warehouse,app=backup_script,client=192.168.1.200 LOG:  checkpoint complete: wrote 1024 buffers (6.3%); 0 WAL file(s) added, 0 removed, 1 recycled
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `postgres` to use this generator. |
| `generator.postgres.workers` | `--generator-postgres-workers` | `BLITZ_GENERATOR_POSTGRES_WORKERS` | `1` | Number of PostgreSQL generator workers (must be ≥ 1) |
| `generator.postgres.rate` | `--generator-postgres-rate` | `BLITZ_GENERATOR_POSTGRES_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: postgres
  postgres:
    workers: 5
    rate: 100ms
```

## Metrics

The PostgreSQL generator exposes the following metrics:

- **`blitz.generator.logs.generated`** (Counter): Total number of logs generated
- **`blitz.generator.workers.active`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write.errors`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_postgres`.


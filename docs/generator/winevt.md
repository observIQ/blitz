# Windows Event (winevt) Generator

The Windows Event generator generates Windows Event logs in unparsed XML format. These logs follow the standard Windows Event XML schema and are suitable for testing Windows Event log processing systems.

## Example Logs

```xml
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Microsoft-Windows-Security-Auditing' Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}'/><EventID>4625</EventID><Version>0</Version><Level>0</Level><Task>12544</Task><Opcode>0</Opcode><Keywords>0x8010000000000000</Keywords><TimeCreated SystemTime='2025-10-30T14:37:24.1621700Z'/><EventRecordID>1536271</EventRecordID><Correlation ActivityID='{2d231b4c-7851-0001-ff20-1ea65f45dc01}'/><Execution ProcessID='660' ThreadID='3060'/><Channel>Security</Channel><Computer>workstation-0</Computer><Security/></System><EventData><Data Name='SubjectUserSid'>S-1-0-0</Data><Data Name='SubjectUserName'>-</Data><Data Name='SubjectDomainName'>-</Data><Data Name='SubjectLogonId'>0x0</Data><Data Name='TargetUserSid'>S-1-0-0</Data><Data Name='TargetUserName'>ADMIN</Data><Data Name='TargetDomainName'>-</Data><Data Name='Status'>0xc000006d</Data><Data Name='FailureReason'>%%2313</Data><Data Name='SubStatus'>0xc0000064</Data><Data Name='LogonType'>3</Data><Data Name='LogonProcessName'>NtLmSsp </Data><Data Name='AuthenticationPackageName'>NTLM</Data><Data Name='WorkstationName'>-</Data><Data Name='TransmittedServices'>-</Data><Data Name='LmPackageName'>-</Data><Data Name='KeyLength'>0</Data><Data Name='ProcessId'>0x0</Data><Data Name='ProcessName'>-</Data><Data Name='IpAddress'>192.0.2.10</Data><Data Name='IpPort'>0</Data></EventData><RenderingInfo Culture='en-US'><Message>An account failed to log on.</Message><Level>Information</Level><Task>Logon</Task><Opcode>Info</Opcode><Channel>Security</Channel><Provider>Microsoft Windows security auditing.</Provider><Keywords><Keyword>Audit Failure</Keyword></Keywords></RenderingInfo></Event>
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `winevt` to use this generator. |
| `generator.winevt.workers` | `--generator-winevt-workers` | `BLITZ_GENERATOR_WINEVT_WORKERS` | `1` | Number of winevt generator workers (must be ≥ 1) |
| `generator.winevt.rate` | `--generator-winevt-rate` | `BLITZ_GENERATOR_WINEVT_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: winevt
  winevt:
    workers: 2
    rate: 500ms
```

## Metrics

The Windows Event generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_winevt`.


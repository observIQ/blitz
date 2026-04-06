# Windows Event Log (wel) Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md)). The XML-output mode in this package is a Producer. A future Windows-API mode that writes directly to the actual Windows Event Log via the EventWriter interface is an Effector and will not be embed-eligible.

The `wel` generator generates Windows Event Log entries spanning many WEL channels (Security, System, Application, PowerShell/Operational, Sysmon/Operational, Defender/Operational, Terminal Services, TaskScheduler, DNS Client, Firewall, plus the Domain-Controller-only DNS Server, DFS Replication, NTDS, AD DS, and Group Policy channels). Event selection is driven by a machine-role filter (`workstation`, `member`, `dc`).

Output is unparsed XML in standard WEL schema form, identical in shape to what `winevt` produces but covering substantially more events, channels, and correlation scenarios (logon/logoff session tracking, process create/exit pairs, etc.).

## Differences from `winevt`

The legacy `winevt` generator emits a single-template Security 4625 (failed logon) variant on a fixed cadence. The `wel` generator:

- Selects from hundreds of event definitions across many channels.
- Tracks short-lived state (logon sessions, process IDs) to emit causally-correlated events (Sysmon Process Create 1 → Process Exit 5, Security Logon 4624 → Logoff 4634, etc.).
- Filters its event pool by machine role so a configured `workstation` instance emits a different event mix than a `dc` instance.
- Allows the channel list to be filtered (e.g. `Security` only).

## Example log

```xml
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Microsoft-Windows-Security-Auditing' Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}'/><EventID>4624</EventID><Version>2</Version><Level>0</Level><Task>12544</Task><Opcode>0</Opcode><Keywords>0x8020000000000000</Keywords><TimeCreated SystemTime='2026-05-21T14:37:24.1621700Z'/><EventRecordID>1536271</EventRecordID><Correlation/><Execution ProcessID='660' ThreadID='3060'/><Channel>Security</Channel><Computer>BLITZ-WEL</Computer><Security/></System><EventData><Data Name='SubjectUserSid'>S-1-5-18</Data><Data Name='SubjectUserName'>BLITZ-WEL$</Data><Data Name='SubjectDomainName'>CONTOSO</Data><Data Name='SubjectLogonId'>0x3e7</Data><Data Name='TargetUserSid'>S-1-5-21-1234567890-1234567890-1234567890-1001</Data><Data Name='TargetUserName'>jsmith</Data><Data Name='TargetDomainName'>CONTOSO</Data><Data Name='TargetLogonId'>0x12345abc</Data><Data Name='LogonType'>2</Data><Data Name='LogonProcessName'>User32</Data><Data Name='AuthenticationPackageName'>Negotiate</Data><Data Name='WorkstationName'>BLITZ-WEL</Data><Data Name='IpAddress'>10.0.0.1</Data><Data Name='IpPort'>0</Data></EventData></Event>
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Set to `wel` to use this generator. |
| `generator.wel.workers` | `--generator-wel-workers` | `BLITZ_GENERATOR_WEL_WORKERS` | `1` | Number of WEL generator workers (must be ≥ 1) |
| `generator.wel.rate` | `--generator-wel-rate` | `BLITZ_GENERATOR_WEL_RATE` | `1s` | Per-worker generation interval |
| `generator.wel.role` | `--generator-wel-role` | `BLITZ_GENERATOR_WEL_ROLE` | `member` | Machine role: `workstation`, `member`, or `dc`. Controls which event pool is eligible. |
| `generator.wel.channels` | `--generator-wel-channels` | `BLITZ_GENERATOR_WEL_CHANNELS` | (all) | Optional channel filter (comma-separated). Empty = all channels eligible for the role. |
| `generator.wel.computer` | `--generator-wel-computer` | `BLITZ_GENERATOR_WEL_COMPUTER` | `BLITZ-WEL` | Computer name to embed in events |
| `generator.wel.domain` | `--generator-wel-domain` | `BLITZ_GENERATOR_WEL_DOMAIN` | `WORKGROUP` | Domain name for AD-flavored events |
| `generator.wel.manageEventSources` | `--generator-wel-manage-event-sources` | `BLITZ_GENERATOR_WEL_MANAGE_EVENT_SOURCES` | `false` | Reserved for the Effector Windows-API mode (no-op for XML mode) |

## Example configurations

### Workstation host emitting a mix of Security + Sysmon + Application events

```yaml
generator:
  type: wel
  wel:
    workers: 2
    rate: 500ms
    role: workstation
    computer: WS-12345
    domain: CONTOSO
```

### Domain controller emitting AD DS / GPO / NTDS / DNS Server events

```yaml
generator:
  type: wel
  wel:
    workers: 1
    rate: 250ms
    role: dc
    computer: DC01
    domain: CONTOSO
```

### Security-only stream (login auditing scenarios)

```yaml
generator:
  type: wel
  wel:
    workers: 1
    rate: 1s
    role: member
    channels:
      - Security
```

## Metrics

The WEL generator emits the same weaver-generated counters as every other Producer generator:

- `blitz_generator_logs_generated_total` (Counter, label `component=wel`, label `channel=<channel>`): Total events emitted.
- `blitz_generator_workers_active` (Gauge, label `component=wel`): Active worker count.
- `blitz_generator_write_errors_total` (Counter, labels `component=wel`, `error_type` ∈ {`unknown`, `timeout`}): Consumer-rejection count.

# Palo Alto Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Palo Alto generator generates realistic PAN-OS firewall syslog entries in the standard comma-separated field format. It emits all 13 log types the Bindplane Palo Alto blueprints parse, each at the width a PAN-OS 11.0 firewall sends.

## Log Types

Every emitted record is one of these types, selected at random per record, under the type string the firewall sends. The field count is the number of comma-separated fields in the record body (field 1 is `FUTURE_USE`).

| Log Type | Fields |
|----------|-------:|
| TRAFFIC | 117 |
| THREAT | 123 |
| SYSTEM | 26 |
| CONFIG | 26 |
| AUTH | 47 |
| CORRELATION | 22 |
| DECRYPTION | 108 |
| GLOBALPROTECT | 50 |
| GTP | 106 |
| HIPMATCH | 32 |
| IPTAG | 28 |
| SCTP | 73 |
| USERID | 37 |

Field order and values follow a real capture from a VM-Series firewall on
PAN-OS 11.0.4-h6, kept as the test fixture
`generator/paloalto/testdata/panos-11.0.4-h6.log`. Fields are named from Palo
Alto's 11.0 "Syslog Field Descriptions". Where the capture and those pages
disagree, the generator follows the capture, since that is what collectors
receive:

- Every captured type carries the config version (`2816`) in field 6, which
  most pages call `FUTURE_USE`.
- Most captured types end with `Cluster Name`, which the pages mark 11.1+.
  SYSTEM and CONFIG end with `High Resolution Timestamp`, and THREAT ends with
  `Flow Type` after `Cluster Name`.
- TRAFFIC and THREAT carry `Flow Type` (`NonProxyTraffic`).
- CONFIG has no Before or After Change Detail.
- SCTP records carry a Traffic-like session prefix and an event name
  (`SCTP association start`).
- GTP records use the GTP page's order, with its `FUTURE_USE` slots filled by
  the Tunnel Inspection page's fields. Tunnel-inspection sessions (`gre`,
  `gtp-u`) leave the GTP fields zero, as captured.
- Authentication and HIP Match logs are typed `AUTH` and `HIPMATCH`.

The capture has no CORRELATION record, no THREAT `virus` or `wildfire` record,
and no GTP-C session. Those records follow the 11.0 field pages and real-device
samples instead.

A CORRELATION record reports the one correlation object seen in a device's log,
Beacon Detection (ID 6005): category `compromised-host`, severity `medium`, an
empty Content/Threat Type, and evidence such as `Host visited known malware URL
(11 times).`

Each record describes one session as the firewall logs it. In TRAFFIC records:

- `Bytes` and `Packets` are the sums of their sent and received counts.
- `Start Time` precedes `Generated Time` by `Elapsed Time` plus the lag the
  device shows: 0-7 s for `start`, 0-4 s for `deny`, 1-4 s for `drop`, and
  3-94 s for `end`, mostly 17-20 s.
- `start` and `end` sessions are allowed. A `start` log's end reason is `n/a`.
- An `end` session ends on `tcp-fin`, `tcp-rst-from-client`,
  `tcp-rst-from-server`, `aged-out`, or `threat` for TCP, and on `aged-out` or
  `threat` otherwise.
- A `deny` session is logged with action `allow` and ends on `policy-deny`, or
  on `auth-policy-redirect` for `web-browsing`.
- A `drop` is denied or dropped with session ID `0` and no egress interface.
- `Category` is the destination's URL category (`private-ip-addresses` for
  RFC 1918) on `end` web sessions and on some `google-base` `start` sessions,
  and `any` otherwise. An `end` session can also log `any` (`web-browsing`),
  `not-resolved` (`google-base`), or a threat category (`malware`, `phishing`,
  `command-and-control`) when it ends on `threat`.
- `Flags` carries `0x400000` exactly when the session is translated, in
  TRAFFIC, THREAT, and DECRYPTION records.

Sessions leave the trust zone (`ethernet1/2`) for the untrust zone
(`ethernet1/1`). To a public address, the source is translated to the untrust
interface's address on a new port. A private destination is not translated,
and its NAT fields are `0.0.0.0` and `0`.

The `Application` names its service, so the protocol and destination port
match it (`ssl` is TCP 443, `dns-base` is UDP 53). Its App-ID metadata
(subcategory, category, technology, risk, characteristics, container) is the
firewall's own: from the capture, or for `ssh` and `smtp-base`, from the
firewall's predefined application database. Session IDs start at a small
number (up to 100,000) and rise by one per session. `Sequence Number` is 64-bit and rises per log type.
Unused counters are `0`, and `Application SaaS` and `Sanctioned State` are
`no`.

Location fields describe their address: an RFC 1918 address gets its region
range (`10.0.0.0-10.255.255.255`), and a public one gets a country name
(`United States`). GlobalProtect `Source Region` describes the client's public
address the same way.

THREAT records are detections the capture logged:

- URL filtering with a scheme-less URL and `(9999)`.
- DNS spyware with `generic:<domain>(<id>)`.
- PE file blocks.
- Vulnerabilities, including `Eicar File Detected(39040)`.
- Data filtering.
- `ml-virus`.

SYSTEM records are captured events, with `Module` `general`. The other types
also follow their captured events:

- GlobalProtect records follow a portal and gateway login.
- User-ID records come from the XML API, GlobalProtect, and Authentication
  Portal. Domain users carry the device's domain (`blitz\jdoe` on
  `pa-fw-01.blitz.local`), or `corp` when the device has no FQDN.
- Each source address maps to one user, as User-ID maps it: every record
  from that host (TRAFFIC, THREAT, DECRYPTION, and User-ID) names the same
  user.
- IP-Tag records register or unregister a tag.
- AUTH records carry Authentication Portal successes and password failures.
- DECRYPTION records carry the captured TLS handshake fields and certificates.

Quoting matches the device. SYSTEM `Description`, THREAT `URL/Filename` and
`URL Category List`, and GlobalProtect `Client OS Version` and `Description`
are quoted whenever they hold a value. Any other field is quoted only when it
holds a comma or a quote.

## The device

Every line describes one firewall. With the simulated environment (the CLI and
embedded loader always build one), that firewall is a Palo Alto device from the
environment's `network_systems`: its hostname (for example `pa-fw-01`) is the
CSV `Device Name` and the record's `host.name`, and its 12-digit hardware
serial is the CSV `Serial Number`. See [Environment](../environment.md#appliance-generators).
Constructed directly in Go without an appliance, the generator describes one
`PA-VM` (also its `host.name`) with a VM-Series serial (15 hex digits) for its
lifetime.

Each line starts with PAN-OS's default BSD syslog header: an RFC 3164
timestamp (space-padded day, `Sep  2 09:15:04`) and the firewall's FQDN,
since the Syslog HOSTNAME Format defaults to FQDN (`pa-fw-01.blitz.local`,
or `PA-VM` without an environment device). The header's timestamp is the
record's `Generated Time`. Through the syslog output the line keeps that
header, with only a `<PRI>` added in front (see
[Syslog output](../output/syslog.md)). The record's severity, and so the
`<PRI>` severity, is the one the firewall sends. THREAT and SYSTEM records map
`informational` to Info, `low` to Notice, `medium` to Warning, `high` to Error,
and `critical` to Critical. Every other log type is Info.

## Example Log (TRAFFIC, from the capture)

```
Oct  7 07:44:37 pa-fw-01.blitz.local 1,2026/10/07 07:44:37,0A1B2C3D4E5F607,TRAFFIC,end,2816,2026/10/07 07:44:37,10.250.2.20,104.20.23.154,10.250.1.10,104.20.23.154,allow-out,,,web-browsing,vsys1,trust,untrust,ethernet1/2,ethernet1/1,fwd-all,2026/10/07 07:44:37,1,1,60632,80,9149,80,0x40001c,tcp,allow,2976,553,2423,15,2026/10/07 07:44:15,2,computer-and-internet-info,,7693931910711476227,0x0,10.0.0.0-10.255.255.255,United States,,7,8,tcp-fin,0,0,0,0,,pa-fw-01,from-policy,,,0,,0,,N/A,0,0,0,0,9817d833-49ce-457f-ae1d-cf1270cb433a,0,0,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,2026-10-07T07:44:37.327-07:00,,,internet-utility,general-internet,browser-based,4,"used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use",,web-browsing,no,no,0,NonProxyTraffic,
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `palo-alto` to use this generator. |
| `generator.palo-alto.workers` | `--generator-palo-alto-workers` | `BLITZ_GENERATOR_PALO_ALTO_WORKERS` | `1` | Number of Palo Alto generator workers (must be ≥ 1) |
| `generator.palo-alto.rate` | `--generator-palo-alto-rate` | `BLITZ_GENERATOR_PALO_ALTO_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: palo-alto
  palo-alto:
    workers: 2
    rate: 500ms
```

## Metrics

The Palo Alto generator exposes the following metrics:

- **`blitz.generator.entries`** (Counter): Total number of logs generated
- **`blitz.generator.active_workers`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write_errors`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics carry a `generator_type` attribute set to `paloalto`. See [Metrics](../metrics.md) for the exported names.


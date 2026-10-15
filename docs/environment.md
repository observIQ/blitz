# Simulated Identity Environment

Blitz can generate telemetry that describes a coherent, simulated fleet of hosts
rather than the single machine the process runs on. A `datagen.Environment` is a
cross-referenced graph of identities — a domain, networks, users, groups,
systems, and storage/network appliances — built deterministically from seeds.
When an environment is configured, every generator resolves a simulated host
from it and stamps that host's identity onto each emitted record.

This page covers the user-facing surface: how to configure the environment, how
generators are mapped to simulated hosts, and which resource attributes get
projected. For the low-level identity model (the identity hierarchy, seed
contract, hostname pools, appliance taxonomy), see
[datagen.md](datagen.md).

## Configuration

The environment is configured under the top-level `environment:` key. The whole
block is optional — an omitted `environment` yields a randomized default
environment, so records still carry a coherent simulated host without any
configuration.

```yaml
environment:
  # AD/DNS domain for the fleet. Default: blitz.local
  domain_name: corp.example.com

  # Per-identity-type determinism. Each field is optional; an omitted field
  # randomizes that identity type, while an explicit value — including 0 — is a
  # deterministic seed. Changing one seed only re-randomizes that slice of the
  # output.
  seed_config:
    shared: 42          # base seed mixed into every type unless that type is set
    systems: 100        # host identities (OS, hostname, interfaces, specs)
    users: 101
    groups: 102
    services: 103
    applications: 104
    networks: 105
    domains: 106
    storage_systems: 107
    network_systems: 108

  # How many of each identity type to generate. A zero (omitted) count uses the
  # datagen default shown below.
  counts:
    systems: 20         # machines (default 20)
    users: 50           # default 50
    groups: 10          # default 10
    networks: 4         # subnets (default 4)
    storage_systems: 2  # storage arrays (default 2)
    network_systems: 4  # network devices (default 4)
    domain_admins: 0    # exact Domain Admins membership; 0 = user-count-scaled default
```

Counts must not be negative; the config is rejected at load time otherwise.

## How generators map to hosts

Each generator resolves exactly one host from the environment, keyed by the
generator's component name (`hostmetrics`, `traces`, `apache`, `nginx`, `wel`,
…). The mapping is deterministic — the same component always resolves to the
same host for a given environment — so a component attributes every record it
emits to one consistent machine, and distinct components spread across the
fleet. Several entries of one type in a `generators:` list take successive
hosts (entry 0, entry 1, ...), so two `hostmetrics` entries describe two
machines while the environment has enough eligible ones.

### Host-limited generators

A generator is only ever handed a host that could emit its telemetry: the right
OS and the right role. Selection stays deterministic per environment seed: the
component name picks among the eligible hosts only.

| Generator      | Eligible hosts                  | Why                                             |
|----------------|---------------------------------|-------------------------------------------------|
| `wel`          | Windows; role per `role` (unset: DC, server, or workstation) | Windows Event Log exists only on Windows; the event pool follows the host's role |
| `kubernetes`   | Linux servers                   | CRI-O container logs come from Linux nodes      |
| `hostmetrics`  | the configured `os` (unset: linux), any role | it emits that OS's metric set       |
| `apache-common`, `apache-combined`, `apache-error`, `nginx`, `postgres`, `json`, `fix`, `traces` | servers that run applications, any OS | server software never runs on a router, workstation, or DC, or on a server whose OS runs no applications |
| `okta`, `filegen` | any                          | the source is not host-bound                    |
| `palo-alto`    | a Palo Alto device from `network_systems` | it describes a firewall, not a host (see below) |

Entries of the same generator type get their own hosts, even when their
requirements differ (`wel` with any role and `wel` `role: dc`). When the random
fleet has fewer eligible hosts than the entries need (no domain controller for a `wel`
`role: dc` entry, one macOS host for two `hostmetrics` `os: macos` entries),
the environment adds the rest, deterministically per seed and after the drawn
hosts, so the drawn fleet is unchanged and every entry describes a real
environment host of its own.

### Appliance generators

A generator that describes a network appliance (`palo-alto`) takes a device
from the environment's `network_systems` instead of a host. Every device has a
hostname in the form `<vendor>-<role>-NN` (`pa-fw-01`, `f5-lb-01`,
`cisco-sw-02`), unique within the fleet, and an FQDN in the environment's
domain (`pa-fw-01.blitz.local`) that a device's syslog header carries. Each
appliance generator entry gets its own device: when the random fleet holds
fewer devices of a vendor than the entries need, the environment appends the
rest, deterministically per seed and without moving any drawn device, so every
entry describes a real device of its own. PAN-OS devices run an 11.0.x release,
matching the PAN-OS 11.0 log format the `palo-alto` generator emits.
The device's hostname is the generator's one name; its resource also carries
`device.manufacturer`, `device.model.name`, `os.name`/`os.version` (the
appliance OS, such as PAN-OS), and its management interface as `host.ip` and
`host.mac`.

A generator resolves one hostname at construction and uses it everywhere: in
`host.name` and in any hostname it writes inside the message (for example the
`wel` `<Computer>` element or the `json` `host` field), so they never disagree.

The hostname is resolved in this order:

1. An explicit hostname setting (`wel` `computer`, `hostmetrics` and `traces`
   `hostname`). It replaces only the resolved host's name; the host's OS, arch,
   tier and other identity attributes stay.
2. The environment host's hostname.
3. With no environment, the generator's own fallback (`BLITZ-WEL` for `wel`, a
   generated name for `hostmetrics` and `traces`), or the running process's
   `os.Hostname()` (or `blitz` if that fails) for generators with no hostname
   setting.

The hostname settings default to empty, so the environment host is used unless
you set one.

`wel` also follows the host's domain membership, as real Windows does. A
domain-joined host writes its FQDN in `<Computer>` (`DC01.contoso.local`) and
its NetBIOS domain as the account domain (`CONTOSO`), while payload fields keep
the short name. A workgroup host writes its bare name and `WORKGROUP`. A set
`domain` wins over the host's domain.

> Finer per-worker granularity — one distinct host per worker within a single
> generator — is a planned opt-in. Today the granularity is one host per
> generator component.

## Projected resource attributes

With an environment configured, every record's `Metadata.Resource` carries the
resolved host's identity, following OpenTelemetry semantic conventions:

| Attribute                     | Source                                                     |
|-------------------------------|------------------------------------------------------------|
| `host.name`                   | the system's hostname                                      |
| `host.id`                     | OS-appropriate machine id (machine-id / GUID / UUID)       |
| `host.arch`                   | CPU architecture (semconv `host.arch` value)               |
| `os.type`                     | semconv value — macOS is reported as `darwin`              |
| `os.name`                     | e.g. `Ubuntu`, `Microsoft Windows Server 2022`, `macOS`; a domain controller always runs Windows Server |
| `os.version`                  | e.g. `22.04.5`, `10.0.20348.2762`, `14.6.1`                |
| `os.build_id`                 | kernel release / Windows build number / macOS build        |
| `os.description`              | e.g. `Ubuntu 22.04.5 LTS`                                   |
| `host.ip`                     | `[]string` of the host's interface IPv4 + IPv6 addresses   |
| `host.mac`                    | `[]string` of the host's interface MAC addresses           |
| `deployment.environment.name` | deployment tier — `production` / `staging` / `test` / `development` |
| `telemetry.source`            | the generating module (`apache`, `nginx`, …)               |

Empty identity fields are omitted rather than emitted blank. Per-generator
constants (`apache.format`, `wel.channel`, `json.type`, …) are layered on top of
this set — see [embed.md](embed.md#resource-attributes).

`host.image.*` (VM/OS image provenance) is a reserved framework hook. It is not
emitted today; a future cloud-identity source will populate it.

## OS → hostname convention

Hostnames are drawn from mythology pools, chosen by OS and role so a hostname
hints at what the machine is:

| Pool     | Mythology | Convention                        |
|----------|-----------|-----------------------------------|
| Norse    | Norse     | Linux servers                     |
| Roman    | Roman     | Windows servers and workstations  |
| Greek    | Greek     | Domain Controllers                |
| Egyptian | Egyptian  | Network appliances / routers      |
| Celtic   | Celtic    | macOS / developer workstations    |

Windows and Domain Controller hostnames render in the uppercase NetBIOS style;
Linux and macOS hosts use the lowercase style.

## Determinism

Given the same `seed_config` (and a fixed time anchor for time-dependent fields
like certificate validity windows), the generated environment is reproducible
run to run. Omit the seeds for a fresh randomized fleet each run; pin them for
reproducible fixtures, demos, and snapshot tests.

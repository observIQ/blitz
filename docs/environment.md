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
fleet.

When no environment is available, generators fall back to the running process's
`os.Hostname()` (or `blitz` if that fails), exactly as before — so nothing about
the default output shape changes when the environment is absent.

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
| `os.name`                     | e.g. `Ubuntu`, `Microsoft Windows Server 2022`, `macOS`    |
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

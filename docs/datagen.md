# Data Generation Package (`internal/datagen`)

The `datagen` package provides reusable, deterministic data generation for synthetic telemetry. It generates coherent identity hierarchies — domains, systems, users, groups, services, and applications — from configurable seeds, ensuring reproducible test environments.

## Overview

Rather than each generator maintaining its own embedded data pools (hostnames, IPs, HTTP methods, etc.), `datagen` provides shared pools and identity types that generators can import. This eliminates duplication and ensures consistency across generators.

## Identity Hierarchy

A single `Environment` is generated at startup and contains all identities with proper cross-references:

```
DomainIdentity (1 per run)
├── CertAuthority (enterprise CA for the domain)
├── NetworkIdentity[] (subnets/VLANs the domain spans)
├── GroupIdentity[] (AD groups, referencing UserIdentities as members)
├── UserIdentity[] (domain users)
└── SystemIdentity[] (machines in the domain)
    ├── CertInfo (TLS cert issued by domain CA)
    ├── ServiceIdentity[] (services running on this system)
    ├── ApplicationIdentity[] (software installed on this system)
    └── NetworkInterface[] (NIC bound to a NetworkIdentity subnet)
```

## Seed Configuration

Deterministic generation is controlled by seeds. A shared seed applies to all identity types unless overridden per type.

### Seed value contract

- **Negative values (e.g. `-1`)** → randomize at startup. The randomized seed is logged so a run can still be reproduced after the fact.
- **`0` or any positive integer** → used verbatim as a deterministic seed. `0` is a legitimate seed value, not a sentinel.
- **Omitted YAML key** → resolves to the field's default. The viper/config layer defaults each seed to `-1` (randomize), so leaving a key out is equivalent to writing `-1`.

The Go API exposes `datagen.NewSeedConfig()` which returns a `*SeedConfig` with every field initialized to `-1`. Production code should obtain SeedConfig values via that constructor or via the config-loading path (which applies the `-1` defaults). A bare `&datagen.SeedConfig{}` literal yields zero-valued fields, which the contract treats as deterministic seed 0 across the board — useful in tests, but not what you want for a randomized run.

```yaml
datagen:
  seed: 12345                    # shared seed; omit or set -1 for random
  seeds:                         # per-type overrides (optional)
    systems: 99999
    users: 88888
    # groups, services, applications, networks, domains fall back to shared
    # when set to -1 or omitted; setting any of them to 0 or a positive int
    # gives that type its own deterministic seed.
```

CLI/env overrides:
- `--datagen-seed` / `BLITZ_DATAGEN_SEED`
- `--datagen-seed-systems` / `BLITZ_DATAGEN_SEED_SYSTEMS`
- `--datagen-seed-users` / `BLITZ_DATAGEN_SEED_USERS`

On startup, `Init()` logs all effective seeds for reproducibility:

```
INFO  datagen seeds initialized  {"shared": 12345, "systems": 99999, "users": 88888, ...}
```

## Pool[T] Type

The core building block is `Pool[T]`, a generic, read-only collection for random selection:

```go
p := datagen.NewPool("a", "b", "c")
item := p.Random(r)           // random single item
items := p.RandomN(r, 2)      // 2 unique random items
all := p.All()                 // copy of all items
merged := datagen.Merge(p1, p2) // combine pools
```

## Mythology Hostnames

Five mythology pools provide thematic hostname generation:

| OS / Role | Pantheon | Example |
|-----------|----------|---------|
| Linux servers | Norse | `thor-web-01` |
| Windows servers | Roman | `MARS-WEB01` |
| Domain Controllers | Greek | `ZEUS-DC01` |
| macOS / dev workstations | Celtic | `brigid-app-03` |
| Network appliances | Egyptian | `ra-proxy-01` |

This mapping is a convention, not enforced. Each caller explicitly passes the pool(s) it wants.

### Hostname Styles

- **StyleLinux**: `{myth}-{role}-{nn}` (lowercase)
- **StyleWindows**: `{MYTH}-{ROLE}{NN}` (uppercase)
- **StyleDC**: `{MYTH}-DC{NN}` (uppercase)

## Shared Data Pools

### HTTP (`http.go`)

- `Methods` — GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
- `Protocols` — HTTP/1.0, HTTP/1.1, HTTP/2.0
- `Status2xx`, `Status3xx`, `Status4xx`, `Status5xx` — status code pools by class
- `RandomStatusCode(r)` — weighted random (70% 2xx, 5% 3xx, 15% 4xx, 10% 5xx)
- `APIPaths` — common API and web paths
- `RefererDomains` — common referer domains

### Networks (`networks.go`)

- `RandomIPv4(r)`, `RandomPrivateIPv4(r)`, `RandomPublicIPv4(r)`, `RandomIPv6(r)`
- `RandomMAC(r)`, `RandomIPInCIDR(r, cidr)`
- `CommonPorts` — common network ports (22, 80, 443, 3306, etc.)
- `TCPUDPProtocols` — tcp, udp, icmp
- `ValidateCIDR(cidr) error`, `(*NetworkIdentity).Validate() error` — CIDR validation; see "NetworkIdentity CIDR contract" below.

#### NetworkIdentity CIDR contract

Blitz networks model "subnets with hosts" — broadcast domains where simulated systems live. The `CIDR` field on `NetworkIdentity` carries this constraint:

- **IPv4 only.** IPv6 support is tracked in PIPE-1001 and is not yet wired through `NetworkIdentity` or any of the random-IP utilities.
- **Prefix length must be /29 or shorter** (numerically smaller — i.e., the subnet must hold at least 8 total addresses / 6 usable hosts). `/30`, `/31`, and `/32` are explicitly **invalid** because they describe point-to-point router links (`/30` and `/31`) or single-host routes (`/32`), neither of which represents a host-bearing subnet in blitz's simulation.
- **Validation is the gate, not a runtime fallback.** Config-loading code that accepts user-supplied CIDRs MUST call `ValidateCIDR(cidr)` (or `NetworkIdentity.Validate()`) and **fail the entire config load** on error — blitz refuses to start rather than silently substituting a default. PIPE-1002 tracks wiring this into the config-load path; until then, validation calls live wherever they're needed and the function is exported for that purpose.
- **`RandomIPInCIDR` has a soft fallback as defense in depth.** If reached with an invalid CIDR (unparseable, non-IPv4, or prefix > /29), it returns a `RandomIPv4(r)` value rather than panicking. Reaching that fallback indicates a missing validation call upstream, not a behavior to depend on.

#### `RandomPublicIPv4` reserved-block coverage

`RandomPublicIPv4` rejects any candidate that lands in a known IETF-reserved IPv4 block, attributed by RFC for traceability:

| RFC | Range(s) | Purpose |
|-----|----------|---------|
| [RFC 1112](https://datatracker.ietf.org/doc/html/rfc1112) | 240.0.0.0/4 | Class E reserved (also excluded by the first-octet cap of 224). |
| [RFC 1122](https://datatracker.ietf.org/doc/html/rfc1122) | 0.0.0.0/8, 127.0.0.0/8 | "this network" + loopback. |
| [RFC 1918](https://datatracker.ietf.org/doc/html/rfc1918) | 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 | Private-use IPv4. |
| [RFC 2544](https://datatracker.ietf.org/doc/html/rfc2544) | 198.18.0.0/15 | Benchmark testing. |
| [RFC 3927](https://datatracker.ietf.org/doc/html/rfc3927) | 169.254.0.0/16 | Link-local IPv4. |
| [RFC 5736](https://datatracker.ietf.org/doc/html/rfc5736) | 192.0.0.0/24 | IETF protocol assignments. |
| [RFC 5737](https://datatracker.ietf.org/doc/html/rfc5737) | 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 | Documentation TEST-NET-1/2/3. |
| [RFC 6598](https://datatracker.ietf.org/doc/html/rfc6598) | 100.64.0.0/10 | CGNAT shared address space. |

[RFC 6890](https://datatracker.ietf.org/doc/html/rfc6890) is the umbrella special-purpose-address registry; the table above cites originating RFCs rather than 6890 directly. New reserved blocks are added by appending entries to `reservedIPv4Blocks` in `internal/datagen/networks.go`. The same paradigm will apply to IPv6 random-public emission once PIPE-1001 lands.

### Windows (`windows.go`)

- `WindowsServices` — real Windows service names (wuauserv, BITS, NTDS, etc.)
- `WindowsServiceDisplayNames` — display names matching service names
- `WindowsProcessPaths` — common process paths (svchost.exe, lsass.exe, etc.)
- `WindowsRegistryPaths` — common registry paths for object access events
- `WindowsTaskPaths` — common scheduled task paths

### Names (`usernames.go`)

- `FirstNames` — 50 common first names
- `Surnames` — 50 common surnames
- `Departments` — Engineering, Sales, IT, etc.
- `Titles` — job titles

## Usage Example

```go
import "github.com/observiq/blitz/internal/datagen"

// Generate a full environment
seeds := datagen.NewSeedConfig() // all fields default to -1 (randomize)
seeds.Shared = 12345             // override with a deterministic shared seed
env := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{
    DomainName:  "contoso.com",
    SystemCount: 20,
    UserCount:   50,
})

// Use shared pools directly
r := rand.New(rand.NewSource(42))
method := datagen.Methods.Random(r)
ip := datagen.RandomIPv4(r)
status := datagen.RandomStatusCode(r)
hostname := datagen.GenerateHostname(r, datagen.StyleLinux, datagen.NorseNames)
```

## How Generators Consume datagen

Generators replace their inline `[]string{...}` pool literals with `datagen` pool calls:

```go
// Before (embedded in generator)
methods := []string{"GET", "POST", "PUT", "DELETE"}
method := methods[r.Intn(len(methods))]

// After (using datagen)
method := datagen.Methods.Random(r)
```

For generators that need the full identity hierarchy (e.g., Windows Event Log generator), they receive the `*Environment` and draw from it directly:

```go
user := env.Users[r.Intn(len(env.Users))]
system := env.Systems[r.Intn(len(env.Systems))]
```

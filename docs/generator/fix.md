# FIX Generator

Emits synthetic FIX (Financial Information eXchange) protocol messages
at a configurable rate. Designed for observability-platform load and
realism testing of financial-trading telemetry pipelines.

## Scope

- **Versions**: FIX 4.2, FIX 4.4, FIX 5.0 SP2 (with FIXT.1.1 session
  layer). 4.0 / 4.1 / 4.3 are out of scope.
- **Asset coverage**: every FIX SecurityType value (tag 167) is emit-
  ready across 10 asset categories:

  | Category | SecurityTypes |
  |---|---|
  | Cash equities & equity-like | CS, PFD, ETF, MF, ADR, WAR, RGT |
  | FX | FOR, FXFWD, FXSWAP, FXNDF |
  | Listed futures | FUT |
  | Listed options | OPT |
  | Government fixed income | TBILL, TNOTE, TBOND, TIPS, TINT |
  | Corporate / credit fixed income | CORP, CB, MUNI, MUNIFIDC, GO, REV |
  | Structured products | ABS, MBS, TMBS, CMBS, CDO |
  | OTC derivatives | IRS, CDS, BSWAP, VARSWAP, TRSWAP, XCS |
  | Repos | REPO, REVREPO, HREPO |
  | Money market | CD, CP, BA, BN |

- **Message types**: NewOrderSingle (D), ExecutionReport (8),
  OrderCancelRequest (F), OrderCancelReplaceRequest (G),
  OrderStatusRequest (H), BusinessMessageReject (j) — plus session
  layer (Logon A, Heartbeat 0, Logout 5, ResendRequest 2, SequenceReset
  4, TestRequest 1, Reject 3).

## Determinism from seed

The generator obeys blitz's deterministic-from-seed contract:

- `Config.Seed >= 0` → deterministic. Worker N uses `Seed+N` as its
  RNG seed. Same seed + same `Config.Start` → byte-identical FIX output
  stream.
- `Config.Seed < 0` → a random seed is picked once at `Start()` and
  logged (the `seed` field of the start log line), so rerunning with that
  value and the same `Config.Start` reproduces the run.

A CLI run cannot set `Config.Start`, so its trade date is the day it runs.
A rerun with the same seed on another day makes the same draws, but over
the instruments listed that day (futures and option months, outstanding
Treasuries, CMBS vintages), so its symbols and security IDs differ.

The contract is locked by `TestGoldenOutputDeterministicFromSeed`.

Timestamps never come from the seed. SendingTime (52) and TransactTime
(60) are FIX `UTCTimestamp` values (`YYYYMMDD-HH:MM:SS.sss`). With
`Config.Start` set, they advance from it by `Rate` per sequence number,
which is what makes a pinned run byte-identical. Without it (a live run),
each message is stamped with the wall clock when it is sent, so a run that
falls behind its rate never drifts behind real time. Every date a message
carries is a `LocalMktDate` (`YYYYMMDD`) derived from that message's trade
date:

- FX: spot (`167=FOR`) settles cash T+0 (`63=1`), next day T+1 (`63=2`),
  or regular at the pair's spot lag (`63=0`; T+1 for USD/CAD, T+2
  otherwise). Forwards, swaps, and NDFs settle `63=6` at spot plus the
  pair's tenor (1M = one calendar month, 1W = seven days). Prices sit near
  each pair's reference level at its market precision (3 decimals for JPY
  pairs, 2 for KRW, 5 for most majors). An NDF settles in USD, as
  non-deliverable forwards do.
- Repos: the near leg settles on the trade date; the far leg closes the
  next business day (overnight) or about one month later (term).
- Money market: issued on the trade date (the next business day when it falls
  on a weekend), maturing at the tenor its symbol names (`GE-CP-30D` 30
  days later, rolled to a business day).
- Corporate, structured, money-market, and equity CUSIPs carry a valid check
  digit. Synthetic ones keep a six-character issuer number with a
  private-use issue number (90-99, 9A-9Y), so they name no real security.
  Listed equities, ETFs, funds, and preferreds carry their real CUSIPs;
  warrants and rights use Nasdaq test symbols (ZVZZT and similar).
- Futures and equities: prices sit near each contract's or symbol's
  reference level, futures on a whole tick of the contract (`0.25` for ES,
  `0.0025` for corn) and equities in whole cents.
- FX messages declare the currency pair as an ISO Currency Code (`22=6`).
- OTC swaps, CDS included, carry synthetic ISINs with the `EZ` prefix the
  derivatives ISIN issuer uses (`22=4`), check digit included; FIX 4.4 has no
  Markit RED SecurityIDSource. Repos carry tri-party identifiers declared as
  Clearing House / Clearing Organization (`22=H`), since a repo has no ISIN.
  FIX 4.2 has no such source, so 4.2 repos send no SecurityID (48) or
  IDSource (22).
- OTC swaps carry an `UnderlyingSecurityID` (309) only for an underlier with
  an identifier (the AAPL total-return swap, with `305=1`); rate indices, FX
  pairs, and CDS reference entities send `UnderlyingSymbol` (311) alone. FIX
  4.4 has no field for a swap's day count or a CDS restructuring clause, so
  neither is sent.
- Treasuries: each note, bond, TIPS, and STRIPS row carries its latest real
  issue from TreasuryDirect, and while that issue is outstanding on the trade
  date a message uses its real CUSIP, coupon, and issue and maturity dates.
  Otherwise, and always for bills, the message describes a synthetic issue:
  a CUSIP with the Treasury prefix, an issue number CUSIP reserves for
  private use (90-99, 9A-9Y), and a valid check digit, so it names no real
  security, outstanding on the trade date, on the term's real maturity
  calendar. Bills run their week count and are issued and mature on their
  issue weekday (Tuesday for 4-week bills, Thursday otherwise). 2-, 5-, and
  7-year notes and their interest STRIPS mature at month-end; 3-year notes on
  the 15th; 10-year notes, 20- and 30-year bonds, and 30-year bond STRIPS on
  February, May, August, or November 15; 5-year TIPS on April or October 15;
  10-year TIPS on January or July 15; and 30-year TIPS on February 15. An
  issue is dated one term before maturity and settles that day, or the next
  weekday when it falls on a weekend; 20-year bonds settle at the end of the
  dated month (next weekday), and TIPS on its last weekday.
- Corporate and municipal bonds, structured products: outstanding on the
  trade date, with issue and maturity exactly the instrument's term apart,
  maturing on the 15th. A year in a symbol follows the dates: a STRIP or
  corporate bond names its maturity year, a structured deal and its pool their
  issue (vintage) year.
- Swaps: mature their named tenor after spot (T+2, or the trade date for
  GBP SONIA, which starts the day it trades). CDS follow the ISDA
  semi-annual roll: a contract traded from 20 March to 19 September matures
  on 20 June, and one traded from 20 September to 19 March on 20 December,
  the tenor after that roll.
- Futures: one of the contract's next four listed months still trading on the
  trade date, by each exchange's last-trading-day rule (CME Group and Cboe
  contract specs, weekends-only calendar).
- Options: one of the expirations every class lists, each the month's third
  Friday. Stock and ETF options use the two near-term months or a January
  LEAPS expiration two years out (three from September expiration week, when
  the next LEAPS year lists). SPX, NDX, and RUT options use the three
  near-term months or the next three March-cycle quarterly months. VIX
  options use the three near-term expirations, each on the Wednesday 30 days
  before the following month's third Friday.

Settlement dates skip weekends but not market holidays.

## Configuration

YAML (standalone CLI):

```yaml
generator:
  type: fix
  fix:
    workers: 4
    rate: 10ms
    version: "4.4"            # "4.2", "4.4", or "5.0sp2"
    senderCompID: BLITZ       # base value; worker idx appended
    targetCompID: VENUE
    enabledCategories:        # omit / leave empty for all 10
      - equities
      - fx
      - options
    seed: 42                  # >=0 deterministic, <0 randomize
```

Every key is also settable by CLI flag and environment variable:

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Set to `fix` to use this generator. |
| `generator.fix.workers` | `--generator-fix-workers` | `BLITZ_GENERATOR_FIX_WORKERS` | `1` | Number of FIX generator workers (must be ≥ 1) |
| `generator.fix.rate` | `--generator-fix-rate` | `BLITZ_GENERATOR_FIX_RATE` | `1s` | Per-worker emission interval (one message per Rate) |
| `generator.fix.version` | `--generator-fix-version` | `BLITZ_GENERATOR_FIX_VERSION` | `FIX.4.4` | FIX protocol version: `4.2`, `4.4`, or `5.0sp2` (the `FIX.x.y` forms are also accepted) |
| `generator.fix.senderCompID` | `--generator-fix-sendercompid` | `BLITZ_GENERATOR_FIX_SENDERCOMPID` | `BLITZ` | Base SenderCompID (tag 49); the worker index is appended |
| `generator.fix.targetCompID` | `--generator-fix-targetcompid` | `BLITZ_GENERATOR_FIX_TARGETCOMPID` | `VENUE` | TargetCompID (tag 56) |
| `generator.fix.enabledCategories` | `--generator-fix-enabledcategories` | `BLITZ_GENERATOR_FIX_ENABLEDCATEGORIES` | (all) | Asset categories to emit, comma-separated; empty = all 10 |
| `generator.fix.seed` | `--generator-fix-seed` | `BLITZ_GENERATOR_FIX_SEED` | `-1` | RNG seed: negative randomizes, `0`+ is deterministic |

The defaults are the `DefaultConfig()` values and apply when a key is left
out of the `generator:` block. How `generators:` list entries take defaults
is described in [Configuration](../configuration.md).

Programmatic (`fix.Config` in Go):

```go
type Config struct {
    Workers            int                       // parallel emission workers
    Rate               time.Duration             // 1 message per Rate per worker
    Version            catalog.Version           // V42, V44, V50SP2
    SenderCompID       string                    // tag 49 base; worker idx appended
    TargetCompID       string                    // tag 56
    EnabledCategories  []catalog.AssetCategory   // empty = all 10
    Seed               int64                     // negative randomizes
    Start              time.Time                 // timestamp anchor; zero = now
}
```

`DefaultConfig()` returns: 1 worker, 1 message/sec, FIX 4.4,
`BLITZ`/`VENUE` CompIDs, all categories, seed -1.

## Architecture

```
generator/fix/
├── fix.go                       — top-level FIXGenerator + workers
├── state/                       — Session + per-category order books
└── catalog/
    ├── version.go               — Version enum + BeginString + ApplVerID
    ├── asset_class.go           — AssetCategory + SecurityType + Category()
    ├── field.go                 — Field, FieldGenerator, GenerateCtx
    ├── framing.go               — SOH framing + BodyLength + CheckSum
    ├── message.go               — MessageDefinition + MessageKey
    ├── registry.go              — global Registry, Register, Get
    ├── v42/                     — FIX 4.2 deltas (mirror + ExecType wrap)
    ├── v50sp2/                  — FIX 5.0 SP2 (mirror + ApplVerID inject)
    └── v44/
        ├── session/             — Logon, Heartbeat, Logout, ...
        ├── app/                 — NewOrderSingle, ExecutionReport, ... (asset-agnostic skeletons)
        ├── equities/            — CS/PFD/ETF/MF/ADR/WAR/RGT
        ├── fx/                  — FOR/FXFWD/FXSWAP/FXNDF
        ├── futures/             — FUT
        ├── options/             — OPT
        ├── govbonds/            — TBILL/TNOTE/TBOND/TIPS/TINT
        ├── corpbonds/           — CORP/CB/MUNI/MUNIFIDC/GO/REV
        ├── structured/          — ABS/MBS/TMBS/CMBS/CDO
        ├── otcderivs/           — IRS/CDS/BSWAP/VARSWAP/TRSWAP/XCS
        ├── repos/               — REPO/REVREPO/HREPO
        └── moneymarket/         — CD/CP/BA/BN
```

## v1 limitations (deferred to follow-up tickets)

- **Repeating groups**: multi-leg combos (NoLegs / LegSymbol), multiple
  underlyings (NoUnderlyings / UnderlyingSymbol), CDO tranche
  attachment points are emitted as flat representative fields, not as
  proper FIX repeating-group structures.
- **OTC derivatives leg modeling**: PayLeg/ReceiveLeg specs for IRS
  emit a flat coupon rate; full two-leg modeling deferred.
- **Real-time market-data messages** (W MarketDataSnapshot,
  X MarketDataIncrementalRefresh) — out of scope; this generator
  focuses on the order-flow side.

## Example use

The generator implements `embed.ProducerModule` and emits records into
an `embed.LogConsumer`. Construction goes through `dispatch.ForEmbed`
when running embedded, or `cmd/blitz`'s output adapter when running
standalone.

```go
import (
    "context"
    "time"

    "go.uber.org/zap"

    "github.com/observiq/blitz/embed"
    "github.com/observiq/blitz/generator/fix"
    "github.com/observiq/blitz/generator/fix/catalog"
)

g, err := fix.New(zap.NewNop(), fix.Config{
    Workers: 4,
    Rate:    10 * time.Millisecond,
    Version: catalog.V44,
    Seed:    42, // deterministic
}, myConsumer) // any embed.LogConsumer
if err != nil { /* ... */ }

if err := g.Start(context.Background()); err != nil { /* ... */ }
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = g.Stop(ctx)
}()
```

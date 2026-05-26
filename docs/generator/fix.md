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
  RNG seed. Same seed + same start time → byte-identical FIX output
  stream.
- `Config.Seed < 0` → randomize per worker (use `time.Now().UnixNano()`).

The contract is locked by `TestGoldenOutputDeterministicFromSeed`.

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

- **Cross-tag instrument coherence**: within one message, fields like
  Symbol (55), SecurityID (48), SecurityType (167), and CFICode (461)
  are independently sampled from the per-category instrument table.
  Wire is well-FORMED but cross-tag semantics may not refer to the
  same instrument. The StateTracker has the hooks needed to pin
  per-message selection — wiring this in is a follow-up.
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

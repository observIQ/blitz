package catalog

import (
	"math/rand"
	"strconv"
	"time"
)

// Tag is a FIX tag number (e.g. 35 for MsgType, 49 for SenderCompID).
type Tag int

// Field is one tag/value pair on the wire. Value is the textual form
// that will be emitted; FIX is a text-on-the-wire protocol with one
// fixed encoding for each tag.
type Field struct {
	Tag   Tag
	Value string
}

// FieldGenerator produces a Field given the supplied RNG and
// build-time context. Generators MUST source ALL randomness from the
// provided *rand.Rand — never from rand.Intn or rand.New(...) inline —
// so the FIX generator's "same seed → identical output" guarantee
// holds.
type FieldGenerator func(r *rand.Rand, ctx *GenerateCtx) Field

// GenerateCtx carries shared inputs available to every FieldGenerator
// during a message build. Fields here are read-only from a generator's
// perspective; mutation is the StateTracker's responsibility and lives
// behind its own locking.
type GenerateCtx struct {
	// Version is the FIX protocol version being emitted.
	Version Version
	// AssetCategory identifies which per-category modeling code is active.
	AssetCategory AssetCategory
	// SenderCompID is the FIX SenderCompID (tag 49) for this session.
	SenderCompID string
	// TargetCompID is the FIX TargetCompID (tag 56) for this session.
	TargetCompID string
	// SeqNum is the outgoing sequence number for the message under
	// construction (tag 34, MsgSeqNum).
	SeqNum int
	// SendingTime is the value to emit for tag 52, SendingTime. The
	// caller is responsible for choosing this so it stays consistent
	// across a build session — never read from time.Now() inside a
	// generator.
	SendingTime string
	// Memo is per-message scratch space for FieldGenerators that need
	// to share derived state across multiple fields within one message
	// — e.g. an instrument pick that must stay consistent across
	// Symbol/SecurityType/SecurityID/price/etc. so the message is
	// internally coherent. Lazily initialized by the first writer in
	// each message; keys are conventionally typed values defined by
	// the package using them.
	Memo map[any]any
	// Now is SendingTime as a time.Time, the anchor for every date a
	// message carries (trade date, settlement, issue, maturity). Zero
	// reads as the Unix epoch so bare test contexts stay deterministic.
	Now time.Time
}

// TradeDate is the UTC calendar date of Now, the T in T+N settlement.
func (c *GenerateCtx) TradeDate() time.Time {
	y, m, d := c.Now.UTC().Date()
	if c.Now.IsZero() {
		y, m, d = time.Unix(0, 0).UTC().Date()
	}
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddBusinessDays returns d moved forward n weekdays, skipping Saturday
// and Sunday.
// Weekends only, no market holiday calendars; add per-currency
// calendars if settlement-date accuracy around holidays matters.
func AddBusinessDays(d time.Time, n int) time.Time {
	for n > 0 {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			n--
		}
	}
	return d
}

// NextBusinessDay returns d, or the first weekday after it when d falls
// on a weekend.
func NextBusinessDay(d time.Time) time.Time {
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// orderQtyKey is the GenerateCtx.Memo key for the message's OrderQty.
type orderQtyKey struct{}

// OrderQty returns the message's OrderQty value, drawing it once with draw
// and memoizing it so a new order's LeavesQty can repeat it: with CumQty 0,
// FIX requires LeavesQty = OrderQty.
func OrderQty(r *rand.Rand, ctx *GenerateCtx, draw func(*rand.Rand) string) string {
	if v, ok := ctx.Memo[orderQtyKey{}]; ok {
		return v.(string)
	}
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	v := draw(r)
	ctx.Memo[orderQtyKey{}] = v
	return v
}

// LiteralField returns a FieldGenerator that always emits the given
// tag with the given fixed string value. Useful for header fields like
// MsgType where the wire form is constant for a message definition.
func LiteralField(tag Tag, value string) FieldGenerator {
	return func(_ *rand.Rand, _ *GenerateCtx) Field {
		return Field{Tag: tag, Value: value}
	}
}

// IntField returns a FieldGenerator that emits a fixed-int tag/value
// pair. The value is rendered via strconv.Itoa.
func IntField(tag Tag, value int) FieldGenerator {
	return func(_ *rand.Rand, _ *GenerateCtx) Field {
		return Field{Tag: tag, Value: strconv.Itoa(value)}
	}
}

// ThirdFriday is the third Friday of the month that is months after the trade
// date's month: the standard monthly listed-option expiration.
func ThirdFriday(trade time.Time, months int) time.Time {
	first := time.Date(trade.Year(), trade.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	offset := (int(time.Friday) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+14)
}

// AddTenor returns spot plus a money-market tenor under the ISDA conventions:
// a day or week tenor rolls following; a month tenor keeps the end-of-month
// rule (spot on its month's last business day lands on the target month's
// last business day), never overflows into the next month, and rolls
// modified-following (forward, or back when forward would change month).
func AddTenor(spot time.Time, months, days int) time.Time {
	if months == 0 {
		return NextBusinessDay(spot.AddDate(0, 0, days))
	}
	y, m, _ := spot.Date()
	target := time.Date(y, m+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	if spot.Equal(lastBusinessDay(spot)) {
		return lastBusinessDay(target)
	}
	last := target.AddDate(0, 1, -1).Day()
	d := time.Date(target.Year(), target.Month(), min(spot.Day(), last), 0, 0, 0, 0, time.UTC)
	if f := NextBusinessDay(d); f.Month() == d.Month() {
		return f
	}
	return lastBusinessDay(d)
}

// lastBusinessDay is the last weekday of d's month.
func lastBusinessDay(d time.Time) time.Time {
	l := time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	for l.Weekday() == time.Saturday || l.Weekday() == time.Sunday {
		l = l.AddDate(0, 0, -1)
	}
	return l
}

// BondDates draws the issue and maturity dates of a bond with a term of
// months that is outstanding on trade: it matures on the 15th of a month
// 1 to months-1 months after trade's month and was issued exactly months
// earlier, so trade falls strictly between them.
func BondDates(r *rand.Rand, trade time.Time, months int) (issue, maturity time.Time) {
	remaining := 1 + r.Intn(max(months-1, 1)) // #nosec G404 -- seeded *rand.Rand
	maturity = time.Date(trade.Year(), trade.Month()+time.Month(remaining), 15, 0, 0, 0, 0, time.UTC)
	return maturity.AddDate(0, -months, 0), maturity
}

// BillDates draws the issue and maturity dates of a Treasury bill of weeks
// that is outstanding on trade: bills are issued and mature on their issue
// weekday (Tuesday for 4-week bills, Thursday for 13-, 26-, and 52-week), so
// it matures on one of the next weeks-1 such days after trade and was issued
// exactly weeks earlier, strictly before trade.
func BillDates(r *rand.Rand, trade time.Time, weeks int, day time.Weekday) (issue, maturity time.Time) {
	next := trade.AddDate(0, 0, 1)
	next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
	next = next.AddDate(0, 0, (int(day)-int(next.Weekday())+7)%7)
	maturity = next.AddDate(0, 0, 7*r.Intn(max(weeks-1, 1))) // #nosec G404 -- seeded *rand.Rand
	return maturity.AddDate(0, 0, -7*weeks), maturity
}

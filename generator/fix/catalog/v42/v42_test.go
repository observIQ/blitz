package v42

import (
	"math/rand"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func TestV42EntryExistsForEveryV44Entry(t *testing.T) {
	v44Count := 0
	v42Count := 0
	for _, def := range catalog.AllDefinitions() {
		switch def.Version {
		case catalog.V44:
			v44Count++
			// Make sure there's a matching V42 entry.
			if catalog.Get(catalog.MessageKey{
				Version: catalog.V42, MsgType: def.MsgType, AssetCategory: def.AssetCategory,
			}) == nil {
				t.Errorf("V44 entry (%s, %s) has no V42 mirror", def.MsgType, def.AssetCategory)
			}
		case catalog.V42:
			v42Count++
		}
	}
	if v42Count != v44Count {
		t.Errorf("V42 mirror count %d != V44 count %d", v42Count, v44Count)
	}
}

func TestV42ExecutionReportExecTypeIsLegacy(t *testing.T) {
	// Find any V42 ExecutionReport and verify ExecType emits "2" (Fill)
	// not "F" (Trade).
	for _, cat := range catalog.AllAssetCategories() {
		def := catalog.Get(catalog.MessageKey{
			Version: catalog.V42, MsgType: app.MsgTypeExecutionReport, AssetCategory: cat,
		})
		if def == nil {
			continue
		}
		r := rand.New(rand.NewSource(1))
		for _, g := range def.Fields {
			f := g(r, &catalog.GenerateCtx{Version: catalog.V42, AssetCategory: cat})
			if f.Tag == app.TagExecType {
				if f.Value == app.ExecTypeFill {
					t.Errorf("V42 %s ExecutionReport emitted V44 ExecType %q — expected legacy %q",
						cat, f.Value, ExecType42Fill)
				}
			}
		}
	}
}

func TestV42BeginString(t *testing.T) {
	if catalog.V42.BeginString() != "FIX.4.2" {
		t.Errorf("V42.BeginString() = %q, want FIX.4.2", catalog.V42.BeginString())
	}
}

func TestV42HasNoApplVerID(t *testing.T) {
	if catalog.V42.ApplVerID() != "" {
		t.Errorf("V42 must not carry ApplVerID, got %q", catalog.V42.ApplVerID())
	}
}

// TestV42IDSourceIsDefinedIn42 pins that every 4.2 message declares its
// SecurityID (48) with an IDSource (22) FIX 4.2 defines (1-9), and never
// sends 48 without 22.
func TestV42IDSourceIsDefinedIn42(t *testing.T) {
	for _, def := range catalog.AllDefinitions() {
		if def.Version != catalog.V42 {
			continue
		}
		for seed := int64(1); seed <= 20; seed++ {
			r := rand.New(rand.NewSource(seed)) // #nosec G404
			ctx := &catalog.GenerateCtx{Version: catalog.V42, AssetCategory: def.AssetCategory, Now: time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC)}
			var id, src string
			for _, g := range def.Fields {
				switch f := g(r, ctx); f.Tag {
				case 48:
					id = f.Value
				case 22:
					src = f.Value
				}
			}
			if src != "" && (len(src) != 1 || src[0] < '1' || src[0] > '9') {
				t.Errorf("%s %s seed %d: IDSource %q is not defined in FIX 4.2", def.AssetCategory, def.MsgType, seed, src)
			}
			if id != "" && src == "" {
				t.Errorf("%s %s seed %d: SecurityID %q sent without IDSource", def.AssetCategory, def.MsgType, seed, id)
			}
		}
	}
}

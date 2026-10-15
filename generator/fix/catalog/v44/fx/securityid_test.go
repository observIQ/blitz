package fx

import (
	"math/rand"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// TestSecurityIDSourceIsISOCurrencyCode pins that the currency-pair
// SecurityID is declared as ISO Currency Code (22=6); "G" is Common
// (Clearstream/Euroclear).
func TestSecurityIDSourceIsISOCurrencyCode(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryFX, Now: time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC)}
	r := rand.New(rand.NewSource(1)) // #nosec G404
	var src string
	for _, g := range def.Fields {
		if f := g(r, ctx); f.Tag == TagSecurityIDSrc {
			src = f.Value
		}
	}
	if src != "6" {
		t.Fatalf("SecurityIDSource = %q, want 6", src)
	}
}

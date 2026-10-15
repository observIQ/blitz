package repos

import (
	"math/rand"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// TestSecurityIDSourceIsClearingOrganization pins that repo SecurityIDs,
// which are tri-party identifiers rather than ISINs, are declared as
// Clearing House / Clearing Organization (22=H).
func TestSecurityIDSourceIsClearingOrganization(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryRepos, Now: time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC)}
	r := rand.New(rand.NewSource(1)) // #nosec G404
	var src string
	for _, g := range def.Fields {
		if f := g(r, ctx); f.Tag == TagSecurityIDSrc {
			src = f.Value
		}
	}
	if src != "H" {
		t.Fatalf("SecurityIDSource = %q, want H", src)
	}
}

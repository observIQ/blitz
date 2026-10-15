package otcderivs

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// TestOTCTagsMeanWhatFIX44Says pins FIX 4.4 tag meanings on every OTC
// message: UnderlyingSecurityID (309) appears only with
// UnderlyingSecurityIDSource (305) and is valid for it, and tags 478
// (CashDistribCurr) and 717 (SettlSessSubID) never appear, since FIX 4.4 has
// no field for a swap's day count or a CDS restructuring clause.
func TestOTCTagsMeanWhatFIX44Says(t *testing.T) {
	reregisterAll()
	sawUnderlyingID := false
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryOTCDerivs})
		for seed := int64(1); seed <= 300; seed++ {
			tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
			for _, banned := range []catalog.Tag{478, 717} {
				if v, ok := tags[banned]; ok {
					t.Errorf("%s seed=%d: carries tag %d=%q", mt, seed, banned, v)
				}
			}
			id, hasID := tags[309]
			src, hasSrc := tags[305]
			if hasID != hasSrc || (hasID && !catalog.ValidSecurityID(id, src)) {
				t.Errorf("%s seed=%d: 309=%q (present %v) with 305=%q (present %v)", mt, seed, id, hasID, src, hasSrc)
			}
			sawUnderlyingID = sawUnderlyingID || hasID
		}
	}
	if !sawUnderlyingID {
		t.Error("no message carried an UnderlyingSecurityID")
	}
}

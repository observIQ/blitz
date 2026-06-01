package catalog

import "testing"

func TestAllAssetCategoriesCovered(t *testing.T) {
	cats := AllAssetCategories()
	if len(cats) != 10 {
		t.Fatalf("AllAssetCategories() = %d, want 10", len(cats))
	}
	for _, c := range cats {
		if c.String() == "unknown" {
			t.Errorf("category %d missing String() label", c)
		}
	}
}

func TestEverySecurityTypeHasCategory(t *testing.T) {
	for _, st := range AllSecurityTypes() {
		cat := st.Category()
		if cat == AssetCategoryUnknown {
			t.Errorf("SecurityType %q has no AssetCategory mapping", st)
		}
	}
}

func TestSecurityTypesByCategoryPartitionsAllSecurityTypes(t *testing.T) {
	// Sum of per-category lists must equal AllSecurityTypes — every
	// SecurityType belongs to exactly one category, no orphans.
	totalByCategory := 0
	for _, cat := range AllAssetCategories() {
		members := SecurityTypesByCategory(cat)
		if len(members) == 0 {
			t.Errorf("category %s has zero SecurityTypes", cat)
		}
		totalByCategory += len(members)
		for _, st := range members {
			if st.Category() != cat {
				t.Errorf("SecurityType %q in %s's bucket but Category() = %s",
					st, cat, st.Category())
			}
		}
	}
	if totalByCategory != len(AllSecurityTypes()) {
		t.Errorf("category-summed count %d != AllSecurityTypes count %d",
			totalByCategory, len(AllSecurityTypes()))
	}
}

func TestSecurityTypesByCategoryUnknownIsNil(t *testing.T) {
	if got := SecurityTypesByCategory(AssetCategoryUnknown); got != nil {
		t.Errorf("SecurityTypesByCategory(Unknown) = %v, want nil", got)
	}
}

// TestExpectedSecurityTypeCount locks in v1 coverage at exactly the
// declared 42 SecurityType values. If this fails, the catalog has
// drifted from the PIPE-1022 ticket scope — update the scope OR the
// count, but the mismatch must not slip through review unnoticed.
func TestExpectedSecurityTypeCount(t *testing.T) {
	const expected = 42
	if got := len(AllSecurityTypes()); got != expected {
		t.Errorf("AllSecurityTypes() count = %d, want %d — scope drift", got, expected)
	}
}

package catalog

import (
	"sort"
	"sync"
)

// registry holds all registered Products keyed by Name. Per-product
// subpackages register at init (single-goroutine); the generator reads
// them on construction. Guarded by an RWMutex for safety.
var (
	mu       sync.RWMutex
	products = map[string]Product{}
)

// Register adds a Product to the global registry. Panics on a duplicate
// Name — that signals a programmer error in the product catalog, not a
// recoverable runtime condition.
func Register(p Product) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := products[p.Name]; exists {
		panic("f5 catalog: duplicate product registration for " + p.Name)
	}
	products[p.Name] = p
}

// Get returns the Product registered under name, or ok=false.
func Get(name string) (Product, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := products[name]
	return p, ok
}

// AllProducts returns every registered Product, sorted by Name for
// deterministic ordering.
func AllProducts() []Product {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Product, 0, len(products))
	for _, p := range products {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ResetForTest empties the registry. Tests only.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	products = map[string]Product{}
}

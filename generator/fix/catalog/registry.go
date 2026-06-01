package catalog

import "sync"

// Registry holds all MessageDefinitions keyed by (Version, MsgType,
// AssetCategory). Per-version and per-category subpackages register
// their definitions into the package-global registry at init time via
// Register.
//
// Concurrency: Register and Get are guarded by a sync.RWMutex.
// Registration happens at init (single-goroutine); Get is called from
// generator hot paths and must be cheap.
type Registry struct {
	mu      sync.RWMutex
	entries map[MessageKey]*MessageDefinition
}

// globalRegistry is the package-level Registry instance. Subpackages
// register into it; the generator reads from it.
var globalRegistry = &Registry{entries: make(map[MessageKey]*MessageDefinition)}

// Register adds a MessageDefinition to the global Registry. Panics on
// duplicate registration of the same (Version, MsgType,
// AssetCategory) tuple — that signals a programmer error in the
// per-category catalog code, not a recoverable runtime condition.
func Register(def MessageDefinition) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	key := def.Key()
	if _, exists := globalRegistry.entries[key]; exists {
		panic("catalog: duplicate MessageDefinition registration for " + registryKeyString(key))
	}
	d := def // capture
	globalRegistry.entries[key] = &d
}

// Get returns the MessageDefinition registered under the given key, or
// nil if none is registered.
func Get(key MessageKey) *MessageDefinition {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.entries[key]
}

// AllDefinitions returns a snapshot of every registered
// MessageDefinition. The returned slice is independent of internal
// state; callers may sort or mutate it freely.
func AllDefinitions() []*MessageDefinition {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	out := make([]*MessageDefinition, 0, len(globalRegistry.entries))
	for _, d := range globalRegistry.entries {
		out = append(out, d)
	}
	return out
}

// ResetForTest empties the global Registry. Intended for tests that
// need a clean registry — must not be called from production code.
func ResetForTest() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.entries = make(map[MessageKey]*MessageDefinition)
}

func registryKeyString(k MessageKey) string {
	return k.Version.String() + "/" + k.MsgType + "/" + k.AssetCategory.String()
}

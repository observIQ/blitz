// Package datagen provides reusable data generation pools and identity types
// for synthetic telemetry generation. It offers deterministic, seed-controlled
// generation of hostnames, users, systems, networks, and other identity data.
package datagen

import "math/rand"

// Pool is a reusable collection of values for random selection.
// It is read-only after construction and safe for concurrent use.
type Pool[T any] struct {
	items []T
}

// NewPool creates a new Pool from the given items.
func NewPool[T any](items ...T) *Pool[T] {
	cp := make([]T, len(items))
	copy(cp, items)
	return &Pool[T]{items: cp}
}

// Random returns a random item from the pool using the provided rand source.
// Panics if the pool is empty.
func (p *Pool[T]) Random(r *rand.Rand) T {
	return p.items[r.Intn(len(p.items))] // #nosec G404
}

// RandomN returns n unique random items from the pool.
// If n >= pool size, returns all items in shuffled order.
// If n <= 0, returns an empty slice.
func (p *Pool[T]) RandomN(r *rand.Rand, n int) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(p.items) {
		n = len(p.items)
	}
	// Fisher-Yates shuffle on a copy, take first n
	cp := make([]T, len(p.items))
	copy(cp, p.items)
	r.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	return cp[:n]
}

// All returns a copy of all items in the pool.
func (p *Pool[T]) All() []T {
	cp := make([]T, len(p.items))
	copy(cp, p.items)
	return cp
}

// Len returns the number of items in the pool.
func (p *Pool[T]) Len() int {
	return len(p.items)
}

// Merge combines multiple pools into a single pool.
func Merge[T any](pools ...*Pool[T]) *Pool[T] {
	total := 0
	for _, p := range pools {
		total += len(p.items)
	}
	items := make([]T, 0, total)
	for _, p := range pools {
		items = append(items, p.items...)
	}
	return &Pool[T]{items: items}
}

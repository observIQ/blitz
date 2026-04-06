package hostmetrics

import (
	"math/rand"

	"github.com/observiq/blitz/output"
)

// Scraper generates metric records for a specific host metric category.
type Scraper interface {
	// Name returns the scraper name.
	Name() string
	// Scrape generates metric records for the current scrape cycle.
	Scrape(r *rand.Rand, hostname string, resource map[string]string) []output.MetricRecord
}

package datagen

import "math/rand"

// HTTP data pools for log generation.
var (
	// Methods is a pool of standard HTTP methods.
	Methods = NewPool("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")

	// Protocols is a pool of HTTP protocol versions.
	Protocols = NewPool("HTTP/1.0", "HTTP/1.1", "HTTP/2.0")

	// Status code pools grouped by class.
	Status2xx = NewPool(200, 201, 202, 204)
	Status3xx = NewPool(301, 302, 304, 307, 308)
	Status4xx = NewPool(400, 401, 403, 404, 405, 408, 429)
	Status5xx = NewPool(500, 502, 503, 504)

	// APIPaths is a pool of common API and web paths.
	APIPaths = NewPool(
		"/api/v1/users", "/api/v1/orders", "/api/v1/products",
		"/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/refresh",
		"/api/v2/data", "/api/v2/search", "/api/v2/metrics",
		"/health", "/healthz", "/ready", "/status",
		"/", "/about", "/contact", "/dashboard", "/settings",
		"/login", "/logout", "/register", "/profile",
		"/search", "/download", "/upload",
	)

	// RefererDomains is a pool of domains used in HTTP referer headers.
	RefererDomains = NewPool(
		"google.com", "bing.com", "github.com", "stackoverflow.com",
		"reddit.com", "linkedin.com", "example.com", "internal.corp",
	)
)

// RandomStatusCode returns a weighted random HTTP status code.
// Distribution: ~70% 2xx, ~5% 3xx, ~15% 4xx, ~10% 5xx.
func RandomStatusCode(r *rand.Rand) int {
	roll := r.Float64() // #nosec G404
	switch {
	case roll < 0.70:
		return Status2xx.Random(r)
	case roll < 0.75:
		return Status3xx.Random(r)
	case roll < 0.90:
		return Status4xx.Random(r)
	default:
		return Status5xx.Random(r)
	}
}

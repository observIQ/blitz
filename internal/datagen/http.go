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

	// APIPaths is a pool of common API and web paths. Includes both short
	// canonical endpoints and longer realistic paths so consumers can emit
	// log lines of varied length without needing their own pool.
	APIPaths = NewPool(
		// Short canonical endpoints
		"/", "/index.html",
		"/api/v1/users", "/api/v1/orders", "/api/v1/products",
		"/api/v1/inventory", "/api/v1/customers", "/api/v1/payments",
		"/api/v1/transactions", "/api/v1/accounts",
		"/api/v1/auth", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/refresh",
		"/api/v1/loans", "/api/v1/transfers", "/api/v1/verification",
		"/api/v2/data", "/api/v2/search", "/api/v2/metrics",
		"/health", "/healthz", "/ready", "/status",
		"/about", "/contact", "/dashboard", "/settings",
		"/login", "/logout", "/register", "/profile",
		"/search", "/download", "/upload",
		// Longer realistic paths for larger log lines
		"/api/v1/users/profile/settings",
		"/api/v2/analytics/reports/summary",
		"/api/v1/orders/history/recent",
		"/api/v2/recommendations/personalized",
		"/api/v1/notifications/preferences",
		"/api/v2/search/advanced/filters",
		"/api/v1/subscriptions/billing/invoices",
		"/api/v2/integrations/webhooks/events",
		"/api/v1/admin/users/permissions/roles",
		"/api/v2/metrics/performance/aggregated",
		"/api/v1/catalog/products/featured",
		"/api/v2/reports/exports/scheduled",
		"/api/v1/account/security/mfa",
		"/api/v2/workflows/tasks/assignments",
		"/api/v1/content/media/uploads",
	)

	// QueryStrings is a pool of realistic URL query-string fragments
	// (including leading "?") used to expand request-line length in HTTP
	// access-log generators.
	QueryStrings = NewPool(
		"?page=1&limit=25&sort=created_at&order=desc",
		"?filter=active&category=electronics&min_price=10.00&max_price=500.00",
		"?q=search+term&lang=en&results=20&offset=0",
		"?user_id=12345&include=profile%2Csettings&format=json",
		"?status=pending&from=2024-01-01&to=2024-12-31&page=1&limit=100",
		"?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9&refresh=true",
		"?fields=id%2Cname%2Cemail%2Ccreated_at&page=2&limit=50",
		"?utm_source=google&utm_medium=cpc&utm_campaign=spring_sale&utm_content=banner",
		"?session_id=abc123def456789&redirect_uri=%2Fdashboard&state=xyz789",
		"?expand=orders%2Cpayments%2Caddress&include_deleted=false&version=2",
		"?q=product+search&category=clothing&size=M&color=blue&brand=nike&sort=price_asc",
		"?start_date=2024-01-01T00%3A00%3A00Z&end_date=2024-12-31T23%3A59%3A59Z&interval=daily",
	)

	// RefererDomains is a pool of bare domain names used in HTTP referer
	// headers when consumers want to construct their own scheme+host prefix.
	// For pre-built scheme+host URL fragments, see RefererURLs.
	RefererDomains = NewPool(
		"google.com", "bing.com", "github.com", "stackoverflow.com",
		"reddit.com", "linkedin.com", "example.com", "internal.corp",
	)

	// RefererURLs is a pool of fully-qualified scheme+host URL prefixes
	// suitable for concatenation with a RefererPages entry. Use when the
	// consumer wants a realistic-looking referer string without assembling
	// scheme + www-prefix itself.
	RefererURLs = NewPool(
		"https://www.example.com",
		"https://search.example.com",
		"https://www.google.com",
		"https://www.bing.com",
		"https://github.com",
		"https://stackoverflow.com",
		"https://www.reddit.com",
		"https://www.linkedin.com",
	)

	// RefererPages is a pool of URL-path fragments (with optional query
	// strings) intended to be concatenated with a RefererURLs entry to
	// produce a realistic referer header.
	RefererPages = NewPool(
		"/",
		"/search",
		"/page1",
		"/page2",
		"/index.html",
		"/products",
		"/about",
		"/contact",
		"/search?q=opentelemetry+collector&category=tools",
		"/products?category=electronics&sort=price_asc&page=2",
		"/blog/posts?tag=observability&limit=10&page=1",
		"/docs/api/v2/reference?section=authentication",
		"/dashboard?view=analytics&period=last30days&metric=requests",
		"/shop?department=networking&brand=cisco&in_stock=true&page=3",
		"/articles?topic=distributed-systems&author=staff&year=2024",
		"/account/orders?status=shipped&from=2024-01-01&limit=50",
		"/pricing?plan=enterprise&billing=annual&seats=50",
		"/docs/guides/getting-started?lang=go&version=v2",
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

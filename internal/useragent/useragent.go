package useragent

import (
	"math/rand"
)

// UserAgent represents a user agent string with its relative weight/frequency
// Weight represents the real-world prevalence of this user agent
type UserAgent struct {
	String string
	Weight int
}

// userAgents contains a consolidated list of modern browser user agents
// weighted by their real-world prevalence as of 2025-2026 based on StatCounter data
// No bots are included per requirements
var userAgents = []UserAgent{
	// Chrome (Chromium-based) - ~60% market share
	// Desktop Windows (highest volume)
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 20,
	},
	// Desktop macOS
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 15,
	},
	// Desktop Linux
	{
		String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 8,
	},
	// Mobile Android
	{
		String: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		Weight: 12,
	},

	// Edge (Chromium-based) - ~5% market share
	// Desktop Windows
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		Weight: 3,
	},
	// Desktop macOS
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		Weight: 1,
	},
	// Desktop Linux
	{
		String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		Weight: 1,
	},

	// Firefox - ~9% market share
	// Desktop Windows
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 4,
	},
	// Desktop macOS
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 2,
	},
	// Desktop Linux
	{
		String: "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 2,
	},
	// Mobile Android
	{
		String: "Mozilla/5.0 (Android 14; Mobile; rv:121.0) Gecko/121.0 Firefox/121.0",
		Weight: 1,
	},

	// Safari - ~18% market share (mostly iOS)
	// Desktop macOS
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		Weight: 4,
	},
	// Mobile iOS
	{
		String: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
		Weight: 10,
	},
	// iPad
	{
		String: "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
		Weight: 4,
	},

	// UC Browser - ~2-3% market share (Asia-Pacific, particularly India)
	{
		String: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) UCBrowser/13.4.0.1306 Mobile Safari/537.36",
		Weight: 2,
	},

	// Samsung Internet - ~1% market share (Samsung Galaxy devices)
	{
		String: "Mozilla/5.0 (Linux; Android 14; SM-G990B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/21.0 Chrome/120.0.0.0 Mobile Safari/537.36",
		Weight: 1,
	},

	// Brave - ~0.5% market share (Desktop - Chromium-based but identifies itself)
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Brave/1.71.118",
		Weight: 1,
	},

	// Opera (Desktop) - ~0.3% market share
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
		Weight: 1,
	},

	// Opera Mini (Mobile) - ~0.3% market share (compression-based mobile browser)
	{
		String: "Opera/9.80 (Android 14; Opera Mini/36.2.2254/119.132) Presto/2.12.423 Version/12.00",
		Weight: 1,
	},

	// Chromium (standalone) - ~0.3% market share
	{
		String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chromium/120.0.0.0 Chrome/120.0.0.0 Safari/537.36",
		Weight: 1,
	},

	// Vivaldi - ~0.1% market share
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Vivaldi/6.5.3206.63",
		Weight: 1,
	},

	// Tor Browser - ~0.2% market share (privacy-focused)
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:115.0) Gecko/20100101 Firefox/115.0",
		Weight: 1,
	},

	// Yandex Browser - ~0.5% market share (popular in Russia/CIS)
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 YaBrowser/24.1.0.0",
		Weight: 1,
	},

	// Pale Moon - ~0.1% market share (Firefox fork, active community)
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:115.0) Gecko/20100101 Goanna/20230101 Firefox/115.0 PaleMoon/33.0.0",
		Weight: 1,
	},

	// DuckDuckGo Privacy Browser (Mobile) - ~0.1% market share
	{
		String: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 DuckDuckGo/5 Chrome/120.0.0.0 Mobile Safari/537.36",
		Weight: 1,
	},

	// Kiwi Browser (Mobile) - ~0.1% market share (Chrome-based with features)
	{
		String: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36 KiwiBrowser/3.9.104",
		Weight: 1,
	},

	// Silk / Amazon (Kindle tablets) - ~0.1% market share
	{
		String: "Mozilla/5.0 (Linux; U; Android 9; en-us; KFMUWI Build/LPA6.200720.038) AppleWebKit/537.36 (KHTML, like Gecko) Silk/120.0 Safari/537.36",
		Weight: 1,
	},

	// Epiphany / GNOME Web - ~0.1% market share (lightweight)
	{
		String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) EpiphanyBrowser/43.0 Safari/605.1.15",
		Weight: 1,
	},
}

// totalWeight caches the sum of all weights for efficient weighted random selection
var totalWeight = func() int {
	sum := 0
	for _, ua := range userAgents {
		sum += ua.Weight
	}
	return sum
}()

// RandomUserAgent returns a random user agent string weighted by real-world prevalence.
// More common browsers are selected more frequently based on their market share.
func RandomUserAgent(r *rand.Rand) string {
	// Generate random number between 0 and totalWeight
	random := r.Intn(totalWeight) // #nosec G404

	// Iterate through user agents, subtracting weights until we reach our random number
	for _, ua := range userAgents {
		random -= ua.Weight
		if random < 0 {
			return ua.String
		}
	}

	// Fallback (should never reach here if weights are correct)
	return userAgents[0].String
}

// UserAgents returns a slice of all available user agents for reference/testing
func UserAgents() []UserAgent {
	return append([]UserAgent{}, userAgents...)
}

// TotalWeight returns the sum of all user agent weights
func TotalWeight() int {
	return totalWeight
}

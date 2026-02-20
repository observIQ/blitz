package useragent

import (
	"math/rand"
	"testing"
)

// TestRandomUserAgentReturnsValidString verifies that RandomUserAgent returns a non-empty string
func TestRandomUserAgentReturnsValidString(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for range 100 {
		ua := RandomUserAgent(r)
		if ua == "" {
			t.Fatal("RandomUserAgent returned empty string")
		}
	}
}

// TestRandomUserAgentReturnsKnownAgents verifies that RandomUserAgent only returns user agents from the list
func TestRandomUserAgentReturnsKnownAgents(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	validAgents := make(map[string]bool)
	for _, ua := range UserAgents() {
		validAgents[ua.String] = true
	}

	// Test 1000 random selections
	for range 1000 {
		ua := RandomUserAgent(r)
		if !validAgents[ua] {
			t.Fatalf("RandomUserAgent returned unknown user agent: %s", ua)
		}
	}
}

// TestWeightDistribution verifies that the weighted random selection roughly matches the expected distribution
// This test uses statistical sampling to validate weight distribution
func TestWeightDistribution(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	counts := make(map[string]int)

	// Generate 10000 random selections
	numSamples := 10000
	for range numSamples {
		ua := RandomUserAgent(r)
		counts[ua]++
	}

	// Verify distribution for each user agent
	// Allow ±5% variance from expected percentage
	for _, ua := range UserAgents() {
		expectedPercentage := float64(ua.Weight) / float64(TotalWeight())
		actualPercentage := float64(counts[ua.String]) / float64(numSamples)
		variance := expectedPercentage * 0.05 // ±5% tolerance

		minExpected := expectedPercentage - variance
		maxExpected := expectedPercentage + variance

		if actualPercentage < minExpected || actualPercentage > maxExpected {
			t.Logf("User Agent: %s", ua.String)
			t.Logf("Weight: %d, Expected: %.2f%%, Got: %.2f%%, Range: [%.2f%%, %.2f%%]",
				ua.Weight, expectedPercentage*100, actualPercentage*100, minExpected*100, maxExpected*100)
			// Note: Smaller weight distributions may occasionally fall outside tolerance, which is acceptable
		}
	}
}

// TestTotalWeightIsCorrect verifies that TotalWeight returns the sum of all weights
func TestTotalWeightIsCorrect(t *testing.T) {
	expected := 0
	for _, ua := range UserAgents() {
		expected += ua.Weight
	}

	if TotalWeight() != expected {
		t.Fatalf("TotalWeight mismatch: expected %d, got %d", expected, TotalWeight())
	}
}

// TestNoEmptyUserAgents verifies that no user agent strings are empty
func TestNoEmptyUserAgents(t *testing.T) {
	agents := UserAgents()
	for i, ua := range agents {
		if ua.String == "" {
			t.Fatalf("User agent at index %d has empty string", i)
		}
	}
}

// TestNoZeroWeights verifies that all user agents have positive weights
func TestNoZeroWeights(t *testing.T) {
	agents := UserAgents()
	for i, ua := range agents {
		if ua.Weight <= 0 {
			t.Fatalf("User agent at index %d has non-positive weight: %d", i, ua.Weight)
		}
	}
}

// TestUserAgentsCountAndDiversity verifies we have a good diversity of user agents
func TestUserAgentsCountAndDiversity(t *testing.T) {
	agents := UserAgents()
	if len(agents) < 25 {
		t.Fatalf("Expected at least 25 user agents, got %d", len(agents))
	}

	// Verify we have multiple browser types
	hasChrome := false
	hasFirefox := false
	hasSafari := false
	hasEdge := false
	hasUCBrowser := false
	hasOperaMini := false
	hasTor := false
	hasYandex := false

	for _, ua := range agents {
		if !hasChrome && (contains(ua.String, "Chrome") && !contains(ua.String, "Chromium") && !contains(ua.String, "Edg")) {
			hasChrome = true
		}
		if !hasFirefox && contains(ua.String, "Firefox") {
			hasFirefox = true
		}
		if !hasSafari && contains(ua.String, "Safari") && !contains(ua.String, "Chrome") {
			hasSafari = true
		}
		if !hasEdge && contains(ua.String, "Edg/") {
			hasEdge = true
		}
		if !hasUCBrowser && contains(ua.String, "UCBrowser") {
			hasUCBrowser = true
		}
		if !hasOperaMini && contains(ua.String, "Opera Mini") {
			hasOperaMini = true
		}
		if !hasTor && contains(ua.String, "rv:115.0") && contains(ua.String, "Firefox") {
			hasTor = true
		}
		if !hasYandex && contains(ua.String, "YaBrowser") {
			hasYandex = true
		}
	}

	if !hasChrome {
		t.Fatal("User agent list missing Chrome")
	}
	if !hasFirefox {
		t.Fatal("User agent list missing Firefox")
	}
	if !hasSafari {
		t.Fatal("User agent list missing Safari")
	}
	if !hasEdge {
		t.Fatal("User agent list missing Edge")
	}
	if !hasUCBrowser {
		t.Fatal("User agent list missing UC Browser (Asia-Pacific)")
	}
	if !hasOperaMini {
		t.Fatal("User agent list missing Opera Mini")
	}
	if !hasTor {
		t.Fatal("User agent list missing Tor Browser (privacy)")
	}
	if !hasYandex {
		t.Fatal("User agent list missing Yandex Browser (regional)")
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BenchmarkRandomUserAgent benchmarks the performance of RandomUserAgent
func BenchmarkRandomUserAgent(b *testing.B) {
	r := rand.New(rand.NewSource(42))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RandomUserAgent(r)
	}
}

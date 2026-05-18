package datagen

import (
	"math/rand"
	"testing"
)

func TestHTTPMethods(t *testing.T) {
	expected := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	for _, m := range Methods.All() {
		if !expected[m] {
			t.Errorf("unexpected HTTP method: %q", m)
		}
	}
	if Methods.Len() != 7 {
		t.Errorf("expected 7 methods, got %d", Methods.Len())
	}
}

func TestHTTPProtocols(t *testing.T) {
	if Protocols.Len() < 3 {
		t.Errorf("expected at least 3 protocols, got %d", Protocols.Len())
	}
}

func TestHTTPStatusPools(t *testing.T) {
	if Status2xx.Len() < 3 {
		t.Errorf("Status2xx has %d items, want at least 3", Status2xx.Len())
	}
	if Status3xx.Len() < 3 {
		t.Errorf("Status3xx has %d items, want at least 3", Status3xx.Len())
	}
	if Status4xx.Len() < 5 {
		t.Errorf("Status4xx has %d items, want at least 5", Status4xx.Len())
	}
	if Status5xx.Len() < 3 {
		t.Errorf("Status5xx has %d items, want at least 3", Status5xx.Len())
	}
}

func TestAPIPaths(t *testing.T) {
	if APIPaths.Len() < 15 {
		t.Errorf("APIPaths has %d items, want at least 15", APIPaths.Len())
	}
}

func TestRefererDomains(t *testing.T) {
	if RefererDomains.Len() < 5 {
		t.Errorf("RefererDomains has %d items, want at least 5", RefererDomains.Len())
	}
}

func TestRandomStatusCode(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	counts := map[string]int{
		"2xx": 0,
		"3xx": 0,
		"4xx": 0,
		"5xx": 0,
	}
	for i := 0; i < 10000; i++ {
		code := RandomStatusCode(r)
		switch {
		case code >= 200 && code < 300:
			counts["2xx"]++
		case code >= 300 && code < 400:
			counts["3xx"]++
		case code >= 400 && code < 500:
			counts["4xx"]++
		case code >= 500 && code < 600:
			counts["5xx"]++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	// Verify rough distribution: 70% 2xx, 5% 3xx, 15% 4xx, 10% 5xx
	total := 10000.0
	if float64(counts["2xx"])/total < 0.60 {
		t.Errorf("2xx should be ~70%%, got %.1f%%", float64(counts["2xx"])/total*100)
	}
	if float64(counts["5xx"])/total < 0.05 {
		t.Errorf("5xx should be ~10%%, got %.1f%%", float64(counts["5xx"])/total*100)
	}
}

func TestRandomStatusCodeDeterministic(t *testing.T) {
	r1 := rand.New(rand.NewSource(99))
	r2 := rand.New(rand.NewSource(99))
	for i := 0; i < 50; i++ {
		if RandomStatusCode(r1) != RandomStatusCode(r2) {
			t.Fatal("same seed should produce same status codes")
		}
	}
}

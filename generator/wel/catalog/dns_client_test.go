package catalog

import (
	"math/rand"
	"testing"
)

func TestDNSClientEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(dnsClientChannel)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 DNS Client events, got %d", len(events))
	}
}

func TestDNSClientEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(dnsClientChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("DNS Client EventID %d: empty message", ev.EventID)
		}
	}
}

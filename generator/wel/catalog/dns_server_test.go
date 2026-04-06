package catalog

import (
	"math/rand"
	"testing"
)

func TestDNSServerEventsRegistered(t *testing.T) {
	regWS := DefaultRegistry(RoleWorkstation)
	if events := regWS.EventsForChannel(dnsServerChannel); len(events) != 0 {
		t.Errorf("DNS Server should not be available for workstation")
	}
	regDC := DefaultRegistry(RoleDC)
	if events := regDC.EventsForChannel(dnsServerChannel); len(events) < 5 {
		t.Fatalf("expected at least 5 DNS Server events for DC, got %d", len(events))
	}
}

func TestDNSServerEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "DC01", DomainName: "contoso.com", Role: RoleDC, Hostnames: []string{"DC02"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(dnsServerChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("DNSServer EventID %d: empty message", ev.EventID)
		}
	}
}

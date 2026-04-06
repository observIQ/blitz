package catalog

import (
	"math/rand"
	"testing"
)

func TestADDomainServiceEventsRegistered(t *testing.T) {
	regWS := DefaultRegistry(RoleWorkstation)
	if events := regWS.EventsForChannel(adDomainServiceChannel); len(events) != 0 {
		t.Errorf("AD Domain Service should not be available for workstation")
	}
	regDC := DefaultRegistry(RoleDC)
	if events := regDC.EventsForChannel(adDomainServiceChannel); len(events) < 4 {
		t.Fatalf("expected at least 4 AD Domain Service events for DC, got %d", len(events))
	}
}

func TestADDomainServiceEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "DC01", DomainName: "contoso.com", Role: RoleDC, Usernames: []string{"admin"}, IPs: []string{"10.0.0.100"}, Hostnames: []string{"DC02"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(adDomainServiceChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("ADDomainService EventID %d: empty message", ev.EventID)
		}
	}
}

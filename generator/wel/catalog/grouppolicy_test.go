package catalog

import (
	"math/rand"
	"testing"
)

func TestGroupPolicyEventsRegistered(t *testing.T) {
	regWS := DefaultRegistry(RoleWorkstation)
	if events := regWS.EventsForChannel(groupPolicyChannel); len(events) != 0 {
		t.Errorf("Group Policy should not be available for workstation")
	}
	regDC := DefaultRegistry(RoleDC)
	if events := regDC.EventsForChannel(groupPolicyChannel); len(events) < 5 {
		t.Fatalf("expected at least 5 Group Policy events for DC, got %d", len(events))
	}
}

func TestGroupPolicyEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "DC01", DomainName: "contoso.com", Role: RoleDC, Usernames: []string{"admin"}, Hostnames: []string{"DC02"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(groupPolicyChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("GroupPolicy EventID %d: empty message", ev.EventID)
		}
	}
}

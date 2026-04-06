package catalog

import (
	"math/rand"
	"testing"
)

func TestFirewallEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(firewallChannel)
	if len(events) < 5 {
		t.Fatalf("expected at least 5 Firewall events, got %d", len(events))
	}
}

func TestFirewallEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(firewallChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("Firewall EventID %d: empty message", ev.EventID)
		}
	}
}

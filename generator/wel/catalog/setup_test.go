package catalog

import (
	"math/rand"
	"testing"
)

func TestSetupEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(setupChannel)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 Setup events, got %d", len(events))
	}
}

func TestSetupEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(setupChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("Setup EventID %d: empty message", ev.EventID)
		}
	}
}

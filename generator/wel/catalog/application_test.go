package catalog

import (
	"math/rand"
	"testing"
)

func TestApplicationEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Application")

	if len(events) == 0 {
		t.Fatal("expected Application events to be registered")
	}

	expectedIDs := map[int]bool{
		1000: false, 1002: false, 1026: false,
		1033: false, 1034: false, 1035: false, 11707: false, 11708: false,
		8193: false, 326: false, 1530: false,
	}

	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected Application event %d to be registered", id)
		}
	}
}

func TestApplicationEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WORKSTATION01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		State:      NewStateTracker(100),
	}

	for _, ev := range reg.EventsForChannel("Application") {
		_, message := ev.Generate(rng, opts)
		if message == "" {
			t.Errorf("Application EventID %d: expected non-empty message", ev.EventID)
		}
	}
}

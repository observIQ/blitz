package catalog

import (
	"math/rand"
	"testing"
)

func TestSystemEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("System")

	if len(events) == 0 {
		t.Fatal("expected System events to be registered")
	}

	expectedIDs := map[int]bool{
		7000: false, 7009: false, 7023: false, 7031: false,
		7036: false, 7040: false, 7045: false,
		41: false, 42: false,
		6005: false, 6006: false, 6008: false, 6013: false,
		7: false, 11: false, 134: false, 1014: false,
		10016: false, 1074: false,
	}

	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected System event %d to be registered", id)
		}
	}
}

func TestSystemEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"admin", "jsmith"},
		IPs:        []string{"10.0.0.1"},
		Hostnames:  []string{"WORKSTATION01"},
		State:      NewStateTracker(100),
	}

	for _, ev := range reg.EventsForChannel("System") {
		_, message := ev.Generate(rng, opts)
		if message == "" {
			t.Errorf("System EventID %d: expected non-empty message", ev.EventID)
		}
	}
}

package catalog

import (
	"math/rand"
	"testing"
)

func TestSecuritySystemEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := map[int]bool{1102: false, 4608: false, 4610: false, 4611: false, 4616: false, 4622: false, 5024: false, 5025: false, 5156: false, 5157: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected system event %d to be registered", id)
		}
	}
}

func TestSecuritySystemEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"admin"},
		IPs:        []string{"10.0.0.1"},
		State:      NewStateTracker(100),
	}

	for _, id := range []int{1102, 4608, 4610, 4611, 4616, 4622, 5024, 5025, 5156, 5157} {
		var def *EventDefinition
		for _, ev := range reg.EventsForChannel("Security") {
			if ev.EventID == id {
				def = ev
				break
			}
		}
		if def == nil {
			t.Errorf("EventID %d not found", id)
			continue
		}
		_, message := def.Generate(rng, opts)
		if message == "" {
			t.Errorf("EventID %d: expected non-empty message", id)
		}
	}
}

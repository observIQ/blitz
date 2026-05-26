package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityObjectEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := map[int]bool{4656: false, 4657: false, 4658: false, 4660: false, 4663: false, 4670: false, 5140: false, 5145: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected object access event %d to be registered", id)
		}
	}
}

func TestSecurityObjectEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith", "admin"},
		IPs:        []string{"10.0.0.100"},
		Hostnames:  []string{"WORKSTATION01"},
		State:      NewStateTracker(100),
	}

	for _, id := range []int{4656, 4657, 4658, 4660, 4663, 4670, 5140, 5145} {
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

		data, message := def.Generate(rng, opts)
		if len(data) == 0 {
			t.Errorf("EventID %d: expected non-empty event data", id)
		}
		if message == "" {
			t.Errorf("EventID %d: expected non-empty message", id)
		}
	}
}

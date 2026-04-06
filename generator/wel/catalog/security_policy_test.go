package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityPolicyEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := map[int]bool{4703: false, 4704: false, 4705: false, 4719: false, 4739: false, 4946: false, 4947: false, 4948: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected policy event %d to be registered", id)
		}
	}
}

func TestSecurityPolicyEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"admin"},
		IPs:        []string{"10.0.0.1"},
		State:      NewStateTracker(100),
	}

	for _, id := range []int{4703, 4704, 4705, 4719, 4739, 4946, 4947, 4948} {
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
			t.Errorf("EventID %d: expected non-empty data", id)
		}
		if message == "" {
			t.Errorf("EventID %d: expected non-empty message", id)
		}
	}
}

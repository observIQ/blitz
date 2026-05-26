package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityDSEventsRegistered(t *testing.T) {
	// DS events should only be available for DC role
	regWS := DefaultRegistry(RoleWorkstation)
	regDC := DefaultRegistry(RoleDC)

	dsIDs := []int{4662, 5136, 5137, 5139, 5141}

	// Should NOT appear for workstation
	wsEvents := regWS.EventsForChannel("Security")
	for _, id := range dsIDs {
		for _, ev := range wsEvents {
			if ev.EventID == id {
				t.Errorf("DS event %d should not be registered for workstation", id)
			}
		}
	}

	// Should appear for DC
	dcEvents := regDC.EventsForChannel("Security")
	dcMap := make(map[int]bool)
	for _, ev := range dcEvents {
		dcMap[ev.EventID] = true
	}
	for _, id := range dsIDs {
		if !dcMap[id] {
			t.Errorf("DS event %d should be registered for DC", id)
		}
	}
}

func TestSecurityDSEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "DC01.contoso.com",
		DomainName: "contoso.com",
		Role:       RoleDC,
		Usernames:  []string{"admin"},
		State:      NewStateTracker(100),
	}

	for _, id := range []int{4662, 5136, 5137, 5139, 5141} {
		var def *EventDefinition
		for _, ev := range reg.EventsForChannel("Security") {
			if ev.EventID == id {
				def = ev
				break
			}
		}
		if def == nil {
			t.Errorf("EventID %d not found for DC", id)
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

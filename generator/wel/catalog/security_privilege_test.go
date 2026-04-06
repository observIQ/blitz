package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityPrivilegeEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := map[int]bool{4672: false, 4673: false, 4674: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected privilege event %d to be registered", id)
		}
	}
}

func TestSecurityPrivilegeEvent4672Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	var def *EventDefinition
	for _, ev := range reg.EventsForChannel("Security") {
		if ev.EventID == 4672 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4672 not found")
	}

	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"admin"},
		State:      NewStateTracker(100),
	}

	data, message := def.Generate(rng, opts)
	if len(data) == 0 {
		t.Error("expected non-empty event data")
	}
	if message == "" {
		t.Error("expected non-empty message")
	}

	// Should have PrivilegeList
	found := false
	for _, d := range data {
		if d.Name == "PrivilegeList" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing PrivilegeList field")
	}
}

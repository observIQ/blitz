package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityProcessEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := map[int]bool{4688: false, 4689: false, 4697: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected process event %d to be registered", id)
		}
	}
}

func TestSecurityProcessEvent4688Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	var def *EventDefinition
	for _, ev := range reg.EventsForChannel("Security") {
		if ev.EventID == 4688 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4688 not found")
	}

	rng := rand.New(rand.NewSource(42))
	st := NewStateTracker(100)
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		State:      st,
	}

	data, message := def.Generate(rng, opts)
	if len(data) == 0 {
		t.Error("expected non-empty event data")
	}
	if message == "" {
		t.Error("expected non-empty message")
	}

	// Verify key fields
	fieldMap := make(map[string]string)
	for _, d := range data {
		fieldMap[d.Name] = d.Value
	}
	for _, field := range []string{"NewProcessId", "NewProcessName", "CommandLine", "ParentProcessName"} {
		if _, ok := fieldMap[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestSecurityProcessEvent4689WithState(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	var def4689 *EventDefinition
	for _, ev := range reg.EventsForChannel("Security") {
		if ev.EventID == 4689 {
			def4689 = ev
			break
		}
	}
	if def4689 == nil {
		t.Fatal("EventID 4689 not found")
	}

	rng := rand.New(rand.NewSource(42))
	st := NewStateTracker(100)
	st.AddProcess("0x1234", `C:\test.exe`, "jsmith")

	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		State:      st,
	}

	data, message := def4689.Generate(rng, opts)
	if len(data) == 0 {
		t.Error("expected non-empty event data")
	}
	if message == "" {
		t.Error("expected non-empty message")
	}
}

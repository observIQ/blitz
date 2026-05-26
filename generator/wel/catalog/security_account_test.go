package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityAccountEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	expectedIDs := []int{
		4720, 4722, 4723, 4724, 4725, 4726,
		4727, 4728, 4729, 4730,
		4731, 4732, 4733, 4734, 4735,
		4737, 4738, 4740,
		4767, 4798, 4799,
	}

	eventMap := make(map[int]bool)
	for _, ev := range events {
		eventMap[ev.EventID] = true
	}

	for _, id := range expectedIDs {
		if !eventMap[id] {
			t.Errorf("expected Security account event %d to be registered", id)
		}
	}
}

func TestSecurityAccountEvent4720Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	var def *EventDefinition
	for _, ev := range reg.EventsForChannel("Security") {
		if ev.EventID == 4720 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4720 not found")
	}

	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "DC01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith", "admin"},
		State:      NewStateTracker(100),
	}

	data, message := def.Generate(rng, opts)
	if len(data) == 0 {
		t.Error("expected non-empty event data")
	}
	if message == "" {
		t.Error("expected non-empty message")
	}

	// Verify key fields present
	fieldMap := make(map[string]string)
	for _, d := range data {
		fieldMap[d.Name] = d.Value
	}
	for _, field := range []string{"SubjectUserName", "SubjectDomainName", "TargetUserName", "TargetDomainName"} {
		if _, ok := fieldMap[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestSecurityAccountEvent4740Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	var def *EventDefinition
	for _, ev := range reg.EventsForChannel("Security") {
		if ev.EventID == 4740 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4740 not found")
	}

	// 4740 is Audit Failure
	if def.Keywords != keywordsAuditFailure {
		t.Errorf("4740 should be Audit Failure, got keywords: 0x%x", def.Keywords)
	}

	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "DC01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		Hostnames:  []string{"WORKSTATION01"},
		State:      NewStateTracker(100),
	}

	data, message := def.Generate(rng, opts)
	if len(data) == 0 {
		t.Error("expected non-empty event data")
	}
	if message == "" {
		t.Error("expected non-empty message")
	}
}

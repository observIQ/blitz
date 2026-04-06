package catalog

import (
	"math/rand"
	"strings"
	"testing"
)

func TestSecurityLogonEventsRegistered(t *testing.T) {
	// Security logon events should be registered for all roles
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	// Find our logon events
	logonIDs := map[int]bool{
		4624: false, 4625: false, 4634: false, 4647: false,
		4648: false, 4778: false, 4779: false,
		4800: false, 4801: false, 4802: false, 4803: false,
	}

	for _, ev := range events {
		if _, ok := logonIDs[ev.EventID]; ok {
			logonIDs[ev.EventID] = true
		}
	}

	for id, found := range logonIDs {
		if !found {
			t.Errorf("expected Security logon event %d to be registered", id)
		}
	}
}

func TestSecurityLogonEvent4624Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	var def *EventDefinition
	for _, ev := range events {
		if ev.EventID == 4624 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4624 not found in registry")
	}

	// Verify definition metadata
	if def.Provider != "Microsoft-Windows-Security-Auditing" {
		t.Errorf("wrong provider: %q", def.Provider)
	}
	if def.Task != 12544 {
		t.Errorf("wrong task: %d", def.Task)
	}
	if def.Keywords != 0x8020000000000000 {
		t.Errorf("wrong keywords: 0x%x", def.Keywords)
	}

	// Generate and verify fields
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01.contoso.com",
		DomainName: "CONTOSO",
		Role:       RoleMember,
		Usernames:  []string{"jsmith", "mjohnson", "bwilliams"},
		IPs:        []string{"10.0.0.100", "192.168.1.50"},
		Hostnames:  []string{"WORKSTATION01", "LAPTOP02"},
		State:      NewStateTracker(100),
	}

	data, message := def.Generate(rng, opts)

	// Check required fields are present
	requiredFields := []string{
		"SubjectUserSid", "SubjectUserName", "SubjectDomainName", "SubjectLogonId",
		"TargetUserSid", "TargetUserName", "TargetDomainName", "TargetLogonId",
		"LogonType", "LogonProcessName", "AuthenticationPackageName",
		"WorkstationName", "IpAddress", "IpPort",
	}

	fieldMap := make(map[string]string)
	for _, d := range data {
		fieldMap[d.Name] = d.Value
	}

	for _, field := range requiredFields {
		if _, ok := fieldMap[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Message should not be empty
	if message == "" {
		t.Error("message should not be empty")
	}
	if !strings.Contains(message, "logged on") {
		t.Errorf("message should contain 'logged on': %q", message)
	}
}

func TestSecurityLogonEvent4625Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	var def *EventDefinition
	for _, ev := range events {
		if ev.EventID == 4625 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4625 not found")
	}

	// Verify it's an Audit Failure
	if def.Keywords != 0x8010000000000000 {
		t.Errorf("4625 should be Audit Failure, got keywords: 0x%x", def.Keywords)
	}

	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WIN-SERVER01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		IPs:        []string{"10.0.0.1"},
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

func TestSecurityLogonEvent4634Generate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Security")

	var def *EventDefinition
	for _, ev := range events {
		if ev.EventID == 4634 {
			def = ev
			break
		}
	}
	if def == nil {
		t.Fatal("EventID 4634 not found")
	}

	rng := rand.New(rand.NewSource(42))
	st := NewStateTracker(100)
	// Pre-populate with a session for the logoff to pick
	st.AddLogonSession("0xABCD1234", "jsmith", "CONTOSO")

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
}

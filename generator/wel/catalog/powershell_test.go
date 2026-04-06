package catalog

import (
	"math/rand"
	"testing"
)

func TestPowerShellEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel("Microsoft-Windows-PowerShell/Operational")

	if len(events) == 0 {
		t.Fatal("expected PowerShell events to be registered")
	}

	expectedIDs := map[int]bool{4104: false, 4103: false, 40961: false, 40962: false}
	for _, ev := range events {
		if _, ok := expectedIDs[ev.EventID]; ok {
			expectedIDs[ev.EventID] = true
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected PowerShell event %d to be registered", id)
		}
	}
}

func TestPowerShellEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "WORKSTATION01",
		DomainName: "CONTOSO",
		Usernames:  []string{"jsmith"},
		State:      NewStateTracker(100),
	}

	for _, ev := range reg.EventsForChannel("Microsoft-Windows-PowerShell/Operational") {
		data, message := ev.Generate(rng, opts)
		if len(data) == 0 {
			t.Errorf("PowerShell EventID %d: expected non-empty data", ev.EventID)
		}
		if message == "" {
			t.Errorf("PowerShell EventID %d: expected non-empty message", ev.EventID)
		}
	}
}

package catalog

import (
	"math/rand"
	"testing"
)

func TestSysmonEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(sysmonChannel)
	if len(events) == 0 {
		t.Fatal("expected Sysmon events")
	}
	expectedIDs := map[int]bool{1: false, 3: false, 5: false, 7: false, 11: false, 13: false, 22: false}
	for _, ev := range events {
		expectedIDs[ev.EventID] = true
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("missing Sysmon event %d", id)
		}
	}
}

func TestSysmonEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, IPs: []string{"10.0.0.1"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(sysmonChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("Sysmon EventID %d: empty message", ev.EventID)
		}
	}
}

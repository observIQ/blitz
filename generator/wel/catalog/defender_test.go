package catalog

import (
	"math/rand"
	"testing"
)

func TestDefenderEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(defenderChannel)
	if len(events) == 0 {
		t.Fatal("expected Defender events")
	}
	expectedIDs := map[int]bool{1000: false, 1001: false, 1002: false, 1006: false, 1007: false, 1116: false, 1117: false, 2000: false, 2001: false, 5001: false, 5007: false}
	for _, ev := range events {
		expectedIDs[ev.EventID] = true
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("missing Defender event %d", id)
		}
	}
}

func TestDefenderEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(defenderChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("Defender EventID %d: empty message", ev.EventID)
		}
	}
}

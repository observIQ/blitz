package catalog

import (
	"math/rand"
	"testing"
)

func TestTaskSchedulerEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(taskSchedulerChannel)
	if len(events) == 0 {
		t.Fatal("expected TaskScheduler events")
	}
	expectedIDs := map[int]bool{100: false, 101: false, 102: false, 106: false, 107: false, 110: false, 111: false, 141: false, 142: false}
	for _, ev := range events {
		expectedIDs[ev.EventID] = true
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("missing TaskScheduler event %d", id)
		}
	}
}

func TestTaskSchedulerEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(taskSchedulerChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("TaskScheduler EventID %d: empty message", ev.EventID)
		}
	}
}

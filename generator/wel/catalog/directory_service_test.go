package catalog

import (
	"math/rand"
	"testing"
)

func TestDirectoryServiceEventsRegistered(t *testing.T) {
	regWS := DefaultRegistry(RoleWorkstation)
	if events := regWS.EventsForChannel(directoryServiceChannel); len(events) != 0 {
		t.Errorf("Directory Service events should not be available for workstation, got %d", len(events))
	}

	regDC := DefaultRegistry(RoleDC)
	events := regDC.EventsForChannel(directoryServiceChannel)
	if len(events) < 7 {
		t.Fatalf("expected at least 7 Directory Service events for DC, got %d", len(events))
	}
}

func TestDirectoryServiceEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "DC01", DomainName: "contoso.com", Role: RoleDC, Usernames: []string{"admin"}, Hostnames: []string{"DC02"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(directoryServiceChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("DirectoryService EventID %d: empty message", ev.EventID)
		}
	}
}

package catalog

import (
	"math/rand"
	"testing"
)

func TestDFSReplicationEventsRegistered(t *testing.T) {
	regWS := DefaultRegistry(RoleWorkstation)
	if events := regWS.EventsForChannel(dfsReplicationChannel); len(events) != 0 {
		t.Errorf("DFS Replication should not be available for workstation, got %d", len(events))
	}
	regDC := DefaultRegistry(RoleDC)
	if events := regDC.EventsForChannel(dfsReplicationChannel); len(events) < 4 {
		t.Fatalf("expected at least 4 DFS Replication events for DC, got %d", len(events))
	}
}

func TestDFSReplicationEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{Computer: "DC01", DomainName: "contoso.com", Role: RoleDC, Usernames: []string{"admin"}, Hostnames: []string{"DC02"}, State: NewStateTracker(10)}
	for _, ev := range reg.EventsForChannel(dfsReplicationChannel) {
		_, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("DFSReplication EventID %d: empty message", ev.EventID)
		}
	}
}

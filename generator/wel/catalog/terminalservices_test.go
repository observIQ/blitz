package catalog

import (
	"math/rand"
	"testing"
)

func TestTerminalServicesEventsRegistered(t *testing.T) {
	reg := DefaultRegistry(RoleWorkstation)
	events := reg.EventsForChannel(terminalServicesChannel)
	if len(events) == 0 {
		t.Fatal("expected TerminalServices events")
	}
	for _, ev := range events {
		_, msg := ev.Generate(rand.New(rand.NewSource(42)), &GenerateOpts{
			Computer: "WS01", DomainName: "CONTOSO", Usernames: []string{"jsmith"}, IPs: []string{"10.0.0.1"}, State: NewStateTracker(10),
		})
		if msg == "" {
			t.Errorf("TerminalServices EventID %d: empty message", ev.EventID)
		}
	}
}

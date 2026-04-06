package catalog

import (
	"math/rand"
	"testing"
)

func TestSecurityAccountLogonEventsRegistered(t *testing.T) {
	// Kerberos events (DC only)
	regWS := DefaultRegistry(RoleWorkstation)
	regDC := DefaultRegistry(RoleDC)

	// 4776 (NTLM) should be available for all roles
	found4776 := false
	for _, ev := range regWS.EventsForChannel("Security") {
		if ev.EventID == 4776 {
			found4776 = true
			break
		}
	}
	if !found4776 {
		t.Error("4776 should be registered for workstation")
	}

	// Kerberos events should only be for DC
	kerberosIDs := []int{4768, 4769, 4770, 4771}
	dcEvents := regDC.EventsForChannel("Security")
	dcMap := make(map[int]bool)
	for _, ev := range dcEvents {
		dcMap[ev.EventID] = true
	}
	for _, id := range kerberosIDs {
		if !dcMap[id] {
			t.Errorf("Kerberos event %d should be registered for DC", id)
		}
	}

	// Kerberos should NOT be for workstation
	for _, ev := range regWS.EventsForChannel("Security") {
		for _, kid := range kerberosIDs {
			if ev.EventID == kid {
				t.Errorf("Kerberos event %d should NOT be registered for workstation", kid)
			}
		}
	}
}

func TestSecurityAccountLogonEventsGenerate(t *testing.T) {
	reg := DefaultRegistry(RoleDC)
	rng := rand.New(rand.NewSource(42))
	opts := &GenerateOpts{
		Computer:   "DC01.contoso.com",
		DomainName: "contoso.com",
		Role:       RoleDC,
		Usernames:  []string{"jsmith", "admin"},
		IPs:        []string{"10.0.0.100"},
		Hostnames:  []string{"WORKSTATION01"},
		State:      NewStateTracker(100),
	}

	for _, id := range []int{4768, 4769, 4770, 4771, 4776} {
		var def *EventDefinition
		for _, ev := range reg.EventsForChannel("Security") {
			if ev.EventID == id {
				def = ev
				break
			}
		}
		if def == nil {
			t.Errorf("EventID %d not found", id)
			continue
		}
		data, message := def.Generate(rng, opts)
		if len(data) == 0 {
			t.Errorf("EventID %d: expected non-empty data", id)
		}
		if message == "" {
			t.Errorf("EventID %d: expected non-empty message", id)
		}
	}
}

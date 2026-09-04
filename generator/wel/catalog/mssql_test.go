package catalog

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// mssqlEventIDs is the set of MSSQLSERVER events this catalog registers.
var mssqlEventIDs = []int{18453, 18454, 18456, 17137, 17187, 18264, 18265}

func TestMSSQLEventsRegistered(t *testing.T) {
	// MSSQL events are member-server scoped, so a workstation registry must not
	// see them and a member-server registry must.
	if got := mssqlEvents(DefaultRegistry(RoleWorkstation)); len(got) != 0 {
		t.Errorf("workstation registry should not include MSSQL events, got %d", len(got))
	}

	found := map[int]bool{}
	for _, ev := range mssqlEvents(DefaultRegistry(RoleMember)) {
		if ev.Channel != "Application" {
			t.Errorf("MSSQL event %d on channel %q, want Application", ev.EventID, ev.Channel)
		}
		found[ev.EventID] = true
	}
	for _, id := range mssqlEventIDs {
		if !found[id] {
			t.Errorf("expected MSSQL event %d registered", id)
		}
	}
}

func TestMSSQLEventMessages(t *testing.T) {
	reg := DefaultRegistry(RoleMember)
	rng := rand.New(rand.NewSource(7))
	opts := &GenerateOpts{
		Computer:   "SQLSERVER01",
		DomainName: "CONTOSO",
		Role:       RoleMember,
		Usernames:  []string{"jsmith", "svc_sql"},
		IPs:        []string{"10.0.0.50"},
	}

	wantSubstr := map[int]string{
		18453: "Windows authentication",
		18454: "SQL Server authentication",
		18456: "Login failed for user",
		17137: "Starting up database",
		17187: "not ready to accept new client connections",
		18264: "Database backed up",
		18265: "Log was backed up",
	}

	seen := map[int]bool{}
	for _, ev := range mssqlEvents(reg) {
		data, msg := ev.Generate(rng, opts)
		if msg == "" {
			t.Errorf("event %d produced an empty message", ev.EventID)
		}
		if len(data) == 0 {
			t.Errorf("event %d produced no EventData fields", ev.EventID)
		}
		if sub, ok := wantSubstr[ev.EventID]; ok {
			seen[ev.EventID] = true
			if !strings.Contains(msg, sub) {
				t.Errorf("event %d message %q should contain %q", ev.EventID, msg, sub)
			}
		}
	}
	for id, sub := range wantSubstr {
		if !seen[id] {
			t.Errorf("MSSQL event %d (%q) not exercised", id, sub)
		}
	}
}

// TestMSSQLBackupTimestampIsCurrent guards that the backup events (18264,
// 18265) render a live, current creation date rather than a hardcoded one, in
// line with dynamic timestamps across the generators.
func TestMSSQLBackupTimestampIsCurrent(t *testing.T) {
	reg := DefaultRegistry(RoleMember)
	rng := rand.New(rand.NewSource(1))
	opts := &GenerateOpts{Computer: "SQLSERVER01", DomainName: "CONTOSO", Role: RoleMember}
	currentYear := fmt.Sprintf("/%d(", time.Now().Year())

	backupIDs := map[int]bool{18264: true, 18265: true}
	checked := map[int]bool{}
	for _, ev := range mssqlEvents(reg) {
		if !backupIDs[ev.EventID] {
			continue
		}
		_, msg := ev.Generate(rng, opts)
		checked[ev.EventID] = true
		if strings.Contains(msg, "01/15/2024") {
			t.Errorf("event %d still emits the hardcoded date: %q", ev.EventID, msg)
		}
		if !strings.Contains(msg, currentYear) {
			t.Errorf("event %d should carry the current year %q, got %q", ev.EventID, currentYear, msg)
		}
	}
	for id := range backupIDs {
		if !checked[id] {
			t.Errorf("backup event %d not exercised", id)
		}
	}
}

// mssqlEvents returns the registry's MSSQLSERVER Application-channel events.
func mssqlEvents(reg *Registry) []*EventDefinition {
	var out []*EventDefinition
	for _, ev := range reg.EventsForChannel("Application") {
		if ev.Provider == "MSSQLSERVER" {
			out = append(out, ev)
		}
	}
	return out
}

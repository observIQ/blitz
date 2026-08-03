package datagen

import (
	"strings"
	"testing"
	"time"
)

func TestGroupScopes(t *testing.T) {
	scopes := []GroupScope{GroupScopeLocal, GroupScopeGlobal, GroupScopeUniversal}
	for _, s := range scopes {
		if s == "" {
			t.Error("GroupScope should not be empty")
		}
	}
}

func TestGroupTypes(t *testing.T) {
	types := []GroupType{GroupTypeSecurity, GroupTypeDistribution}
	for _, gt := range types {
		if gt == "" {
			t.Error("GroupType should not be empty")
		}
	}
}

func TestGenerateGroups(t *testing.T) {
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 20, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	groups, err := GenerateGroups(42, 10, 0, domain, users)
	if err != nil {
		t.Fatalf("GenerateGroups: %v", err)
	}

	t.Run("correct count", func(t *testing.T) {
		if len(groups) < 10 {
			t.Errorf("expected at least 10 groups, got %d", len(groups))
		}
	})

	t.Run("has built-in groups", func(t *testing.T) {
		names := make(map[string]bool)
		for _, g := range groups {
			names[g.Name] = true
		}
		if !names["Domain Admins"] {
			t.Error("expected 'Domain Admins' group")
		}
		if !names["Domain Users"] {
			t.Error("expected 'Domain Users' group")
		}
	})

	t.Run("SID format", func(t *testing.T) {
		for _, g := range groups {
			if !strings.HasPrefix(g.SID, "S-1-5-21-") {
				t.Errorf("group SID %q should start with S-1-5-21-", g.SID)
			}
		}
	})

	t.Run("DN format", func(t *testing.T) {
		for _, g := range groups {
			if !strings.HasPrefix(g.DN, "CN=") {
				t.Errorf("group DN %q should start with CN=", g.DN)
			}
		}
	})

	t.Run("members assigned", func(t *testing.T) {
		hasMembers := false
		for _, g := range groups {
			if len(g.MemberSIDs) > 0 {
				hasMembers = true
				break
			}
		}
		if !hasMembers {
			t.Error("at least one group should have members")
		}
	})

	t.Run("domain admins has members", func(t *testing.T) {
		for _, g := range groups {
			if g.Name == "Domain Admins" {
				if len(g.MemberSIDs) < 1 {
					t.Error("Domain Admins should have at least 1 member")
				}
				return
			}
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		g1, err := GenerateGroups(99, 5, 0, domain, users)
		if err != nil {
			t.Fatalf("GenerateGroups: %v", err)
		}
		g2, err := GenerateGroups(99, 5, 0, domain, users)
		if err != nil {
			t.Fatalf("GenerateGroups: %v", err)
		}
		for i := range g1 {
			if g1[i].Name != g2[i].Name {
				t.Errorf("same seed should produce same groups: %q vs %q", g1[i].Name, g2[i].Name)
			}
		}
	})
}

func TestGenerateGroupsBuiltinFloor(t *testing.T) {
	// Built-in AD groups (n=9) are always included; targetTotal below the floor
	// returns just the built-ins, not a truncated subset.
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 5, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	groups, err := GenerateGroups(42, 3, 0, domain, users)
	if err != nil {
		t.Fatalf("GenerateGroups: %v", err)
	}
	if len(groups) != len(builtinGroups) {
		t.Errorf("targetTotal=3 with %d built-ins should return %d groups, got %d",
			len(builtinGroups), len(builtinGroups), len(groups))
	}
}

func TestGenerateGroupsCap(t *testing.T) {
	// targetTotal beyond MaxGroupCount caps at MaxGroupCount.
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 5, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	groups, err := GenerateGroups(42, 100, 0, domain, users)
	if err != nil {
		t.Fatalf("GenerateGroups: %v", err)
	}
	if len(groups) != MaxGroupCount {
		t.Errorf("targetTotal=100 should return MaxGroupCount=%d groups, got %d", MaxGroupCount, len(groups))
	}
}

func TestDefaultDomainAdminsCount(t *testing.T) {
	cases := []struct {
		users int
		want  int
	}{
		{0, 2}, {1, 2}, {10, 2},
		{11, 3}, {50, 3},
		{51, 5}, {200, 5},
		{201, 8}, {1000, 8},
		{1001, 15}, {5000, 15},
		{5001, 25}, {10000, 25},
		{10001, 35}, {1000000, 35},
	}
	for _, c := range cases {
		if got := defaultDomainAdminsCount(c.users); got != c.want {
			t.Errorf("defaultDomainAdminsCount(%d) = %d, want %d", c.users, got, c.want)
		}
	}
}

func TestDomainAdminsUniqueAndExact(t *testing.T) {
	// Explicit adminCount: exactly that many unique users; no duplicates in
	// either MemberSIDs or any user's GroupSIDs.
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 100, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	const want = 10
	groups, err := GenerateGroups(42, 12, want, domain, users)
	if err != nil {
		t.Fatalf("GenerateGroups: %v", err)
	}

	var adminGroup *GroupIdentity
	for _, g := range groups {
		if g.Name == "Domain Admins" {
			adminGroup = g
			break
		}
	}
	if adminGroup == nil {
		t.Fatal("Domain Admins group not found")
	}

	if len(adminGroup.MemberSIDs) != want {
		t.Errorf("Domain Admins membership: got %d, want %d", len(adminGroup.MemberSIDs), want)
	}
	seen := make(map[string]bool)
	for _, sid := range adminGroup.MemberSIDs {
		if seen[sid] {
			t.Errorf("duplicate SID in Domain Admins MemberSIDs: %q", sid)
		}
		seen[sid] = true
	}

	for _, u := range users {
		count := 0
		for _, gs := range u.GroupSIDs {
			if gs == adminGroup.SID {
				count++
			}
		}
		if count > 1 {
			t.Errorf("user %s has Domain Admins SID %d times in GroupSIDs", u.Username, count)
		}
	}
}

func TestDomainAdminsCappedToUserCount(t *testing.T) {
	// adminCount > len(users) caps at len(users) instead of looping forever
	// or emitting duplicates.
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 3, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	groups, err := GenerateGroups(42, 9, 100, domain, users)
	if err != nil {
		t.Fatalf("GenerateGroups: %v", err)
	}
	for _, g := range groups {
		if g.Name == "Domain Admins" {
			if len(g.MemberSIDs) != len(users) {
				t.Errorf("adminCount=100 with 3 users: got %d members, want 3", len(g.MemberSIDs))
			}
		}
	}
}

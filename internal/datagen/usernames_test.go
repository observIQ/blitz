package datagen

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestNamePools(t *testing.T) {
	if FirstNames.Len() < 50 {
		t.Errorf("FirstNames has %d items, want at least 50", FirstNames.Len())
	}
	if Surnames.Len() < 50 {
		t.Errorf("Surnames has %d items, want at least 50", Surnames.Len())
	}
}

func TestDepartmentPool(t *testing.T) {
	if Departments.Len() < 8 {
		t.Errorf("Departments has %d items, want at least 8", Departments.Len())
	}
}

func TestTitlePool(t *testing.T) {
	if Titles.Len() < 5 {
		t.Errorf("Titles has %d items, want at least 5", Titles.Len())
	}
}

func TestGenerateUserIdentity(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())

	t.Run("basic fields populated", func(t *testing.T) {
		user, err := GenerateUserIdentity(r, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		if user.FirstName == "" {
			t.Error("FirstName should not be empty")
		}
		if user.LastName == "" {
			t.Error("LastName should not be empty")
		}
		if user.Username == "" {
			t.Error("Username should not be empty")
		}
		if !strings.HasSuffix(user.UPN, "@contoso.com") {
			t.Errorf("UPN %q should end with '@contoso.com'", user.UPN)
		}
		if !strings.HasSuffix(user.Email, "@contoso.com") {
			t.Errorf("Email %q should end with '@contoso.com'", user.Email)
		}
	})

	t.Run("display name is title case", func(t *testing.T) {
		user, err := GenerateUserIdentity(r, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		parts := strings.Split(user.DisplayName, " ")
		if len(parts) != 2 {
			t.Errorf("DisplayName %q should be 'First Last'", user.DisplayName)
		}
	})

	t.Run("SID format", func(t *testing.T) {
		user, err := GenerateUserIdentity(r, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		if !strings.HasPrefix(user.SID, domain.DomainSID+"-") {
			t.Errorf("user SID %q should start with domain SID %q", user.SID, domain.DomainSID)
		}
	})

	t.Run("has department and title", func(t *testing.T) {
		user, err := GenerateUserIdentity(r, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		if user.Department == "" {
			t.Error("Department should not be empty")
		}
		if user.Title == "" {
			t.Error("Title should not be empty")
		}
	})

	t.Run("DN format", func(t *testing.T) {
		user, err := GenerateUserIdentity(r, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		if !strings.HasPrefix(user.DN, "CN=") {
			t.Errorf("DN %q should start with 'CN='", user.DN)
		}
		if !strings.Contains(user.DN, "DC=contoso") {
			t.Errorf("DN %q should contain 'DC=contoso'", user.DN)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		r1 := rand.New(rand.NewSource(99))
		r2 := rand.New(rand.NewSource(99))
		u1, err := GenerateUserIdentity(r1, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		u2, err := GenerateUserIdentity(r2, domain)
		if err != nil {
			t.Fatalf("GenerateUserIdentity: %v", err)
		}
		if u1.Username != u2.Username {
			t.Errorf("same seed should produce same username: %q vs %q", u1.Username, u2.Username)
		}
	})
}

func TestGenerateUsers(t *testing.T) {
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 20, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}
	if len(users) != 20 {
		t.Errorf("expected 20 users, got %d", len(users))
	}

	// Check uniqueness of usernames
	seen := make(map[string]bool)
	for _, u := range users {
		if seen[u.Username] {
			t.Errorf("duplicate username %q", u.Username)
		}
		seen[u.Username] = true
	}
}

func TestGenerateUsersInternalConsistency(t *testing.T) {
	// With a small name pool relative to count, duplicate usernames are
	// highly likely. After disambiguation, every user's Username, UPN,
	// Email, DisplayName, and DN must be self-consistent: the numeric
	// suffix on Username must show up in UPN/Email local-parts, and
	// DisplayName must match the CN portion of DN.
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())
	users, err := GenerateUsers(42, 200, domain)
	if err != nil {
		t.Fatalf("GenerateUsers: %v", err)
	}

	upns := make(map[string]bool)
	emails := make(map[string]bool)
	dns := make(map[string]bool)
	for _, u := range users {
		// UPN and Email must mirror Username's local-part transformation.
		// Username is first[0]+last; UPN/Email local-part is first.last —
		// but both share the suffix when one is added.
		hasSuffix := false
		for _, ch := range u.Username {
			if ch >= '0' && ch <= '9' {
				hasSuffix = true
				break
			}
		}
		if hasSuffix {
			// UPN/Email local part must contain a digit too.
			at := strings.IndexByte(u.UPN, '@')
			if at == -1 {
				t.Errorf("user %s: UPN %q has no @", u.Username, u.UPN)
				continue
			}
			localPart := u.UPN[:at]
			localHasDigit := false
			for _, ch := range localPart {
				if ch >= '0' && ch <= '9' {
					localHasDigit = true
					break
				}
			}
			if !localHasDigit {
				t.Errorf("user %s has suffixed Username but UPN %q does not have a numeric suffix in local-part", u.Username, u.UPN)
			}
			// DisplayName must contain the suffix qualifier in parentheses.
			if !strings.Contains(u.DisplayName, "(") {
				t.Errorf("user %s has suffixed Username but DisplayName %q has no qualifier", u.Username, u.DisplayName)
			}
		}

		// Uniqueness across all identifying fields.
		if upns[u.UPN] {
			t.Errorf("duplicate UPN %q", u.UPN)
		}
		upns[u.UPN] = true
		if emails[u.Email] {
			t.Errorf("duplicate Email %q", u.Email)
		}
		emails[u.Email] = true
		if dns[u.DN] {
			t.Errorf("duplicate DN %q", u.DN)
		}
		dns[u.DN] = true

		// DN's CN must reflect DisplayName.
		if strings.HasPrefix(u.DN, "CN=") {
			rest := u.DN[3:]
			comma := strings.IndexByte(rest, ',')
			if comma == -1 {
				t.Errorf("user %s: DN %q has no comma after CN=", u.Username, u.DN)
				continue
			}
			cn := rest[:comma]
			if cn != u.DisplayName {
				t.Errorf("user %s: DN CN=%q does not match DisplayName %q", u.Username, cn, u.DisplayName)
			}
		}
	}
}

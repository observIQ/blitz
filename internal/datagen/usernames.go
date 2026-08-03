package datagen

import (
	"fmt"
	"math/rand"
	"strings"
)

// Name pools — 50 first names and 50 surnames.
var (
	FirstNames = NewPool(
		"james", "mary", "john", "patricia", "robert",
		"jennifer", "michael", "linda", "david", "elizabeth",
		"william", "barbara", "richard", "susan", "joseph",
		"jessica", "thomas", "sarah", "christopher", "karen",
		"charles", "lisa", "daniel", "nancy", "matthew",
		"betty", "anthony", "margaret", "mark", "sandra",
		"steven", "ashley", "paul", "emily", "andrew",
		"donna", "joshua", "michelle", "kenneth", "carol",
		"kevin", "amanda", "brian", "melissa", "george",
		"deborah", "timothy", "stephanie", "ronald", "rebecca",
	)

	Surnames = NewPool(
		"smith", "johnson", "williams", "brown", "jones",
		"garcia", "miller", "davis", "rodriguez", "martinez",
		"hernandez", "lopez", "gonzalez", "wilson", "anderson",
		"thomas", "taylor", "moore", "jackson", "martin",
		"lee", "perez", "thompson", "white", "harris",
		"sanchez", "clark", "ramirez", "lewis", "robinson",
		"walker", "young", "allen", "king", "wright",
		"scott", "torres", "nguyen", "hill", "flores",
		"green", "adams", "nelson", "baker", "hall",
		"rivera", "campbell", "mitchell", "carter", "roberts",
	)

	Departments = NewPool(
		"Engineering", "Sales", "Marketing", "Finance", "HR",
		"IT", "Operations", "Legal", "Security", "Executive",
	)

	Titles = NewPool(
		"Junior Developer", "Senior Developer", "Staff Engineer",
		"Team Lead", "Manager", "Director", "VP",
		"Analyst", "Consultant", "Administrator",
		"Specialist", "Coordinator", "Architect",
	)
)

// UserIdentity represents a domain user.
type UserIdentity struct {
	FirstName   string
	LastName    string
	Username    string   // sAMAccountName: "jsmith"
	UPN         string   // "james.smith@contoso.com"
	DisplayName string   // "James Smith"
	Email       string   // "james.smith@contoso.com"
	SID         string   // "S-1-5-21-..."
	Department  string   // "Engineering"
	Title       string   // "Senior Developer"
	DN          string   // "CN=James Smith,OU=Engineering,DC=contoso,DC=com"
	GroupSIDs   []string // back-references to GroupIdentity.SID
}

// GenerateUserIdentity creates a random user identity within the given domain.
func GenerateUserIdentity(r *rand.Rand, domain *DomainIdentity) (*UserIdentity, error) {
	if domain == nil {
		return nil, fmt.Errorf("datagen: GenerateUserIdentity: domain must not be nil")
	}
	first := FirstNames.Random(r)
	last := Surnames.Random(r)

	username := string(first[0]) + last // "jsmith"
	displayName := titleCase(first) + " " + titleCase(last)
	upn := first + "." + last + "@" + domain.Name
	email := upn

	// Build DN
	dept := Departments.Random(r)
	parts := strings.Split(domain.Name, ".")
	dcParts := make([]string, len(parts))
	for i, p := range parts {
		dcParts[i] = "DC=" + p
	}
	dn := fmt.Sprintf("CN=%s,OU=%s,%s", displayName, dept, strings.Join(dcParts, ","))

	// Generate user RID (1000+)
	rid := r.Intn(50000) + 1000 // #nosec G404
	sid := fmt.Sprintf("%s-%d", domain.DomainSID, rid)

	title := Titles.Random(r)

	return &UserIdentity{
		FirstName:   first,
		LastName:    last,
		Username:    username,
		UPN:         upn,
		DisplayName: displayName,
		Email:       email,
		SID:         sid,
		Department:  dept,
		Title:       title,
		DN:          dn,
	}, nil
}

// GenerateUsers produces a deterministic set of users from a seed.
//
// When two generated users would share a sAMAccountName (Username), the
// second and subsequent collisions get a numeric suffix that propagates
// through every dependent field — Username, UPN, Email, DisplayName, and
// the CN component of DN — so the returned UserIdentity stays internally
// consistent. In real AD, sAMAccountName, UPN, and mail must all be unique;
// the suffix scheme mirrors that.
func GenerateUsers(seed int64, count int, domain *DomainIdentity) ([]*UserIdentity, error) {
	r := rand.New(rand.NewSource(seed)) // #nosec G404
	users := make([]*UserIdentity, count)
	seen := make(map[string]int)
	for i := range users {
		u, err := GenerateUserIdentity(r, domain)
		if err != nil {
			return nil, err
		}
		seen[u.Username]++
		if seen[u.Username] > 1 {
			disambiguateUser(u, seen[u.Username])
		}
		users[i] = u
	}
	return users, nil
}

// disambiguateUser appends a numeric suffix to all identifier-bearing fields
// of u so the user remains internally consistent after a Username collision.
// suffix is the duplicate index (>= 2 by construction in GenerateUsers).
func disambiguateUser(u *UserIdentity, suffix int) {
	u.Username = fmt.Sprintf("%s%d", u.Username, suffix)
	// UPN and Email share the local@domain shape; rebuild the local part.
	if at := strings.IndexByte(u.UPN, '@'); at != -1 {
		u.UPN = fmt.Sprintf("%s%d%s", u.UPN[:at], suffix, u.UPN[at:])
	}
	if at := strings.IndexByte(u.Email, '@'); at != -1 {
		u.Email = fmt.Sprintf("%s%d%s", u.Email[:at], suffix, u.Email[at:])
	}
	// DisplayName + DN's CN component get a "(N)" qualifier — keeps the DN
	// human-readable while ensuring the CN is distinct.
	u.DisplayName = fmt.Sprintf("%s (%d)", u.DisplayName, suffix)
	if strings.HasPrefix(u.DN, "CN=") {
		if comma := strings.IndexByte(u.DN, ','); comma != -1 {
			u.DN = fmt.Sprintf("CN=%s,%s", u.DisplayName, u.DN[comma+1:])
		}
	}
}

// titleCase capitalizes the first letter of a string.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

package datagen

import (
	"fmt"
	"math/rand"
	"strings"
)

// GroupScope represents an AD group scope.
type GroupScope string

const (
	GroupScopeLocal     GroupScope = "DomainLocal"
	GroupScopeGlobal    GroupScope = "Global"
	GroupScopeUniversal GroupScope = "Universal"
)

// GroupType represents an AD group type.
type GroupType string

const (
	GroupTypeSecurity     GroupType = "Security"
	GroupTypeDistribution GroupType = "Distribution"
)

// GroupIdentity represents an AD security or distribution group.
type GroupIdentity struct {
	Name        string // "Domain Admins"
	SID         string // "S-1-5-21-...-512"
	DN          string // "CN=Domain Admins,CN=Users,DC=contoso,DC=com"
	Scope       GroupScope
	Type        GroupType
	MemberSIDs  []string // references to UserIdentity.SID
	Description string
}

// builtinGroup defines a built-in AD group template.
type builtinGroup struct {
	name        string
	rid         int
	scope       GroupScope
	groupType   GroupType
	description string
}

var builtinGroups = []builtinGroup{
	{"Domain Admins", 512, GroupScopeGlobal, GroupTypeSecurity, "Designated administrators of the domain"},
	{"Domain Users", 513, GroupScopeGlobal, GroupTypeSecurity, "All domain users"},
	{"Enterprise Admins", 519, GroupScopeUniversal, GroupTypeSecurity, "Enterprise administrators"},
	{"Schema Admins", 518, GroupScopeUniversal, GroupTypeSecurity, "Schema administrators"},
	{"DNS Admins", 1101, GroupScopeLocal, GroupTypeSecurity, "DNS administrators"},
	{"Group Policy Creator Owners", 520, GroupScopeGlobal, GroupTypeSecurity, "Group policy creators"},
	{"Server Operators", 549, GroupScopeLocal, GroupTypeSecurity, "Server operators"},
	{"Backup Operators", 551, GroupScopeLocal, GroupTypeSecurity, "Backup operators"},
	{"Remote Desktop Users", 555, GroupScopeLocal, GroupTypeSecurity, "Remote desktop users"},
}

// deptGroupCatalog is the static list of department-based groups appended
// after the built-ins until targetTotal is reached.
var deptGroupCatalog = []string{
	"Engineering-Team", "Sales-Team", "IT-Support",
	"Marketing-Team", "Finance-Team", "HR-Team",
	"Operations-Team", "Legal-Team", "Security-Team",
}

// MaxGroupCount is the largest number of groups GenerateGroups can return:
// every built-in plus every department group in the catalog.
var MaxGroupCount = len(builtinGroups) + len(deptGroupCatalog)

// defaultDomainAdminsCount returns a realistic Domain Admins count for a
// directory of n users. Microsoft's published guidance is qualitative
// ("minimize Domain Admins membership"), not a sizing table. Real-world AD
// audits typically observe membership scaling with org size; the step
// thresholds below match the upper end of what audited environments commonly
// find, not Microsoft best-practice. Callers wanting tight-shop simulation
// should override via the explicit adminCount argument or
// EnvironmentOpts.DomainAdminsCount.
func defaultDomainAdminsCount(userCount int) int {
	switch {
	case userCount <= 10:
		return 2
	case userCount <= 50:
		return 3
	case userCount <= 200:
		return 5
	case userCount <= 1000:
		return 8
	case userCount <= 5000:
		return 15
	case userCount <= 10000:
		return 25
	default:
		return 35
	}
}

// GenerateGroups produces groups including built-in AD groups and
// department-based groups, then assigns users to those groups by department
// and to Domain Admins / Domain Users.
//
// The built-in AD groups (Domain Admins, Domain Users, Enterprise Admins,
// etc.) are always included regardless of targetTotal — an AD environment
// without these foundational groups would be incoherent. Department groups
// are appended from the catalog until the total reaches targetTotal or the
// catalog is exhausted; the maximum total is MaxGroupCount.
//
// adminCount controls how many users are placed in Domain Admins:
//   - adminCount > 0: that many users (capped at len(users)).
//   - adminCount <= 0: a default scaled to len(users) — see
//     defaultDomainAdminsCount for thresholds.
//
// User selection for Domain Admins is sampled without replacement (Fisher-Yates
// partial shuffle), so no user appears twice in domainAdmins.MemberSIDs and
// no user has duplicate Domain Admins references in their GroupSIDs.
func GenerateGroups(seed int64, targetTotal int, adminCount int, domain *DomainIdentity, users []*UserIdentity) ([]*GroupIdentity, error) {
	if domain == nil {
		return nil, fmt.Errorf("datagen: GenerateGroups: domain must not be nil")
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404

	dcSuffix := domainToDC(domain.Name)
	groups := make([]*GroupIdentity, 0, MaxGroupCount)

	// Add built-in groups (always included)
	for _, bg := range builtinGroups {
		g := &GroupIdentity{
			Name:        bg.name,
			SID:         fmt.Sprintf("%s-%d", domain.DomainSID, bg.rid),
			DN:          fmt.Sprintf("CN=%s,CN=Users,%s", bg.name, dcSuffix),
			Scope:       bg.scope,
			Type:        bg.groupType,
			Description: bg.description,
		}
		groups = append(groups, g)
	}

	// Add department-based groups until we reach targetTotal
	rid := 2000
	for _, name := range deptGroupCatalog {
		if len(groups) >= targetTotal {
			break
		}
		g := &GroupIdentity{
			Name:        name,
			SID:         fmt.Sprintf("%s-%d", domain.DomainSID, rid),
			DN:          fmt.Sprintf("CN=%s,OU=Groups,%s", name, dcSuffix),
			Scope:       GroupScopeGlobal,
			Type:        GroupTypeSecurity,
			Description: fmt.Sprintf("Members of the %s department", strings.TrimSuffix(name, "-Team")),
		}
		groups = append(groups, g)
		rid++
	}

	// Assign users to groups
	assignUsersToGroups(r, groups, users, adminCount)

	return groups, nil
}

// assignUsersToGroups distributes users across groups: every user joins
// Domain Users, a sampled-without-replacement subset joins Domain Admins,
// and users join their department-named group when one exists.
func assignUsersToGroups(r *rand.Rand, groups []*GroupIdentity, users []*UserIdentity, adminCount int) {
	if len(users) == 0 || len(groups) == 0 {
		return
	}

	// Find Domain Admins and Domain Users groups
	var domainAdmins, domainUsers *GroupIdentity
	deptGroupMap := make(map[string]*GroupIdentity)
	for _, g := range groups {
		switch g.Name {
		case "Domain Admins":
			domainAdmins = g
		case "Domain Users":
			domainUsers = g
		}
		// Map department-based groups
		if strings.HasSuffix(g.Name, "-Team") {
			dept := strings.TrimSuffix(g.Name, "-Team")
			deptGroupMap[dept] = g
		}
	}

	// All users go to Domain Users
	if domainUsers != nil {
		for _, u := range users {
			domainUsers.MemberSIDs = append(domainUsers.MemberSIDs, u.SID)
			u.GroupSIDs = append(u.GroupSIDs, domainUsers.SID)
		}
	}

	// Assign users to Domain Admins, sampled without replacement.
	if domainAdmins != nil {
		want := adminCount
		if want <= 0 {
			want = defaultDomainAdminsCount(len(users))
		}
		if want > len(users) {
			want = len(users)
		}
		for _, idx := range partialShuffle(r, len(users), want) {
			domainAdmins.MemberSIDs = append(domainAdmins.MemberSIDs, users[idx].SID)
			users[idx].GroupSIDs = append(users[idx].GroupSIDs, domainAdmins.SID)
		}
	}

	// Assign users to department groups
	for _, u := range users {
		dept := u.Department
		if g, ok := deptGroupMap[dept]; ok {
			g.MemberSIDs = append(g.MemberSIDs, u.SID)
			u.GroupSIDs = append(u.GroupSIDs, g.SID)
		}
	}
}

// partialShuffle returns k unique indices from [0, n) selected uniformly at
// random via a Fisher-Yates partial shuffle. If k >= n it returns all n
// indices in random order. Allocates one int slice of length n.
func partialShuffle(r *rand.Rand, n, k int) []int {
	if k > n {
		k = n
	}
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for i := 0; i < k; i++ {
		j := i + r.Intn(n-i) // #nosec G404
		indices[i], indices[j] = indices[j], indices[i]
	}
	return indices[:k]
}

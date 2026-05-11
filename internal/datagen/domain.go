package datagen

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// DomainIdentity represents a simulated Active Directory domain.
type DomainIdentity struct {
	Name            string         // "contoso.com"
	NetBIOSName     string         // "CONTOSO"
	ForestName      string         // "contoso.com" (single-domain forest)
	DomainSID       string         // "S-1-5-21-xxxxxxxxxx-xxxxxxxxxx-xxxxxxxxxx"
	FunctionalLevel string         // "2016"
	Sites           []string       // ["Default-First-Site-Name", "Branch-Office-01"]
	CA              *CertAuthority // Enterprise root CA
}

// CertAuthority represents a certificate authority for the domain.
type CertAuthority struct {
	CommonName   string    // "contoso-ROOT-CA"
	Thumbprint   string    // SHA1 hex (40 chars)
	SerialNumber string    // Hex serial
	ValidFrom    time.Time // 5 years before the supplied "now"
	ValidTo      time.Time // 5 years after the supplied "now"
	CRLDistPoint string    // "ldap:///CN=contoso-ROOT-CA,...,DC=contoso,DC=com"
}

// GenerateDomainIdentity creates a deterministic domain identity from (seed, now).
// If domainName is empty, defaults to "blitz.local". The "now" parameter pins
// the CertAuthority validity window so that the function is reproducible from
// (seed, now) alone — callers that want a stable test fixture pass a fixed
// timestamp; callers that want a currently-valid cert pass the wall clock.
func GenerateDomainIdentity(seed int64, domainName string, now time.Time) *DomainIdentity {
	r := rand.New(rand.NewSource(seed)) // #nosec G404

	if domainName == "" {
		domainName = "blitz.local"
	}

	// Extract NetBIOS name (first label, uppercased)
	netbios := strings.ToUpper(strings.Split(domainName, ".")[0])

	// Generate domain SID
	sid := fmt.Sprintf("S-1-5-21-%d-%d-%d",
		r.Int31n(2000000000)+1000000000, // #nosec G404
		r.Int31n(2000000000)+1000000000, // #nosec G404
		r.Int31n(2000000000)+1000000000) // #nosec G404

	// Generate sites
	sites := []string{"Default-First-Site-Name"}
	if r.Float64() > 0.3 { // #nosec G404
		sites = append(sites, "Branch-Office-01")
	}

	// Generate CA
	ca := generateCertAuthority(r, netbios, domainName, now)

	return &DomainIdentity{
		Name:            domainName,
		NetBIOSName:     netbios,
		ForestName:      domainName,
		DomainSID:       sid,
		FunctionalLevel: "2016",
		Sites:           sites,
		CA:              ca,
	}
}

// generateCertAuthority creates a deterministic root CA for the domain.
// The validity window is pinned to the supplied "now" so the function is
// reproducible from its inputs alone.
func generateCertAuthority(r *rand.Rand, netbios, domainName string, now time.Time) *CertAuthority {
	caName := fmt.Sprintf("%s-ROOT-CA", strings.ToLower(netbios))

	// Thumbprint: 40 hex chars
	thumbprint := randomHex(r, 20)

	// Serial number: 16 hex chars
	serial := randomHex(r, 8)

	validFrom := now.AddDate(-5, 0, 0) // 5 years ago
	validTo := now.AddDate(5, 0, 0)    // 5 years from now

	crl := fmt.Sprintf("ldap:///CN=%s,CN=AIA,CN=Public Key Services,CN=Services,CN=Configuration,%s",
		caName, domainToDC(domainName))

	return &CertAuthority{
		CommonName:   caName,
		Thumbprint:   thumbprint,
		SerialNumber: serial,
		ValidFrom:    validFrom,
		ValidTo:      validTo,
		CRLDistPoint: crl,
	}
}

// domainToDC converts a domain name like "contoso.com" to its
// LDAP-distinguished-name suffix "DC=contoso,DC=com".
func domainToDC(name string) string {
	parts := strings.Split(name, ".")
	dcParts := make([]string, len(parts))
	for i, p := range parts {
		dcParts[i] = "DC=" + p
	}
	return strings.Join(dcParts, ",")
}

// randomHex generates a hex string of nBytes length (2 hex chars per byte).
func randomHex(r *rand.Rand, nBytes int) string {
	var sb strings.Builder
	sb.Grow(nBytes * 2)
	for i := 0; i < nBytes; i++ {
		fmt.Fprintf(&sb, "%02x", r.Intn(256)) // #nosec G404
	}
	return sb.String()
}

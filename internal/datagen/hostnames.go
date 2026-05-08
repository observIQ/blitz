package datagen

import (
	"fmt"
	"math/rand"
	"strings"
)

// Mythology name pools — each with 26 names from their respective pantheon.
var (
	// NorseNames contains names from Norse mythology. Convention: Linux servers.
	NorseNames = NewPool(
		"odin", "thor", "freya", "loki", "tyr", "heimdall", "baldur",
		"frigg", "sif", "bragi", "idun", "njord", "skadi", "vidar", "vali",
		"forseti", "hermod", "hod", "mimir", "ran", "aegir", "fenrir",
		"jormungandr", "hel", "surtr", "ymir",
	)

	// GreekNames contains names from Greek mythology. Convention: Domain Controllers.
	GreekNames = NewPool(
		"zeus", "athena", "apollo", "artemis", "hermes", "hera",
		"poseidon", "demeter", "ares", "aphrodite", "hephaestus", "dionysus",
		"hades", "persephone", "hecate", "nike", "iris", "eos", "selene",
		"helios", "atlas", "prometheus", "pandora", "orpheus", "icarus", "chronos",
	)

	// RomanNames contains names from Roman mythology. Convention: Windows servers.
	RomanNames = NewPool(
		"jupiter", "minerva", "mars", "venus", "mercury", "diana",
		"neptune", "ceres", "vulcan", "juno", "pluto", "bacchus", "saturn",
		"aurora", "flora", "fortuna", "luna", "sol", "terra", "victoria",
		"bellona", "faunus", "janus", "pax", "trivia", "vesta",
	)

	// EgyptianNames contains names from Egyptian mythology. Convention: Network appliances.
	EgyptianNames = NewPool(
		"ra", "isis", "osiris", "anubis", "horus", "thoth",
		"bastet", "sekhmet", "ptah", "hathor", "sobek", "maat", "nut",
		"geb", "tefnut", "shu", "nephthys", "set", "khonsu", "amon",
		"wadjet", "neith", "serket", "khnum", "taweret", "bes",
	)

	// CelticNames contains names from Celtic mythology. Convention: macOS / dev workstations.
	CelticNames = NewPool(
		"brigid", "cernunnos", "morrigan", "lugh", "danu", "dagda",
		"nuada", "ogma", "aengus", "boann", "manannan", "rhiannon", "arawn",
		"belenus", "epona", "taranis", "mabon", "cerridwen", "gwydion",
		"llyr", "blodeuwedd", "govannon", "arianrhod", "diancecht", "midir", "cliodhna",
	)

	// AllMythologyNames combines all mythology pools for general use.
	AllMythologyNames = Merge(NorseNames, GreekNames, RomanNames, EgyptianNames, CelticNames)

	// Roles are server/machine role labels used in hostname generation.
	Roles = NewPool(
		"web", "db", "app", "api", "cache",
		"worker", "proxy", "monitor", "log", "queue",
		"mail", "dns", "auth", "vault", "ci",
	)
)

// HostnameStyle controls the naming convention for generated hostnames.
type HostnameStyle int

const (
	// StyleLinux produces hostnames like "thor-web-01".
	StyleLinux HostnameStyle = iota
	// StyleWindows produces hostnames like "THOR-WEB01".
	StyleWindows
	// StyleDC produces DC-style hostnames like "THOR-DC01".
	StyleDC
)

// GenerateHostname produces a single random hostname using the given style and name pool.
// If names is nil, defaults to AllMythologyNames.
func GenerateHostname(r *rand.Rand, style HostnameStyle, names *Pool[string]) string {
	if names == nil {
		names = AllMythologyNames
	}
	name := names.Random(r)
	num := r.Intn(20) + 1 // #nosec G404

	switch style {
	case StyleLinux:
		role := Roles.Random(r)
		return fmt.Sprintf("%s-%s-%02d", strings.ToLower(name), strings.ToLower(role), num)
	case StyleWindows:
		role := Roles.Random(r)
		return fmt.Sprintf("%s-%s%02d", strings.ToUpper(name), strings.ToUpper(role), num)
	case StyleDC:
		return fmt.Sprintf("%s-DC%02d", strings.ToUpper(name), num)
	default:
		role := Roles.Random(r)
		return fmt.Sprintf("%s-%s-%02d", strings.ToLower(name), strings.ToLower(role), num)
	}
}

// GenerateHostnames produces a deterministic set of hostnames from a seed.
// The same seed + pool always produces the same set.
// If names is nil, defaults to AllMythologyNames.
func GenerateHostnames(seed int64, count int, style HostnameStyle, names *Pool[string]) []string {
	r := rand.New(rand.NewSource(seed)) // #nosec G404
	hostnames := make([]string, count)
	for i := range hostnames {
		hostnames[i] = GenerateHostname(r, style, names)
	}
	return hostnames
}

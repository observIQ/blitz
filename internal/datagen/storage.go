package datagen

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

// StorageVendor identifies a storage-array manufacturer.
type StorageVendor string

const (
	StorageVendorHPE     StorageVendor = "hpe"
	StorageVendorNetApp  StorageVendor = "netapp"
	StorageVendorPure    StorageVendor = "pure"
	StorageVendorDellEMC StorageVendor = "dell-emc"
)

// StorageDrive is a physical drive in a storage array.
type StorageDrive struct {
	Slot       string // "shelf1-bay14"
	Type       string // "ssd", "nvme-ssd", "hdd"
	CapacityTB float64
	Model      string
	Serial     string
}

// StorageShelf is a drive shelf/enclosure holding a set of drives.
type StorageShelf struct {
	ID        string // "shelf-01"
	Model     string
	DriveBays int
	Drives    []StorageDrive
}

// StorageController is an array controller node.
type StorageController struct {
	ID              string // "ctrl-A"
	Serial          string
	Role            string // "active", "standby"
	FirmwareVersion string
}

// StorageCapacity models an array's capacity accounting. Effective capacity is
// usable capacity multiplied by the data-reduction ratio (dedup + compression).
type StorageCapacity struct {
	RawCapacityTB       float64
	UsableCapacityTB    float64
	EffectiveCapacityTB float64
	DataReductionRatio  float64
}

// StorageSystemIdentity is a first-class storage-array machine in the simulated
// environment: a vendor/model/serial box running an embedded ApplianceOS, with
// storage-fabric identifiers, a capacity model, and hardware inventory.
type StorageSystemIdentity struct {
	Vendor StorageVendor
	Model  string
	Serial string
	OS     *ApplianceOS

	// Storage-fabric identifiers.
	WWN  string   // node World Wide Name (8-byte colon hex)
	IQN  string   // iSCSI Qualified Name
	WWPN []string // per-port Fibre Channel World Wide Port Names
	NAA  string   // NAA type-6 identifier for the array's volume namespace

	Capacity StorageCapacity

	Controllers []StorageController
	Shelves     []StorageShelf
	Drives      []StorageDrive

	AdminUserRef        *UserIdentity
	ManagementInterface *NetworkInterface
}

// storageModelSpec describes a concrete storage model: its vendor, the OS
// family it runs, its predominant drive type, and a raw-capacity range in TB.
type storageModelSpec struct {
	vendor    StorageVendor
	model     string
	osFamily  ApplianceOSFamily
	driveType string
	minRawTB  float64
	maxRawTB  float64
}

// hpeStorageModels is the first vendor pool: HPE Nimble, 3PAR, Alletra, and
// StoreOnce models. Capacity ranges are representative, not exhaustive.
var hpeStorageModels = []storageModelSpec{
	{StorageVendorHPE, "Nimble HF20", FamilyNimbleOS, "hybrid", 21, 126},
	{StorageVendorHPE, "Nimble HF40", FamilyNimbleOS, "hybrid", 42, 336},
	{StorageVendorHPE, "Nimble AF40", FamilyNimbleOS, "ssd", 23, 184},
	{StorageVendorHPE, "Nimble AF80", FamilyNimbleOS, "ssd", 46, 553},
	{StorageVendorHPE, "3PAR 8200", Family3PAROS, "hybrid", 20, 750},
	{StorageVendorHPE, "3PAR 8440", Family3PAROS, "ssd", 40, 2000},
	{StorageVendorHPE, "3PAR 9450", Family3PAROS, "ssd", 50, 6000},
	{StorageVendorHPE, "Alletra 6010", FamilyAlletraOS, "nvme-ssd", 23, 184},
	{StorageVendorHPE, "Alletra 6030", FamilyAlletraOS, "nvme-ssd", 46, 553},
	{StorageVendorHPE, "Alletra MP B10000", FamilyAlletraOS, "nvme-ssd", 46, 1105},
	{StorageVendorHPE, "Alletra MP X10000", FamilyAlletraOS, "nvme-ssd", 100, 5000},
	{StorageVendorHPE, "StoreOnce 3660", FamilyStoreOnceOS, "hdd", 36, 216},
	{StorageVendorHPE, "StoreOnce 5260", FamilyStoreOnceOS, "hdd", 108, 1080},
}

// Storage-fabric identifier formats. WWN/WWPN are 8-byte colon-separated hex;
// NAA is a type-6 (32-hex) name; IQN follows RFC 3720's iqn.YYYY-MM.domain:id.
var (
	storageWWNRE = regexp.MustCompile(`^([0-9a-f]{2}:){7}[0-9a-f]{2}$`)
	storageNAARE = regexp.MustCompile(`^naa\.6[0-9a-f]{31}$`)
	storageIQNRE = regexp.MustCompile(`^iqn\.\d{4}-\d{2}\.[a-z0-9.-]+:.+$`)
)

// maxDataReduction caps the plausible effective/usable capacity multiplier used
// by Validate; real dedup+compression rarely exceeds this.
const maxDataReduction = 10.0

// storageIQNDomain maps a vendor to its reverse-DNS naming authority for IQNs.
var storageIQNDomain = map[StorageVendor]string{
	StorageVendorHPE:     "com.hpe",
	StorageVendorNetApp:  "com.netapp",
	StorageVendorPure:    "com.purestorage",
	StorageVendorDellEMC: "com.dell",
}

// randomWWNLike returns an 8-byte colon-hex name with a fixed leading byte
// (0x50 for a node WWN, 0x20 for an FC port WWPN).
func randomWWNLike(r *rand.Rand, first byte) string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = byte(r.Intn(256)) // #nosec G404
	}
	b[0] = first
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7])
}

// randomNAA returns an NAA type-6 identifier: "naa.6" + 31 hex nibbles.
func randomNAA(r *rand.Rand) string {
	const hexch = "0123456789abcdef"
	var sb strings.Builder
	sb.WriteString("naa.6")
	for i := 0; i < 31; i++ {
		sb.WriteByte(hexch[r.Intn(16)]) // #nosec G404
	}
	return sb.String()
}

// storageSerial returns a vendor-prefixed uppercase serial, e.g. "HPE-1A2B3C4D5E".
func storageSerial(r *rand.Rand, vendor StorageVendor) string {
	return fmt.Sprintf("%s-%s", strings.ToUpper(string(vendor)), strings.ToUpper(randomHex(r, 5)))
}

// iqnFor builds an IQN from the vendor's naming authority and the array serial.
func iqnFor(vendor StorageVendor, serial string) string {
	domain := storageIQNDomain[vendor]
	if domain == "" {
		domain = "com.example"
	}
	return fmt.Sprintf("iqn.2007-11.%s:%s", domain, strings.ToLower(serial))
}

// driveTypeAndCapacity returns the concrete drive type and per-drive capacity
// (TB) for a model's predominant media.
func driveTypeAndCapacity(r *rand.Rand, driveType string) (string, float64) {
	switch driveType {
	case "nvme-ssd":
		caps := []float64{3.84, 7.68, 15.36}
		return "nvme-ssd", caps[r.Intn(len(caps))] // #nosec G404
	case "ssd":
		caps := []float64{1.92, 3.84, 7.68}
		return "ssd", caps[r.Intn(len(caps))] // #nosec G404
	default: // "hdd", "hybrid"
		caps := []float64{4, 8, 12, 16}
		return "hdd", caps[r.Intn(len(caps))] // #nosec G404
	}
}

// Validate reports whether the StorageSystemIdentity is well-formed: a
// family-coherent OS vendor, well-formed WWN/IQN/NAA/WWPN, and a sane capacity
// model. Returns an error rather than panicking, per the datagen error-return
// convention.
func (s *StorageSystemIdentity) Validate() error {
	if s.Vendor == "" {
		return fmt.Errorf("storage system vendor must not be empty")
	}
	if s.Model == "" {
		return fmt.Errorf("storage system model must not be empty")
	}
	if s.Serial == "" {
		return fmt.Errorf("storage system serial must not be empty")
	}
	if s.OS == nil {
		return fmt.Errorf("storage system %q has nil OS", s.Model)
	}
	if err := s.OS.Validate(); err != nil {
		return fmt.Errorf("storage system %q OS: %w", s.Model, err)
	}
	if !storageWWNRE.MatchString(s.WWN) {
		return fmt.Errorf("storage system %q has malformed WWN %q", s.Model, s.WWN)
	}
	if !storageNAARE.MatchString(s.NAA) {
		return fmt.Errorf("storage system %q has malformed NAA %q", s.Model, s.NAA)
	}
	if !storageIQNRE.MatchString(s.IQN) {
		return fmt.Errorf("storage system %q has malformed IQN %q", s.Model, s.IQN)
	}
	for _, p := range s.WWPN {
		if !storageWWNRE.MatchString(p) {
			return fmt.Errorf("storage system %q has malformed WWPN %q", s.Model, p)
		}
	}
	c := s.Capacity
	if c.RawCapacityTB <= 0 || c.UsableCapacityTB <= 0 {
		return fmt.Errorf("storage system %q has non-positive capacity", s.Model)
	}
	if c.UsableCapacityTB > c.RawCapacityTB {
		return fmt.Errorf("storage system %q usable %.1fTB exceeds raw %.1fTB", s.Model, c.UsableCapacityTB, c.RawCapacityTB)
	}
	if c.EffectiveCapacityTB < c.UsableCapacityTB {
		return fmt.Errorf("storage system %q effective %.1fTB below usable %.1fTB", s.Model, c.EffectiveCapacityTB, c.UsableCapacityTB)
	}
	if c.DataReductionRatio < 1 {
		return fmt.Errorf("storage system %q data reduction ratio %.2f is below 1", s.Model, c.DataReductionRatio)
	}
	if c.EffectiveCapacityTB > c.UsableCapacityTB*maxDataReduction {
		return fmt.Errorf("storage system %q effective %.1fTB implies reduction above %.0fx", s.Model, c.EffectiveCapacityTB, maxDataReduction)
	}
	return nil
}

// generateStorageSystem builds a StorageSystemIdentity for a specific model
// spec with deterministic output for a given RNG state.
func generateStorageSystem(r *rand.Rand, spec storageModelSpec) *StorageSystemIdentity {
	os := GenerateApplianceOS(r, spec.osFamily)
	serial := storageSerial(r, spec.vendor)

	// Capacity: usable is 72-85% of raw; effective applies a 2-8x reduction.
	raw := spec.minRawTB + r.Float64()*(spec.maxRawTB-spec.minRawTB) // #nosec G404
	usable := raw * (0.72 + r.Float64()*0.13)                        // #nosec G404
	reduction := 2.0 + r.Float64()*6.0                               // #nosec G404
	effective := usable * reduction

	// Controllers: active/standby HA pair.
	controllers := []StorageController{
		{ID: "ctrl-A", Serial: storageSerial(r, spec.vendor), Role: "active", FirmwareVersion: os.Version},
		{ID: "ctrl-B", Serial: storageSerial(r, spec.vendor), Role: "standby", FirmwareVersion: os.Version},
	}

	// Drives in a single shelf.
	const bays = 24
	dType, dCap := driveTypeAndCapacity(r, spec.driveType)
	nDrives := randRange(r, 12, bays)
	drives := make([]StorageDrive, nDrives)
	for i := range drives {
		drives[i] = StorageDrive{
			Slot:       fmt.Sprintf("shelf1-bay%02d", i+1),
			Type:       dType,
			CapacityTB: dCap,
			Model:      fmt.Sprintf("%s %.2fTB %s", strings.ToUpper(string(spec.vendor)), dCap, strings.ToUpper(dType)),
			Serial:     storageSerial(r, spec.vendor),
		}
	}
	shelves := []StorageShelf{{
		ID:        "shelf-01",
		Model:     spec.model + " DBE",
		DriveBays: bays,
		Drives:    drives,
	}}

	// FC ports: 2 or 4 WWPNs.
	nPorts := 2 * randRange(r, 1, 2)
	wwpn := make([]string, nPorts)
	for i := range wwpn {
		wwpn[i] = randomWWNLike(r, 0x20)
	}

	return &StorageSystemIdentity{
		Vendor: spec.vendor,
		Model:  spec.model,
		Serial: serial,
		OS:     &os,
		WWN:    randomWWNLike(r, 0x50),
		IQN:    iqnFor(spec.vendor, serial),
		WWPN:   wwpn,
		NAA:    randomNAA(r),
		Capacity: StorageCapacity{
			RawCapacityTB:       raw,
			UsableCapacityTB:    usable,
			EffectiveCapacityTB: effective,
			DataReductionRatio:  reduction,
		},
		Controllers: controllers,
		Shelves:     shelves,
		Drives:      drives,
		ManagementInterface: &NetworkInterface{
			Name:       "mgmt0",
			IPv4:       RandomPrivateIPv4(r),
			MACAddress: RandomMAC(r),
		},
	}
}

// RandomStorageSystemIdentity returns a storage array drawn at random from the
// built-in vendor pools (currently HPE).
func RandomStorageSystemIdentity(r *rand.Rand) *StorageSystemIdentity {
	spec := hpeStorageModels[r.Intn(len(hpeStorageModels))] // #nosec G404
	return generateStorageSystem(r, spec)
}

package catalog

// AssetCategory groups FIX SecurityType values (tag 167) by shared wire
// structure. Every SecurityType the FIX generator emits belongs to
// exactly one category; per-category modeling code lives in its own
// subpackage and handles all SecurityTypes within that category.
//
// The 10 categories here cover every SecurityType value the generator
// emits. There is no "Other" or "Unsupported" bucket — if a
// SecurityType is going to be emitted, it must have a home category.
type AssetCategory int

const (
	// AssetCategoryUnknown is the zero value; not a valid emission target.
	AssetCategoryUnknown AssetCategory = iota

	// AssetCategoryEquities — cash equities and equity-like instruments.
	// SecurityType ∈ {CS, PFD, ETF, MF, ADR, WAR, RGT}.
	AssetCategoryEquities

	// AssetCategoryFX — foreign exchange.
	// SecurityType ∈ {FOR, FXFWD, FXSWAP, FXNDF}.
	AssetCategoryFX

	// AssetCategoryFutures — listed futures including multi-leg combos.
	// SecurityType = FUT.
	AssetCategoryFutures

	// AssetCategoryOptions — listed options including multi-leg combos.
	// SecurityType = OPT.
	AssetCategoryOptions

	// AssetCategoryGovBonds — government / sovereign fixed income.
	// SecurityType ∈ {TBILL, TNOTE, TBOND, TIPS, TINT}.
	AssetCategoryGovBonds

	// AssetCategoryCorpBonds — corporate, convertible, and municipal credit.
	// SecurityType ∈ {CORP, CB, MUNI, MUNIFIDC, GO, REV}.
	AssetCategoryCorpBonds

	// AssetCategoryStructured — structured / securitized products.
	// SecurityType ∈ {ABS, MBS, TMBS, CMBS, CDO}.
	AssetCategoryStructured

	// AssetCategoryOTCDerivs — OTC derivatives.
	// SecurityType ∈ {IRS, CDS, BSWAP, VARSWAP, TRSWAP, XCS}.
	AssetCategoryOTCDerivs

	// AssetCategoryRepos — repurchase agreements.
	// SecurityType ∈ {REPO, REVREPO, HREPO}.
	AssetCategoryRepos

	// AssetCategoryMoneyMarket — short-maturity instruments.
	// SecurityType ∈ {CD, CP, BA, BN}.
	AssetCategoryMoneyMarket
)

// String returns a short label for the category. Used for logging,
// metrics labels, and config error messages.
func (a AssetCategory) String() string {
	switch a {
	case AssetCategoryEquities:
		return "equities"
	case AssetCategoryFX:
		return "fx"
	case AssetCategoryFutures:
		return "futures"
	case AssetCategoryOptions:
		return "options"
	case AssetCategoryGovBonds:
		return "govbonds"
	case AssetCategoryCorpBonds:
		return "corpbonds"
	case AssetCategoryStructured:
		return "structured"
	case AssetCategoryOTCDerivs:
		return "otcderivs"
	case AssetCategoryRepos:
		return "repos"
	case AssetCategoryMoneyMarket:
		return "moneymarket"
	default:
		return "unknown"
	}
}

// AllAssetCategories returns the categories in declaration order.
// Useful for test matrices and config validation.
func AllAssetCategories() []AssetCategory {
	return []AssetCategory{
		AssetCategoryEquities,
		AssetCategoryFX,
		AssetCategoryFutures,
		AssetCategoryOptions,
		AssetCategoryGovBonds,
		AssetCategoryCorpBonds,
		AssetCategoryStructured,
		AssetCategoryOTCDerivs,
		AssetCategoryRepos,
		AssetCategoryMoneyMarket,
	}
}

// SecurityType is the FIX tag-167 value (the on-the-wire string).
// Constants below enumerate every value the FIX generator emits, and
// Category() maps each to its owning AssetCategory.
type SecurityType string

// SecurityType constants — every value the generator can emit.
// Grouped by category for readability.
const (
	// Equities (AssetCategoryEquities)
	SecCS  SecurityType = "CS"  // Common Stock
	SecPFD SecurityType = "PFD" // Preferred Stock
	SecETF SecurityType = "ETF" // Exchange Traded Fund
	SecMF  SecurityType = "MF"  // Mutual Fund
	SecADR SecurityType = "ADR" // American Depositary Receipt
	SecWAR SecurityType = "WAR" // Warrant
	SecRGT SecurityType = "RGT" // Right

	// FX (AssetCategoryFX)
	SecFOR    SecurityType = "FOR"    // Foreign Exchange (spot)
	SecFXFWD  SecurityType = "FXFWD"  // FX Forward
	SecFXSWAP SecurityType = "FXSWAP" // FX Swap
	SecFXNDF  SecurityType = "FXNDF"  // FX Non-Deliverable Forward

	// Listed futures (AssetCategoryFutures)
	SecFUT SecurityType = "FUT" // Future

	// Listed options (AssetCategoryOptions)
	SecOPT SecurityType = "OPT" // Option

	// Government fixed income (AssetCategoryGovBonds)
	SecTBILL SecurityType = "TBILL" // US Treasury Bill
	SecTNOTE SecurityType = "TNOTE" // US Treasury Note
	SecTBOND SecurityType = "TBOND" // US Treasury Bond
	SecTIPS  SecurityType = "TIPS"  // Treasury Inflation Protected Security
	SecTINT  SecurityType = "TINT"  // Interest strip from a coupon-bearing bond

	// Corporate / credit fixed income (AssetCategoryCorpBonds)
	SecCORP     SecurityType = "CORP"     // Corporate Bond
	SecCB       SecurityType = "CB"       // Convertible Bond
	SecMUNI     SecurityType = "MUNI"     // Municipal Bond
	SecMUNIFIDC SecurityType = "MUNIFIDC" // Municipal FDIC
	SecGO       SecurityType = "GO"       // General Obligation Bond
	SecREV      SecurityType = "REV"      // Revenue Bond

	// Structured products (AssetCategoryStructured)
	SecABS  SecurityType = "ABS"  // Asset-Backed Security
	SecMBS  SecurityType = "MBS"  // Mortgage-Backed Security
	SecTMBS SecurityType = "TMBS" // TBA Mortgage-Backed Security
	SecCMBS SecurityType = "CMBS" // Commercial MBS
	SecCDO  SecurityType = "CDO"  // Collateralized Debt Obligation

	// OTC derivatives (AssetCategoryOTCDerivs)
	SecIRS     SecurityType = "IRS"     // Interest Rate Swap
	SecCDS     SecurityType = "CDS"     // Credit Default Swap
	SecBSWAP   SecurityType = "BSWAP"   // Basis Swap
	SecVARSWAP SecurityType = "VARSWAP" // Variance Swap
	SecTRSWAP  SecurityType = "TRSWAP"  // Total Return Swap
	SecXCS     SecurityType = "XCS"     // Cross-Currency Swap

	// Repos (AssetCategoryRepos)
	SecREPO    SecurityType = "REPO"    // Repurchase Agreement
	SecREVREPO SecurityType = "REVREPO" // Reverse Repo
	SecHREPO   SecurityType = "HREPO"   // Hold-in-Custody Repo

	// Money market (AssetCategoryMoneyMarket)
	SecCD SecurityType = "CD" // Certificate of Deposit
	SecCP SecurityType = "CP" // Commercial Paper
	SecBA SecurityType = "BA" // Banker's Acceptance
	SecBN SecurityType = "BN" // Banker's Note
)

// securityTypeCategory maps every SecurityType to its owning category.
// Populated by init() so test code can iterate it.
var securityTypeCategory = map[SecurityType]AssetCategory{
	SecCS: AssetCategoryEquities, SecPFD: AssetCategoryEquities,
	SecETF: AssetCategoryEquities, SecMF: AssetCategoryEquities,
	SecADR: AssetCategoryEquities, SecWAR: AssetCategoryEquities,
	SecRGT: AssetCategoryEquities,

	SecFOR: AssetCategoryFX, SecFXFWD: AssetCategoryFX,
	SecFXSWAP: AssetCategoryFX, SecFXNDF: AssetCategoryFX,

	SecFUT: AssetCategoryFutures,
	SecOPT: AssetCategoryOptions,

	SecTBILL: AssetCategoryGovBonds, SecTNOTE: AssetCategoryGovBonds,
	SecTBOND: AssetCategoryGovBonds, SecTIPS: AssetCategoryGovBonds,
	SecTINT: AssetCategoryGovBonds,

	SecCORP: AssetCategoryCorpBonds, SecCB: AssetCategoryCorpBonds,
	SecMUNI: AssetCategoryCorpBonds, SecMUNIFIDC: AssetCategoryCorpBonds,
	SecGO: AssetCategoryCorpBonds, SecREV: AssetCategoryCorpBonds,

	SecABS: AssetCategoryStructured, SecMBS: AssetCategoryStructured,
	SecTMBS: AssetCategoryStructured, SecCMBS: AssetCategoryStructured,
	SecCDO: AssetCategoryStructured,

	SecIRS: AssetCategoryOTCDerivs, SecCDS: AssetCategoryOTCDerivs,
	SecBSWAP: AssetCategoryOTCDerivs, SecVARSWAP: AssetCategoryOTCDerivs,
	SecTRSWAP: AssetCategoryOTCDerivs, SecXCS: AssetCategoryOTCDerivs,

	SecREPO: AssetCategoryRepos, SecREVREPO: AssetCategoryRepos,
	SecHREPO: AssetCategoryRepos,

	SecCD: AssetCategoryMoneyMarket, SecCP: AssetCategoryMoneyMarket,
	SecBA: AssetCategoryMoneyMarket, SecBN: AssetCategoryMoneyMarket,
}

// Category returns the AssetCategory the SecurityType belongs to, or
// AssetCategoryUnknown if the value isn't recognized.
func (s SecurityType) Category() AssetCategory {
	return securityTypeCategory[s]
}

// AllSecurityTypes returns the SecurityType values declared in this
// catalog, in a stable order. Useful for golden-output tests and for
// the StateTracker's per-instrument book initialization.
func AllSecurityTypes() []SecurityType {
	return []SecurityType{
		// Equities
		SecCS, SecPFD, SecETF, SecMF, SecADR, SecWAR, SecRGT,
		// FX
		SecFOR, SecFXFWD, SecFXSWAP, SecFXNDF,
		// Futures
		SecFUT,
		// Options
		SecOPT,
		// GovBonds
		SecTBILL, SecTNOTE, SecTBOND, SecTIPS, SecTINT,
		// CorpBonds
		SecCORP, SecCB, SecMUNI, SecMUNIFIDC, SecGO, SecREV,
		// Structured
		SecABS, SecMBS, SecTMBS, SecCMBS, SecCDO,
		// OTC Derivs
		SecIRS, SecCDS, SecBSWAP, SecVARSWAP, SecTRSWAP, SecXCS,
		// Repos
		SecREPO, SecREVREPO, SecHREPO,
		// Money Market
		SecCD, SecCP, SecBA, SecBN,
	}
}

// SecurityTypesByCategory returns the SecurityTypes that belong to the
// given category, in declaration order. Returns nil for
// AssetCategoryUnknown or unrecognized categories.
func SecurityTypesByCategory(cat AssetCategory) []SecurityType {
	var out []SecurityType
	for _, st := range AllSecurityTypes() {
		if st.Category() == cat {
			out = append(out, st)
		}
	}
	return out
}

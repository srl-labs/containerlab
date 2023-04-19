package types

// LicensePolicy is a value of LicensePolicy.
type LicensePolicy string

const (
	// LicensePolicyRequired means a node should exit if no license provided.
	LicensePolicyRequired = "required"
	// LicensePolicyWarn means a node should warn (but not exit) if no license provided.
	LicensePolicyWarn = "warn"
	// LicensePolicyNone means a node doesn't care about a license.
	LicensePolicyNone = "none"
)

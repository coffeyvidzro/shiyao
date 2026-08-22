package polar

import "context"

// Client provides access to the Polar API used by Shiyao.
type Client interface {
	GetLicenseKey(context.Context, string) (*LicenseKey, error)
	ValidateLicenseKey(context.Context, ValidateLicenseKeyRequest) (*ValidatedLicenseKey, error)
}

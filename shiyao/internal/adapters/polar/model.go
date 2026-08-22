package polar

import (
	"time"

	"github.com/google/uuid"
)

// LicenseKey represents the Polar license-key resource used by Shiyao.
type LicenseKey struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	CustomerID       uuid.UUID
	BenefitID        uuid.UUID
	Key              string
	DisplayKey       string
	Status           LicenseKeyStatus
	LimitActivations *int
	Usage            int
	LimitUsage       *int
	Validations      int
	LastValidatedAt  *time.Time
	ExpiresAt        *time.Time
	Activations      []LicenseKeyActivation
}

type LicenseKeyStatus string

const (
	LicenseKeyStatusGranted  LicenseKeyStatus = "granted"
	LicenseKeyStatusRevoked  LicenseKeyStatus = "revoked"
	LicenseKeyStatusDisabled LicenseKeyStatus = "disabled"
)

type LicenseKeyActivation struct {
	ID           uuid.UUID
	LicenseKeyID uuid.UUID
	Label        string
	Meta         map[string]any
	CreatedAt    time.Time
	ModifiedAt   *time.Time
}

// ValidateLicenseKeyRequest contains the fields accepted by Polar's license
// validation endpoint. Optional fields are pointers so omitted values remain
// distinguishable from explicit zero values.
type ValidateLicenseKeyRequest struct {
	Key            string
	OrganizationID uuid.UUID
	ActivationID   *uuid.UUID
	BenefitID      *uuid.UUID
	CustomerID     *uuid.UUID
	IncrementUsage *int
	Conditions     map[string]any
}

// ValidatedLicenseKey represents the response returned by Polar after
// validating a license key.
type ValidatedLicenseKey struct {
	ID               uuid.UUID
	CreatedAt        time.Time
	ModifiedAt       *time.Time
	OrganizationID   uuid.UUID
	CustomerID       uuid.UUID
	BenefitID        uuid.UUID
	Key              string
	DisplayKey       string
	Status           LicenseKeyStatus
	LimitActivations *int
	Usage            int
	LimitUsage       *int
	Validations      int
	LastValidatedAt  *time.Time
	ExpiresAt        *time.Time
	Activation       *LicenseKeyActivation
}

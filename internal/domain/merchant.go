package domain

import (
	"errors"
	"time"
)

var ErrMerchantAlreadyExists = errors.New("merchant already exists")
var ErrInvalidMerchantCredentials = errors.New("invalid merchant credentials")
var ErrMerchantNotFound = errors.New("merchant not found")

type MerchantOnboardingStatus struct {
	Merchant             Merchant                 `json:"merchant"`
	Issuer               *AppleWalletIssuer       `json:"issuer,omitempty"`
	PassType             *ApplePassType           `json:"pass_type,omitempty"`
	Certificate          *AppleSigningCertificate `json:"certificate,omitempty"`
	Design               *ApplePassDesign         `json:"design,omitempty"`
	PaymentComplete      bool                     `json:"payment_complete"`
	IssuerConfigured     bool                     `json:"issuer_configured"`
	PassTypeConfigured   bool                     `json:"pass_type_configured"`
	CertificateValidated bool                     `json:"certificate_validated"`
	AssetsUploaded       bool                     `json:"assets_uploaded"`
	DesignConfigured     bool                     `json:"design_configured"`
	CanActivate          bool                     `json:"can_activate"`
	OnboardingStatus     string                   `json:"onboarding_status"`
	Errors               []string                 `json:"errors"`
}

type Merchant struct {
	MerchantID    string    `json:"merchant_id"`
	CreationDate  time.Time `json:"creation_date"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Merchant      string    `json:"merchant"`
	MerchantSlug  string    `json:"merchant_slug"`
	MerchantEmail string    `json:"merchant_email"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"payment_status"`
}

type CreateMerchantInput struct {
	MerchantID    string
	FirstName     string
	LastName      string
	Company       string
	MerchantSlug  string
	MerchantEmail string
	PasswordHash  string
}

type MerchantCredentials struct {
	MerchantID    string
	FirstName     string
	LastName      string
	Merchant      string
	MerchantSlug  string
	MerchantEmail string
	PasswordHash  string
	Status        string
	PaymentStatus string
}

func (m MerchantCredentials) PublicMerchant() Merchant {
	return Merchant{
		MerchantID:    m.MerchantID,
		FirstName:     m.FirstName,
		LastName:      m.LastName,
		Merchant:      m.Merchant,
		MerchantSlug:  m.MerchantSlug,
		MerchantEmail: m.MerchantEmail,
		Status:        m.Status,
		PaymentStatus: m.PaymentStatus,
	}
}

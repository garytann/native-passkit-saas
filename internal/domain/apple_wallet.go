package domain

import (
	"errors"
	"time"
)

var ErrAppleWalletIssuerNotFound = errors.New("apple wallet issuer not found")
var ErrAppleWalletPassTypeNotFound = errors.New("apple wallet pass type not found")
var ErrAppleWalletCertificateNotFound = errors.New("apple wallet certificate not found")
var ErrAppleWalletNotProvisioned = errors.New("apple wallet is not provisioned for merchant")
var ErrAppleWalletConfigurationConflict = errors.New("apple wallet configuration already exists")

type AppleWalletIssuer struct {
	IssuerID         string    `json:"issuer_id"`
	MerchantID       string    `json:"merchant_id"`
	OwnershipMode    string    `json:"ownership_mode"`
	TeamIdentifier   string    `json:"team_identifier"`
	OrganizationName string    `json:"organization_name"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateAppleWalletIssuerInput struct {
	IssuerID         string
	MerchantID       string
	OwnershipMode    string
	TeamIdentifier   string
	OrganizationName string
	Status           string
}

type UpdateAppleWalletIssuerInput struct {
	IssuerID         string
	MerchantID       string
	OwnershipMode    string
	TeamIdentifier   string
	OrganizationName string
}

type ApplePassType struct {
	ApplePassTypeID    string    `json:"apple_pass_type_id"`
	IssuerID           string    `json:"issuer_id"`
	PassTypeIdentifier string    `json:"pass_type_identifier"`
	PassStyle          string    `json:"pass_style"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateApplePassTypeInput struct {
	ApplePassTypeID    string
	MerchantID         string
	PassTypeIdentifier string
	PassStyle          string
}

type AppleSigningCertificate struct {
	CertificateID          string     `json:"certificate_id"`
	ApplePassTypeID        string     `json:"apple_pass_type_id"`
	CertificateRef         string     `json:"certificate_ref"`
	PrivateKeyRef          string     `json:"private_key_ref"`
	CertificateFingerprint string     `json:"certificate_fingerprint"`
	CertificateSerial      string     `json:"certificate_serial_number"`
	ValidFrom              time.Time  `json:"valid_from"`
	ExpiresAt              time.Time  `json:"expires_at"`
	Status                 string     `json:"status"`
	CreatedAt              time.Time  `json:"created_at"`
	ActivatedAt            *time.Time `json:"activated_at,omitempty"`
}

type ActivateAppleSigningCertificateInput struct {
	CertificateID          string
	MerchantID             string
	PassTypeIdentifier     string
	CertificateRef         string
	PrivateKeyRef          string
	CertificateFingerprint string
	CertificateSerial      string
	ValidFrom              time.Time
	ExpiresAt              time.Time
}

type AppleWalletConfiguration struct {
	Issuer       AppleWalletIssuer         `json:"issuer"`
	PassTypes    []ApplePassType           `json:"pass_types"`
	Certificates []AppleSigningCertificate `json:"certificates"`
}

type ActiveAppleWalletCredential struct {
	MerchantID             string
	IssuerID               string
	ApplePassTypeID        string
	CertificateID          string
	OrganizationName       string
	TeamIdentifier         string
	PassTypeIdentifier     string
	PassStyle              string
	CertificateRef         string
	PrivateKeyRef          string
	CertificateFingerprint string
	ExpiresAt              time.Time
}

func NewAppleWalletIssuerID() (string, error) {
	return newReadableID("AWI-")
}

func NewApplePassTypeID() (string, error) {
	return newReadableID("APT-")
}

func NewAppleCertificateID() (string, error) {
	return newReadableID("CRT-")
}

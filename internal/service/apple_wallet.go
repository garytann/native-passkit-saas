package service

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

type AppleWalletRepository interface {
	GetIssuerByMerchantID(ctx context.Context, merchantID string) (domain.AppleWalletIssuer, error)
	UpdateIssuer(ctx context.Context, input domain.UpdateAppleWalletIssuerInput) (domain.AppleWalletIssuer, error)
	CreatePassType(ctx context.Context, input domain.CreateApplePassTypeInput) (domain.ApplePassType, error)
	ActivateCertificate(ctx context.Context, input domain.ActivateAppleSigningCertificateInput) (domain.AppleSigningCertificate, error)
	ConfigurationByMerchantID(ctx context.Context, merchantID string) (domain.AppleWalletConfiguration, error)
	ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error)
	ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error)
}

type AppleWalletCredentialResolver interface {
	ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error)
	ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error)
}

type AppleWalletService struct {
	repo                AppleWalletRepository
	certificateBasePath string
	wwdrCertPath        string
	timeout             time.Duration
}

type ConfigureAppleWalletIssuerInput struct {
	MerchantID       string
	OwnershipMode    string
	TeamIdentifier   string
	OrganizationName string
}

type CreateApplePassTypeRequest struct {
	MerchantID         string
	PassTypeIdentifier string
	PassStyle          string
}

type ProvisionAppleCertificateRequest struct {
	MerchantID         string
	PassTypeIdentifier string
	CredentialAlias    string
}

var credentialAliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var passTypeIdentifierPattern = regexp.MustCompile(`^pass\.[A-Za-z0-9.-]+$`)
var subjectUIDOID = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}

func NewAppleWalletService(cfg config.Config, repo AppleWalletRepository) AppleWalletService {
	return AppleWalletService{
		repo:                repo,
		certificateBasePath: strings.TrimSpace(cfg.CertificateBasePath),
		wwdrCertPath:        strings.TrimSpace(cfg.WWDRCertPath),
		timeout:             5 * time.Second,
	}
}

func (s AppleWalletService) Configuration(ctx context.Context, merchantID string) (domain.AppleWalletConfiguration, error) {
	if s.repo == nil {
		return domain.AppleWalletConfiguration{}, ErrRepositoryUnavailable
	}
	merchantID = strings.TrimSpace(merchantID)
	if merchantID == "" {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: "merchant id is required"}
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repo.ConfigurationByMerchantID(ctx, merchantID)
}

func (s AppleWalletService) ConfigureIssuer(ctx context.Context, input ConfigureAppleWalletIssuerInput) (domain.AppleWalletIssuer, error) {
	if s.repo == nil {
		return domain.AppleWalletIssuer{}, ErrRepositoryUnavailable
	}
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.OwnershipMode = strings.TrimSpace(input.OwnershipMode)
	input.TeamIdentifier = strings.TrimSpace(input.TeamIdentifier)
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	if input.OwnershipMode == "" {
		input.OwnershipMode = "platform_managed"
	}
	switch {
	case input.MerchantID == "":
		return domain.AppleWalletIssuer{}, ValidationError{Message: "merchant id is required"}
	case input.OwnershipMode != "platform_managed" && input.OwnershipMode != "merchant_managed":
		return domain.AppleWalletIssuer{}, ValidationError{Message: "ownership_mode must be platform_managed or merchant_managed"}
	case input.TeamIdentifier == "":
		return domain.AppleWalletIssuer{}, ValidationError{Message: "team_identifier is required"}
	case input.OrganizationName == "":
		return domain.AppleWalletIssuer{}, ValidationError{Message: "organization_name is required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	issuerID, err := domain.NewAppleWalletIssuerID()
	if err != nil {
		return domain.AppleWalletIssuer{}, err
	}
	return s.repo.UpdateIssuer(ctx, domain.UpdateAppleWalletIssuerInput{
		IssuerID:         issuerID,
		MerchantID:       input.MerchantID,
		OwnershipMode:    input.OwnershipMode,
		TeamIdentifier:   input.TeamIdentifier,
		OrganizationName: input.OrganizationName,
	})
}

func (s AppleWalletService) CreatePassType(ctx context.Context, input CreateApplePassTypeRequest) (domain.ApplePassType, error) {
	if s.repo == nil {
		return domain.ApplePassType{}, ErrRepositoryUnavailable
	}
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.PassTypeIdentifier = strings.TrimSpace(input.PassTypeIdentifier)
	input.PassStyle = strings.TrimSpace(input.PassStyle)
	if input.PassStyle == "" {
		input.PassStyle = "storeCard"
	}
	switch {
	case input.MerchantID == "":
		return domain.ApplePassType{}, ValidationError{Message: "merchant id is required"}
	case !passTypeIdentifierPattern.MatchString(input.PassTypeIdentifier):
		return domain.ApplePassType{}, ValidationError{Message: "pass_type_identifier must start with pass. and contain only letters, numbers, dots, or hyphens"}
	case input.PassStyle != "storeCard" && input.PassStyle != "coupon" && input.PassStyle != "generic":
		return domain.ApplePassType{}, ValidationError{Message: "pass_style must be storeCard, coupon, or generic"}
	}

	passTypeID, err := domain.NewApplePassTypeID()
	if err != nil {
		return domain.ApplePassType{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repo.CreatePassType(ctx, domain.CreateApplePassTypeInput{
		ApplePassTypeID:    passTypeID,
		MerchantID:         input.MerchantID,
		PassTypeIdentifier: input.PassTypeIdentifier,
		PassStyle:          input.PassStyle,
	})
}

func (s AppleWalletService) ProvisionCertificate(ctx context.Context, input ProvisionAppleCertificateRequest) (domain.AppleSigningCertificate, error) {
	if s.repo == nil {
		return domain.AppleSigningCertificate{}, ErrRepositoryUnavailable
	}
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.PassTypeIdentifier = strings.TrimSpace(input.PassTypeIdentifier)
	input.CredentialAlias = strings.TrimSpace(strings.ToLower(input.CredentialAlias))
	switch {
	case input.MerchantID == "":
		return domain.AppleSigningCertificate{}, ValidationError{Message: "merchant id is required"}
	case !passTypeIdentifierPattern.MatchString(input.PassTypeIdentifier):
		return domain.AppleSigningCertificate{}, ValidationError{Message: "pass_type_identifier is invalid"}
	case !credentialAliasPattern.MatchString(input.CredentialAlias):
		return domain.AppleSigningCertificate{}, ValidationError{Message: "credential_alias must contain lowercase letters, numbers, or hyphens"}
	case s.certificateBasePath == "":
		return domain.AppleSigningCertificate{}, errors.New("apple certificate base path is not configured")
	case s.wwdrCertPath == "":
		return domain.AppleSigningCertificate{}, errors.New("Apple WWDR certificate path is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	issuer, err := s.repo.GetIssuerByMerchantID(ctx, input.MerchantID)
	if err != nil {
		return domain.AppleSigningCertificate{}, err
	}

	credentialDir := filepath.Join(s.certificateBasePath, input.CredentialAlias)
	certificateRef := filepath.Join(credentialDir, "pass-cert.pem")
	privateKeyRef := filepath.Join(credentialDir, "pass-key.pem")
	metadata, err := validateAppleCertificate(certificateRef, privateKeyRef, s.wwdrCertPath, input.PassTypeIdentifier, issuer.TeamIdentifier, time.Now().UTC())
	if err != nil {
		return domain.AppleSigningCertificate{}, ValidationError{Message: err.Error()}
	}

	certificateID, err := domain.NewAppleCertificateID()
	if err != nil {
		return domain.AppleSigningCertificate{}, err
	}
	return s.repo.ActivateCertificate(ctx, domain.ActivateAppleSigningCertificateInput{
		CertificateID:          certificateID,
		MerchantID:             input.MerchantID,
		PassTypeIdentifier:     input.PassTypeIdentifier,
		CertificateRef:         certificateRef,
		PrivateKeyRef:          privateKeyRef,
		CertificateFingerprint: metadata.Fingerprint,
		CertificateSerial:      metadata.SerialNumber,
		ValidFrom:              metadata.ValidFrom,
		ExpiresAt:              metadata.ExpiresAt,
	})
}

func (s AppleWalletService) ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error) {
	if s.repo == nil {
		return domain.ActiveAppleWalletCredential{}, ErrRepositoryUnavailable
	}
	return s.repo.ResolveActiveByMerchant(ctx, strings.TrimSpace(merchant))
}

func (s AppleWalletService) ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error) {
	if s.repo == nil {
		return domain.ActiveAppleWalletCredential{}, ErrRepositoryUnavailable
	}
	return s.repo.ResolveActiveByPassType(ctx, strings.TrimSpace(passTypeIdentifier))
}

type appleCertificateMetadata struct {
	Fingerprint  string
	SerialNumber string
	ValidFrom    time.Time
	ExpiresAt    time.Time
}

func validateAppleCertificate(certificatePath string, privateKeyPath string, wwdrPath string, expectedPassTypeID string, expectedTeamID string, now time.Time) (appleCertificateMetadata, error) {
	certificate, certificateDER, err := loadCertificate(certificatePath)
	if err != nil {
		return appleCertificateMetadata{}, fmt.Errorf("load pass certificate: %w", err)
	}
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return appleCertificateMetadata{}, fmt.Errorf("load pass private key: %w", err)
	}
	if err := publicKeysMatch(certificate.PublicKey, privateKey.Public()); err != nil {
		return appleCertificateMetadata{}, err
	}

	certificatePassTypeID := certificateSubjectUID(certificate)
	if certificatePassTypeID != expectedPassTypeID {
		return appleCertificateMetadata{}, fmt.Errorf("certificate Pass Type ID %q does not match %q", certificatePassTypeID, expectedPassTypeID)
	}
	if expectedTeamID == "" {
		return appleCertificateMetadata{}, errors.New("issuer team identifier is not configured")
	}
	if !containsString(certificate.Subject.OrganizationalUnit, expectedTeamID) {
		return appleCertificateMetadata{}, fmt.Errorf("certificate Team ID does not match %q", expectedTeamID)
	}
	if now.Before(certificate.NotBefore) {
		return appleCertificateMetadata{}, fmt.Errorf("certificate is not valid until %s", certificate.NotBefore.UTC().Format(time.RFC3339))
	}
	if !now.Before(certificate.NotAfter) {
		return appleCertificateMetadata{}, fmt.Errorf("certificate expired at %s", certificate.NotAfter.UTC().Format(time.RFC3339))
	}

	wwdr, _, err := loadCertificate(wwdrPath)
	if err != nil {
		return appleCertificateMetadata{}, fmt.Errorf("load Apple WWDR certificate: %w", err)
	}
	if err := certificate.CheckSignatureFrom(wwdr); err != nil {
		return appleCertificateMetadata{}, fmt.Errorf("pass certificate is not signed by configured Apple WWDR certificate: %w", err)
	}

	fingerprint := sha256.Sum256(certificateDER)
	return appleCertificateMetadata{
		Fingerprint:  strings.ToUpper(hex.EncodeToString(fingerprint[:])),
		SerialNumber: strings.ToUpper(certificate.SerialNumber.Text(16)),
		ValidFrom:    certificate.NotBefore.UTC(),
		ExpiresAt:    certificate.NotAfter.UTC(),
	}, nil
}

func loadCertificate(path string) (*x509.Certificate, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	der := data
	if block, _ := pem.Decode(data); block != nil {
		if block.Type != "CERTIFICATE" {
			return nil, nil, fmt.Errorf("unexpected PEM block %q", block.Type)
		}
		der = block.Bytes
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return certificate, der, nil
}

func loadPrivateKey(path string) (crypto.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("private key is not PEM encoded")
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			return signer, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported or encrypted private key")
}

func publicKeysMatch(certificateKey any, privateKey crypto.PublicKey) error {
	certificateDER, err := x509.MarshalPKIXPublicKey(certificateKey)
	if err != nil {
		return fmt.Errorf("encode certificate public key: %w", err)
	}
	privateDER, err := x509.MarshalPKIXPublicKey(privateKey)
	if err != nil {
		return fmt.Errorf("encode private key public component: %w", err)
	}
	if !strings.EqualFold(hex.EncodeToString(certificateDER), hex.EncodeToString(privateDER)) {
		return errors.New("private key does not match pass certificate")
	}
	return nil
}

func certificateSubjectUID(certificate *x509.Certificate) string {
	for _, name := range certificate.Subject.Names {
		if name.Type.Equal(subjectUIDOID) {
			if value, ok := name.Value.(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

var _ crypto.Signer = (*rsa.PrivateKey)(nil)
var _ crypto.Signer = (*ecdsa.PrivateKey)(nil)

package service

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"net/url"
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
	SaveValidatedCertificate(ctx context.Context, input domain.ActivateAppleSigningCertificateInput) (domain.AppleSigningCertificate, error)
	ActivateConfiguration(ctx context.Context, merchantID string, passTypeIdentifier string) error
	ConfigurationByMerchantID(ctx context.Context, merchantID string) (domain.AppleWalletConfiguration, error)
	ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error)
	ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error)
	UpsertPassDesign(ctx context.Context, input domain.UpsertApplePassDesignInput) (domain.ApplePassDesign, error)
	GetPassDesign(ctx context.Context, merchantID string, passTypeIdentifier string) (domain.ApplePassDesign, error)
	ResolveActivePassDesign(ctx context.Context, applePassTypeID string) (domain.ApplePassDesign, error)
}

type AppleWalletCredentialResolver interface {
	ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error)
	ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error)
}

type ApplePassDesignResolver interface {
	ResolveActivePassDesign(ctx context.Context, applePassTypeID string) (domain.ApplePassDesign, error)
}

type AppleWalletService struct {
	repo                AppleWalletRepository
	certificateBasePath string
	assetBasePath       string
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

type UpsertApplePassDesignRequest struct {
	MerchantID          string
	PassTypeIdentifier  string
	ProgramName         string
	PassDescription     string
	StampGoal           int
	BackgroundColor     string
	ForegroundColor     string
	LabelColor          string
	AssetBundleRef      string
	StampInstructions   string
	RewardTitle         string
	RewardDetails       string
	SupportText         string
	TermsText           string
	Links               []ApplePassLinkRequest
	CampaignDisplayMode string
}

type ApplePassLinkRequest struct {
	LinkType string `json:"link_type"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

var credentialAliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var passTypeIdentifierPattern = regexp.MustCompile(`^pass\.[A-Za-z0-9.-]+$`)
var hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var subjectUIDOID = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}

func NewAppleWalletService(cfg config.Config, repo AppleWalletRepository) AppleWalletService {
	return AppleWalletService{
		repo:                repo,
		certificateBasePath: strings.TrimSpace(cfg.CertificateBasePath),
		assetBasePath:       strings.TrimSpace(cfg.PassAssetBasePath),
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
	return s.repo.SaveValidatedCertificate(ctx, domain.ActivateAppleSigningCertificateInput{
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

func (s AppleWalletService) Activate(ctx context.Context, merchantID string, passTypeIdentifier string) (domain.AppleWalletConfiguration, error) {
	if s.repo == nil {
		return domain.AppleWalletConfiguration{}, ErrRepositoryUnavailable
	}
	merchantID = strings.TrimSpace(merchantID)
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	if merchantID == "" {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: "merchant id is required"}
	}
	if !passTypeIdentifierPattern.MatchString(passTypeIdentifier) {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: "pass_type_identifier is invalid"}
	}

	configuration, err := s.repo.ConfigurationByMerchantID(ctx, merchantID)
	if err != nil {
		return domain.AppleWalletConfiguration{}, err
	}
	design, ok := designForPassType(configuration, passTypeIdentifier)
	if !ok {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: "pass design is required before activation"}
	}
	if design.AssetBundleRef == "" {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: "pass assets are required before activation"}
	}
	if err := validatePassAssetBundle(filepath.Join(s.assetBasePath, design.AssetBundleRef)); err != nil {
		return domain.AppleWalletConfiguration{}, ValidationError{Message: err.Error()}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.repo.ActivateConfiguration(ctx, merchantID, passTypeIdentifier); err != nil {
		return domain.AppleWalletConfiguration{}, err
	}
	return s.repo.ConfigurationByMerchantID(ctx, merchantID)
}

func designForPassType(configuration domain.AppleWalletConfiguration, passTypeIdentifier string) (domain.ApplePassDesign, bool) {
	for _, passType := range configuration.PassTypes {
		if passType.PassTypeIdentifier != passTypeIdentifier {
			continue
		}
		for _, design := range configuration.Designs {
			if design.ApplePassTypeID == passType.ApplePassTypeID {
				return design, true
			}
		}
	}
	return domain.ApplePassDesign{}, false
}

func (s AppleWalletService) PassDesign(ctx context.Context, merchantID string, passTypeIdentifier string) (domain.ApplePassDesign, error) {
	if s.repo == nil {
		return domain.ApplePassDesign{}, ErrRepositoryUnavailable
	}
	merchantID = strings.TrimSpace(merchantID)
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	if merchantID == "" {
		return domain.ApplePassDesign{}, ValidationError{Message: "merchant id is required"}
	}
	if !passTypeIdentifierPattern.MatchString(passTypeIdentifier) {
		return domain.ApplePassDesign{}, ValidationError{Message: "pass_type_identifier is invalid"}
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repo.GetPassDesign(ctx, merchantID, passTypeIdentifier)
}

func (s AppleWalletService) UpsertPassDesign(ctx context.Context, input UpsertApplePassDesignRequest) (domain.ApplePassDesign, error) {
	if s.repo == nil {
		return domain.ApplePassDesign{}, ErrRepositoryUnavailable
	}
	normalizePassDesignRequest(&input)
	if err := validatePassDesignRequest(input); err != nil {
		return domain.ApplePassDesign{}, err
	}
	existing, err := s.repo.GetPassDesign(ctx, input.MerchantID, input.PassTypeIdentifier)
	if err == nil {
		if err := validateStampGoalChange(existing, input.StampGoal); err != nil {
			return domain.ApplePassDesign{}, err
		}
	} else if !errors.Is(err, domain.ErrApplePassDesignNotFound) {
		return domain.ApplePassDesign{}, err
	}
	if input.AssetBundleRef != "" {
		if s.assetBasePath == "" {
			return domain.ApplePassDesign{}, ValidationError{Message: "apple pass asset base path is not configured"}
		}
		if err := validatePassAssetBundle(filepath.Join(s.assetBasePath, input.AssetBundleRef)); err != nil {
			return domain.ApplePassDesign{}, ValidationError{Message: err.Error()}
		}
	}

	designID, err := domain.NewApplePassDesignID()
	if err != nil {
		return domain.ApplePassDesign{}, err
	}
	links := make([]domain.UpsertApplePassLinkInput, 0, len(input.Links))
	for index, link := range input.Links {
		linkID, err := domain.NewApplePassLinkID()
		if err != nil {
			return domain.ApplePassDesign{}, err
		}
		links = append(links, domain.UpsertApplePassLinkInput{
			LinkID:    linkID,
			LinkType:  link.LinkType,
			Label:     link.Label,
			URL:       link.URL,
			SortOrder: index,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repo.UpsertPassDesign(ctx, domain.UpsertApplePassDesignInput{
		DesignID:            designID,
		MerchantID:          input.MerchantID,
		PassTypeIdentifier:  input.PassTypeIdentifier,
		ProgramName:         input.ProgramName,
		PassDescription:     input.PassDescription,
		StampGoal:           input.StampGoal,
		BackgroundColor:     strings.ToUpper(input.BackgroundColor),
		ForegroundColor:     strings.ToUpper(input.ForegroundColor),
		LabelColor:          strings.ToUpper(input.LabelColor),
		AssetBundleRef:      input.AssetBundleRef,
		StampInstructions:   input.StampInstructions,
		RewardTitle:         input.RewardTitle,
		RewardDetails:       input.RewardDetails,
		SupportText:         input.SupportText,
		TermsText:           input.TermsText,
		Links:               links,
		CampaignDisplayMode: input.CampaignDisplayMode,
	})
}

func (s AppleWalletService) ResolveActivePassDesign(ctx context.Context, applePassTypeID string) (domain.ApplePassDesign, error) {
	if s.repo == nil {
		return domain.ApplePassDesign{}, ErrRepositoryUnavailable
	}
	return s.repo.ResolveActivePassDesign(ctx, strings.TrimSpace(applePassTypeID))
}

func (s AppleWalletService) StampGoalForMerchant(ctx context.Context, merchant string) (int, error) {
	credential, err := s.ResolveActiveByMerchant(ctx, merchant)
	if err != nil {
		return 0, err
	}
	design, err := s.ResolveActivePassDesign(ctx, credential.ApplePassTypeID)
	if err != nil {
		return 0, err
	}
	return design.StampGoal, nil
}

func (s AppleWalletService) RewardTitleForMerchant(ctx context.Context, merchant string) (string, error) {
	credential, err := s.ResolveActiveByMerchant(ctx, merchant)
	if err != nil {
		return "", err
	}
	design, err := s.ResolveActivePassDesign(ctx, credential.ApplePassTypeID)
	if err != nil {
		return "", err
	}
	return design.RewardTitle, nil
}

func normalizePassDesignRequest(input *UpsertApplePassDesignRequest) {
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.PassTypeIdentifier = strings.TrimSpace(input.PassTypeIdentifier)
	input.ProgramName = strings.TrimSpace(input.ProgramName)
	input.PassDescription = strings.TrimSpace(input.PassDescription)
	input.BackgroundColor = strings.TrimSpace(input.BackgroundColor)
	input.ForegroundColor = strings.TrimSpace(input.ForegroundColor)
	input.LabelColor = strings.TrimSpace(input.LabelColor)
	input.AssetBundleRef = strings.TrimSpace(strings.ToLower(input.AssetBundleRef))
	input.StampInstructions = strings.TrimSpace(input.StampInstructions)
	input.RewardTitle = strings.TrimSpace(input.RewardTitle)
	input.RewardDetails = strings.TrimSpace(input.RewardDetails)
	input.SupportText = strings.TrimSpace(input.SupportText)
	input.TermsText = strings.TrimSpace(input.TermsText)
	input.CampaignDisplayMode = strings.TrimSpace(strings.ToLower(input.CampaignDisplayMode))
	if input.StampGoal == 0 {
		input.StampGoal = 10
	}
	if input.BackgroundColor == "" {
		input.BackgroundColor = "#5434F5"
	}
	if input.ForegroundColor == "" {
		input.ForegroundColor = "#FFFFFF"
	}
	if input.LabelColor == "" {
		input.LabelColor = "#E1DCFF"
	}
	if input.SupportText == "" {
		input.SupportText = "Contact the merchant for loyalty card assistance."
	}
	if input.StampInstructions == "" {
		input.StampInstructions = "Present this pass when making an eligible purchase to collect a stamp."
	}
	if input.RewardTitle == "" {
		input.RewardTitle = "Loyalty Reward"
	}
	if input.RewardDetails == "" {
		input.RewardDetails = "Complete your stamp card to receive the merchant reward."
	}
	if input.TermsText == "" {
		input.TermsText = "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms."
	}
	if input.CampaignDisplayMode == "" {
		input.CampaignDisplayMode = "details"
	}
	for index := range input.Links {
		input.Links[index].LinkType = strings.ToLower(strings.TrimSpace(input.Links[index].LinkType))
		input.Links[index].Label = strings.TrimSpace(input.Links[index].Label)
		input.Links[index].URL = strings.TrimSpace(input.Links[index].URL)
	}
}

func validatePassDesignRequest(input UpsertApplePassDesignRequest) error {
	switch {
	case input.MerchantID == "":
		return ValidationError{Message: "merchant id is required"}
	case !passTypeIdentifierPattern.MatchString(input.PassTypeIdentifier):
		return ValidationError{Message: "pass_type_identifier is invalid"}
	case input.ProgramName == "":
		return ValidationError{Message: "program_name is required"}
	case len(input.ProgramName) > 80:
		return ValidationError{Message: "program_name must be 80 characters or fewer"}
	case input.PassDescription == "":
		return ValidationError{Message: "pass_description is required"}
	case len(input.PassDescription) > 160:
		return ValidationError{Message: "pass_description must be 160 characters or fewer"}
	case input.StampGoal < 1 || input.StampGoal > 100:
		return ValidationError{Message: "stamp_goal must be between 1 and 100"}
	case !hexColorPattern.MatchString(input.BackgroundColor), !hexColorPattern.MatchString(input.ForegroundColor), !hexColorPattern.MatchString(input.LabelColor):
		return ValidationError{Message: "pass colors must use six-digit hex values such as #5434F5"}
	case input.AssetBundleRef != "" && !credentialAliasPattern.MatchString(input.AssetBundleRef):
		return ValidationError{Message: "asset_bundle_ref must contain lowercase letters, numbers, or hyphens"}
	case len(input.StampInstructions) > 500:
		return ValidationError{Message: "stamp_instructions must be 500 characters or fewer"}
	case len(input.RewardTitle) > 40:
		return ValidationError{Message: "reward_title must be 40 characters or fewer"}
	case len(input.RewardDetails) > 500:
		return ValidationError{Message: "reward_details must be 500 characters or fewer"}
	case len(input.SupportText) > 500:
		return ValidationError{Message: "support_text must be 500 characters or fewer"}
	case len(input.TermsText) > 5000:
		return ValidationError{Message: "terms_text must be 5000 characters or fewer"}
	case input.CampaignDisplayMode != "none" && input.CampaignDisplayMode != "details":
		return ValidationError{Message: "campaign_display_mode must be none or details"}
	}
	if len(input.Links) > 5 {
		return ValidationError{Message: "no more than 5 pass links are allowed"}
	}
	seenTypes := make(map[string]struct{}, len(input.Links))
	for _, link := range input.Links {
		switch link.LinkType {
		case "website", "instagram", "whatsapp", "custom":
		default:
			return ValidationError{Message: "link_type must be website, instagram, whatsapp, or custom"}
		}
		if _, exists := seenTypes[link.LinkType]; exists {
			return ValidationError{Message: "each pass link type can only be used once"}
		}
		seenTypes[link.LinkType] = struct{}{}
		if link.Label == "" || len(link.Label) > 40 {
			return ValidationError{Message: "link label is required and must be 40 characters or fewer"}
		}
		if len(link.URL) > 500 {
			return ValidationError{Message: "link URL must be 500 characters or fewer"}
		}
		parsed, err := url.ParseRequestURI(link.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return ValidationError{Message: "pass links must use a valid HTTPS URL"}
		}
	}
	return nil
}

func validateStampGoalChange(existing domain.ApplePassDesign, requestedGoal int) error {
	if existing.Status == "active" && existing.StampGoal != requestedGoal {
		return ValidationError{Message: "stamp_goal cannot be changed after pass activation"}
	}
	return nil
}

var requiredPassAssets = []string{
	"icon.png", "icon@2x.png", "icon@3x.png",
	"logo.png", "logo@2x.png", "logo@3x.png",
}

func validatePassAssetBundle(directory string) error {
	dimensions := make(map[string]image.Config, len(requiredPassAssets))
	for _, name := range requiredPassAssets {
		path := filepath.Join(directory, name)
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("pass asset %s is not readable: %w", name, err)
		}
		cfg, _, decodeErr := image.DecodeConfig(file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("pass asset %s is not a valid PNG: %w", name, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close pass asset %s: %w", name, closeErr)
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return fmt.Errorf("pass asset %s has invalid dimensions", name)
		}
		dimensions[name] = cfg
	}
	icon := dimensions["icon.png"]
	if icon.Width != icon.Height || icon.Width > 38 {
		return fmt.Errorf("icon.png must be square and no larger than 38 x 38 pixels")
	}
	logo := dimensions["logo.png"]
	if logo.Width > 160 || logo.Height > 50 {
		return fmt.Errorf("logo.png must be no larger than 160 x 50 pixels")
	}
	for _, asset := range []string{"icon", "logo"} {
		base := dimensions[asset+".png"]
		for scale, suffix := range map[int]string{2: "@2x.png", 3: "@3x.png"} {
			scaled := dimensions[asset+suffix]
			if scaled.Width != base.Width*scale || scaled.Height != base.Height*scale {
				return fmt.Errorf("%s%s must be exactly %dx the dimensions of %s.png", asset, suffix, scale, asset)
			}
		}
	}
	return nil
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

func (s AppleWalletService) StoreCertificateFiles(alias string, certificate []byte, privateKey []byte) error {
	alias = strings.TrimSpace(strings.ToLower(alias))
	if !credentialAliasPattern.MatchString(alias) {
		return ValidationError{Message: "certificate alias is invalid"}
	}
	if s.certificateBasePath == "" {
		return errors.New("apple certificate base path is not configured")
	}
	if len(certificate) == 0 || len(privateKey) == 0 {
		return ValidationError{Message: "pass certificate and private key are required"}
	}
	directory := filepath.Join(s.certificateBasePath, alias)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create certificate directory: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(directory, "pass-cert.pem"), certificate, 0644); err != nil {
		return fmt.Errorf("store pass certificate: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(directory, "pass-key.pem"), privateKey, 0600); err != nil {
		return fmt.Errorf("store pass private key: %w", err)
	}
	return nil
}

func NewAppleStorageAlias(merchantSlug string) (string, error) {
	base := strings.Trim(strings.ToLower(merchantSlug), "-")
	if len(base) > 52 {
		base = strings.TrimRight(base[:52], "-")
	}
	if !credentialAliasPattern.MatchString(base) {
		return "", ValidationError{Message: "merchant slug cannot be used for Apple Wallet storage"}
	}
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate Apple Wallet storage alias: %w", err)
	}
	return base + "-" + hex.EncodeToString(random), nil
}

func (s AppleWalletService) StorePassAssets(alias string, assets map[string][]byte) error {
	alias = strings.TrimSpace(strings.ToLower(alias))
	if !credentialAliasPattern.MatchString(alias) {
		return ValidationError{Message: "asset bundle alias is invalid"}
	}
	if s.assetBasePath == "" {
		return errors.New("apple pass asset base path is not configured")
	}
	directory := filepath.Join(s.assetBasePath, alias)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("create pass asset directory: %w", err)
	}
	for _, name := range requiredPassAssets {
		data := assets[name]
		if len(data) == 0 {
			return ValidationError{Message: fmt.Sprintf("pass asset %s is required", name)}
		}
		if err := writeAtomicFile(filepath.Join(directory, name), data, 0644); err != nil {
			return fmt.Errorf("store pass asset %s: %w", name, err)
		}
	}
	if err := validatePassAssetBundle(directory); err != nil {
		return ValidationError{Message: err.Error()}
	}
	return nil
}

func (s AppleWalletService) ValidateAssetBundle(alias string) error {
	alias = strings.TrimSpace(strings.ToLower(alias))
	if !credentialAliasPattern.MatchString(alias) || s.assetBasePath == "" {
		return ValidationError{Message: "pass asset bundle is not configured"}
	}
	return validatePassAssetBundle(filepath.Join(s.assetBasePath, alias))
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
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

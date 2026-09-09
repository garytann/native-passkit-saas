package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/domain"
)

type AdminOnboardingService struct {
	merchants MerchantService
	wallet    AppleWalletService
}

func NewAdminOnboardingService(merchants MerchantService, wallet AppleWalletService) AdminOnboardingService {
	return AdminOnboardingService{merchants: merchants, wallet: wallet}
}

func (s AdminOnboardingService) ListMerchants(ctx context.Context) ([]domain.Merchant, error) {
	return s.merchants.ListMerchants(ctx)
}

func (s AdminOnboardingService) UpdatePaymentStatus(ctx context.Context, merchantID string, status string) (domain.MerchantOnboardingStatus, error) {
	if _, err := s.merchants.UpdatePaymentStatus(ctx, merchantID, status); err != nil {
		return domain.MerchantOnboardingStatus{}, err
	}
	return s.Status(ctx, merchantID)
}

func (s AdminOnboardingService) Status(ctx context.Context, merchantID string) (domain.MerchantOnboardingStatus, error) {
	merchant, err := s.merchants.MerchantByID(ctx, merchantID)
	if err != nil {
		return domain.MerchantOnboardingStatus{}, err
	}
	result := domain.MerchantOnboardingStatus{
		Merchant:         merchant,
		PaymentComplete:  merchant.PaymentStatus == "active",
		OnboardingStatus: "provisioning",
		Errors:           []string{},
	}

	configuration, err := s.wallet.Configuration(ctx, merchantID)
	if err != nil {
		if errors.Is(err, domain.ErrAppleWalletIssuerNotFound) {
			result.Errors = append(result.Errors, "Apple Wallet issuer has not been created")
			return finalizeOnboardingStatus(result), nil
		}
		return domain.MerchantOnboardingStatus{}, err
	}
	result.Issuer = &configuration.Issuer
	result.IssuerConfigured = strings.TrimSpace(configuration.Issuer.TeamIdentifier) != "" && strings.TrimSpace(configuration.Issuer.OrganizationName) != ""

	if len(configuration.PassTypes) > 0 {
		passType := configuration.PassTypes[len(configuration.PassTypes)-1]
		result.PassType = &passType
		result.PassTypeConfigured = true
		for _, certificate := range configuration.Certificates {
			if certificate.ApplePassTypeID == passType.ApplePassTypeID && (certificate.Status == "validated" || certificate.Status == "active") && certificate.ExpiresAt.After(time.Now()) {
				result.Certificate = &certificate
				result.CertificateValidated = true
				break
			}
		}
		for _, candidate := range configuration.Designs {
			if candidate.ApplePassTypeID != passType.ApplePassTypeID {
				continue
			}
			design := candidate
			result.Design = &design
			result.DesignConfigured = true
			if design.AssetBundleRef != "" && s.wallet.ValidateAssetBundle(design.AssetBundleRef) == nil {
				result.AssetsUploaded = true
			}
			break
		}
	}

	return finalizeOnboardingStatus(result), nil
}

func (s AdminOnboardingService) Activate(ctx context.Context, merchantID string, passTypeIdentifier string) (domain.MerchantOnboardingStatus, error) {
	status, err := s.Status(ctx, merchantID)
	if err != nil {
		return domain.MerchantOnboardingStatus{}, err
	}
	if !status.CanActivate {
		return domain.MerchantOnboardingStatus{}, ValidationError{Message: "merchant onboarding is incomplete"}
	}
	if status.PassType == nil || status.PassType.PassTypeIdentifier != strings.TrimSpace(passTypeIdentifier) {
		return domain.MerchantOnboardingStatus{}, ValidationError{Message: "pass type identifier does not match the configured merchant pass"}
	}
	if _, err := s.wallet.Activate(ctx, merchantID, passTypeIdentifier); err != nil {
		return domain.MerchantOnboardingStatus{}, err
	}
	return s.Status(ctx, merchantID)
}

func finalizeOnboardingStatus(result domain.MerchantOnboardingStatus) domain.MerchantOnboardingStatus {
	if !result.PaymentComplete {
		result.OnboardingStatus = "awaiting_payment"
	} else if !result.IssuerConfigured {
		result.OnboardingStatus = "needs_issuer"
	} else if !result.PassTypeConfigured {
		result.OnboardingStatus = "needs_pass_type"
	} else if !result.CertificateValidated {
		result.OnboardingStatus = "needs_certificate"
	} else if !result.DesignConfigured || !result.AssetsUploaded {
		result.OnboardingStatus = "needs_design"
	} else {
		result.CanActivate = true
		result.OnboardingStatus = "ready"
		if result.Issuer != nil && result.Issuer.Status == "active" && result.PassType != nil && result.PassType.Status == "active" && result.Certificate != nil && result.Certificate.Status == "active" && result.Design != nil && result.Design.Status == "active" {
			result.OnboardingStatus = "active"
		}
	}
	return result
}

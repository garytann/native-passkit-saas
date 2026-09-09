package service

import (
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

func TestFinalizeOnboardingStatusRequiresEveryProvisioningStep(t *testing.T) {
	status := finalizeOnboardingStatus(domain.MerchantOnboardingStatus{
		PaymentComplete:      true,
		IssuerConfigured:     true,
		PassTypeConfigured:   true,
		CertificateValidated: true,
		AssetsUploaded:       true,
		DesignConfigured:     true,
	})
	if !status.CanActivate || status.OnboardingStatus != "ready" {
		t.Fatalf("expected ready onboarding status, got %+v", status)
	}
}

func TestFinalizeOnboardingStatusBlocksUnpaidMerchant(t *testing.T) {
	status := finalizeOnboardingStatus(domain.MerchantOnboardingStatus{
		IssuerConfigured:     true,
		PassTypeConfigured:   true,
		CertificateValidated: true,
		AssetsUploaded:       true,
		DesignConfigured:     true,
	})
	if status.CanActivate || status.OnboardingStatus != "awaiting_payment" {
		t.Fatalf("expected payment gate, got %+v", status)
	}
}

func TestAdminTokenCannotBeUsedAsMerchantToken(t *testing.T) {
	tokens := NewTokenManager("test-secret")
	token, err := tokens.IssueAdminToken("admin@justloyalty.test")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.VerifyAdminToken(token)
	if err != nil || claims.Role != "platform_admin" {
		t.Fatalf("expected valid platform admin token: %+v, %v", claims, err)
	}
	if _, err := tokens.VerifyMerchantToken(token); err == nil {
		t.Fatal("expected merchant token verification to reject admin token")
	}
}

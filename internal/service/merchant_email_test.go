package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"native-ios-passkit-wip/internal/domain"
)

type merchantRepositoryStub struct {
	credentials domain.MerchantCredentials
	merchant    domain.Merchant
}

func (*merchantRepositoryStub) CreateWithPendingWalletIssuer(context.Context, domain.CreateMerchantInput, domain.CreateAppleWalletIssuerInput) (domain.Merchant, error) {
	return domain.Merchant{}, nil
}
func (r *merchantRepositoryStub) GetByEmail(context.Context, string) (domain.MerchantCredentials, error) {
	return r.credentials, nil
}
func (*merchantRepositoryStub) GetBySlug(context.Context, string) (domain.Merchant, error) {
	return domain.Merchant{}, nil
}
func (r *merchantRepositoryStub) GetByID(context.Context, string) (domain.Merchant, error) {
	return r.merchant, nil
}
func (*merchantRepositoryStub) List(context.Context) ([]domain.Merchant, error) { return nil, nil }
func (*merchantRepositoryStub) UpdatePaymentStatus(context.Context, string, string) (domain.Merchant, error) {
	return domain.Merchant{}, nil
}

func TestMerchantLoginRequiresVerifiedEmailAfterPasswordCheck(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &merchantRepositoryStub{credentials: domain.MerchantCredentials{MerchantID: "MCH-TEST1234", Merchant: "Daily Brew", MerchantEmail: "owner@example.com", PasswordHash: string(hash), Status: "pending_verification"}}
	service := NewMerchantService(repo, NewTokenManager("test-secret"))
	_, err = service.LoginMerchant(context.Background(), LoginMerchantInput{Email: "owner@example.com", Password: "password123"})
	if !errors.Is(err, domain.ErrMerchantEmailNotVerified) {
		t.Fatalf("login error = %v, want email verification error", err)
	}

	_, err = service.LoginMerchant(context.Background(), LoginMerchantInput{Email: "owner@example.com", Password: "wrong-password"})
	if !errors.Is(err, domain.ErrInvalidMerchantCredentials) {
		t.Fatalf("wrong-password error = %v, want invalid credentials", err)
	}
}

func TestMerchantLoginAllowsExistingVerifiedMerchant(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	verifiedAt := time.Now().UTC()
	repo := &merchantRepositoryStub{credentials: domain.MerchantCredentials{MerchantID: "MCH-TEST1234", Merchant: "Daily Brew", MerchantEmail: "owner@example.com", PasswordHash: string(hash), Status: "active", EmailVerifiedAt: &verifiedAt}}
	service := NewMerchantService(repo, NewTokenManager("test-secret"))
	result, err := service.LoginMerchant(context.Background(), LoginMerchantInput{Email: "owner@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" {
		t.Fatal("expected verified merchant login to issue a token")
	}
}

func TestMerchantByIDBlocksUnverifiedOnboarding(t *testing.T) {
	repo := &merchantRepositoryStub{merchant: domain.Merchant{MerchantID: "MCH-TEST1234", Status: "pending_verification"}}
	service := NewMerchantService(repo, NewTokenManager("test-secret"))
	_, err := service.MerchantByID(context.Background(), repo.merchant.MerchantID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("MerchantByID error = %v, want forbidden", err)
	}

	verifiedAt := time.Now().UTC()
	repo.merchant.Status = "active"
	repo.merchant.EmailVerifiedAt = &verifiedAt
	merchant, err := service.MerchantByID(context.Background(), repo.merchant.MerchantID)
	if err != nil {
		t.Fatal(err)
	}
	if merchant.MerchantID != repo.merchant.MerchantID {
		t.Fatalf("merchant id = %q", merchant.MerchantID)
	}
}

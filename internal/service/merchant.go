package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"native-ios-passkit-wip/internal/domain"
)

type MerchantRepository interface {
	Create(ctx context.Context, input domain.CreateMerchantInput) (domain.Merchant, error)
	GetByEmail(ctx context.Context, email string) (domain.MerchantCredentials, error)
}

type RegisterMerchantInput struct {
	FirstName     string
	LastName      string
	Company       string
	MerchantEmail string
	Password      string
}

type MerchantService struct {
	repo         MerchantRepository
	tokenManager TokenManager
	timeout      time.Duration
}

type LoginMerchantInput struct {
	Email    string
	Password string
}

type LoginMerchantResult struct {
	Merchant domain.Merchant
	Token    string
}

func NewMerchantService(repo MerchantRepository, tokenManager TokenManager) MerchantService {
	return MerchantService{
		repo:         repo,
		tokenManager: tokenManager,
		timeout:      5 * time.Second,
	}
}

func (s MerchantService) RegisterMerchant(ctx context.Context, input RegisterMerchantInput) (domain.Merchant, error) {
	if s.repo == nil {
		return domain.Merchant{}, ErrRepositoryUnavailable
	}

	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Company = strings.TrimSpace(input.Company)
	input.MerchantEmail = strings.TrimSpace(strings.ToLower(input.MerchantEmail))
	if err := validateRegisterMerchantInput(input); err != nil {
		return domain.Merchant{}, err
	}

	merchantID, err := domain.NewReadableMerchantID()
	if err != nil {
		return domain.Merchant{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("hash merchant password: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	merchant, err := s.repo.Create(ctx, domain.CreateMerchantInput{
		MerchantID:    merchantID,
		FirstName:     input.FirstName,
		LastName:      input.LastName,
		Company:       input.Company,
		MerchantEmail: input.MerchantEmail,
		PasswordHash:  string(passwordHash),
	})
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("register merchant: %w", err)
	}

	return merchant, nil
}

func (s MerchantService) LoginMerchant(ctx context.Context, input LoginMerchantInput) (LoginMerchantResult, error) {
	if s.repo == nil {
		return LoginMerchantResult{}, ErrRepositoryUnavailable
	}

	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if err := validateLoginMerchantInput(input); err != nil {
		return LoginMerchantResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	merchant, err := s.repo.GetByEmail(ctx, input.Email)
	if err != nil {
		return LoginMerchantResult{}, domain.ErrInvalidMerchantCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(merchant.PasswordHash), []byte(input.Password)); err != nil {
		return LoginMerchantResult{}, domain.ErrInvalidMerchantCredentials
	}

	token, err := s.tokenManager.IssueMerchantToken(merchant.MerchantID, merchant.Merchant)
	if err != nil {
		return LoginMerchantResult{}, err
	}

	return LoginMerchantResult{
		Merchant: merchant.PublicMerchant(),
		Token:    token,
	}, nil
}

func validateRegisterMerchantInput(input RegisterMerchantInput) error {
	switch {
	case input.FirstName == "":
		return ValidationError{Message: "first_name is required"}
	case input.LastName == "":
		return ValidationError{Message: "last_name is required"}
	case input.Company == "":
		return ValidationError{Message: "company is required"}
	case input.MerchantEmail == "":
		return ValidationError{Message: "merchant_email is required"}
	case !strings.Contains(input.MerchantEmail, "@"):
		return ValidationError{Message: "merchant_email must be valid"}
	case input.Password == "":
		return ValidationError{Message: "password is required"}
	case len(input.Password) < 8:
		return ValidationError{Message: "password must be at least 8 characters"}
	default:
		return nil
	}
}

func validateLoginMerchantInput(input LoginMerchantInput) error {
	switch {
	case input.Email == "":
		return ValidationError{Message: "email is required"}
	case !strings.Contains(input.Email, "@"):
		return ValidationError{Message: "email must be valid"}
	case input.Password == "":
		return ValidationError{Message: "password is required"}
	default:
		return nil
	}
}

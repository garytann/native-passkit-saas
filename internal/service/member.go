package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/domain"
)

type MemberRepository interface {
	Create(ctx context.Context, input domain.CreateStoreMemberInput) (domain.StoreMember, error)
	GetByID(ctx context.Context, memberID string) (domain.StoreMember, error)
	ListByMerchant(ctx context.Context, merchant string) ([]domain.StoreMember, error)
	UpdateStampProgressForMerchant(ctx context.Context, memberID string, merchant string, delta int) (domain.StampUpdate, error)
}

type RegisterMemberInput struct {
	Name     string
	Email    string
	Merchant string
}

type RegisterMemberResult struct {
	Member     domain.StoreMember
	PassURL    string
	PreviewURL string
	QRCodeURL  string
}

type UpdateMemberStampsInput struct {
	MemberID   string
	Merchant   string
	StampDelta int
}

type UpdateMemberStampsResult struct {
	domain.StampUpdate
	WalletUpdate WalletUpdateDispatch `json:"wallet_update"`
}

type WalletUpdateDispatch struct {
	RegisteredPassCount int    `json:"registered_pass_count"`
	PushSentCount       int    `json:"push_sent_count"`
	PushFailedCount     int    `json:"push_failed_count"`
	LastError           string `json:"last_error,omitempty"`
}

type MemberService struct {
	repo                MemberRepository
	merchants           MerchantLookupRepository
	linker              PassLinker
	passUpdater         WalletPassUpdater
	walletRegistrations CampaignWalletRegistrationRepository
	pushSender          PassUpdateSender
	timeout             time.Duration
}

type MerchantLookupRepository interface {
	GetBySlug(ctx context.Context, slug string) (domain.Merchant, error)
}

type PassLinker interface {
	PassURL(memberID string) string
	PreviewURL(memberID string) string
	QRCodeURL(memberID string) string
}

func NewMemberService(repo MemberRepository, linker PassLinker) MemberService {
	return NewMemberServiceWithWalletUpdates(repo, nil, linker, nil, nil, nil)
}

func NewMemberServiceWithMerchants(repo MemberRepository, merchants MerchantLookupRepository, linker PassLinker) MemberService {
	return NewMemberServiceWithWalletUpdates(repo, merchants, linker, nil, nil, nil)
}

func NewMemberServiceWithWalletUpdates(repo MemberRepository, merchants MerchantLookupRepository, linker PassLinker, passUpdater WalletPassUpdater, walletRegistrations CampaignWalletRegistrationRepository, pushSender PassUpdateSender) MemberService {
	return MemberService{
		repo:                repo,
		merchants:           merchants,
		linker:              linker,
		passUpdater:         passUpdater,
		walletRegistrations: walletRegistrations,
		pushSender:          pushSender,
		timeout:             5 * time.Second,
	}
}

func (s MemberService) RegisterMember(ctx context.Context, input RegisterMemberInput) (RegisterMemberResult, error) {
	if s.repo == nil {
		return RegisterMemberResult{}, ErrRepositoryUnavailable
	}
	if s.linker == nil {
		return RegisterMemberResult{}, errors.New("pass linker is not configured")
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Merchant = strings.TrimSpace(input.Merchant)
	if err := validateRegisterMemberInput(input); err != nil {
		return RegisterMemberResult{}, err
	}

	memberID, err := domain.NewReadableMemberID()
	if err != nil {
		return RegisterMemberResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	member, err := s.repo.Create(ctx, domain.CreateStoreMemberInput{
		MemberID:      memberID,
		CustomerName:  input.Name,
		CustomerEmail: input.Email,
		Merchant:      input.Merchant,
	})
	if err != nil {
		return RegisterMemberResult{}, fmt.Errorf("register member: %w", err)
	}

	return RegisterMemberResult{
		Member:     member,
		PassURL:    s.linker.PassURL(member.MemberID),
		PreviewURL: s.linker.PreviewURL(member.MemberID),
		QRCodeURL:  s.linker.QRCodeURL(member.MemberID),
	}, nil
}

func (s MemberService) ListMembersForMerchant(ctx context.Context, merchant string) ([]domain.StoreMember, error) {
	if s.repo == nil {
		return nil, ErrRepositoryUnavailable
	}

	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return nil, ValidationError{Message: "merchant is required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	members, err := s.repo.ListByMerchant(ctx, merchant)
	if err != nil {
		return nil, fmt.Errorf("list members for merchant: %w", err)
	}

	return members, nil
}

func (s MemberService) RegisterMemberByMerchantSlug(ctx context.Context, merchantSlug string, input RegisterMemberInput) (RegisterMemberResult, error) {
	if s.merchants == nil {
		return RegisterMemberResult{}, ErrRepositoryUnavailable
	}

	merchantSlug = strings.TrimSpace(strings.ToLower(merchantSlug))
	if merchantSlug == "" {
		return RegisterMemberResult{}, ValidationError{Message: "merchant slug is required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	merchant, err := s.merchants.GetBySlug(ctx, merchantSlug)
	if err != nil {
		return RegisterMemberResult{}, err
	}

	input.Merchant = merchant.Merchant
	return s.RegisterMember(ctx, input)
}

func (s MemberService) UpdateMemberStamps(ctx context.Context, input UpdateMemberStampsInput) (UpdateMemberStampsResult, error) {
	if s.repo == nil {
		return UpdateMemberStampsResult{}, ErrRepositoryUnavailable
	}

	input.MemberID = strings.TrimSpace(input.MemberID)
	input.Merchant = strings.TrimSpace(input.Merchant)
	if err := validateUpdateMemberStampsInput(input); err != nil {
		return UpdateMemberStampsResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	update, err := s.repo.UpdateStampProgressForMerchant(ctx, input.MemberID, input.Merchant, input.StampDelta)
	if err != nil {
		if errors.Is(err, domain.ErrStoreMemberForbidden) {
			return UpdateMemberStampsResult{}, ErrForbidden
		}
		return UpdateMemberStampsResult{}, fmt.Errorf("update member stamps: %w", err)
	}

	return UpdateMemberStampsResult{
		StampUpdate:  update,
		WalletUpdate: s.dispatchWalletPassUpdate(ctx, update.Member.MemberID),
	}, nil
}

func (s MemberService) dispatchWalletPassUpdate(ctx context.Context, memberID string) WalletUpdateDispatch {
	memberID = strings.TrimSpace(memberID)
	if memberID == "" {
		return WalletUpdateDispatch{LastError: "member id is required"}
	}
	if s.passUpdater == nil {
		return WalletUpdateDispatch{}
	}

	if err := s.passUpdater.MarkPassUpdated(ctx, memberID); err != nil {
		if errors.Is(err, domain.ErrWalletPassNotFound) {
			return WalletUpdateDispatch{LastError: "wallet pass not found"}
		}
		return WalletUpdateDispatch{LastError: err.Error()}
	}

	if s.walletRegistrations == nil || s.pushSender == nil {
		return WalletUpdateDispatch{}
	}

	registrations, err := s.walletRegistrations.ListBySerialNumber(ctx, memberID)
	if err != nil {
		return WalletUpdateDispatch{LastError: err.Error()}
	}

	result := WalletUpdateDispatch{
		RegisteredPassCount: len(registrations),
	}
	for _, registration := range registrations {
		if err := s.pushSender.SendPassUpdate(ctx, registration.PushToken); err != nil {
			result.PushFailedCount += 1
			result.LastError = err.Error()
			continue
		}
		result.PushSentCount += 1
	}
	if result.PushSentCount > 0 {
		result.LastError = ""
	}
	return result
}

func validateRegisterMemberInput(input RegisterMemberInput) error {
	switch {
	case input.Name == "":
		return ValidationError{Message: "name is required"}
	case input.Email == "":
		return ValidationError{Message: "email is required"}
	case !strings.Contains(input.Email, "@"):
		return ValidationError{Message: "email must be valid"}
	case input.Merchant == "":
		return ValidationError{Message: "merchant is required"}
	default:
		return nil
	}
}

func validateUpdateMemberStampsInput(input UpdateMemberStampsInput) error {
	switch {
	case input.MemberID == "":
		return ValidationError{Message: "member id is required"}
	case input.Merchant == "":
		return ValidationError{Message: "merchant is required"}
	case input.StampDelta == 0:
		return ValidationError{Message: "stamp_delta is required"}
	case input.StampDelta < -10 || input.StampDelta > 10:
		return ValidationError{Message: "stamp_delta must be between -10 and 10"}
	default:
		return nil
	}
}

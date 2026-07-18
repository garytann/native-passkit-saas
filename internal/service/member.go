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

type MemberService struct {
	repo      MemberRepository
	merchants MerchantLookupRepository
	linker    PassLinker
	timeout   time.Duration
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
	return MemberService{
		repo:    repo,
		linker:  linker,
		timeout: 5 * time.Second,
	}
}

func NewMemberServiceWithMerchants(repo MemberRepository, merchants MerchantLookupRepository, linker PassLinker) MemberService {
	return MemberService{
		repo:      repo,
		merchants: merchants,
		linker:    linker,
		timeout:   5 * time.Second,
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

func (s MemberService) UpdateMemberStamps(ctx context.Context, input UpdateMemberStampsInput) (domain.StampUpdate, error) {
	if s.repo == nil {
		return domain.StampUpdate{}, ErrRepositoryUnavailable
	}

	input.MemberID = strings.TrimSpace(input.MemberID)
	input.Merchant = strings.TrimSpace(input.Merchant)
	if err := validateUpdateMemberStampsInput(input); err != nil {
		return domain.StampUpdate{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	update, err := s.repo.UpdateStampProgressForMerchant(ctx, input.MemberID, input.Merchant, input.StampDelta)
	if err != nil {
		if errors.Is(err, domain.ErrStoreMemberForbidden) {
			return domain.StampUpdate{}, ErrForbidden
		}
		return domain.StampUpdate{}, fmt.Errorf("update member stamps: %w", err)
	}

	return update, nil
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

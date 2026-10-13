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
	GetByIDForMerchant(ctx context.Context, memberID string, merchant string) (domain.StoreMember, error)
	ListByMerchant(ctx context.Context, merchant string) ([]domain.StoreMember, error)
	UpdateStampProgressForMerchant(ctx context.Context, memberID string, merchant string, delta int, stampGoal int) (domain.StampUpdate, error)
	ApplyRewardMutation(ctx context.Context, input domain.RewardMutationInput) (domain.RewardMutation, error)
}

type StampGoalProvider interface {
	StampGoalForMerchant(ctx context.Context, merchant string) (int, error)
}

type RewardTitleProvider interface {
	RewardTitleForMerchant(ctx context.Context, merchant string) (string, error)
}

type RegisterMemberInput struct {
	Name     string
	Email    string
	Merchant string
}

type RegisterMemberResult struct {
	Member      domain.StoreMember
	PassURL     string
	PreviewURL  string
	QRCodeURL   string
	WalletLinks map[string]WalletRegistrationLink
}

type WalletRegistrationLink struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

type RegistrationWalletAvailability interface {
	AvailableForMerchant(ctx context.Context, merchant string) bool
}

type RegistrationWalletProvider interface {
	RegistrationWalletAvailability
	RegistrationLink(memberID string) string
}

type UpdateMemberStampsInput struct {
	MemberID   string
	Merchant   string
	StampDelta int
}

type UpdateMemberStampsResult struct {
	domain.StampUpdate
	StampGoal    int                  `json:"stamp_goal"`
	WalletUpdate WalletUpdateDispatch `json:"wallet_update"`
}

type GetMemberResult struct {
	Member      domain.StoreMember `json:"member"`
	StampGoal   int                `json:"stamp_goal"`
	RewardTitle string             `json:"reward_title"`
}

type MutateMemberRewardInput struct {
	MemberID       string
	MerchantID     string
	Merchant       string
	IdempotencyKey string
}

type MutateMemberRewardResult struct {
	domain.RewardMutation
	StampGoal    int                  `json:"stamp_goal"`
	RewardTitle  string               `json:"reward_title"`
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
	stampGoals          StampGoalProvider
	appleRegistration   RegistrationWalletAvailability
	registrationWallet  RegistrationWalletProvider
	timeout             time.Duration
}

func (s MemberService) WithRegistrationWallet(provider RegistrationWalletProvider) MemberService {
	s.registrationWallet = provider
	return s
}

func (s MemberService) WithAppleRegistrationWallet(provider RegistrationWalletAvailability) MemberService {
	s.appleRegistration = provider
	return s
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
	return NewMemberServiceWithWalletUpdatesAndGoals(repo, merchants, linker, passUpdater, walletRegistrations, pushSender, nil)
}

func NewMemberServiceWithWalletUpdatesAndGoals(repo MemberRepository, merchants MerchantLookupRepository, linker PassLinker, passUpdater WalletPassUpdater, walletRegistrations CampaignWalletRegistrationRepository, pushSender PassUpdateSender, stampGoals StampGoalProvider) MemberService {
	return MemberService{
		repo:                repo,
		merchants:           merchants,
		linker:              linker,
		passUpdater:         passUpdater,
		walletRegistrations: walletRegistrations,
		pushSender:          pushSender,
		stampGoals:          stampGoals,
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

	walletLinks := make(map[string]WalletRegistrationLink, 2)
	applePassURL := ""
	if s.appleRegistration == nil || s.appleRegistration.AvailableForMerchant(ctx, member.Merchant) {
		applePassURL = s.linker.PassURL(member.MemberID)
		walletLinks["apple"] = WalletRegistrationLink{URL: applePassURL, Label: "Add to Apple Wallet"}
	}
	if s.registrationWallet != nil && s.registrationWallet.AvailableForMerchant(ctx, member.Merchant) {
		walletLinks["google"] = WalletRegistrationLink{URL: s.registrationWallet.RegistrationLink(member.MemberID), Label: "Add to Google Wallet"}
	}
	return RegisterMemberResult{
		Member:      member,
		PassURL:     applePassURL,
		PreviewURL:  s.linker.PreviewURL(member.MemberID),
		QRCodeURL:   s.linker.QRCodeURL(member.MemberID),
		WalletLinks: walletLinks,
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

func (s MemberService) GetMemberForMerchant(ctx context.Context, memberID string, merchant string) (GetMemberResult, error) {
	if s.repo == nil {
		return GetMemberResult{}, ErrRepositoryUnavailable
	}
	memberID = strings.TrimSpace(memberID)
	merchant = strings.TrimSpace(merchant)
	if memberID == "" {
		return GetMemberResult{}, ValidationError{Message: "member id is required"}
	}
	if merchant == "" {
		return GetMemberResult{}, ValidationError{Message: "merchant is required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	member, err := s.repo.GetByIDForMerchant(ctx, memberID, merchant)
	if err != nil {
		if errors.Is(err, domain.ErrStoreMemberForbidden) {
			return GetMemberResult{}, ErrForbidden
		}
		return GetMemberResult{}, fmt.Errorf("get member for merchant: %w", err)
	}
	stampGoal, rewardTitle, err := s.memberProgram(ctx, merchant)
	if err != nil {
		return GetMemberResult{}, err
	}
	return GetMemberResult{Member: member, StampGoal: stampGoal, RewardTitle: rewardTitle}, nil
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

	stampGoal, _, err := s.memberProgram(ctx, input.Merchant)
	if err != nil {
		return UpdateMemberStampsResult{}, err
	}

	update, err := s.repo.UpdateStampProgressForMerchant(ctx, input.MemberID, input.Merchant, input.StampDelta, stampGoal)
	if err != nil {
		if errors.Is(err, domain.ErrStoreMemberForbidden) {
			return UpdateMemberStampsResult{}, ErrForbidden
		}
		return UpdateMemberStampsResult{}, fmt.Errorf("update member stamps: %w", err)
	}

	return UpdateMemberStampsResult{
		StampUpdate:  update,
		StampGoal:    stampGoal,
		WalletUpdate: s.dispatchWalletPassUpdate(ctx, update.Member.MemberID),
	}, nil
}

func (s MemberService) IssueMemberReward(ctx context.Context, input MutateMemberRewardInput) (MutateMemberRewardResult, error) {
	return s.mutateMemberReward(ctx, input, "issue")
}

func (s MemberService) RedeemMemberReward(ctx context.Context, input MutateMemberRewardInput) (MutateMemberRewardResult, error) {
	return s.mutateMemberReward(ctx, input, "redeem")
}

func (s MemberService) mutateMemberReward(ctx context.Context, input MutateMemberRewardInput, transactionType string) (MutateMemberRewardResult, error) {
	if s.repo == nil {
		return MutateMemberRewardResult{}, ErrRepositoryUnavailable
	}
	input.MemberID = strings.TrimSpace(input.MemberID)
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.Merchant = strings.TrimSpace(input.Merchant)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	switch {
	case input.MemberID == "":
		return MutateMemberRewardResult{}, ValidationError{Message: "member id is required"}
	case input.MerchantID == "":
		return MutateMemberRewardResult{}, ValidationError{Message: "merchant id is required"}
	case input.Merchant == "":
		return MutateMemberRewardResult{}, ValidationError{Message: "merchant is required"}
	case input.IdempotencyKey == "":
		return MutateMemberRewardResult{}, ValidationError{Message: "Idempotency-Key header is required"}
	case len(input.IdempotencyKey) > 128:
		return MutateMemberRewardResult{}, ValidationError{Message: "Idempotency-Key must be 128 characters or fewer"}
	}

	transactionID, err := domain.NewReadableRewardTransactionID()
	if err != nil {
		return MutateMemberRewardResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	stampGoal, rewardTitle, err := s.memberProgram(ctx, input.Merchant)
	if err != nil {
		return MutateMemberRewardResult{}, err
	}
	mutation, err := s.repo.ApplyRewardMutation(ctx, domain.RewardMutationInput{
		TransactionID:   transactionID,
		MemberID:        input.MemberID,
		MerchantID:      input.MerchantID,
		Merchant:        input.Merchant,
		TransactionType: transactionType,
		StampGoal:       stampGoal,
		IdempotencyKey:  input.IdempotencyKey,
	})
	if err != nil {
		if errors.Is(err, domain.ErrStoreMemberForbidden) {
			return MutateMemberRewardResult{}, ErrForbidden
		}
		return MutateMemberRewardResult{}, fmt.Errorf("%s member reward: %w", transactionType, err)
	}

	result := MutateMemberRewardResult{
		RewardMutation: mutation,
		StampGoal:      stampGoal,
		RewardTitle:    rewardTitle,
	}
	if !mutation.Replayed {
		result.WalletUpdate = s.dispatchWalletPassUpdate(ctx, mutation.Member.MemberID)
	}
	return result, nil
}

func (s MemberService) memberProgram(ctx context.Context, merchant string) (int, string, error) {
	stampGoal := 10
	if s.stampGoals != nil {
		resolvedGoal, err := s.stampGoals.StampGoalForMerchant(ctx, merchant)
		if err != nil {
			return 0, "", fmt.Errorf("resolve merchant stamp goal: %w", err)
		}
		stampGoal = resolvedGoal
	}
	if stampGoal < 1 || stampGoal > 100 {
		return 0, "", ValidationError{Message: "merchant stamp goal must be between 1 and 100"}
	}
	rewardTitle := "Loyalty Reward"
	if provider, ok := s.stampGoals.(RewardTitleProvider); ok {
		resolvedTitle, err := provider.RewardTitleForMerchant(ctx, merchant)
		if err != nil {
			return 0, "", fmt.Errorf("resolve merchant reward title: %w", err)
		}
		if strings.TrimSpace(resolvedTitle) != "" {
			rewardTitle = strings.TrimSpace(resolvedTitle)
		}
	}
	return stampGoal, rewardTitle, nil
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
		if err := s.pushSender.SendPassUpdate(ctx, registration.PassTypeIdentifier, registration.PushToken); err != nil {
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

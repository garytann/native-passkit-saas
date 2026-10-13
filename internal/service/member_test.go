package service

import (
	"context"
	"errors"
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

type fakeMemberRepository struct {
	updateErr           error
	receivedGoal        int
	stampUpdate         domain.StampUpdate
	rewardErr           error
	rewardMutation      domain.RewardMutation
	receivedRewardInput domain.RewardMutationInput
}

func (fakeMemberRepository) Create(context.Context, domain.CreateStoreMemberInput) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (fakeMemberRepository) GetByID(context.Context, string) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (fakeMemberRepository) GetByIDForMerchant(context.Context, string, string) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (fakeMemberRepository) ListByMerchant(context.Context, string) ([]domain.StoreMember, error) {
	return nil, nil
}

func (r *fakeMemberRepository) UpdateStampProgressForMerchant(_ context.Context, _ string, _ string, _ int, stampGoal int) (domain.StampUpdate, error) {
	r.receivedGoal = stampGoal
	if r.updateErr != nil {
		return domain.StampUpdate{}, r.updateErr
	}
	return r.stampUpdate, nil
}

func (r *fakeMemberRepository) ApplyRewardMutation(_ context.Context, input domain.RewardMutationInput) (domain.RewardMutation, error) {
	r.receivedRewardInput = input
	return r.rewardMutation, r.rewardErr
}

type fakeStampGoalProvider struct {
	goal int
	err  error
}

func (p fakeStampGoalProvider) StampGoalForMerchant(context.Context, string) (int, error) {
	return p.goal, p.err
}

func (p fakeStampGoalProvider) RewardTitleForMerchant(context.Context, string) (string, error) {
	return "Free Coffee", p.err
}

type fakeWalletPassUpdater struct {
	markCount int
}

type registrationMemberRepository struct{ fakeMemberRepository }

func (*registrationMemberRepository) Create(_ context.Context, input domain.CreateStoreMemberInput) (domain.StoreMember, error) {
	return domain.StoreMember{MemberID: input.MemberID, CustomerName: input.CustomerName, CustomerEmail: input.CustomerEmail, Merchant: input.Merchant, Status: "active"}, nil
}

type fakePassLinker struct{}

func (fakePassLinker) PassURL(memberID string) string {
	return "https://example.test/apple/" + memberID
}
func (fakePassLinker) PreviewURL(memberID string) string {
	return "https://example.test/preview/" + memberID
}
func (fakePassLinker) QRCodeURL(memberID string) string { return "https://example.test/qr/" + memberID }

type fakeRegistrationWallet struct {
	available bool
	baseURL   string
}

func (w fakeRegistrationWallet) AvailableForMerchant(context.Context, string) bool {
	return w.available
}
func (w fakeRegistrationWallet) RegistrationLink(memberID string) string {
	return w.baseURL + memberID
}

func (u *fakeWalletPassUpdater) MarkPassUpdated(context.Context, string) error {
	u.markCount++
	return nil
}

func TestUpdateMemberStampsMapsMerchantMismatchToForbidden(t *testing.T) {
	repository := &fakeMemberRepository{updateErr: domain.ErrStoreMemberForbidden}
	service := NewMemberService(repository, nil)

	_, err := service.UpdateMemberStamps(context.Background(), UpdateMemberStampsInput{
		MemberID:   "MBR-ABC12345",
		Merchant:   "Daily Brew",
		StampDelta: 1,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestRegisterMemberReturnsOnlyActiveWalletPlatforms(t *testing.T) {
	repository := &registrationMemberRepository{}
	memberService := NewMemberService(repository, fakePassLinker{}).
		WithAppleRegistrationWallet(fakeRegistrationWallet{available: false}).
		WithRegistrationWallet(fakeRegistrationWallet{available: true, baseURL: "https://example.test/google/"})

	result, err := memberService.RegisterMember(context.Background(), RegisterMemberInput{Name: "Taylor", Email: "taylor@example.com", Merchant: "Daily Brew"})
	if err != nil {
		t.Fatalf("register member: %v", err)
	}
	if result.PassURL != "" {
		t.Fatalf("Apple compatibility URL should be empty for a Google-only merchant, got %q", result.PassURL)
	}
	if _, exists := result.WalletLinks["apple"]; exists {
		t.Fatal("inactive Apple Wallet link should be hidden")
	}
	if result.WalletLinks["google"].URL == "" {
		t.Fatal("active Google Wallet link should be returned")
	}
}

func TestIssueMemberRewardUsesGoalAndDispatchesWalletUpdate(t *testing.T) {
	repository := &fakeMemberRepository{rewardMutation: domain.RewardMutation{
		Member:      domain.StoreMember{MemberID: "MBR-ABC12345", AvailableRewards: 1, Status: "active"},
		Transaction: domain.MemberRewardTransaction{TransactionType: "issue", NewRewardBalance: 1},
	}}
	updater := &fakeWalletPassUpdater{}
	memberService := NewMemberServiceWithWalletUpdatesAndGoals(repository, nil, nil, updater, nil, nil, fakeStampGoalProvider{goal: 12})

	result, err := memberService.IssueMemberReward(context.Background(), MutateMemberRewardInput{
		MemberID: "MBR-ABC12345", MerchantID: "MCH-ABC12345", Merchant: "Daily Brew", IdempotencyKey: "issue-key-1",
	})
	if err != nil {
		t.Fatalf("issue member reward: %v", err)
	}
	if repository.receivedRewardInput.StampGoal != 12 || repository.receivedRewardInput.TransactionType != "issue" {
		t.Fatalf("unexpected reward input: %+v", repository.receivedRewardInput)
	}
	if result.RewardTitle != "Free Coffee" || result.StampGoal != 12 {
		t.Fatalf("unexpected programme result: %+v", result)
	}
	if updater.markCount != 1 {
		t.Fatalf("expected one Wallet update, got %d", updater.markCount)
	}
}

func TestRewardReplaySkipsWalletUpdate(t *testing.T) {
	repository := &fakeMemberRepository{rewardMutation: domain.RewardMutation{
		Member:      domain.StoreMember{MemberID: "MBR-ABC12345", AvailableRewards: 1},
		Transaction: domain.MemberRewardTransaction{TransactionType: "redeem"},
		Replayed:    true,
	}}
	updater := &fakeWalletPassUpdater{}
	memberService := NewMemberServiceWithWalletUpdatesAndGoals(repository, nil, nil, updater, nil, nil, fakeStampGoalProvider{goal: 10})

	result, err := memberService.RedeemMemberReward(context.Background(), MutateMemberRewardInput{
		MemberID: "MBR-ABC12345", MerchantID: "MCH-ABC12345", Merchant: "Daily Brew", IdempotencyKey: "redeem-key-1",
	})
	if err != nil {
		t.Fatalf("replay reward redemption: %v", err)
	}
	if !result.Replayed {
		t.Fatal("expected replay result")
	}
	if updater.markCount != 0 {
		t.Fatalf("expected replay to skip Wallet update, got %d", updater.markCount)
	}
}

func TestUpdateMemberStampsUsesConfiguredMerchantGoal(t *testing.T) {
	repository := &fakeMemberRepository{stampUpdate: domain.StampUpdate{Member: domain.StoreMember{MemberID: "MBR-ABC12345"}}}
	service := NewMemberServiceWithWalletUpdatesAndGoals(repository, nil, nil, nil, nil, nil, fakeStampGoalProvider{goal: 12})

	result, err := service.UpdateMemberStamps(context.Background(), UpdateMemberStampsInput{
		MemberID:   "MBR-ABC12345",
		Merchant:   "Daily Brew",
		StampDelta: 1,
	})
	if err != nil {
		t.Fatalf("update member stamps: %v", err)
	}
	if repository.receivedGoal != 12 || result.StampGoal != 12 {
		t.Fatalf("expected configured goal 12, repository=%d result=%d", repository.receivedGoal, result.StampGoal)
	}
}

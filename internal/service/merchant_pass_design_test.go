package service

import (
	"context"
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

type fakeSharedPrograms struct {
	program domain.LoyaltyProgram
	input   domain.UpdateLoyaltyProgramInput
}

func (f *fakeSharedPrograms) ProgramByMerchantID(context.Context, string) (domain.LoyaltyProgram, error) {
	return f.program, nil
}

func (f *fakeSharedPrograms) UpdateProgram(_ context.Context, input domain.UpdateLoyaltyProgramInput) (domain.LoyaltyProgram, error) {
	f.input = input
	f.program.ProgramName = input.ProgramName
	f.program.RewardTitle = input.RewardTitle
	f.program.Version++
	return f.program, nil
}

func TestCampaignDisplayModeForPublishPreservesCurrentWhenOmitted(t *testing.T) {
	if got := campaignDisplayModeForPublish("", "none"); got != "none" {
		t.Fatalf("expected omitted mode to preserve current value, got %q", got)
	}
}

func TestCampaignDisplayModeForPublishNormalizesRequestedMode(t *testing.T) {
	if got := campaignDisplayModeForPublish(" FRONT_AND_DETAILS ", "details"); got != "front_and_details" {
		t.Fatalf("expected requested mode to be normalized, got %q", got)
	}
}

func TestStampIconKeyForPublishPreservesCurrentWhenOmitted(t *testing.T) {
	if got := stampIconKeyForPublish("", "coffee"); got != "coffee" {
		t.Fatalf("expected omitted icon to preserve current value, got %q", got)
	}
}

func TestStampIconKeyForPublishNormalizesRequestedValue(t *testing.T) {
	if got := stampIconKeyForPublish(" SHOPPING_BAG ", "none"); got != "shopping_bag" {
		t.Fatalf("expected requested icon to be normalized, got %q", got)
	}
}

func TestMerchantPassDesignFallsBackToSharedProgram(t *testing.T) {
	programs := &fakeSharedPrograms{program: domain.LoyaltyProgram{
		ProgramID: "LPR-TEST", MerchantID: "MCH-TEST", ProgramName: "Daily Brew Loyalty Card",
		PassDescription: "Daily Brew Loyalty Card", StampGoal: 10, BackgroundColor: "#5434F5",
		AssetBundleRef: "daily-brew", StampInstructions: "Present the card.", RewardTitle: "Free Coffee",
		RewardDetails: "Complete the card.", SupportText: "Contact Daily Brew.", TermsText: "Terms apply.",
		CampaignDisplayMode: "details", StampIconKey: "logo", Status: "active", Version: 2,
	}}
	passDesigns := NewMerchantPassDesignService(AppleWalletService{}, nil, nil, nil).WithSharedPrograms(programs)

	loaded, err := passDesigns.Design(context.Background(), "MCH-TEST")
	if err != nil {
		t.Fatalf("load shared programme: %v", err)
	}
	if loaded.PassTypeIdentifier != "" || loaded.Design.ProgramName != programs.program.ProgramName {
		t.Fatalf("unexpected shared design result: %+v", loaded)
	}

	updated, err := passDesigns.Publish(context.Background(), PublishMerchantPassDesignRequest{
		MerchantID: "MCH-TEST", ProgramName: "Daily Brew Rewards", PassDescription: "Daily Brew Rewards",
		StampGoal: 10, StampInstructions: "Present the card.", RewardTitle: "Free Pastry",
		RewardDetails: "Complete the card.", SupportText: "Contact Daily Brew.", TermsText: "Terms apply.",
		CampaignDisplayMode: "details", StampIconKey: "logo",
	})
	if err != nil {
		t.Fatalf("publish shared programme: %v", err)
	}
	if programs.input.ProgramName != "Daily Brew Rewards" || updated.Design.RewardTitle != "Free Pastry" {
		t.Fatalf("shared programme was not updated: input=%+v result=%+v", programs.input, updated)
	}
}

package service

import (
	"context"
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

type campaignValidationRepository struct{}

func (campaignValidationRepository) CreateCampaign(context.Context, domain.CreateCampaignInput) (domain.Campaign, error) {
	return domain.Campaign{}, nil
}

func (campaignValidationRepository) ListCampaignsByMerchant(context.Context, string) ([]domain.Campaign, error) {
	return nil, nil
}

func (campaignValidationRepository) CreateDeliveries(context.Context, []domain.CreateCampaignDeliveryInput) ([]domain.CampaignDelivery, error) {
	return nil, nil
}

func (campaignValidationRepository) UpdateDeliveryStatus(context.Context, domain.UpdateCampaignDeliveryStatusInput) (domain.CampaignDelivery, error) {
	return domain.CampaignDelivery{}, nil
}

func TestCreateCampaignRejectsTitleOverFortyCharacters(t *testing.T) {
	service := CampaignService{repo: campaignValidationRepository{}, members: &fakeMemberRepository{}}
	_, err := service.CreateForMerchant(context.Background(), CreateCampaignRequest{
		Merchant: "Coffee Brew",
		Title:    "12345678901234567890123456789012345678901",
		Message:  "A valid campaign message.",
	})
	if err == nil {
		t.Fatal("expected campaign title over 40 characters to fail validation")
	}
}

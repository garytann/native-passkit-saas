package service

import "testing"

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

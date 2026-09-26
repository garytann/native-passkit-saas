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

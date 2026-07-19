package applepass

import "testing"

func TestNewStampCardPassUsesPurpleLoyaltyProgramLayout(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{
		PassTypeID:       "pass.com.example.loyalty",
		TeamID:           "TEAM12345",
		OrganizationName: "Saga Loyalty",
	}, StampCardMember{
		ID:         "MBR-12345",
		FullName:   "Gary Tan",
		BrandName:  "Coffee Brew",
		StampCount: 6,
		StampGoal:  10,
		BarcodeID:  "stamp-card:MBR-12345",
	})

	if pass.LogoText != "" {
		t.Fatalf("expected logo text to be omitted, got %q", pass.LogoText)
	}
	if pass.BackgroundColor != "rgb(84, 52, 245)" {
		t.Fatalf("expected purple background, got %q", pass.BackgroundColor)
	}
	if pass.StoreCard == nil {
		t.Fatal("expected storeCard payload")
	}
	if got := pass.StoreCard.HeaderFields[0].Value; got != "6 / 10" {
		t.Fatalf("expected stamp header value, got %v", got)
	}
	if got := pass.StoreCard.PrimaryFields[0].Label; got != "10 STAMPS UNTIL NEXT REWARD" {
		t.Fatalf("expected primary label, got %q", got)
	}
	if got := pass.StoreCard.PrimaryFields[0].Value; got != "Coffee Brew Loyalty Program" {
		t.Fatalf("expected loyalty program name, got %v", got)
	}
}

package applepass

import "testing"

func TestNewStampCardPassUsesMerchantDesignAndCleanFront(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{
		PassTypeID:       "pass.com.example.loyalty",
		TeamID:           "TEAM12345",
		OrganizationName: "Just Loyalty",
	}, StampCardDesign{
		ProgramName:       "Coffee Brew Loyalty Program",
		PassDescription:   "Collect stamps and enjoy rewards at Coffee Brew.",
		StampGoal:         12,
		BackgroundColor:   "#5434F5",
		ForegroundColor:   "#FFFFFF",
		LabelColor:        "#E1DCFF",
		StampInstructions: "Scan this pass at checkout.",
		RewardDetails:     "Receive a complimentary coffee after 12 stamps.",
		Links: []StampCardLink{
			{Key: "instagram", Label: "Instagram", URL: "https://instagram.com/coffeebrew"},
		},
		CampaignDisplayMode: "details",
	}, StampCardMember{
		ID:         "MBR-12345",
		FullName:   "Gary Tan",
		BrandName:  "Coffee Brew",
		StampCount: 6,
		StampGoal:  10,
		BarcodeID:  "stamp-card:MBR-12345",
		Promotion:  "Free pastry this Friday",
	})

	if pass.LogoText != "" {
		t.Fatalf("expected logo text to be omitted, got %q", pass.LogoText)
	}
	if pass.BackgroundColor != "rgb(84, 52, 245)" {
		t.Fatalf("expected configured purple background, got %q", pass.BackgroundColor)
	}
	if pass.Description != "Collect stamps and enjoy rewards at Coffee Brew." {
		t.Fatalf("expected configured description, got %q", pass.Description)
	}
	if pass.StoreCard == nil {
		t.Fatal("expected storeCard payload")
	}
	if got := pass.StoreCard.HeaderFields[0].Value; got != "6 / 12" {
		t.Fatalf("expected design stamp goal in header, got %v", got)
	}
	if got := pass.StoreCard.HeaderFields[0].ChangeMessage; got != "Your stamp progress is now %@." {
		t.Fatalf("expected stamp update change message, got %q", got)
	}
	if got := pass.StoreCard.PrimaryFields[0].Value; got != "Coffee Brew Loyalty Program" {
		t.Fatalf("expected program name, got %v", got)
	}
	if len(pass.StoreCard.SecondaryFields) != 0 || len(pass.StoreCard.AuxiliaryFields) != 0 {
		t.Fatal("expected member, remaining count, and campaign fields to be absent from the pass front")
	}
	latestOffer, ok := fieldByKey(pass.StoreCard.BackFields, "latest_offer")
	if !ok {
		t.Fatal("expected campaign to be retained as latest_offer on the pass back")
	}
	if latestOffer.Value != "Free pastry this Friday" || latestOffer.ChangeMessage != "New offer: %@" {
		t.Fatalf("unexpected latest offer field: %+v", latestOffer)
	}
	progress, ok := fieldByKey(pass.StoreCard.BackFields, "reward_progress")
	if !ok || progress.Value != "6 stamps remaining." {
		t.Fatalf("unexpected reward progress field: %+v", progress)
	}
	instagram, ok := fieldByKey(pass.StoreCard.BackFields, "link_instagram")
	if !ok || instagram.Value != "https://instagram.com/coffeebrew" {
		t.Fatalf("unexpected Instagram field: %+v", instagram)
	}
	if _, ok := fieldByKey(pass.StoreCard.BackFields, "powered_by"); !ok {
		t.Fatal("expected Just Loyalty attribution on pass details")
	}
	if _, ok := fieldByKey(pass.StoreCard.BackFields, "member"); ok {
		t.Fatal("expected member name to be omitted from pass fields")
	}
}

func TestNewStampCardPassCanSuppressCampaignDetails(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{
		CampaignDisplayMode: "none",
	}, StampCardMember{
		ID:        "MBR-12345",
		BarcodeID: "stamp-card:MBR-12345",
		Promotion: "Hidden campaign",
	})

	if _, ok := fieldByKey(pass.StoreCard.BackFields, "latest_offer"); ok {
		t.Fatal("expected campaign details to be suppressed")
	}
}

func fieldByKey(fields []Field, key string) (Field, bool) {
	for _, field := range fields {
		if field.Key == key {
			return field, true
		}
	}
	return Field{}, false
}

package applepass

import (
	"encoding/json"
	"strings"
	"testing"
)

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
		RewardTitle:       "Free Coffee",
		RewardDetails:     "Receive a complimentary coffee after 12 stamps.",
		Links: []StampCardLink{
			{Key: "instagram", Label: "Instagram", URL: "https://instagram.com/coffeebrew"},
		},
		CampaignDisplayMode: "details",
		StampIconKey:        "coffee",
	}, StampCardMember{
		ID:               "MBR-12345",
		FullName:         "Gary Tan",
		BrandName:        "Coffee Brew",
		StampCount:       6,
		StampGoal:        10,
		AvailableRewards: 2,
		BarcodeID:        "stamp-card:MBR-12345",
		PromotionTitle:   "Friday Treat",
		Promotion:        "Free pastry this Friday",
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
	if pass.SuppressStripShine {
		t.Fatal("expected goals above ten to retain the numeric-only fallback")
	}
	if got := pass.StoreCard.HeaderFields[0].Value; got != "6 / 12" {
		t.Fatalf("expected design stamp goal in header, got %v", got)
	}
	if got := pass.StoreCard.HeaderFields[0].ChangeMessage; got != "Your stamp progress is now %@." {
		t.Fatalf("expected stamp update change message, got %q", got)
	}
	if len(pass.StoreCard.PrimaryFields) != 0 {
		t.Fatalf("expected primary fields to be omitted, got %+v", pass.StoreCard.PrimaryFields)
	}
	data, err := json.Marshal(pass)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "primaryFields") {
		t.Fatalf("expected primaryFields to be omitted from pass JSON: %s", data)
	}
	programme, ok := fieldByKey(pass.StoreCard.BackFields, "programme_name")
	if !ok || programme.Label != "Programme" || programme.Value != "Coffee Brew Loyalty Program" {
		t.Fatalf("unexpected programme details field: %+v", programme)
	}
	if _, ok := fieldByKey(pass.StoreCard.BackFields, "about"); ok {
		t.Fatal("expected About This Programme to be omitted")
	}
	if len(pass.StoreCard.SecondaryFields) != 1 {
		t.Fatalf("expected one next reward field, got %d", len(pass.StoreCard.SecondaryFields))
	}
	nextReward := pass.StoreCard.SecondaryFields[0]
	if nextReward.Label != "Next Reward" || nextReward.Value != "Free Coffee" {
		t.Fatalf("unexpected next reward field: %+v", nextReward)
	}
	if nextReward.ChangeMessage != "Your next reward is now %@." {
		t.Fatalf("unexpected next reward change message: %q", nextReward.ChangeMessage)
	}
	if len(pass.StoreCard.AuxiliaryFields) != 0 {
		t.Fatal("expected member, remaining count, and campaign fields to be absent from auxiliary fields")
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
	rewards, ok := fieldByKey(pass.StoreCard.BackFields, "rewards_earned")
	if !ok || rewards.Value != 2 || rewards.ChangeMessage != "Available rewards: %@." {
		t.Fatalf("unexpected rewards earned field: %+v", rewards)
	}
	if pass.UserInfo["availableRewards"] != "2" {
		t.Fatalf("expected available reward balance in userInfo, got %q", pass.UserInfo["availableRewards"])
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

func TestNewStampCardPassDefaultsToMerchantLoyaltyCard(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{}, StampCardMember{BrandName: "Coffee Brew"})
	programme, ok := fieldByKey(pass.StoreCard.BackFields, "programme_name")
	if !ok || programme.Value != "Coffee Brew Loyalty Card" {
		t.Fatalf("expected merchant loyalty card fallback, got %+v", programme)
	}
	if len(pass.StoreCard.PrimaryFields) != 0 {
		t.Fatalf("expected no primary fields, got %+v", pass.StoreCard.PrimaryFields)
	}
}

func TestNewStampCardPassSuppressesShineForVisualStampStrip(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{StampGoal: 10, StampIconKey: "coffee"}, StampCardMember{ID: "MBR-12345"})
	if !pass.SuppressStripShine {
		t.Fatal("expected generated stamp strip shine to be suppressed")
	}
}

func TestNewStampCardPassCanSuppressCampaignDetails(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{
		CampaignDisplayMode: "none",
	}, StampCardMember{
		ID:             "MBR-12345",
		BarcodeID:      "stamp-card:MBR-12345",
		PromotionTitle: "Hidden campaign title",
		Promotion:      "Hidden campaign",
	})

	if _, ok := fieldByKey(pass.StoreCard.BackFields, "latest_offer"); ok {
		t.Fatal("expected campaign details to be suppressed")
	}
	if _, ok := fieldByKey(pass.StoreCard.AuxiliaryFields, "latest_offer_title"); ok {
		t.Fatal("expected campaign front field to be suppressed")
	}
}

func TestNewStampCardPassShowsCampaignTitleOnFrontAndMessageInDetails(t *testing.T) {
	longTitle := "Coffee 优惠 promotion with a title that exceeds forty characters"
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{
		CampaignDisplayMode: "front_and_details",
	}, StampCardMember{
		ID:             "MBR-12345",
		BarcodeID:      "stamp-card:MBR-12345",
		PromotionTitle: longTitle,
		Promotion:      "Receive a free pastry with your next coffee this Friday.",
	})

	frontOffer, ok := fieldByKey(pass.StoreCard.AuxiliaryFields, "latest_offer_title")
	if !ok {
		t.Fatal("expected latest campaign title in auxiliary fields")
	}
	frontValue, ok := frontOffer.Value.(string)
	if !ok || len([]rune(frontValue)) != 40 || frontValue[len(frontValue)-3:] != "..." {
		t.Fatalf("expected a rune-safe 40-character title, got %q", frontValue)
	}
	if frontOffer.ChangeMessage != "" {
		t.Fatalf("expected no duplicate change message on auxiliary field, got %q", frontOffer.ChangeMessage)
	}

	backOffer, ok := fieldByKey(pass.StoreCard.BackFields, "latest_offer")
	if !ok || backOffer.Value != "Receive a free pastry with your next coffee this Friday." {
		t.Fatalf("expected full campaign message in pass details, got %+v", backOffer)
	}
	if backOffer.ChangeMessage != "New offer: %@" {
		t.Fatalf("expected notification copy only on pass details field, got %q", backOffer.ChangeMessage)
	}
}

func TestNewStampCardPassOmitsEmptyCampaign(t *testing.T) {
	pass := NewStampCardPass(TemplateConfig{}, StampCardDesign{
		CampaignDisplayMode: "front_and_details",
	}, StampCardMember{ID: "MBR-12345", BarcodeID: "stamp-card:MBR-12345"})

	if _, ok := fieldByKey(pass.StoreCard.AuxiliaryFields, "latest_offer_title"); ok {
		t.Fatal("expected empty campaign title to be omitted")
	}
	if _, ok := fieldByKey(pass.StoreCard.BackFields, "latest_offer"); ok {
		t.Fatal("expected empty campaign message to be omitted")
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

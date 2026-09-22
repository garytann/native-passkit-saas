package applepass

import (
	"fmt"
	"strconv"
)

type Pass struct {
	FormatVersion       int               `json:"formatVersion"`
	PassTypeIdentifier  string            `json:"passTypeIdentifier"`
	SerialNumber        string            `json:"serialNumber"`
	TeamIdentifier      string            `json:"teamIdentifier"`
	OrganizationName    string            `json:"organizationName"`
	Description         string            `json:"description"`
	LogoText            string            `json:"logoText,omitempty"`
	ForegroundColor     string            `json:"foregroundColor,omitempty"`
	BackgroundColor     string            `json:"backgroundColor,omitempty"`
	LabelColor          string            `json:"labelColor,omitempty"`
	SharingProhibited   bool              `json:"sharingProhibited,omitempty"`
	AuthenticationToken string            `json:"authenticationToken,omitempty"`
	WebServiceURL       string            `json:"webServiceURL,omitempty"`
	Barcodes            []Barcode         `json:"barcodes,omitempty"`
	Barcode             *Barcode          `json:"barcode,omitempty"`
	StoreCard           *PassStructure    `json:"storeCard,omitempty"`
	UserInfo            map[string]string `json:"userInfo,omitempty"`
}

type Barcode struct {
	Format          string `json:"format"`
	Message         string `json:"message"`
	MessageEncoding string `json:"messageEncoding"`
	AltText         string `json:"altText,omitempty"`
}

type PassStructure struct {
	HeaderFields    []Field `json:"headerFields,omitempty"`
	PrimaryFields   []Field `json:"primaryFields,omitempty"`
	SecondaryFields []Field `json:"secondaryFields,omitempty"`
	AuxiliaryFields []Field `json:"auxiliaryFields,omitempty"`
	BackFields      []Field `json:"backFields,omitempty"`
}

type Field struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	Value         any    `json:"value"`
	ChangeMessage string `json:"changeMessage,omitempty"`
}

type StampCardMember struct {
	ID         string
	FullName   string
	BrandName  string
	RewardName string
	StampCount int
	StampGoal  int
	BarcodeID  string
	Promotion  string
}

type TemplateConfig struct {
	PassTypeID          string
	TeamID              string
	OrganizationName    string
	WebServiceURL       string
	AuthenticationToken string
}

type StampCardDesign struct {
	ProgramName         string
	PassDescription     string
	StampGoal           int
	BackgroundColor     string
	ForegroundColor     string
	LabelColor          string
	StampInstructions   string
	RewardDetails       string
	SupportText         string
	TermsText           string
	Links               []StampCardLink
	CampaignDisplayMode string
}

type StampCardLink struct {
	Key   string
	Label string
	URL   string
}

func NewStampCardPass(cfg TemplateConfig, design StampCardDesign, member StampCardMember) Pass {
	design = normalizeStampCardDesign(design, member)
	member.StampGoal = design.StampGoal
	barcode := Barcode{
		Format:          "PKBarcodeFormatQR",
		Message:         member.BarcodeID,
		MessageEncoding: "iso-8859-1",
		AltText:         member.BarcodeID,
	}
	backFields := []Field{
		{Key: "about", Label: "About This Programme", Value: design.PassDescription},
	}
	if design.CampaignDisplayMode == "details" && member.Promotion != "" {
		backFields = append(backFields, Field{
			Key:           "latest_offer",
			Label:         "Latest Promotion",
			Value:         member.Promotion,
			ChangeMessage: "New offer: %@",
		})
	}
	backFields = append(backFields,
		Field{Key: "stamp_instructions", Label: "How to Collect Stamps", Value: design.StampInstructions},
		Field{Key: "reward_details", Label: "Reward Details", Value: design.RewardDetails},
		Field{Key: "reward_progress", Label: "Stamps Until Next Reward", Value: rewardProgress(member.StampCount, member.StampGoal)},
		Field{Key: "member_id", Label: "Member ID", Value: member.ID},
	)
	for _, link := range design.Links {
		backFields = append(backFields, Field{Key: "link_" + link.Key, Label: link.Label, Value: link.URL})
	}
	backFields = append(backFields,
		Field{Key: "support", Label: "Questions? Contact the Merchant", Value: design.SupportText},
		Field{Key: "terms", Label: "Terms and Conditions", Value: design.TermsText},
		Field{Key: "powered_by", Label: "Powered by Just Loyalty", Value: "https://www.just-loyalty.com"},
	)
	pass := Pass{
		FormatVersion:      1,
		PassTypeIdentifier: cfg.PassTypeID,
		SerialNumber:       member.ID,
		TeamIdentifier:     cfg.TeamID,
		OrganizationName:   cfg.OrganizationName,
		Description:        design.PassDescription,
		ForegroundColor:    walletColor(design.ForegroundColor),
		BackgroundColor:    walletColor(design.BackgroundColor),
		LabelColor:         walletColor(design.LabelColor),
		Barcodes:           []Barcode{barcode},
		Barcode:            &barcode,
		StoreCard: &PassStructure{
			HeaderFields: []Field{
				{
					Key:           "stamps",
					Label:         "Stamps",
					Value:         stampProgress(member.StampCount, member.StampGoal),
					ChangeMessage: "Your stamp progress is now %@.",
				},
			},
			PrimaryFields: []Field{
				{Key: "loyalty_program", Label: "Loyalty Card", Value: design.ProgramName},
			},
			BackFields: backFields,
		},
		UserInfo: map[string]string{
			"memberID":       member.ID,
			"loyaltyProgram": design.ProgramName,
			"stampCount":     stampProgress(member.StampCount, member.StampGoal),
		},
	}

	if design.CampaignDisplayMode == "details" && member.Promotion != "" {
		pass.UserInfo["promotion"] = member.Promotion
	}

	if cfg.WebServiceURL != "" && cfg.AuthenticationToken != "" {
		pass.WebServiceURL = cfg.WebServiceURL
		pass.AuthenticationToken = cfg.AuthenticationToken
	}

	return pass
}

func normalizeStampCardDesign(design StampCardDesign, member StampCardMember) StampCardDesign {
	if design.ProgramName == "" {
		design.ProgramName = loyaltyProgramName(member)
	}
	if design.PassDescription == "" {
		design.PassDescription = "Digital loyalty stamp card"
	}
	if design.StampGoal <= 0 {
		design.StampGoal = member.StampGoal
	}
	if design.StampGoal <= 0 {
		design.StampGoal = 10
	}
	if design.BackgroundColor == "" {
		design.BackgroundColor = "#5434F5"
	}
	if design.ForegroundColor == "" {
		design.ForegroundColor = "#FFFFFF"
	}
	if design.LabelColor == "" {
		design.LabelColor = "#E1DCFF"
	}
	if design.SupportText == "" {
		design.SupportText = "Contact the merchant for loyalty card assistance."
	}
	if design.StampInstructions == "" {
		design.StampInstructions = "Present this pass when making an eligible purchase to collect a stamp."
	}
	if design.RewardDetails == "" {
		design.RewardDetails = "Complete your stamp card to receive the merchant reward."
	}
	if design.TermsText == "" {
		design.TermsText = "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms."
	}
	if design.CampaignDisplayMode == "" {
		design.CampaignDisplayMode = "details"
	}
	return design
}

func walletColor(value string) string {
	if len(value) != 7 || value[0] != '#' {
		return value
	}
	r, errR := strconv.ParseUint(value[1:3], 16, 8)
	g, errG := strconv.ParseUint(value[3:5], 16, 8)
	b, errB := strconv.ParseUint(value[5:7], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return value
	}
	return fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
}

func loyaltyProgramName(member StampCardMember) string {
	if member.RewardName != "" {
		return member.RewardName
	}
	if member.BrandName == "" {
		return "Loyalty Program"
	}
	return fmt.Sprintf("%s Loyalty Program", member.BrandName)
}

func stampProgress(count int, goal int) string {
	return fmt.Sprintf("%d / %d", count, goal)
}

func rewardProgress(count int, goal int) string {
	remaining := goal - count
	if remaining <= 0 {
		return "Your next reward is ready."
	}
	if remaining == 1 {
		return "1 stamp remaining."
	}
	return fmt.Sprintf("%d stamps remaining.", remaining)
}

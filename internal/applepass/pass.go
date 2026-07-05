package applepass

import "fmt"

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
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

type StampCardMember struct {
	ID         string
	FullName   string
	BrandName  string
	RewardName string
	StampCount int
	StampGoal  int
	BarcodeID  string
}

type TemplateConfig struct {
	PassTypeID       string
	TeamID           string
	OrganizationName string
	WebServiceURL    string
}

func NewStampCardPass(cfg TemplateConfig, member StampCardMember) Pass {
	barcode := Barcode{
		Format:          "PKBarcodeFormatQR",
		Message:         member.BarcodeID,
		MessageEncoding: "iso-8859-1",
		AltText:         member.BarcodeID,
	}

	pass := Pass{
		FormatVersion:      1,
		PassTypeIdentifier: cfg.PassTypeID,
		SerialNumber:       member.ID,
		TeamIdentifier:     cfg.TeamID,
		OrganizationName:   cfg.OrganizationName,
		Description:        "F&B loyalty stamp card",
		LogoText:           member.BrandName,
		ForegroundColor:    "rgb(255, 255, 255)",
		BackgroundColor:    "rgb(36, 93, 82)",
		LabelColor:         "rgb(214, 238, 228)",
		Barcodes:           []Barcode{barcode},
		Barcode:            &barcode,
		StoreCard: &PassStructure{
			HeaderFields: []Field{
				{Key: "stamps", Label: "Stamps", Value: stampProgress(member.StampCount, member.StampGoal)},
			},
			PrimaryFields: []Field{
				{Key: "reward", Label: "Reward", Value: member.RewardName},
			},
			SecondaryFields: []Field{
				{Key: "member", Label: "Member", Value: member.FullName},
			},
			AuxiliaryFields: []Field{
				{Key: "remaining", Label: "Remaining", Value: remainingStamps(member.StampCount, member.StampGoal)},
			},
			BackFields: []Field{
				{Key: "member_id", Label: "Member ID", Value: member.ID},
				{Key: "support", Label: "Support", Value: "Contact hello@dailybrew.example for loyalty card assistance."},
				{Key: "terms", Label: "Terms", Value: "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms."},
			},
		},
		UserInfo: map[string]string{
			"memberID":   member.ID,
			"stampCount": stampProgress(member.StampCount, member.StampGoal),
		},
	}

	if cfg.WebServiceURL != "" {
		pass.WebServiceURL = cfg.WebServiceURL
		pass.AuthenticationToken = "replace-with-stable-random-token"
	}

	return pass
}

func stampProgress(count int, goal int) string {
	return fmt.Sprintf("%d / %d", count, goal)
}

func remainingStamps(count int, goal int) string {
	remaining := goal - count
	if remaining <= 0 {
		return "Reward ready"
	}
	if remaining == 1 {
		return "1 more"
	}
	return fmt.Sprintf("%d more", remaining)
}

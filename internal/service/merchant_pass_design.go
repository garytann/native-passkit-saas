package service

import (
	"context"
	"fmt"
	"strings"

	"native-ios-passkit-wip/internal/domain"
)

type PassDesignWalletPassRepository interface {
	TouchByPassTypeIdentifier(ctx context.Context, passTypeIdentifier string) (int64, error)
}

type PassDesignWalletRegistrationRepository interface {
	ListByPassTypeIdentifier(ctx context.Context, passTypeIdentifier string) ([]domain.WalletRegistration, error)
}

type MerchantPassDesignService struct {
	wallet              AppleWalletService
	walletPasses        PassDesignWalletPassRepository
	walletRegistrations PassDesignWalletRegistrationRepository
	pushSender          PassUpdateSender
}

type MerchantPassDesignResult struct {
	PassTypeIdentifier string                 `json:"pass_type_identifier"`
	Design             domain.ApplePassDesign `json:"design"`
}

type PublishMerchantPassDesignRequest struct {
	MerchantID          string
	ProgramName         string
	PassDescription     string
	StampGoal           int
	StampInstructions   string
	RewardTitle         string
	RewardDetails       string
	SupportText         string
	TermsText           string
	Links               []ApplePassLinkRequest
	CampaignDisplayMode string
	StampIconKey        string
}

type PublishMerchantPassDesignResult struct {
	PassTypeIdentifier  string                 `json:"pass_type_identifier"`
	Design              domain.ApplePassDesign `json:"design"`
	UpdatedPassCount    int64                  `json:"updated_pass_count"`
	RegisteredPassCount int                    `json:"registered_pass_count"`
	PushSentCount       int                    `json:"push_sent_count"`
	PushFailedCount     int                    `json:"push_failed_count"`
	LastError           string                 `json:"last_error,omitempty"`
}

type PublishMerchantStampIconRequest struct {
	MerchantID string
	IconKey    string
	IconPNG    []byte
}

func NewMerchantPassDesignService(wallet AppleWalletService, walletPasses PassDesignWalletPassRepository, walletRegistrations PassDesignWalletRegistrationRepository, pushSender PassUpdateSender) MerchantPassDesignService {
	return MerchantPassDesignService{
		wallet:              wallet,
		walletPasses:        walletPasses,
		walletRegistrations: walletRegistrations,
		pushSender:          pushSender,
	}
}

func (s MerchantPassDesignService) Design(ctx context.Context, merchantID string) (MerchantPassDesignResult, error) {
	merchantID = strings.TrimSpace(merchantID)
	if merchantID == "" {
		return MerchantPassDesignResult{}, ValidationError{Message: "merchant id is required"}
	}
	configuration, err := s.wallet.Configuration(ctx, merchantID)
	if err != nil {
		return MerchantPassDesignResult{}, err
	}
	for _, passType := range configuration.PassTypes {
		if passType.Status != "active" {
			continue
		}
		if design, ok := designForPassType(configuration, passType.PassTypeIdentifier); ok && design.Status == "active" {
			return MerchantPassDesignResult{PassTypeIdentifier: passType.PassTypeIdentifier, Design: design}, nil
		}
	}
	return MerchantPassDesignResult{}, domain.ErrAppleWalletNotProvisioned
}

func (s MerchantPassDesignService) Publish(ctx context.Context, input PublishMerchantPassDesignRequest) (PublishMerchantPassDesignResult, error) {
	current, err := s.Design(ctx, input.MerchantID)
	if err != nil {
		return PublishMerchantPassDesignResult{}, err
	}
	campaignDisplayMode := campaignDisplayModeForPublish(input.CampaignDisplayMode, current.Design.CampaignDisplayMode)
	stampIconKey := stampIconKeyForPublish(input.StampIconKey, current.Design.StampIconKey)
	design, err := s.wallet.UpsertPassDesign(ctx, UpsertApplePassDesignRequest{
		MerchantID:          input.MerchantID,
		PassTypeIdentifier:  current.PassTypeIdentifier,
		ProgramName:         input.ProgramName,
		PassDescription:     input.PassDescription,
		StampGoal:           input.StampGoal,
		BackgroundColor:     current.Design.BackgroundColor,
		ForegroundColor:     current.Design.ForegroundColor,
		LabelColor:          current.Design.LabelColor,
		AssetBundleRef:      current.Design.AssetBundleRef,
		StampInstructions:   input.StampInstructions,
		RewardTitle:         input.RewardTitle,
		RewardDetails:       input.RewardDetails,
		SupportText:         input.SupportText,
		TermsText:           input.TermsText,
		Links:               input.Links,
		CampaignDisplayMode: campaignDisplayMode,
		StampIconKey:        stampIconKey,
	})
	if err != nil {
		return PublishMerchantPassDesignResult{}, err
	}

	result := PublishMerchantPassDesignResult{
		PassTypeIdentifier: current.PassTypeIdentifier,
		Design:             design,
	}
	if s.walletPasses == nil {
		return result, nil
	}
	result.UpdatedPassCount, err = s.walletPasses.TouchByPassTypeIdentifier(ctx, current.PassTypeIdentifier)
	if err != nil {
		return PublishMerchantPassDesignResult{}, fmt.Errorf("mark merchant passes updated: %w", err)
	}
	if s.walletRegistrations == nil || s.pushSender == nil {
		return result, nil
	}

	registrations, err := s.walletRegistrations.ListByPassTypeIdentifier(ctx, current.PassTypeIdentifier)
	if err != nil {
		return PublishMerchantPassDesignResult{}, fmt.Errorf("list merchant wallet registrations: %w", err)
	}
	result.RegisteredPassCount = len(registrations)
	seenPushTokens := make(map[string]struct{}, len(registrations))
	for _, registration := range registrations {
		pushKey := registration.PassTypeIdentifier + "\x00" + registration.PushToken
		if _, exists := seenPushTokens[pushKey]; exists {
			continue
		}
		seenPushTokens[pushKey] = struct{}{}
		if err := s.pushSender.SendPassUpdate(ctx, registration.PassTypeIdentifier, registration.PushToken); err != nil {
			result.PushFailedCount++
			result.LastError = err.Error()
			continue
		}
		result.PushSentCount++
	}
	if result.PushFailedCount == 0 {
		result.LastError = ""
	}
	return result, nil
}

func (s MerchantPassDesignService) PublishStampIcon(ctx context.Context, input PublishMerchantStampIconRequest) (PublishMerchantPassDesignResult, error) {
	input.MerchantID = strings.TrimSpace(input.MerchantID)
	input.IconKey = strings.ToLower(strings.TrimSpace(input.IconKey))
	if input.MerchantID == "" {
		return PublishMerchantPassDesignResult{}, ValidationError{Message: "merchant id is required"}
	}
	if !validStampIconKey(input.IconKey) {
		return PublishMerchantPassDesignResult{}, ValidationError{Message: "stamp_icon_key must be none, logo, coffee, star, gift, heart, shopping_bag, or custom"}
	}
	current, err := s.Design(ctx, input.MerchantID)
	if err != nil {
		return PublishMerchantPassDesignResult{}, err
	}
	if input.IconKey == "custom" {
		if err := s.wallet.StoreCustomStampIcon(current.Design.AssetBundleRef, input.IconPNG); err != nil {
			return PublishMerchantPassDesignResult{}, err
		}
	}
	links := make([]ApplePassLinkRequest, 0, len(current.Design.Links))
	for _, link := range current.Design.Links {
		links = append(links, ApplePassLinkRequest{LinkType: link.LinkType, Label: link.Label, URL: link.URL})
	}
	return s.Publish(ctx, PublishMerchantPassDesignRequest{
		MerchantID:          input.MerchantID,
		ProgramName:         current.Design.ProgramName,
		PassDescription:     current.Design.PassDescription,
		StampGoal:           current.Design.StampGoal,
		StampInstructions:   current.Design.StampInstructions,
		RewardTitle:         current.Design.RewardTitle,
		RewardDetails:       current.Design.RewardDetails,
		SupportText:         current.Design.SupportText,
		TermsText:           current.Design.TermsText,
		Links:               links,
		CampaignDisplayMode: current.Design.CampaignDisplayMode,
		StampIconKey:        input.IconKey,
	})
}

func (s MerchantPassDesignService) StampArtwork(ctx context.Context, merchantID string, source string) ([]byte, error) {
	current, err := s.Design(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	return s.wallet.StampArtwork(current.Design.AssetBundleRef, source)
}

func campaignDisplayModeForPublish(requested string, current string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" {
		return current
	}
	return requested
}

func stampIconKeyForPublish(requested string, current string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" {
		return current
	}
	return requested
}

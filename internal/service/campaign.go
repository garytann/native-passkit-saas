package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/domain"
)

type CampaignRepository interface {
	CreateCampaign(ctx context.Context, input domain.CreateCampaignInput) (domain.Campaign, error)
	ListCampaignsByMerchant(ctx context.Context, merchant string) ([]domain.Campaign, error)
	CreateDeliveries(ctx context.Context, inputs []domain.CreateCampaignDeliveryInput) ([]domain.CampaignDelivery, error)
	UpdateDeliveryStatus(ctx context.Context, input domain.UpdateCampaignDeliveryStatusInput) (domain.CampaignDelivery, error)
}

type WalletPassUpdater interface {
	MarkPassUpdated(ctx context.Context, serialNumber string) error
}

type CampaignWalletRegistrationRepository interface {
	ListBySerialNumber(ctx context.Context, serialNumber string) ([]domain.WalletRegistration, error)
}

type CampaignService struct {
	repo                CampaignRepository
	members             MemberRepository
	passUpdater         WalletPassUpdater
	walletRegistrations CampaignWalletRegistrationRepository
	pushSender          PassUpdateSender
	campaignNotifier    CampaignWalletNotifier
	timeout             time.Duration
}

type CampaignWalletNotifier interface {
	NotifyCampaign(ctx context.Context, merchant string, campaign domain.Campaign) error
}

func (s CampaignService) WithCampaignNotifier(notifier CampaignWalletNotifier) CampaignService {
	s.campaignNotifier = notifier
	return s
}

type CreateCampaignRequest struct {
	Merchant string
	Title    string
	Message  string
}

type CreateCampaignResult struct {
	Campaign            domain.Campaign           `json:"campaign"`
	Deliveries          []domain.CampaignDelivery `json:"deliveries"`
	DeliveryCount       int                       `json:"delivery_count"`
	RegisteredPassCount int                       `json:"registered_pass_count"`
	PushSentCount       int                       `json:"push_sent_count"`
	PushFailedCount     int                       `json:"push_failed_count"`
}

func NewCampaignService(repo CampaignRepository, members MemberRepository, passUpdater WalletPassUpdater) CampaignService {
	return NewCampaignServiceWithPush(repo, members, passUpdater, nil, nil)
}

func NewCampaignServiceWithPush(repo CampaignRepository, members MemberRepository, passUpdater WalletPassUpdater, walletRegistrations CampaignWalletRegistrationRepository, pushSender PassUpdateSender) CampaignService {
	return CampaignService{
		repo:                repo,
		members:             members,
		passUpdater:         passUpdater,
		walletRegistrations: walletRegistrations,
		pushSender:          pushSender,
		timeout:             5 * time.Second,
	}
}

func (s CampaignService) CreateForMerchant(ctx context.Context, input CreateCampaignRequest) (CreateCampaignResult, error) {
	if s.repo == nil || s.members == nil {
		return CreateCampaignResult{}, ErrRepositoryUnavailable
	}
	input.Merchant = strings.TrimSpace(input.Merchant)
	input.Title = strings.TrimSpace(input.Title)
	input.Message = strings.TrimSpace(input.Message)
	if input.Merchant == "" {
		return CreateCampaignResult{}, ValidationError{Message: "merchant is required"}
	}
	if input.Title == "" {
		return CreateCampaignResult{}, ValidationError{Message: "campaign title is required"}
	}
	if len([]rune(input.Title)) > 40 {
		return CreateCampaignResult{}, ValidationError{Message: "campaign title must be 40 characters or fewer"}
	}
	if input.Message == "" {
		return CreateCampaignResult{}, ValidationError{Message: "campaign message is required"}
	}

	campaignID, err := domain.NewReadableCampaignID()
	if err != nil {
		return CreateCampaignResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	campaign, err := s.repo.CreateCampaign(ctx, domain.CreateCampaignInput{
		CampaignID: campaignID,
		Merchant:   input.Merchant,
		Title:      input.Title,
		Message:    input.Message,
		Status:     "active",
	})
	if err != nil {
		return CreateCampaignResult{}, err
	}

	members, err := s.members.ListByMerchant(ctx, input.Merchant)
	if err != nil {
		return CreateCampaignResult{}, fmt.Errorf("list campaign members: %w", err)
	}

	deliveryInputs := make([]domain.CreateCampaignDeliveryInput, 0, len(members))
	for _, member := range members {
		deliveryID, err := domain.NewReadableDeliveryID()
		if err != nil {
			return CreateCampaignResult{}, err
		}
		deliveryInputs = append(deliveryInputs, domain.CreateCampaignDeliveryInput{
			DeliveryID: deliveryID,
			CampaignID: campaign.CampaignID,
			MemberID:   member.MemberID,
			Status:     "queued",
		})
	}

	deliveries, err := s.repo.CreateDeliveries(ctx, deliveryInputs)
	if err != nil {
		return CreateCampaignResult{}, err
	}

	if s.passUpdater != nil {
		for _, delivery := range deliveries {
			if err := s.passUpdater.MarkPassUpdated(ctx, delivery.MemberID); err != nil && !errors.Is(err, domain.ErrWalletPassNotFound) {
				return CreateCampaignResult{}, err
			}
		}
	}
	if s.campaignNotifier != nil {
		if err := s.campaignNotifier.NotifyCampaign(ctx, input.Merchant, campaign); err != nil && !errors.Is(err, domain.ErrGoogleWalletNotProvisioned) {
			return CreateCampaignResult{}, err
		}
	}

	deliveries, registeredPassCount, pushSentCount, pushFailedCount, err := s.dispatchPassUpdates(ctx, deliveries)
	if err != nil {
		return CreateCampaignResult{}, err
	}

	return CreateCampaignResult{
		Campaign:            campaign,
		Deliveries:          deliveries,
		DeliveryCount:       len(deliveries),
		RegisteredPassCount: registeredPassCount,
		PushSentCount:       pushSentCount,
		PushFailedCount:     pushFailedCount,
	}, nil
}

func (s CampaignService) dispatchPassUpdates(ctx context.Context, deliveries []domain.CampaignDelivery) ([]domain.CampaignDelivery, int, int, int, error) {
	if s.walletRegistrations == nil || s.pushSender == nil {
		return deliveries, 0, 0, 0, nil
	}

	updatedDeliveries := make([]domain.CampaignDelivery, 0, len(deliveries))
	registeredPassCount := 0
	pushSentCount := 0
	pushFailedCount := 0

	for _, delivery := range deliveries {
		registrations, err := s.walletRegistrations.ListBySerialNumber(ctx, delivery.MemberID)
		if err != nil {
			return nil, 0, 0, 0, fmt.Errorf("list wallet registrations for campaign delivery: %w", err)
		}
		if len(registrations) == 0 {
			updated, err := s.repo.UpdateDeliveryStatus(ctx, domain.UpdateCampaignDeliveryStatusInput{
				DeliveryID: delivery.DeliveryID,
				Status:     "queued",
				LastError:  "no wallet registrations for member",
			})
			if err != nil {
				return nil, 0, 0, 0, err
			}
			updatedDeliveries = append(updatedDeliveries, updated)
			continue
		}

		registeredPassCount += len(registrations)
		successes := 0
		var lastError string
		for _, registration := range registrations {
			if err := s.pushSender.SendPassUpdate(ctx, registration.PassTypeIdentifier, registration.PushToken); err != nil {
				pushFailedCount += 1
				lastError = err.Error()
				continue
			}
			successes += 1
			pushSentCount += 1
		}

		status := "failed"
		var deliveredAt *time.Time
		if successes > 0 {
			now := time.Now().UTC()
			deliveredAt = &now
			status = "sent"
			lastError = ""
		}

		updated, err := s.repo.UpdateDeliveryStatus(ctx, domain.UpdateCampaignDeliveryStatusInput{
			DeliveryID:  delivery.DeliveryID,
			Status:      status,
			DeliveredAt: deliveredAt,
			LastError:   lastError,
		})
		if err != nil {
			return nil, 0, 0, 0, err
		}
		updatedDeliveries = append(updatedDeliveries, updated)
	}

	return updatedDeliveries, registeredPassCount, pushSentCount, pushFailedCount, nil
}

func (s CampaignService) ListForMerchant(ctx context.Context, merchant string) ([]domain.Campaign, error) {
	if s.repo == nil {
		return nil, ErrRepositoryUnavailable
	}
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return nil, ValidationError{Message: "merchant is required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	return s.repo.ListCampaignsByMerchant(ctx, merchant)
}

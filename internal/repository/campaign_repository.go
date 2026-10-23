package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"native-ios-passkit-wip/internal/domain"
)

type CampaignRepository struct {
	db *gorm.DB
}

type campaignModel struct {
	CampaignID   string     `gorm:"column:campaign_id;primaryKey"`
	CreationDate time.Time  `gorm:"column:creation_date;default:now()"`
	Merchant     string     `gorm:"column:merchant;not null;index"`
	Title        string     `gorm:"column:title;not null"`
	Message      string     `gorm:"column:message;not null"`
	Status       string     `gorm:"column:status;default:draft"`
	StartDate    *time.Time `gorm:"column:start_date"`
	EndDate      *time.Time `gorm:"column:end_date"`
	SentAt       *time.Time `gorm:"column:sent_at"`
}

func (campaignModel) TableName() string {
	return "campaigns"
}

type campaignDeliveryModel struct {
	DeliveryID   string     `gorm:"column:delivery_id;primaryKey"`
	CreationDate time.Time  `gorm:"column:creation_date;default:now()"`
	CampaignID   string     `gorm:"column:campaign_id;not null;index"`
	MemberID     string     `gorm:"column:member_id;not null;index"`
	Status       string     `gorm:"column:status;default:pending"`
	DeliveredAt  *time.Time `gorm:"column:delivered_at"`
	ViewedAt     *time.Time `gorm:"column:viewed_at"`
	RedeemedAt   *time.Time `gorm:"column:redeemed_at"`
	LastError    string     `gorm:"column:last_error;default:''"`
}

func (campaignDeliveryModel) TableName() string {
	return "campaign_deliveries"
}

func NewCampaignRepository(db *gorm.DB) CampaignRepository {
	return CampaignRepository{db: db}
}

func (r CampaignRepository) CreateCampaign(ctx context.Context, input domain.CreateCampaignInput) (domain.Campaign, error) {
	status := input.Status
	if status == "" {
		status = "draft"
	}

	campaign := campaignModel{
		CampaignID: input.CampaignID,
		Merchant:   input.Merchant,
		Title:      input.Title,
		Message:    input.Message,
		Status:     status,
		StartDate:  input.StartDate,
		EndDate:    input.EndDate,
	}

	if err := r.db.WithContext(ctx).Create(&campaign).Error; err != nil {
		return domain.Campaign{}, fmt.Errorf("create campaign: %w", err)
	}

	return campaignFromModel(campaign), nil
}

func (r CampaignRepository) ListCampaignsByMerchant(ctx context.Context, merchant string) ([]domain.Campaign, error) {
	var rows []campaignModel
	if err := r.db.WithContext(ctx).
		Where("merchant = ?", merchant).
		Order("creation_date DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list campaigns by merchant: %w", err)
	}

	campaigns := make([]domain.Campaign, 0, len(rows))
	for _, row := range rows {
		campaigns = append(campaigns, campaignFromModel(row))
	}
	return campaigns, nil
}

func (r CampaignRepository) CampaignByID(ctx context.Context, campaignID string) (domain.Campaign, error) {
	var row campaignModel
	if err := r.db.WithContext(ctx).Where("campaign_id = ?", campaignID).Take(&row).Error; err != nil {
		return domain.Campaign{}, fmt.Errorf("get campaign by ID: %w", err)
	}
	return campaignFromModel(row), nil
}

func (r CampaignRepository) LatestActiveCampaignForMerchant(ctx context.Context, merchant string) (domain.Campaign, error) {
	var row campaignModel
	if err := r.db.WithContext(ctx).
		Where("merchant = ? AND status IN ?", merchant, []string{"active", "sent"}).
		Order("creation_date DESC, campaign_id DESC").
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Campaign{}, nil
		}
		return domain.Campaign{}, fmt.Errorf("get latest active campaign for merchant: %w", err)
	}
	return campaignFromModel(row), nil
}

func (r CampaignRepository) CreateDeliveries(ctx context.Context, inputs []domain.CreateCampaignDeliveryInput) ([]domain.CampaignDelivery, error) {
	rows := make([]campaignDeliveryModel, 0, len(inputs))
	for _, input := range inputs {
		status := input.Status
		if status == "" {
			status = "pending"
		}
		rows = append(rows, campaignDeliveryModel{
			DeliveryID: input.DeliveryID,
			CampaignID: input.CampaignID,
			MemberID:   input.MemberID,
			Status:     status,
		})
	}

	if len(rows) == 0 {
		return []domain.CampaignDelivery{}, nil
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "campaign_id"}, {Name: "member_id"}},
			DoNothing: true,
		}).
		Create(&rows).Error; err != nil {
		return nil, fmt.Errorf("create campaign deliveries: %w", err)
	}

	deliveries := make([]domain.CampaignDelivery, 0, len(rows))
	for _, row := range rows {
		deliveries = append(deliveries, campaignDeliveryFromModel(row))
	}
	return deliveries, nil
}

func (r CampaignRepository) UpdateDeliveryStatus(ctx context.Context, input domain.UpdateCampaignDeliveryStatusInput) (domain.CampaignDelivery, error) {
	updates := map[string]any{
		"status":     input.Status,
		"last_error": input.LastError,
	}
	if input.DeliveredAt != nil {
		updates["delivered_at"] = input.DeliveredAt
	}

	var row campaignDeliveryModel
	if err := r.db.WithContext(ctx).
		Model(&campaignDeliveryModel{}).
		Where("delivery_id = ?", input.DeliveryID).
		Updates(updates).Error; err != nil {
		return domain.CampaignDelivery{}, fmt.Errorf("update campaign delivery status: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("delivery_id = ?", input.DeliveryID).
		First(&row).Error; err != nil {
		return domain.CampaignDelivery{}, fmt.Errorf("load updated campaign delivery: %w", err)
	}

	return campaignDeliveryFromModel(row), nil
}

func (r CampaignRepository) ActiveDeliveryForMember(ctx context.Context, memberID string) (domain.CampaignDelivery, error) {
	var row campaignDeliveryModel
	if err := r.db.WithContext(ctx).
		Where("member_id = ? AND status IN ?", memberID, []string{"pending", "queued", "sent"}).
		Order("creation_date DESC").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.CampaignDelivery{}, nil
		}
		return domain.CampaignDelivery{}, fmt.Errorf("get active campaign delivery for member: %w", err)
	}

	return campaignDeliveryFromModel(row), nil
}

func (r CampaignRepository) ActiveCampaignForMember(ctx context.Context, memberID string) (domain.Campaign, error) {
	var row campaignModel
	if err := r.db.WithContext(ctx).
		Table("campaigns AS c").
		Select("c.*").
		Joins("INNER JOIN campaign_deliveries AS cd ON cd.campaign_id = c.campaign_id").
		Where("cd.member_id = ? AND cd.status IN ? AND c.status IN ?", memberID, []string{"pending", "queued", "sent"}, []string{"active", "sent"}).
		Order("cd.creation_date DESC").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Campaign{}, nil
		}
		return domain.Campaign{}, fmt.Errorf("get active campaign for member: %w", err)
	}

	return campaignFromModel(row), nil
}

func campaignFromModel(campaign campaignModel) domain.Campaign {
	return domain.Campaign{
		CampaignID:   campaign.CampaignID,
		CreationDate: campaign.CreationDate,
		Merchant:     campaign.Merchant,
		Title:        campaign.Title,
		Message:      campaign.Message,
		Status:       campaign.Status,
		StartDate:    campaign.StartDate,
		EndDate:      campaign.EndDate,
		SentAt:       campaign.SentAt,
	}
}

func campaignDeliveryFromModel(delivery campaignDeliveryModel) domain.CampaignDelivery {
	return domain.CampaignDelivery{
		DeliveryID:   delivery.DeliveryID,
		CreationDate: delivery.CreationDate,
		CampaignID:   delivery.CampaignID,
		MemberID:     delivery.MemberID,
		Status:       delivery.Status,
		DeliveredAt:  delivery.DeliveredAt,
		ViewedAt:     delivery.ViewedAt,
		RedeemedAt:   delivery.RedeemedAt,
		LastError:    delivery.LastError,
	}
}

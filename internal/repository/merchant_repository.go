package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"native-ios-passkit-wip/internal/domain"
)

type MerchantRepository struct {
	db *gorm.DB
}

type merchantModel struct {
	MerchantID    string    `gorm:"column:merchant_id;primaryKey"`
	CreationDate  time.Time `gorm:"column:creation_date;default:now()"`
	Merchant      string    `gorm:"column:merchant;uniqueIndex;not null"`
	MerchantEmail string    `gorm:"column:merchant_email;not null"`
	Status        string    `gorm:"column:status;default:active"`
}

func (merchantModel) TableName() string {
	return "merchants"
}

func NewMerchantRepository(db *gorm.DB) MerchantRepository {
	return MerchantRepository{db: db}
}

func AutoMigrateMerchants(db *gorm.DB) error {
	if err := db.AutoMigrate(&merchantModel{}); err != nil {
		return fmt.Errorf("auto migrate merchants: %w", err)
	}
	return nil
}

func (r MerchantRepository) Create(ctx context.Context, input domain.CreateMerchantInput) (domain.Merchant, error) {
	merchant := merchantModel{
		MerchantID:    input.MerchantID,
		Merchant:      input.Merchant,
		MerchantEmail: input.MerchantEmail,
	}

	if err := r.db.WithContext(ctx).Create(&merchant).Error; err != nil {
		return domain.Merchant{}, fmt.Errorf("create merchant: %w", err)
	}

	return domain.Merchant{
		MerchantID:    merchant.MerchantID,
		CreationDate:  merchant.CreationDate,
		Merchant:      merchant.Merchant,
		MerchantEmail: merchant.MerchantEmail,
		Status:        merchant.Status,
	}, nil
}

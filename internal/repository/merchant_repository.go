package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"native-ios-passkit-wip/internal/domain"
)

type MerchantRepository struct {
	db *gorm.DB
}

type merchantModel struct {
	MerchantID    string    `gorm:"column:merchant_id;primaryKey"`
	CreationDate  time.Time `gorm:"column:creation_date;default:now()"`
	FirstName     string    `gorm:"column:first_name;default:''"`
	LastName      string    `gorm:"column:last_name;default:''"`
	Merchant      string    `gorm:"column:merchant;uniqueIndex;not null"`
	MerchantEmail string    `gorm:"column:merchant_email;uniqueIndex;not null"`
	PasswordHash  string    `gorm:"column:password_hash;default:'';not null"`
	Status        string    `gorm:"column:status;default:active"`
	PaymentStatus string    `gorm:"column:payment_status;default:inactive;not null"`
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
		FirstName:     input.FirstName,
		LastName:      input.LastName,
		Merchant:      input.Company,
		MerchantEmail: input.MerchantEmail,
		PasswordHash:  input.PasswordHash,
		PaymentStatus: "inactive",
	}

	if err := r.db.WithContext(ctx).Create(&merchant).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.Merchant{}, domain.ErrMerchantAlreadyExists
		}
		return domain.Merchant{}, fmt.Errorf("create merchant: %w", err)
	}

	return domain.Merchant{
		MerchantID:    merchant.MerchantID,
		CreationDate:  merchant.CreationDate,
		FirstName:     merchant.FirstName,
		LastName:      merchant.LastName,
		Merchant:      merchant.Merchant,
		MerchantEmail: merchant.MerchantEmail,
		Status:        merchant.Status,
		PaymentStatus: merchant.PaymentStatus,
	}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

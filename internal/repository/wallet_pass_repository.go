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

type WalletPassRepository struct {
	db *gorm.DB
}

type walletPassModel struct {
	PassTypeIdentifier  string    `gorm:"column:pass_type_identifier;primaryKey"`
	SerialNumber        string    `gorm:"column:serial_number;primaryKey"`
	AuthenticationToken string    `gorm:"column:authentication_token;uniqueIndex;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;default:now()"`
	LastUpdatedAt       time.Time `gorm:"column:last_updated_at;default:now();index"`
}

func (walletPassModel) TableName() string {
	return "wallet_passes"
}

func NewWalletPassRepository(db *gorm.DB) WalletPassRepository {
	return WalletPassRepository{db: db}
}

func (r WalletPassRepository) GetOrCreate(ctx context.Context, input domain.CreateWalletPassInput) (domain.WalletPass, error) {
	walletPass := walletPassModel{
		PassTypeIdentifier:  input.PassTypeIdentifier,
		SerialNumber:        input.SerialNumber,
		AuthenticationToken: input.AuthenticationToken,
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pass_type_identifier"}, {Name: "serial_number"}},
			DoNothing: true,
		}).
		Create(&walletPass).Error; err != nil {
		return domain.WalletPass{}, fmt.Errorf("create wallet pass: %w", err)
	}

	return r.GetBySerial(ctx, input.PassTypeIdentifier, input.SerialNumber)
}

func (r WalletPassRepository) GetBySerial(ctx context.Context, passTypeIdentifier string, serialNumber string) (domain.WalletPass, error) {
	var walletPass walletPassModel
	if err := r.db.WithContext(ctx).
		First(
			&walletPass,
			"pass_type_identifier = ? AND serial_number = ?",
			passTypeIdentifier,
			serialNumber,
		).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.WalletPass{}, domain.ErrWalletPassNotFound
		}
		return domain.WalletPass{}, fmt.Errorf("get wallet pass: %w", err)
	}

	return walletPassFromModel(walletPass), nil
}

func (r WalletPassRepository) TouchBySerial(ctx context.Context, serialNumber string) error {
	result := r.db.WithContext(ctx).
		Model(&walletPassModel{}).
		Where("serial_number = ?", serialNumber).
		Update("last_updated_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("touch wallet pass: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrWalletPassNotFound
	}
	return nil
}

func walletPassFromModel(walletPass walletPassModel) domain.WalletPass {
	return domain.WalletPass{
		PassTypeIdentifier:  walletPass.PassTypeIdentifier,
		SerialNumber:        walletPass.SerialNumber,
		AuthenticationToken: walletPass.AuthenticationToken,
		CreatedAt:           walletPass.CreatedAt,
		LastUpdatedAt:       walletPass.LastUpdatedAt,
	}
}

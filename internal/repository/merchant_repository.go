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
	MerchantSlug  string    `gorm:"column:merchant_slug;uniqueIndex"`
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
	return r.create(ctx, r.db, input)
}

func (r MerchantRepository) CreateWithPendingWalletIssuer(ctx context.Context, input domain.CreateMerchantInput, issuerInput domain.CreateAppleWalletIssuerInput) (domain.Merchant, error) {
	var created domain.Merchant
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		merchant, err := r.create(ctx, tx, input)
		if err != nil {
			return err
		}
		issuer := appleWalletIssuerModel{
			IssuerID:         issuerInput.IssuerID,
			MerchantID:       issuerInput.MerchantID,
			OwnershipMode:    issuerInput.OwnershipMode,
			TeamIdentifier:   issuerInput.TeamIdentifier,
			OrganizationName: issuerInput.OrganizationName,
			Status:           issuerInput.Status,
		}
		if err := tx.WithContext(ctx).Create(&issuer).Error; err != nil {
			return fmt.Errorf("create pending apple wallet issuer: %w", err)
		}
		created = merchant
		return nil
	})
	if err != nil {
		return domain.Merchant{}, err
	}
	return created, nil
}

func (r MerchantRepository) create(ctx context.Context, db *gorm.DB, input domain.CreateMerchantInput) (domain.Merchant, error) {
	merchant := merchantModel{
		MerchantID:    input.MerchantID,
		FirstName:     input.FirstName,
		LastName:      input.LastName,
		Merchant:      input.Company,
		MerchantSlug:  input.MerchantSlug,
		MerchantEmail: input.MerchantEmail,
		PasswordHash:  input.PasswordHash,
		PaymentStatus: "inactive",
	}

	if err := db.WithContext(ctx).Create(&merchant).Error; err != nil {
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
		MerchantSlug:  merchant.MerchantSlug,
		MerchantEmail: merchant.MerchantEmail,
		Status:        merchant.Status,
		PaymentStatus: merchant.PaymentStatus,
	}, nil
}

func (r MerchantRepository) GetByEmail(ctx context.Context, email string) (domain.MerchantCredentials, error) {
	var merchant merchantModel
	if err := r.db.WithContext(ctx).First(&merchant, "merchant_email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.MerchantCredentials{}, domain.ErrInvalidMerchantCredentials
		}
		return domain.MerchantCredentials{}, fmt.Errorf("get merchant by email: %w", err)
	}

	return domain.MerchantCredentials{
		MerchantID:    merchant.MerchantID,
		FirstName:     merchant.FirstName,
		LastName:      merchant.LastName,
		Merchant:      merchant.Merchant,
		MerchantSlug:  merchant.MerchantSlug,
		MerchantEmail: merchant.MerchantEmail,
		PasswordHash:  merchant.PasswordHash,
		Status:        merchant.Status,
		PaymentStatus: merchant.PaymentStatus,
	}, nil
}

func (r MerchantRepository) GetBySlug(ctx context.Context, slug string) (domain.Merchant, error) {
	var merchant merchantModel
	if err := r.db.WithContext(ctx).
		Where("merchant_slug = ? AND password_hash <> ''", slug).
		First(&merchant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Merchant{}, domain.ErrMerchantNotFound
		}
		return domain.Merchant{}, fmt.Errorf("get merchant by slug: %w", err)
	}

	return domain.Merchant{
		MerchantID:    merchant.MerchantID,
		CreationDate:  merchant.CreationDate,
		FirstName:     merchant.FirstName,
		LastName:      merchant.LastName,
		Merchant:      merchant.Merchant,
		MerchantSlug:  merchant.MerchantSlug,
		MerchantEmail: merchant.MerchantEmail,
		Status:        merchant.Status,
		PaymentStatus: merchant.PaymentStatus,
	}, nil
}

func (r MerchantRepository) GetByID(ctx context.Context, merchantID string) (domain.Merchant, error) {
	var merchant merchantModel
	if err := r.db.WithContext(ctx).First(&merchant, "merchant_id = ?", merchantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Merchant{}, domain.ErrMerchantNotFound
		}
		return domain.Merchant{}, fmt.Errorf("get merchant by id: %w", err)
	}
	return merchantFromModel(merchant), nil
}

func (r MerchantRepository) List(ctx context.Context) ([]domain.Merchant, error) {
	var models []merchantModel
	if err := r.db.WithContext(ctx).
		Where("password_hash <> ''").
		Order("creation_date DESC").
		Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list merchants: %w", err)
	}
	merchants := make([]domain.Merchant, 0, len(models))
	for _, model := range models {
		merchants = append(merchants, merchantFromModel(model))
	}
	return merchants, nil
}

func (r MerchantRepository) UpdatePaymentStatus(ctx context.Context, merchantID string, status string) (domain.Merchant, error) {
	result := r.db.WithContext(ctx).Model(&merchantModel{}).
		Where("merchant_id = ?", merchantID).
		Update("payment_status", status)
	if result.Error != nil {
		return domain.Merchant{}, fmt.Errorf("update merchant payment status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.Merchant{}, domain.ErrMerchantNotFound
	}
	return r.GetByID(ctx, merchantID)
}

func merchantFromModel(merchant merchantModel) domain.Merchant {
	return domain.Merchant{
		MerchantID:    merchant.MerchantID,
		CreationDate:  merchant.CreationDate,
		FirstName:     merchant.FirstName,
		LastName:      merchant.LastName,
		Merchant:      merchant.Merchant,
		MerchantSlug:  merchant.MerchantSlug,
		MerchantEmail: merchant.MerchantEmail,
		Status:        merchant.Status,
		PaymentStatus: merchant.PaymentStatus,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

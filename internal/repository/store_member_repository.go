package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"native-ios-passkit-wip/internal/domain"
)

type StoreMemberRepository struct {
	db *gorm.DB
}

type storeMemberModel struct {
	MemberID      string    `gorm:"column:member_id;primaryKey"`
	CreationDate  time.Time `gorm:"column:creation_date;default:now()"`
	CustomerName  string    `gorm:"column:customer_name"`
	CustomerEmail string    `gorm:"column:customer_email"`
	Merchant      string    `gorm:"column:merchant;not null;index"`
	StampProgress int       `gorm:"column:stamp_progress;default:0"`
	Status        string    `gorm:"column:status;default:active"`
}

func (storeMemberModel) TableName() string {
	return "store_members"
}

func NewStoreMemberRepository(db *gorm.DB) StoreMemberRepository {
	return StoreMemberRepository{db: db}
}

func AutoMigrateStoreMembers(db *gorm.DB) error {
	if err := db.AutoMigrate(&storeMemberModel{}); err != nil {
		return fmt.Errorf("auto migrate store members: %w", err)
	}
	if err := backfillMerchantsFromStoreMembers(db); err != nil {
		return err
	}
	if err := ensureStoreMembersMerchantConstraint(db); err != nil {
		return err
	}
	return nil
}

func backfillMerchantsFromStoreMembers(db *gorm.DB) error {
	if err := db.Exec(`
		INSERT INTO merchants (merchant_id, merchant, merchant_email)
		SELECT
			'MCH-' || UPPER(SUBSTRING(MD5(merchant) FROM 1 FOR 8)),
			merchant,
			'merchant-' || LOWER(SUBSTRING(MD5(merchant) FROM 1 FOR 10)) || '@example.com'
		FROM (
			SELECT DISTINCT merchant
			FROM store_members
			WHERE merchant IS NOT NULL AND merchant <> ''
		) existing_merchants
		ON CONFLICT (merchant) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("backfill merchants from store members: %w", err)
	}

	return nil
}

func ensureStoreMembersMerchantConstraint(db *gorm.DB) error {
	const constraintName = "fk_store_members_merchant"

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.table_constraints
			WHERE constraint_name = ?
			  AND table_name = 'store_members'
		)
	`, constraintName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check store member merchant constraint: %w", err)
	}

	if exists {
		return nil
	}

	if err := db.Exec(`
		ALTER TABLE store_members
		ADD CONSTRAINT fk_store_members_merchant
		FOREIGN KEY (merchant)
		REFERENCES merchants (merchant)
		ON UPDATE CASCADE
		ON DELETE RESTRICT
	`).Error; err != nil {
		return fmt.Errorf("create store member merchant constraint: %w", err)
	}

	return nil
}

func (r StoreMemberRepository) Create(ctx context.Context, input domain.CreateStoreMemberInput) (domain.StoreMember, error) {
	if err := r.ensureMerchant(ctx, input.Merchant); err != nil {
		return domain.StoreMember{}, err
	}

	member := storeMemberModel{
		MemberID:      input.MemberID,
		CustomerName:  input.CustomerName,
		CustomerEmail: input.CustomerEmail,
		Merchant:      input.Merchant,
	}

	if err := r.db.WithContext(ctx).Create(&member).Error; err != nil {
		return domain.StoreMember{}, fmt.Errorf("create store member: %w", err)
	}

	return domain.StoreMember{
		MemberID:      member.MemberID,
		CreationDate:  member.CreationDate,
		CustomerName:  member.CustomerName,
		CustomerEmail: member.CustomerEmail,
		Merchant:      member.Merchant,
		StampProgress: member.StampProgress,
		Status:        member.Status,
	}, nil
}

func (r StoreMemberRepository) ensureMerchant(ctx context.Context, merchantName string) error {
	merchantName = strings.TrimSpace(merchantName)
	if merchantName == "" {
		return fmt.Errorf("merchant is required")
	}

	merchant := merchantModel{
		MerchantID:    deterministicMerchantID(merchantName),
		Merchant:      merchantName,
		MerchantEmail: placeholderMerchantEmail(merchantName),
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "merchant"}},
			DoNothing: true,
		}).
		Create(&merchant).Error; err != nil {
		return fmt.Errorf("ensure merchant exists: %w", err)
	}

	return nil
}

func deterministicMerchantID(merchantName string) string {
	return "MCH-" + strings.ToUpper(domain.StableToken(merchantName, 8))
}

func placeholderMerchantEmail(merchantName string) string {
	token := strings.ToLower(domain.StableToken(merchantName, 10))
	return "merchant-" + token + "@example.com"
}

func (r StoreMemberRepository) GetByID(ctx context.Context, memberID string) (domain.StoreMember, error) {
	var member storeMemberModel
	if err := r.db.WithContext(ctx).First(&member, "member_id = ?", memberID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.StoreMember{}, domain.ErrStoreMemberNotFound
		}
		return domain.StoreMember{}, fmt.Errorf("get store member by id: %w", err)
	}

	return domain.StoreMember{
		MemberID:      member.MemberID,
		CreationDate:  member.CreationDate,
		CustomerName:  member.CustomerName,
		CustomerEmail: member.CustomerEmail,
		Merchant:      member.Merchant,
		StampProgress: member.StampProgress,
		Status:        member.Status,
	}, nil
}

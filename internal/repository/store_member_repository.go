package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
	if err := ensureStoreMembersMerchantConstraint(db); err != nil {
		return err
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
	member := storeMemberModel{
		MemberID:      input.MemberID,
		CustomerName:  input.CustomerName,
		CustomerEmail: input.CustomerEmail,
		Merchant:      input.Merchant,
	}

	if err := r.db.WithContext(ctx).Create(&member).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.StoreMember{}, domain.ErrMerchantNotFound
		}
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

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
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

func (r StoreMemberRepository) ListByMerchant(ctx context.Context, merchantName string) ([]domain.StoreMember, error) {
	var rows []storeMemberModel
	if err := r.db.WithContext(ctx).
		Where("merchant = ?", merchantName).
		Order("creation_date DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list store members by merchant: %w", err)
	}

	members := make([]domain.StoreMember, 0, len(rows))
	for _, member := range rows {
		members = append(members, domain.StoreMember{
			MemberID:      member.MemberID,
			CreationDate:  member.CreationDate,
			CustomerName:  member.CustomerName,
			CustomerEmail: member.CustomerEmail,
			Merchant:      member.Merchant,
			StampProgress: member.StampProgress,
			Status:        member.Status,
		})
	}

	return members, nil
}

func (r StoreMemberRepository) UpdateStampProgressForMerchant(ctx context.Context, memberID string, merchantName string, delta int) (domain.StampUpdate, error) {
	var update domain.StampUpdate

	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var member storeMemberModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&member, "member_id = ?", memberID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrStoreMemberNotFound
			}
			return fmt.Errorf("get store member for stamp update: %w", err)
		}

		if member.Merchant != merchantName {
			return domain.ErrStoreMemberForbidden
		}

		previous := member.StampProgress
		next := previous + delta
		if next < 0 {
			next = 0
		}
		if next > 10 {
			next = 10
		}

		member.StampProgress = next
		if next >= 10 {
			member.Status = "reward_ready"
		} else if member.Status == "reward_ready" {
			member.Status = "active"
		}

		if err := tx.Save(&member).Error; err != nil {
			return fmt.Errorf("update store member stamp progress: %w", err)
		}

		update = domain.StampUpdate{
			Member: domain.StoreMember{
				MemberID:      member.MemberID,
				CreationDate:  member.CreationDate,
				CustomerName:  member.CustomerName,
				CustomerEmail: member.CustomerEmail,
				Merchant:      member.Merchant,
				StampProgress: member.StampProgress,
				Status:        member.Status,
			},
			StampDelta:            delta,
			PreviousStampProgress: previous,
			NewStampProgress:      next,
		}

		return nil
	}); err != nil {
		return domain.StampUpdate{}, err
	}

	return update, nil
}

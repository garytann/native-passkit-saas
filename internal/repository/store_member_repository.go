package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
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
	Merchant      string    `gorm:"column:merchant"`
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

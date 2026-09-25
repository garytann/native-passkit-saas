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
	MemberID         string    `gorm:"column:member_id;primaryKey"`
	CreationDate     time.Time `gorm:"column:creation_date;default:now()"`
	CustomerName     string    `gorm:"column:customer_name"`
	CustomerEmail    string    `gorm:"column:customer_email"`
	Merchant         string    `gorm:"column:merchant;not null;index"`
	StampProgress    int       `gorm:"column:stamp_progress;default:0"`
	AvailableRewards int       `gorm:"column:available_rewards;default:0"`
	Status           string    `gorm:"column:status;default:active"`
}

type memberRewardTransactionModel struct {
	TransactionID         string    `gorm:"column:transaction_id;primaryKey"`
	MemberID              string    `gorm:"column:member_id;not null;index"`
	MerchantID            string    `gorm:"column:merchant_id;not null;index;uniqueIndex:idx_reward_merchant_idempotency"`
	TransactionType       string    `gorm:"column:transaction_type;not null"`
	Quantity              int       `gorm:"column:quantity;not null"`
	PreviousRewardBalance int       `gorm:"column:previous_reward_balance;not null"`
	NewRewardBalance      int       `gorm:"column:new_reward_balance;not null"`
	PreviousStampProgress int       `gorm:"column:previous_stamp_progress;not null"`
	NewStampProgress      int       `gorm:"column:new_stamp_progress;not null"`
	CreationDate          time.Time `gorm:"column:creation_date;default:now()"`
	IdempotencyKey        string    `gorm:"column:idempotency_key;not null;uniqueIndex:idx_reward_merchant_idempotency"`
}

func (memberRewardTransactionModel) TableName() string {
	return "member_reward_transactions"
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

	return storeMemberFromModel(member), nil
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

	return storeMemberFromModel(member), nil
}

func (r StoreMemberRepository) GetByIDForMerchant(ctx context.Context, memberID string, merchantName string) (domain.StoreMember, error) {
	member, err := r.GetByID(ctx, memberID)
	if err != nil {
		return domain.StoreMember{}, err
	}
	if member.Merchant != merchantName {
		return domain.StoreMember{}, domain.ErrStoreMemberForbidden
	}
	return member, nil
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
		members = append(members, storeMemberFromModel(member))
	}

	return members, nil
}

func (r StoreMemberRepository) UpdateStampProgressForMerchant(ctx context.Context, memberID string, merchantName string, delta int, stampGoal int) (domain.StampUpdate, error) {
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
		next, status := nextStampState(previous, delta, stampGoal, member.Status)
		member.StampProgress = next
		member.Status = status

		if err := tx.Save(&member).Error; err != nil {
			return fmt.Errorf("update store member stamp progress: %w", err)
		}

		update = domain.StampUpdate{
			Member:                storeMemberFromModel(member),
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

func (r StoreMemberRepository) ApplyRewardMutation(ctx context.Context, input domain.RewardMutationInput) (domain.RewardMutation, error) {
	var result domain.RewardMutation

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var member storeMemberModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&member, "member_id = ?", input.MemberID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrStoreMemberNotFound
			}
			return fmt.Errorf("get store member for reward update: %w", err)
		}
		if member.Merchant != input.Merchant {
			return domain.ErrStoreMemberForbidden
		}

		var existing memberRewardTransactionModel
		err := tx.Where("merchant_id = ? AND idempotency_key = ?", input.MerchantID, input.IdempotencyKey).First(&existing).Error
		if err == nil {
			if existing.MemberID != input.MemberID || existing.TransactionType != input.TransactionType {
				return domain.ErrIdempotencyKeyConflict
			}
			result = domain.RewardMutation{
				Member:      storeMemberFromModel(member),
				Transaction: rewardTransactionFromModel(existing),
				Replayed:    true,
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find reward transaction by idempotency key: %w", err)
		}

		previousRewards := member.AvailableRewards
		previousStamps := member.StampProgress
		if err := applyRewardState(&member, input.TransactionType, input.StampGoal); err != nil {
			return err
		}

		if err := tx.Save(&member).Error; err != nil {
			return fmt.Errorf("update member reward balance: %w", err)
		}

		transaction := memberRewardTransactionModel{
			TransactionID:         input.TransactionID,
			MemberID:              member.MemberID,
			MerchantID:            input.MerchantID,
			TransactionType:       input.TransactionType,
			Quantity:              1,
			PreviousRewardBalance: previousRewards,
			NewRewardBalance:      member.AvailableRewards,
			PreviousStampProgress: previousStamps,
			NewStampProgress:      member.StampProgress,
			IdempotencyKey:        input.IdempotencyKey,
		}
		if err := tx.Create(&transaction).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.ErrIdempotencyKeyConflict
			}
			return fmt.Errorf("create member reward transaction: %w", err)
		}

		result = domain.RewardMutation{
			Member:      storeMemberFromModel(member),
			Transaction: rewardTransactionFromModel(transaction),
		}
		return nil
	})
	if err != nil {
		return domain.RewardMutation{}, err
	}
	return result, nil
}

func storeMemberFromModel(member storeMemberModel) domain.StoreMember {
	return domain.StoreMember{
		MemberID:         member.MemberID,
		CreationDate:     member.CreationDate,
		CustomerName:     member.CustomerName,
		CustomerEmail:    member.CustomerEmail,
		Merchant:         member.Merchant,
		StampProgress:    member.StampProgress,
		AvailableRewards: member.AvailableRewards,
		Status:           member.Status,
	}
}

func rewardTransactionFromModel(transaction memberRewardTransactionModel) domain.MemberRewardTransaction {
	return domain.MemberRewardTransaction{
		TransactionID:         transaction.TransactionID,
		MemberID:              transaction.MemberID,
		MerchantID:            transaction.MerchantID,
		TransactionType:       transaction.TransactionType,
		Quantity:              transaction.Quantity,
		PreviousRewardBalance: transaction.PreviousRewardBalance,
		NewRewardBalance:      transaction.NewRewardBalance,
		PreviousStampProgress: transaction.PreviousStampProgress,
		NewStampProgress:      transaction.NewStampProgress,
		CreationDate:          transaction.CreationDate,
		IdempotencyKey:        transaction.IdempotencyKey,
	}
}

func applyRewardState(member *storeMemberModel, transactionType string, stampGoal int) error {
	if member.Status == "inactive" {
		return domain.ErrMemberInactive
	}
	switch transactionType {
	case "issue":
		if member.StampProgress != stampGoal {
			return domain.ErrRewardNotReady
		}
		member.StampProgress -= stampGoal
		member.AvailableRewards++
		member.Status = "active"
	case "redeem":
		if member.AvailableRewards < 1 {
			return domain.ErrNoAvailableRewards
		}
		member.AvailableRewards--
	default:
		return fmt.Errorf("unsupported reward transaction type %q", transactionType)
	}
	return nil
}

func nextStampState(previous int, delta int, stampGoal int, currentStatus string) (int, string) {
	next := previous + delta
	if next < 0 {
		next = 0
	}
	if next > stampGoal {
		next = stampGoal
	}

	status := currentStatus
	if next >= stampGoal {
		status = "reward_ready"
	} else if status == "reward_ready" {
		status = "active"
	}
	return next, status
}

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

type EmailRepository struct {
	db *gorm.DB
}

type merchantEmailVerificationModel struct {
	VerificationID    string     `gorm:"column:verification_id;primaryKey"`
	MerchantID        string     `gorm:"column:merchant_id;not null;index"`
	RecipientEmail    string     `gorm:"column:recipient_email;not null;index"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null"`
	Attempts          int        `gorm:"column:attempts;not null;default:0"`
	MaxAttempts       int        `gorm:"column:max_attempts;not null;default:5"`
	ResendAvailableAt time.Time  `gorm:"column:resend_available_at;not null"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (merchantEmailVerificationModel) TableName() string { return "merchant_email_verifications" }

type emailJobModel struct {
	EmailJobID     string     `gorm:"column:email_job_id;primaryKey"`
	JobType        string     `gorm:"column:job_type;not null"`
	MerchantID     string     `gorm:"column:merchant_id;not null;index"`
	VerificationID *string    `gorm:"column:verification_id"`
	RecipientEmail string     `gorm:"column:recipient_email;not null"`
	RecipientName  string     `gorm:"column:recipient_name;not null;default:''"`
	Status         string     `gorm:"column:status;not null;default:pending"`
	Attempts       int        `gorm:"column:attempts;not null;default:0"`
	MaxAttempts    int        `gorm:"column:max_attempts;not null;default:8"`
	NextAttemptAt  time.Time  `gorm:"column:next_attempt_at;not null"`
	LastError      string     `gorm:"column:last_error;not null;default:''"`
	MessageID      string     `gorm:"column:message_id;not null"`
	IdempotencyKey string     `gorm:"column:idempotency_key;not null;uniqueIndex"`
	SentAt         *time.Time `gorm:"column:sent_at"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (emailJobModel) TableName() string { return "email_jobs" }

func NewEmailRepository(db *gorm.DB) EmailRepository { return EmailRepository{db: db} }

func (r EmailRepository) CreatePendingRegistration(ctx context.Context, registration domain.PendingMerchantRegistration) (domain.EmailVerificationResult, error) {
	var result domain.EmailVerificationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		merchant, err := (MerchantRepository{db: tx}).create(ctx, tx, registration.Merchant)
		if err != nil {
			return err
		}

		issuer := appleWalletIssuerModel{
			IssuerID:         registration.Issuer.IssuerID,
			MerchantID:       registration.Issuer.MerchantID,
			OwnershipMode:    registration.Issuer.OwnershipMode,
			TeamIdentifier:   registration.Issuer.TeamIdentifier,
			OrganizationName: registration.Issuer.OrganizationName,
			Status:           registration.Issuer.Status,
		}
		if err := tx.WithContext(ctx).Create(&issuer).Error; err != nil {
			return fmt.Errorf("create pending apple wallet issuer: %w", err)
		}

		verification := emailVerificationModel(registration.Verification)
		if err := tx.WithContext(ctx).Create(&verification).Error; err != nil {
			return fmt.Errorf("create merchant email verification: %w", err)
		}
		job := emailJobModelFromDomain(registration.EmailJob)
		if err := tx.WithContext(ctx).Create(&job).Error; err != nil {
			return fmt.Errorf("enqueue verification email: %w", err)
		}

		result = domain.EmailVerificationResult{Merchant: merchant, Verification: verificationFromModel(verification)}
		return nil
	})
	if err != nil {
		return domain.EmailVerificationResult{}, err
	}
	return result, nil
}

func (r EmailRepository) VerificationByEmail(ctx context.Context, email string) (domain.EmailVerificationResult, error) {
	var merchant merchantModel
	if err := r.db.WithContext(ctx).Where("merchant_email = ?", email).First(&merchant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.EmailVerificationResult{}, domain.ErrEmailVerificationNotFound
		}
		return domain.EmailVerificationResult{}, fmt.Errorf("get merchant for email verification: %w", err)
	}
	if merchant.EmailVerifiedAt != nil {
		return domain.EmailVerificationResult{Merchant: merchantFromModel(merchant), AlreadyVerified: true}, nil
	}

	var verification merchantEmailVerificationModel
	if err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND consumed_at IS NULL", merchant.MerchantID).
		Order("created_at DESC").First(&verification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.EmailVerificationResult{}, domain.ErrEmailVerificationNotFound
		}
		return domain.EmailVerificationResult{}, fmt.Errorf("get active email verification: %w", err)
	}
	return domain.EmailVerificationResult{Merchant: merchantFromModel(merchant), Verification: verificationFromModel(verification)}, nil
}

func (r EmailRepository) ApplyVerificationAttempt(ctx context.Context, input domain.VerifyMerchantEmailInput) (domain.EmailVerificationResult, error) {
	var result domain.EmailVerificationResult
	var outcome error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Read the challenge first to discover its merchant. The merchant row is
		// locked before the challenge so verification and resend use the same
		// lock order and cannot deadlock each other.
		var verification merchantEmailVerificationModel
		if err := tx.First(&verification, "verification_id = ?", input.VerificationID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrEmailVerificationInvalid
				return nil
			}
			return fmt.Errorf("lock email verification: %w", err)
		}

		var merchant merchantModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&merchant, "merchant_id = ?", verification.MerchantID).Error; err != nil {
			return fmt.Errorf("lock merchant for verification: %w", err)
		}
		if merchant.EmailVerifiedAt != nil {
			result = domain.EmailVerificationResult{Merchant: merchantFromModel(merchant), Verification: verificationFromModel(verification), AlreadyVerified: true}
			return nil
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&verification, "verification_id = ?", input.VerificationID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrEmailVerificationInvalid
				return nil
			}
			return fmt.Errorf("lock email verification: %w", err)
		}

		now := time.Now().UTC()
		if verification.ConsumedAt != nil || !now.Before(verification.ExpiresAt) || verification.Attempts >= verification.MaxAttempts {
			outcome = domain.ErrEmailVerificationInvalid
			return nil
		}

		if !input.CodeValid {
			verification.Attempts++
			verification.UpdatedAt = now
			if err := tx.Save(&verification).Error; err != nil {
				return fmt.Errorf("record verification attempt: %w", err)
			}
			if verification.Attempts >= verification.MaxAttempts {
				outcome = domain.ErrEmailVerificationAttemptsExhausted
			} else {
				outcome = domain.ErrEmailVerificationInvalid
			}
			return nil
		}

		verification.ConsumedAt = &now
		verification.UpdatedAt = now
		if err := tx.Save(&verification).Error; err != nil {
			return fmt.Errorf("consume email verification: %w", err)
		}
		merchant.EmailVerifiedAt = &now
		merchant.Status = "active"
		if err := tx.Save(&merchant).Error; err != nil {
			return fmt.Errorf("activate verified merchant: %w", err)
		}
		welcome := emailJobModelFromDomain(input.WelcomeJob)
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(&welcome).Error; err != nil {
			return fmt.Errorf("enqueue merchant welcome email: %w", err)
		}
		result = domain.EmailVerificationResult{Merchant: merchantFromModel(merchant), Verification: verificationFromModel(verification)}
		return nil
	})
	if err != nil {
		return domain.EmailVerificationResult{}, err
	}
	if outcome != nil {
		return domain.EmailVerificationResult{}, outcome
	}
	return result, nil
}

func (r EmailRepository) ResendVerification(ctx context.Context, verification domain.MerchantEmailVerification, job domain.EmailJob) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var merchant merchantModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("merchant_email = ?", verification.RecipientEmail).First(&merchant).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrEmailVerificationNotFound
			}
			return fmt.Errorf("lock merchant for verification resend: %w", err)
		}
		if merchant.EmailVerifiedAt != nil {
			return nil
		}

		var latest merchantEmailVerificationModel
		err := tx.Where("merchant_id = ?", merchant.MerchantID).Order("created_at DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("get latest verification for resend: %w", err)
		}
		now := time.Now().UTC()
		if err == nil && now.Before(latest.ResendAvailableAt) {
			return domain.ErrEmailVerificationRateLimited
		}
		var recent int64
		if err := tx.Model(&merchantEmailVerificationModel{}).
			Where("merchant_id = ? AND created_at >= ?", merchant.MerchantID, now.Add(-time.Hour)).Count(&recent).Error; err != nil {
			return fmt.Errorf("count recent email verifications: %w", err)
		}
		var firstCreatedAt time.Time
		if err := tx.Model(&merchantEmailVerificationModel{}).
			Where("merchant_id = ?", merchant.MerchantID).
			Select("MIN(created_at)").Scan(&firstCreatedAt).Error; err != nil {
			return fmt.Errorf("get initial email verification: %w", err)
		}
		limit := int64(5)
		if firstCreatedAt.After(now.Add(-time.Hour)) {
			limit++
		}
		if recent >= limit {
			return domain.ErrEmailVerificationRateLimited
		}
		if err := tx.Model(&merchantEmailVerificationModel{}).
			Where("merchant_id = ? AND consumed_at IS NULL", merchant.MerchantID).
			Updates(map[string]any{"consumed_at": now, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("invalidate previous email verification: %w", err)
		}
		if err := tx.Model(&emailJobModel{}).
			Where("merchant_id = ? AND job_type = ? AND status IN ?", merchant.MerchantID, "merchant_verification", []string{"pending", "failed", "processing"}).
			Updates(map[string]any{"status": "failed", "attempts": gorm.Expr("max_attempts"), "last_error": "verification superseded by resend", "updated_at": now}).Error; err != nil {
			return fmt.Errorf("supersede previous verification email jobs: %w", err)
		}

		verification.MerchantID = merchant.MerchantID
		model := emailVerificationModel(verification)
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create replacement email verification: %w", err)
		}
		job.MerchantID = merchant.MerchantID
		jobModel := emailJobModelFromDomain(job)
		if err := tx.Create(&jobModel).Error; err != nil {
			return fmt.Errorf("enqueue replacement verification email: %w", err)
		}
		return nil
	})
}

func (r EmailRepository) MerchantByID(ctx context.Context, merchantID string) (domain.Merchant, error) {
	var merchant merchantModel
	if err := r.db.WithContext(ctx).First(&merchant, "merchant_id = ?", merchantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Merchant{}, domain.ErrMerchantNotFound
		}
		return domain.Merchant{}, fmt.Errorf("get merchant for email: %w", err)
	}
	return merchantFromModel(merchant), nil
}

func (r EmailRepository) ClaimPendingEmailJobs(ctx context.Context, limit int) ([]domain.EmailJob, error) {
	if limit < 1 {
		limit = 20
	}
	var models []emailJobModel
	err := r.db.WithContext(ctx).Raw(`
		WITH picked AS (
			SELECT email_job_id FROM email_jobs
			WHERE (status IN ('pending', 'failed') OR (status = 'processing' AND updated_at < NOW() - INTERVAL '10 minutes'))
			  AND attempts < max_attempts AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at, created_at
			FOR UPDATE SKIP LOCKED LIMIT ?
		)
		UPDATE email_jobs AS job
		SET status = 'processing', attempts = attempts + 1, updated_at = NOW()
		FROM picked WHERE job.email_job_id = picked.email_job_id
		RETURNING job.*
	`, limit).Scan(&models).Error
	if err != nil {
		return nil, fmt.Errorf("claim email jobs: %w", err)
	}
	jobs := make([]domain.EmailJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, emailJobFromModel(model))
	}
	return jobs, nil
}

func (r EmailRepository) CompleteEmailJob(ctx context.Context, jobID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&emailJobModel{}).Where("email_job_id = ?", jobID).
		Updates(map[string]any{"status": "sent", "sent_at": now, "last_error": "", "updated_at": now}).Error
}

func (r EmailRepository) RetryEmailJob(ctx context.Context, job domain.EmailJob, message string) error {
	delay := time.Duration(1<<minInt(job.Attempts, 6)) * time.Minute
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&emailJobModel{}).Where("email_job_id = ?", job.EmailJobID).
		Updates(map[string]any{"status": "failed", "last_error": message, "next_attempt_at": now.Add(delay), "updated_at": now}).Error
}

func (r EmailRepository) FailEmailJob(ctx context.Context, jobID, message string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&emailJobModel{}).Where("email_job_id = ?", jobID).
		Updates(map[string]any{"status": "failed", "attempts": gorm.Expr("max_attempts"), "last_error": message, "updated_at": now}).Error
}

func emailVerificationModel(v domain.MerchantEmailVerification) merchantEmailVerificationModel {
	return merchantEmailVerificationModel{VerificationID: v.VerificationID, MerchantID: v.MerchantID, RecipientEmail: v.RecipientEmail, ExpiresAt: v.ExpiresAt, Attempts: v.Attempts, MaxAttempts: v.MaxAttempts, ResendAvailableAt: v.ResendAvailableAt, ConsumedAt: v.ConsumedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func verificationFromModel(v merchantEmailVerificationModel) domain.MerchantEmailVerification {
	return domain.MerchantEmailVerification{VerificationID: v.VerificationID, MerchantID: v.MerchantID, RecipientEmail: v.RecipientEmail, ExpiresAt: v.ExpiresAt, Attempts: v.Attempts, MaxAttempts: v.MaxAttempts, ResendAvailableAt: v.ResendAvailableAt, ConsumedAt: v.ConsumedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func emailJobModelFromDomain(job domain.EmailJob) emailJobModel {
	var verificationID *string
	if job.VerificationID != "" {
		verificationID = &job.VerificationID
	}
	return emailJobModel{EmailJobID: job.EmailJobID, JobType: job.JobType, MerchantID: job.MerchantID, VerificationID: verificationID, RecipientEmail: job.RecipientEmail, RecipientName: job.RecipientName, Status: job.Status, Attempts: job.Attempts, MaxAttempts: job.MaxAttempts, NextAttemptAt: job.NextAttemptAt, LastError: job.LastError, MessageID: job.MessageID, IdempotencyKey: job.IdempotencyKey, SentAt: job.SentAt, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt}
}

func emailJobFromModel(job emailJobModel) domain.EmailJob {
	verificationID := ""
	if job.VerificationID != nil {
		verificationID = *job.VerificationID
	}
	return domain.EmailJob{EmailJobID: job.EmailJobID, JobType: job.JobType, MerchantID: job.MerchantID, VerificationID: verificationID, RecipientEmail: job.RecipientEmail, RecipientName: job.RecipientName, Status: job.Status, Attempts: job.Attempts, MaxAttempts: job.MaxAttempts, NextAttemptAt: job.NextAttemptAt, LastError: job.LastError, MessageID: job.MessageID, IdempotencyKey: job.IdempotencyKey, SentAt: job.SentAt, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt}
}

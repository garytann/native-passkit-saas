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

type GoogleWalletRepository struct{ db *gorm.DB }

type loyaltyProgramModel struct {
	ProgramID           string    `gorm:"column:program_id;primaryKey"`
	MerchantID          string    `gorm:"column:merchant_id;uniqueIndex;not null"`
	ProgramName         string    `gorm:"column:program_name;not null"`
	PassDescription     string    `gorm:"column:pass_description;not null"`
	StampGoal           int       `gorm:"column:stamp_goal;not null"`
	BackgroundColor     string    `gorm:"column:background_color;not null"`
	AssetBundleRef      string    `gorm:"column:asset_bundle_ref;not null"`
	StampInstructions   string    `gorm:"column:stamp_instructions;not null"`
	RewardTitle         string    `gorm:"column:reward_title;not null"`
	RewardDetails       string    `gorm:"column:reward_details;not null"`
	SupportText         string    `gorm:"column:support_text;not null"`
	TermsText           string    `gorm:"column:terms_text;not null"`
	CampaignDisplayMode string    `gorm:"column:campaign_display_mode;not null"`
	StampIconKey        string    `gorm:"column:stamp_icon_key;not null"`
	Status              string    `gorm:"column:status;not null"`
	Version             int       `gorm:"column:version;not null"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
}

func (loyaltyProgramModel) TableName() string { return "loyalty_programs" }

type loyaltyProgramLinkModel struct {
	LinkID    string    `gorm:"column:link_id;primaryKey"`
	ProgramID string    `gorm:"column:program_id;not null"`
	LinkType  string    `gorm:"column:link_type;not null"`
	Label     string    `gorm:"column:label;not null"`
	URL       string    `gorm:"column:url;not null"`
	SortOrder int       `gorm:"column:sort_order;not null"`
	Status    string    `gorm:"column:status;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (loyaltyProgramLinkModel) TableName() string { return "loyalty_program_links" }

type googleWalletClassModel struct {
	GoogleClassID        string     `gorm:"column:google_class_id;primaryKey"`
	ProgramID            string     `gorm:"column:program_id;uniqueIndex;not null"`
	IssuerID             string     `gorm:"column:issuer_id;not null"`
	ClassSuffix          string     `gorm:"column:class_suffix;not null"`
	ReviewStatus         string     `gorm:"column:review_status;not null"`
	Status               string     `gorm:"column:status;not null"`
	ProgramLogoRef       string     `gorm:"column:program_logo_ref;not null"`
	WideLogoRef          string     `gorm:"column:wide_logo_ref;not null"`
	SyncedProgramVersion int        `gorm:"column:synced_program_version;not null"`
	LastSyncedAt         *time.Time `gorm:"column:last_synced_at"`
	LastError            string     `gorm:"column:last_error;not null"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (googleWalletClassModel) TableName() string { return "google_wallet_classes" }

type googleWalletObjectModel struct {
	GoogleObjectID         string     `gorm:"column:google_object_id;primaryKey"`
	MemberID               string     `gorm:"column:member_id;uniqueIndex;not null"`
	GoogleClassID          string     `gorm:"column:google_class_id;not null"`
	ImageToken             string     `gorm:"column:image_token;uniqueIndex;not null"`
	Status                 string     `gorm:"column:status;not null"`
	SyncedStampProgress    int        `gorm:"column:synced_stamp_progress;not null"`
	SyncedAvailableRewards int        `gorm:"column:synced_available_rewards;not null"`
	SyncedProgramVersion   int        `gorm:"column:synced_program_version;not null"`
	LastSyncedAt           *time.Time `gorm:"column:last_synced_at"`
	LastError              string     `gorm:"column:last_error;not null"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (googleWalletObjectModel) TableName() string { return "google_wallet_objects" }

type walletSyncJobModel struct {
	SyncJobID      string     `gorm:"column:sync_job_id;primaryKey"`
	Platform       string     `gorm:"column:platform;not null"`
	TargetType     string     `gorm:"column:target_type;not null"`
	TargetID       string     `gorm:"column:target_id;not null"`
	DesiredVersion string     `gorm:"column:desired_version;not null"`
	Status         string     `gorm:"column:status;not null"`
	Attempts       int        `gorm:"column:attempts;not null"`
	MaxAttempts    int        `gorm:"column:max_attempts;not null"`
	NextAttemptAt  time.Time  `gorm:"column:next_attempt_at;not null"`
	LastError      string     `gorm:"column:last_error;not null"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at"`
}

func (walletSyncJobModel) TableName() string { return "wallet_sync_jobs" }

func NewGoogleWalletRepository(db *gorm.DB) GoogleWalletRepository {
	return GoogleWalletRepository{db: db}
}

func (r GoogleWalletRepository) ProgramByMerchantID(ctx context.Context, merchantID string) (domain.LoyaltyProgram, error) {
	var model loyaltyProgramModel
	if err := r.db.WithContext(ctx).Where("merchant_id = ?", merchantID).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.LoyaltyProgram{}, domain.ErrLoyaltyProgramNotFound
		}
		return domain.LoyaltyProgram{}, fmt.Errorf("get loyalty program: %w", err)
	}
	return r.programWithLinks(ctx, model)
}

func (r GoogleWalletRepository) ProgramByID(ctx context.Context, programID string) (domain.LoyaltyProgram, error) {
	var model loyaltyProgramModel
	if err := r.db.WithContext(ctx).Where("program_id = ?", programID).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.LoyaltyProgram{}, domain.ErrLoyaltyProgramNotFound
		}
		return domain.LoyaltyProgram{}, fmt.Errorf("get loyalty program by id: %w", err)
	}
	return r.programWithLinks(ctx, model)
}

func (r GoogleWalletRepository) EnsureProgram(ctx context.Context, merchant domain.Merchant, programID string) (domain.LoyaltyProgram, error) {
	program, err := r.ProgramByMerchantID(ctx, merchant.MerchantID)
	if err == nil {
		return program, nil
	}
	if !errors.Is(err, domain.ErrLoyaltyProgramNotFound) {
		return domain.LoyaltyProgram{}, err
	}
	now := time.Now().UTC()
	model := loyaltyProgramModel{
		ProgramID: programID, MerchantID: merchant.MerchantID,
		ProgramName:     merchant.Merchant + " Loyalty Card",
		PassDescription: merchant.Merchant + " Loyalty Card",
		StampGoal:       10, BackgroundColor: "#5434F5", AssetBundleRef: merchant.MerchantSlug,
		StampInstructions: "Present this pass when making an eligible purchase to collect a stamp.",
		RewardTitle:       "Loyalty Reward", RewardDetails: "Complete your stamp card to receive the merchant reward.",
		SupportText:         "Contact " + merchant.Merchant + " for loyalty card assistance.",
		TermsText:           "Collect one stamp for each eligible purchase. Rewards are subject to outlet terms.",
		CampaignDisplayMode: "details", StampIconKey: "logo", Status: "draft", Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return r.ProgramByMerchantID(ctx, merchant.MerchantID)
		}
		return domain.LoyaltyProgram{}, fmt.Errorf("create loyalty program: %w", err)
	}
	return loyaltyProgramFromModel(model), nil
}

func (r GoogleWalletRepository) UpdateProgram(ctx context.Context, input domain.UpdateLoyaltyProgramInput) (domain.LoyaltyProgram, error) {
	var programID string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current loyaltyProgramModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("merchant_id = ?", input.MerchantID).Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrLoyaltyProgramNotFound
			}
			return err
		}
		programID = current.ProgramID
		updates := map[string]any{
			"program_name": input.ProgramName, "pass_description": input.PassDescription,
			"stamp_goal": input.StampGoal, "background_color": input.BackgroundColor,
			"stamp_instructions": input.StampInstructions, "reward_title": input.RewardTitle,
			"reward_details": input.RewardDetails, "support_text": input.SupportText,
			"terms_text": input.TermsText, "campaign_display_mode": input.CampaignDisplayMode,
			"stamp_icon_key": input.StampIconKey, "version": current.Version + 1,
			"updated_at": time.Now().UTC(),
		}
		if err := tx.Model(&loyaltyProgramModel{}).Where("program_id = ?", programID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("program_id = ?", programID).Delete(&loyaltyProgramLinkModel{}).Error; err != nil {
			return err
		}
		for _, link := range input.Links {
			model := loyaltyProgramLinkModel{LinkID: link.LinkID, ProgramID: programID, LinkType: link.LinkType, Label: link.Label, URL: link.URL, SortOrder: link.SortOrder, Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.LoyaltyProgram{}, fmt.Errorf("update loyalty program: %w", err)
	}
	return r.ProgramByID(ctx, programID)
}

func (r GoogleWalletRepository) SetProgramStatus(ctx context.Context, programID, status string) error {
	return r.db.WithContext(ctx).Model(&loyaltyProgramModel{}).Where("program_id = ?", programID).Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error
}

func (r GoogleWalletRepository) BumpProgramVersion(ctx context.Context, programID string) (domain.LoyaltyProgram, error) {
	result := r.db.WithContext(ctx).Model(&loyaltyProgramModel{}).Where("program_id = ?", programID).Updates(map[string]any{"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return domain.LoyaltyProgram{}, fmt.Errorf("increment loyalty program version: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.LoyaltyProgram{}, domain.ErrLoyaltyProgramNotFound
	}
	return r.ProgramByID(ctx, programID)
}

func (r GoogleWalletRepository) ProgramByMerchantName(ctx context.Context, merchant string) (domain.LoyaltyProgram, error) {
	var model loyaltyProgramModel
	err := r.db.WithContext(ctx).Table("loyalty_programs AS program").Select("program.*").
		Joins("INNER JOIN merchants AS merchant ON merchant.merchant_id = program.merchant_id").
		Where("merchant.merchant = ?", merchant).Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.LoyaltyProgram{}, domain.ErrLoyaltyProgramNotFound
		}
		return domain.LoyaltyProgram{}, fmt.Errorf("get loyalty program by merchant: %w", err)
	}
	return r.programWithLinks(ctx, model)
}

func (r GoogleWalletRepository) ClassByMerchantID(ctx context.Context, merchantID string) (domain.GoogleWalletClass, error) {
	var model googleWalletClassModel
	err := r.db.WithContext(ctx).Table("google_wallet_classes AS class").Select("class.*").
		Joins("INNER JOIN loyalty_programs AS program ON program.program_id = class.program_id").
		Where("program.merchant_id = ?", merchantID).Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletClass{}, domain.ErrGoogleWalletClassNotFound
		}
		return domain.GoogleWalletClass{}, fmt.Errorf("get google wallet class: %w", err)
	}
	return googleClassFromModel(model), nil
}

func (r GoogleWalletRepository) ActiveClassByMerchantName(ctx context.Context, merchant string) (domain.GoogleWalletClass, domain.LoyaltyProgram, error) {
	program, err := r.ProgramByMerchantName(ctx, merchant)
	if err != nil {
		return domain.GoogleWalletClass{}, domain.LoyaltyProgram{}, err
	}
	var model googleWalletClassModel
	if err := r.db.WithContext(ctx).Where("program_id = ? AND status = ? AND review_status = ?", program.ProgramID, "active", "APPROVED").Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletClass{}, domain.LoyaltyProgram{}, domain.ErrGoogleWalletNotProvisioned
		}
		return domain.GoogleWalletClass{}, domain.LoyaltyProgram{}, fmt.Errorf("resolve active google wallet class: %w", err)
	}
	return googleClassFromModel(model), program, nil
}

func (r GoogleWalletRepository) UpsertClass(ctx context.Context, input domain.UpsertGoogleWalletClassInput) (domain.GoogleWalletClass, error) {
	now := time.Now().UTC()
	model := googleWalletClassModel{GoogleClassID: input.GoogleClassID, ProgramID: input.ProgramID, IssuerID: input.IssuerID, ClassSuffix: input.ClassSuffix, ReviewStatus: input.ReviewStatus, Status: input.Status, ProgramLogoRef: input.ProgramLogoRef, WideLogoRef: input.WideLogoRef, UpdatedAt: now}
	updates := map[string]any{"issuer_id": input.IssuerID, "class_suffix": input.ClassSuffix, "review_status": input.ReviewStatus, "status": input.Status, "program_logo_ref": input.ProgramLogoRef, "wide_logo_ref": input.WideLogoRef, "updated_at": now}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "program_id"}}, DoUpdates: clause.Assignments(updates)}).Create(&model).Error; err != nil {
		return domain.GoogleWalletClass{}, fmt.Errorf("upsert google wallet class: %w", err)
	}
	return r.ClassByProgramID(ctx, input.ProgramID)
}

func (r GoogleWalletRepository) ClassByProgramID(ctx context.Context, programID string) (domain.GoogleWalletClass, error) {
	var model googleWalletClassModel
	if err := r.db.WithContext(ctx).Where("program_id = ?", programID).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletClass{}, domain.ErrGoogleWalletClassNotFound
		}
		return domain.GoogleWalletClass{}, fmt.Errorf("get google wallet class by program: %w", err)
	}
	return googleClassFromModel(model), nil
}

func (r GoogleWalletRepository) ClassByID(ctx context.Context, classID string) (domain.GoogleWalletClass, error) {
	var model googleWalletClassModel
	if err := r.db.WithContext(ctx).Where("google_class_id = ?", classID).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletClass{}, domain.ErrGoogleWalletClassNotFound
		}
		return domain.GoogleWalletClass{}, fmt.Errorf("get google wallet class by id: %w", err)
	}
	return googleClassFromModel(model), nil
}

func (r GoogleWalletRepository) SetClassState(ctx context.Context, classID, reviewStatus, status, lastError string, version int) error {
	updates := map[string]any{"review_status": reviewStatus, "status": status, "last_error": lastError, "updated_at": time.Now().UTC()}
	if lastError == "" {
		now := time.Now().UTC()
		updates["last_synced_at"] = now
		updates["synced_program_version"] = version
	}
	return r.db.WithContext(ctx).Model(&googleWalletClassModel{}).Where("google_class_id = ?", classID).Updates(updates).Error
}

func (r GoogleWalletRepository) GetOrCreateObject(ctx context.Context, input domain.UpsertGoogleWalletObjectInput) (domain.GoogleWalletObject, error) {
	model := googleWalletObjectModel{GoogleObjectID: input.GoogleObjectID, MemberID: input.MemberID, GoogleClassID: input.GoogleClassID, ImageToken: input.ImageToken, Status: "provisioned"}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "member_id"}}, DoNothing: true}).Create(&model).Error; err != nil {
		return domain.GoogleWalletObject{}, fmt.Errorf("create google wallet object: %w", err)
	}
	return r.ObjectByMemberID(ctx, input.MemberID)
}

func (r GoogleWalletRepository) ObjectByMemberID(ctx context.Context, memberID string) (domain.GoogleWalletObject, error) {
	var model googleWalletObjectModel
	if err := r.db.WithContext(ctx).Where("member_id = ?", memberID).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletObject{}, domain.ErrGoogleWalletObjectNotFound
		}
		return domain.GoogleWalletObject{}, fmt.Errorf("get google wallet object: %w", err)
	}
	return googleObjectFromModel(model), nil
}

func (r GoogleWalletRepository) ObjectByImageToken(ctx context.Context, token string) (domain.GoogleWalletObject, error) {
	var model googleWalletObjectModel
	if err := r.db.WithContext(ctx).Where("image_token = ?", token).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.GoogleWalletObject{}, domain.ErrGoogleWalletObjectNotFound
		}
		return domain.GoogleWalletObject{}, fmt.Errorf("get google wallet image object: %w", err)
	}
	return googleObjectFromModel(model), nil
}

func (r GoogleWalletRepository) ObjectsByClassID(ctx context.Context, classID string) ([]domain.GoogleWalletObject, error) {
	var models []googleWalletObjectModel
	if err := r.db.WithContext(ctx).Where("google_class_id = ?", classID).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list google wallet objects: %w", err)
	}
	objects := make([]domain.GoogleWalletObject, 0, len(models))
	for _, model := range models {
		objects = append(objects, googleObjectFromModel(model))
	}
	return objects, nil
}

func (r GoogleWalletRepository) MarkObjectSynced(ctx context.Context, objectID string, member domain.StoreMember, programVersion int) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&googleWalletObjectModel{}).Where("google_object_id = ?", objectID).Updates(map[string]any{"status": "active", "synced_stamp_progress": member.StampProgress, "synced_available_rewards": member.AvailableRewards, "synced_program_version": programVersion, "last_synced_at": now, "last_error": "", "updated_at": now}).Error
}

func (r GoogleWalletRepository) MarkObjectError(ctx context.Context, objectID, message string) error {
	return r.db.WithContext(ctx).Model(&googleWalletObjectModel{}).Where("google_object_id = ?", objectID).Updates(map[string]any{"status": "sync_error", "last_error": message, "updated_at": time.Now().UTC()}).Error
}

func (r GoogleWalletRepository) EnqueueSync(ctx context.Context, input domain.EnqueueWalletSyncJobInput) error {
	model := walletSyncJobModel{SyncJobID: input.SyncJobID, Platform: input.Platform, TargetType: input.TargetType, TargetID: input.TargetID, DesiredVersion: input.DesiredVersion, Status: "pending", MaxAttempts: 8, NextAttemptAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "platform"}, {Name: "target_type"}, {Name: "target_id"}, {Name: "desired_version"}}, DoNothing: true}).Create(&model).Error
}

func (r GoogleWalletRepository) ClaimPendingSyncJobs(ctx context.Context, limit int) ([]domain.WalletSyncJob, error) {
	if limit < 1 {
		limit = 20
	}
	var models []walletSyncJobModel
	err := r.db.WithContext(ctx).Raw(`
		WITH picked AS (
			SELECT sync_job_id FROM wallet_sync_jobs
			WHERE platform = 'google'
			  AND (status IN ('pending', 'failed') OR (status = 'processing' AND updated_at < NOW() - INTERVAL '10 minutes'))
			  AND attempts < max_attempts AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at, created_at
			FOR UPDATE SKIP LOCKED LIMIT ?
		)
		UPDATE wallet_sync_jobs AS job
		SET status = 'processing', attempts = attempts + 1, updated_at = NOW()
		FROM picked WHERE job.sync_job_id = picked.sync_job_id
		RETURNING job.*
	`, limit).Scan(&models).Error
	if err != nil {
		return nil, fmt.Errorf("claim wallet sync jobs: %w", err)
	}
	jobs := make([]domain.WalletSyncJob, 0, len(models))
	for _, model := range models {
		jobs = append(jobs, walletSyncJobFromModel(model))
	}
	return jobs, nil
}

func (r GoogleWalletRepository) CompleteSyncJob(ctx context.Context, syncJobID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&walletSyncJobModel{}).Where("sync_job_id = ?", syncJobID).Updates(map[string]any{"status": "completed", "completed_at": now, "last_error": "", "updated_at": now}).Error
}

func (r GoogleWalletRepository) RetrySyncJob(ctx context.Context, job domain.WalletSyncJob, message string) error {
	delay := time.Duration(1<<minInt(job.Attempts, 6)) * time.Minute
	status := "failed"
	return r.db.WithContext(ctx).Model(&walletSyncJobModel{}).Where("sync_job_id = ?", job.SyncJobID).Updates(map[string]any{"status": status, "last_error": message, "next_attempt_at": time.Now().UTC().Add(delay), "updated_at": time.Now().UTC()}).Error
}

func (r GoogleWalletRepository) programWithLinks(ctx context.Context, model loyaltyProgramModel) (domain.LoyaltyProgram, error) {
	program := loyaltyProgramFromModel(model)
	var links []loyaltyProgramLinkModel
	if err := r.db.WithContext(ctx).Where("program_id = ? AND status = ?", model.ProgramID, "active").Order("sort_order ASC, created_at ASC").Find(&links).Error; err != nil {
		return domain.LoyaltyProgram{}, fmt.Errorf("list loyalty program links: %w", err)
	}
	for _, link := range links {
		program.Links = append(program.Links, domain.ApplePassLink{LinkID: link.LinkID, LinkType: link.LinkType, Label: link.Label, URL: link.URL, SortOrder: link.SortOrder})
	}
	return program, nil
}

func loyaltyProgramFromModel(m loyaltyProgramModel) domain.LoyaltyProgram {
	return domain.LoyaltyProgram{ProgramID: m.ProgramID, MerchantID: m.MerchantID, ProgramName: m.ProgramName, PassDescription: m.PassDescription, StampGoal: m.StampGoal, BackgroundColor: m.BackgroundColor, AssetBundleRef: m.AssetBundleRef, StampInstructions: m.StampInstructions, RewardTitle: m.RewardTitle, RewardDetails: m.RewardDetails, SupportText: m.SupportText, TermsText: m.TermsText, CampaignDisplayMode: m.CampaignDisplayMode, StampIconKey: m.StampIconKey, Status: m.Status, Version: m.Version, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func googleClassFromModel(m googleWalletClassModel) domain.GoogleWalletClass {
	return domain.GoogleWalletClass{GoogleClassID: m.GoogleClassID, ProgramID: m.ProgramID, IssuerID: m.IssuerID, ClassSuffix: m.ClassSuffix, ReviewStatus: m.ReviewStatus, Status: m.Status, ProgramLogoRef: m.ProgramLogoRef, WideLogoRef: m.WideLogoRef, SyncedProgramVersion: m.SyncedProgramVersion, LastSyncedAt: m.LastSyncedAt, LastError: m.LastError, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func googleObjectFromModel(m googleWalletObjectModel) domain.GoogleWalletObject {
	return domain.GoogleWalletObject{GoogleObjectID: m.GoogleObjectID, MemberID: m.MemberID, GoogleClassID: m.GoogleClassID, ImageToken: m.ImageToken, Status: m.Status, SyncedStampProgress: m.SyncedStampProgress, SyncedAvailableRewards: m.SyncedAvailableRewards, SyncedProgramVersion: m.SyncedProgramVersion, LastSyncedAt: m.LastSyncedAt, LastError: m.LastError, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func walletSyncJobFromModel(m walletSyncJobModel) domain.WalletSyncJob {
	return domain.WalletSyncJob{SyncJobID: m.SyncJobID, Platform: m.Platform, TargetType: m.TargetType, TargetID: m.TargetID, DesiredVersion: m.DesiredVersion, Status: m.Status, Attempts: m.Attempts, MaxAttempts: m.MaxAttempts, NextAttemptAt: m.NextAttemptAt, LastError: m.LastError, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, CompletedAt: m.CompletedAt}
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

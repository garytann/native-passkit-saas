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

type AppleWalletRepository struct {
	db *gorm.DB
}

type appleWalletIssuerModel struct {
	IssuerID         string    `gorm:"column:issuer_id;primaryKey"`
	MerchantID       string    `gorm:"column:merchant_id;uniqueIndex;not null"`
	OwnershipMode    string    `gorm:"column:ownership_mode;not null"`
	TeamIdentifier   string    `gorm:"column:team_identifier;not null"`
	OrganizationName string    `gorm:"column:organization_name;not null"`
	Status           string    `gorm:"column:status;not null"`
	CreatedAt        time.Time `gorm:"column:created_at;default:now()"`
	UpdatedAt        time.Time `gorm:"column:updated_at;default:now()"`
}

func (appleWalletIssuerModel) TableName() string {
	return "apple_wallet_issuers"
}

type applePassTypeModel struct {
	ApplePassTypeID    string    `gorm:"column:apple_pass_type_id;primaryKey"`
	IssuerID           string    `gorm:"column:issuer_id;not null;index"`
	PassTypeIdentifier string    `gorm:"column:pass_type_identifier;uniqueIndex;not null"`
	PassStyle          string    `gorm:"column:pass_style;not null"`
	Status             string    `gorm:"column:status;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;default:now()"`
	UpdatedAt          time.Time `gorm:"column:updated_at;default:now()"`
}

func (applePassTypeModel) TableName() string {
	return "apple_pass_types"
}

type appleSigningCertificateModel struct {
	CertificateID          string     `gorm:"column:certificate_id;primaryKey"`
	ApplePassTypeID        string     `gorm:"column:apple_pass_type_id;not null;index"`
	CertificateRef         string     `gorm:"column:certificate_ref;not null"`
	PrivateKeyRef          string     `gorm:"column:private_key_ref;not null"`
	CertificateFingerprint string     `gorm:"column:certificate_fingerprint;uniqueIndex;not null"`
	CertificateSerial      string     `gorm:"column:certificate_serial_number;not null"`
	ValidFrom              time.Time  `gorm:"column:valid_from;not null"`
	ExpiresAt              time.Time  `gorm:"column:expires_at;not null;index"`
	Status                 string     `gorm:"column:status;not null"`
	CreatedAt              time.Time  `gorm:"column:created_at;default:now()"`
	ActivatedAt            *time.Time `gorm:"column:activated_at"`
}

type applePassDesignModel struct {
	DesignID            string    `gorm:"column:design_id;primaryKey"`
	ApplePassTypeID     string    `gorm:"column:apple_pass_type_id;uniqueIndex;not null"`
	ProgramName         string    `gorm:"column:program_name;not null"`
	PassDescription     string    `gorm:"column:pass_description;not null"`
	StampGoal           int       `gorm:"column:stamp_goal;not null"`
	BackgroundColor     string    `gorm:"column:background_color;not null"`
	ForegroundColor     string    `gorm:"column:foreground_color;not null"`
	LabelColor          string    `gorm:"column:label_color;not null"`
	AssetBundleRef      string    `gorm:"column:asset_bundle_ref;not null"`
	SupportText         string    `gorm:"column:support_text;not null"`
	TermsText           string    `gorm:"column:terms_text;not null"`
	CampaignDisplayMode string    `gorm:"column:campaign_display_mode;not null"`
	Status              string    `gorm:"column:status;not null"`
	Version             int       `gorm:"column:version;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;default:now()"`
	UpdatedAt           time.Time `gorm:"column:updated_at;default:now()"`
}

func (applePassDesignModel) TableName() string {
	return "apple_pass_designs"
}

func (appleSigningCertificateModel) TableName() string {
	return "apple_signing_certificates"
}

type activeAppleWalletCredentialRow struct {
	MerchantID             string    `gorm:"column:merchant_id"`
	IssuerID               string    `gorm:"column:issuer_id"`
	ApplePassTypeID        string    `gorm:"column:apple_pass_type_id"`
	CertificateID          string    `gorm:"column:certificate_id"`
	OrganizationName       string    `gorm:"column:organization_name"`
	TeamIdentifier         string    `gorm:"column:team_identifier"`
	PassTypeIdentifier     string    `gorm:"column:pass_type_identifier"`
	PassStyle              string    `gorm:"column:pass_style"`
	CertificateRef         string    `gorm:"column:certificate_ref"`
	PrivateKeyRef          string    `gorm:"column:private_key_ref"`
	CertificateFingerprint string    `gorm:"column:certificate_fingerprint"`
	ExpiresAt              time.Time `gorm:"column:expires_at"`
}

func NewAppleWalletRepository(db *gorm.DB) AppleWalletRepository {
	return AppleWalletRepository{db: db}
}

func (r AppleWalletRepository) GetIssuerByMerchantID(ctx context.Context, merchantID string) (domain.AppleWalletIssuer, error) {
	var model appleWalletIssuerModel
	if err := r.db.WithContext(ctx).First(&model, "merchant_id = ?", merchantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AppleWalletIssuer{}, domain.ErrAppleWalletIssuerNotFound
		}
		return domain.AppleWalletIssuer{}, fmt.Errorf("get apple wallet issuer: %w", err)
	}
	return appleWalletIssuerFromModel(model), nil
}

func (r AppleWalletRepository) UpdateIssuer(ctx context.Context, input domain.UpdateAppleWalletIssuerInput) (domain.AppleWalletIssuer, error) {
	result := r.db.WithContext(ctx).
		Model(&appleWalletIssuerModel{}).
		Where("merchant_id = ?", input.MerchantID).
		Updates(map[string]any{
			"ownership_mode":    input.OwnershipMode,
			"team_identifier":   input.TeamIdentifier,
			"organization_name": input.OrganizationName,
			"updated_at":        time.Now().UTC(),
		})
	if result.Error != nil {
		return domain.AppleWalletIssuer{}, fmt.Errorf("update apple wallet issuer: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		model := appleWalletIssuerModel{
			IssuerID:         input.IssuerID,
			MerchantID:       input.MerchantID,
			OwnershipMode:    input.OwnershipMode,
			TeamIdentifier:   input.TeamIdentifier,
			OrganizationName: input.OrganizationName,
			Status:           "pending",
		}
		if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.AppleWalletIssuer{}, domain.ErrAppleWalletConfigurationConflict
			}
			return domain.AppleWalletIssuer{}, fmt.Errorf("create apple wallet issuer: %w", err)
		}
	}
	return r.GetIssuerByMerchantID(ctx, input.MerchantID)
}

func (r AppleWalletRepository) CreatePassType(ctx context.Context, input domain.CreateApplePassTypeInput) (domain.ApplePassType, error) {
	issuer, err := r.GetIssuerByMerchantID(ctx, input.MerchantID)
	if err != nil {
		return domain.ApplePassType{}, err
	}
	model := applePassTypeModel{
		ApplePassTypeID:    input.ApplePassTypeID,
		IssuerID:           issuer.IssuerID,
		PassTypeIdentifier: input.PassTypeIdentifier,
		PassStyle:          input.PassStyle,
		Status:             "pending",
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ApplePassType{}, domain.ErrAppleWalletConfigurationConflict
		}
		return domain.ApplePassType{}, fmt.Errorf("create apple pass type: %w", err)
	}
	return applePassTypeFromModel(model), nil
}

func (r AppleWalletRepository) ActivateCertificate(ctx context.Context, input domain.ActivateAppleSigningCertificateInput) (domain.AppleSigningCertificate, error) {
	var activated appleSigningCertificateModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var passType applePassTypeModel
		if err := tx.Table("apple_pass_types AS apt").
			Select("apt.*").
			Joins("INNER JOIN apple_wallet_issuers AS awi ON awi.issuer_id = apt.issuer_id").
			Where("awi.merchant_id = ? AND apt.pass_type_identifier = ?", input.MerchantID, input.PassTypeIdentifier).
			Take(&passType).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrAppleWalletPassTypeNotFound
			}
			return fmt.Errorf("find apple pass type: %w", err)
		}

		if err := tx.Model(&appleSigningCertificateModel{}).
			Where("apple_pass_type_id = ? AND status = ?", passType.ApplePassTypeID, "active").
			Updates(map[string]any{"status": "retiring"}).Error; err != nil {
			return fmt.Errorf("retire active apple certificate: %w", err)
		}

		now := time.Now().UTC()
		activated = appleSigningCertificateModel{
			CertificateID:          input.CertificateID,
			ApplePassTypeID:        passType.ApplePassTypeID,
			CertificateRef:         input.CertificateRef,
			PrivateKeyRef:          input.PrivateKeyRef,
			CertificateFingerprint: input.CertificateFingerprint,
			CertificateSerial:      input.CertificateSerial,
			ValidFrom:              input.ValidFrom,
			ExpiresAt:              input.ExpiresAt,
			Status:                 "active",
			ActivatedAt:            &now,
		}
		if err := tx.Create(&activated).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.ErrAppleWalletConfigurationConflict
			}
			return fmt.Errorf("create apple signing certificate: %w", err)
		}

		if err := tx.Model(&applePassTypeModel{}).
			Where("apple_pass_type_id = ?", passType.ApplePassTypeID).
			Updates(map[string]any{"status": "active", "updated_at": now}).Error; err != nil {
			return fmt.Errorf("activate apple pass type: %w", err)
		}
		if err := tx.Model(&appleWalletIssuerModel{}).
			Where("issuer_id = ?", passType.IssuerID).
			Updates(map[string]any{"status": "active", "updated_at": now}).Error; err != nil {
			return fmt.Errorf("activate apple wallet issuer: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.AppleSigningCertificate{}, err
	}
	return appleSigningCertificateFromModel(activated), nil
}

func (r AppleWalletRepository) UpsertPassDesign(ctx context.Context, input domain.UpsertApplePassDesignInput) (domain.ApplePassDesign, error) {
	var saved applePassDesignModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		passType, err := findPassTypeForMerchant(tx, input.MerchantID, input.PassTypeIdentifier)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		saved = applePassDesignModel{
			DesignID:            input.DesignID,
			ApplePassTypeID:     passType.ApplePassTypeID,
			ProgramName:         input.ProgramName,
			PassDescription:     input.PassDescription,
			StampGoal:           input.StampGoal,
			BackgroundColor:     input.BackgroundColor,
			ForegroundColor:     input.ForegroundColor,
			LabelColor:          input.LabelColor,
			AssetBundleRef:      input.AssetBundleRef,
			SupportText:         input.SupportText,
			TermsText:           input.TermsText,
			CampaignDisplayMode: input.CampaignDisplayMode,
			Status:              "active",
			Version:             1,
			UpdatedAt:           now,
		}
		updates := map[string]any{
			"program_name":          input.ProgramName,
			"pass_description":      input.PassDescription,
			"stamp_goal":            input.StampGoal,
			"background_color":      input.BackgroundColor,
			"foreground_color":      input.ForegroundColor,
			"label_color":           input.LabelColor,
			"asset_bundle_ref":      input.AssetBundleRef,
			"support_text":          input.SupportText,
			"terms_text":            input.TermsText,
			"campaign_display_mode": input.CampaignDisplayMode,
			"status":                "active",
			"version":               gorm.Expr("apple_pass_designs.version + 1"),
			"updated_at":            now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "apple_pass_type_id"}},
			DoUpdates: clause.Assignments(updates),
		}).Create(&saved).Error; err != nil {
			return fmt.Errorf("upsert apple pass design: %w", err)
		}
		if err := tx.Where("apple_pass_type_id = ?", passType.ApplePassTypeID).Take(&saved).Error; err != nil {
			return fmt.Errorf("load upserted apple pass design: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.ApplePassDesign{}, err
	}
	return applePassDesignFromModel(saved), nil
}

func (r AppleWalletRepository) GetPassDesign(ctx context.Context, merchantID string, passTypeIdentifier string) (domain.ApplePassDesign, error) {
	var model applePassDesignModel
	err := r.db.WithContext(ctx).Table("apple_pass_designs AS design").
		Select("design.*").
		Joins("INNER JOIN apple_pass_types AS apt ON apt.apple_pass_type_id = design.apple_pass_type_id").
		Joins("INNER JOIN apple_wallet_issuers AS awi ON awi.issuer_id = apt.issuer_id").
		Where("awi.merchant_id = ? AND apt.pass_type_identifier = ?", merchantID, passTypeIdentifier).
		Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ApplePassDesign{}, domain.ErrApplePassDesignNotFound
		}
		return domain.ApplePassDesign{}, fmt.Errorf("get apple pass design: %w", err)
	}
	return applePassDesignFromModel(model), nil
}

func (r AppleWalletRepository) ResolveActivePassDesign(ctx context.Context, applePassTypeID string) (domain.ApplePassDesign, error) {
	var model applePassDesignModel
	err := r.db.WithContext(ctx).
		Where("apple_pass_type_id = ? AND status = ?", applePassTypeID, "active").
		Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ApplePassDesign{}, domain.ErrApplePassDesignNotFound
		}
		return domain.ApplePassDesign{}, fmt.Errorf("resolve active apple pass design: %w", err)
	}
	return applePassDesignFromModel(model), nil
}

func findPassTypeForMerchant(db *gorm.DB, merchantID string, passTypeIdentifier string) (applePassTypeModel, error) {
	var passType applePassTypeModel
	err := db.Table("apple_pass_types AS apt").
		Select("apt.*").
		Joins("INNER JOIN apple_wallet_issuers AS awi ON awi.issuer_id = apt.issuer_id").
		Where("awi.merchant_id = ? AND apt.pass_type_identifier = ?", merchantID, passTypeIdentifier).
		Take(&passType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return applePassTypeModel{}, domain.ErrAppleWalletPassTypeNotFound
		}
		return applePassTypeModel{}, fmt.Errorf("find apple pass type: %w", err)
	}
	return passType, nil
}

func (r AppleWalletRepository) ConfigurationByMerchantID(ctx context.Context, merchantID string) (domain.AppleWalletConfiguration, error) {
	issuer, err := r.GetIssuerByMerchantID(ctx, merchantID)
	if err != nil {
		return domain.AppleWalletConfiguration{}, err
	}

	var passTypeModels []applePassTypeModel
	if err := r.db.WithContext(ctx).Where("issuer_id = ?", issuer.IssuerID).Order("created_at ASC").Find(&passTypeModels).Error; err != nil {
		return domain.AppleWalletConfiguration{}, fmt.Errorf("list apple pass types: %w", err)
	}
	passTypes := make([]domain.ApplePassType, 0, len(passTypeModels))
	passTypeIDs := make([]string, 0, len(passTypeModels))
	for _, model := range passTypeModels {
		passTypes = append(passTypes, applePassTypeFromModel(model))
		passTypeIDs = append(passTypeIDs, model.ApplePassTypeID)
	}

	certificates := make([]domain.AppleSigningCertificate, 0)
	designs := make([]domain.ApplePassDesign, 0)
	if len(passTypeIDs) > 0 {
		var certificateModels []appleSigningCertificateModel
		if err := r.db.WithContext(ctx).
			Where("apple_pass_type_id IN ?", passTypeIDs).
			Order("created_at DESC").
			Find(&certificateModels).Error; err != nil {
			return domain.AppleWalletConfiguration{}, fmt.Errorf("list apple signing certificates: %w", err)
		}
		for _, model := range certificateModels {
			certificates = append(certificates, appleSigningCertificateFromModel(model))
		}

		var designModels []applePassDesignModel
		if err := r.db.WithContext(ctx).
			Where("apple_pass_type_id IN ?", passTypeIDs).
			Order("created_at ASC").
			Find(&designModels).Error; err != nil {
			return domain.AppleWalletConfiguration{}, fmt.Errorf("list apple pass designs: %w", err)
		}
		for _, model := range designModels {
			designs = append(designs, applePassDesignFromModel(model))
		}
	}

	return domain.AppleWalletConfiguration{
		Issuer:       issuer,
		PassTypes:    passTypes,
		Certificates: certificates,
		Designs:      designs,
	}, nil
}

func (r AppleWalletRepository) ResolveActiveByMerchant(ctx context.Context, merchant string) (domain.ActiveAppleWalletCredential, error) {
	var row activeAppleWalletCredentialRow
	err := activeAppleWalletCredentialQuery(r.db.WithContext(ctx)).
		Where("m.merchant = ?", merchant).
		Take(&row).Error
	if err == nil {
		return activeAppleWalletCredentialFromRow(row), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ActiveAppleWalletCredential{}, fmt.Errorf("resolve apple wallet credential by merchant: %w", err)
	}

	var count int64
	if err := r.db.WithContext(ctx).Table("apple_wallet_issuers AS awi").
		Joins("INNER JOIN merchants AS m ON m.merchant_id = awi.merchant_id").
		Where("m.merchant = ?", merchant).
		Count(&count).Error; err != nil {
		return domain.ActiveAppleWalletCredential{}, fmt.Errorf("check apple wallet issuer: %w", err)
	}
	if count > 0 {
		return domain.ActiveAppleWalletCredential{}, domain.ErrAppleWalletNotProvisioned
	}
	return domain.ActiveAppleWalletCredential{}, domain.ErrAppleWalletIssuerNotFound
}

func (r AppleWalletRepository) ResolveActiveByPassType(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error) {
	var row activeAppleWalletCredentialRow
	err := activeAppleWalletCredentialQuery(r.db.WithContext(ctx)).
		Where("apt.pass_type_identifier = ?", passTypeIdentifier).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ActiveAppleWalletCredential{}, domain.ErrAppleWalletNotProvisioned
		}
		return domain.ActiveAppleWalletCredential{}, fmt.Errorf("resolve apple wallet credential by pass type: %w", err)
	}
	return activeAppleWalletCredentialFromRow(row), nil
}

func activeAppleWalletCredentialQuery(db *gorm.DB) *gorm.DB {
	return db.Table("apple_signing_certificates AS cert").
		Select(`
			awi.merchant_id,
			awi.issuer_id,
			apt.apple_pass_type_id,
			cert.certificate_id,
			awi.organization_name,
			awi.team_identifier,
			apt.pass_type_identifier,
			apt.pass_style,
			cert.certificate_ref,
			cert.private_key_ref,
			cert.certificate_fingerprint,
			cert.expires_at
		`).
		Joins("INNER JOIN apple_pass_types AS apt ON apt.apple_pass_type_id = cert.apple_pass_type_id").
		Joins("INNER JOIN apple_wallet_issuers AS awi ON awi.issuer_id = apt.issuer_id").
		Joins("INNER JOIN merchants AS m ON m.merchant_id = awi.merchant_id").
		Where("cert.status = ? AND apt.status = ? AND awi.status = ?", "active", "active", "active")
}

func appleWalletIssuerFromModel(model appleWalletIssuerModel) domain.AppleWalletIssuer {
	return domain.AppleWalletIssuer{
		IssuerID:         model.IssuerID,
		MerchantID:       model.MerchantID,
		OwnershipMode:    model.OwnershipMode,
		TeamIdentifier:   model.TeamIdentifier,
		OrganizationName: model.OrganizationName,
		Status:           model.Status,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func applePassTypeFromModel(model applePassTypeModel) domain.ApplePassType {
	return domain.ApplePassType{
		ApplePassTypeID:    model.ApplePassTypeID,
		IssuerID:           model.IssuerID,
		PassTypeIdentifier: model.PassTypeIdentifier,
		PassStyle:          model.PassStyle,
		Status:             model.Status,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func appleSigningCertificateFromModel(model appleSigningCertificateModel) domain.AppleSigningCertificate {
	return domain.AppleSigningCertificate{
		CertificateID:          model.CertificateID,
		ApplePassTypeID:        model.ApplePassTypeID,
		CertificateRef:         model.CertificateRef,
		PrivateKeyRef:          model.PrivateKeyRef,
		CertificateFingerprint: model.CertificateFingerprint,
		CertificateSerial:      model.CertificateSerial,
		ValidFrom:              model.ValidFrom,
		ExpiresAt:              model.ExpiresAt,
		Status:                 model.Status,
		CreatedAt:              model.CreatedAt,
		ActivatedAt:            model.ActivatedAt,
	}
}

func applePassDesignFromModel(model applePassDesignModel) domain.ApplePassDesign {
	return domain.ApplePassDesign{
		DesignID:            model.DesignID,
		ApplePassTypeID:     model.ApplePassTypeID,
		ProgramName:         model.ProgramName,
		PassDescription:     model.PassDescription,
		StampGoal:           model.StampGoal,
		BackgroundColor:     model.BackgroundColor,
		ForegroundColor:     model.ForegroundColor,
		LabelColor:          model.LabelColor,
		AssetBundleRef:      model.AssetBundleRef,
		SupportText:         model.SupportText,
		TermsText:           model.TermsText,
		CampaignDisplayMode: model.CampaignDisplayMode,
		Status:              model.Status,
		Version:             model.Version,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}

func activeAppleWalletCredentialFromRow(row activeAppleWalletCredentialRow) domain.ActiveAppleWalletCredential {
	return domain.ActiveAppleWalletCredential{
		MerchantID:             row.MerchantID,
		IssuerID:               row.IssuerID,
		ApplePassTypeID:        row.ApplePassTypeID,
		CertificateID:          row.CertificateID,
		OrganizationName:       row.OrganizationName,
		TeamIdentifier:         row.TeamIdentifier,
		PassTypeIdentifier:     row.PassTypeIdentifier,
		PassStyle:              row.PassStyle,
		CertificateRef:         row.CertificateRef,
		PrivateKeyRef:          row.PrivateKeyRef,
		CertificateFingerprint: row.CertificateFingerprint,
		ExpiresAt:              row.ExpiresAt,
	}
}

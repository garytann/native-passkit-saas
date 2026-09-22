package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"native-ios-passkit-wip/internal/domain"
)

type WalletRegistrationRepository struct {
	db *gorm.DB
}

type walletRegistrationModel struct {
	DeviceLibraryIdentifier string    `gorm:"column:device_library_identifier;primaryKey"`
	PassTypeIdentifier      string    `gorm:"column:pass_type_identifier;primaryKey"`
	SerialNumber            string    `gorm:"column:serial_number;primaryKey"`
	PushToken               string    `gorm:"column:push_token;not null;index"`
	AuthenticationToken     string    `gorm:"column:authentication_token;not null"`
	RegisteredAt            time.Time `gorm:"column:registered_at;default:now()"`
	UpdatedAt               time.Time `gorm:"column:updated_at;default:now()"`
}

func (walletRegistrationModel) TableName() string {
	return "wallet_registrations"
}

func NewWalletRegistrationRepository(db *gorm.DB) WalletRegistrationRepository {
	return WalletRegistrationRepository{db: db}
}

func (r WalletRegistrationRepository) Upsert(ctx context.Context, input domain.UpsertWalletRegistrationInput) (domain.WalletRegistration, error) {
	now := time.Now().UTC()
	registration := walletRegistrationModel{
		DeviceLibraryIdentifier: input.DeviceLibraryIdentifier,
		PassTypeIdentifier:      input.PassTypeIdentifier,
		SerialNumber:            input.SerialNumber,
		PushToken:               input.PushToken,
		AuthenticationToken:     input.AuthenticationToken,
		RegisteredAt:            now,
		UpdatedAt:               now,
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "device_library_identifier"},
				{Name: "pass_type_identifier"},
				{Name: "serial_number"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"push_token":           input.PushToken,
				"authentication_token": input.AuthenticationToken,
				"updated_at":           now,
			}),
		}).
		Create(&registration).Error; err != nil {
		return domain.WalletRegistration{}, fmt.Errorf("upsert wallet registration: %w", err)
	}

	return walletRegistrationFromModel(registration), nil
}

func (r WalletRegistrationRepository) Exists(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, serialNumber string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&walletRegistrationModel{}).
		Where(
			"device_library_identifier = ? AND pass_type_identifier = ? AND serial_number = ?",
			deviceLibraryIdentifier,
			passTypeIdentifier,
			serialNumber,
		).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check wallet registration exists: %w", err)
	}
	return count > 0, nil
}

func (r WalletRegistrationRepository) Delete(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, serialNumber string) error {
	if err := r.db.WithContext(ctx).
		Where(
			"device_library_identifier = ? AND pass_type_identifier = ? AND serial_number = ?",
			deviceLibraryIdentifier,
			passTypeIdentifier,
			serialNumber,
		).
		Delete(&walletRegistrationModel{}).Error; err != nil {
		return fmt.Errorf("delete wallet registration: %w", err)
	}

	return nil
}

func (r WalletRegistrationRepository) ListBySerialNumber(ctx context.Context, serialNumber string) ([]domain.WalletRegistration, error) {
	var rows []walletRegistrationModel
	if err := r.db.WithContext(ctx).
		Where("serial_number = ?", serialNumber).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list wallet registrations by serial number: %w", err)
	}

	registrations := make([]domain.WalletRegistration, 0, len(rows))
	for _, row := range rows {
		registrations = append(registrations, walletRegistrationFromModel(row))
	}
	return registrations, nil
}

func (r WalletRegistrationRepository) ListByPassTypeIdentifier(ctx context.Context, passTypeIdentifier string) ([]domain.WalletRegistration, error) {
	var rows []walletRegistrationModel
	if err := r.db.WithContext(ctx).
		Where("pass_type_identifier = ?", passTypeIdentifier).
		Order("registered_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list wallet registrations by pass type: %w", err)
	}

	registrations := make([]domain.WalletRegistration, 0, len(rows))
	for _, row := range rows {
		registrations = append(registrations, walletRegistrationFromModel(row))
	}
	return registrations, nil
}

func (r WalletRegistrationRepository) ListUpdatedSerialNumbers(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, since *time.Time) ([]string, time.Time, error) {
	type updatedPassRow struct {
		SerialNumber  string
		LastUpdatedAt time.Time
	}

	query := r.db.WithContext(ctx).
		Table("wallet_registrations AS wr").
		Select("wp.serial_number, wp.last_updated_at").
		Joins(`
			INNER JOIN wallet_passes AS wp
			ON wp.pass_type_identifier = wr.pass_type_identifier
			AND wp.serial_number = wr.serial_number
		`).
		Where("wr.device_library_identifier = ? AND wr.pass_type_identifier = ?", deviceLibraryIdentifier, passTypeIdentifier).
		Order("wp.last_updated_at DESC")

	if since != nil {
		query = query.Where("wp.last_updated_at > ?", *since)
	}

	var rows []updatedPassRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, time.Time{}, fmt.Errorf("list updated serial numbers: %w", err)
	}

	serials := make([]string, 0, len(rows))
	var lastUpdated time.Time
	for _, row := range rows {
		serials = append(serials, row.SerialNumber)
		if row.LastUpdatedAt.After(lastUpdated) {
			lastUpdated = row.LastUpdatedAt
		}
	}

	return serials, lastUpdated, nil
}

func walletRegistrationFromModel(registration walletRegistrationModel) domain.WalletRegistration {
	return domain.WalletRegistration{
		DeviceLibraryIdentifier: registration.DeviceLibraryIdentifier,
		PassTypeIdentifier:      registration.PassTypeIdentifier,
		SerialNumber:            registration.SerialNumber,
		PushToken:               registration.PushToken,
		AuthenticationToken:     registration.AuthenticationToken,
		RegisteredAt:            registration.RegisteredAt,
		UpdatedAt:               registration.UpdatedAt,
	}
}

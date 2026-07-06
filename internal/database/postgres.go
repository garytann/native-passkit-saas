package database

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenPostgres(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	if err := ensureDatabase(ctx, databaseURL); err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres pool: %w", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func ensureDatabase(ctx context.Context, databaseURL string) error {
	databaseName, maintenanceURL, err := maintenanceDatabaseURL(databaseURL)
	if err != nil {
		return err
	}

	db, err := gorm.Open(postgres.Open(maintenanceURL), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open maintenance postgres database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get maintenance postgres pool: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping maintenance postgres database: %w", err)
	}

	var exists bool
	if err := db.WithContext(ctx).
		Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", databaseName).
		Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database exists: %w", err)
	}

	if exists {
		return nil
	}

	if err := validateDatabaseName(databaseName); err != nil {
		return err
	}

	if err := db.WithContext(ctx).Exec("CREATE DATABASE " + quoteIdentifier(databaseName)).Error; err != nil {
		return fmt.Errorf("create database %q: %w", databaseName, err)
	}

	return nil
}

func maintenanceDatabaseURL(databaseURL string) (string, string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", "", fmt.Errorf("parse database url: %w", err)
	}

	databaseName := strings.TrimPrefix(parsed.Path, "/")
	if databaseName == "" {
		return "", "", fmt.Errorf("database name is required in DATABASE_URL")
	}

	maintenance := *parsed
	maintenance.Path = path.Join("/", "postgres")
	return databaseName, maintenance.String(), nil
}

func validateDatabaseName(databaseName string) error {
	ok, err := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, databaseName)
	if err != nil {
		return fmt.Errorf("validate database name: %w", err)
	}
	if !ok {
		return fmt.Errorf("database name %q must contain only letters, numbers, and underscores, and cannot start with a number", databaseName)
	}
	return nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

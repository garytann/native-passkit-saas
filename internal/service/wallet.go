package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/domain"
)

type WalletRegistrationRepository interface {
	Upsert(ctx context.Context, input domain.UpsertWalletRegistrationInput) (domain.WalletRegistration, error)
	Exists(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, serialNumber string) (bool, error)
	Delete(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, serialNumber string) error
	ListUpdatedSerialNumbers(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, since *time.Time) ([]string, time.Time, error)
}

type WalletService struct {
	repo    WalletRegistrationRepository
	timeout time.Duration
}

type RegisterWalletPassInput struct {
	DeviceLibraryIdentifier string
	PassTypeIdentifier      string
	SerialNumber            string
	PushToken               string
	AuthenticationToken     string
}

type UpdatedSerialNumbersResult struct {
	SerialNumbers []string `json:"serialNumbers"`
	LastUpdated   string   `json:"lastUpdated"`
}

func NewWalletService(repo WalletRegistrationRepository) WalletService {
	return WalletService{
		repo:    repo,
		timeout: 5 * time.Second,
	}
}

func (s WalletService) RegisterPass(ctx context.Context, input RegisterWalletPassInput) (bool, error) {
	if s.repo == nil {
		return false, ErrRepositoryUnavailable
	}
	input.DeviceLibraryIdentifier = strings.TrimSpace(input.DeviceLibraryIdentifier)
	input.PassTypeIdentifier = strings.TrimSpace(input.PassTypeIdentifier)
	input.SerialNumber = strings.TrimSpace(input.SerialNumber)
	input.PushToken = strings.TrimSpace(input.PushToken)
	input.AuthenticationToken = strings.TrimSpace(input.AuthenticationToken)
	if input.DeviceLibraryIdentifier == "" || input.PassTypeIdentifier == "" || input.SerialNumber == "" || input.PushToken == "" || input.AuthenticationToken == "" {
		return false, ValidationError{Message: "device, pass type, serial number, push token, and authentication token are required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	exists, err := s.repo.Exists(ctx, input.DeviceLibraryIdentifier, input.PassTypeIdentifier, input.SerialNumber)
	if err != nil {
		return false, err
	}

	if _, err := s.repo.Upsert(ctx, domain.UpsertWalletRegistrationInput{
		DeviceLibraryIdentifier: input.DeviceLibraryIdentifier,
		PassTypeIdentifier:      input.PassTypeIdentifier,
		SerialNumber:            input.SerialNumber,
		PushToken:               input.PushToken,
		AuthenticationToken:     input.AuthenticationToken,
	}); err != nil {
		return false, err
	}

	return !exists, nil
}

func (s WalletService) UnregisterPass(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, serialNumber string) error {
	if s.repo == nil {
		return ErrRepositoryUnavailable
	}
	deviceLibraryIdentifier = strings.TrimSpace(deviceLibraryIdentifier)
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	serialNumber = strings.TrimSpace(serialNumber)
	if deviceLibraryIdentifier == "" || passTypeIdentifier == "" || serialNumber == "" {
		return ValidationError{Message: "device, pass type, and serial number are required"}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	return s.repo.Delete(ctx, deviceLibraryIdentifier, passTypeIdentifier, serialNumber)
}

func (s WalletService) UpdatedSerialNumbers(ctx context.Context, deviceLibraryIdentifier string, passTypeIdentifier string, previousLastUpdated string) (UpdatedSerialNumbersResult, error) {
	if s.repo == nil {
		return UpdatedSerialNumbersResult{}, ErrRepositoryUnavailable
	}
	deviceLibraryIdentifier = strings.TrimSpace(deviceLibraryIdentifier)
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	if deviceLibraryIdentifier == "" || passTypeIdentifier == "" {
		return UpdatedSerialNumbersResult{}, ValidationError{Message: "device and pass type are required"}
	}

	since, err := parseLastUpdatedTag(previousLastUpdated)
	if err != nil {
		return UpdatedSerialNumbersResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	serials, lastUpdated, err := s.repo.ListUpdatedSerialNumbers(ctx, deviceLibraryIdentifier, passTypeIdentifier, since)
	if err != nil {
		return UpdatedSerialNumbersResult{}, err
	}
	if len(serials) == 0 {
		return UpdatedSerialNumbersResult{}, nil
	}

	return UpdatedSerialNumbersResult{
		SerialNumbers: serials,
		LastUpdated:   strconv.FormatInt(lastUpdated.Unix(), 10),
	}, nil
}

func parseLastUpdatedTag(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, ValidationError{Message: fmt.Sprintf("invalid passesUpdatedSince value %q", value)}
	}
	since := time.Unix(seconds, 0).UTC()
	return &since, nil
}

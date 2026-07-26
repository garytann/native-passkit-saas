package domain

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var ErrWalletPassNotFound = errors.New("wallet pass not found")

type WalletPass struct {
	PassTypeIdentifier  string    `json:"pass_type_identifier"`
	SerialNumber        string    `json:"serial_number"`
	AuthenticationToken string    `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
	LastUpdatedAt       time.Time `json:"last_updated_at"`
}

type CreateWalletPassInput struct {
	PassTypeIdentifier  string
	SerialNumber        string
	AuthenticationToken string
}

type WalletRegistration struct {
	DeviceLibraryIdentifier string    `json:"device_library_identifier"`
	PassTypeIdentifier      string    `json:"pass_type_identifier"`
	SerialNumber            string    `json:"serial_number"`
	PushToken               string    `json:"push_token"`
	AuthenticationToken     string    `json:"-"`
	RegisteredAt            time.Time `json:"registered_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type UpsertWalletRegistrationInput struct {
	DeviceLibraryIdentifier string
	PassTypeIdentifier      string
	SerialNumber            string
	PushToken               string
	AuthenticationToken     string
}

func NewWalletAuthenticationToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate wallet authentication token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

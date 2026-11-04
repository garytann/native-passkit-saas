package domain

import (
	"errors"
	"time"
)

var ErrMerchantEmailNotVerified = errors.New("merchant email is not verified")
var ErrEmailVerificationNotFound = errors.New("email verification not found")
var ErrEmailVerificationInvalid = errors.New("invalid or expired verification code")
var ErrEmailVerificationAttemptsExhausted = errors.New("verification attempts exhausted")
var ErrEmailVerificationRateLimited = errors.New("verification resend rate limited")

type MerchantEmailVerification struct {
	VerificationID    string
	MerchantID        string
	RecipientEmail    string
	ExpiresAt         time.Time
	Attempts          int
	MaxAttempts       int
	ResendAvailableAt time.Time
	ConsumedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type EmailJob struct {
	EmailJobID     string
	JobType        string
	MerchantID     string
	VerificationID string
	RecipientEmail string
	RecipientName  string
	Status         string
	Attempts       int
	MaxAttempts    int
	NextAttemptAt  time.Time
	LastError      string
	MessageID      string
	IdempotencyKey string
	SentAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PendingMerchantRegistration struct {
	Merchant     CreateMerchantInput
	Issuer       CreateAppleWalletIssuerInput
	Verification MerchantEmailVerification
	EmailJob     EmailJob
}

type VerifyMerchantEmailInput struct {
	VerificationID string
	CodeValid      bool
	WelcomeJob     EmailJob
}

type EmailVerificationResult struct {
	Merchant        Merchant
	Verification    MerchantEmailVerification
	AlreadyVerified bool
}

func NewEmailVerificationID() (string, error) {
	return newReadableID("EVF-")
}

func NewEmailJobID() (string, error) {
	return newReadableID("EMJ-")
}

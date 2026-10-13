package domain

import (
	"errors"
	"time"
)

var ErrLoyaltyProgramNotFound = errors.New("loyalty program not found")
var ErrGoogleWalletClassNotFound = errors.New("google wallet class not found")
var ErrGoogleWalletObjectNotFound = errors.New("google wallet object not found")
var ErrGoogleWalletNotProvisioned = errors.New("google wallet is not provisioned for merchant")
var ErrGoogleWalletDisabled = errors.New("google wallet is disabled")

type LoyaltyProgram struct {
	ProgramID           string          `json:"program_id"`
	MerchantID          string          `json:"merchant_id"`
	ProgramName         string          `json:"program_name"`
	PassDescription     string          `json:"pass_description"`
	StampGoal           int             `json:"stamp_goal"`
	BackgroundColor     string          `json:"background_color"`
	AssetBundleRef      string          `json:"asset_bundle_ref"`
	StampInstructions   string          `json:"stamp_instructions"`
	RewardTitle         string          `json:"reward_title"`
	RewardDetails       string          `json:"reward_details"`
	SupportText         string          `json:"support_text"`
	TermsText           string          `json:"terms_text"`
	CampaignDisplayMode string          `json:"campaign_display_mode"`
	StampIconKey        string          `json:"stamp_icon_key"`
	Status              string          `json:"status"`
	Version             int             `json:"version"`
	Links               []ApplePassLink `json:"links"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type GoogleWalletClass struct {
	GoogleClassID        string     `json:"google_class_id"`
	ProgramID            string     `json:"program_id"`
	IssuerID             string     `json:"issuer_id"`
	ClassSuffix          string     `json:"class_suffix"`
	ReviewStatus         string     `json:"review_status"`
	Status               string     `json:"status"`
	ProgramLogoRef       string     `json:"program_logo_ref"`
	WideLogoRef          string     `json:"wide_logo_ref"`
	SyncedProgramVersion int        `json:"synced_program_version"`
	LastSyncedAt         *time.Time `json:"last_synced_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type GoogleWalletObject struct {
	GoogleObjectID         string     `json:"google_object_id"`
	MemberID               string     `json:"member_id"`
	GoogleClassID          string     `json:"google_class_id"`
	ImageToken             string     `json:"-"`
	Status                 string     `json:"status"`
	SyncedStampProgress    int        `json:"synced_stamp_progress"`
	SyncedAvailableRewards int        `json:"synced_available_rewards"`
	SyncedProgramVersion   int        `json:"synced_program_version"`
	LastSyncedAt           *time.Time `json:"last_synced_at,omitempty"`
	LastError              string     `json:"last_error,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type GoogleWalletConfiguration struct {
	Enabled             bool               `json:"enabled"`
	ProgramLogoUploaded bool               `json:"program_logo_uploaded"`
	WideLogoUploaded    bool               `json:"wide_logo_uploaded"`
	Class               *GoogleWalletClass `json:"class,omitempty"`
	Program             *LoyaltyProgram    `json:"program,omitempty"`
}

type UpsertGoogleWalletClassInput struct {
	GoogleClassID  string
	ProgramID      string
	IssuerID       string
	ClassSuffix    string
	ReviewStatus   string
	Status         string
	ProgramLogoRef string
	WideLogoRef    string
}

type UpsertGoogleWalletObjectInput struct {
	GoogleObjectID string
	MemberID       string
	GoogleClassID  string
	ImageToken     string
}

type WalletSyncJob struct {
	SyncJobID      string     `json:"sync_job_id"`
	Platform       string     `json:"platform"`
	TargetType     string     `json:"target_type"`
	TargetID       string     `json:"target_id"`
	DesiredVersion string     `json:"desired_version"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"max_attempts"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type EnqueueWalletSyncJobInput struct {
	SyncJobID      string
	Platform       string
	TargetType     string
	TargetID       string
	DesiredVersion string
}

type UpdateLoyaltyProgramInput struct {
	MerchantID          string
	ProgramName         string
	PassDescription     string
	StampGoal           int
	BackgroundColor     string
	StampInstructions   string
	RewardTitle         string
	RewardDetails       string
	SupportText         string
	TermsText           string
	CampaignDisplayMode string
	StampIconKey        string
	Links               []UpsertApplePassLinkInput
}

func NewLoyaltyProgramID() (string, error) { return newReadableID("LPR-") }

func NewWalletSyncJobID() (string, error) { return newReadableID("WSJ-") }

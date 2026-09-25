package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const readableAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

var ErrStoreMemberNotFound = errors.New("store member not found")
var ErrStoreMemberForbidden = errors.New("store member does not belong to merchant")
var ErrMemberInactive = errors.New("member is inactive")
var ErrRewardNotReady = errors.New("member has not completed the stamp goal")
var ErrNoAvailableRewards = errors.New("member has no available rewards")
var ErrIdempotencyKeyConflict = errors.New("idempotency key was already used for another operation")

type StoreMember struct {
	MemberID         string    `json:"member_id"`
	CreationDate     time.Time `json:"creation_date"`
	CustomerName     string    `json:"customer_name"`
	CustomerEmail    string    `json:"customer_email"`
	Merchant         string    `json:"merchant"`
	StampProgress    int       `json:"stamp_progress"`
	AvailableRewards int       `json:"available_rewards"`
	Status           string    `json:"status"`
}

type CreateStoreMemberInput struct {
	MemberID      string
	CustomerName  string
	CustomerEmail string
	Merchant      string
}

type StampUpdate struct {
	Member                StoreMember `json:"member"`
	StampDelta            int         `json:"stamp_delta"`
	PreviousStampProgress int         `json:"previous_stamp_progress"`
	NewStampProgress      int         `json:"new_stamp_progress"`
}

type MemberRewardTransaction struct {
	TransactionID         string    `json:"transaction_id"`
	MemberID              string    `json:"member_id"`
	MerchantID            string    `json:"merchant_id"`
	TransactionType       string    `json:"type"`
	Quantity              int       `json:"quantity"`
	PreviousRewardBalance int       `json:"previous_reward_balance"`
	NewRewardBalance      int       `json:"new_reward_balance"`
	PreviousStampProgress int       `json:"previous_stamp_progress"`
	NewStampProgress      int       `json:"new_stamp_progress"`
	CreationDate          time.Time `json:"creation_date"`
	IdempotencyKey        string    `json:"-"`
}

type RewardMutation struct {
	Member      StoreMember             `json:"member"`
	Transaction MemberRewardTransaction `json:"transaction"`
	Replayed    bool                    `json:"replayed"`
}

type RewardMutationInput struct {
	TransactionID   string
	MemberID        string
	MerchantID      string
	Merchant        string
	TransactionType string
	StampGoal       int
	IdempotencyKey  string
}

func NewReadableMemberID() (string, error) {
	return newReadableID("MBR-")
}

func NewReadableMerchantID() (string, error) {
	return newReadableID("MCH-")
}

func NewReadableRewardTransactionID() (string, error) {
	return newReadableID("RWT-")
}

func newReadableID(prefix string) (string, error) {
	const length = 8
	var builder strings.Builder
	builder.Grow(len(prefix) + length)
	builder.WriteString(prefix)

	max := big.NewInt(int64(len(readableAlphabet)))
	for i := 0; i < length; i++ {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate readable id: %w", err)
		}
		builder.WriteByte(readableAlphabet[value.Int64()])
	}

	return builder.String(), nil
}

func StableToken(value string, length int) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	token := hex.EncodeToString(sum[:])
	if length <= 0 || length > len(token) {
		return token
	}
	return token[:length]
}

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

type StoreMember struct {
	MemberID      string    `json:"member_id"`
	CreationDate  time.Time `json:"creation_date"`
	CustomerName  string    `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	Merchant      string    `json:"merchant"`
	StampProgress int       `json:"stamp_progress"`
	Status        string    `json:"status"`
}

type CreateStoreMemberInput struct {
	MemberID      string
	CustomerName  string
	CustomerEmail string
	Merchant      string
}

func NewReadableMemberID() (string, error) {
	const length = 8
	var builder strings.Builder
	builder.Grow(len("MBR-") + length)
	builder.WriteString("MBR-")

	max := big.NewInt(int64(len(readableAlphabet)))
	for i := 0; i < length; i++ {
		value, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate member id: %w", err)
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

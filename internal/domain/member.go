package domain

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const readableAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

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

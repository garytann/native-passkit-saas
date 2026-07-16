package domain

import (
	"errors"
	"time"
)

var ErrMerchantAlreadyExists = errors.New("merchant already exists")

type Merchant struct {
	MerchantID    string    `json:"merchant_id"`
	CreationDate  time.Time `json:"creation_date"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Merchant      string    `json:"merchant"`
	MerchantEmail string    `json:"merchant_email"`
	Status        string    `json:"status"`
	PaymentStatus string    `json:"payment_status"`
}

type CreateMerchantInput struct {
	MerchantID    string
	FirstName     string
	LastName      string
	Company       string
	MerchantEmail string
	PasswordHash  string
}

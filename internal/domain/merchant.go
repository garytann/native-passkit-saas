package domain

import (
	"time"
)

type Merchant struct {
	MerchantID    string    `json:"merchant_id"`
	CreationDate  time.Time `json:"creation_date"`
	Merchant      string    `json:"merchant"`
	MerchantEmail string    `json:"merchant_email"`
	Status        string    `json:"status"`
}

type CreateMerchantInput struct {
	MerchantID    string
	Merchant      string
	MerchantEmail string
}

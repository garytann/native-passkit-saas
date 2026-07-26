package domain

import "time"

type Campaign struct {
	CampaignID   string     `json:"campaign_id"`
	CreationDate time.Time  `json:"creation_date"`
	Merchant     string     `json:"merchant"`
	Title        string     `json:"title"`
	Message      string     `json:"message"`
	Status       string     `json:"status"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
}

type CreateCampaignInput struct {
	CampaignID string
	Merchant   string
	Title      string
	Message    string
	Status     string
	StartDate  *time.Time
	EndDate    *time.Time
}

type CampaignDelivery struct {
	DeliveryID   string     `json:"delivery_id"`
	CreationDate time.Time  `json:"creation_date"`
	CampaignID   string     `json:"campaign_id"`
	MemberID     string     `json:"member_id"`
	Status       string     `json:"status"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	ViewedAt     *time.Time `json:"viewed_at,omitempty"`
	RedeemedAt   *time.Time `json:"redeemed_at,omitempty"`
	LastError    string     `json:"last_error"`
}

type CreateCampaignDeliveryInput struct {
	DeliveryID string
	CampaignID string
	MemberID   string
	Status     string
}

type UpdateCampaignDeliveryStatusInput struct {
	DeliveryID  string
	Status      string
	DeliveredAt *time.Time
	LastError   string
}

func NewReadableCampaignID() (string, error) {
	return newReadableID("CMP-")
}

func NewReadableDeliveryID() (string, error) {
	return newReadableID("DLV-")
}

package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MerchantTokenClaims struct {
	MerchantID string `json:"merchant_id"`
	Merchant   string `json:"merchant"`
	ExpiresAt  int64  `json:"exp"`
}

type AdminTokenClaims struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"exp"`
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string) TokenManager {
	return TokenManager{
		secret: []byte(secret),
		ttl:    24 * time.Hour,
	}
}

func (m TokenManager) IssueMerchantToken(merchantID string, merchant string) (string, error) {
	claims := MerchantTokenClaims{
		MerchantID: merchantID,
		Merchant:   merchant,
		ExpiresAt:  time.Now().Add(m.ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal token claims: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (m TokenManager) IssueAdminToken(email string) (string, error) {
	claims := AdminTokenClaims{
		Email:     strings.TrimSpace(strings.ToLower(email)),
		Role:      "platform_admin",
		ExpiresAt: time.Now().Add(8 * time.Hour).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal admin token claims: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signedPayload := "admin." + encodedPayload
	return signedPayload + "." + m.sign(signedPayload), nil
}

func (m TokenManager) VerifyAdminToken(token string) (AdminTokenClaims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 || parts[0] != "admin" || parts[1] == "" || parts[2] == "" {
		return AdminTokenClaims{}, ErrUnauthorized
	}
	signedPayload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(m.sign(signedPayload))) {
		return AdminTokenClaims{}, ErrUnauthorized
	}

	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AdminTokenClaims{}, ErrUnauthorized
	}
	var claims AdminTokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return AdminTokenClaims{}, ErrUnauthorized
	}
	if claims.Email == "" || claims.Role != "platform_admin" || time.Now().Unix() > claims.ExpiresAt {
		return AdminTokenClaims{}, ErrUnauthorized
	}
	return claims, nil
}

func (m TokenManager) VerifyMerchantToken(token string) (MerchantTokenClaims, error) {
	payload, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || payload == "" || signature == "" {
		return MerchantTokenClaims{}, ErrUnauthorized
	}
	if !hmac.Equal([]byte(signature), []byte(m.sign(payload))) {
		return MerchantTokenClaims{}, ErrUnauthorized
	}

	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return MerchantTokenClaims{}, ErrUnauthorized
	}

	var claims MerchantTokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return MerchantTokenClaims{}, ErrUnauthorized
	}
	if claims.MerchantID == "" || claims.Merchant == "" || time.Now().Unix() > claims.ExpiresAt {
		return MerchantTokenClaims{}, ErrUnauthorized
	}

	return claims, nil
}

func (m TokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func BearerToken(authorizationHeader string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authorizationHeader, prefix) {
		return "", ErrUnauthorized
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, prefix))
	if token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

var ErrUnauthorized = errors.New("unauthorized")

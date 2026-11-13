package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"native-ios-passkit-wip/internal/config"
)

type GmailAPIEmailSender struct {
	api *gmail.Service
}

func NewGmailAPIEmailSender(ctx context.Context, cfg config.Config) (EmailSender, error) {
	if !cfg.EmailEnabled {
		return nil, nil
	}

	user := strings.TrimSpace(cfg.GmailAPIUser)
	credentialsPath := strings.TrimSpace(cfg.GmailCredentialsPath)
	if user == "" || credentialsPath == "" {
		return nil, errors.New("GMAIL_API_USER and GMAIL_API_CREDENTIALS_FILE are required when email delivery is enabled")
	}
	if !validEmailAddress(user) {
		return nil, errors.New("GMAIL_API_USER must be a valid email address")
	}

	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read Gmail API credentials: %w", err)
	}
	jwtConfig, err := google.JWTConfigFromJSON(credentials, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("parse Gmail API credentials: %w", err)
	}
	if strings.TrimSpace(jwtConfig.Email) == "" || len(jwtConfig.PrivateKey) == 0 {
		return nil, errors.New("Gmail API credentials are missing client_email or private_key")
	}
	if err := validateServiceAccountPrivateKey(jwtConfig.PrivateKey); err != nil {
		return nil, fmt.Errorf("validate Gmail API private key: %w", err)
	}

	jwtConfig.Subject = user
	api, err := gmail.NewService(ctx, option.WithHTTPClient(jwtConfig.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("initialize Gmail API: %w", err)
	}
	return GmailAPIEmailSender{api: api}, nil
}

func (s GmailAPIEmailSender) Send(ctx context.Context, message EmailMessage) error {
	if s.api == nil {
		return errors.New("Gmail API client is not initialized")
	}
	payload, err := buildMIMEMessage(message)
	if err != nil {
		return err
	}
	raw := base64.RawURLEncoding.EncodeToString(payload)
	if _, err := s.api.Users.Messages.Send("me", &gmail.Message{Raw: raw}).Context(ctx).Do(); err != nil {
		return fmt.Errorf("send email through Gmail API: %w", err)
	}
	return nil
}

func validateServiceAccountPrivateKey(privateKey []byte) error {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return errors.New("private_key is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	if _, ok := key.(*rsa.PrivateKey); !ok {
		return errors.New("private_key is not an RSA key")
	}
	return nil
}

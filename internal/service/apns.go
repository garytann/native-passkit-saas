package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"native-ios-passkit-wip/internal/config"
)

type PassUpdateSender interface {
	SendPassUpdate(ctx context.Context, pushToken string) error
}

type APNSPassUpdateSender struct {
	PassTypeIdentifier string
	Endpoint           string
	CertPath           string
	KeyPath            string

	once      sync.Once
	client    *http.Client
	clientErr error
}

func NewAPNSPassUpdateSender(cfg config.Config) *APNSPassUpdateSender {
	return &APNSPassUpdateSender{
		PassTypeIdentifier: cfg.PassTypeID,
		Endpoint:           cfg.APNSEndpoint,
		CertPath:           cfg.PassCertPath,
		KeyPath:            cfg.PassKeyPath,
	}
}

func (s *APNSPassUpdateSender) SendPassUpdate(ctx context.Context, pushToken string) error {
	pushToken = strings.TrimSpace(pushToken)
	if pushToken == "" {
		return ValidationError{Message: "push token is required"}
	}

	client, err := s.httpClient()
	if err != nil {
		return err
	}

	endpoint := strings.TrimRight(s.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://api.push.apple.com"
	}
	requestURL := endpoint + "/3/device/" + url.PathEscape(pushToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf("create APNs request: %w", err)
	}
	req.Header.Set("apns-topic", strings.TrimSpace(s.PassTypeIdentifier))
	req.Header.Set("apns-push-type", "background")
	req.Header.Set("apns-priority", "5")
	req.Header.Set("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send APNs pass update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	reason := strings.TrimSpace(string(body))
	if reason == "" {
		reason = resp.Status
	}
	return fmt.Errorf("APNs pass update failed: status=%d reason=%s", resp.StatusCode, reason)
}

func (s *APNSPassUpdateSender) httpClient() (*http.Client, error) {
	s.once.Do(func() {
		cert, err := tls.LoadX509KeyPair(s.CertPath, s.KeyPath)
		if err != nil {
			s.clientErr = fmt.Errorf("load APNs pass certificate: %w", err)
			return
		}

		s.client = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				ForceAttemptHTTP2: true,
				TLSClientConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				},
			},
		}
	})
	if s.clientErr != nil {
		return nil, s.clientErr
	}
	return s.client, nil
}

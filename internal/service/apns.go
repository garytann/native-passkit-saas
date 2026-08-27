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
	"native-ios-passkit-wip/internal/domain"
)

type PassUpdateSender interface {
	SendPassUpdate(ctx context.Context, passTypeIdentifier string, pushToken string) error
}

type APNSPassUpdateSender struct {
	cfg         config.Config
	Endpoint    string
	credentials AppleWalletCredentialResolver
	mu          sync.Mutex
	clients     map[string]*http.Client
}

func NewAPNSPassUpdateSender(cfg config.Config, credentialResolvers ...AppleWalletCredentialResolver) *APNSPassUpdateSender {
	sender := &APNSPassUpdateSender{
		cfg:      cfg,
		Endpoint: cfg.APNSEndpoint,
		clients:  make(map[string]*http.Client),
	}
	if len(credentialResolvers) > 0 {
		sender.credentials = credentialResolvers[0]
	}
	return sender
}

func (s *APNSPassUpdateSender) SendPassUpdate(ctx context.Context, passTypeIdentifier string, pushToken string) error {
	passTypeIdentifier = strings.TrimSpace(passTypeIdentifier)
	pushToken = strings.TrimSpace(pushToken)
	if passTypeIdentifier == "" {
		return ValidationError{Message: "pass type identifier is required"}
	}
	if pushToken == "" {
		return ValidationError{Message: "push token is required"}
	}

	credential, err := s.credential(ctx, passTypeIdentifier)
	if err != nil {
		return err
	}
	client, err := s.httpClient(credential)
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
	req.Header.Set("apns-topic", passTypeIdentifier)
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

func (s *APNSPassUpdateSender) credential(ctx context.Context, passTypeIdentifier string) (domain.ActiveAppleWalletCredential, error) {
	if s.credentials != nil {
		credential, err := s.credentials.ResolveActiveByPassType(ctx, passTypeIdentifier)
		if err == nil {
			return credential, nil
		}
		if passTypeIdentifier != strings.TrimSpace(s.cfg.PassTypeID) {
			return domain.ActiveAppleWalletCredential{}, err
		}
	}
	if passTypeIdentifier != strings.TrimSpace(s.cfg.PassTypeID) {
		return domain.ActiveAppleWalletCredential{}, domain.ErrAppleWalletNotProvisioned
	}
	return domain.ActiveAppleWalletCredential{
		PassTypeIdentifier: passTypeIdentifier,
		CertificateRef:     s.cfg.PassCertPath,
		PrivateKeyRef:      s.cfg.PassKeyPath,
	}, nil
}

func (s *APNSPassUpdateSender) httpClient(credential domain.ActiveAppleWalletCredential) (*http.Client, error) {
	cacheKey := credential.CertificateID
	if cacheKey == "" {
		cacheKey = credential.CertificateRef + "\x00" + credential.PrivateKeyRef
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if client, ok := s.clients[cacheKey]; ok {
		return client, nil
	}

	cert, err := tls.LoadX509KeyPair(credential.CertificateRef, credential.PrivateKeyRef)
	if err != nil {
		return nil, fmt.Errorf("load APNs pass certificate: %w", err)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		},
	}
	s.clients[cacheKey] = client
	return client, nil
}

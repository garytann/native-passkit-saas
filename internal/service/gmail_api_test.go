package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"native-ios-passkit-wip/internal/config"
)

func TestGmailAPIEmailSenderSendsBase64URLMIMEMessage(t *testing.T) {
	var request gmail.Message
	client := &http.Client{Transport: roundTripFunc(func(incoming *http.Request) (*http.Response, error) {
		if incoming.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", incoming.Method)
		}
		if !strings.HasSuffix(incoming.URL.Path, "/users/me/messages/send") {
			t.Errorf("path = %s, want Gmail messages.send endpoint", incoming.URL.Path)
		}
		if err := json.NewDecoder(incoming.Body).Decode(&request); err != nil {
			t.Errorf("decode Gmail request: %v", err)
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"gmail-message-id"}`), nil
	})}

	api, err := gmail.NewService(context.Background(), option.WithEndpoint("https://gmail.test/"), option.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	sender := GmailAPIEmailSender{api: api}
	err = sender.Send(context.Background(), EmailMessage{
		FromName:  "Just Loyalty",
		FromEmail: "hello@justloyalty.com",
		ReplyTo:   "hello@justloyalty.com",
		To:        "owner@example.com",
		Subject:   "Verify your account",
		TextBody:  "Verification code: 123456",
		HTMLBody:  "<p>Verification code: <strong>123456</strong></p>",
		MessageID: "<emj-test@justloyalty.com>",
	})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := base64.RawURLEncoding.DecodeString(request.Raw)
	if err != nil {
		t.Fatalf("decode raw Gmail message: %v", err)
	}
	message := string(payload)
	for _, expected := range []string{
		"From: Just Loyalty <hello@justloyalty.com>",
		"To: owner@example.com",
		"Reply-To: hello@justloyalty.com",
		"Message-ID: <emj-test@justloyalty.com>",
		"Content-Type: multipart/alternative",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("Gmail MIME message does not contain %q:\n%s", expected, message)
		}
	}
}

func TestGmailAPIEmailSenderReturnsAPIError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusForbidden, `{"error":{"code":403,"message":"delegation denied","status":"PERMISSION_DENIED"}}`), nil
	})}

	api, err := gmail.NewService(context.Background(), option.WithEndpoint("https://gmail.test/"), option.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	err = (GmailAPIEmailSender{api: api}).Send(context.Background(), EmailMessage{
		FromName: "Just Loyalty", FromEmail: "hello@justloyalty.com", To: "owner@example.com",
		Subject: "Test", TextBody: "Text", HTMLBody: "<p>HTML</p>", MessageID: "<test@justloyalty.com>",
	})
	if err == nil || !strings.Contains(err.Error(), "delegation denied") {
		t.Fatalf("Send() error = %v, want delegated API error", err)
	}
}

func TestNewGmailAPIEmailSenderValidatesConfiguration(t *testing.T) {
	base := config.Config{EmailEnabled: true, GmailAPIUser: "gary@justloyalty.com"}
	if _, err := NewGmailAPIEmailSender(context.Background(), base); err == nil || !strings.Contains(err.Error(), "GMAIL_API_CREDENTIALS_FILE") {
		t.Fatalf("missing credentials error = %v", err)
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"type":"service_account"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	base.GmailCredentialsPath = invalidPath
	if _, err := NewGmailAPIEmailSender(context.Background(), base); err == nil {
		t.Fatal("expected incomplete service-account credentials to be rejected")
	}

	validPath := filepath.Join(t.TempDir(), "service-account.json")
	if err := os.WriteFile(validPath, testGmailServiceAccountJSON(t), 0o600); err != nil {
		t.Fatal(err)
	}
	base.GmailCredentialsPath = validPath
	sender, err := NewGmailAPIEmailSender(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if sender == nil {
		t.Fatal("expected enabled Gmail API sender")
	}
}

func TestNewGmailAPIEmailSenderDisabled(t *testing.T) {
	sender, err := NewGmailAPIEmailSender(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sender != nil {
		t.Fatal("disabled email delivery should not initialize a sender")
	}
}

func testGmailServiceAccountJSON(t *testing.T) []byte {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	credentials, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "email-worker@example-project.iam.gserviceaccount.com",
		"private_key":  string(encodedKey),
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatal(err)
	}
	return credentials
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

type emailRepositoryStub struct {
	verificationResult domain.EmailVerificationResult
	verificationErr    error
	appliedInput       domain.VerifyMerchantEmailInput
	appliedResult      domain.EmailVerificationResult
	merchant           domain.Merchant
	jobs               []domain.EmailJob
	completedJobID     string
	retriedJobID       string
}

func (r *emailRepositoryStub) CreatePendingRegistration(context.Context, domain.PendingMerchantRegistration) (domain.EmailVerificationResult, error) {
	return r.verificationResult, r.verificationErr
}
func (r *emailRepositoryStub) VerificationByEmail(context.Context, string) (domain.EmailVerificationResult, error) {
	return r.verificationResult, r.verificationErr
}
func (r *emailRepositoryStub) ApplyVerificationAttempt(_ context.Context, input domain.VerifyMerchantEmailInput) (domain.EmailVerificationResult, error) {
	r.appliedInput = input
	return r.appliedResult, r.verificationErr
}
func (*emailRepositoryStub) ResendVerification(context.Context, domain.MerchantEmailVerification, domain.EmailJob) error {
	return nil
}
func (r *emailRepositoryStub) MerchantByID(context.Context, string) (domain.Merchant, error) {
	return r.merchant, nil
}
func (r *emailRepositoryStub) ClaimPendingEmailJobs(context.Context, int) ([]domain.EmailJob, error) {
	jobs := r.jobs
	r.jobs = nil
	return jobs, nil
}
func (r *emailRepositoryStub) CompleteEmailJob(_ context.Context, id string) error {
	r.completedJobID = id
	return nil
}
func (r *emailRepositoryStub) RetryEmailJob(_ context.Context, job domain.EmailJob, _ string) error {
	r.retriedJobID = job.EmailJobID
	return nil
}
func (*emailRepositoryStub) FailEmailJob(context.Context, string, string) error { return nil }

type emailSenderStub struct {
	message EmailMessage
	err     error
}

func (s *emailSenderStub) Send(_ context.Context, message EmailMessage) error {
	s.message = message
	return s.err
}

func emailTestConfig() config.Config {
	return config.Config{
		EmailEnabled:            true,
		EmailVerificationSecret: strings.Repeat("s", 32),
		EmailFromName:           "Just Loyalty",
		EmailFromAddress:        "hello@justloyalty.com",
		EmailReplyTo:            "hello@justloyalty.com",
		PublicBaseURL:           "https://app.just-loyalty.com",
	}
}

func TestVerificationPINIsDeterministicAndSixDigits(t *testing.T) {
	service, err := NewEmailService(emailTestConfig(), &emailRepositoryStub{}, &emailSenderStub{})
	if err != nil {
		t.Fatal(err)
	}
	first := service.verificationPIN("EVF-ABCDEFGH", "Owner@Example.com")
	second := service.verificationPIN("EVF-ABCDEFGH", "owner@example.com")
	if first != second {
		t.Fatalf("expected normalized email to produce the same PIN, got %q and %q", first, second)
	}
	if len(first) != 6 || !allDigits(first) {
		t.Fatalf("PIN = %q, want six digits", first)
	}
	if first == service.verificationPIN("EVF-OTHER123", "owner@example.com") {
		t.Fatal("different verification IDs should not produce the same test PIN")
	}
}

func TestNewEmailServiceValidatesSenderAddresses(t *testing.T) {
	cfg := emailTestConfig()
	cfg.EmailFromAddress = "not-an-email"
	if _, err := NewEmailService(cfg, &emailRepositoryStub{}, &emailSenderStub{}); err == nil || !strings.Contains(err.Error(), "EMAIL_FROM_ADDRESS") {
		t.Fatalf("invalid sender error = %v", err)
	}

	cfg = emailTestConfig()
	cfg.EmailReplyTo = "not-an-email"
	if _, err := NewEmailService(cfg, &emailRepositoryStub{}, &emailSenderStub{}); err == nil || !strings.Contains(err.Error(), "EMAIL_REPLY_TO") {
		t.Fatalf("invalid reply-to error = %v", err)
	}
}

func TestVerifyMerchantEmailPassesConstantTimeResultToRepository(t *testing.T) {
	now := time.Now().UTC()
	merchant := domain.Merchant{MerchantID: "MCH-TEST1234", FirstName: "Gary", Merchant: "Daily Brew", MerchantEmail: "owner@example.com", Status: "pending_verification"}
	repo := &emailRepositoryStub{
		verificationResult: domain.EmailVerificationResult{
			Merchant:     merchant,
			Verification: domain.MerchantEmailVerification{VerificationID: "EVF-TEST1234", MerchantID: merchant.MerchantID, RecipientEmail: merchant.MerchantEmail, ExpiresAt: now.Add(time.Minute)},
		},
		appliedResult: domain.EmailVerificationResult{Merchant: domain.Merchant{MerchantID: merchant.MerchantID, Status: "active"}},
	}
	service, err := NewEmailService(emailTestConfig(), repo, &emailSenderStub{})
	if err != nil {
		t.Fatal(err)
	}
	pin := service.verificationPIN("EVF-TEST1234", merchant.MerchantEmail)
	result, err := service.VerifyMerchantEmail(context.Background(), merchant.MerchantEmail, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.appliedInput.CodeValid {
		t.Fatal("expected matching PIN to be marked valid")
	}
	if repo.appliedInput.WelcomeJob.IdempotencyKey != "merchant-welcome:"+merchant.MerchantID {
		t.Fatalf("unexpected welcome idempotency key %q", repo.appliedInput.WelcomeJob.IdempotencyKey)
	}
	if result.Merchant.Status != "active" {
		t.Fatalf("verified merchant status = %q", result.Merchant.Status)
	}
}

func TestVerificationEmailTemplateEscapesMerchantContent(t *testing.T) {
	repo := &emailRepositoryStub{
		merchant:           domain.Merchant{MerchantID: "MCH-TEST1234", FirstName: `<script>alert("x")</script>`, Merchant: `<b>Coffee</b>`},
		verificationResult: domain.EmailVerificationResult{Verification: domain.MerchantEmailVerification{VerificationID: "EVF-TEST1234", ExpiresAt: time.Now().UTC().Add(time.Minute), MaxAttempts: 5}},
	}
	service, err := NewEmailService(emailTestConfig(), repo, &emailSenderStub{})
	if err != nil {
		t.Fatal(err)
	}
	message, err := service.messageForJob(context.Background(), domain.EmailJob{JobType: "merchant_verification", MerchantID: repo.merchant.MerchantID, VerificationID: "EVF-TEST1234", RecipientEmail: "owner@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message.HTMLBody, `<script>`) {
		t.Fatalf("HTML template did not escape merchant content: %s", message.HTMLBody)
	}
	if !strings.Contains(message.HTMLBody, "&lt;script&gt;") {
		t.Fatalf("escaped merchant name is missing: %s", message.HTMLBody)
	}
}

func TestVerificationEmailRepositoryFailureRemainsRetryable(t *testing.T) {
	repositoryFailure := errors.New("database temporarily unavailable")
	repo := &emailRepositoryStub{
		merchant:        domain.Merchant{MerchantID: "MCH-TEST1234", FirstName: "Gary", Merchant: "Daily Brew"},
		verificationErr: repositoryFailure,
	}
	service, err := NewEmailService(emailTestConfig(), repo, &emailSenderStub{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.messageForJob(context.Background(), domain.EmailJob{JobType: "merchant_verification", MerchantID: repo.merchant.MerchantID, VerificationID: "EVF-TEST1234", RecipientEmail: "owner@example.com"})
	if !errors.Is(err, repositoryFailure) {
		t.Fatalf("messageForJob error = %v, want repository failure", err)
	}
}

func TestProcessEmailJobsCompletesAndRetries(t *testing.T) {
	job := domain.EmailJob{EmailJobID: "EMJ-TEST1234", JobType: "merchant_welcome", MerchantID: "MCH-TEST1234", RecipientEmail: "owner@example.com", MessageID: "<test@justloyalty.com>"}
	repo := &emailRepositoryStub{merchant: domain.Merchant{MerchantID: job.MerchantID, FirstName: "Gary", Merchant: "Daily Brew", MerchantEmail: job.RecipientEmail}, jobs: []domain.EmailJob{job}}
	sender := &emailSenderStub{}
	service, err := NewEmailService(emailTestConfig(), repo, sender)
	if err != nil {
		t.Fatal(err)
	}
	service.processJobs(context.Background())
	if repo.completedJobID != job.EmailJobID {
		t.Fatalf("completed job = %q", repo.completedJobID)
	}
	if sender.message.Subject != "Welcome to Just Loyalty" {
		t.Fatalf("welcome subject = %q", sender.message.Subject)
	}

	repo.completedJobID = ""
	repo.jobs = []domain.EmailJob{job}
	sender.err = errors.New("temporary Gmail API failure")
	service.processJobs(context.Background())
	if repo.retriedJobID != job.EmailJobID {
		t.Fatalf("retried job = %q", repo.retriedJobID)
	}
}

func TestBuildMIMEMessageRemovesHeaderNewlines(t *testing.T) {
	payload, err := buildMIMEMessage(EmailMessage{FromName: "Just Loyalty\r\nBcc: attacker@example.com", FromEmail: "hello@justloyalty.com", To: "owner@example.com", Subject: "Verify\r\nBcc: attacker@example.com", TextBody: "Text", HTMLBody: "<p>HTML</p>", MessageID: "<test@justloyalty.com>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "\r\nBcc:") {
		t.Fatalf("message contains an injected Bcc header: %s", payload)
	}
}

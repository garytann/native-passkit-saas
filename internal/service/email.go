package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"log"
	"strings"
	texttemplate "text/template"
	"time"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
)

const (
	verificationTTL      = 10 * time.Minute
	verificationCooldown = time.Minute
)

var errVerificationEmailSuperseded = errors.New("verification email is expired or superseded")

type EmailRepository interface {
	CreatePendingRegistration(context.Context, domain.PendingMerchantRegistration) (domain.EmailVerificationResult, error)
	VerificationByEmail(context.Context, string) (domain.EmailVerificationResult, error)
	ApplyVerificationAttempt(context.Context, domain.VerifyMerchantEmailInput) (domain.EmailVerificationResult, error)
	ResendVerification(context.Context, domain.MerchantEmailVerification, domain.EmailJob) error
	MerchantByID(context.Context, string) (domain.Merchant, error)
	ClaimPendingEmailJobs(context.Context, int) ([]domain.EmailJob, error)
	CompleteEmailJob(context.Context, string) error
	RetryEmailJob(context.Context, domain.EmailJob, string) error
	FailEmailJob(context.Context, string, string) error
}

type EmailSender interface {
	Send(context.Context, EmailMessage) error
}

type EmailMessage struct {
	To        string
	FromName  string
	FromEmail string
	ReplyTo   string
	Subject   string
	TextBody  string
	HTMLBody  string
	MessageID string
}

type EmailService struct {
	cfg    config.Config
	repo   EmailRepository
	sender EmailSender
}

type VerifyEmailResult struct {
	Merchant        domain.Merchant
	AlreadyVerified bool
}

func NewEmailService(cfg config.Config, repo EmailRepository, sender EmailSender) (EmailService, error) {
	service := EmailService{cfg: cfg, repo: repo, sender: sender}
	if !cfg.EmailEnabled {
		return service, nil
	}
	if repo == nil {
		return EmailService{}, ErrRepositoryUnavailable
	}
	if sender == nil {
		return EmailService{}, errors.New("email sender is required when email delivery is enabled")
	}
	if len(strings.TrimSpace(cfg.EmailVerificationSecret)) < 32 {
		return EmailService{}, errors.New("EMAIL_VERIFICATION_SECRET must contain at least 32 characters")
	}
	if strings.TrimSpace(cfg.EmailFromAddress) == "" {
		return EmailService{}, errors.New("EMAIL_FROM_ADDRESS is required when email delivery is enabled")
	}
	if !validEmailAddress(cfg.EmailFromAddress) {
		return EmailService{}, errors.New("EMAIL_FROM_ADDRESS must be a valid email address")
	}
	if strings.TrimSpace(cfg.EmailReplyTo) != "" && !validEmailAddress(cfg.EmailReplyTo) {
		return EmailService{}, errors.New("EMAIL_REPLY_TO must be a valid email address")
	}
	return service, nil
}

func (s EmailService) Enabled() bool {
	return s.cfg.EmailEnabled && s.repo != nil && s.sender != nil
}

func (s EmailService) CreatePendingRegistration(ctx context.Context, merchant domain.CreateMerchantInput, issuer domain.CreateAppleWalletIssuerInput) (domain.EmailVerificationResult, error) {
	if !s.Enabled() {
		return domain.EmailVerificationResult{}, ErrEmailUnavailable
	}
	verification, job, err := s.newVerification(merchant.MerchantID, merchant.MerchantEmail, merchant.FirstName)
	if err != nil {
		return domain.EmailVerificationResult{}, err
	}
	return s.repo.CreatePendingRegistration(ctx, domain.PendingMerchantRegistration{
		Merchant: merchant, Issuer: issuer, Verification: verification, EmailJob: job,
	})
}

func (s EmailService) VerifyMerchantEmail(ctx context.Context, email, pin string) (VerifyEmailResult, error) {
	if !s.Enabled() {
		return VerifyEmailResult{}, ErrEmailUnavailable
	}
	email = strings.TrimSpace(strings.ToLower(email))
	pin = strings.TrimSpace(pin)
	if email == "" || !validEmailAddress(email) {
		return VerifyEmailResult{}, ValidationError{Message: "merchant_email must be valid"}
	}
	if len(pin) != 6 || !allDigits(pin) {
		return VerifyEmailResult{}, ValidationError{Message: "pin must contain 6 digits"}
	}

	current, err := s.repo.VerificationByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrEmailVerificationNotFound) {
			return VerifyEmailResult{}, domain.ErrEmailVerificationInvalid
		}
		return VerifyEmailResult{}, err
	}
	if current.AlreadyVerified {
		return VerifyEmailResult{Merchant: current.Merchant, AlreadyVerified: true}, nil
	}
	expected := s.verificationPIN(current.Verification.VerificationID, current.Verification.RecipientEmail)
	valid := subtle.ConstantTimeCompare([]byte(expected), []byte(pin)) == 1
	welcome, err := s.newWelcomeJob(current.Merchant)
	if err != nil {
		return VerifyEmailResult{}, err
	}
	result, err := s.repo.ApplyVerificationAttempt(ctx, domain.VerifyMerchantEmailInput{
		VerificationID: current.Verification.VerificationID,
		CodeValid:      valid,
		WelcomeJob:     welcome,
	})
	if err != nil {
		return VerifyEmailResult{}, err
	}
	return VerifyEmailResult{Merchant: result.Merchant, AlreadyVerified: result.AlreadyVerified}, nil
}

func (s EmailService) ResendVerification(ctx context.Context, email string) error {
	if !s.Enabled() {
		return ErrEmailUnavailable
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !validEmailAddress(email) {
		return ValidationError{Message: "merchant_email must be valid"}
	}
	current, err := s.repo.VerificationByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrEmailVerificationNotFound) {
			return nil
		}
		return err
	}
	if current.AlreadyVerified {
		return nil
	}
	verification, job, err := s.newVerification(current.Merchant.MerchantID, email, current.Merchant.FirstName)
	if err != nil {
		return err
	}
	if err := s.repo.ResendVerification(ctx, verification, job); err != nil {
		if errors.Is(err, domain.ErrEmailVerificationNotFound) || errors.Is(err, domain.ErrEmailVerificationRateLimited) {
			return nil
		}
		return err
	}
	return nil
}

func (s EmailService) RunWorker(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processJobs(ctx)
		}
	}
}

func (s EmailService) processJobs(ctx context.Context) {
	jobs, err := s.repo.ClaimPendingEmailJobs(ctx, 20)
	if err != nil {
		log.Printf("claim email jobs: %v", err)
		return
	}
	for _, job := range jobs {
		log.Printf("process email job id=%s type=%s attempt=%d", job.EmailJobID, job.JobType, job.Attempts)
		message, err := s.messageForJob(ctx, job)
		if errors.Is(err, errVerificationEmailSuperseded) {
			if failErr := s.repo.FailEmailJob(ctx, job.EmailJobID, err.Error()); failErr != nil {
				log.Printf("permanently fail superseded email job %s: %v", job.EmailJobID, failErr)
			}
			continue
		}
		if err == nil {
			err = s.sender.Send(ctx, message)
		}
		if err == nil {
			if completeErr := s.repo.CompleteEmailJob(ctx, job.EmailJobID); completeErr != nil {
				log.Printf("complete email job %s: %v", job.EmailJobID, completeErr)
			} else {
				log.Printf("email job sent id=%s type=%s attempt=%d", job.EmailJobID, job.JobType, job.Attempts)
			}
			continue
		}
		log.Printf("email job failed id=%s type=%s attempt=%d: %v", job.EmailJobID, job.JobType, job.Attempts, err)
		if retryErr := s.repo.RetryEmailJob(ctx, job, err.Error()); retryErr != nil {
			log.Printf("retry email job %s: %v", job.EmailJobID, retryErr)
		}
	}
}

func (s EmailService) messageForJob(ctx context.Context, job domain.EmailJob) (EmailMessage, error) {
	merchant, err := s.repo.MerchantByID(ctx, job.MerchantID)
	if err != nil {
		return EmailMessage{}, err
	}
	data := emailTemplateData{
		FirstName: merchant.FirstName,
		Company:   merchant.Merchant,
		LoginURL:  strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/login",
	}
	subject := ""
	textSource := ""
	htmlSource := ""
	switch job.JobType {
	case "merchant_verification":
		if job.VerificationID == "" {
			return EmailMessage{}, errors.New("verification email job has no verification id")
		}
		current, err := s.repo.VerificationByEmail(ctx, job.RecipientEmail)
		if err != nil {
			if errors.Is(err, domain.ErrEmailVerificationNotFound) {
				return EmailMessage{}, errVerificationEmailSuperseded
			}
			return EmailMessage{}, err
		}
		if current.AlreadyVerified || current.Verification.VerificationID != job.VerificationID || !time.Now().UTC().Before(current.Verification.ExpiresAt) || current.Verification.Attempts >= current.Verification.MaxAttempts {
			return EmailMessage{}, errVerificationEmailSuperseded
		}
		data.PIN = s.verificationPIN(job.VerificationID, job.RecipientEmail)
		subject = "Verify your Just Loyalty merchant account"
		textSource = verificationTextTemplate
		htmlSource = verificationHTMLTemplate
	case "merchant_welcome":
		subject = "Welcome to Just Loyalty"
		textSource = welcomeTextTemplate
		htmlSource = welcomeHTMLTemplate
	default:
		return EmailMessage{}, fmt.Errorf("unsupported email job type %q", job.JobType)
	}
	textBody, err := executeTextTemplate(job.JobType+"-text", textSource, data)
	if err != nil {
		return EmailMessage{}, err
	}
	htmlBody, err := executeHTMLTemplate(job.JobType+"-html", htmlSource, data)
	if err != nil {
		return EmailMessage{}, err
	}
	return EmailMessage{To: job.RecipientEmail, FromName: s.cfg.EmailFromName, FromEmail: s.cfg.EmailFromAddress, ReplyTo: s.cfg.EmailReplyTo, Subject: subject, TextBody: textBody, HTMLBody: htmlBody, MessageID: job.MessageID}, nil
}

func (s EmailService) newVerification(merchantID, email, firstName string) (domain.MerchantEmailVerification, domain.EmailJob, error) {
	verificationID, err := domain.NewEmailVerificationID()
	if err != nil {
		return domain.MerchantEmailVerification{}, domain.EmailJob{}, err
	}
	jobID, err := domain.NewEmailJobID()
	if err != nil {
		return domain.MerchantEmailVerification{}, domain.EmailJob{}, err
	}
	now := time.Now().UTC()
	verification := domain.MerchantEmailVerification{VerificationID: verificationID, MerchantID: merchantID, RecipientEmail: email, ExpiresAt: now.Add(verificationTTL), MaxAttempts: 5, ResendAvailableAt: now.Add(verificationCooldown), CreatedAt: now, UpdatedAt: now}
	job := domain.EmailJob{EmailJobID: jobID, JobType: "merchant_verification", MerchantID: merchantID, VerificationID: verificationID, RecipientEmail: email, RecipientName: firstName, Status: "pending", MaxAttempts: 8, NextAttemptAt: now, MessageID: messageID(jobID, s.cfg.EmailFromAddress), IdempotencyKey: "merchant-verification:" + verificationID, CreatedAt: now, UpdatedAt: now}
	return verification, job, nil
}

func (s EmailService) newWelcomeJob(merchant domain.Merchant) (domain.EmailJob, error) {
	jobID, err := domain.NewEmailJobID()
	if err != nil {
		return domain.EmailJob{}, err
	}
	now := time.Now().UTC()
	return domain.EmailJob{EmailJobID: jobID, JobType: "merchant_welcome", MerchantID: merchant.MerchantID, RecipientEmail: merchant.MerchantEmail, RecipientName: merchant.FirstName, Status: "pending", MaxAttempts: 8, NextAttemptAt: now, MessageID: messageID(jobID, s.cfg.EmailFromAddress), IdempotencyKey: "merchant-welcome:" + merchant.MerchantID, CreatedAt: now, UpdatedAt: now}, nil
}

func (s EmailService) verificationPIN(verificationID, email string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.EmailVerificationSecret))
	_, _ = mac.Write([]byte(verificationID + "|" + strings.ToLower(strings.TrimSpace(email))))
	value := binary.BigEndian.Uint32(mac.Sum(nil)[:4]) % 1000000
	return fmt.Sprintf("%06d", value)
}

func messageID(jobID, from string) string {
	domainName := "just-loyalty.local"
	if _, candidate, ok := strings.Cut(strings.TrimSpace(from), "@"); ok && candidate != "" {
		domainName = candidate
	}
	return fmt.Sprintf("<%s@%s>", strings.ToLower(jobID), domainName)
}

func allDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

type emailTemplateData struct {
	FirstName string
	Company   string
	PIN       string
	LoginURL  string
}

func executeTextTemplate(name, source string, data emailTemplateData) (string, error) {
	parsed, err := texttemplate.New(name).Parse(source)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	if err := parsed.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

func executeHTMLTemplate(name, source string, data emailTemplateData) (string, error) {
	parsed, err := template.New(name).Parse(source)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	if err := parsed.Execute(&output, data); err != nil {
		return "", err
	}
	return output.String(), nil
}

const verificationTextTemplate = `Hi {{.FirstName}},

Use this verification code to confirm your Just Loyalty merchant account:

{{.PIN}}

This code expires in 10 minutes. If you did not create this account, you can ignore this email.

Just Loyalty
`

const verificationHTMLTemplate = `<!doctype html><html><body style="margin:0;background:#f4f6fb;font-family:Arial,sans-serif;color:#17191f"><div style="max-width:560px;margin:0 auto;padding:40px 20px"><div style="background:#fff;border:1px solid #e2e6ee;border-radius:8px;padding:32px"><div style="font-size:14px;font-weight:700;color:#5434f5">JUST LOYALTY</div><h1 style="font-size:26px;margin:18px 0 12px">Verify your merchant account</h1><p style="line-height:1.6;color:#59616e">Hi {{.FirstName}}, use the code below to confirm your email address.</p><div style="margin:28px 0;padding:18px;background:#f4f1ff;border-radius:8px;text-align:center;font-size:34px;font-weight:800;letter-spacing:8px;color:#351db8">{{.PIN}}</div><p style="line-height:1.6;color:#59616e">This code expires in 10 minutes. If you did not create this account, you can ignore this email.</p></div></div></body></html>`

const welcomeTextTemplate = `Hi {{.FirstName}},

Welcome to Just Loyalty. Your merchant account for {{.Company}} has been verified.

Sign in to begin setting up your loyalty programme:
{{.LoginURL}}

Just Loyalty
`

const welcomeHTMLTemplate = `<!doctype html><html><body style="margin:0;background:#f4f6fb;font-family:Arial,sans-serif;color:#17191f"><div style="max-width:560px;margin:0 auto;padding:40px 20px"><div style="background:#fff;border:1px solid #e2e6ee;border-radius:8px;padding:32px"><div style="font-size:14px;font-weight:700;color:#5434f5">JUST LOYALTY</div><h1 style="font-size:26px;margin:18px 0 12px">Welcome aboard</h1><p style="line-height:1.6;color:#59616e">Hi {{.FirstName}}, your merchant account for <strong>{{.Company}}</strong> has been verified. You can now begin setting up your loyalty programme.</p><a href="{{.LoginURL}}" style="display:inline-block;margin-top:18px;padding:14px 22px;border-radius:8px;background:#5434f5;color:#fff;text-decoration:none;font-weight:700">Sign in to Just Loyalty</a></div></div></body></html>`

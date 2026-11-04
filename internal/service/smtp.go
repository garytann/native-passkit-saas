package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/config"
)

type SMTPEmailSender struct {
	host     string
	port     string
	username string
	password string
}

func NewSMTPEmailSender(cfg config.Config) (SMTPEmailSender, error) {
	if !cfg.EmailEnabled {
		return SMTPEmailSender{}, nil
	}
	host := strings.TrimSpace(cfg.EmailSMTPHost)
	port := strings.TrimSpace(cfg.EmailSMTPPort)
	username := strings.TrimSpace(cfg.EmailSMTPUsername)
	passwordPath := strings.TrimSpace(cfg.EmailSMTPPasswordFile)
	if host == "" || port == "" || username == "" || passwordPath == "" {
		return SMTPEmailSender{}, errors.New("EMAIL_SMTP_HOST, EMAIL_SMTP_PORT, EMAIL_SMTP_USERNAME, and EMAIL_SMTP_PASSWORD_FILE are required")
	}
	contents, err := os.ReadFile(passwordPath)
	if err != nil {
		return SMTPEmailSender{}, fmt.Errorf("read SMTP password file: %w", err)
	}
	password := strings.ReplaceAll(strings.TrimSpace(string(contents)), " ", "")
	if password == "" {
		return SMTPEmailSender{}, errors.New("SMTP password file is empty")
	}
	return SMTPEmailSender{host: host, port: port, username: username, password: password}, nil
}

func (s SMTPEmailSender) Send(ctx context.Context, message EmailMessage) error {
	address := net.JoinHostPort(s.host, s.port)
	connection, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect SMTP server: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(30 * time.Second))

	client, err := smtp.NewClient(connection, s.host)
	if err != nil {
		return fmt.Errorf("start SMTP client: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("SMTP server does not advertise STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("start SMTP TLS: %w", err)
	}
	if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
		return fmt.Errorf("authenticate SMTP account: %w", err)
	}
	if err := client.Mail(message.FromEmail); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	payload, err := buildMIMEMessage(message)
	if err != nil {
		_ = writer.Close()
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func buildMIMEMessage(message EmailMessage) ([]byte, error) {
	var alternatives bytes.Buffer
	parts := multipart.NewWriter(&alternatives)
	if err := writeAlternative(parts, "text/plain; charset=UTF-8", message.TextBody); err != nil {
		return nil, err
	}
	if err := writeAlternative(parts, "text/html; charset=UTF-8", message.HTMLBody); err != nil {
		return nil, err
	}
	if err := parts.Close(); err != nil {
		return nil, fmt.Errorf("close MIME body: %w", err)
	}

	fromName := mime.QEncoding.Encode("UTF-8", cleanHeader(message.FromName))
	var output bytes.Buffer
	fmt.Fprintf(&output, "From: %s <%s>\r\n", fromName, cleanHeader(message.FromEmail))
	fmt.Fprintf(&output, "To: %s\r\n", cleanHeader(message.To))
	if strings.TrimSpace(message.ReplyTo) != "" {
		fmt.Fprintf(&output, "Reply-To: %s\r\n", cleanHeader(message.ReplyTo))
	}
	fmt.Fprintf(&output, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", cleanHeader(message.Subject)))
	fmt.Fprintf(&output, "Message-ID: %s\r\n", cleanHeader(message.MessageID))
	fmt.Fprintf(&output, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&output, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&output, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", parts.Boundary())
	output.Write(alternatives.Bytes())
	return output.Bytes(), nil
}

func writeAlternative(writer *multipart.Writer, contentType, body string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create MIME part: %w", err)
	}
	encoded := quotedprintable.NewWriter(part)
	if _, err := encoded.Write([]byte(body)); err != nil {
		return fmt.Errorf("write MIME part: %w", err)
	}
	if err := encoded.Close(); err != nil {
		return fmt.Errorf("close MIME part: %w", err)
	}
	return nil
}

func cleanHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}

package service

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
	"time"
)

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
	fmt.Fprint(&output, "MIME-Version: 1.0\r\n")
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

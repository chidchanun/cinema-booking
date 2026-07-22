package notification

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

var ErrEmailNotConfigured = errors.New("email service is not configured")

type SMTPEmailSender struct {
	host, port, username, password, from string
}

func NewSMTPEmailSender(host, port, username, password, from string) *SMTPEmailSender {
	return &SMTPEmailSender{
		host: strings.TrimSpace(host), port: strings.TrimSpace(port),
		username: strings.TrimSpace(username), password: password, from: strings.TrimSpace(from),
	}
}

func (s *SMTPEmailSender) Send(ctx context.Context, to, subject, body string) error {
	if s.host == "" || s.port == "" || s.from == "" {
		return ErrEmailNotConfigured
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fromAddress := s.from
	if start := strings.LastIndex(fromAddress, "<"); start >= 0 && strings.HasSuffix(fromAddress, ">") {
		fromAddress = fromAddress[start+1 : len(fromAddress)-1]
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	message := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body,
	))
	if err := smtp.SendMail(s.host+":"+s.port, auth, fromAddress, []string{to}, message); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

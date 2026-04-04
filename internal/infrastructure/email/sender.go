package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog/log"
)

// Sender implements domain.EmailSender.
type Sender struct {
	host     string
	port     int
	user     string
	pass     string
	fromName string
	fromAddr string
}

// NewSender creates an SMTP email sender.
func NewSender(host string, port int, user, pass, fromName, fromAddr string) *Sender {
	return &Sender{
		host: host, port: port, user: user, pass: pass,
		fromName: fromName, fromAddr: fromAddr,
	}
}

// Send delivers an email via SMTP.
func (s *Sender) Send(ctx context.Context, to, subject, bodyHTML string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	from := s.fromAddr
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddr)
	}

	var msg strings.Builder
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(bodyHTML)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	if err := smtp.SendMail(addr, auth, s.fromAddr, []string{to}, []byte(msg.String())); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}

	log.Info().Str("to", to).Str("subject", subject).Msg("email sent")
	return nil
}

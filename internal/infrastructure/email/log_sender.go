package email

import (
	"context"

	"github.com/rs/zerolog/log"
)

// LogSender is a dev-only email sender that logs instead of sending.
type LogSender struct{}

// NewLogSender creates a dev-only email sender.
func NewLogSender() *LogSender {
	return &LogSender{}
}

// Send logs the email content instead of delivering it.
func (s *LogSender) Send(ctx context.Context, to, subject, bodyHTML string) error {
	log.Info().
		Str("to", to).
		Str("subject", subject).
		Int("body_len", len(bodyHTML)).
		Msg("[DEV] email logged (not sent)")
	return nil
}

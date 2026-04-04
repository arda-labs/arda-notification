package domain

import "context"

// EmailSender is the port for sending notification emails.
type EmailSender interface {
	// Send delivers an HTML email to the given address.
	Send(ctx context.Context, to, subject, bodyHTML string) error
}

// Package mail sends account emails (recovery address verification and
// password reset). No email provider is chosen yet (GAP-16): without one the
// email features are switched off, except in local development, where the
// message is written to the server log instead.
package mail

import (
	"context"
	"log/slog"
)

type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogSender writes messages to the log. LOCAL DEVELOPMENT ONLY.
type LogSender struct{ Log *slog.Logger }

func (s LogSender) Send(_ context.Context, to, subject, body string) error {
	s.Log.Warn("DEV MAIL (not sent)", "to", to, "subject", subject, "body", body)
	return nil
}

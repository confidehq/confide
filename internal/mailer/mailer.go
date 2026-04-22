package mailer

import (
	"fmt"
	"net/smtp"

	"github.com/rs/zerolog/log"
)

// Mailer sends transactional email over SMTP.
// If Host is empty all sends are no-ops (useful in development/test).
type Mailer struct {
	Host      string
	Port      string
	User      string
	Pass      string
	FromEmail string
}

// New constructs a Mailer. When host is empty, sends are silently skipped.
func New(host, port, user, pass, fromEmail string) *Mailer {
	return &Mailer{
		Host:      host,
		Port:      port,
		User:      user,
		Pass:      pass,
		FromEmail: fromEmail,
	}
}

// SendInvitation sends a workspace invitation email.
// link is the full accept URL including the raw token.
func (m *Mailer) SendInvitation(to, workspaceName, inviterUsername, role, link string) {
	if m.Host == "" {
		log.Info().Str("to", to).Msg("mailer: SMTP not configured, skipping invitation email")
		return
	}

	subject := fmt.Sprintf("You've been invited to %s on Confide", workspaceName)
	body := fmt.Sprintf(invitationBody, inviterUsername, workspaceName, role, link)
	msg := buildMessage(m.FromEmail, to, subject, body)

	addr := m.Host + ":" + m.Port
	auth := smtp.PlainAuth("", m.User, m.Pass, m.Host)
	if err := smtp.SendMail(addr, auth, m.FromEmail, []string{to}, []byte(msg)); err != nil {
		log.Error().Str("to", to).Err(err).Msg("mailer: failed to send invitation")
	}
}

func buildMessage(from, to, subject, body string) string {
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body,
	)
}

const invitationBody = `%s has invited you to join the "%s" workspace on Confide with the role: %s.

Follow this link to accept the invitation:
%s

This link expires in 7 days. If you were not expecting this invitation, you can safely ignore it.

— The Confide Team
`

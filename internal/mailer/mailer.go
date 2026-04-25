package mailer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Mailer sends transactional email over SMTP or the Resend REST API.
// If both ResendAPIKey and Host are empty, all sends are no-ops (useful in development/test).
type Mailer struct {
	log          zerolog.Logger
	Host         string
	Port         string
	User         string
	Pass         string
	FromEmail    string
	ResendAPIKey string
}

// New constructs a Mailer. When resendAPIKey is non-empty it is preferred over SMTP.
// When both are empty, sends are silently skipped.
func New(host, port, user, pass, fromEmail, resendAPIKey string) *Mailer {
	return &Mailer{
		log:          log.With().Str("module", "mailer").Logger(),
		Host:         host,
		Port:         port,
		User:         user,
		Pass:         pass,
		FromEmail:    fromEmail,
		ResendAPIKey: resendAPIKey,
	}
}

// SendInvitation sends a workspace invitation email.
func (m *Mailer) SendInvitation(to, workspaceName, inviterUsername, role, link string) {
	subject := fmt.Sprintf("You've been invited to %s on Confide", workspaceName)
	body := fmt.Sprintf(invitationBody, inviterUsername, workspaceName, role, link)

	var err error
	if m.ResendAPIKey != "" {
		err = m.resendText(to, subject, body)
	} else if m.Host != "" {
		msg := buildPlainMessage(m.FromEmail, to, subject, body)
		err = m.smtp(to, msg)
	} else {
		m.log.Info().Str("to", to).Msg("mailer: not configured, skipping invitation email")
		return
	}
	if err != nil {
		m.log.Error().Str("to", to).Err(err).Msg("mailer: failed to send invitation")
	}
}

// SendPGPResponse forwards a PGP-encrypted form response.
// Via Resend: sends the armored ciphertext as a text/plain body — Proton Mail
// recognises PGP/Inline and decrypts it automatically.
// Via SMTP: uses RFC 3156 PGP/MIME for broader client compatibility.
func (m *Mailer) SendPGPResponse(to, formID, armoredData string) {
	subject := "New form response"
	var err error
	if m.ResendAPIKey != "" {
		err = m.resendText(to, subject, armoredData)
	} else if m.Host != "" {
		msg := buildPGPMIMEMessage(m.FromEmail, to, subject, armoredData)
		err = m.smtp(to, msg)
	} else {
		m.log.Info().Str("to", to).Msg("mailer: not configured, skipping PGP notification")
		return
	}
	if err != nil {
		m.log.Error().Str("to", to).Str("formId", formID).Err(err).Msg("mailer: failed to send PGP notification")
	}
}

// resendText posts a plain-text email via the Resend REST API.
func (m *Mailer) resendText(to, subject, text string) error {
	body, err := json.Marshal(map[string]any{
		"from":    m.FromEmail,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("mailer: marshal resend payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailer: build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: resend request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("mailer: resend returned %d: %v", resp.StatusCode, errBody)
	}
	return nil
}

// smtp sends a raw RFC 2822 message via SMTP.
func (m *Mailer) smtp(to, rawMessage string) error {
	addr := m.Host + ":" + m.Port
	auth := smtp.PlainAuth("", m.User, m.Pass, m.Host)
	if err := smtp.SendMail(addr, auth, m.FromEmail, []string{to}, []byte(rawMessage)); err != nil {
		return fmt.Errorf("mailer: smtp: %w", err)
	}
	return nil
}

// buildPGPMIMEMessage constructs a RFC 3156 PGP/MIME encrypted message.
func buildPGPMIMEMessage(from, to, subject, armoredData string) string {
	boundary := "pgp-" + randomHex(12)
	return fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/encrypted; boundary=\"%s\"; protocol=\"application/pgp-encrypted\"\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: application/pgp-encrypted\r\n"+
			"\r\n"+
			"Version: 1\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: application/octet-stream; name=\"encrypted.asc\"\r\n"+
			"Content-Disposition: inline; filename=\"encrypted.asc\"\r\n"+
			"\r\n"+
			"%s\r\n"+
			"--%s--\r\n",
		from, to, subject,
		boundary,
		boundary,
		boundary,
		armoredData,
		boundary,
	)
}

func buildPlainMessage(from, to, subject, body string) string {
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body,
	)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

const invitationBody = `%s has invited you to join the "%s" workspace on Confide with the role: %s.

Follow this link to accept the invitation:
%s

This link expires in 7 days. If you were not expecting this invitation, you can safely ignore it.

— The Confide Team
`

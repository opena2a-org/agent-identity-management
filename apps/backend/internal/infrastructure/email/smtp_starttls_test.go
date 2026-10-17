package email

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email/smtptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// STARTTLS, AUTH PLAIN and Message-ID against a loopback SMTP server
// ===========================

const starttlsFromDomain = "mail.example.org"

// starttlsCredentials builds a username and password at run time.
func starttlsCredentials(t *testing.T) (string, string) {
	t.Helper()
	b := make([]byte, 12)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "aim-relay-" + hex.EncodeToString(b[:4]), hex.EncodeToString(b[4:])
}

// newSTARTTLSSink starts a loopback SMTP server with opts and stops it when
// the test ends.
func newSTARTTLSSink(t *testing.T, opts ...smtptest.Option) *smtptest.Server {
	t.Helper()
	sink, err := smtptest.NewServer(opts...)
	require.NoError(t, err)
	t.Cleanup(sink.Close)
	return sink
}

// newSTARTTLSService returns an SMTP email service with TLS enabled that
// sends to sink and trusts the sink's certificate.
func newSTARTTLSService(t *testing.T, sink *smtptest.Server, username, password string) *SMTPEmailService {
	t.Helper()
	service, err := NewSMTPEmailService(domain.EmailConfig{
		FromAddress: "noreply@" + starttlsFromDomain,
		FromName:    "Agent Identity Management",
		SMTP: domain.SMTPConfig{
			Host:       sink.Host(),
			Port:       sink.Port(),
			Username:   username,
			Password:   password,
			TLSEnabled: true,
		},
	})
	require.NoError(t, err)
	service.rootCAs = sink.CertPool()
	return service
}

// messageIDs returns every Message-ID header value of a received message.
func messageIDs(t *testing.T, data string) []string {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	require.NoError(t, err, "the received message does not parse")
	return msg.Header[textproto.CanonicalMIMEHeaderKey("Message-ID")]
}

// sessionsOf groups the sink's commands by session, in order.
func sessionsOf(commands []smtptest.Command) map[int][]smtptest.Command {
	sessions := map[int][]smtptest.Command{}
	for _, cmd := range commands {
		sessions[cmd.Session] = append(sessions[cmd.Session], cmd)
	}
	return sessions
}

// assertOnlyEHLOAndSTARTTLSInClear checks one session: the commands sent in
// clear text are EHLO then STARTTLS, and every AUTH arrived over TLS.
func assertOnlyEHLOAndSTARTTLSInClear(t *testing.T, session []smtptest.Command) {
	t.Helper()
	var clear []string
	for _, cmd := range session {
		if !cmd.TLS {
			clear = append(clear, cmd.Verb)
		}
		if cmd.Verb == "AUTH" {
			assert.True(t, cmd.TLS, "AUTH arrived in clear text")
		}
	}
	assert.Equal(t, []string{"EHLO", "STARTTLS"}, clear, "commands sent in clear text")
}

// verbs returns the sink's command verbs, in order.
func verbs(commands []smtptest.Command) []string {
	out := make([]string, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, cmd.Verb)
	}
	return out
}

func TestSMTPEmailService_STARTTLSAuthPlainRoundTrip(t *testing.T) {
	t.Run("validates and delivers over TLS after AUTH PLAIN, each message with its own Message-ID", func(t *testing.T) {
		username, password := starttlsCredentials(t)
		sink := newSTARTTLSSink(t, smtptest.RequireSTARTTLS(), smtptest.RequireAuthPlain(username, password))
		service := newSTARTTLSService(t, sink, username, password)

		require.NoError(t, service.ValidateConnection())

		const to = "owner@example.net"
		require.NoError(t, service.SendEmail(to, "Reset your password", "<p>Reset your password.</p>", true))

		txs := sink.Transactions()
		require.Len(t, txs, 1, "one message reached the sink")
		assert.True(t, txs[0].TLS, "the message was sent over TLS")
		assert.Equal(t, username, txs[0].Username, "the session authenticated before the message")
		assert.Equal(t, "noreply@"+starttlsFromDomain, txs[0].From)
		assert.Equal(t, []string{to}, txs[0].To, "envelope recipient")
		assertTrackingSuppressed(t, txs[0].Data)

		sessions := sessionsOf(sink.Commands())
		require.Len(t, sessions, 2, "ValidateConnection and SendEmail each open one session")
		for id, session := range sessions {
			t.Logf("session %d: %v", id, verbs(session))
			assertOnlyEHLOAndSTARTTLSInClear(t, session)
		}

		messageIDPattern := regexp.MustCompile(`^<[0-9a-f]{32}@` + regexp.QuoteMeta(starttlsFromDomain) + `>$`)
		first := messageIDs(t, txs[0].Data)
		require.Len(t, first, 1, "the message carries exactly one Message-ID header")
		assert.Regexp(t, messageIDPattern, first[0])

		require.NoError(t, service.SendEmail(to, "Reset your password", "<p>Reset your password.</p>", true))
		txs = sink.Transactions()
		require.Len(t, txs, 2)
		second := messageIDs(t, txs[1].Data)
		require.Len(t, second, 1, "the second message carries exactly one Message-ID header")
		assert.Regexp(t, messageIDPattern, second[0])
		assert.NotEqual(t, first[0], second[0], "each message gets its own Message-ID")
	})

	t.Run("a wrong password fails with an authentication error and sends nothing", func(t *testing.T) {
		username, password := starttlsCredentials(t)
		sink := newSTARTTLSSink(t, smtptest.RequireSTARTTLS(), smtptest.RequireAuthPlain(username, password))
		service := newSTARTTLSService(t, sink, username, password+"-wrong")

		err := service.ValidateConnection()
		require.Error(t, err)
		assert.ErrorContains(t, err, "authentication failed")

		err = service.SendEmail("owner@example.net", "Reset your password", "body", false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "authentication failed")
		var reply *textproto.Error
		require.True(t, errors.As(err, &reply), "the error carries the server's reply: %v", err)
		assert.Equal(t, 535, reply.Code)

		assert.Empty(t, sink.Transactions(), "no message reached the sink")
		assert.NotContains(t, verbs(sink.Commands()), "MAIL")
	})

	t.Run("a server that offers no STARTTLS is refused before AUTH and MAIL", func(t *testing.T) {
		username, password := starttlsCredentials(t)
		sink := newSTARTTLSSink(t, smtptest.NoSTARTTLS(), smtptest.RequireAuthPlain(username, password))
		service := newSTARTTLSService(t, sink, username, password)

		require.Error(t, service.ValidateConnection())

		err := service.SendEmail("owner@example.net", "Reset your password", "body", false)
		require.Error(t, err)
		assert.ErrorContains(t, err, "TLS")

		sent := verbs(sink.Commands())
		t.Logf("commands: %v", sent)
		assert.NotContains(t, sent, "AUTH")
		assert.NotContains(t, sent, "MAIL")
		assert.Empty(t, sink.Transactions(), "no message reached the sink")
	})
}

// A from address without a domain gives the Message-ID the SMTP host as its
// right-hand side, so the header still has the form <id-left@id-right>.
func TestSMTPEmailService_MessageIDWithoutFromDomainUsesTheSMTPHost(t *testing.T) {
	service, err := NewSMTPEmailService(domain.EmailConfig{
		FromAddress: "noreply",
		SMTP:        domain.SMTPConfig{Host: "smtp.example.com", Port: 587},
	})
	require.NoError(t, err)

	ids := messageIDs(t, service.buildMessage("noreply", "owner@example.net", "Subject", "body", false))
	require.Len(t, ids, 1)
	assert.Regexp(t, `^<[0-9a-f]{32}@smtp\.example\.com>$`, ids[0])
}

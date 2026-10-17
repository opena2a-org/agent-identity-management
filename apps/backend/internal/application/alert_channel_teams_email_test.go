package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email/smtptest"
)

// fakeEmailService records each SendEmail and fails a recipient's first
// failFirst[to] sends, or every send when failFirst[to] is negative.
type fakeEmailService struct {
	mu        sync.Mutex
	failFirst map[string]int
	sent      []sentEmail
	attempts  map[string]int
}

type sentEmail struct {
	to, subject, body string
	isHTML            bool
}

func newFakeEmailService() *fakeEmailService {
	return &fakeEmailService{failFirst: map[string]int{}, attempts: map[string]int{}}
}

func (f *fakeEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts[to]++
	if n := f.failFirst[to]; n < 0 || f.attempts[to] <= n {
		return fmt.Errorf("RCPT TO failed: 550 5.1.1 <%s>: mailbox unavailable", to)
	}
	f.sent = append(f.sent, sentEmail{to: to, subject: subject, body: body, isHTML: isHTML})
	return nil
}

func (f *fakeEmailService) SendTemplatedEmail(domain.EmailTemplate, string, interface{}) error {
	return errors.New("not used")
}

func (f *fakeEmailService) SendBulkEmail([]string, string, string, bool) error {
	return errors.New("not used")
}

func (f *fakeEmailService) ValidateConnection() error { return nil }

func (f *fakeEmailService) messages() []sentEmail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentEmail(nil), f.sent...)
}

func newTestAlertChannelServiceWithEmail(store AlertChannelStore, mailer domain.EmailService) *AlertChannelService {
	s := newTestAlertChannelService(store)
	s.email = mailer
	return s
}

// teamsURL is a workflow URL built from fragments. Any https host is taken.
func teamsURL() string {
	return "https://" + "workflows.example.com" + "/teams/" + "unit-" + uuid.NewString()
}

// The Teams body is the documented message: type message, one attachment of
// contentType application/vnd.microsoft.card.adaptive with contentUrl null,
// and an AdaptiveCard of version 1.2 whose body holds one TextBlock for each
// line of the Slack text, without Slack's escaping.
func TestTeamsAlertMessage_FieldByField(t *testing.T) {
	alert := &domain.Alert{
		ID:          uuid.New(),
		Severity:    domain.AlertSeverityHigh,
		Title:       "<!here> & co",
		Description: "a <@U123> mention",
		AgentName:   "agent <x>",
	}
	raw, err := encodeAlertChannelBody(TeamsAlertMessage(alert, "https://aim.example.test/dashboard/admin/alerts"))
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Len(t, body, 2, "the message carries type and attachments only: %s", raw)
	assert.Equal(t, "message", body["type"])
	attachments, ok := body["attachments"].([]interface{})
	require.True(t, ok, "%s", raw)
	require.Len(t, attachments, 1)
	attachment := attachments[0].(map[string]interface{})
	assert.Len(t, attachment, 3, "contentType, contentUrl and content: %v", attachment)
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", attachment["contentType"])
	contentURL, present := attachment["contentUrl"]
	assert.True(t, present, "contentUrl is sent")
	assert.Nil(t, contentURL, "contentUrl is null")
	content := attachment["content"].(map[string]interface{})
	assert.Len(t, content, 4, "$schema, type, version and body: %v", content)
	assert.Equal(t, "http://adaptivecards.io/schemas/adaptive-card.json", content["$schema"])
	assert.Equal(t, "AdaptiveCard", content["type"])
	assert.Equal(t, "1.2", content["version"])
	assert.Equal(t, []interface{}{
		map[string]interface{}{"type": "TextBlock", "text": "AIM alert, severity high: <!here> & co"},
		map[string]interface{}{"type": "TextBlock", "text": "a <@U123> mention"},
		map[string]interface{}{"type": "TextBlock", "text": "Agent: agent <x>"},
		map[string]interface{}{"type": "TextBlock", "text": "https://aim.example.test/dashboard/admin/alerts"},
	}, content["body"])
	assert.Contains(t, string(raw), `"text":"Agent: agent <x>"`, "the body is not HTML-escaped on the wire")

	// One TextBlock per line of the Slack text, a description's own lines
	// included, and no agent line when the alert names none.
	alert.AgentName = ""
	alert.Description = "first\nsecond"
	message := TeamsAlertMessage(alert, "https://aim.example.test/dashboard/admin/alerts")
	slackLines := strings.Split(SlackAlertText(alert, "https://aim.example.test/dashboard/admin/alerts"), "\n")
	require.Len(t, message.Attachments[0].Content.Body, len(slackLines))
	assert.Equal(t, "second", message.Attachments[0].Content.Body[2].Text)
}

// A Teams delivery is a POST with Content-Type application/json and no
// Authorization header, and any 2xx is delivered.
func TestAlertChannelDelivery_TeamsOnTheWire(t *testing.T) {
	type got struct {
		method, contentType string
		authorization       bool
		body                []byte
	}
	var requests []got
	var mu sync.Mutex
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_, auth := r.Header["Authorization"]
		mu.Lock()
		requests = append(requests, got{r.Method, r.Header.Get("Content-Type"), auth, raw})
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	org := uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelTeams,
		Config: map[string]string{"webhookUrl": teamsURL()}, MinSeverity: "info", IsActive: true}
	store := newFakeAlertChannelStore(channel)
	s := newTestAlertChannelService(store)
	s.newHTTPClient = receiverClient(srv)

	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "warning", Title: "t", Description: "d"})

	require.Len(t, requests, 1)
	assert.Equal(t, http.MethodPost, requests[0].method)
	assert.Equal(t, "application/json", requests[0].contentType)
	assert.False(t, requests[0].authorization, "a Teams delivery carries no Authorization header")
	var message TeamsMessage
	require.NoError(t, json.Unmarshal(requests[0].body, &message))
	assert.Equal(t, "message", message.Type)
	assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "delivered", code: intPtr(202)}}, store.recorded(),
		"202 is a 2xx, so it is delivered")
}

func TestAlertChannelCreate_TeamsConfig(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	teams := func(url string) json.RawMessage { return json.RawMessage(`{"webhookUrl":"` + url + `"}`) }
	for _, tc := range []struct {
		name   string
		config json.RawMessage
		reason string
	}{
		{"http", teams("http://example.com/x"), "config.webhookUrl must be the https URL of a Teams workflow"},
		{"no host", teams("https:///x"), "config.webhookUrl"},
		{"user information", teams("https://user:pass@example.com/x"), "config.webhookUrl"},
		{"not a URL", teams("example.com/x"), "config.webhookUrl"},
		{"no URL", json.RawMessage(`{}`), "config.webhookUrl is required"},
		{"routing key", json.RawMessage(`{"webhookUrl":"` + teamsURL() + `","routingKey":"` + routingKey() + `"}`), "unknown field: config.routingKey"},
		{"recipients", json.RawMessage(`{"webhookUrl":"` + teamsURL() + `","recipients":["a@example.com"]}`), "unknown field: config.recipients"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeAlertChannelStore()
			s := newTestAlertChannelService(store)
			_, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
				Name: strPtr("teams"), Kind: strPtr("teams"), Config: tc.config, MinSeverity: strPtr("info"),
			})
			var requestErr *AlertChannelRequestError
			require.ErrorAs(t, err, &requestErr)
			assert.Contains(t, requestErr.Reason, tc.reason)
			assert.NotContains(t, requestErr.Reason, "pass", "a refusal never repeats the URL")
			assert.Empty(t, store.channels)
		})
	}

	// Any host is accepted; the URL goes through the webhook URL check and is
	// masked like a Slack URL.
	url := teamsURL()
	s := newTestAlertChannelService(newFakeAlertChannelStore())
	var checked string
	s.checkURL = func(raw string) error { checked = raw; return nil }
	channel, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr("teams"), Kind: strPtr("teams"), Config: teams(url), MinSeverity: strPtr("info"),
	})
	require.NoError(t, err)
	assert.Equal(t, url, checked)
	assert.Equal(t, map[string]string{"webhookUrl": url}, channel.Config)
	assert.Nil(t, channel.Recipients)
	assert.Equal(t, map[string]interface{}{"webhookUrl": "****" + url[len(url)-4:]}, channel.View().Config)

	s.checkURL = func(string) error { return fmt.Errorf("%w: private address", ErrWebhookURLRefused) }
	_, err = s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr("teams-2"), Kind: strPtr("teams"), Config: teams(teamsURL()), MinSeverity: strPtr("info"),
	})
	var requestErr *AlertChannelRequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Contains(t, requestErr.Reason, "config.webhookUrl")
}

// Recipients are 1 to 20 bare addresses: net/mail.ParseAddress accepts each,
// and its parsed address is the trimmed entry itself.
func TestCheckAlertChannelRecipients(t *testing.T) {
	twenty := make([]string, 20)
	for i := range twenty {
		twenty[i] = fmt.Sprintf("ops%d@example.com", i)
	}
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{"one", `["ops@example.com"]`, []string{"ops@example.com"}},
		{"trimmed", `["  ops@example.com\t"]`, []string{"ops@example.com"}},
		{"twenty", mustJSON(t, twenty), twenty},
		{"subdomain and plus", `["sec+alerts@mail.example.com"]`, []string{"sec+alerts@mail.example.com"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkAlertChannelRecipients(json.RawMessage(tc.raw))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"empty", `[]`},
		{"null", `null`},
		{"twenty-one", mustJSON(t, append(append([]string(nil), twenty...), "x@example.com"))},
		{"a string", `"ops@example.com"`},
		{"a number", `[1]`},
		{"display name", `["Ops <ops@example.com>"]`},
		{"angle brackets", `["<ops@example.com>"]`},
		{"two in one", `["a@example.com, b@example.com"]`},
		{"header injection", `["a@example.com\r\nBcc: x@example.com"]`},
		{"line feed", `["a@example.com\nb@example.com"]`},
		{"no domain", `["ops"]`},
		{"empty entry", `[""]`},
		{"comment", `["ops@example.com (ops)"]`},
		{"quoted local part", `["\"a,b\"@example.com"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := checkAlertChannelRecipients(json.RawMessage(tc.raw))
			var requestErr *AlertChannelRequestError
			require.ErrorAs(t, err, &requestErr)
			assert.Contains(t, requestErr.Reason, "config.recipients")
		})
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func TestAlertChannelCreate_EmailConfig(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	s := newTestAlertChannelServiceWithEmail(newFakeAlertChannelStore(), newFakeEmailService())

	channel, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr("mail"), Kind: strPtr("email"), MinSeverity: strPtr("info"),
		Config: json.RawMessage(`{"recipients":["ops@example.com"," sec@example.com"]}`),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"ops@example.com", "sec@example.com"}, channel.Recipients)
	assert.Empty(t, channel.Config)
	assert.Equal(t, map[string]interface{}{"recipients": []string{"ops@example.com", "sec@example.com"}}, channel.View().Config,
		"recipients are not credentials and are answered unmasked")

	for _, tc := range []struct {
		name, config, reason string
	}{
		{"no recipients", `{}`, "config.recipients is required"},
		{"webhook URL", `{"recipients":["ops@example.com"],"webhookUrl":"` + teamsURL() + `"}`, "unknown field: config.webhookUrl"},
		{"injection", `{"recipients":["a@example.com\r\nBcc: x@example.com"]}`, "config.recipients"},
		{"unknown", `{"recipients":["ops@example.com"],"cc":["x@example.com"]}`, "unknown field: config.cc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
				Name: strPtr("x"), Kind: strPtr("email"), MinSeverity: strPtr("info"), Config: json.RawMessage(tc.config),
			})
			var requestErr *AlertChannelRequestError
			require.ErrorAs(t, err, &requestErr)
			assert.Contains(t, requestErr.Reason, tc.reason)
		})
	}

	// Recipients on a Slack channel are an unknown field.
	_, err = s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr("x"), Kind: strPtr("slack"), MinSeverity: strPtr("info"),
		Config: json.RawMessage(`{"webhookUrl":"` + slackURL() + `","recipients":["ops@example.com"]}`),
	})
	var requestErr *AlertChannelRequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, "unknown field: config.recipients", requestErr.Reason)

	// A kind change from email to Slack replaces the recipients with the URL.
	url := slackURL()
	updated, err := s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{
		Kind: strPtr("slack"), Config: json.RawMessage(`{"webhookUrl":"` + url + `"}`),
	})
	require.NoError(t, err)
	assert.Nil(t, updated.Recipients)
	assert.Equal(t, map[string]string{"webhookUrl": url}, updated.Config)
}

// The subject is "AIM alert (<severity>): <title>" with CR and LF removed and
// cut to 200 characters, so a title cannot add a header.
func TestAlertEmailSubject_IsOneHeaderLine(t *testing.T) {
	alert := &domain.Alert{Severity: domain.AlertSeverityWarning, Title: "breach\r\nBcc: x@example.com"}
	subject := AlertEmailSubject(alert)
	assert.Equal(t, "AIM alert (warning): breachBcc: x@example.com", subject)
	assert.NotContains(t, subject, "\r")
	assert.NotContains(t, subject, "\n")

	alert.Title = strings.Repeat("é", 300)
	assert.Equal(t, 200, len([]rune(AlertEmailSubject(alert))))
	assert.True(t, strings.HasPrefix(AlertEmailSubject(alert), "AIM alert (warning): é"))

	assert.Equal(t, "AIM test message for alert channel opsBcc: x@example.com", TestEmailSubject("ops\r\nBcc: x@example.com"))

	// Through the server's SMTP email service, the message has one Subject
	// header and no Bcc header.
	sink, err := smtptest.NewServer()
	require.NoError(t, err)
	defer sink.Close()
	mailer, err := email.NewSMTPEmailService(domain.EmailConfig{
		FromAddress: "aim@example.com",
		SMTP:        domain.SMTPConfig{Host: sink.Host(), Port: sink.Port()},
	})
	require.NoError(t, err)
	org := uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelEmail,
		Config: map[string]string{}, Recipients: []string{"ops@example.com"}, MinSeverity: "info", IsActive: true}
	s := newTestAlertChannelServiceWithEmail(newFakeAlertChannelStore(channel), mailer)
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "warning",
		Title: "breach\r\nBcc: x@example.com", Description: "d"})

	txs := sink.Transactions()
	require.Len(t, txs, 1)
	assert.Equal(t, []string{"ops@example.com"}, txs[0].To)
	msg, err := mail.ReadMessage(strings.NewReader(txs[0].Data))
	require.NoError(t, err)
	assert.Equal(t, []string{"AIM alert (warning): breachBcc: x@example.com"}, msg.Header["Subject"])
	assert.NotContains(t, msg.Header, "Bcc")
}

// The body holds the title, description, agent, alert id, alert type and the
// dashboard's alerts page, as plain text.
func TestAlertEmailBody(t *testing.T) {
	org := uuid.New()
	mailer := newFakeEmailService()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelEmail,
		Config: map[string]string{}, Recipients: []string{"ops@example.com"}, MinSeverity: "info", IsActive: true}
	s := newTestAlertChannelServiceWithEmail(newFakeAlertChannelStore(channel), mailer)
	alert := &domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "high", AlertType: domain.AlertSecurityBreach,
		Title: "Capability Violation Detected: billing", Description: "Agent 'billing' attempted 'db:write'", AgentName: "billing <x>"}
	s.deliverAlert(alert)

	sent := mailer.messages()
	require.Len(t, sent, 1)
	assert.False(t, sent[0].isHTML, "the body is plain text")
	assert.Equal(t, "AIM alert (high): Capability Violation Detected: billing", sent[0].subject)
	assert.Equal(t, "AIM alert, severity high: Capability Violation Detected: billing\n\n"+
		"Agent 'billing' attempted 'db:write'\n\n"+
		"Agent: billing <x>\n"+
		"Alert ID: "+alert.ID.String()+"\n"+
		"Alert type: security_breach\n\n"+
		"https://aim.example.test/dashboard/admin/alerts\n", sent[0].body)
}

// Each recipient is sent one message; a failed recipient is tried up to four
// times in all and does not stop the others. The delivery is failed, with no
// code, unless every recipient was sent the message.
func TestAlertChannelDelivery_EmailRetriesEachRecipient(t *testing.T) {
	org := uuid.New()
	mailer := newFakeEmailService()
	mailer.failFirst["flaky@example.com"] = 2
	mailer.failFirst["gone@example.com"] = -1
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelEmail, Config: map[string]string{},
		Recipients: []string{"gone@example.com", "flaky@example.com", "ok@example.com"}, MinSeverity: "info", IsActive: true}
	store := newFakeAlertChannelStore(channel)
	s := newTestAlertChannelServiceWithEmail(store, mailer)

	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(os.Stderr)

	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "info", Title: "t"})

	assert.Equal(t, 4, mailer.attempts["gone@example.com"], "a recipient that always fails is tried four times")
	assert.Equal(t, 3, mailer.attempts["flaky@example.com"], "a recipient that fails twice is sent on the third try")
	assert.Equal(t, 1, mailer.attempts["ok@example.com"], "a failed recipient does not stop the others")
	var to []string
	for _, m := range mailer.messages() {
		to = append(to, m.to)
	}
	assert.Equal(t, []string{"flaky@example.com", "ok@example.com"}, to)
	assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "failed", code: nil}}, store.recorded())
	assert.Contains(t, logged.String(), "1 of 3 recipients were not sent the message")
	assert.NotContains(t, logged.String(), "gone@example.com", "a log line carries no address")

	// When every recipient is sent the message, the delivery is delivered.
	mailer.failFirst = map[string]int{}
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "info", Title: "t"})
	assert.Equal(t, recordedDelivery{orgID: org, id: channel.ID, status: "delivered", code: nil}, store.recorded()[1])
}

// With no email service: an email channel cannot be created or set up, an
// existing one's deliveries fail as email_unavailable, and its test message
// is answered with that class.
func TestAlertChannel_NoEmailService(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "mail", Kind: domain.AlertChannelEmail,
		Config: map[string]string{}, Recipients: []string{"ops@example.com"}, MinSeverity: "info", IsActive: true}
	slackChannel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "slack", Kind: domain.AlertChannelSlack,
		Config: map[string]string{"webhookUrl": slackURL()}, MinSeverity: "critical", IsActive: true}
	store := newFakeAlertChannelStore(channel, slackChannel)
	s := newTestAlertChannelService(store)
	require.Nil(t, s.email)

	// Create.
	_, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr("new"), Kind: strPtr("email"), MinSeverity: strPtr("info"),
		Config: json.RawMessage(`{"recipients":["ops@example.com"]}`),
	})
	var requestErr *AlertChannelRequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, "email is not configured on this server", requestErr.Reason)

	// Update to an email channel, and new recipients for one.
	_, err = s.UpdateAlertChannel(context.Background(), org, slackChannel.ID, &AlertChannelRequest{
		Kind: strPtr("email"), Config: json.RawMessage(`{"recipients":["ops@example.com"]}`),
	})
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, "email is not configured on this server", requestErr.Reason)
	_, err = s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{
		Config: json.RawMessage(`{"recipients":["sec@example.com"]}`),
	})
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, "email is not configured on this server", requestErr.Reason)

	// Delivery.
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "info", Title: "t"})
	assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "failed", code: nil}}, store.recorded())

	// Test message.
	result, err := s.SendTestMessage(context.Background(), org, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, &AlertChannelTestResult{Delivered: false, StatusCode: nil, Error: "email_unavailable"}, result)

	// Deactivating the channel needs no email service.
	updated, err := s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{IsActive: new(bool)})
	require.NoError(t, err)
	assert.False(t, updated.IsActive)
}

// A test message through an email channel is sent to each recipient once,
// with no retry.
func TestAlertChannelSendTestMessage_Email(t *testing.T) {
	org := uuid.New()
	mailer := newFakeEmailService()
	mailer.failFirst["sec@example.com"] = 1
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "mail", Kind: domain.AlertChannelEmail,
		Config: map[string]string{}, Recipients: []string{"ops@example.com", "sec@example.com"}, MinSeverity: "warning"}
	store := newFakeAlertChannelStore(channel)
	s := newTestAlertChannelServiceWithEmail(store, mailer)

	result, err := s.SendTestMessage(context.Background(), org, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, &AlertChannelTestResult{Delivered: false, StatusCode: nil, Error: "email_failed"}, result)
	assert.Equal(t, 1, mailer.attempts["sec@example.com"], "a test message is not retried")

	result, err = s.SendTestMessage(context.Background(), org, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, &AlertChannelTestResult{Delivered: true}, result)
	sent := mailer.messages()
	require.Len(t, sent, 3)
	for _, m := range sent {
		assert.Equal(t, "AIM test message for alert channel mail", m.subject)
		assert.False(t, m.isHTML)
	}
	assert.Equal(t, []recordedDelivery{
		{orgID: org, id: channel.ID, status: "failed", code: nil},
		{orgID: org, id: channel.ID, status: "delivered", code: nil},
	}, store.recorded())
}

// The config column holds the recipients as a JSON array next to the config
// strings, and reads back the same.
func TestAlertChannelConfigJSON_RoundTrip(t *testing.T) {
	mailChannel := &domain.AlertChannel{Config: map[string]string{}, Recipients: []string{"ops@example.com", "sec@example.com"}}
	raw, err := mailChannel.ConfigJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"recipients":["ops@example.com","sec@example.com"]}`, string(raw))
	var back domain.AlertChannel
	require.NoError(t, back.SetConfigJSON(raw))
	assert.Equal(t, mailChannel.Recipients, back.Recipients)
	assert.Empty(t, back.Config)

	slack := &domain.AlertChannel{Config: map[string]string{"webhookUrl": "u"}}
	raw, err = slack.ConfigJSON()
	require.NoError(t, err)
	assert.Equal(t, `{"webhookUrl":"u"}`, string(raw))
	back = domain.AlertChannel{}
	require.NoError(t, back.SetConfigJSON(raw))
	assert.Equal(t, map[string]string{"webhookUrl": "u"}, back.Config)
	assert.Nil(t, back.Recipients)

	assert.Error(t, back.SetConfigJSON([]byte(`{"webhookUrl":1}`)))
	assert.Error(t, back.SetConfigJSON([]byte(`{"recipients":"ops@example.com"}`)))
}

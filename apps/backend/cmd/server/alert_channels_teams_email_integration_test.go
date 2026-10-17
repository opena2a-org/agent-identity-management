//go:build integration

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email/smtptest"
)

// teamsRequest is one request the Teams receiver accepted.
type teamsRequest struct {
	path          string
	authorization bool
	body          map[string]interface{}
}

// teamsReceiver answers as the Teams Workflows trigger "When a Teams webhook
// request is received" is documented to (learn.microsoft.com/en-us/connectors/teams/,
// and "Send Adaptive Cards using an Incoming Webhook" on the Teams platform
// pages): the request is a POST only, so anything else is answered 405; the
// body is {"type":"message","attachments":[{"contentType":
// "application/vnd.microsoft.card.adaptive","contentUrl":null,"content":<card>}]}
// with type, contentType, contentUrl and content required; and with the
// "Anyone" option an Authorization header makes the POST fail. A body that
// breaks any of these, or whose card is not an AdaptiveCard with a body array,
// is answered 400. The success status is not documented; this receiver
// answers 202, so the sender must take any 2xx.
type teamsReceiver struct {
	*httptest.Server
	mu       sync.Mutex
	accepted []teamsRequest
	refused  int
}

func newTeamsReceiver(t *testing.T) *teamsReceiver {
	t.Helper()
	r := &teamsReceiver{}
	r.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(req.Body)
		_, hasAuthorization := req.Header["Authorization"]
		var body map[string]interface{}
		if err != nil || hasAuthorization || !validTeamsMessage(raw) || json.Unmarshal(raw, &body) != nil {
			r.mu.Lock()
			r.refused++
			r.mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.accepted = append(r.accepted, teamsRequest{path: req.URL.Path, authorization: hasAuthorization, body: body})
		r.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(r.Close)
	return r
}

func validTeamsMessage(raw []byte) bool {
	var message struct {
		Type        string                       `json:"type"`
		Attachments []map[string]json.RawMessage `json:"attachments"`
	}
	if err := json.Unmarshal(raw, &message); err != nil || message.Type != "message" || len(message.Attachments) == 0 {
		return false
	}
	for _, attachment := range message.Attachments {
		contentType, hasType := attachment["contentType"]
		_, hasURL := attachment["contentUrl"]
		content, hasContent := attachment["content"]
		if !hasType || !hasURL || !hasContent {
			return false
		}
		var kind string
		if json.Unmarshal(contentType, &kind) != nil || kind != "application/vnd.microsoft.card.adaptive" {
			return false
		}
		var card struct {
			Type string            `json:"type"`
			Body []json.RawMessage `json:"body"`
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(content, &card) != nil || json.Unmarshal(content, &fields) != nil || card.Type != "AdaptiveCard" {
			return false
		}
		if body, ok := fields["body"]; !ok || !strings.HasPrefix(strings.TrimSpace(string(body)), "[") {
			return false
		}
	}
	return true
}

func (r *teamsReceiver) requests() ([]teamsRequest, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]teamsRequest(nil), r.accepted...), r.refused
}

// textBlocks returns the text of each TextBlock in an accepted body's first
// card.
func textBlocks(t *testing.T, body map[string]interface{}) []string {
	t.Helper()
	attachments, _ := body["attachments"].([]interface{})
	require.NotEmpty(t, attachments, "%v", body)
	attachment, _ := attachments[0].(map[string]interface{})
	content, _ := attachment["content"].(map[string]interface{})
	blocks, _ := content["body"].([]interface{})
	var texts []string
	for _, b := range blocks {
		block, _ := b.(map[string]interface{})
		if block["type"] == "TextBlock" {
			text, _ := block["text"].(string)
			texts = append(texts, text)
		}
	}
	return texts
}

// sinkMessage is one transaction the SMTP sink recorded, its data parsed.
type sinkMessage struct {
	to      []string
	subject string
	body    string
}

func sinkMessages(t *testing.T, sink *smtptest.Server) []sinkMessage {
	t.Helper()
	var out []sinkMessage
	for _, tx := range sink.Transactions() {
		msg, err := mail.ReadMessage(strings.NewReader(tx.Data))
		require.NoError(t, err, "the message data parses as a message")
		raw, err := io.ReadAll(msg.Body)
		require.NoError(t, err)
		out = append(out, sinkMessage{to: tx.To, subject: msg.Header.Get("Subject"), body: string(raw)})
	}
	return out
}

// An organization's alerts reach a Microsoft Teams workflow and email
// addresses, through the server's route table against a migrated database.
// Three channels are created through the API: Teams with the floor warning,
// email A with the floor info and two recipients, and email B with the floor
// critical. One warning alert, raised by an agent's verification, reaches the
// Teams workflow once, in the documented message format and with no
// Authorization header, and each of email A's recipients once through the
// server's SMTP email service, and does not reach email B. The request
// checks, the masking and the test message are driven through the same routes.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAlertChannelsTeamsEmailRoundTrip ./cmd/server/
func TestAlertChannelsTeamsEmailRoundTrip(t *testing.T) {
	sink, err := smtptest.NewServer()
	require.NoError(t, err)
	t.Cleanup(sink.Close)
	mailer, err := email.NewSMTPEmailService(domain.EmailConfig{
		Provider:    "smtp",
		FromAddress: "aim-alerts@example.com",
		FromName:    "AIM",
		SMTP:        domain.SMTPConfig{Host: sink.Host(), Port: sink.Port(), TLSEnabled: false},
	})
	require.NoError(t, err)

	const frontendURL = "https://aim.example.test"
	r := newAlertChannelsRig(t, frontendURL, mailer)
	teams := newTeamsReceiver(t)
	r.channels.SetDeliveryClientForIntegrationTest(dialByHost(map[string]string{
		"example.com:443": teams.Listener.Addr().String(),
	}, teams.Certificate()))

	// The workflow URL, built from fragments at run time.
	teamsPath := "/teams-workflow/" + "aim-test-" + uuid.NewString()
	teamsURL := "https://" + "example.com" + teamsPath

	// Requests the server refuses, each naming its field.
	for _, refusal := range []struct {
		body   map[string]interface{}
		reason string
	}{
		{map[string]interface{}{"name": "x", "kind": "email", "minSeverity": "info",
			"config": map[string]interface{}{"recipients": []string{"a@example.com\r\nBcc: x@example.com"}}}, "config.recipients"},
		{map[string]interface{}{"name": "x", "kind": "teams", "minSeverity": "info",
			"config": map[string]string{"webhookUrl": "http://example.com/x"}}, "config.webhookUrl"},
	} {
		status, body := r.do(fiber.MethodPost, "/api/v1/alert-channels", r.managerToken, refusal.body)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		var answer map[string]string
		require.NoError(t, json.Unmarshal([]byte(body), &answer), body)
		assert.Contains(t, answer["error"], refusal.reason)
	}

	teamsChannel := r.create("teams", "teams", map[string]string{"webhookUrl": teamsURL}, "warning")
	emailA := r.create("email-a", "email", map[string]interface{}{"recipients": []string{"ops@example.com", "sec@example.com"}}, "info")
	emailB := r.create("email-b", "email", map[string]interface{}{"recipients": []string{"cso@example.com"}}, "critical")

	// GET masks the Teams URL and shows the recipients.
	status, body, view := r.get(teamsChannel.ID)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, map[string]interface{}{"webhookUrl": "****" + teamsURL[len(teamsURL)-4:]}, view.Config)
	assert.NotContains(t, body, teamsPath)
	status, body, view = r.get(emailA.ID)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, map[string]interface{}{"recipients": []interface{}{"ops@example.com", "sec@example.com"}}, view.Config)

	alertID, alertTitle := r.raiseWarningAlert()

	// Wait for both deliveries to be recorded.
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, _, tv := r.get(teamsChannel.ID)
		_, _, ev := r.get(emailA.ID)
		if tv.LastDeliveryStatus != nil && ev.LastDeliveryStatus != nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "the alert's deliveries were not recorded")
		time.Sleep(50 * time.Millisecond)
	}

	// Teams: one accepted request, with no Authorization header, whose card
	// carries the alert's title.
	accepted, refused := teams.requests()
	assert.Zero(t, refused, "the Teams receiver refused a request")
	require.Len(t, accepted, 1, "the Teams workflow gets the warning alert once")
	assert.Equal(t, teamsPath, accepted[0].path)
	assert.False(t, accepted[0].authorization)
	texts := textBlocks(t, accepted[0].body)
	require.NotEmpty(t, texts)
	assert.Contains(t, strings.Join(texts, "\n"), alertTitle)
	assert.Contains(t, texts, "Agent: "+r.agentDisplay, "a TextBlock's text is the alert's string as written, without Slack's escaping")
	assert.Contains(t, texts, frontendURL+"/dashboard/admin/alerts")

	// Email: one message to each of email A's recipients, and none to email B's.
	messages := sinkMessages(t, sink)
	require.Len(t, messages, 2, "email A's two recipients each get one message")
	var recipients []string
	for _, m := range messages {
		require.Len(t, m.to, 1, "one recipient per message")
		recipients = append(recipients, m.to[0])
		assert.Contains(t, m.subject, alertTitle)
		assert.Equal(t, "AIM alert (warning): "+alertTitle, m.subject)
		assert.Contains(t, m.body, alertID.String())
		assert.Contains(t, m.body, frontendURL+"/dashboard/admin/alerts")
	}
	sort.Strings(recipients)
	assert.Equal(t, []string{"ops@example.com", "sec@example.com"}, recipients)
	for _, tx := range sink.Transactions() {
		assert.NotContains(t, tx.To, "cso@example.com", "email B's floor is above the alert's severity")
		assert.Equal(t, "aim-alerts@example.com", tx.From)
	}

	// Each channel's last delivery.
	_, _, tv := r.get(teamsChannel.ID)
	_, _, ev := r.get(emailA.ID)
	_, _, bv := r.get(emailB.ID)
	require.NotNil(t, tv.LastDeliveryStatus)
	require.NotNil(t, ev.LastDeliveryStatus)
	assert.Equal(t, "delivered", *tv.LastDeliveryStatus)
	assert.Equal(t, "delivered", *ev.LastDeliveryStatus)
	require.NotNil(t, tv.LastDeliveryCode)
	assert.Equal(t, http.StatusAccepted, *tv.LastDeliveryCode)
	assert.Nil(t, ev.LastDeliveryCode, "an email delivery has no HTTP status")
	assert.Nil(t, bv.LastDeliveryStatus)

	// A test message through email A goes to each recipient once.
	status, body = r.do(fiber.MethodPost, "/api/v1/alert-channels/"+emailA.ID.String()+"/test", r.managerToken, nil)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.JSONEq(t, `{"delivered": true, "statusCode": null}`, body)
	messages = sinkMessages(t, sink)
	require.Len(t, messages, 4)
	recipients = nil
	for _, m := range messages[2:] {
		require.Len(t, m.to, 1)
		recipients = append(recipients, m.to[0])
		assert.Equal(t, "AIM test message for alert channel email-a", m.subject)
	}
	sort.Strings(recipients)
	assert.Equal(t, []string{"ops@example.com", "sec@example.com"}, recipients)

	// A test message through Teams.
	status, body = r.do(fiber.MethodPost, "/api/v1/alert-channels/"+teamsChannel.ID.String()+"/test", r.managerToken, nil)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.JSONEq(t, `{"delivered": true, "statusCode": 202}`, body)
	accepted, _ = teams.requests()
	require.Len(t, accepted, 2)
	assert.Equal(t, []string{"AIM test message for alert channel teams"}, textBlocks(t, accepted[1].body))
}

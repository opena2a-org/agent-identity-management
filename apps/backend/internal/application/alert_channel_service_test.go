package application

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// fakeAlertChannelStore keeps channels in memory and records each delivery.
type fakeAlertChannelStore struct {
	mu         sync.Mutex
	channels   map[uuid.UUID]*domain.AlertChannel
	floorsAsk  [][]domain.AlertSeverity
	deliveries []recordedDelivery
}

type recordedDelivery struct {
	orgID, id uuid.UUID
	status    string
	code      *int
}

func newFakeAlertChannelStore(channels ...*domain.AlertChannel) *fakeAlertChannelStore {
	s := &fakeAlertChannelStore{channels: map[uuid.UUID]*domain.AlertChannel{}}
	for _, c := range channels {
		s.channels[c.ID] = c
	}
	return s
}

func (s *fakeAlertChannelStore) Create(_ context.Context, c *domain.AlertChannel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[c.ID] = c
	return nil
}

func (s *fakeAlertChannelStore) GetByID(_ context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[id]
	if !ok || c.OrganizationID != orgID {
		return nil, domain.ErrAlertChannelNotFound
	}
	copied := *c
	return &copied, nil
}

func (s *fakeAlertChannelStore) List(_ context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.AlertChannel
	for _, c := range s.channels {
		if c.OrganizationID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *fakeAlertChannelStore) ListActiveForFloors(_ context.Context, orgID uuid.UUID, floors []domain.AlertSeverity) ([]*domain.AlertChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.floorsAsk = append(s.floorsAsk, floors)
	var out []*domain.AlertChannel
	for _, c := range s.channels {
		if c.OrganizationID != orgID || !c.IsActive {
			continue
		}
		for _, f := range floors {
			if c.MinSeverity == f {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

func (s *fakeAlertChannelStore) Update(_ context.Context, c *domain.AlertChannel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.channels[c.ID]; !ok || old.OrganizationID != c.OrganizationID {
		return domain.ErrAlertChannelNotFound
	}
	s.channels[c.ID] = c
	return nil
}

func (s *fakeAlertChannelStore) Delete(_ context.Context, orgID, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.channels[id]; !ok || c.OrganizationID != orgID {
		return domain.ErrAlertChannelNotFound
	}
	delete(s.channels, id)
	return nil
}

func (s *fakeAlertChannelStore) RecordDelivery(_ context.Context, orgID, id uuid.UUID, status string, code *int, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, recordedDelivery{orgID: orgID, id: id, status: status, code: code})
	return nil
}

func (s *fakeAlertChannelStore) recorded() []recordedDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedDelivery(nil), s.deliveries...)
}

// newTestAlertChannelService returns a service on store that accepts every
// Slack URL without a lookup and does not wait between attempts.
func newTestAlertChannelService(store AlertChannelStore) *AlertChannelService {
	s := NewAlertChannelService(store, "https://aim.example.test")
	s.checkURL = func(string) error { return nil }
	s.retryDelays = []time.Duration{0, 0, 0}
	return s
}

// receiverClient returns a client builder whose every connection goes to
// srv, trusting its certificate.
func receiverClient(srv *httptest.Server) func(time.Duration) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, network, srv.Listener.Addr().String())
				},
				// httptest's certificate is issued for example.com.
				TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "example.com", MinVersion: tls.VersionTLS12},
			},
		}
	}
}

// slackURL is a Slack incoming webhook URL built from fragments.
func slackURL() string {
	return "https://" + "hooks.slack.com" + "/services/" + "unit-" + uuid.NewString()
}

func routingKey() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func strPtr(s string) *string { return &s }

// The floor admits an alert whose severity ranks at or above it: info <
// warning < high < critical. All sixteen pairs.
func TestAlertChannelFloor_AllSixteenPairs(t *testing.T) {
	order := []domain.AlertSeverity{"info", "warning", "high", "critical"}
	for fi, floor := range order {
		for si, severity := range order {
			assert.Equal(t, si >= fi, domain.AlertChannelAdmits(floor, severity), "floor %s, alert %s", floor, severity)
		}
	}
	assert.False(t, domain.AlertChannelAdmits("info", "medium"), "a severity that is not one of the four reaches no channel")
	assert.False(t, domain.AlertChannelAdmits("medium", "critical"))

	assert.Equal(t, []domain.AlertSeverity{"info"}, domain.AlertChannelFloorsAdmitting("info"))
	assert.Equal(t, []domain.AlertSeverity{"info", "warning"}, domain.AlertChannelFloorsAdmitting("warning"))
	assert.Equal(t, []domain.AlertSeverity{"info", "warning", "high"}, domain.AlertChannelFloorsAdmitting("high"))
	assert.Equal(t, order, domain.AlertChannelFloorsAdmitting("critical"))
	assert.Empty(t, domain.AlertChannelFloorsAdmitting("medium"))
}

func TestPagerDutySeverity_MapsEachAIMSeverity(t *testing.T) {
	for aim, want := range map[domain.AlertSeverity]string{
		"critical": "critical",
		"high":     "error",
		"warning":  "warning",
		"info":     "info",
	} {
		got, ok := PagerDutySeverity(aim)
		assert.True(t, ok, aim)
		assert.Equal(t, want, got, aim)
	}
	_, ok := PagerDutySeverity("medium")
	assert.False(t, ok)
}

func TestPagerDutyAlertEvent_CarriesThePublishedFields(t *testing.T) {
	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityHigh,
		Title:          strings.Repeat("é", 1500),
		Description:    "the description",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		CreatedAt:      time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC),
	}
	key := routingKey()

	event, err := PagerDutyAlertEvent(alert, key, "aim.example.test")
	require.NoError(t, err)
	assert.Equal(t, key, event.RoutingKey)
	assert.Equal(t, "trigger", event.EventAction)
	assert.Equal(t, alert.ID.String(), event.DedupKey)
	assert.Equal(t, strings.Repeat("é", 1024), event.Payload.Summary, "the summary is cut at 1024 characters")
	assert.Equal(t, "aim.example.test", event.Payload.Source, "an alert that names no agent gives the dashboard's host")
	assert.Equal(t, "error", event.Payload.Severity)
	assert.Equal(t, "2026-10-16T12:30:00Z", event.Payload.Timestamp)
	assert.Equal(t, "security_breach", event.Payload.Class)
	assert.Equal(t, map[string]interface{}{
		"alertId":        alert.ID.String(),
		"organizationId": alert.OrganizationID.String(),
		"alertType":      "security_breach",
		"severity":       "high",
		"description":    "the description",
		"resourceType":   "agent",
		"resourceId":     alert.ResourceID.String(),
	}, event.Payload.CustomDetails)

	alert.Title = "short"
	alert.AgentName = "billing-agent"
	event, err = PagerDutyAlertEvent(alert, key, "aim.example.test")
	require.NoError(t, err)
	assert.Equal(t, "short", event.Payload.Summary, "a title under the limit is not cut")
	assert.Equal(t, "billing-agent", event.Payload.Source, "the source is the alert's agent when it names one")
}

func TestAlertChannelView_MasksEveryConfigValue(t *testing.T) {
	url := slackURL()
	key := routingKey()
	channel := &domain.AlertChannel{ID: uuid.New(), Config: map[string]string{"webhookUrl": url, "routingKey": key}}

	view := channel.View()
	assert.Equal(t, "****"+url[len(url)-4:], view.Config["webhookUrl"])
	assert.Equal(t, "****"+key[len(key)-4:], view.Config["routingKey"])
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), url)
	assert.NotContains(t, string(raw), key)

	full, err := json.Marshal(channel)
	require.NoError(t, err)
	assert.NotContains(t, string(full), url, "the channel itself never serializes its config")
	assert.NotContains(t, string(full), key)

	assert.Equal(t, "****", domain.MaskAlertChannelSecret("abcd"))
	assert.Equal(t, "****bcde", domain.MaskAlertChannelSecret("abcde"))
}

func TestSlackAlertText_EscapesTheAlertsStrings(t *testing.T) {
	assert.Equal(t, "&lt;!here&gt; &amp; co", EscapeSlackText("<!here> & co"))

	alert := &domain.Alert{
		Severity:    domain.AlertSeverityCritical,
		Title:       "<!here> & co",
		Description: "a <@U123> mention",
		AgentName:   "agent <x>",
	}
	assert.Equal(t, strings.Join([]string{
		"AIM alert, severity critical: &lt;!here&gt; &amp; co",
		"a &lt;@U123&gt; mention",
		"Agent: agent &lt;x&gt;",
		"https://aim.example.test/dashboard/admin/alerts",
	}, "\n"), SlackAlertText(alert, "https://aim.example.test/dashboard/admin/alerts"))

	alert.AgentName = ""
	assert.NotContains(t, SlackAlertText(alert, "https://aim.example.test/dashboard/admin/alerts"), "Agent:",
		"the agent line is there only when the alert names one")
}

// The Slack body on the wire carries text alone, escaped as written.
func TestAlertChannelDelivery_SlackBodyOnTheWire(t *testing.T) {
	var got []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		got, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	org := uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelSlack,
		Config: map[string]string{"webhookUrl": slackURL()}, MinSeverity: "info", IsActive: true}
	store := newFakeAlertChannelStore(channel)
	s := newTestAlertChannelService(store)
	s.newHTTPClient = receiverClient(srv)

	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "warning", Title: "<!here> & co", Description: "d"})

	assert.Contains(t, string(got), `"text":"AIM alert, severity warning: &lt;!here&gt; &amp; co\nd\nhttps://aim.example.test/dashboard/admin/alerts"`)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &body))
	assert.Len(t, body, 1, "the body carries text and nothing else: %s", got)
	assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "delivered", code: intPtr(200)}}, store.recorded())
}

func intPtr(i int) *int { return &i }

func TestJudgeAlertChannelAttempt(t *testing.T) {
	for _, code := range []int{200, 202} {
		assert.Equal(t, attemptDelivered, judgeAlertChannelAttempt(code, nil), code)
	}
	for _, code := range []int{400, 401, 403, 404} {
		assert.Equal(t, attemptFailed, judgeAlertChannelAttempt(code, nil), code)
	}
	for _, code := range []int{429, 500, 503} {
		assert.Equal(t, attemptRetry, judgeAlertChannelAttempt(code, nil), code)
	}
	assert.Equal(t, attemptRetry, judgeAlertChannelAttempt(0, &net.OpError{Op: "dial", Err: errors.New("connection refused")}))
	assert.Equal(t, attemptFailed, judgeAlertChannelAttempt(0, &net.OpError{Op: "dial", Err: &utils.EgressRefusedError{Class: "loopback"}}),
		"an address the egress policy refuses is refused again")

	assert.Equal(t, "status 404", alertChannelFailureClass(404, nil))
	assert.Equal(t, "refused address", alertChannelFailureClass(0, &utils.EgressRefusedError{Class: "loopback"}))
	assert.Equal(t, "timeout", alertChannelFailureClass(0, context.DeadlineExceeded))
	assert.Equal(t, "unreachable", alertChannelFailureClass(0, errors.New("connection refused")))
}

// A delivery the receiver keeps answering with a status worth a retry makes
// four attempts and records the last status; a status that is not retried
// makes one.
func TestAlertChannelDelivery_RetriesAtMostThreeTimes(t *testing.T) {
	for _, tc := range []struct {
		status   int
		attempts int32
	}{
		{http.StatusInternalServerError, 4},
		{http.StatusServiceUnavailable, 4},
		{http.StatusTooManyRequests, 4},
		{http.StatusBadRequest, 1},
		{http.StatusForbidden, 1},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			var attempts atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			org := uuid.New()
			channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelPagerDuty,
				Config: map[string]string{"routingKey": routingKey()}, MinSeverity: "info", IsActive: true}
			store := newFakeAlertChannelStore(channel)
			s := newTestAlertChannelService(store)
			s.newHTTPClient = receiverClient(srv)

			s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "info", Title: "t"})

			assert.Equal(t, tc.attempts, attempts.Load())
			assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "failed", code: intPtr(tc.status)}}, store.recorded())
		})
	}
}

// A receiver that cannot be reached is retried, and the record has no code.
func TestAlertChannelDelivery_NetworkErrorIsRetriedAndRecordsNoCode(t *testing.T) {
	var dials atomic.Int32
	org := uuid.New()
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Kind: domain.AlertChannelSlack,
		Config: map[string]string{"webhookUrl": slackURL()}, MinSeverity: "info", IsActive: true}
	store := newFakeAlertChannelStore(channel)
	s := newTestAlertChannelService(store)
	s.newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout, Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				dials.Add(1)
				return nil, errors.New("connection refused")
			},
		}}
	}

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: org, Severity: "info", Title: "t"})

	assert.Equal(t, int32(4), dials.Load())
	assert.Equal(t, []recordedDelivery{{orgID: org, id: channel.ID, status: "failed"}}, store.recorded())
	assert.Contains(t, logged.String(), "not delivered: unreachable")
	assert.NotContains(t, logged.String(), channel.Config["webhookUrl"], "no log line names the destination")
	assert.NotContains(t, logged.String(), "/services/")
}

// Only the organization's active channels whose floor admits the alert are
// asked for, and an alert of a severity outside the four reaches none.
func TestAlertChannelDelivery_AsksForTheFloorsTheSeverityAdmits(t *testing.T) {
	store := newFakeAlertChannelStore()
	s := newTestAlertChannelService(store)
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: uuid.New(), Severity: "warning"})
	s.deliverAlert(&domain.Alert{ID: uuid.New(), OrganizationID: uuid.New(), Severity: "medium"})
	assert.Equal(t, [][]domain.AlertSeverity{{"info", "warning"}}, store.floorsAsk)
}

// The observer never blocks: when 1024 alerts wait, the next one is dropped
// with a log line naming it, and Enqueue returns.
func TestAlertChannelEnqueue_DropsWhenTheQueueIsFull(t *testing.T) {
	s := newTestAlertChannelService(newFakeAlertChannelStore())
	for i := 0; i < AlertChannelQueueCapacity; i++ {
		s.Enqueue(&domain.Alert{ID: uuid.New(), Severity: "info"})
	}
	require.Len(t, s.queue, AlertChannelQueueCapacity)

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dropped := &domain.Alert{ID: uuid.New(), Severity: "critical"}
	done := make(chan struct{})
	go func() {
		s.Enqueue(dropped)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Enqueue blocked on a full queue")
	}
	assert.Len(t, s.queue, AlertChannelQueueCapacity)
	assert.Contains(t, logged.String(), "alert "+dropped.ID.String()+": alert channel queue full")
}

func TestAlertChannelCreate_RefusesWhatItCannotSend(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	slack := func(url string) json.RawMessage { return json.RawMessage(`{"webhookUrl":"` + url + `"}`) }
	for _, tc := range []struct {
		name   string
		req    AlertChannelRequest
		reason string
	}{
		{"no name", AlertChannelRequest{Name: strPtr(" "), Kind: strPtr("slack"), Config: slack(slackURL()), MinSeverity: strPtr("info")}, "name is required"},
		{"long name", AlertChannelRequest{Name: strPtr(strings.Repeat("n", 101)), Kind: strPtr("slack"), Config: slack(slackURL()), MinSeverity: strPtr("info")}, "name must be at most 100 characters"},
		{"unknown kind", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("sms"), Config: json.RawMessage(`{}`), MinSeverity: strPtr("info")}, "unsupported kind: sms"},
		{"unknown floor", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack(slackURL()), MinSeverity: strPtr("medium")}, "minSeverity must be one of info, warning, high, critical"},
		{"no config", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), MinSeverity: strPtr("info")}, "config is required"},
		{"null config", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: json.RawMessage(`null`), MinSeverity: strPtr("info")}, "config must be an object"},
		{"unknown config field", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: json.RawMessage(`{"webhookUrl":"` + slackURL() + `","channel":"#x"}`), MinSeverity: strPtr("info")}, "unknown field: config.channel"},
		{"pagerduty field on slack", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: json.RawMessage(`{"routingKey":"` + routingKey() + `"}`), MinSeverity: strPtr("info")}, "unknown field: config.routingKey"},
		{"other host", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack("https://example.com/x"), MinSeverity: strPtr("info")}, "config.webhookUrl must be a Slack incoming webhook URL"},
		{"http", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack("http://hooks.slack.com/services/x"), MinSeverity: strPtr("info")}, "config.webhookUrl"},
		{"no path", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack("https://hooks.slack.com/services/"), MinSeverity: strPtr("info")}, "config.webhookUrl"},
		{"query only", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack("https://hooks.slack.com/services/?a=b"), MinSeverity: strPtr("info")}, "config.webhookUrl"},
		{"other path", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"), Config: slack("https://hooks.slack.com/servicesx/y"), MinSeverity: strPtr("info")}, "config.webhookUrl"},
		{"short key", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("pagerduty"), Config: json.RawMessage(`{"routingKey":"` + routingKey()[:31] + `"}`), MinSeverity: strPtr("info")}, "config.routingKey must be the 32-character integration key"},
		{"long key", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("pagerduty"), Config: json.RawMessage(`{"routingKey":"` + routingKey() + `x"}`), MinSeverity: strPtr("info")}, "config.routingKey"},
		{"no key", AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("pagerduty"), Config: json.RawMessage(`{}`), MinSeverity: strPtr("info")}, "config.routingKey is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeAlertChannelStore()
			s := newTestAlertChannelService(store)
			_, err := s.CreateAlertChannel(context.Background(), org, user, &tc.req)
			var requestErr *AlertChannelRequestError
			require.ErrorAs(t, err, &requestErr)
			assert.Contains(t, requestErr.Reason, tc.reason)
			assert.NotContains(t, requestErr.Reason, "unit-", "a refusal never repeats the destination")
			assert.Empty(t, store.channels)
		})
	}
}

// A Slack URL the webhook URL check refuses is refused as config.webhookUrl;
// a lookup that failed for another reason is not a refusal.
func TestAlertChannelCreate_SlackURLGoesThroughTheWebhookURLCheck(t *testing.T) {
	req := AlertChannelRequest{Name: strPtr("x"), Kind: strPtr("slack"),
		Config: json.RawMessage(`{"webhookUrl":"` + slackURL() + `"}`), MinSeverity: strPtr("info")}

	s := newTestAlertChannelService(newFakeAlertChannelStore())
	s.checkURL = func(string) error { return fmt.Errorf("%w: private address", ErrWebhookURLRefused) }
	_, err := s.CreateAlertChannel(context.Background(), uuid.New(), uuid.New(), &req)
	var requestErr *AlertChannelRequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Contains(t, requestErr.Reason, "config.webhookUrl")

	lookup := errors.New("webhook URL host lookup failed")
	s.checkURL = func(string) error { return lookup }
	_, err = s.CreateAlertChannel(context.Background(), uuid.New(), uuid.New(), &req)
	assert.ErrorIs(t, err, lookup)
	assert.False(t, errors.As(err, &requestErr))
}

func TestAlertChannelCreateAndUpdate(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	store := newFakeAlertChannelStore()
	s := newTestAlertChannelService(store)
	url := slackURL()

	channel, err := s.CreateAlertChannel(context.Background(), org, user, &AlertChannelRequest{
		Name: strPtr(" ops "), Kind: strPtr("slack"), Config: json.RawMessage(`{"webhookUrl":"` + url + `"}`), MinSeverity: strPtr("high"),
	})
	require.NoError(t, err)
	assert.Equal(t, "ops", channel.Name)
	assert.True(t, channel.IsActive, "isActive defaults to true")
	assert.Equal(t, org, channel.OrganizationID)
	assert.Equal(t, &user, channel.CreatedBy)
	assert.Equal(t, map[string]string{"webhookUrl": url}, channel.Config)

	// An update without config keeps the stored destination.
	updated, err := s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{MinSeverity: strPtr("critical"), IsActive: new(bool)})
	require.NoError(t, err)
	assert.Equal(t, domain.AlertSeverityCritical, updated.MinSeverity)
	assert.False(t, updated.IsActive)
	assert.Equal(t, map[string]string{"webhookUrl": url}, updated.Config)

	// A kind change needs a config for the new kind.
	_, err = s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{Kind: strPtr("pagerduty")})
	var requestErr *AlertChannelRequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, "config is required when kind changes", requestErr.Reason)

	key := routingKey()
	updated, err = s.UpdateAlertChannel(context.Background(), org, channel.ID, &AlertChannelRequest{
		Kind: strPtr("pagerduty"), Config: json.RawMessage(`{"routingKey":"` + key + `"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"routingKey": key}, updated.Config, "a config replaces the stored one whole")

	// Another organization's id is not found.
	_, err = s.UpdateAlertChannel(context.Background(), uuid.New(), channel.ID, &AlertChannelRequest{MinSeverity: strPtr("info")})
	assert.ErrorIs(t, err, domain.ErrAlertChannelNotFound)
	_, err = s.SendTestMessage(context.Background(), uuid.New(), channel.ID)
	assert.ErrorIs(t, err, domain.ErrAlertChannelNotFound)
}

// A test message goes once, with no retry, in each kind's format, and is
// recorded.
func TestAlertChannelSendTestMessage(t *testing.T) {
	var bodies []map[string]interface{}
	var paths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		paths = append(paths, r.Host+r.URL.Path)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	org := uuid.New()
	key := routingKey()
	slackChannel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "ops <x>", Kind: domain.AlertChannelSlack,
		Config: map[string]string{"webhookUrl": slackURL()}, MinSeverity: "info"}
	pdChannel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "pager", Kind: domain.AlertChannelPagerDuty,
		Config: map[string]string{"routingKey": key}, MinSeverity: "info"}
	store := newFakeAlertChannelStore(slackChannel, pdChannel)
	s := newTestAlertChannelService(store)
	s.newHTTPClient = receiverClient(srv)

	result, err := s.SendTestMessage(context.Background(), org, slackChannel.ID)
	require.NoError(t, err)
	assert.Equal(t, &AlertChannelTestResult{Delivered: false, StatusCode: intPtr(503), Error: "status 503"}, result)
	result, err = s.SendTestMessage(context.Background(), org, pdChannel.ID)
	require.NoError(t, err)
	assert.False(t, result.Delivered)

	require.Len(t, bodies, 2, "a test message is not retried")
	assert.Equal(t, map[string]interface{}{"text": "AIM test message for alert channel ops &lt;x&gt;"}, bodies[0])
	assert.Equal(t, "events.pagerduty.com/v2/enqueue", paths[1], "a PagerDuty event goes to the Events API v2")
	assert.Equal(t, key, bodies[1]["routing_key"])
	assert.Equal(t, "trigger", bodies[1]["event_action"])
	assert.Equal(t, "aim-channel-test-"+pdChannel.ID.String(), bodies[1]["dedup_key"])
	payload := bodies[1]["payload"].(map[string]interface{})
	assert.Equal(t, "info", payload["severity"])
	assert.Equal(t, "AIM test event for alert channel pager", payload["summary"])
	assert.Equal(t, "aim.example.test", payload["source"])

	assert.Equal(t, []recordedDelivery{
		{orgID: org, id: slackChannel.ID, status: "failed", code: intPtr(503)},
		{orgID: org, id: pdChannel.ID, status: "failed", code: intPtr(503)},
	}, store.recorded())
}

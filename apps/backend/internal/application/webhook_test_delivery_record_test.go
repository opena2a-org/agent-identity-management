package application

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin how a test delivery (POST /webhooks/:id/test) is
// sent and recorded: with the headers of an event delivery, as a delivery
// record that carries the webhook's organization and a status, and as a failed
// record when the endpoint gives no answer.

// testDeliveryEndpoint answers each request with status and a 4 KB body and
// keeps the headers of the last request it received.
func testDeliveryEndpoint(t *testing.T, status int) (*httptest.Server, func() http.Header) {
	t.Helper()
	var (
		mu   sync.Mutex
		last http.Header
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		last = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 4096))
	}))
	t.Cleanup(srv.Close)
	return srv, func() http.Header {
		mu.Lock()
		defer mu.Unlock()
		return last
	}
}

// newTestDeliveryService returns a service whose deliveries go through a plain
// client, since the egress client refuses the loopback address of a test
// endpoint (TestWebhookDelivery_RefusesStoredURLThatResolvesToLoopback pins
// that).
func newTestDeliveryService(t *testing.T) (*WebhookService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	svc := NewWebhookService(repository.NewWebhookRepository(db))
	svc.newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout}
	}
	return svc, mock
}

// recordedErrorContaining matches a recorded error message that contains it.
type recordedErrorContaining string

func (part recordedErrorContaining) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && strings.Contains(s, string(part))
}

// A test delivery sends the headers an event delivery sends, so a receiver
// that routes or filters on X-Webhook-ID or User-Agent treats both alike.
func TestWebhookTestDelivery_SendsTheHeadersOfAnEventDelivery(t *testing.T) {
	endpoint, lastHeaders := testDeliveryEndpoint(t, http.StatusOK)
	svc, mock := newTestDeliveryService(t)
	webhook := &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		URL:            endpoint.URL + "/hook",
		Events:         []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		Secret:         "not-a-real-secret",
	}

	_, _, _, err := svc.attemptDelivery(webhook, domain.WebhookEventAgentCreated, []byte(`{"event":"agent.created"}`), 5*time.Second)
	require.NoError(t, err)
	eventDelivery := lastHeaders()

	mock.ExpectExec("INSERT INTO webhook_deliveries").WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = svc.sendWebhookWithResult(webhook, "webhook.test", map[string]string{"event": "webhook.test"})
	require.NoError(t, err)
	testDelivery := lastHeaders()

	for _, name := range []string{"Content-Type", "User-Agent", "X-Webhook-ID"} {
		assert.Equal(t, eventDelivery.Get(name), testDelivery.Get(name), "%s differs between a test delivery and an event delivery", name)
	}
	assert.Equal(t, webhook.ID.String(), testDelivery.Get("X-Webhook-ID"))
	assert.Equal(t, "AIM-Webhook/1.0", testDelivery.Get("User-Agent"))
	assert.Equal(t, "webhook.test", testDelivery.Get("X-Webhook-Event"))
	assert.NotEmpty(t, testDelivery.Get("X-Webhook-Signature"))
}

// A test delivery is recorded with the webhook's organization (the delivery
// table refers to organizations, so a record without one is refused) and a
// status, and keeps at most 1 KB of the endpoint's answer, as a delivery
// attempt does.
func TestWebhookTestDelivery_RecordCarriesOrganizationStatusAndBoundedBody(t *testing.T) {
	for _, tc := range []struct {
		status     int
		wantStatus domain.WebhookDeliveryStatus
		wantErrMsg string
	}{
		{http.StatusOK, domain.DeliveryStatusSuccess, ""},
		{http.StatusInternalServerError, domain.DeliveryStatusFailed, "HTTP 500"},
	} {
		t.Run(string(tc.wantStatus), func(t *testing.T) {
			endpoint, _ := testDeliveryEndpoint(t, tc.status)
			svc, mock := newTestDeliveryService(t)
			webhook := &domain.Webhook{
				ID:             uuid.New(),
				OrganizationID: uuid.New(),
				URL:            endpoint.URL + "/hook",
				Secret:         "not-a-real-secret",
			}

			// The columns of WebhookRepository.RecordDelivery, in order.
			mock.ExpectExec("INSERT INTO webhook_deliveries").WithArgs(
				sqlmock.AnyArg(),           // id
				webhook.ID,                 // webhook_id
				webhook.OrganizationID,     // organization_id
				"webhook.test",             // event
				sqlmock.AnyArg(),           // payload
				string(tc.wantStatus),      // status
				tc.status,                  // status_code
				strings.Repeat("a", 1024),  // response_body
				tc.wantErrMsg,              // error_message
				tc.status == http.StatusOK, // success
				1,                          // attempt_count
				sqlmock.AnyArg(),           // duration_ms
				sqlmock.AnyArg(),           // last_attempt_at
				sqlmock.AnyArg(),           // next_retry_at
				sqlmock.AnyArg(),           // created_at
				sqlmock.AnyArg(),           // completed_at
			).WillReturnResult(sqlmock.NewResult(0, 1))

			statusCode, _ := svc.sendWebhookWithResult(webhook, "webhook.test", map[string]string{"event": "webhook.test"})
			assert.Equal(t, tc.status, statusCode)
			require.NoError(t, mock.ExpectationsWereMet(), "the test delivery record differs from the one expected")
		})
	}
}

// A test delivery whose record cannot be stored still reports the endpoint's
// answer, and the storage error is written to the server log.
func TestWebhookTestDelivery_LogsARecordItCouldNotStore(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	endpoint, _ := testDeliveryEndpoint(t, http.StatusOK)
	svc, mock := newTestDeliveryService(t)
	webhook := &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		URL:            endpoint.URL + "/hook",
		Secret:         "not-a-real-secret",
	}
	const storageDetail = "FAKE-storage-detail violates foreign key constraint"
	mock.ExpectExec("INSERT INTO webhook_deliveries").WillReturnError(errors.New(storageDetail))

	statusCode, err := svc.sendWebhookWithResult(webhook, "webhook.test", map[string]string{"event": "webhook.test"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, logged.String(), storageDetail, "the server log has no line for the record that was not stored")
}

// A test delivery that gets no answer from the endpoint is recorded as a
// failed delivery with the connection error, as an event delivery that gets
// no answer is recorded with its error. The endpoint either refuses the
// connection or has an address the egress client refuses to connect to.
func TestWebhookTestDelivery_WithoutAnAnswerIsRecordedAsFailed(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL + "/hook"
	closed.Close()

	for _, tc := range []struct {
		name string
		url  string
		// plainClient sends through a plain client, which connects to a
		// loopback address; otherwise the egress client is used.
		plainClient bool
		wantInError string
	}{
		{name: "connection refused", url: closedURL, plainClient: true, wantInError: "connection refused"},
		{name: "address refused by the egress client", url: "http://127.0.0.1:9/hook", wantInError: "destination address is not allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			svc := NewWebhookService(repository.NewWebhookRepository(db))
			if tc.plainClient {
				svc.newHTTPClient = func(timeout time.Duration) *http.Client {
					return &http.Client{Timeout: timeout}
				}
			}
			webhook := &domain.Webhook{
				ID:             uuid.New(),
				OrganizationID: uuid.New(),
				URL:            tc.url,
				Secret:         "not-a-real-secret",
			}

			// The columns of WebhookRepository.RecordDelivery, in order.
			mock.ExpectExec("INSERT INTO webhook_deliveries").WithArgs(
				sqlmock.AnyArg(),                        // id
				webhook.ID,                              // webhook_id
				webhook.OrganizationID,                  // organization_id
				"webhook.test",                          // event
				sqlmock.AnyArg(),                        // payload
				string(domain.DeliveryStatusFailed),     // status
				0,                                       // status_code
				"",                                      // response_body
				recordedErrorContaining(tc.wantInError), // error_message
				false,                                   // success
				1,                                       // attempt_count
				sqlmock.AnyArg(),                        // duration_ms
				sqlmock.AnyArg(),                        // last_attempt_at
				sqlmock.AnyArg(),                        // next_retry_at
				sqlmock.AnyArg(),                        // created_at
				sqlmock.AnyArg(),                        // completed_at
			).WillReturnResult(sqlmock.NewResult(0, 1))

			statusCode, deliveryErr := svc.sendWebhookWithResult(webhook, "webhook.test", map[string]string{"event": "webhook.test"})
			if deliveryErr == nil || statusCode != 0 {
				t.Errorf("status = %d, error = %v; want 0 and the connection error", statusCode, deliveryErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("the test delivery was not recorded as failed: %v", err)
			}
		})
	}
}

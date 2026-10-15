package application

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// columnLog records, in order, the values one column of the delivery insert
// is given. Deliveries run on their own goroutines, so it locks.
type columnLog struct {
	mu     sync.Mutex
	values []driver.Value
}

func (c *columnLog) Match(v driver.Value) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values, v)
	return true
}

func (c *columnLog) all() []driver.Value {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]driver.Value(nil), c.values...)
}

// String is what sqlmock prints for the argument when it reports unmet
// expectations, which it may do while a delivery is still recording.
func (c *columnLog) String() string {
	return fmt.Sprintf("columnLog%v", c.all())
}

// deliveryInserts logs the status, error message and next retry time of each
// delivery record stored. The mock must match in order, so each insert is
// matched against one expectation only.
type deliveryInserts struct {
	status, errorMessage, nextRetryAt *columnLog
}

func newDeliveryInserts() deliveryInserts {
	return deliveryInserts{&columnLog{}, &columnLog{}, &columnLog{}}
}

// expect expects one more delivery insert.
func (d deliveryInserts) expect(mock sqlmock.Sqlmock) {
	args := make([]driver.Value, 16)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[5], args[8], args[13] = d.status, d.errorMessage, d.nextRetryAt
	mock.ExpectExec("INSERT INTO webhook_deliveries").WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectStatsUpdate(mock sqlmock.Sqlmock) {
	mock.ExpectExec("UPDATE webhooks").WillReturnResult(sqlmock.NewResult(0, 1))
}

func boundsTestPayload(webhook *domain.Webhook) *domain.WebhookPayload {
	return &domain.WebhookPayload{
		ID:             uuid.NewString(),
		Event:          domain.WebhookEventAgentCreated,
		Timestamp:      time.Now().UTC(),
		OrganizationID: webhook.OrganizationID.String(),
		Data:           map[string]interface{}{"agentId": uuid.NewString()},
	}
}

func boundsTestWebhook(url string) *domain.Webhook {
	return &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		URL:            url,
		Events:         []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		Secret:         "not-a-real-secret",
		IsActive:       true,
	}
}

func failingEndpoint(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// waitRecorder stands in for the retry timer: it records each delay asked
// for, with the time it was asked, and fires at once.
type waitRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
	asked  []time.Time
}

func (w *waitRecorder) timer(d time.Duration) *time.Timer {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.delays = append(w.delays, d)
	w.asked = append(w.asked, time.Now())
	return time.NewTimer(0)
}

func TestDeliveryBounds_AdmitCapsPendingDeliveriesOverallAndPerWebhook(t *testing.T) {
	b := newDeliveryBounds(&WebhookConfig{MaxPendingDeliveries: 3, MaxPendingPerWebhook: 2})
	a, c := uuid.New(), uuid.New()

	for i := 0; i < 2; i++ {
		ok, _ := b.admit(a)
		require.True(t, ok)
	}
	ok, reason := b.admit(a)
	assert.False(t, ok, "a third delivery to one webhook is over MaxPendingPerWebhook")
	assert.Equal(t, "not sent: this webhook's pending deliveries are at their limit of 2", reason)

	ok, _ = b.admit(c)
	require.True(t, ok, "another webhook still has room")
	ok, reason = b.admit(uuid.New())
	assert.False(t, ok, "a fourth delivery is over MaxPendingDeliveries")
	assert.Equal(t, "not sent: pending event deliveries are at their limit of 3", reason)

	b.done(a)
	ok, _ = b.admit(c)
	assert.True(t, ok, "a delivery that is done gives its place back")

	b.done(a)
	b.done(c)
	b.done(c)
	b.mu.Lock()
	defer b.mu.Unlock()
	assert.Zero(t, b.pending)
	assert.Empty(t, b.pendingByWebhook, "a webhook with no pending delivery is forgotten")
	assert.Empty(t, b.webhookAttempts)
}

// The limits docs/API.md gives under "Delivery and retries".
func TestDefaultWebhookConfig_DeliveryLimitsAreTheDocumentedOnes(t *testing.T) {
	config := DefaultWebhookConfig()
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 30*time.Second, config.MaxTimeout)
	assert.Equal(t, 30*time.Minute, config.MaxRetryDelay)
	assert.Equal(t, 1000, config.MaxPendingDeliveries)
	assert.Equal(t, 100, config.MaxPendingPerWebhook)
	assert.Equal(t, 5, config.DeliveryWorkerCount)
	assert.Equal(t, 2, config.MaxAttemptsPerWebhook)
}

func TestDeliveryBounds_ZeroConfigKeepsTheDefaultBounds(t *testing.T) {
	b := newDeliveryBounds(&WebhookConfig{})
	assert.Equal(t, defaultMaxPendingDeliveries, b.maxPending)
	assert.Equal(t, defaultMaxPendingPerWebhook, b.maxPendingPerWebhook)
	assert.Equal(t, defaultMaxAttemptsPerWebhook, b.maxAttemptsPerWebhook)
	assert.Equal(t, defaultMaxAttemptsInFlight, cap(b.attempts))
}

// acquireAsync asks for an attempt slot on a goroutine and returns the channel
// its release function arrives on.
func acquireAsync(b *deliveryBounds, webhookID uuid.UUID, stop <-chan struct{}) <-chan func() {
	got := make(chan func(), 1)
	go func() {
		if release, ok := b.acquireAttempt(webhookID, stop); ok {
			got <- release
		} else {
			close(got)
		}
	}()
	return got
}

func assertWaiting(t *testing.T, got <-chan func(), msg string) {
	t.Helper()
	select {
	case <-got:
		t.Fatal(msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertGranted(t *testing.T, got <-chan func(), msg string) func() {
	t.Helper()
	select {
	case release, ok := <-got:
		require.True(t, ok, msg)
		return release
	case <-time.After(5 * time.Second):
		t.Fatal(msg)
		return nil
	}
}

// A slow endpoint holds at most MaxAttemptsPerWebhook attempt slots, so a
// delivery to another webhook is sent while it waits.
func TestDeliveryBounds_AttemptsToOneWebhookAreCapped(t *testing.T) {
	b := newDeliveryBounds(&WebhookConfig{DeliveryWorkerCount: 5, MaxAttemptsPerWebhook: 1})
	stop := make(chan struct{})
	slow, other := uuid.New(), uuid.New()

	releaseSlow, ok := b.acquireAttempt(slow, stop)
	require.True(t, ok)
	second := acquireAsync(b, slow, stop)
	assertWaiting(t, second, "a second attempt to the slow webhook waits for the first")

	releaseOther := assertGranted(t, acquireAsync(b, other, stop), "another webhook is not held up by the slow one")
	releaseOther()

	releaseSlow()
	assertGranted(t, second, "the second attempt is sent once the first ends")()
}

func TestDeliveryBounds_AttemptsInFlightAreCappedAcrossWebhooks(t *testing.T) {
	b := newDeliveryBounds(&WebhookConfig{DeliveryWorkerCount: 2, MaxAttemptsPerWebhook: 5})
	stop := make(chan struct{})

	release1, ok := b.acquireAttempt(uuid.New(), stop)
	require.True(t, ok)
	release2, ok := b.acquireAttempt(uuid.New(), stop)
	require.True(t, ok)
	third := acquireAsync(b, uuid.New(), stop)
	assertWaiting(t, third, "a third attempt waits while DeliveryWorkerCount attempts are in flight")

	release1()
	assertGranted(t, third, "the third attempt is sent once a slot is free")()
	release2()
}

func TestDeliveryBounds_StopEndsAWaitForAnAttemptSlot(t *testing.T) {
	b := newDeliveryBounds(&WebhookConfig{DeliveryWorkerCount: 1, MaxAttemptsPerWebhook: 1})
	stop := make(chan struct{})
	webhookID := uuid.New()

	release, ok := b.acquireAttempt(webhookID, stop)
	require.True(t, ok)
	waiting := acquireAsync(b, webhookID, stop)
	assertWaiting(t, waiting, "the attempt waits for the slot")

	close(stop)
	select {
	case _, granted := <-waiting:
		assert.False(t, granted, "a stopped wait is not granted a slot")
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not end the wait")
	}
	_, ok = b.acquireAttempt(uuid.New(), stop)
	assert.False(t, ok, "no slot is handed out after stop")

	release()
	assert.Zero(t, len(b.attempts), "an ended wait holds no slot")
}

// The webhook's stored retry count, retry delay and timeout are capped by
// MaxRetries, MaxRetryDelay and MaxTimeout.
func TestWebhookDelivery_RetriesDelayAndTimeoutAreCappedWhateverTheWebhookStores(t *testing.T) {
	endpoint, hits := failingEndpoint(t)
	svc, mock := newMockedWebhookService(t)
	svc.SetConfig(&WebhookConfig{
		DefaultTimeout:     30 * time.Second,
		DefaultMaxRetries:  3,
		DefaultRetryDelay:  time.Minute,
		MaxRetryDelay:      10 * time.Minute,
		RetryBackoffFactor: 2,
		MaxRetries:         2,
		MaxTimeout:         2 * time.Second,
	})
	var timeouts []time.Duration
	var timeoutsMu sync.Mutex
	svc.newHTTPClient = func(timeout time.Duration) *http.Client {
		timeoutsMu.Lock()
		timeouts = append(timeouts, timeout)
		timeoutsMu.Unlock()
		return loopbackAdmittingClient(timeout)
	}
	waits := &waitRecorder{}
	svc.newRetryTimer = waits.timer

	webhook := boundsTestWebhook(endpoint.URL)
	webhook.MaxRetries = 1000
	webhook.RetryDelaySeconds = 86400
	webhook.TimeoutSeconds = 86400

	inserts := newDeliveryInserts()
	for i := 0; i < 3; i++ {
		inserts.expect(mock)
	}
	expectStatsUpdate(mock)

	svc.deliverWithRetry(webhook, boundsTestPayload(webhook))

	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, int32(3), atomic.LoadInt32(hits), "one attempt and MaxRetries retries")
	assert.Equal(t, []time.Duration{2 * time.Second, 2 * time.Second, 2 * time.Second}, timeouts)
	assert.Equal(t, []time.Duration{10 * time.Minute, 10 * time.Minute}, waits.delays)
	assert.Equal(t, []driver.Value{
		string(domain.DeliveryStatusRetrying), string(domain.DeliveryStatusRetrying), string(domain.DeliveryStatusAbandoned),
	}, inserts.status.all())
	assert.Nil(t, inserts.nextRetryAt.all()[2], "an abandoned delivery has no next retry")
}

// A webhook created through CreateWebhook stores no retry count, retry delay
// or timeout, so its deliveries take the defaults: a 30-second wait per
// attempt and 3 retries, after 60, 120 and 240 seconds.
func TestWebhookDelivery_AWebhookCreatedThroughTheAPIGetsTheDefaultSchedule(t *testing.T) {
	svc, mock := newMockedWebhookService(t)

	stored := []*columnLog{{}, {}, {}}
	args := make([]driver.Value, 15)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[7], args[8], args[9] = stored[0], stored[1], stored[2]
	mock.ExpectExec("INSERT INTO webhooks").WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))

	webhook, err := svc.CreateWebhook(context.Background(), &CreateWebhookRequest{
		Name:   "schedule",
		URL:    "https://1.1.1.1/hook",
		Events: []domain.WebhookEvent{domain.WebhookEventAgentCreated},
	}, uuid.New(), uuid.New())
	require.NoError(t, err)
	for i, column := range []string{"timeout_seconds", "max_retries", "retry_delay_seconds"} {
		assert.Equal(t, []driver.Value{int64(0)}, stored[i].all(), "%s is stored as 0", column)
	}

	endpoint, hits := failingEndpoint(t)
	webhook.URL = endpoint.URL
	var timeouts []time.Duration
	svc.newHTTPClient = func(timeout time.Duration) *http.Client {
		timeouts = append(timeouts, timeout)
		return loopbackAdmittingClient(timeout)
	}
	waits := &waitRecorder{}
	svc.newRetryTimer = waits.timer

	inserts := newDeliveryInserts()
	for i := 0; i < 4; i++ {
		inserts.expect(mock)
	}
	expectStatsUpdate(mock)

	svc.deliverWithRetry(webhook, boundsTestPayload(webhook))

	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, int32(4), atomic.LoadInt32(hits))
	assert.Equal(t, []time.Duration{30 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}, timeouts)
	assert.Equal(t, []time.Duration{60 * time.Second, 120 * time.Second, 240 * time.Second}, waits.delays)
}

// Each wait before a retry is the delay the retrying record names in
// nextRetryAt, and the delay grows by RetryBackoffFactor after it.
func TestWebhookDelivery_RetryWaitsTheDelayItRecords(t *testing.T) {
	endpoint, hits := failingEndpoint(t)
	svc, mock := newMockedWebhookService(t)
	svc.SetConfig(&WebhookConfig{
		DefaultTimeout:     5 * time.Second,
		DefaultMaxRetries:  2,
		DefaultRetryDelay:  time.Second,
		MaxRetryDelay:      time.Hour,
		RetryBackoffFactor: 4,
	})
	svc.newHTTPClient = loopbackAdmittingClient
	waits := &waitRecorder{}
	svc.newRetryTimer = waits.timer
	webhook := boundsTestWebhook(endpoint.URL)

	inserts := newDeliveryInserts()
	for i := 0; i < 3; i++ {
		inserts.expect(mock)
	}
	expectStatsUpdate(mock)

	svc.deliverWithRetry(webhook, boundsTestPayload(webhook))

	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, int32(3), atomic.LoadInt32(hits))
	require.Equal(t, []time.Duration{time.Second, 4 * time.Second}, waits.delays)
	next := inserts.nextRetryAt.all()
	for i, d := range waits.delays {
		recorded, ok := next[i].(time.Time)
		require.True(t, ok, "retrying record %d names its next retry", i)
		assert.WithinDuration(t, waits.asked[i].Add(d), recorded, 500*time.Millisecond,
			"the wait before retry %d ends when its record says", i+1)
	}
}

// A delivery the bounds refuse is recorded as abandoned with the reason, and
// its endpoint is not called.
func TestWebhookDispatch_ADeliveryOverThePendingBoundIsRecordedAndNotSent(t *testing.T) {
	arrived := make(chan struct{}, 4)
	release := make(chan struct{})
	var hits int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		arrived <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(endpoint.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	svc, mock := newMockedWebhookService(t)
	config := DefaultWebhookConfig()
	config.MaxPendingPerWebhook = 1
	svc.SetConfig(config)
	svc.newHTTPClient = loopbackAdmittingClient
	webhook := boundsTestWebhook(endpoint.URL)

	inserts := newDeliveryInserts()
	inserts.expect(mock) // the refused delivery
	inserts.expect(mock) // the sent one
	expectStatsUpdate(mock)

	svc.dispatch(webhook, boundsTestPayload(webhook))
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the first delivery did not reach its endpoint")
	}

	svc.dispatch(webhook, boundsTestPayload(webhook))
	assert.Equal(t, []driver.Value{string(domain.DeliveryStatusAbandoned)}, inserts.status.all(),
		"the refused delivery is recorded before dispatch returns")
	assert.Equal(t, []driver.Value{"not sent: this webhook's pending deliveries are at their limit of 1"},
		inserts.errorMessage.all())

	close(release)
	assert.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, string(domain.DeliveryStatusSuccess), inserts.status.all()[1])
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits), "the refused delivery never reached the endpoint")
}

// Stop ends a delivery waiting to retry and records it as abandoned, and a
// delivery dispatched after Stop is not sent.
func TestWebhookService_StopEndsAWaitingRetryAndRefusesLaterDeliveries(t *testing.T) {
	endpoint, hits := failingEndpoint(t)
	svc, mock := newMockedWebhookService(t)
	config := DefaultWebhookConfig()
	config.DefaultRetryDelay = time.Hour
	config.MaxRetryDelay = time.Hour
	svc.SetConfig(config)
	svc.newHTTPClient = loopbackAdmittingClient
	webhook := boundsTestWebhook(endpoint.URL)

	inserts := newDeliveryInserts()
	inserts.expect(mock) // retrying after the first attempt
	inserts.expect(mock) // abandoned when Stop ends the wait
	inserts.expect(mock) // the delivery dispatched after Stop

	svc.dispatch(webhook, boundsTestPayload(webhook))
	require.Eventually(t, func() bool { return len(inserts.status.all()) == 1 }, 5*time.Second, 10*time.Millisecond,
		"the first attempt fails and the delivery waits to retry")

	require.True(t, svc.Stop(5*time.Second), "the waiting delivery ends within the drain timeout")
	assert.Equal(t, []driver.Value{
		string(domain.DeliveryStatusRetrying), string(domain.DeliveryStatusAbandoned),
	}, inserts.status.all())
	assert.Equal(t, "HTTP 500; not sent: the server stopped before the attempt", inserts.errorMessage.all()[1])
	assert.Nil(t, inserts.nextRetryAt.all()[1])

	svc.dispatch(webhook, boundsTestPayload(webhook))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, string(domain.DeliveryStatusAbandoned), inserts.status.all()[2])
	assert.Equal(t, "not sent: the server is stopping", inserts.errorMessage.all()[2])
	assert.Equal(t, int32(1), atomic.LoadInt32(hits), "no attempt is sent after Stop")

	assert.True(t, svc.Stop(time.Second), "Stop may be called again")
}

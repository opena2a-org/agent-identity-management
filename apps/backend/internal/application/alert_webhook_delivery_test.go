package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// coalescingAlertRepo is an in-memory AlertRepository whose CreateCoalesced
// applies the same predicate as the SQL behind the repository's (same
// organization, same key, unacknowledged, created at or after since), so the
// coalescing window is tested against the service's clock. Methods CreateAlert
// does not use are left to the embedded nil interface and panic if called.
type coalescingAlertRepo struct {
	domain.AlertRepository

	mu          sync.Mutex
	alerts      []*domain.Alert
	occurrences map[uuid.UUID]int
	lastSeen    map[uuid.UUID]time.Time
}

func newCoalescingAlertRepo() *coalescingAlertRepo {
	return &coalescingAlertRepo{
		occurrences: make(map[uuid.UUID]int),
		lastSeen:    make(map[uuid.UUID]time.Time),
	}
}

func (r *coalescingAlertRepo) Create(alert *domain.Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.insert(alert)
	return nil
}

func (r *coalescingAlertRepo) CreateCoalesced(alert *domain.Alert, since, seenAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var newest *domain.Alert
	for _, a := range r.alerts {
		if a.OrganizationID != alert.OrganizationID || a.DedupeKey != alert.DedupeKey || a.IsAcknowledged || a.CreatedAt.Before(since) {
			continue
		}
		if newest == nil || a.CreatedAt.After(newest.CreatedAt) {
			newest = a
		}
	}
	if newest != nil {
		r.occurrences[newest.ID]++
		r.lastSeen[newest.ID] = seenAt
		return false, nil
	}
	r.insert(alert)
	return true, nil
}

func (r *coalescingAlertRepo) insert(alert *domain.Alert) {
	if alert.ID == uuid.Nil {
		alert.ID = uuid.New()
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}
	r.alerts = append(r.alerts, alert)
	r.occurrences[alert.ID] = 1
}

func (r *coalescingAlertRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.alerts)
}

// dispatchedEvent is one TriggerEvent call as recordingDispatcher saw it.
type dispatchedEvent struct {
	ctxErr error
	orgID  uuid.UUID
	event  domain.WebhookEvent
	data   map[string]interface{}
}

// recordingDispatcher stands in for WebhookService and records every
// TriggerEvent call, including whether its context had already ended.
type recordingDispatcher struct {
	calls chan dispatchedEvent
}

func newRecordingDispatcher() *recordingDispatcher {
	return &recordingDispatcher{calls: make(chan dispatchedEvent, 16)}
}

func (d *recordingDispatcher) TriggerEvent(ctx context.Context, orgID uuid.UUID, event domain.WebhookEvent, data map[string]interface{}) error {
	d.calls <- dispatchedEvent{ctxErr: ctx.Err(), orgID: orgID, event: event, data: data}
	return nil
}

func (d *recordingDispatcher) next(t *testing.T) dispatchedEvent {
	t.Helper()
	select {
	case e := <-d.calls:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook event was dispatched")
		return dispatchedEvent{}
	}
}

func (d *recordingDispatcher) assertNoMore(t *testing.T) {
	t.Helper()
	select {
	case e := <-d.calls:
		t.Fatalf("unexpected extra webhook event %s for alert %v", e.event, e.data["alertId"])
	case <-time.After(200 * time.Millisecond):
	}
}

func capabilityViolationAlert(orgID, agentID uuid.UUID, capability, resource string) *domain.Alert {
	return &domain.Alert{
		OrganizationID: orgID,
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityHigh,
		Title:          "Capability Violation: agent attempted " + capability,
		Description:    "Agent attempted capability " + capability + " on resource " + resource,
		ResourceType:   "agent",
		ResourceID:     agentID,
		DedupeKey:      CapabilityViolationDedupeKey(agentID, capability, resource),
	}
}

func TestAlertService_CreateAlert_CoalescesRepeatsInsideTheWindow(t *testing.T) {
	repo := newCoalescingAlertRepo()
	dispatcher := newRecordingDispatcher()
	service := NewAlertService(repo, nil, nil)
	service.webhookService = dispatcher

	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	clock := t0
	service.now = func() time.Time { return clock }

	ctx := context.Background()
	orgID, agentID := uuid.New(), uuid.New()

	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	require.Equal(t, 1, repo.count())
	first := repo.alerts[0]
	assert.Equal(t, t0, first.CreatedAt, "a coalescing alert is created at the service clock the window is measured on")
	created := dispatcher.next(t)
	assert.Equal(t, domain.WebhookEventAlertCreated, created.event)
	assert.Equal(t, first.ID.String(), created.data["alertId"])

	// One second inside the window: counted on the open alert, no new row, no webhook.
	clock = t0.Add(AlertCoalesceWindow - time.Second)
	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	assert.Equal(t, 1, repo.count(), "a repeat at W-1s must not create a second alert")
	assert.Equal(t, 2, repo.occurrences[first.ID])
	assert.Equal(t, clock, repo.lastSeen[first.ID])

	// One second past the window: a new alert and one new alert.created.
	clock = t0.Add(AlertCoalesceWindow + time.Second)
	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	require.Equal(t, 2, repo.count(), "a repeat at W+1s is a new alert")
	second := repo.alerts[1]
	assert.Equal(t, 1, repo.occurrences[second.ID])
	created = dispatcher.next(t)
	assert.Equal(t, second.ID.String(), created.data["alertId"])

	// Three violations, two alerts, two deliveries.
	dispatcher.assertNoMore(t)
}

func TestAlertService_CreateAlert_DistinctKeysAndAcknowledgedAlertsDoNotCoalesce(t *testing.T) {
	repo := newCoalescingAlertRepo()
	dispatcher := newRecordingDispatcher()
	service := NewAlertService(repo, nil, nil)
	service.webhookService = dispatcher

	t0 := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	clock := t0
	service.now = func() time.Time { return clock }

	ctx := context.Background()
	orgID, agentID := uuid.New(), uuid.New()

	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	dispatcher.next(t)

	// Another resource is another violation.
	clock = t0.Add(time.Minute)
	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/shadow")))
	assert.Equal(t, 2, repo.count())
	dispatcher.next(t)

	// Acknowledging ends the coalescing: the next repeat inside the window is a new alert.
	repo.alerts[0].IsAcknowledged = true
	clock = t0.Add(2 * time.Minute)
	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(orgID, agentID, "file:write", "/etc/passwd")))
	assert.Equal(t, 3, repo.count())
	dispatcher.next(t)

	dispatcher.assertNoMore(t)
}

func TestAlertService_CreateAlert_DeliveryContextDoesNotEndWithTheRequest(t *testing.T) {
	dispatcher := newRecordingDispatcher()
	service := NewAlertService(newCoalescingAlertRepo(), nil, nil)
	service.webhookService = dispatcher

	// The request that created the alert has ended by the time delivery runs.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, service.CreateAlert(ctx, capabilityViolationAlert(uuid.New(), uuid.New(), "db:query", "users")))

	got := dispatcher.next(t)
	assert.NoError(t, got.ctxErr, "webhook delivery must not run under the creating request's context")
}

// TestAlertService_CreateAlert_DeliversOneSignedAlertCreated drives the real
// WebhookService against a recording endpoint: the alert is created under an
// already-cancelled context, and the webhook subscribes to alert.created as
// its second event, so X-Webhook-Event cannot be satisfied by the first one.
func TestAlertService_CreateAlert_DeliversOneSignedAlertCreated(t *testing.T) {
	type received struct {
		header http.Header
		body   []byte
	}
	posts := make(chan received, 4)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		posts <- received{header: r.Header.Clone(), body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer endpoint.Close()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	orgID, webhookID := uuid.New(), uuid.New()
	secret := "test-webhook-secret"
	now := time.Now().UTC()
	mock.ExpectQuery(`FROM webhooks`).WithArgs(orgID).WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "organization_id", "name", "url", "events", "secret", "is_active",
			"timeout_seconds", "max_retries", "retry_delay_seconds",
			"last_triggered", "success_count", "failure_count", "created_by", "created_at", "updated_at",
		}).AddRow(
			webhookID.String(), orgID.String(), "alerts", endpoint.URL,
			[]byte("{alert.acknowledged,alert.created}"), secret, true,
			5, 1, 1,
			nil, 0, 0, uuid.New().String(), now, now,
		))
	mock.ExpectExec(`INSERT INTO webhook_deliveries`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE webhooks`).WillReturnResult(sqlmock.NewResult(0, 1))

	webhookService := NewWebhookService(repository.NewWebhookRepository(db))
	// The endpoint is an httptest server on loopback, which the egress client refuses.
	webhookService.newHTTPClient = loopbackAdmittingClient
	service := NewAlertService(newCoalescingAlertRepo(), nil, nil)
	service.SetWebhookService(webhookService)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	alert := capabilityViolationAlert(orgID, uuid.New(), "file:write", "/etc/passwd")
	require.NoError(t, service.CreateAlert(ctx, alert))

	var post received
	select {
	case post = <-posts:
	case <-time.After(10 * time.Second):
		t.Fatal("the subscribed endpoint received no alert.created POST")
	}

	assert.Equal(t, "alert.created", post.header.Get("X-Webhook-Event"),
		"X-Webhook-Event must carry the delivered event, not the webhook's first subscription")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(post.body)
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), post.header.Get("X-Webhook-Signature"),
		"the signature must verify over the delivered body")

	var payload domain.WebhookPayload
	require.NoError(t, json.Unmarshal(post.body, &payload))
	assert.Equal(t, domain.WebhookEventAlertCreated, payload.Event)
	assert.Equal(t, orgID.String(), payload.OrganizationID)
	assert.Equal(t, alert.ID.String(), payload.Data["alertId"])

	assert.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, 5*time.Second, 10*time.Millisecond,
		"the delivery must be recorded as delivered")

	select {
	case extra := <-posts:
		t.Fatalf("expected exactly one POST per created alert, got another with event %q", extra.header.Get("X-Webhook-Event"))
	case <-time.After(200 * time.Millisecond):
	}
}

func TestCapabilityViolationDedupeKey(t *testing.T) {
	agentID := uuid.New()
	key := CapabilityViolationDedupeKey(agentID, "file:write", "/etc/passwd")

	assert.Equal(t, key, CapabilityViolationDedupeKey(agentID, "file:write", "/etc/passwd"), "the same violation has the same key")
	assert.True(t, strings.HasPrefix(key, "capability_violation:"+agentID.String()+":"))

	assert.NotEqual(t, key, CapabilityViolationDedupeKey(uuid.New(), "file:write", "/etc/passwd"), "another agent")
	assert.NotEqual(t, key, CapabilityViolationDedupeKey(agentID, "file:read", "/etc/passwd"), "another capability")
	assert.NotEqual(t, key, CapabilityViolationDedupeKey(agentID, "file:write", "/etc/shadow"), "another resource")
	assert.NotEqual(t,
		CapabilityViolationDedupeKey(agentID, "a:b", "c"),
		CapabilityViolationDedupeKey(agentID, "a", "b:c"),
		"moving a separator between capability and resource is another violation")

	// The resource is caller-supplied; the key length must not follow it.
	long := CapabilityViolationDedupeKey(agentID, "db:query", strings.Repeat("x", 64*1024))
	assert.Equal(t, len(key), len(long))
}

// TestSetWebhookService_HasNoCallerInCmd holds the order of the change that
// turns alert.created delivery on. Coalescing bounds how many alerts the
// capability-violation producer creates; wiring the webhook service into the
// server is a separate change that lands after the delivery-side bound (retry
// budget and concurrency ceiling for a slow or failing endpoint). That change
// removes this test.
func TestSetWebhookService_HasNoCallerInCmd(t *testing.T) {
	root := repoRoot(t)
	countCalls := func(dir string) int {
		t.Helper()
		n := 0
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			n += strings.Count(string(src), ".SetWebhookService(")
			return nil
		})
		require.NoError(t, err)
		return n
	}

	// Positive control: the same scan finds the calls this package's tests make,
	// so a zero for cmd/ is a real zero and not a scan that matches nothing.
	require.Greater(t, countCalls(filepath.Join(root, "apps", "backend", "internal", "application")), 0)

	assert.Zero(t, countCalls(filepath.Join(root, "apps", "backend", "cmd")),
		"alert.created delivery is switched on by its own change, after the delivery-side bound")
}

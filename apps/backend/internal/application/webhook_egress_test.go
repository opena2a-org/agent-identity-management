package application

import (
	"bytes"
	"context"
	"database/sql/driver"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// egressAdmitLoopback is the production address policy with loopback admitted,
// so a delivery can reach an httptest server.
func egressAdmitLoopback(ip net.IP) (string, bool) {
	if ip.IsLoopback() {
		return "", false
	}
	return utils.DeniedAddressClass(ip)
}

func loopbackAdmittingClient(timeout time.Duration) *http.Client {
	return utils.NewEgressClientWithPolicy(timeout, egressAdmitLoopback)
}

// argCapture is a sqlmock argument that records the value it is matched against.
type argCapture struct{ value driver.Value }

func (c *argCapture) Match(v driver.Value) bool {
	c.value = v
	return true
}

// expectWebhookRow serves a webhook straight from the repository's result set, the way
// a row looks after its URL was accepted and the name later moved.
func expectWebhookRow(mock sqlmock.Sqlmock, id uuid.UUID, url string) {
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "name", "url", "events", "secret", "is_active",
		"timeout_seconds", "max_retries", "retry_delay_seconds",
		"last_triggered", "success_count", "failure_count", "created_by", "created_at", "updated_at",
	}).AddRow(id.String(), uuid.New().String(), "endpoint", url, "{webhook.test}", "secret", true, 10, 0, 0, nil, 0, 0, uuid.New().String(), now, now)
	mock.ExpectQuery("FROM webhooks").WithArgs(id.String()).WillReturnRows(rows)
}

// expectDelivery expects one delivery insert and returns its status code and
// response body arguments.
func expectDelivery(mock sqlmock.Sqlmock) (statusCode, responseBody *argCapture) {
	statusCode, responseBody = &argCapture{}, &argCapture{}
	args := make([]driver.Value, 16)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[6], args[7] = statusCode, responseBody
	mock.ExpectExec("INSERT INTO webhook_deliveries").WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
	return statusCode, responseBody
}

func newMockedWebhookService(t *testing.T) (*WebhookService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewWebhookService(repository.NewWebhookRepository(db)), mock
}

func countingEndpoint(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

var ipLiteral = regexp.MustCompile(`\b\d{1,3}(\.\d{1,3}){3}\b|\[?[0-9A-Fa-f]{0,4}:[0-9A-Fa-f]{0,4}:[0-9A-Fa-f:.]*\]?`)

func TestWebhookDelivery_RedirectIsRecordedAndNotFollowed(t *testing.T) {
	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusFound} {
		internal, internalHits := countingEndpoint(t)
		endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, internal.URL+"/latest/meta-data/", code)
		}))
		t.Cleanup(endpoint.Close)

		svc, mock := newMockedWebhookService(t)
		svc.newHTTPClient = loopbackAdmittingClient
		webhook := &domain.Webhook{ID: uuid.New(), URL: endpoint.URL, Events: []domain.WebhookEvent{"agent.created"}, Secret: "s"}

		// Delivery, retry and replay all send through attemptDelivery.
		status, _, _, err := svc.attemptDelivery(webhook, "agent.created", []byte(`{}`), 5*time.Second)
		require.NoError(t, err)
		assert.Equal(t, code, status, "delivery returns the redirect status")

		// The test send records the redirect as a failed delivery.
		expectWebhookRow(mock, webhook.ID, endpoint.URL)
		recordedStatus, _ := expectDelivery(mock)
		result, err := svc.TestWebhook(context.Background(), webhook.ID)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, code, result.StatusCode)
		assert.EqualValues(t, code, recordedStatus.value, "delivery record keeps the redirect status")
		assert.NoError(t, mock.ExpectationsWereMet())

		assert.EqualValues(t, 0, atomic.LoadInt32(internalHits), "redirect target must not be requested")

		// Control: a plain client against the same endpoint reaches the target.
		resp, err := (&http.Client{}).Post(endpoint.URL, "application/json", bytes.NewBufferString(`{}`))
		require.NoError(t, err)
		resp.Body.Close()
		assert.EqualValues(t, 1, atomic.LoadInt32(internalHits), "control: plain client follows the redirect")
	}
}

func TestWebhookDelivery_RefusesStoredURLThatResolvesToLoopback(t *testing.T) {
	internal, hits := countingEndpoint(t)
	_, port, err := net.SplitHostPort(internal.Listener.Addr().String())
	require.NoError(t, err)
	// Validation refuses this URL at create time; the row stands for one whose
	// name resolved to a public address then and to loopback now.
	storedURL := "http://localhost:" + port + "/"
	webhook := &domain.Webhook{ID: uuid.New(), URL: storedURL, Events: []domain.WebhookEvent{"agent.created"}, Secret: "s"}

	svc, mock := newMockedWebhookService(t)
	expectWebhookRow(mock, webhook.ID, storedURL)
	result, err := svc.TestWebhook(context.Background(), webhook.ID)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 0, result.StatusCode)
	assert.Contains(t, result.ErrorMessage, "not allowed (loopback)")
	assert.False(t, ipLiteral.MatchString(result.ErrorMessage), "test result names an address: %q", result.ErrorMessage)
	assert.NoError(t, mock.ExpectationsWereMet())

	_, _, _, err = svc.attemptDelivery(webhook, "agent.created", []byte(`{}`), 5*time.Second)
	require.Error(t, err)
	assert.False(t, ipLiteral.MatchString(err.Error()), "delivery error names an address: %q", err.Error())

	assert.EqualValues(t, 0, atomic.LoadInt32(hits), "loopback endpoint must not be reached")

	// Control: the same row with loopback admitted is delivered.
	control, mock := newMockedWebhookService(t)
	control.newHTTPClient = loopbackAdmittingClient
	expectWebhookRow(mock, webhook.ID, storedURL)
	expectDelivery(mock)
	result, err = control.TestWebhook(context.Background(), webhook.ID)
	require.NoError(t, err)
	assert.True(t, result.Success, result.ErrorMessage)
	assert.EqualValues(t, 1, atomic.LoadInt32(hits), "control: loopback endpoint reached once")
}

func TestWebhookRefusalText_RegexFindsPlantedAddress(t *testing.T) {
	for _, planted := range []string{
		`Post "http://hooks.example/": dial tcp 10.0.0.5:443: connect: connection refused`,
		`failed to resolve hostname "hooks.example": lookup hooks.example on [fd00::53]:53: no such host`,
	} {
		assert.True(t, ipLiteral.MatchString(planted), "control: regex misses %q", planted)
	}
}

func TestWebhookDelivery_StoresAtMostOneKilobyteOfResponse(t *testing.T) {
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte("a"), 2<<20))
	}))
	t.Cleanup(large.Close)

	svc, mock := newMockedWebhookService(t)
	svc.newHTTPClient = loopbackAdmittingClient
	webhook := &domain.Webhook{ID: uuid.New(), URL: large.URL, Events: []domain.WebhookEvent{"agent.created"}, Secret: "s"}

	expectWebhookRow(mock, webhook.ID, large.URL)
	_, recordedBody := expectDelivery(mock)
	_, err := svc.TestWebhook(context.Background(), webhook.ID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	body, ok := recordedBody.value.(string)
	require.True(t, ok, "response body argument is %T", recordedBody.value)
	assert.LessOrEqual(t, len(body), 1024, "test send stores the response body")

	status, deliveredBody, _, err := svc.attemptDelivery(webhook, "agent.created", []byte(`{}`), 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.LessOrEqual(t, len(deliveredBody), 1024, "delivery stores the response body")
}

// Every client that sends to a tenant- or agent-supplied URL is built by the
// egress constructor.
func TestTenantURLClients_AreBuiltByTheEgressConstructor(t *testing.T) {
	forbidden := regexp.MustCompile(`http\.Client\{|new\(http\.Client\)|http\.DefaultClient|http\.DefaultTransport|\bhttp\.(Get|Post|PostForm|Head)\(`)
	for _, file := range []string{"webhook_service.go", "mcp_capability_service.go", "mcp_service.go", "a2a_service.go"} {
		src, err := os.ReadFile(file)
		require.NoError(t, err)
		assert.Contains(t, string(src), "utils.NewEgressClient", file)
		assert.Empty(t, forbidden.FindAllString(string(src), -1), "%s builds an HTTP client outside the egress constructor", file)
	}

	// Only tests may widen the address policy.
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Base(path) == "egress_client.go" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		assert.NotContains(t, string(src), "NewEgressClientWithPolicy", "%s uses the test-only egress policy", path)
		return nil
	})
	require.NoError(t, err)
}

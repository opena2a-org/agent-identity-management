package siem

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/sentineltest"
)

const (
	stubTenant = "00000000-0000-0000-0000-0000000000aa"
	stubClient = "00000000-0000-0000-0000-0000000000bb"
	stubDCR    = "dcr-00000000000000000000000000000001"
	stubStream = "Custom-AIMSecurityEvents"
)

// stubSecret is assembled at run time; it is a placeholder, not a credential.
var stubSecret = strings.Join([]string{"stub", "sentinel", "secret"}, "-")

type sentinelStubs struct {
	tokens    *sentineltest.TokenServer
	ingestion *sentineltest.IngestionServer
}

func newSentinelStubs(t *testing.T) sentinelStubs {
	t.Helper()
	tokens := sentineltest.NewTokenServer(stubTenant, stubClient, stubSecret)
	t.Cleanup(tokens.Close)
	ingestion := sentineltest.NewIngestionServer(stubDCR, stubStream, tokens)
	t.Cleanup(ingestion.Close)
	return sentinelStubs{tokens: tokens, ingestion: ingestion}
}

func (s sentinelStubs) adapter(secret string) *SentinelAdapter {
	return NewSentinelAdapter(SentinelConfig{
		Endpoint:       s.ingestion.Endpoint(),
		DCRImmutableID: stubDCR,
		StreamName:     stubStream,
		TenantID:       stubTenant,
		ClientID:       stubClient,
		ClientSecret:   secret,
		AuthorityHost:  s.tokens.AuthorityHost(),
	})
}

func sentinelEvent(eventType EventType) *SIEMEvent {
	return &SIEMEvent{
		EventType: eventType,
		Timestamp: time.Date(2026, 10, 16, 18, 0, 0, 0, time.UTC),
		Severity:  SeverityLow,
		Source:    "opena2a-aim",
	}
}

func TestSentinelAdapter_DeliversABatchAsOneUpload(t *testing.T) {
	stubs := newSentinelStubs(t)
	adapter := stubs.adapter(stubSecret)

	require.NoError(t, adapter.SendBatch(context.Background(), []*SIEMEvent{
		sentinelEvent(EventSignInFailed), sentinelEvent(EventFGADenied),
	}))

	accepted := stubs.ingestion.Accepted()
	require.Len(t, accepted, 2)
	first, _ := accepted[0].String("EventType")
	second, _ := accepted[1].String("EventType")
	assert.Equal(t, []string{"aim.auth.signin_failed", "aim.fga.denied"}, []string{first, second})
	assert.Equal(t, 1, stubs.ingestion.Requests(), "one upload for the batch")
	assert.Empty(t, stubs.ingestion.Refusals())
	assert.NoError(t, adapter.SendBatch(context.Background(), nil), "an empty batch sends nothing")
	assert.Equal(t, 1, stubs.ingestion.Requests())
	assert.Equal(t, 1, stubs.tokens.Requests())
}

func TestSentinelAdapter_CachesTheTokenAcrossBatches(t *testing.T) {
	stubs := newSentinelStubs(t)
	adapter := stubs.adapter(stubSecret)

	require.NoError(t, adapter.SendBatch(context.Background(), []*SIEMEvent{sentinelEvent(EventSignInFailed)}))
	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventFGADenied)))

	assert.Equal(t, 1, stubs.tokens.Requests(), "the second batch reuses the token")
	assert.Len(t, stubs.ingestion.Accepted(), 2)
}

func TestSentinelAdapter_AsksForANewTokenSixtySecondsBeforeExpiry(t *testing.T) {
	stubs := newSentinelStubs(t)
	adapter := stubs.adapter(stubSecret)
	start := time.Now()
	now := start
	adapter.now = func() time.Time { return now }

	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
	require.Equal(t, 1, stubs.tokens.Requests())

	now = start.Add(sentineltest.ExpiresIn*time.Second - 61*time.Second)
	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
	assert.Equal(t, 1, stubs.tokens.Requests(), "61 seconds before expiry the token is still used")

	now = start.Add(sentineltest.ExpiresIn*time.Second - 60*time.Second)
	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
	assert.Equal(t, 2, stubs.tokens.Requests(), "60 seconds before expiry a new token is asked for")
	assert.Len(t, stubs.ingestion.Accepted(), 3)
}

func TestSentinelAdapter_A401FromIngestionClearsTheToken(t *testing.T) {
	stubs := newSentinelStubs(t)
	adapter := stubs.adapter(stubSecret)
	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))

	// The ingestion endpoint no longer accepts the cached token.
	stubs.tokens.RevokeAll()
	err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	var status *StatusError
	require.True(t, errors.As(err, &status), "got %v", err)
	assert.Equal(t, http.StatusUnauthorized, status.StatusCode)
	assert.Equal(t, "sentinel", status.Service)
	assert.False(t, status.Retryable())
	assert.Equal(t, 1, stubs.tokens.Requests())

	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
	assert.Equal(t, 2, stubs.tokens.Requests(), "the next batch asks for a new token")
	assert.Len(t, stubs.ingestion.Accepted(), 2)
}

func TestSentinelAdapter_AWrongSecretIsNotRetryable(t *testing.T) {
	stubs := newSentinelStubs(t)
	wrong := stubSecret + "-wrong"
	adapter := stubs.adapter(wrong)

	err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	var status *StatusError
	require.True(t, errors.As(err, &status), "got %v", err)
	assert.Equal(t, http.StatusUnauthorized, status.StatusCode)
	assert.Contains(t, status.Service, "sentinel")
	assert.False(t, status.Retryable())
	assert.Contains(t, err.Error(), "invalid_client")
	assert.NotContains(t, err.Error(), wrong)
	assert.Equal(t, []sentineltest.Refusal{{Status: 401, Error: "invalid_client"}}, stubs.tokens.Refusals())
	assert.Equal(t, 0, stubs.ingestion.Requests(), "no upload without a token")

	// Each batch asks again: a refused token is not cached.
	_ = adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	assert.Equal(t, 2, stubs.tokens.Requests())
}

// A forwarder does not retry a refused token, and counts the batch failed.
func TestSentinelAdapter_ForwarderCountsARefusedTokenOnce(t *testing.T) {
	stubs := newSentinelStubs(t)
	f := NewForwarder([]BatchAdapter{stubs.adapter(stubSecret + "-wrong")}, ForwarderConfig{RetryBackoff: time.Millisecond}, nil)
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	require.True(t, f.Enqueue(sentinelEvent(EventSignInFailed)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.Flush(ctx))

	assert.Equal(t, 1, stubs.tokens.Requests(), "not retried")
	assert.Equal(t, []AdapterStats{{Adapter: "sentinel", Sent: 0, Failed: 1}}, f.AdapterStats())
	assert.True(t, f.Health().Failing)
}

// recordedRequest is a request a plain loopback server received.
type recordedRequest struct {
	path, rawQuery, authorization string
}

// plainServer answers each request with the next of statuses and body, and
// records it.
func plainServer(t *testing.T, statuses []int, body string) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, recordedRequest{r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization")})
		status := http.StatusNoContent
		if len(seen) <= len(statuses) {
			status = statuses[len(seen)-1]
		}
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), seen...)
	}
}

func TestSentinelAdapter_UploadAnswers(t *testing.T) {
	for _, tc := range []struct {
		status        int
		retryable     bool
		keepsTheToken bool
	}{
		{http.StatusForbidden, false, false},
		{http.StatusBadRequest, false, true},
		{http.StatusRequestTimeout, true, true},
		{http.StatusTooManyRequests, true, true},
		{http.StatusInternalServerError, true, true},
		{http.StatusServiceUnavailable, true, true},
		{http.StatusTemporaryRedirect, false, true},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			stubs := newSentinelStubs(t)
			upload, seen := plainServer(t, []int{tc.status}, "refused")
			adapter := stubs.adapter(stubSecret)
			adapter.config.Endpoint = upload.URL

			err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
			var status *StatusError
			require.True(t, errors.As(err, &status), "got %v", err)
			assert.Equal(t, tc.status, status.StatusCode)
			assert.Equal(t, tc.retryable, status.Retryable())

			require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
			want := 2
			if tc.keepsTheToken {
				want = 1
			}
			assert.Equal(t, want, stubs.tokens.Requests())
			assert.Len(t, seen(), 2)
		})
	}
}

func TestSentinelAdapter_AnyTwoHundredIsDelivery(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		stubs := newSentinelStubs(t)
		upload, _ := plainServer(t, []int{code}, "")
		adapter := stubs.adapter(stubSecret)
		adapter.config.Endpoint = upload.URL
		assert.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)), "status %d", code)
	}
}

func TestSentinelAdapter_EscapesTheRuleAndStream(t *testing.T) {
	stubs := newSentinelStubs(t)
	upload, seen := plainServer(t, nil, "")
	adapter := stubs.adapter(stubSecret)
	adapter.config.Endpoint = upload.URL + "/"
	adapter.config.DCRImmutableID = "dcr id/x"
	adapter.config.StreamName = "Custom-A?b"

	require.NoError(t, adapter.Send(context.Background(), sentinelEvent(EventSignInFailed)))
	got := seen()
	require.Len(t, got, 1)
	assert.Equal(t, "/dataCollectionRules/dcr%20id%2Fx/streams/Custom-A%3Fb", got[0].path)
	assert.Equal(t, "api-version=2023-01-01", got[0].rawQuery)
	assert.True(t, strings.HasPrefix(got[0].authorization, "Bearer "))
}

func TestSentinelAdapter_NoErrorCarriesTheSecretOrTheToken(t *testing.T) {
	stubs := newSentinelStubs(t)

	// A token endpoint that echoes the secret in its refusal. "invalid_client: "
	// is 16 bytes, so the echoed secret starts at byte 500 of the error body
	// and runs past the 512 bytes a StatusError keeps.
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client",
			"error_description": strings.Repeat("x", 484) + r.PostForm.Get("client_secret") + " trailing",
		})
	}))
	t.Cleanup(echo.Close)
	adapter := stubs.adapter(stubSecret)
	adapter.config.AuthorityHost = echo.URL
	err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	var status *StatusError
	require.True(t, errors.As(err, &status), "got %v", err)
	assert.NotContains(t, err.Error(), stubSecret)
	assert.NotContains(t, err.Error(), stubSecret[:8], "no part of the secret survives the cut to 512 bytes")
	assert.Contains(t, status.Body, "[redacted]")
	assert.LessOrEqual(t, len(status.Body), maxErrorBody)

	// An ingestion endpoint that echoes the bearer in its refusal.
	var bearer string
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("refused " + r.Header.Get("Authorization")))
	}))
	t.Cleanup(upload.Close)
	adapter = stubs.adapter(stubSecret)
	adapter.config.Endpoint = upload.URL
	err = adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	require.Error(t, err)
	require.NotEmpty(t, bearer)
	assert.NotContains(t, err.Error(), bearer)
	assert.Contains(t, err.Error(), "[redacted]")
}

func TestSentinelAdapter_DoesNotFollowARedirectWithTheSecret(t *testing.T) {
	stubs := newSentinelStubs(t)
	elsewhere, seen := plainServer(t, nil, "")
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirect.Close)
	adapter := stubs.adapter(stubSecret)
	adapter.config.AuthorityHost = redirect.URL

	err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
	var status *StatusError
	require.True(t, errors.As(err, &status), "got %v", err)
	assert.Equal(t, http.StatusTemporaryRedirect, status.StatusCode)
	assert.Empty(t, seen(), "the token request is not sent on")
}

func TestSentinelAdapter_TokenAnswersWithoutABearer(t *testing.T) {
	for name, body := range map[string]string{
		"not json":          `<html></html>`,
		"no access token":   `{"token_type":"Bearer","expires_in":3599}`,
		"another type":      `{"token_type":"MAC","expires_in":3599,"access_token":"abc"}`,
		"no expiry":         `{"token_type":"Bearer","access_token":"abc"}`,
		"a negative expiry": `{"token_type":"Bearer","expires_in":-1,"access_token":"abc"}`,
	} {
		t.Run(name, func(t *testing.T) {
			stubs := newSentinelStubs(t)
			token, _ := plainServer(t, []int{http.StatusOK}, body)
			adapter := stubs.adapter(stubSecret)
			adapter.config.AuthorityHost = token.URL
			err := adapter.Send(context.Background(), sentinelEvent(EventSignInFailed))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "abc")
			assert.Equal(t, 0, stubs.ingestion.Requests())
		})
	}
}

// Each record carries the eight columns, each named as the Logs Ingestion API
// allows, mapped from the event.
func TestSentinelAdapter_RecordColumns(t *testing.T) {
	stubs := newSentinelStubs(t)
	adapter := stubs.adapter(stubSecret)
	denver := time.FixedZone("MDT", -6*60*60)

	full := &SIEMEvent{
		EventType:  EventFGADenied,
		Timestamp:  time.Date(2026, 10, 16, 12, 0, 0, 123456789, denver),
		AgentID:    "agent-1",
		Capability: "db.read",
		Outcome:    "DENY",
		Severity:   SeverityMedium,
		Details:    map[string]interface{}{"organizationId": "org-1", "deniedBy": "capability_check"},
		Source:     "aim-test",
	}
	bare := &SIEMEvent{EventType: EventSignInFailed, Timestamp: time.Date(2026, 10, 16, 18, 0, 0, 0, time.UTC), Severity: SeverityLow}
	require.NoError(t, adapter.SendBatch(context.Background(), []*SIEMEvent{full, bare}))

	accepted := stubs.ingestion.Accepted()
	require.Len(t, accepted, 2)
	columns := make([]string, 0, len(accepted[0]))
	for name := range accepted[0] {
		columns = append(columns, name)
		assert.True(t, sentineltest.AllowedColumn(name), name)
	}
	sort.Strings(columns)
	assert.Equal(t, []string{"AgentId", "Capability", "Details", "EventType", "Outcome", "Severity", "Source", "TimeGenerated"}, columns)

	column := func(record sentineltest.Record, name string) string {
		value, ok := record.String(name)
		require.True(t, ok, "column %s is a string", name)
		return value
	}
	assert.Equal(t, "2026-10-16T18:00:00.123456789Z", column(accepted[0], "TimeGenerated"), "the event's time in UTC")
	assert.Equal(t, "aim.fga.denied", column(accepted[0], "EventType"))
	assert.Equal(t, "medium", column(accepted[0], "Severity"))
	assert.Equal(t, "agent-1", column(accepted[0], "AgentId"))
	assert.Equal(t, "db.read", column(accepted[0], "Capability"))
	assert.Equal(t, "DENY", column(accepted[0], "Outcome"))
	assert.Equal(t, "aim-test", column(accepted[0], "Source"))
	assert.JSONEq(t, `{"organizationId":"org-1","deniedBy":"capability_check"}`, string(accepted[0]["Details"]))

	assert.Equal(t, "2026-10-16T18:00:00Z", column(accepted[1], "TimeGenerated"))
	assert.Equal(t, "aim.auth.signin_failed", column(accepted[1], "EventType"))
	assert.Equal(t, "", column(accepted[1], "AgentId"))
	assert.Equal(t, "opena2a-aim", column(accepted[1], "Source"), "the default source")
	assert.JSONEq(t, `{}`, string(accepted[1]["Details"]), "Details is always an object")

	assert.Equal(t, "aim-test", full.Source, "the event is not modified")
	assert.Nil(t, bare.Details)
}

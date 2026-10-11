package siem

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hecRequest is one request a fake HTTP Event Collector received.
type hecRequest struct {
	authorization string
	contentType   string
	events        []map[string]interface{}
}

// fakeHEC answers each request with the next status of statuses (200 once
// they run out) and records what it received.
type fakeHEC struct {
	mu       sync.Mutex
	statuses []int
	requests []hecRequest
}

func (h *fakeHEC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	req := hecRequest{authorization: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type")}
	dec := json.NewDecoder(bytes.NewReader(body))
	for {
		var event map[string]interface{}
		if err := dec.Decode(&event); err != nil {
			break
		}
		req.events = append(req.events, event)
	}

	h.mu.Lock()
	h.requests = append(h.requests, req)
	status := http.StatusOK
	if len(h.statuses) > 0 {
		status, h.statuses = h.statuses[0], h.statuses[1:]
	}
	h.mu.Unlock()

	w.WriteHeader(status)
	if status == http.StatusOK {
		_, _ = w.Write([]byte(`{"text":"Success","code":0}`))
	} else {
		_, _ = w.Write([]byte(`{"text":"refused","code":9}`))
	}
}

func (h *fakeHEC) received() []hecRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hecRequest(nil), h.requests...)
}

func TestSplunkAdapter_SendBatchPostsOneHECEventPerEvent(t *testing.T) {
	hec := &fakeHEC{}
	server := httptest.NewServer(hec)
	t.Cleanup(server.Close)

	adapter := NewSplunkAdapter(SplunkConfig{HECURL: server.URL, HECToken: "fake-hec-token", Index: "aim"})
	at := time.Date(2026, 10, 11, 12, 0, 0, 250_000_000, time.UTC)
	first := &SIEMEvent{EventType: "aim.audit.login", Timestamp: at, Severity: SeverityInfo, Source: "opena2a-aim"}
	second := &SIEMEvent{EventType: "aim.audit.honeytoken_triggered", Timestamp: at, AgentID: "agent-1", Severity: SeverityHigh, Source: "opena2a-aim"}

	require.NoError(t, adapter.SendBatch(context.Background(), []*SIEMEvent{first, second}))

	reqs := hec.received()
	require.Len(t, reqs, 1, "one request carries the whole batch")
	assert.Equal(t, "Splunk fake-hec-token", reqs[0].authorization)
	assert.Equal(t, "application/json", reqs[0].contentType)
	require.Len(t, reqs[0].events, 2)
	for i, want := range []string{"aim.audit.login", "aim.audit.honeytoken_triggered"} {
		hecEvent := reqs[0].events[i]
		assert.Equal(t, "opena2a:aim", hecEvent["sourcetype"])
		assert.Equal(t, "opena2a-aim", hecEvent["source"])
		assert.Equal(t, "aim", hecEvent["index"])
		assert.Equal(t, 1791720000.25, hecEvent["time"])
		inner, ok := hecEvent["event"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, want, inner["eventType"])
	}
	inner := reqs[0].events[0]["event"].(map[string]interface{})
	_, hasAgent := inner["agentId"]
	assert.False(t, hasAgent, "an event with no agent carries no agentId")
}

func TestSplunkAdapter_NoIndexLeavesTheTokenDefault(t *testing.T) {
	hec := &fakeHEC{}
	server := httptest.NewServer(hec)
	t.Cleanup(server.Close)

	adapter := NewSplunkAdapter(SplunkConfig{HECURL: server.URL, HECToken: "fake-hec-token"})
	require.NoError(t, adapter.Send(context.Background(), &SIEMEvent{EventType: "aim.audit.login", Timestamp: time.Now()}))

	reqs := hec.received()
	require.Len(t, reqs, 1)
	require.Len(t, reqs[0].events, 1)
	_, hasIndex := reqs[0].events[0]["index"]
	assert.False(t, hasIndex, "with no index configured the HEC token's default index applies")
}

func TestSplunkAdapter_SendBatchDoesNotModifyTheEvents(t *testing.T) {
	server := httptest.NewServer(&fakeHEC{})
	t.Cleanup(server.Close)

	adapter := NewSplunkAdapter(SplunkConfig{HECURL: server.URL, HECToken: "fake-hec-token", Source: "custom-source"})
	event := &SIEMEvent{EventType: "aim.audit.login", Timestamp: time.Now(), Source: "opena2a-aim"}
	require.NoError(t, adapter.SendBatch(context.Background(), []*SIEMEvent{event}))
	assert.Equal(t, "opena2a-aim", event.Source)
}

func TestSplunkAdapter_RefusalIsAStatusError(t *testing.T) {
	server := httptest.NewServer(&fakeHEC{statuses: []int{http.StatusForbidden}})
	t.Cleanup(server.Close)

	adapter := NewSplunkAdapter(SplunkConfig{HECURL: server.URL, HECToken: "fake-hec-token"})
	err := adapter.Send(context.Background(), &SIEMEvent{EventType: "aim.audit.login", Timestamp: time.Now()})

	var status *StatusError
	require.True(t, errors.As(err, &status), "got %v", err)
	assert.Equal(t, http.StatusForbidden, status.StatusCode)
	assert.False(t, status.Retryable())
	assert.Contains(t, err.Error(), "Splunk HEC returned 403")
	assert.NotContains(t, err.Error(), "fake-hec-token")
}

func TestStatusError_Retryable(t *testing.T) {
	for code, want := range map[int]bool{400: false, 401: false, 403: false, 404: false, 408: true, 429: true, 500: true, 503: true} {
		assert.Equal(t, want, (&StatusError{StatusCode: code}).Retryable(), "status %d", code)
	}
}

// Each HEC event of a batch is written on its own line.
func TestSplunkAdapter_SendBatchWritesOneEventPerLine(t *testing.T) {
	bodies := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies <- body
	}))
	t.Cleanup(server.Close)

	adapter := NewSplunkAdapter(SplunkConfig{HECURL: server.URL, HECToken: "fake-hec-token"})
	events := []*SIEMEvent{{EventType: "a", Timestamp: time.Now()}, {EventType: "b", Timestamp: time.Now()}, {EventType: "c", Timestamp: time.Now()}}
	require.NoError(t, adapter.SendBatch(context.Background(), events))
	body := <-bodies

	lines := 0
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		var v map[string]interface{}
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &v))
		lines++
	}
	assert.Equal(t, 3, lines)
}

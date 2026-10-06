package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/require"
)

const (
	callerTraceID  = "0af7651916cd43dd8448eb211c80319c"
	callerParentID = "b7ad6b7169203331"
	callerHeader   = "00-" + callerTraceID + "-" + callerParentID + "-01"
	eventA         = "a1a1a1a1-0000-4000-8000-00000000000a"
	eventB         = "a1a1a1a1-0000-4000-8000-00000000000b"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestMintIsSixteenRandomBytesNeverAllZeros(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := Mint()
		require.NoError(t, err)
		require.True(t, ValidID(id), "minted %q", id)
		require.False(t, seen[id], "minted %q twice", id)
		seen[id] = true
	}

	_, err := mint(zeroReader{})
	require.Error(t, err, "an all-zero identifier is never minted")
	_, err = mint(strings.NewReader("short"))
	require.Error(t, err, "a short read is never padded into an identifier")
}

func TestValidIDIsThirtyTwoLowercaseHexNotAllZeros(t *testing.T) {
	require.True(t, ValidID(callerTraceID))
	for _, bad := range []string{
		"", strings.Repeat("0", 32), strings.ToUpper(callerTraceID), callerTraceID[:31],
		callerTraceID + "0", "0af7651916cd43dd8448eb211c80319g",
	} {
		require.False(t, ValidID(bad), "%q", bad)
	}
}

func TestParseTraceparentKeepsOnlyTheTraceIDOfAValidVersion00Header(t *testing.T) {
	got, ok := ParseTraceparent(callerHeader)
	require.True(t, ok)
	require.Equal(t, callerTraceID, got)
	got, ok = ParseTraceparent("00-" + callerTraceID + "-" + callerParentID + "-00")
	require.True(t, ok)
	require.Equal(t, callerTraceID, got)

	for name, header := range map[string]string{
		"empty":              "",
		"version 01":         "01-" + callerTraceID + "-" + callerParentID + "-01",
		"version ff":         "ff-" + callerTraceID + "-" + callerParentID + "-01",
		"all-zero trace-id":  "00-" + strings.Repeat("0", 32) + "-" + callerParentID + "-01",
		"all-zero parent-id": "00-" + callerTraceID + "-" + strings.Repeat("0", 16) + "-01",
		"uppercase trace-id": "00-" + strings.ToUpper(callerTraceID) + "-" + callerParentID + "-01",
		"uppercase flags":    "00-" + callerTraceID + "-" + callerParentID + "-0A",
		"trailing field":     callerHeader + "-00",
		"short trace-id":     "00-" + callerTraceID[:31] + "-" + callerParentID + "-01",
		"wrong separator":    "00_" + callerTraceID + "-" + callerParentID + "-01",
		"leading space":      " " + callerHeader[:54],
	} {
		got, ok := ParseTraceparent(header)
		require.False(t, ok, name)
		require.Empty(t, got, name)
	}
}

func draft(eventID string) record.Draft {
	return record.Draft{
		EventID:  eventID,
		Type:     "opena2a.administrative",
		Retained: map[string]any{"opena2a": map[string]any{"admin_action": "tag_created"}},
	}
}

func TestStampGivesTheFirstRecordOfATraceTheContextsTrace(t *testing.T) {
	ctx, err := Begin(context.Background())
	require.NoError(t, err)
	tc, ok := From(ctx)
	require.True(t, ok)

	d := draft(eventA)
	require.NoError(t, Stamp(ctx, &d, nil))
	require.Equal(t, tc.TraceID, d.Tenant["trace_id"])
	require.Contains(t, d.Tenant, "parent_id")
	require.Nil(t, d.Tenant["parent_id"])
	require.Equal(t, OriginServer, d.Retained["opena2a"].(map[string]any)["trace_origin"])
	require.Equal(t, "tag_created", d.Retained["opena2a"].(map[string]any)["admin_action"])
	require.Nil(t, d.Personal, "a job run carries no request trace")

	link, err := Check(d)
	require.NoError(t, err)
	require.Equal(t, Link{TraceID: tc.TraceID, Origin: OriginServer}, link)
}

func TestStampGivesARecordWithAParentTheParentsTrace(t *testing.T) {
	parentTrace, err := Mint()
	require.NoError(t, err)
	ctx := With(context.Background(), Context{TraceID: "ffffffffffffffffffffffffffffffff", RequestTraceID: callerTraceID})

	d := draft(eventB)
	require.NoError(t, Stamp(ctx, &d, &Parent{EventID: eventA, TraceID: parentTrace}))
	require.Equal(t, parentTrace, d.Tenant["trace_id"], "the parent's trace, not the request's")
	require.Equal(t, eventA, d.Tenant["parent_id"])
	require.Equal(t, OriginParent, d.Retained["opena2a"].(map[string]any)["trace_origin"])
	require.Equal(t, callerTraceID, d.Personal["opena2a"].(map[string]any)["request_trace_id"])

	link, err := Check(d)
	require.NoError(t, err)
	require.Equal(t, Link{TraceID: parentTrace, ParentID: eventA, Origin: OriginParent}, link)

	// A job with no request trace still follows the parent.
	d = draft(eventB)
	require.NoError(t, Stamp(context.Background(), &d, &Parent{EventID: eventA, TraceID: parentTrace}))
	require.Equal(t, parentTrace, d.Tenant["trace_id"])
}

func TestStampRefusesARecordWithNoTraceToTake(t *testing.T) {
	d := draft(eventA)
	require.ErrorIs(t, Stamp(context.Background(), &d, nil), ErrNoTrace)
	require.Nil(t, d.Tenant, "a refused draft is left as it was")

	traceID, err := Mint()
	require.NoError(t, err)
	for name, p := range map[string]*Parent{
		"parent trace all zeros": {EventID: eventA, TraceID: strings.Repeat("0", 32)},
		"parent trace uppercase": {EventID: eventA, TraceID: strings.ToUpper(callerTraceID)},
		"parent event not uuid":  {EventID: "agent-1", TraceID: traceID},
		"parent is the record":   {EventID: eventB, TraceID: traceID},
	} {
		d := draft(eventB)
		require.ErrorIs(t, Stamp(context.Background(), &d, p), ErrInvalidTrace, name)
	}
}

// A run that writes records for several organizations gives each its own
// trace, and every record the run writes for one organization the same one.
func TestRunMintsOneTracePerOrganization(t *testing.T) {
	const (
		orgA = "0a0a0a0a-0000-4000-8000-00000000000a"
		orgB = "0b0b0b0b-0000-4000-8000-00000000000b"
	)
	// The run starts inside a request that sent a traceparent; its records
	// still carry only the run's traces.
	ctx := With(context.Background(), Context{TraceID: "ffffffffffffffffffffffffffffffff", RequestTraceID: callerTraceID})
	run := NewRun()

	stamp := func(org, eventID string) record.Draft {
		t.Helper()
		orgCtx, err := run.For(ctx, org)
		require.NoError(t, err)
		d := draft(eventID)
		require.NoError(t, Stamp(orgCtx, &d, nil))
		return d
	}
	a1 := stamp(orgA, eventA)
	b1 := stamp(orgB, eventB)
	a2 := stamp(orgA, "a1a1a1a1-0000-4000-8000-00000000000c")

	traceA, traceB := a1.Tenant["trace_id"].(string), b1.Tenant["trace_id"].(string)
	require.True(t, ValidID(traceA))
	require.True(t, ValidID(traceB))
	require.NotEqual(t, traceA, traceB, "no trace is shared by two organizations")
	require.Equal(t, traceA, a2.Tenant["trace_id"], "one organization's records of a run share its trace")
	for _, d := range []record.Draft{a1, b1, a2} {
		require.NotEqual(t, "ffffffffffffffffffffffffffffffff", d.Tenant["trace_id"], "not the trace of the request that started the run")
		require.Nil(t, d.Personal, "a run's records carry no request trace")
		require.Equal(t, OriginServer, d.Retained["opena2a"].(map[string]any)["trace_origin"])
	}

	// Another run mints other traces for the same organizations.
	other, err := NewRun().For(ctx, orgA)
	require.NoError(t, err)
	tc, _ := From(other)
	require.NotEqual(t, traceA, tc.TraceID)

	for _, bad := range []string{"", "org-1", strings.ToUpper(orgA)} {
		_, err := run.For(ctx, bad)
		require.ErrorIs(t, err, ErrInvalidTrace, bad)
	}
}

func TestRunIsSafeForConcurrentUse(t *testing.T) {
	const org = "0a0a0a0a-0000-4000-8000-00000000000a"
	run := NewRun()
	traces := make(chan string, 16)
	for i := 0; i < cap(traces); i++ {
		go func() {
			ctx, err := run.For(context.Background(), org)
			if err != nil {
				traces <- err.Error()
				return
			}
			tc, _ := From(ctx)
			traces <- tc.TraceID
		}()
	}
	first := <-traces
	require.True(t, ValidID(first), first)
	for i := 1; i < cap(traces); i++ {
		require.Equal(t, first, <-traces, "one organization has one trace per run")
	}
}

// Stamp copies the "opena2a" object it adds to, so drafts built from one
// shared map do not see each other's members.
func TestStampDoesNotWriteIntoASharedExtensionObject(t *testing.T) {
	shared := map[string]any{"admin_action": "tag_created"}
	ctx, err := Begin(context.Background())
	require.NoError(t, err)
	d := record.Draft{EventID: eventA, Type: "opena2a.administrative", Retained: map[string]any{"opena2a": shared}}
	require.NoError(t, Stamp(ctx, &d, nil))
	require.NotContains(t, shared, "trace_origin")
}

func TestCheckRefusesADraftWithoutAPlaceInATrace(t *testing.T) {
	stamped := func() record.Draft {
		ctx := With(context.Background(), Context{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", RequestTraceID: callerTraceID})
		d := draft(eventB)
		require.NoError(t, Stamp(ctx, &d, &Parent{EventID: eventA, TraceID: "4bf92f3577b34da6a3ce929d0e0e4736"}))
		return d
	}
	_, err := Check(stamped())
	require.NoError(t, err)

	ext := func(m map[string]any) map[string]any { return m["opena2a"].(map[string]any) }
	for name, mutate := range map[string]func(d *record.Draft){
		"no trace_id":                    func(d *record.Draft) { delete(d.Tenant, "trace_id") },
		"trace_id null":                  func(d *record.Draft) { d.Tenant["trace_id"] = nil },
		"trace_id all zeros":             func(d *record.Draft) { d.Tenant["trace_id"] = strings.Repeat("0", 32) },
		"trace_id uppercase":             func(d *record.Draft) { d.Tenant["trace_id"] = strings.ToUpper(callerTraceID) },
		"trace_id not a string":          func(d *record.Draft) { d.Tenant["trace_id"] = int64(7) },
		"trace_id in the retained part":  func(d *record.Draft) { d.Retained["trace_id"] = d.Tenant["trace_id"]; delete(d.Tenant, "trace_id") },
		"trace_id in the personal part":  func(d *record.Draft) { d.Personal["trace_id"] = d.Tenant["trace_id"]; delete(d.Tenant, "trace_id") },
		"no parent_id":                   func(d *record.Draft) { delete(d.Tenant, "parent_id") },
		"parent_id not a uuid":           func(d *record.Draft) { d.Tenant["parent_id"] = "A1A1A1A1-0000-4000-8000-00000000000A" },
		"parent_id is the record":        func(d *record.Draft) { d.Tenant["parent_id"] = eventB },
		"no trace_origin":                func(d *record.Draft) { delete(ext(d.Retained), "trace_origin") },
		"trace_origin caller":            func(d *record.Draft) { ext(d.Retained)["trace_origin"] = "caller" },
		"trace_origin server for child":  func(d *record.Draft) { ext(d.Retained)["trace_origin"] = OriginServer },
		"trace_origin parent for a root": func(d *record.Draft) { d.Tenant["parent_id"] = nil },
		"request trace in tenant part": func(d *record.Draft) {
			d.Tenant["opena2a"] = map[string]any{"request_trace_id": callerTraceID}
		},
		"request trace in retained part": func(d *record.Draft) { ext(d.Retained)["request_trace_id"] = callerTraceID },
		"request trace malformed":        func(d *record.Draft) { ext(d.Personal)["request_trace_id"] = callerHeader },
	} {
		d := stamped()
		mutate(&d)
		_, err := Check(d)
		require.ErrorIs(t, err, ErrInvalidTrace, name)
	}
}

func TestTraceIDOfPartReadsAStoredTenantPart(t *testing.T) {
	got, err := TraceIDOfPart([]byte(`{"parent_id":null,"trace_id":"` + callerTraceID + `"}`))
	require.NoError(t, err)
	require.Equal(t, callerTraceID, got)
	for _, bad := range []string{`{}`, `{"trace_id":null}`, `{"trace_id":"00000000000000000000000000000000"}`, `not json`} {
		_, err := TraceIDOfPart([]byte(bad))
		require.ErrorIs(t, err, ErrInvalidTrace, bad)
	}
}

// stampedApp answers each request with the trace members two drafts of that
// request were stamped with, and the request headers it saw.
func stampedApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(Middleware())
	app.Get("/", func(c fiber.Ctx) error {
		tc, ok := From(c.Context())
		if !ok {
			return fiber.ErrInternalServerError
		}
		first, second := draft(eventA), draft(eventB)
		if err := Stamp(c.Context(), &first, nil); err != nil {
			return err
		}
		if err := Stamp(c.Context(), &second, nil); err != nil {
			return err
		}
		return c.JSON(map[string]any{
			"context": fmt.Sprintf("%+v", tc),
			"drafts":  []record.Draft{first, second},
			"headers": c.GetReqHeaders(),
		})
	})
	return app
}

type stampedResponse struct {
	Context string              `json:"context"`
	Drafts  []record.Draft      `json:"drafts"`
	Headers map[string][]string `json:"headers"`
}

func get(t *testing.T, app *fiber.App, header http.Header) (stampedResponse, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for name, values := range header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body bytes.Buffer
	_, err = body.ReadFrom(res.Body)
	require.NoError(t, err)
	var out stampedResponse
	require.NoError(t, json.Unmarshal(body.Bytes(), &out))
	for name := range res.Header {
		require.NotEqual(t, "traceparent", strings.ToLower(name), "no response header hands a trace back")
	}
	return out, body.String()
}

func TestMiddlewareGivesOneRequestsRecordsItsOwnMintedTrace(t *testing.T) {
	app := stampedApp(t)
	first, _ := get(t, app, nil)
	second, _ := get(t, app, nil)

	require.Len(t, first.Drafts, 2)
	trace := first.Drafts[0].Tenant["trace_id"]
	require.True(t, ValidID(trace.(string)))
	require.Equal(t, trace, first.Drafts[1].Tenant["trace_id"], "one request's records share its trace")
	require.NotEqual(t, trace, second.Drafts[0].Tenant["trace_id"], "each request mints its own")
	require.Equal(t, second.Drafts[0].Tenant["trace_id"], second.Drafts[1].Tenant["trace_id"])
}

func TestMiddlewareNeverMakesTheCallersTraceparentATraceID(t *testing.T) {
	app := stampedApp(t)
	out, raw := get(t, app, http.Header{"Traceparent": {callerHeader}})

	for _, d := range out.Drafts {
		require.True(t, ValidID(d.Tenant["trace_id"].(string)))
		require.NotEqual(t, callerTraceID, d.Tenant["trace_id"])
		require.Equal(t, OriginServer, d.Retained["opena2a"].(map[string]any)["trace_origin"])
		require.Equal(t, map[string]any{"request_trace_id": callerTraceID}, d.Personal["opena2a"],
			"the caller's trace-id lands in the personal part, and only it")
		retained, err := json.Marshal(d.Retained)
		require.NoError(t, err)
		tenant, err := json.Marshal(d.Tenant)
		require.NoError(t, err)
		require.NotContains(t, string(retained), callerTraceID)
		require.NotContains(t, string(tenant), callerTraceID)
	}
	require.Contains(t, raw, callerTraceID, "control: the response carries the caller's trace-id where it was kept")
}

func TestMiddlewareDropsAnyOtherTraceparent(t *testing.T) {
	app := stampedApp(t)
	for name, values := range map[string][]string{
		"none":               nil,
		"version 01":         {"01-" + callerTraceID + "-" + callerParentID + "-01"},
		"all-zero parent-id": {"00-" + callerTraceID + "-0000000000000000-01"},
		"uppercase":          {strings.ToUpper(callerHeader)},
		"two headers":        {callerHeader, callerHeader},
	} {
		header := http.Header{}
		for _, v := range values {
			header.Add("Traceparent", v)
		}
		out, _ := get(t, app, header)
		for _, d := range out.Drafts {
			require.Nil(t, d.Personal, name)
			require.NotEqual(t, callerTraceID, d.Tenant["trace_id"], name)
		}
		require.Contains(t, out.Context, "RequestTraceID:}", name)
		// The header reached the handler, and nothing of it was kept.
		require.Len(t, out.Headers["Traceparent"], len(values), name)
	}
}

// Nothing but the trace-id field of traceparent is read from the request's
// trace context: canaries in tracestate, baggage and traceparent's own
// parent-id reach no record member and no log line.
func TestMiddlewareReadsNothingElseOfTheTraceContext(t *testing.T) {
	const (
		baggageCanary    = "canary-baggage-user-5d4c3b2a"
		tracestateCanary = "canary-tracestate-7f3e"
	)
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	app := stampedApp(t)
	out, _ := get(t, app, http.Header{
		"Traceparent": {callerHeader},
		"Tracestate":  {"vendor=" + tracestateCanary},
		"Baggage":     {"userId=" + baggageCanary},
	})

	// Positive control: the canaries were in the request as sent.
	require.Equal(t, []string{"userId=" + baggageCanary}, out.Headers["Baggage"])
	require.Equal(t, []string{"vendor=" + tracestateCanary}, out.Headers["Tracestate"])
	require.Equal(t, []string{callerHeader}, out.Headers["Traceparent"])

	members, err := json.Marshal(out.Drafts)
	require.NoError(t, err)
	for _, canary := range []string{baggageCanary, tracestateCanary, callerParentID} {
		require.NotContains(t, string(members), canary)
		require.NotContains(t, out.Context, canary)
		require.NotContains(t, logs.String(), canary)
	}
	require.Contains(t, string(members), callerTraceID, "control: the one field kept is in the members")
}

//go:build integration

package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/stretchr/testify/require"
)

const (
	callerTraceID  = "0af7651916cd43dd8448eb211c80319c"
	callerParentID = "b7ad6b7169203331"
	callerHeader   = "00-" + callerTraceID + "-" + callerParentID + "-01"
)

// storedRecord is one ledger row as stored, with its parts decoded.
type storedRecord struct {
	Seq          int64
	EventID      string
	Type         string
	Payload      []byte
	TenantPart   []byte
	PersonalPart []byte
	Retained     map[string]any
	Tenant       map[string]any
	Personal     map[string]any
}

func (r storedRecord) traceID() string {
	s, _ := r.Tenant["trace_id"].(string)
	return s
}

func (r storedRecord) parentID() (string, bool) {
	raw, present := r.Tenant["parent_id"]
	s, _ := raw.(string)
	return s, present
}

func (r storedRecord) ext(part map[string]any, name string) any {
	ext, _ := part["opena2a"].(map[string]any)
	return ext[name]
}

// readStored reads a chain's rows from the database, nothing else.
func readStored(t *testing.T, db *sql.DB, chainID string) []storedRecord {
	t.Helper()
	rows, err := db.Query(`SELECT seq, event_id, record_type, payload, tenant_part, personal_part
  FROM audit_records WHERE chain_id = $1 ORDER BY seq`, chainID)
	require.NoError(t, err)
	defer rows.Close()
	var out []storedRecord
	for rows.Next() {
		var r storedRecord
		require.NoError(t, rows.Scan(&r.Seq, &r.EventID, &r.Type, &r.Payload, &r.TenantPart, &r.PersonalPart))
		require.NoError(t, json.Unmarshal(r.Payload, &r.Retained))
		if r.TenantPart != nil {
			require.NoError(t, json.Unmarshal(r.TenantPart, &r.Tenant))
		}
		if r.PersonalPart != nil {
			require.NoError(t, json.Unmarshal(r.PersonalPart, &r.Personal))
		}
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func byEventID(records []storedRecord) map[string]storedRecord {
	out := map[string]storedRecord{}
	for _, r := range records {
		out[r.EventID] = r
	}
	return out
}

// typedDraft is a record of the given type with no trace members yet.
func typedDraft(recordType string, tenant, personal map[string]any) record.Draft {
	return record.Draft{
		EventID:  uuid.NewString(),
		Type:     recordType,
		Retained: map[string]any{"opena2a": map[string]any{"source": "service"}},
		Tenant:   tenant,
		Personal: personal,
	}
}

// serve runs one request through the trace middleware to handler.
func serve(t *testing.T, handler fiber.Handler, header http.Header) {
	t.Helper()
	app := fiber.New()
	app.Use(trace.Middleware())
	app.Get("/", handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for name, values := range header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer res.Body.Close()
	var body bytes.Buffer
	_, _ = body.ReadFrom(res.Body)
	require.Equal(t, http.StatusNoContent, res.StatusCode, body.String())
}

// write stamps d in the request's context, with parent when it is set, and
// appends it.
func write(c fiber.Ctx, h harness, org string, d record.Draft, parent *trace.Parent) (Appended, error) {
	if err := trace.Stamp(c.Context(), &d, parent); err != nil {
		return Appended{}, err
	}
	return h.w.Write(c.Context(), Write{Class: ClassExpansion, OrganizationID: org, Draft: d})
}

// Records of every type carry a trace_id and a parent reference, one
// request's records share the trace it minted, a record with a parent
// carries its parent's trace, and a trace is walked from the stored rows by
// those identifiers alone.
func TestRecordTraceIsWalkedFromStoredRecordsAlone(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)

	var genesis, a, b, c, d, e Appended
	var requestTrace [3]string

	// Request 0 starts the chain: the genesis is the first record of that
	// request's trace.
	serve(t, func(fc fiber.Ctx) error {
		tc, _ := trace.From(fc.Context())
		requestTrace[0] = tc.TraceID
		var err error
		genesis, err = h.w.start(fc.Context(), org)
		if err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, nil)

	// Request 1 writes two records that start no trace of their own: both
	// take the trace the request minted.
	serve(t, func(fc fiber.Ctx) error {
		tc, _ := trace.From(fc.Context())
		requestTrace[1] = tc.TraceID
		var err error
		if a, err = write(fc, h, org, typedDraft("opena2a.administrative", nil, nil), nil); err != nil {
			return err
		}
		if b, err = write(fc, h, org, typedDraft("action", map[string]any{"opena2a": map[string]any{
			"subject_agent_id": uuid.NewString()}}, nil), nil); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, nil)

	// Request 2 carries a caller's traceparent. Its child of b takes b's
	// trace; its other record takes the trace request 2 minted. Neither takes
	// the caller's.
	serve(t, func(fc fiber.Ctx) error {
		tc, _ := trace.From(fc.Context())
		requestTrace[2] = tc.TraceID
		var err error
		if c, err = write(fc, h, org, typedDraft("authorization_transition", nil,
			map[string]any{"actor": "user:" + uuid.NewString()}), b.AsParent()); err != nil {
			return err
		}
		if d, err = write(fc, h, org, typedDraft("opena2a.administrative", nil, nil), nil); err != nil {
			return err
		}
		if e, err = write(fc, h, org, typedDraft("delegation", nil, nil), c.AsParent()); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, http.Header{"Traceparent": {callerHeader}})

	// From here on, only stored rows are read.
	records := readStored(t, plain, genesis.ChainID)
	require.Len(t, records, 6)
	types := map[string]bool{}
	for _, r := range records {
		types[r.Type] = true
		require.True(t, trace.ValidID(r.traceID()), "%s at %d has no trace_id", r.Type, r.Seq)
		parentID, present := r.parentID()
		require.True(t, present, "%s at %d has no parent_id member", r.Type, r.Seq)
		if parentID == "" {
			require.Equal(t, trace.OriginServer, r.ext(r.Retained, "trace_origin"), "%s at %d", r.Type, r.Seq)
		} else {
			require.Equal(t, trace.OriginParent, r.ext(r.Retained, "trace_origin"), "%s at %d", r.Type, r.Seq)
		}
		require.NotEqual(t, callerTraceID, r.traceID(), "a traceparent never becomes a trace_id")
	}
	require.Equal(t, map[string]bool{record.TypeChainGenesis: true, "opena2a.administrative": true,
		"action": true, "authorization_transition": true, "delegation": true}, types)

	stored := byEventID(records)
	require.Equal(t, requestTrace[0], stored[genesis.EventID].traceID(), "the genesis takes its request's trace")
	require.Equal(t, requestTrace[1], stored[a.EventID].traceID(), "one request's records share its trace")
	require.Equal(t, requestTrace[1], stored[b.EventID].traceID(), "one request's records share its trace")
	require.Equal(t, requestTrace[2], stored[d.EventID].traceID())
	require.NotEqual(t, requestTrace[1], requestTrace[2])

	// Walk from e to the root of its trace by parent_id.
	var walk []string
	for at := stored[e.EventID]; ; {
		walk = append(walk, at.EventID)
		require.Equal(t, requestTrace[1], at.traceID(), "every record of the walk shares the root's trace")
		parentID, _ := at.parentID()
		if parentID == "" {
			break
		}
		next, ok := stored[parentID]
		require.True(t, ok, "parent %s of %s is in the chain", parentID, at.EventID)
		at = next
	}
	require.Equal(t, []string{e.EventID, c.EventID, b.EventID}, walk)

	// The trace groups the records of the run, across requests.
	byTrace := map[string][]string{}
	for _, r := range records {
		byTrace[r.traceID()] = append(byTrace[r.traceID()], r.EventID)
	}
	require.Equal(t, []string{a.EventID, b.EventID, c.EventID, e.EventID}, byTrace[requestTrace[1]])
	require.Equal(t, []string{d.EventID}, byTrace[requestTrace[2]])
	require.Equal(t, []string{genesis.EventID}, byTrace[requestTrace[0]])

	// The caller's trace-id is kept, in the personal part, on the records of
	// the request that sent it, and on no other.
	for _, id := range []string{c.EventID, d.EventID, e.EventID} {
		require.Equal(t, callerTraceID, stored[id].ext(stored[id].Personal, "request_trace_id"))
	}
	for _, id := range []string{genesis.EventID, a.EventID, b.EventID} {
		require.Nil(t, stored[id].ext(stored[id].Personal, "request_trace_id"))
	}

	// What was stored still verifies as one chain.
	chain, err := ReadChain(context.Background(), plain, genesis.ChainID)
	require.NoError(t, err)
	res, err := record.Verify(chain, h.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
	require.Equal(t, 6, res.Verified)
}

// A record whose parent is not a record of the organization's chain in the
// same trace is refused whole: its state change does not commit, and the
// refusal is counted with reason trace.
func TestRecordWriterRefusesAParentOutsideTheRecordsTrace(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org, other := seedOrg(t, plain), seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx, err := trace.Begin(context.Background())
	require.NoError(t, err)

	_, err = h.w.start(ctx, org)
	require.NoError(t, err)
	_, err = h.w.start(ctx, other)
	require.NoError(t, err)
	appendOne := func(organizationID string) Appended {
		t.Helper()
		got, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: organizationID, Draft: testDraft()})
		require.NoError(t, err)
		return got
	}
	parent, foreign, erased := appendOne(org), appendOne(other), appendOne(org)
	_, err = plain.Exec(`UPDATE audit_records SET tenant_part = NULL, tenant_salt = NULL WHERE event_id = $1`, erased.EventID)
	require.NoError(t, err)

	rename := func(name string) func(context.Context, *sql.Tx) error {
		return func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE organizations SET name = $1 WHERE id = $2`, name, org)
			return err
		}
	}
	nameOf := func() string {
		var name string
		require.NoError(t, plain.QueryRow(`SELECT name FROM organizations WHERE id = $1`, org).Scan(&name))
		return name
	}
	before := nameOf()

	otherTrace, err := trace.Mint()
	require.NoError(t, err)
	for name, p := range map[string]*trace.Parent{
		"parent names no record":          {EventID: uuid.NewString(), TraceID: parent.TraceID},
		"parent in another organization":  foreign.AsParent(),
		"trace_id not the parent's":       {EventID: parent.EventID, TraceID: otherTrace},
		"parent's tenant part was erased": erased.AsParent(),
	} {
		d := testDraft()
		require.NoError(t, trace.Stamp(ctx, &d, p), name)
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: d, Apply: rename("renamed-" + name)})
		wantWriteError(t, err, ClassExpansion, ReasonTrace)
		require.Equal(t, before, nameOf(), "%s: the state change did not commit", name)
	}
	one, total := h.failures(t, ClassExpansion, ReasonTrace)
	require.Equal(t, 4.0, one)
	require.Equal(t, 4.0, total)

	// Control: the same write with its parent in its trace commits.
	d := testDraft()
	require.NoError(t, trace.Stamp(ctx, &d, parent.AsParent()))
	child, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: d, Apply: rename("renamed-ok")})
	require.NoError(t, err)
	require.Equal(t, parent.TraceID, child.TraceID)
	require.Equal(t, "renamed-ok", nameOf())
}

// A guard may edit a draft's members, but not its trace members.
func TestRecordGuardCannotChangeTheTrace(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	other, err := trace.Mint()
	require.NoError(t, err)
	_, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
		Guard: func(_ context.Context, _ time.Time, _ *Probe, d *record.Draft) (*Stored, error) {
			d.Tenant["trace_id"] = other
			return nil, nil
		}})
	wantWriteError(t, err, ClassExpansion, ReasonGuard)
}

// The retained bytes, which no erasure touches, carry none of the
// identifiers planted through a request: not the trace_id, the parent_id or
// the caller's traceparent, and not the tenant and personal values beside
// them. The caller's tracestate and baggage reach no column and no log line.
// One identifier written twice gives two different commitments.
func TestRecordRetainedBytesCarryNoPlantedIdentifier(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	var stdLogs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&stdLogs)
	t.Cleanup(func() { log.SetOutput(prev) })

	const (
		baggageCanary    = "canary-baggage-user-5d4c3b2a"
		tracestateCanary = "canary-tracestate-7f3e"
	)
	resourceID, actor := uuid.NewString(), "user:"+uuid.NewString()
	var genesis, first, twin, child Appended
	var minted string
	serve(t, func(fc fiber.Ctx) error {
		tc, _ := trace.From(fc.Context())
		minted = tc.TraceID
		var err error
		if genesis, err = h.w.start(fc.Context(), org); err != nil {
			return err
		}
		tenant := func() map[string]any { return map[string]any{"opena2a": map[string]any{"resource_id": resourceID}} }
		personal := func() map[string]any { return map[string]any{"actor": actor} }
		if first, err = write(fc, h, org, typedDraft("opena2a.administrative", tenant(), personal()), nil); err != nil {
			return err
		}
		if twin, err = write(fc, h, org, typedDraft("opena2a.administrative", tenant(), personal()), nil); err != nil {
			return err
		}
		if child, err = write(fc, h, org, typedDraft("action", nil, nil), first.AsParent()); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, http.Header{
		"Traceparent": {callerHeader},
		"Tracestate":  {"vendor=" + tracestateCanary},
		"Baggage":     {"userId=" + baggageCanary},
	})

	// rowsWith counts the chain's rows whose column holds needle.
	rowsWith := func(column, needle string) int {
		t.Helper()
		var n int
		require.NoError(t, plain.QueryRow(`SELECT count(*) FROM audit_records
 WHERE chain_id = $1 AND position(convert_to($2, 'UTF8') IN coalesce(`+column+`, ''::bytea)) > 0`,
			genesis.ChainID, needle).Scan(&n))
		return n
	}

	// Positive control: the scan finds each planted value where it belongs.
	require.Equal(t, 4, rowsWith("tenant_part", minted), "trace_id sits in every record's tenant part")
	require.Equal(t, 2, rowsWith("tenant_part", resourceID))
	require.Equal(t, 1, rowsWith("tenant_part", first.EventID), "the child's parent_id")
	require.Equal(t, 4, rowsWith("personal_part", callerTraceID), "the caller's trace-id, in the personal part")
	require.Equal(t, 2, rowsWith("personal_part", actor))
	require.Equal(t, 1, rowsWith("payload", child.EventID), "the scan reads the retained bytes")

	for _, planted := range []string{minted, callerTraceID, resourceID, actor} {
		require.Zero(t, rowsWith("payload", planted), "%s is in the retained bytes", planted)
	}
	// first's event_id is first's own retained member; as the child's
	// parent_id it is never in the child's retained bytes.
	var childPayload []byte
	require.NoError(t, plain.QueryRow(`SELECT payload FROM audit_records WHERE chain_id = $1 AND event_id = $2`,
		genesis.ChainID, child.EventID).Scan(&childPayload))
	require.NotContains(t, string(childPayload), first.EventID)

	for _, canary := range []string{baggageCanary, tracestateCanary, callerParentID} {
		for _, column := range []string{"payload", "tenant_part", "personal_part"} {
			require.Zero(t, rowsWith(column, canary), "%s is in %s", canary, column)
		}
		require.NotContains(t, stdLogs.String(), canary)
		require.NotContains(t, strings.Join(h.logs.lines(), "\n"), canary)
	}

	// One identifier written twice gives two different commitments.
	stored := byEventID(readStored(t, plain, genesis.ChainID))
	a, b := stored[first.EventID], stored[twin.EventID]
	require.Equal(t, a.TenantPart, b.TenantPart, "the two tenant parts are the same bytes")
	require.Equal(t, a.PersonalPart, b.PersonalPart, "the two personal parts are the same bytes")
	commitments := func(r storedRecord) map[string]any {
		return r.ext(r.Retained, "commitments").(map[string]any)
	}
	require.NotEqual(t, commitments(a)["tenant"], commitments(b)["tenant"])
	require.NotEqual(t, commitments(a)["personal"], commitments(b)["personal"])
}

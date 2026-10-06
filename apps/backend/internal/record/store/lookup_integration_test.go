//go:build integration

package store

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/stretchr/testify/require"
)

// A job run that writes records for two organizations gives each
// organization's chain its own trace: the genesis and every record the run
// writes for one organization share it, and no record of one chain carries
// the other's.
func TestRecordJobRunMintsOneTracePerOrganization(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	orgA, orgB := seedOrg(t, plain), seedOrg(t, plain)

	run := trace.NewRun()
	chains := map[string]string{}
	for _, org := range []string{orgA, orgB, orgA, orgB} {
		ctx, err := run.For(context.Background(), org)
		require.NoError(t, err)
		if _, started := chains[org]; !started {
			genesis, err := h.w.start(ctx, org)
			require.NoError(t, err)
			chains[org] = genesis.ChainID
		}
		d := record.Draft{
			EventID:  uuid.NewString(),
			Type:     "opena2a.administrative",
			Retained: map[string]any{"opena2a": map[string]any{"source": "system"}},
		}
		require.NoError(t, trace.Stamp(ctx, &d, nil))
		_, err = h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: d})
		require.NoError(t, err)
	}

	// From here on, only stored rows are read.
	traces := map[string]string{}
	for org, chainID := range chains {
		records := readStored(t, plain, chainID)
		require.Len(t, records, 3, "the genesis and two records")
		traces[org] = records[0].traceID()
		require.True(t, trace.ValidID(traces[org]))
		for _, r := range records {
			require.Equal(t, traces[org], r.traceID(), "one organization's records of the run share its trace")
			parentID, present := r.parentID()
			require.True(t, present)
			require.Empty(t, parentID)
			require.Equal(t, trace.OriginServer, r.ext(r.Retained, "trace_origin"))
		}
		chain, err := ReadChain(context.Background(), plain, chainID)
		require.NoError(t, err)
		res, err := record.Verify(chain, h.keys.publicKey())
		require.NoError(t, err)
		require.True(t, res.OK(), "verification failed: %v", res.Failure)
	}
	require.NotEqual(t, traces[orgA], traces[orgB], "no trace_id is shared by two organizations' chains")
}

// eventIDs returns the event_id of each record, after checking that it is
// signed by key and that its personal part carries requestTraceID.
func eventIDs(t *testing.T, records []record.Record, key record.PublicKey, requestTraceID string) []string {
	t.Helper()
	out := []string{}
	for _, rec := range records {
		payload, err := record.Open(rec.Envelope, record.ClassRecordV1, keyVerifier{key})
		require.NoError(t, err)
		var retained struct {
			EventID string `json:"event_id"`
		}
		require.NoError(t, json.Unmarshal(payload, &retained))
		var personal map[string]any
		require.NoError(t, json.Unmarshal(rec.PersonalPart, &personal))
		require.Equal(t, requestTraceID, personal["opena2a"].(map[string]any)["request_trace_id"])
		out = append(out, retained.EventID)
	}
	return out
}

// A caller matches its own trace to the records through the request trace,
// inside its own organization only. Another organization's value returns
// nothing, and a record whose personal part is erased is no longer found.
func TestRecordLookupByRequestTraceIsScopedToTheOrganization(t *testing.T) {
	const (
		otherTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		otherHeader  = "00-" + otherTraceID + "-00f067aa0ba902b7-01"
		unknownTrace = "5ce0e9a56015fec5aadfa328ae398115"
	)
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	orgA, orgB, orgNoChain := seedOrg(t, plain), seedOrg(t, plain), seedOrg(t, plain)
	ctx := context.Background()
	var chainA string
	for _, org := range []string{orgA, orgB} {
		started, err := trace.Begin(ctx)
		require.NoError(t, err)
		genesis, err := h.w.start(started, org)
		require.NoError(t, err)
		if org == orgA {
			chainA = genesis.ChainID
		}
	}

	var a1, a2, a3, a4, b1 Appended
	// Organization A, a request that sends the caller's traceparent: a record
	// and its child.
	serve(t, func(fc fiber.Ctx) error {
		var err error
		if a1, err = write(fc, h, orgA, typedDraft("action", nil, nil), nil); err != nil {
			return err
		}
		if a2, err = write(fc, h, orgA, typedDraft("authorization_transition", nil,
			map[string]any{"actor": "user:" + uuid.NewString()}), a1.AsParent()); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, http.Header{"Traceparent": {callerHeader}})
	// Organization A, a request with no traceparent.
	serve(t, func(fc fiber.Ctx) error {
		var err error
		if a3, err = write(fc, h, orgA, typedDraft("opena2a.administrative", nil, nil), nil); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, nil)
	// Organization A, a request with another caller trace.
	serve(t, func(fc fiber.Ctx) error {
		var err error
		if a4, err = write(fc, h, orgA, typedDraft("opena2a.administrative", nil, nil), nil); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, http.Header{"Traceparent": {otherHeader}})
	// Organization B, a request that sends the same traceparent as A's first.
	serve(t, func(fc fiber.Ctx) error {
		var err error
		if b1, err = write(fc, h, orgB, typedDraft("action", nil, nil), nil); err != nil {
			return err
		}
		return fc.SendStatus(http.StatusNoContent)
	}, http.Header{"Traceparent": {callerHeader}})

	key := h.keys.publicKey()
	lookup := func(org, requestTraceID string) []string {
		t.Helper()
		records, err := RecordsByRequestTrace(ctx, plain, org, requestTraceID)
		require.NoError(t, err)
		return eventIDs(t, records, key, requestTraceID)
	}

	require.Equal(t, []string{a1.EventID, a2.EventID}, lookup(orgA, callerTraceID),
		"the records of the requests that sent the trace, in chain order")
	require.Equal(t, []string{a4.EventID}, lookup(orgA, otherTraceID))
	require.NotContains(t, lookup(orgA, callerTraceID), a3.EventID, "a request without a traceparent is not matched")
	require.Equal(t, []string{b1.EventID}, lookup(orgB, callerTraceID),
		"a value two organizations' callers sent is found in each only for its own records")

	// Negative: organization A's value returns nothing in organization B.
	require.Empty(t, lookup(orgB, otherTraceID))
	require.Empty(t, lookup(orgA, unknownTrace))
	require.Empty(t, lookup(orgNoChain, callerTraceID))

	for name, bad := range map[string][2]string{
		"uppercase trace":      {orgA, strings.ToUpper(callerTraceID)},
		"all-zero trace":       {orgA, strings.Repeat("0", 32)},
		"traceparent header":   {orgA, callerHeader},
		"empty trace":          {orgA, ""},
		"malformed org":        {"org-1", callerTraceID},
		"uppercase org":        {strings.ToUpper(orgA), callerTraceID},
		"empty org":            {"", callerTraceID},
		"trace id as org uuid": {callerTraceID, callerTraceID},
	} {
		_, err := RecordsByRequestTrace(ctx, plain, bad[0], bad[1])
		require.ErrorIs(t, err, ErrInvalidWrite, name)
	}

	// The value is read from the personal part itself: once a record's
	// personal part is erased, the lookup no longer finds it, and the chain
	// still verifies.
	res, err := plain.Exec(`UPDATE audit_records SET personal_part = NULL, personal_salt = NULL
 WHERE chain_id = $1 AND event_id = $2`, chainA, a1.EventID)
	require.NoError(t, err)
	n, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	require.Equal(t, []string{a2.EventID}, lookup(orgA, callerTraceID))
	chain, err := ReadChain(ctx, plain, chainA)
	require.NoError(t, err)
	verified, err := record.Verify(chain, key)
	require.NoError(t, err)
	require.True(t, verified.OK(), "verification failed: %v", verified.Failure)
}

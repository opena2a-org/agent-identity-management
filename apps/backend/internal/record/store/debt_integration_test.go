//go:build integration

package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/stretchr/testify/require"
)

// A planted head that cannot be extended refuses that organization's
// expansions with reason chain_head, lets its reductions commit with debt
// rows, and leaves other organizations unaffected.
func TestRecordReductionCommitsWithADebtWhileTheHeadCannotBeExtended(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	planted, other := seedOrg(t, plain), seedOrg(t, plain)
	for _, org := range []string{planted, other} {
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
	}
	breakHead(t, plain, planted)

	_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: planted, Draft: testDraft(), Apply: markOrg(planted)})
	wantWriteError(t, err, ClassExpansion, ReasonChainHead)
	require.False(t, marked(t, plain, planted), "a refused expansion's state change rolls back")
	require.Empty(t, debtRows(t, plain, planted), "an expansion leaves no debt")

	draft := testDraft()
	out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: planted, Draft: draft, Apply: markOrg(planted)})
	require.NoError(t, err)
	require.Equal(t, &Debt{ID: draft.EventID, Reason: ReasonChainHead}, out.Debt)
	require.True(t, marked(t, plain, planted), "the reduction's state change committed")
	rows := debtRows(t, plain, planted)
	require.Len(t, rows, 1)
	require.Contains(t, rows, draft.EventID)
	require.Equal(t, 1, positions(t, plain, planted), "nothing was appended after the genesis")
	one, _ := h.failures(t, ClassReduction, ReasonChainHead)
	require.Equal(t, 1.0, one)
	require.Equal(t, 1.0, h.counter(t, "aim_record_debts_written_total"))
	require.Equal(t, PathUnavailable, h.w.RecordPath(time.Now()).State)

	for _, class := range []Class{ClassExpansion, ClassReduction} {
		got, err := h.w.Write(ctx, Write{Class: class, OrganizationID: other, Draft: testDraft(), Apply: markOrg(other)})
		require.NoError(t, err)
		require.Nil(t, got.Debt)
		require.True(t, marked(t, plain, other))
		unmark(t, plain, other)
	}
	require.Empty(t, debtRows(t, plain, other))
	require.Equal(t, 3, positions(t, plain, other))
}

// Per reason, a reduction commits with exactly one debt row and no new
// position, and an expansion leaves no change, no debt and no position. A
// state change that fails, or that its caller rolls back, leaves no debt.
func TestRecordReductionDebtPerReason(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	ctx := context.Background()
	keys := newTestKeys(t)

	type fault struct {
		reason Reason
		inject func(t *testing.T, h harness, org string) (undo func())
		guard  Guard
	}
	faults := []fault{
		{reason: ReasonChainHead, inject: func(t *testing.T, _ harness, org string) func() {
			return breakHead(t, plain, org)
		}},
		{reason: ReasonSigner, inject: func(_ *testing.T, h harness, _ string) func() {
			h.keys.fail = errors.New("injected signer fault")
			return func() { h.keys.fail = nil }
		}},
		{reason: ReasonLockTimeout, inject: func(t *testing.T, _ harness, org string) func() {
			holder, err := plain.Begin()
			require.NoError(t, err)
			_, err = holder.Exec(lockChainQuery, org)
			require.NoError(t, err)
			return func() { _ = holder.Rollback() }
		}},
		{reason: ReasonDatabase, inject: func(*testing.T, harness, string) func() {
			tp.blockBefore(chainStateQuery, StatementTimeout+200*time.Millisecond)
			return func() { tp.blockBefore(chainStateQuery, 0) }
		}},
		{reason: ReasonGuard, inject: func(*testing.T, harness, string) func() { return func() {} },
			guard: func(context.Context, time.Time, *Probe, *record.Draft) (*Stored, error) {
				return nil, errors.New("refused by the guard")
			}},
	}
	for _, f := range faults {
		for _, class := range []Class{ClassExpansion, ClassReduction} {
			t.Run(string(f.reason)+"/"+string(class), func(t *testing.T) {
				h := newHarness(t, db, keys)
				org := seedOrg(t, plain)
				_, err := h.w.start(ctx, org)
				require.NoError(t, err)
				undo := f.inject(t, h, org)
				out, err := h.w.Write(ctx, Write{Class: class, OrganizationID: org, Draft: testDraft(),
					Apply: markOrg(org), Guard: f.guard})
				undo()
				if class == ClassExpansion {
					wantWriteError(t, err, class, f.reason)
					require.False(t, marked(t, plain, org))
					require.Empty(t, debtRows(t, plain, org))
				} else {
					require.NoError(t, err)
					require.NotNil(t, out.Debt)
					require.Equal(t, f.reason, out.Debt.Reason)
					require.True(t, marked(t, plain, org))
					require.Len(t, debtRows(t, plain, org), 1)
				}
				require.Equal(t, 1, positions(t, plain, org), "no position after the genesis")
			})
		}
	}

	t.Run("caller_rollback", func(t *testing.T) {
		h := newHarness(t, db, keys)
		org := seedOrg(t, plain)
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
		breakHead(t, plain, org)
		refused := errors.New("state change refused")
		_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(),
			Apply: func(ctx context.Context, tx *sql.Tx) error {
				if err := markOrg(org)(ctx, tx); err != nil {
					return err
				}
				return refused
			}})
		require.ErrorIs(t, err, refused)
		require.False(t, marked(t, plain, org))
		require.Empty(t, debtRows(t, plain, org))
		_, total := h.failures(t, ClassReduction, ReasonChainHead)
		require.Equal(t, 0.0, total, "a failed state change is not a failed record write")
	})
}

// A reduction whose debt row cannot be written either still commits, with
// no record and no debt, and its SECURITY line says so: debt=unwritten, which
// no refused write's line carries.
func TestRecordReductionWhoseDebtCannotBeWrittenStillCommits(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	restore := breakHead(t, plain, org)

	first := testDraft()
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: first})
	require.NoError(t, err)
	require.Len(t, debtRows(t, plain, org), 1)
	before := debtRows(t, plain, org)

	// The same event id again: the append still fails, and so does the debt
	// row, whose id is taken.
	again := testDraft()
	again.EventID = first.EventID
	h = newHarness(t, db, h.keys)
	out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: again, Apply: markOrg(org)})
	require.NoError(t, err)
	require.Equal(t, &Debt{ID: "", Reason: ReasonChainHead}, out.Debt)
	require.True(t, marked(t, plain, org), "the reduction committed")
	require.Equal(t, before, debtRows(t, plain, org), "the first debt is unchanged")
	require.Equal(t, 0.0, h.counter(t, "aim_record_debts_written_total"))
	require.Equal(t, []string{"SECURITY record_write_failed class=reduction reason=chain_head debt=unwritten"}, h.logs.lines())
	restore()
}

// The settler appends each debt's late record, built from the row alone,
// and deletes the row in the same transaction. The chain verifies.
func TestRecordSettlerAppendsTheLateRecordAndDeletesTheDebt(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)
	restore := breakHead(t, plain, org)
	draft := testDraft()
	occurred := time.Date(2026, time.October, 6, 9, 8, 7, 654321000, time.UTC)
	draft.Timestamp = occurred
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: draft})
	require.NoError(t, err)

	s := NewSettler(h.w)
	run := s.Run(ctx)
	require.True(t, run.Success)
	require.Zero(t, run.Settled, "a debt does not settle while its chain cannot be extended")
	require.GreaterOrEqual(t, run.Open, int64(1))
	require.Len(t, debtRows(t, plain, org), 1, "a failed settle leaves the debt as it was")

	restore()
	run = s.Run(ctx)
	require.True(t, run.Success)
	require.GreaterOrEqual(t, run.Settled, 1)
	require.Empty(t, debtRows(t, plain, org))
	require.Equal(t, 2, positions(t, plain, org))

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	res, err := record.Verify(records, h.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)

	retained, tenant, personal := recordParts(t, records[1])
	require.Equal(t, draft.EventID, retained["event_id"])
	ext := retained["opena2a"].(map[string]any)
	require.Equal(t, true, ext["late"])
	require.Equal(t, "2026-10-06T09:08:07.654321Z", ext["occurred_at"])
	require.Equal(t, "tag_created", ext["admin_action"])
	require.Equal(t, org, tenant["opena2a"].(map[string]any)["organization_id"])
	require.Equal(t, draft.Tenant["opena2a"].(map[string]any)["resource_id"], tenant["opena2a"].(map[string]any)["resource_id"])
	require.Equal(t, draft.Personal["actor"], personal["actor"])

	require.GreaterOrEqual(t, h.counter(t, "aim_record_debts_settled_total"), 1.0)
	require.Greater(t, h.value(t, "aim_record_debt_settler_last_success_timestamp_seconds"), 0.0)
}

// recordParts decodes a stored record's three parts.
func recordParts(t *testing.T, rec record.Record) (retained, tenant, personal map[string]any) {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString(rec.Envelope.Payload)
	require.NoError(t, err)
	decode := func(b []byte) map[string]any {
		if len(b) == 0 {
			return nil
		}
		v, err := decodeCanonicalJSON(string(b))
		require.NoError(t, err)
		return v.(map[string]any)
	}
	return decode(payload), decode(rec.TenantPart), decode(rec.PersonalPart)
}

// Every column of record_debts becomes exactly one member of the late
// record, in its part, and the late record carries nothing else but the
// record package's own members, the two that mark it late, and
// opena2a.trace_origin, which follows from parent_id.
func TestRecordDebtColumnsAreTheLateRecordsMembers(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)

	rows, err := plain.Query(`SELECT column_name FROM information_schema.columns
 WHERE table_schema = current_schema() AND table_name = 'record_debts' ORDER BY ordinal_position`)
	require.NoError(t, err)
	var columns []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		columns = append(columns, name)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	var listed []string
	for _, c := range debtColumns {
		listed = append(listed, c.name)
	}
	require.Equal(t, listed, columns, "record_debts has exactly the columns the late record is built from")

	restore := breakHead(t, plain, org)
	drafts := []record.Draft{agentDebtDraft(), operatorDebtDraft()}
	// The agent's draft follows the genesis in its trace; the operator's is
	// the first record of a trace of its own, so its parent_id is null.
	parents := []*trace.Parent{genesis.AsParent(), nil}
	for i := range drafts {
		drafts[i].EventID = uuid.NewString()
		if ext, ok := drafts[i].Tenant["opena2a"].(map[string]any); ok {
			ext = cloneMembers(ext)
			ext["organization_id"] = org
			drafts[i].Tenant["opena2a"] = ext
		}
		traced, err := trace.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, trace.Stamp(traced, &drafts[i], parents[i]))
		out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: drafts[i]})
		require.NoError(t, err)
		require.NotEmpty(t, out.Debt.ID)
	}
	stored := debtRows(t, plain, org)
	require.Len(t, stored, 2)
	seen := map[string]bool{}
	byID := map[string]map[string]any{}
	for id, raw := range stored {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &row))
		byID[id] = row
		for name, v := range row {
			if v != nil {
				seen[name] = true
			}
		}
	}
	require.Len(t, seen, len(debtColumns), "the two debts set every column between them")

	restore()
	run := NewSettler(h.w).Run(ctx)
	require.True(t, run.Success)
	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 3)

	own := map[string]bool{"event_id": true, "type": true, "timestamp": true, "opena2a.schema": true,
		"opena2a.chain": true, "opena2a.commitments": true, "opena2a.sig_algs": true, "opena2a.late": true}
	for _, rec := range records[1:] {
		retained, tenant, personal := recordParts(t, rec)
		row := byID[retained["event_id"].(string)]
		require.NotNil(t, row, "a late record for each debt")
		members := map[string]memberAt{}
		for part, m := range map[string]map[string]any{partRetained: retained, partTenant: tenant, partPersonal: personal} {
			flatten(part, m, members)
		}
		resourceType, _ := row["resource_type"].(string)
		for _, c := range debtColumns {
			value := row[c.name]
			got, present := members[c.member]
			if value == nil {
				if c.name == "actor" || c.name == "actor_class" {
					continue // both become actor; the other one may set it
				}
				if c.name == "parent_id" && row["trace_id"] != nil {
					// The first record of a trace carries parent_id null.
					require.True(t, present, "a traced record carries parent_id")
					require.Nil(t, got.value, "a NULL parent_id is a null parent_id")
					delete(members, c.member)
					continue
				}
				require.False(t, present, "%s is NULL, so %s is absent", c.name, c.member)
				continue
			}
			require.True(t, present, "%s becomes %s", c.name, c.member)
			require.Equal(t, c.partFor(resourceType), got.part, "%s sits in its part", c.member)
			require.Equal(t, columnText(t, c, value), memberText(t, got.value), "%s carries %s", c.member, c.name)
			delete(members, c.member)
		}
		// opena2a.trace_origin is no column: it follows from parent_id.
		origin := members["opena2a.trace_origin"]
		require.Equal(t, partRetained, origin.part)
		require.Equal(t, traceOrigin(row["parent_id"]), origin.value, "trace_origin follows parent_id")
		delete(members, "opena2a.trace_origin")
		for name := range own {
			delete(members, name)
		}
		require.Empty(t, members, "every member of the late record comes from a column or is the package's own")
	}
}

type memberAt struct {
	part  string
	value any
}

func flatten(part string, m map[string]any, into map[string]memberAt) {
	for k, v := range m {
		if k == "opena2a" {
			for ek, ev := range v.(map[string]any) {
				into["opena2a."+ek] = memberAt{part: part, value: ev}
			}
			continue
		}
		into[k] = memberAt{part: part, value: v}
	}
}

// columnText renders a column's value as row_to_json gave it, in the form
// the record carries it.
func columnText(t *testing.T, c debtColumn, v any) string {
	t.Helper()
	if c.name == "occurred_at" {
		at, err := time.Parse(time.RFC3339Nano, v.(string))
		require.NoError(t, err)
		s, err := record.FormatTimestamp(at)
		require.NoError(t, err)
		return memberText(t, s)
	}
	return memberText(t, v)
}

func memberText(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(normalize(v))
	require.NoError(t, err)
	return string(b)
}

// normalize turns every number into a float64, the form encoding/json
// decodes into, so values decoded two ways compare equal.
func normalize(v any) any {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalize(x[i])
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k := range x {
			out[k] = normalize(x[k])
		}
		return out
	}
	return v
}

// Each settler and writer statement on record_debts names one
// organization: A's settle and read leave B's planted debt unchanged and
// unreturned, and B's run reads and settles it.
func TestRecordSettlerReadsOneOrganizationAtATime(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	orgA, orgB := seedOrg(t, plain), seedOrg(t, plain)
	restore := map[string]func(){}
	for _, org := range []string{orgA, orgB} {
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
		restore[org] = breakHead(t, plain, org)
		_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft()})
		require.NoError(t, err)
	}
	restore[orgA]()
	restore[orgB]()
	restore[orgB] = breakHead(t, plain, orgB) // B's chain stays unsettleable until its own run
	bBefore := debtRows(t, plain, orgB)
	require.Len(t, bBefore, 1)

	s := NewSettler(h.w)
	tp.reset()
	settled, err := s.settleOrganization(ctx, orgA)
	require.NoError(t, err)
	require.Equal(t, 1, settled)
	open, _, err := s.openDebts(ctx, orgA)
	require.NoError(t, err)
	require.Zero(t, open)
	require.Equal(t, bBefore, debtRows(t, plain, orgB), "A's settle leaves B's debt as it was")
	for _, statement := range tp.recorded() {
		if strings.Contains(statement, "record_debts") {
			require.Contains(t, statement, "organization_id = $1", statement)
		}
	}

	var bID string
	for id := range bBefore {
		bID = id
	}
	ids, err := plain.Query(openDebtIDsQuery, orgA, settleBatch)
	require.NoError(t, err)
	for ids.Next() {
		var id string
		require.NoError(t, ids.Scan(&id))
		require.NotEqual(t, bID, id, "A's read does not return B's debt")
	}
	ids.Close()

	// Positive control: B's own run reads its debt and, once its chain can
	// be extended, settles it.
	open, _, err = s.openDebts(ctx, orgB)
	require.NoError(t, err)
	require.Equal(t, int64(1), open)
	restore[orgB]()
	settled, err = s.settleOrganization(ctx, orgB)
	require.NoError(t, err)
	require.Equal(t, 1, settled)
	require.Empty(t, debtRows(t, plain, orgB))
}

// Two settlers running at once append one late record per debt.
func TestRecordTwoSettlersGiveOneLateRecordPerDebt(t *testing.T) {
	plain := openPlain(t)
	ctx := context.Background()
	keys := newTestKeys(t)
	dbA, _ := openTapped(t, 0)
	dbB, _ := openTapped(t, 0)
	a, b := newHarness(t, dbA, keys), newHarness(t, dbB, keys)
	org := seedOrg(t, plain)
	genesis, err := a.w.start(ctx, org)
	require.NoError(t, err)
	restore := breakHead(t, plain, org)
	const debts = 6
	for i := 0; i < debts; i++ {
		_, err := a.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft()})
		require.NoError(t, err)
	}
	restore()

	var wg sync.WaitGroup
	for _, h := range []harness{a, b} {
		wg.Add(1)
		go func(h harness) {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				_, _ = NewSettler(h.w).settleOrganization(ctx, org)
			}
		}(h)
	}
	wg.Wait()

	require.Empty(t, debtRows(t, plain, org))
	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 1+debts, "one late record per debt")
	res, err := record.Verify(records, keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
}

// The settler's line carries its four fields and no organization; the
// open-debt series carry no label. The lock-wait line names no
// organization either.
func TestRecordSettlerLineAndSeriesNameNoOrganization(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	breakHead(t, plain, org)
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)

	h2 := newHarness(t, db, h.keys)
	run := NewSettler(h2.w).Run(ctx)
	h2.w.EmitLockWaits()
	lines := h2.logs.lines()
	var settlerLine, lockLine string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, EventDebtSettlerRun+" "):
			settlerLine = line
		case strings.HasPrefix(line, EventAppendLockWaits+" "):
			lockLine = line
		}
	}
	require.Equal(t, fmt.Sprintf("%s open=%d oldest_open_seconds=%.3f settled=%d success=%t",
		EventDebtSettlerRun, run.Open, run.OldestOpenSeconds, run.Settled, run.Success), settlerLine)
	require.GreaterOrEqual(t, run.Open, int64(1))
	require.Greater(t, run.OldestOpenSeconds, 0.0)
	require.NotEmpty(t, lockLine)
	for _, line := range []string{settlerLine, lockLine} {
		require.NotContains(t, line, org)
	}
	require.Equal(t, float64(run.Open), h2.value(t, "aim_record_debts_open"))
	require.Equal(t, run.OldestOpenSeconds, h2.value(t, "aim_record_debt_oldest_open_seconds"))

	families, err := h2.reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				require.Contains(t, []string{"class", "reason"}, l.GetName(), "%s has label %s", f.GetName(), l.GetName())
			}
		}
	}
}

// A reduction and a settle send as many statements under the append lock as
// an expansion: the savepoint and the debt's delete come before the lock.
func TestRecordStatementsUnderTheAppendLockForReductionsAndSettles(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	underLock := func(t *testing.T) []string {
		t.Helper()
		got := tp.recorded()
		at := -1
		for i, s := range got {
			if s == lockChainQuery {
				at = i
			}
		}
		require.GreaterOrEqual(t, at, 0, "no lock statement was sent")
		return got[at+1:]
	}

	tp.reset()
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(), Apply: markOrg(org)})
	require.NoError(t, err)
	statements := underLock(t)
	t.Logf("reduction: %d statements after the lock: %v", len(statements), statementLabels(statements))
	require.Equal(t, []string{setStatementTimeoutQuery, chainStateQuery, appendQuery, "COMMIT"}, statements)

	restore := breakHead(t, plain, org)
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)
	restore()
	tp.reset()
	settled, err := NewSettler(h.w).settleOrganization(ctx, org)
	require.NoError(t, err)
	require.Equal(t, 1, settled)
	statements = underLock(t)
	t.Logf("settle: %d statements after the lock: %v", len(statements), statementLabels(statements))
	require.Equal(t, []string{setStatementTimeoutQuery, chainStateQuery, appendQuery, "COMMIT"}, statements)
}

// A reduction that finds no free connection slot of its chain within the
// lock timeout takes a debt slot instead, and commits with a debt without
// waiting for the append lock.
func TestRecordReductionWithNoFreeSlotCommitsWithADebt(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	const hold = `SELECT pg_sleep(3)`
	var wg sync.WaitGroup
	for i := 0; i < connectionsPerChain; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
				Apply: func(ctx context.Context, tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, hold)
					return err
				}})
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var running int
		require.NoError(t, plain.QueryRow(`SELECT count(*) FROM pg_stat_activity
 WHERE datname = current_database() AND query = $1`, hold).Scan(&running))
		if running == connectionsPerChain {
			break
		}
		require.True(t, time.Now().Before(deadline), "the slot holders did not start")
		time.Sleep(10 * time.Millisecond)
	}

	started := time.Now()
	out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(), Apply: markOrg(org)})
	elapsed := time.Since(started)
	require.NoError(t, err)
	require.NotNil(t, out.Debt)
	require.NotEmpty(t, out.Debt.ID)
	require.Equal(t, ReasonLockTimeout, out.Debt.Reason)
	require.True(t, marked(t, plain, org))
	require.Len(t, debtRows(t, plain, org), 1)
	require.Less(t, elapsed, LockTimeout+time.Second, "the debt path does not wait for the append lock")
	wg.Wait()
}

// A reduction that sets NoDebt is written like any other class: a draft no
// debt row can hold is appended, and when the append fails the write is
// refused whole, leaving no debt and no state change for its caller to undo.
func TestRecordReductionWithNoDebtIsRefusedWholeWhenTheAppendFails(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	unholdable := func() record.Draft {
		d := testDraft()
		d.Personal["email"] = "someone@example.com"
		return d
	}
	out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: unholdable(), NoDebt: true})
	require.NoError(t, err, "a draft no debt can hold is appended when the write owes no debt")
	require.Nil(t, out.Debt)
	require.Equal(t, 2, positions(t, plain, org))

	breakHead(t, plain, org)
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: unholdable(), NoDebt: true,
		Apply: markOrg(org)})
	wantWriteError(t, err, ClassReduction, ReasonChainHead)
	require.False(t, marked(t, plain, org), "the state change rolls back with the record")
	require.Empty(t, debtRows(t, plain, org))
}

// A reduction on the debt path is held to the append path's trace rule: when
// it names a parent that is no record of the organization's chain, it is
// refused whole before its state change runs, and no debt is written.
func TestRecordReductionWithNoFreeSlotAndAnUnknownParentIsRefused(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)

	const hold = `SELECT pg_sleep(3.1)`
	var wg sync.WaitGroup
	for i := 0; i < connectionsPerChain; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
				Apply: func(ctx context.Context, tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, hold)
					return err
				}})
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var running int
		require.NoError(t, plain.QueryRow(`SELECT count(*) FROM pg_stat_activity
 WHERE datname = current_database() AND query = $1`, hold).Scan(&running))
		if running == connectionsPerChain {
			break
		}
		require.True(t, time.Now().Before(deadline), "the slot holders did not start")
		time.Sleep(10 * time.Millisecond)
	}

	draft := testDraft()
	unknown := &trace.Parent{EventID: uuid.NewString(), TraceID: genesis.TraceID}
	require.NoError(t, trace.Stamp(ctx, &draft, unknown))
	applied := false
	_, err = h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: draft,
		Apply: func(context.Context, *sql.Tx) error { applied = true; return nil }})
	wantWriteError(t, err, ClassReduction, ReasonTrace)
	require.False(t, applied, "the state change does not run")
	require.Empty(t, debtRows(t, plain, org), "no debt is owed for a record that cannot be written")
	wg.Wait()
}

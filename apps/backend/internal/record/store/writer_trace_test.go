package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/stretchr/testify/require"
)

// The record types the writer appends. The genesis is written by Start, which
// stamps it itself.
var appendedTypes = []string{"action", "authorization_transition", "opena2a.administrative", "delegation"}

// The writer refuses a record of any type without a trace_id, and decides so
// before it takes a connection: the offline writer's database is never
// reachable.
func TestRecordWriterRefusesARecordWithoutATraceID(t *testing.T) {
	ctx, err := trace.Begin(context.Background())
	require.NoError(t, err)
	n := 0
	for _, recordType := range appendedTypes {
		for _, class := range classes {
			w, reg, logs := offlineWriter(t)
			d := record.Draft{
				EventID:  "a1a1a1a1-0000-4000-8000-000000000001",
				Type:     recordType,
				Retained: map[string]any{"opena2a": map[string]any{"source": "service"}},
				Tenant:   map[string]any{"opena2a": map[string]any{"organization_id": "3f2504e0-4f89-41d3-9a0c-0305e82c3301"}},
			}
			require.NoError(t, trace.Stamp(ctx, &d, nil))
			delete(d.Tenant, "trace_id")

			_, err := w.Write(context.Background(), Write{
				Class: class, OrganizationID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301", Draft: d,
			})
			var we *WriteError
			require.True(t, errors.As(err, &we), "%s %s: want a *WriteError, got %v", recordType, class, err)
			require.Equal(t, ReasonTrace, we.Reason, "%s %s: %v", recordType, class, err)
			require.ErrorIs(t, err, trace.ErrInvalidTrace)
			require.Equal(t, 1.0, failureTotal(t, reg))
			require.Equal(t, "SECURITY record_write_failed class="+string(class)+" reason=trace debt=none\n", logs.String())
			n++
		}
	}
	require.Equal(t, len(appendedTypes)*len(classes), n)
}

// Every way a draft can lack its place in a trace is refused with reason
// trace, before the writer takes a connection.
func TestRecordWriterRefusesADraftWithoutAPlaceInATrace(t *testing.T) {
	ctx, err := trace.Begin(context.Background())
	require.NoError(t, err)
	for name, mutate := range map[string]func(d *record.Draft){
		"no tenant part":       func(d *record.Draft) { d.Tenant, d.TenantSalt = nil, nil },
		"trace_id all zeros":   func(d *record.Draft) { d.Tenant["trace_id"] = strings.Repeat("0", 32) },
		"no parent_id":         func(d *record.Draft) { delete(d.Tenant, "parent_id") },
		"no trace_origin":      func(d *record.Draft) { d.Retained["opena2a"] = map[string]any{"source": "service"} },
		"trace_origin caller":  func(d *record.Draft) { d.Retained["opena2a"] = map[string]any{"trace_origin": "caller"} },
		"parent without trace": func(d *record.Draft) { d.Tenant["parent_id"] = "a1a1a1a1-0000-4000-8000-000000000000" },
	} {
		w, _, _ := offlineWriter(t)
		d := record.Draft{
			EventID:  "a1a1a1a1-0000-4000-8000-000000000001",
			Type:     "opena2a.administrative",
			Retained: map[string]any{"opena2a": map[string]any{"source": "service"}},
		}
		require.NoError(t, trace.Stamp(ctx, &d, nil))
		mutate(&d)
		_, err := w.Write(context.Background(), Write{
			Class: ClassObservation, OrganizationID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301", Draft: d,
		})
		var we *WriteError
		require.True(t, errors.As(err, &we), "%s: want a *WriteError, got %v", name, err)
		require.Equal(t, ReasonTrace, we.Reason, "%s: %v", name, err)
	}
}

// insertIntoLedger matches a statement that inserts into the ledger table.
var insertIntoLedger = regexp.MustCompile(`(?i)\binsert\s+into\s+(?:"?public"?\s*\.\s*)?"?audit_records"?\b`)

// There is exactly one INSERT site for the ledger outside tests: appendQuery.
// Every record, the genesis included, is written through it.
func TestRecordLedgerHasExactlyOneInsertSite(t *testing.T) {
	// Positive control: the pattern finds each form a site could take.
	require.Len(t, insertIntoLedger.FindAllString("INSERT INTO audit_records (a)\n"+
		"insert  into public.audit_records\n"+`INSERT INTO "audit_records"`, -1), 3)
	require.Empty(t, insertIntoLedger.FindAllString("INSERT INTO audit_records_archive; INSERT INTO audit_logs", -1))

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err, "the walk starts at the backend module root")

	var sites []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") || !(strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".sql")) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for range insertIntoLedger.FindAllIndex(src, -1) {
			sites = append(sites, rel)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join("internal", "record", "store", "writer.go")}, sites)
	require.Equal(t, 1, len(insertIntoLedger.FindAllString(appendQuery, -1)), "the one site is appendQuery")
}

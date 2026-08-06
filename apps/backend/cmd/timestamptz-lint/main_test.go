package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The cases that matter are the ones where a naive `grep -i timestamp` gets it wrong.
// This repo really does have a column named `timestamp` (audit_logs), migration headers
// really do discuss the word in prose, and migration 106's postcondition really does query
// for the string literal 'timestamp without time zone'. A lint that fires on any of those
// gets disabled within a week.
func TestScanSQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want int
	}{
		// --- must flag ---
		{
			name: "bare TIMESTAMP column",
			sql:  `CREATE TABLE t (created_at TIMESTAMP NOT NULL DEFAULT NOW());`,
			want: 1,
		},
		{
			name: "explicit WITHOUT TIME ZONE",
			sql:  `CREATE TABLE t (created_at TIMESTAMP WITHOUT TIME ZONE);`,
			want: 1,
		},
		{
			name: "lowercase bare timestamp",
			sql:  `ALTER TABLE t ADD COLUMN expires_at timestamp;`,
			want: 1,
		},
		{
			name: "ADD COLUMN IF NOT EXISTS, the shape migration 065 used",
			sql:  `ALTER TABLE agents ADD COLUMN IF NOT EXISTS pqc_key_expires_at TIMESTAMP;`,
			want: 1,
		},
		{
			name: "two naked columns on separate lines",
			sql: `CREATE TABLE t (
				a TIMESTAMP,
				b TIMESTAMP
			);`,
			want: 2,
		},

		// --- must NOT flag ---
		{
			name: "TIMESTAMPTZ",
			sql:  `CREATE TABLE t (created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`,
			want: 0,
		},
		{
			name: "TIMESTAMP WITH TIME ZONE spelled long",
			sql:  `CREATE TABLE t (created_at TIMESTAMP WITH TIME ZONE);`,
			want: 0,
		},
		{
			name: "CURRENT_TIMESTAMP default",
			sql:  `CREATE TABLE t (created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);`,
			want: 0,
		},
		{
			name: "column named timestamp, correctly typed — the audit_logs case",
			sql:  `CREATE TABLE audit_logs (timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW());`,
			want: 0,
		},
		{
			name: "quoted identifier named timestamp",
			sql:  `ALTER TABLE audit_logs ALTER COLUMN "timestamp" TYPE timestamptz;`,
			want: 0,
		},
		{
			name: "index on a column named timestamp",
			sql:  `CREATE INDEX idx_audit_logs_timestamp ON audit_logs USING btree ("timestamp" DESC);`,
			want: 0,
		},
		{
			name: "line comment discussing TIMESTAMP in prose",
			sql:  `-- A naked TIMESTAMP column drops the offset; use TIMESTAMPTZ instead.`,
			want: 0,
		},
		{
			name: "block comment discussing TIMESTAMP in prose",
			sql:  "/* migration notes: TIMESTAMP columns were converted here.\n   TIMESTAMP again. */",
			want: 0,
		},
		{
			name: "string literal naming the type — migration 106's postcondition",
			sql:  `SELECT count(*) FROM information_schema.columns WHERE data_type = 'timestamp without time zone';`,
			want: 0,
		},
		{
			name: "SET LOCAL TimeZone",
			sql:  `SET LOCAL TimeZone = 'UTC';`,
			want: 0,
		},
		{
			name: "trailing comment after a valid declaration",
			sql:  `created_at TIMESTAMPTZ NOT NULL, -- was TIMESTAMP before migration 106`,
			want: 0,
		},

		// --- bypasses found by adversarial review; each was a miss before the fix, and each
		// --- is red-proofed by a mutation that reverts exactly the property it guards ---
		{
			// The earlier rule looked FORWARD for a type keyword to spot a column named
			// `timestamp`. On this line that swallowed the real naked declaration entirely.
			name: "naked column on the same line as a column NAMED timestamp",
			sql:  `CREATE TABLE audit2 (created_at TIMESTAMP, timestamp TIMESTAMPTZ NOT NULL);`,
			want: 1,
		},
		{
			// Migration 014 declares alerts.acknowledged_at exactly this way. Blanking
			// dollar-quoted bodies as if they were string data made it invisible.
			name: "DDL inside a DO block is real code and must be scanned",
			sql: `DO $$ BEGIN
    ALTER TABLE alerts ADD COLUMN acknowledged_at TIMESTAMP;
END $$;`,
			want: 1,
		},
		{
			// An apostrophe inside a dollar-quoted block used to open a phantom string
			// literal that ran to EOF, silencing every later declaration in the file.
			name: "apostrophe inside a dollar-quoted block does not silence what follows",
			sql: `DO $mig$ BEGIN RAISE NOTICE $msg$agent's key rotated$msg$; END $mig$;
ALTER TABLE agents ADD COLUMN key_rotated_at TIMESTAMP NOT NULL DEFAULT NOW();`,
			want: 1,
		},
		{
			name: "ALTER COLUMN ... TYPE TIMESTAMP is one finding, not two",
			sql:  `ALTER TABLE t ALTER COLUMN x TYPE TIMESTAMP;`,
			want: 1,
		},
		{
			// A column reference inside an index definition is not a type declaration.
			// This exact line exists at migrations/001_initial_schema.sql:130.
			name: "unquoted column reference named timestamp in an index",
			sql:  `CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);`,
			want: 0,
		},
		{
			name: "double-quoted identifier containing a comment marker",
			sql:  `CREATE INDEX ix ON t ("a--b"); ALTER TABLE u ADD COLUMN c TIMESTAMP;`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanSQL("test.sql", tt.sql)
			if len(got) != tt.want {
				t.Errorf("scanSQL() = %d finding(s), want %d\nSQL: %s\nfindings: %+v",
					len(got), tt.want, tt.sql, got)
			}
		})
	}
}

// A column declared naked on a multi-line CREATE TABLE must be reported on its own line, or
// the finding sends the reader to the wrong place in a 200-line migration.
func TestScanSQLReportsCorrectLine(t *testing.T) {
	sql := `-- header comment
CREATE TABLE t (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMP
);`
	got := scanSQL("test.sql", sql)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].Line != 5 {
		t.Errorf("finding reported on line %d, want 5", got[0].Line)
	}
	if !strings.Contains(got[0].Text, "expires_at") {
		t.Errorf("finding text %q does not name the offending column", got[0].Text)
	}
}

// The migration this lint shipped alongside must itself pass — it converts naked columns and
// legitimately mentions the type in prose, in a string literal, and as a quoted identifier.
func TestMigration106IsClean(t *testing.T) {
	got := scanDirOrSkip(t, "../../migrations")
	for _, f := range got {
		if strings.Contains(f.File, "106_convert_naked_timestamps") {
			t.Errorf("migration 106 flagged itself at line %d: %s", f.Line, f.Text)
		}
	}
}

// A finding carries a source excerpt so the reader can see the offending declaration. Carrying
// the WHOLE line means a long line with many declarations amplifies quadratically — measured at
// 8.8 GB of output from a 440 KB one-line file with 20,000 declarations, which fills a CI
// runner's disk while the lint itself finishes in under two seconds.
//
// The bound is asserted from both sides: the total output must stay far below the input's own
// worst case, AND each excerpt must be individually bounded. Asserting only a total would pass
// against a build that truncated to 10 KB per finding.
func TestFindingExcerptIsBounded(t *testing.T) {
	const declarations = 2000
	line := strings.Repeat("created_at TIMESTAMP, ", declarations)

	got := scanSQL("long.sql", line)
	if len(got) != declarations {
		t.Fatalf("expected %d findings, got %d", declarations, len(got))
	}

	total := 0
	for i, f := range got {
		if len(f.Text) > maxFindingTextLen+len("… (line truncated)") {
			t.Fatalf("finding %d carries a %d-byte excerpt; bound is %d",
				i, len(f.Text), maxFindingTextLen)
		}
		total += len(f.Text)
	}

	// Pre-fix this is declarations * len(line) — about 88 MB for these inputs. Post-fix it is
	// declarations * ~200. The bound sits between the two so removing the truncation fails it.
	untruncated := declarations * len(line)
	if total >= untruncated/10 {
		t.Errorf("total excerpt bytes %d is not meaningfully below the untruncated %d;"+
			" the excerpt bound is not being applied", total, untruncated)
	}
}

// The excerpt must say that it was shortened. A silently truncated line reads as the whole
// declaration and sends the reader looking for a syntax error that is not there.
func TestTruncatedExcerptIsMarked(t *testing.T) {
	long := "created_at TIMESTAMP" + strings.Repeat(" /*pad*/", 200)
	got := scanSQL("long.sql", long)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if !strings.Contains(got[0].Text, "truncated") {
		t.Errorf("excerpt was shortened without saying so: %q", got[0].Text)
	}

	short := "created_at TIMESTAMP;"
	got = scanSQL("short.sql", short)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if strings.Contains(got[0].Text, "truncated") {
		t.Errorf("a short line must not be marked truncated: %q", got[0].Text)
	}
}

// The baseline suppresses a COUNT per location, not a location. Treating it as a set let a
// baselined line grow a second naked declaration with the baseline file unchanged — the guard
// reporting success on a brand-new defect, which is the worst failure a guard can have.
func TestBaselineDoesNotMaskAnAdditionalFindingOnABaselinedLine(t *testing.T) {
	dir := t.TempDir()
	sql := "CREATE TABLE t (\n    created_at TIMESTAMP NOT NULL, secret_rotated_at TIMESTAMP NOT NULL\n);\n"
	if err := os.WriteFile(filepath.Join(dir, "090_probe.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("fixture must produce 2 findings on one line, got %d: %+v", len(found), found)
	}
	loc := found[0].BaselineKey()
	if found[1].BaselineKey() != loc {
		t.Fatalf("fixture must put both findings on the same line, got %s and %s",
			loc, found[1].BaselineKey())
	}

	// A baseline recording ONE finding at that location must still report the second.
	baselinePath := filepath.Join(dir, "baseline.txt")
	if err := os.WriteFile(baselinePath, []byte(loc+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := loadBaseline(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := baseline[loc]; got != 1 {
		t.Fatalf("baseline should record 1 allowance for %s, got %d", loc, got)
	}

	// Exercise the REAL filter, not a copy of it. An inline re-implementation here would stay
	// green while the shipped binary regressed to set semantics.
	reported := applyBaseline(found, baseline)
	if len(reported) != 1 {
		t.Errorf("a second naked declaration on a baselined line must be reported; got %d (%+v)",
			len(reported), reported)
	}
}

// The baseline must cover the historical declarations exactly, with nothing left over. An entry
// with no matching finding is an allowance sitting idle, ready to absorb a future defect.
func TestHistoricalBaselineMatchesTheTreeExactly(t *testing.T) {
	found := scanDirOrSkip(t, "../../migrations")
	baseline, err := loadBaseline("historical-baseline.txt")
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, f := range found {
		counts[f.BaselineKey()]++
	}

	for loc, want := range baseline {
		if counts[loc] != want {
			t.Errorf("baseline allows %d finding(s) at %s but the tree has %d;"+
				" a stale baseline entry can absorb a future defect", want, loc, counts[loc])
		}
	}
	for loc, got := range counts {
		if baseline[loc] != got {
			t.Errorf("tree has %d finding(s) at %s but the baseline records %d",
				got, loc, baseline[loc])
		}
	}
}

func scanDirOrSkip(t *testing.T, dir string) []finding {
	t.Helper()
	got, err := scanDir(dir)
	if err != nil {
		t.Skipf("cannot scan %s: %v", dir, err)
	}
	return got
}

// A byte-slice truncation splits multi-byte runes and emits a lone continuation byte, making
// the tool's whole output invalid UTF-8 and breaking any strict consumer (JSON encoders, CI
// annotation parsers). 16 migration files already contain non-ASCII, so this is reachable.
func TestTruncatedExcerptStaysValidUTF8(t *testing.T) {
	// Place an em dash (3 bytes) so that it straddles the truncation boundary.
	for pad := maxFindingTextLen - 4; pad <= maxFindingTextLen+2; pad++ {
		line := "created_at TIMESTAMP " + strings.Repeat("x", pad) + "—tail"
		got := scanSQL("utf8.sql", line)
		if len(got) != 1 {
			t.Fatalf("pad=%d: expected 1 finding, got %d", pad, len(got))
		}
		if !utf8.ValidString(got[0].Text) {
			t.Errorf("pad=%d: excerpt is not valid UTF-8: %q", pad, got[0].Text)
		}
	}
}

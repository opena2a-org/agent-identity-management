// timestamptz-lint enforces the schema invariant "TIMESTAMPTZ always, never TIMESTAMP".
//
// A `timestamp without time zone` column silently shifts the stored instant by the writing
// process's UTC offset: lib/pq sends a Go time.Time as its wall clock, Postgres drops the
// offset, and on read lib/pq labels the naked value UTC. East of UTC an API key stays valid
// past its stated expiry by exactly that offset. The same corruption reaches columns no Go
// code touches, because `DEFAULT NOW()` is cast under the PostgreSQL *session* zone.
//
// This is the source half of the guard: it flags a naked column in a migration file before it
// is ever applied. The database half is TestNakedTimestampSchemaHasNoNakedTimestampColumns,
// which asserts the applied schema holds zero of them. Both are needed — the lint cannot see a
// column that entered through a path other than this repo's migrations, and the test cannot
// see a migration that has not been applied yet.
//
// It exists because the convention alone did not hold. "TIMESTAMPTZ always" was already
// written into the CA charter, and migrations 065, 090 and 101 each added naked columns after
// it was. Migration 106 converted the 42 that had accumulated.
//
// Findings are reported as file:line: <text>; exit code 1 if any, 0 otherwise. A usage error or
// an unreadable target exits 2 rather than 0, so a mistyped path in CI fails the job instead of
// reporting a clean scan of nothing.
//
// Baseline support: pass --baseline <path> to a file of file:line entries to allowlist during
// a staged migration. It should stay empty — a non-empty baseline is a review trigger.
//
// KNOWN LIMITATIONS — this is a line-oriented tokenizer, not a SQL parser, and these shapes are
// measured misses rather than assumed ones. Each is caught by the database half of the guard
// (TestNakedTimestampSchemaHasNoNakedTimestampColumns and migration 106's postcondition, both
// of which resolve domains and read pg_attribute), which is why the pair is the guard and
// neither half is:
//
//   - A quoted column name: `"my col" TIMESTAMP`. A quoted previous token returns false.
//   - A column named with a non-reserved keyword this file lists, e.g. `key TIMESTAMP`.
//     `system_config.key` shows the repo really does use such names.
//   - `CREATE DOMAIN d AS TIMESTAMP`, and a type declared on a continuation line.
//   - `now()::timestamp` in a materialized view body.
//
// And two false-positive shapes, which cost a spurious CI failure rather than a missed column:
// `TIMESTAMP(3) WITH TIME ZONE` (the precision paren sits where WITH is looked for), and a
// PL/pgSQL local like `DECLARE v_now timestamp;` inside a DO block.
//
// A single stray backslash-escaped quote inside an E-string still blanks the rest of a FILE,
// because the string scanner understands a doubled single quote but not backslash escapes. No
// migration currently contains one. Inside a dollar-quoted block the damage is bounded to that
// block.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// maxFindingTextLen bounds the source excerpt carried on a finding.
//
// The excerpt exists to orient a reader who already has file:line; it is not the fix. Without
// a bound, every finding on a line carries that whole line, so a single long line with many
// declarations amplifies quadratically: a 440 KB one-line file with 20,000 declarations
// produced 8.8 GB of output — enough to fill a CI runner's disk while the lint itself ran in
// under two seconds. Measured, not hypothesised.
const maxFindingTextLen = 200

type finding struct {
	File string
	Line int
	Text string
	Why  string
}

// Location is the human-facing position, carrying whatever path the caller passed in.
func (f finding) Location() string {
	return fmt.Sprintf("%s:%d", f.File, f.Line)
}

// BaselineKey is the position as the baseline records it: base filename and line, never the
// caller's path prefix. CI runs the lint as `timestamptz-lint migrations` from apps/backend
// while the package test scans `../../migrations`; keying on the full path would make the same
// declaration two different entries, so the baseline would silently stop applying under one of
// them. Migration filenames are unique, which is what makes the basename sufficient.
func (f finding) BaselineKey() string {
	return fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line)
}

func main() {
	baselinePath := flag.String("baseline", "", "path to baseline file of file:line entries to allowlist")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: timestamptz-lint [--baseline path] <dir> [<dir>...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	baseline, err := loadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "timestamptz-lint: load baseline: %v\n", err)
		os.Exit(2)
	}

	var findings []finding
	for _, dir := range dirs {
		fs, err := scanDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "timestamptz-lint: scan %s: %v\n", dir, err)
			os.Exit(2)
		}
		findings = append(findings, fs...)
	}

	reported := applyBaseline(findings, baseline)
	sort.Slice(reported, func(i, j int) bool {
		if reported[i].File != reported[j].File {
			return reported[i].File < reported[j].File
		}
		return reported[i].Line < reported[j].Line
	})

	for _, f := range reported {
		fmt.Printf("%s: %s (%s)\n", f.Location(), strings.TrimSpace(f.Text), f.Why)
	}

	if len(reported) > 0 {
		fmt.Fprintf(os.Stderr, "\ntimestamptz-lint: %d naked TIMESTAMP declaration(s) found.\n", len(reported))
		fmt.Fprintf(os.Stderr,
			"Use TIMESTAMPTZ. A naked TIMESTAMP shifts the stored instant by the writer's UTC\n"+
				"offset, and by the PG session's for DEFAULT NOW(). See migration 106.\n")
		os.Exit(1)
	}
}

// applyBaseline returns the findings that survive the baseline, consuming one allowance per
// baselined location.
//
// This lives in its own function so a test can exercise the REAL filtering. A test that
// re-implements this loop inline proves nothing about the binary: reverting the decrement here
// (the exact defect the counting fix closes) leaves such a test green while the shipped tool
// silently allows a new naked column through.
func applyBaseline(findings []finding, baseline map[string]int) []finding {
	remaining := make(map[string]int, len(baseline))
	for loc, n := range baseline {
		remaining[loc] = n
	}
	var reported []finding
	for _, f := range findings {
		if remaining[f.BaselineKey()] > 0 {
			remaining[f.BaselineKey()]--
			continue
		}
		reported = append(reported, f)
	}
	return reported
}

// loadBaseline reads allowlisted locations as a COUNT per location, not a set.
//
// A set silently masks a new defect: one baselined line that grows a second naked declaration
// still matches the same file:line, so the new column is allowed through with the baseline file
// unchanged. Counting means the baseline suppresses exactly as many findings as were recorded
// at that location and no more — a second declaration on a baselined line is reported.
func loadBaseline(path string) (map[string]int, error) {
	if path == "" {
		return map[string]int{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer f.Close()

	entries := map[string]int{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries[line]++
	}
	return entries, scanner.Err()
}

func scanDir(dir string) ([]finding, error) {
	var findings []finding
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".sql") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings = append(findings, scanSQL(path, string(data))...)
		return nil
	})
	return findings, err
}

// scanSQL reports naked TIMESTAMP type declarations in one SQL file.
//
// Comments and string literals are blanked before tokenizing, so prose in a migration header
// and a literal like 'timestamp without time zone' (which the migration 106 postcondition
// legitimately queries for) cannot produce a finding.
func scanSQL(path, src string) []finding {
	var findings []finding
	stripped := blankNonCode(src)
	lines := strings.Split(stripped, "\n")
	rawLines := strings.Split(src, "\n")

	for i, line := range lines {
		for _, why := range nakedTimestampsIn(line) {
			findings = append(findings, finding{
				File: path,
				Line: i + 1,
				Text: truncateExcerpt(rawLines[i]),
				Why:  why,
			})
		}
	}
	return findings
}

// truncateExcerpt bounds one source line to maxFindingTextLen, marking any elision so a reader
// is never shown a silently shortened line and told it is the whole one.
func truncateExcerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxFindingTextLen {
		return s
	}
	// Back off to a rune boundary. A plain byte slice splits multi-byte characters and emits a
	// lone continuation byte, which makes the whole output invalid UTF-8 — 16 migration files
	// already contain non-ASCII, so this is reachable, not theoretical.
	cut := maxFindingTextLen
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "… (line truncated)"
}

// sqlKeywords are words that can appear immediately before a type but are not column names.
// A TIMESTAMP preceded by one of these is not a column's declared type — except for TYPE
// itself, which is exactly the `ALTER COLUMN x TYPE TIMESTAMP` shape we do want to flag.
var sqlKeywords = map[string]bool{
	"ADD": true, "ALTER": true, "AND": true, "AS": true, "ASC": true, "BY": true,
	"CHECK": true, "COLUMN": true, "CREATE": true, "DEFAULT": true, "DESC": true,
	"DISTINCT": true, "EXISTS": true, "FROM": true, "IF": true, "INDEX": true,
	"INTO": true, "IS": true, "KEY": true, "NOT": true, "NULL": true, "ON": true,
	"OR": true, "ORDER": true, "PRIMARY": true, "REFERENCES": true, "RETURNS": true,
	"SELECT": true, "SET": true, "TABLE": true, "UNIQUE": true, "USING": true,
	"VALUES": true, "WHERE": true,
}

// nakedTimestampsIn returns a reason string per naked TIMESTAMP type usage on the line.
//
// A TIMESTAMP token is a declared TYPE only when what precedes it can introduce one: a column
// name (a bare identifier), or the keyword TYPE. Deciding from the PRECEDING token rather than
// the following one is what keeps three real shapes straight:
//
//	created_at TIMESTAMP, timestamp TIMESTAMPTZ   -> flags created_at, ignores the column named
//	                                                 `timestamp` (an earlier rule that looked
//	                                                  FORWARD for a type keyword missed the
//	                                                  naked column entirely on this line)
//	CREATE INDEX ... ON audit_logs(timestamp DESC) -> ignored; `timestamp` follows a paren, so
//	                                                  it is a column reference, not a type
//	ALTER COLUMN x TYPE TIMESTAMP                  -> flagged once, on TYPE
func nakedTimestampsIn(line string) []string {
	toks := tokenize(line)
	var out []string

	for i, tok := range toks {
		if tok.quoted || tok.punct || !strings.EqualFold(tok.text, "TIMESTAMP") {
			continue
		}
		if !introducesAType(toks, i) {
			continue
		}

		switch {
		case followedBy(toks, i, "WITH", "TIME", "ZONE"):
			// TIMESTAMP WITH TIME ZONE — correct, just spelled the long way.
			continue
		case followedBy(toks, i, "WITHOUT", "TIME", "ZONE"):
			out = append(out, "TIMESTAMP WITHOUT TIME ZONE; use TIMESTAMPTZ")
		default:
			out = append(out, "bare TIMESTAMP; use TIMESTAMPTZ")
		}
	}
	return out
}

// introducesAType reports whether the token before position i can introduce a column type.
func introducesAType(toks []token, i int) bool {
	if i == 0 {
		// Nothing precedes it on this line. A type continued from the previous line is
		// possible; flagging here would false-positive on a bare `timestamp` reference, and
		// the multi-line case is covered by the schema-invariant integration test.
		return false
	}
	prev := toks[i-1]
	if prev.quoted || prev.punct {
		// `(timestamp DESC)` or `, timestamp` — a column reference or a column NAME.
		return false
	}
	if strings.EqualFold(prev.text, "TYPE") {
		return true
	}
	// Any other bare identifier that is not a SQL keyword is a column name, so this
	// TIMESTAMP is its declared type.
	return !sqlKeywords[strings.ToUpper(prev.text)]
}

type token struct {
	text   string
	quoted bool // double-quoted identifier — never a type
	punct  bool // a single punctuation byte, e.g. ( , ; :
}

func nextToken(toks []token, i int) (token, bool) {
	if i+1 >= len(toks) {
		return token{}, false
	}
	return toks[i+1], true
}

func followedBy(toks []token, i int, want ...string) bool {
	for n, w := range want {
		t, ok := nextToken(toks, i+n)
		if !ok || t.quoted || !strings.EqualFold(t.text, w) {
			return false
		}
	}
	return true
}

// tokenize splits a line into word tokens, double-quoted identifiers, and punctuation.
//
// `_` counts as a word character, which is what keeps CURRENT_TIMESTAMP from tokenizing as a
// bare TIMESTAMP. Punctuation is emitted rather than discarded because the preceding token is
// how a type is told apart from a column reference: `(timestamp DESC)` and `, timestamp` are
// both column references, and only a bare identifier before it makes it a declared type.
func tokenize(line string) []token {
	var toks []token
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, token{text: cur.String()})
			cur.Reset()
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			flush()
			j := i + 1
			for j < len(line) && line[j] != '"' {
				j++
			}
			toks = append(toks, token{text: line[i+1 : min(j, len(line))], quoted: true})
			i = j
		case isWordByte(c):
			cur.WriteByte(c)
		case c == ' ' || c == '\t' || c == '\r':
			flush()
		default:
			flush()
			toks = append(toks, token{text: string(c), punct: true})
		}
	}
	flush()
	return toks
}

func isWordByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// blankNonCode replaces the contents of -- comments, /* */ comments, '...' string literals and
// $tag$...$tag$ dollar-quoted blocks with spaces, preserving line structure so reported line
// numbers stay correct.
//
// Dollar-quoting is handled FIRST and is not optional. Every migration in this repo that uses a
// DO block contains prose with apostrophes inside `$$ ... $$`. Without this case, the first such
// apostrophe opens a string literal that never closes, blanking the remainder of the FILE — so
// the lint reports nothing and exits 0 for every declaration after it. A guard whose failure
// mode is silence is worse than no guard, because the green result is indistinguishable.
func blankNonCode(src string) string {
	out := []byte(src)
	blankRegion(src, out, 0, len(src))
	return string(out)
}

// blankRegion blanks non-code within [start,end), recursing into dollar-quoted bodies.
//
// A dollar-quoted body is scanned as CODE, not blanked. In this repo the conditional migrations
// put real DDL inside `DO $$ ... $$` — migration 014 declares `alerts.acknowledged_at TIMESTAMP`
// there — so blanking those bodies would make the lint blind to exactly the migrations that are
// hardest to review by eye.
//
// Recursing is also what contains the failure. An apostrophe in prose inside a dollar-quoted
// block (`$msg$agent's key$msg$`) still opens a phantom literal, but the region bounds it: it
// can swallow the rest of that BLOCK and no further. Before, it silenced the rest of the FILE.
func blankRegion(src string, out []byte, start, end int) {
	blank := func(from, to int) {
		for k := from; k < to && k < len(out); k++ {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
	}

	for i := start; i < end; i++ {
		switch {
		case src[i] == '$':
			tag, ok := dollarTagAt(src, i)
			if !ok {
				continue
			}
			bodyStart := i + len(tag)
			if bodyStart > end {
				continue
			}
			rel := strings.Index(src[bodyStart:end], tag)
			if rel < 0 {
				// Unterminated within this region. Blank the delimiter and keep reading the
				// remainder as code rather than giving up on the rest of the file.
				blank(i, bodyStart)
				i = bodyStart - 1
				continue
			}
			bodyEnd := bodyStart + rel
			closeEnd := bodyEnd + len(tag)
			blank(i, bodyStart)      // opening delimiter
			blank(bodyEnd, closeEnd) // closing delimiter
			blankRegion(src, out, bodyStart, bodyEnd)
			i = closeEnd - 1
		case src[i] == '"':
			// A double-quoted identifier is code, and its contents are not. Skip over it
			// without interpreting them, or an identifier like "a--b" reads as a comment
			// start and blanks the rest of the line.
			j := i + 1
			for j < end && src[j] != '"' {
				j++
			}
			i = min(j, end-1)
		case src[i] == '-' && i+1 < end && src[i+1] == '-':
			j := strings.IndexByte(src[i:end], '\n')
			if j < 0 {
				blank(i, end)
				return
			}
			blank(i, i+j)
			i += j
		case src[i] == '/' && i+1 < end && src[i+1] == '*':
			j := strings.Index(src[i+2:end], "*/")
			if j < 0 {
				blank(i, end)
				return
			}
			blank(i, i+2+j+2)
			i += 2 + j + 1
		case src[i] == '\'':
			j := i + 1
			for j < end {
				if src[j] == '\'' {
					// '' is an escaped quote inside the literal.
					if j+1 < end && src[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			blank(i, min(j+1, end))
			i = j
		}
	}
}

// dollarTagAt returns the dollar-quote tag starting at i, e.g. "$$" or "$body$".
//
// A tag is `$`, an optional identifier that must not start with a digit, then `$`. Anything
// else beginning with `$` (a positional parameter like `$1`, or a lone `$`) is not a tag.
func dollarTagAt(src string, i int) (string, bool) {
	if i >= len(src) || src[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(src) && src[j] != '$' {
		if !isWordByte(src[j]) || (j == i+1 && src[j] >= '0' && src[j] <= '9') {
			return "", false
		}
		j++
	}
	if j >= len(src) {
		return "", false
	}
	return src[i : j+1], true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

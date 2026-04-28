//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestFGA_PqArray_RoundTripSpecialChars is the regression test for H4. The
// previous hand-rolled `pq_from_strings` only escaped `"`, so any
// attribute element containing `\`, `{`, `}`, or `,` would corrupt the
// audit-attestation table on insert. With pq.Array, those characters
// must round-trip cleanly.
//
// Build-tag gated: requires a Postgres reachable via TEST_DATABASE_URL.
// Skip if absent so the unit-test pipeline stays self-contained. Run via
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestFGA_PqArray_RoundTripSpecialChars \
//	  ./internal/application/...
func TestFGA_PqArray_RoundTripSpecialChars(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration round-trip test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	// Pin a single connection so CREATE TEMP TABLE / INSERT / SELECT all
	// share the same session; otherwise the pool can route the SELECT to
	// a connection that never saw the temp table.
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	// Use a throwaway temp table so we don't depend on (or pollute) any
	// production schema. text[] mirrors attributes_accessed +
	// fga_steps_triggered in tool_call_history / access_attestations.
	_, err = conn.ExecContext(ctx, `CREATE TEMP TABLE pq_roundtrip (id SERIAL PRIMARY KEY, attrs TEXT[])`)
	require.NoError(t, err)

	// Each of these would have been mangled by pq_from_strings:
	//   `,`  was element-separator, would split the value mid-element
	//   `{` / `}`  bracket-confused the parser
	//   `\`  was un-escaped, broke the next quoted character
	// The standalone `"` case (the one pq_from_strings did handle) is
	// included to confirm we didn't regress that.
	want := []string{
		`comma,inside`,
		`brace{open`,
		`brace}close`,
		`back\slash`,
		`quote"inside`,
		`combined ,{}\"`,
		`empty-next-is-ok`,
		``,
	}

	_, err = conn.ExecContext(ctx, `INSERT INTO pq_roundtrip (attrs) VALUES ($1)`, pq.Array(want))
	require.NoError(t, err, "pq.Array insert must accept arbitrary string values")

	var got []string
	err = conn.QueryRowContext(ctx, `SELECT attrs FROM pq_roundtrip ORDER BY id DESC LIMIT 1`).Scan(pq.Array(&got))
	require.NoError(t, err)
	require.Equal(t, want, got,
		"pq.Array must round-trip every special-char value byte-for-byte; pq_from_strings did not")
}

//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Regression tests for migration 106 — `timestamp without time zone` columns losing the
// writing process's UTC offset.
//
// `lib/pq` sends a Go `time.Time` as its wall clock. A naked `timestamp` column drops the
// offset; on read `lib/pq` labels the value UTC. The instant therefore shifts by the writer's
// offset, and east of UTC an API key stays valid past its stated expiry by exactly that much.
//
// There are two independent paths to the same corruption and the tests below cover one each:
//
//	axis (a) the Go process zone — a `time.Time` carried into the INSERT
//	axis (b) the PostgreSQL session zone — `DEFAULT NOW()` and migration 092's
//	         `NEW.verified_at := NOW()` trigger, neither of which any Go code touches
//
// Axis (b) is why the fix is the column type and not a `.UTC()` sweep at the write sites:
// no amount of Go-side discipline reaches a `DEFAULT NOW()`.
//
// The existing TestAgentRevocation_APIKeyMiddlewareEnforcesKeyFields deliberately sidesteps
// this bug by calling `.UTC()` before writing, and documents why. These tests are the ones
// that do not sidestep it.
//
// Red-proof: all three fail against the schema at migration 105 and pass at 106. Verified by
// running them at both revisions, not by inspection.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with migrations
// through 106 applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run NakedTimestamp ./internal/infrastructure/repository/...

// zoneEastOfUTC is the security-relevant direction: a credential written here outlives its
// stated expiry by the offset. Asia/Tokyo is +09:00 year-round, so the test does not depend
// on when it runs relative to a DST boundary.
const zoneEastOfUTC = "Asia/Tokyo"

func openNakedTimestampTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping naked timestamp integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	return db
}

// seedOrgUserAgent creates the FK chain api_keys requires and registers its teardown.
func seedOrgUserAgent(t *testing.T, db *sql.DB, ctx context.Context, label string) (orgID, userID, agentID uuid.UUID) {
	t.Helper()

	orgID, userID, agentID = uuid.New(), uuid.New(), uuid.New()
	suffix := agentID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM api_keys WHERE agent_id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, label+"-org-"+suffix, label+"-"+suffix+".test")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'test', $5, NOW(), NOW())`,
		userID, orgID, label+"-"+suffix+"@test.invalid", label+"-user", label+"-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'service', $5, NOW(), NOW())`,
		agentID, orgID, label+"-agent-"+suffix, label+"-agent", userID)
	require.NoError(t, err)

	return orgID, userID, agentID
}

// TestNakedTimestampSchemaHasNoNakedTimestampColumns is the invariant that keeps the class
// closed. The convention "TIMESTAMPTZ always, never TIMESTAMP" already existed in the CA
// charter and migrations 065, 090 and 101 each violated it anyway, so it is asserted here
// rather than left to review.
//
// This is the database half of the guard. cmd/timestamptz-lint is the source half; it catches
// a naked column in a migration file before it is ever applied. Both are needed: the lint
// cannot see a column that entered through a path other than this repo's migrations.
func TestNakedTimestampSchemaHasNoNakedTimestampColumns(t *testing.T) {
	db := openNakedTimestampTestDB(t)
	ctx := context.Background()

	// pg_attribute, not information_schema.columns, for the same two reasons migration 106's
	// postcondition uses it: information_schema is privilege-filtered (the identical schema
	// returned 5 rows as superuser and 0 as a dedicated migration role), and it reports an
	// array column's data_type as 'ARRAY', so `timestamp[]` is invisible to a data_type
	// comparison. Checking atttypid also reaches matviews, views and composite types.
	rows, err := db.QueryContext(ctx,
		`SELECT n.nspname || '.' || c.relname || '.' || a.attname
		   FROM pg_attribute a
		   JOIN pg_class c     ON c.oid = a.attrelid
		   JOIN pg_namespace n ON n.oid = c.relnamespace
		   JOIN pg_type ty     ON ty.oid = a.atttypid
		  WHERE a.attnum > 0
		    AND NOT a.attisdropped
		    -- Direct naked timestamp, or a DOMAIN over one: a domain column's atttypid is the
		    -- domain's OID, so comparing atttypid alone would miss CREATE DOMAIN d AS TIMESTAMP.
		    AND (
		          a.atttypid IN ('pg_catalog.timestamp'::regtype, 'pg_catalog.timestamp[]'::regtype)
		       OR (ty.typtype = 'd'
		           AND ty.typbasetype IN ('pg_catalog.timestamp'::regtype,
		                                  'pg_catalog.timestamp[]'::regtype))
		    )
		    AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		    -- Exclude temp relations: the integration packages share one database and
		    -- fga_engine_pq_integration_test.go creates a TEMP TABLE, so without this the
		    -- assertion fails or passes depending on test interleaving.
		    AND c.relpersistence <> 't'
		    AND c.relkind IN ('r', 'p', 'm', 'v', 'f', 'c')
		    AND NOT EXISTS (
		        SELECT 1 FROM pg_depend d
		         WHERE d.classid = 'pg_class'::regclass
		           AND d.objid = c.oid AND d.deptype = 'e'
		    )
		  ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()

	var offenders []string
	for rows.Next() {
		var col string
		require.NoError(t, rows.Scan(&col))
		offenders = append(offenders, col)
	}
	require.NoError(t, rows.Err())

	require.Emptyf(t, offenders,
		"%d column(s) are `timestamp without time zone`: %v\n"+
			"Every timestamp column must be TIMESTAMPTZ. A naked timestamp silently shifts the "+
			"stored instant by the writing process's UTC offset (and, for DEFAULT NOW(), by the "+
			"PostgreSQL session's). Add the column as TIMESTAMPTZ, or convert it the way "+
			"migration 106 does.",
		len(offenders), offenders)
}

// TestNakedTimestampAPIKeyExpiryRoundTripFromNonUTCZone covers axis (a): a Go `time.Time` in a
// non-UTC location carried into the INSERT.
//
// This is the write path api_key_service.CreateAPIKey uses — it computes
// `time.Now().AddDate(0, 0, expiresInDays)`, which carries the process's zone.
func TestNakedTimestampAPIKeyExpiryRoundTripFromNonUTCZone(t *testing.T) {
	db := openNakedTimestampTestDB(t)
	ctx := context.Background()

	orgID, userID, agentID := seedOrgUserAgent(t, db, ctx, "naked-ts-expiry")

	loc, err := time.LoadLocation(zoneEastOfUTC)
	require.NoError(t, err)

	// One instant, expressed in a zone east of UTC. Whole seconds so the assertion cannot
	// trip on Postgres storing microseconds against Go's nanoseconds.
	intended := time.Date(2027, 3, 14, 9, 26, 53, 0, time.UTC)
	expiresAt := intended.In(loc)

	// Precondition: the value really is offset from UTC, or the test proves nothing.
	_, offsetSeconds := expiresAt.Zone()
	require.NotZero(t, offsetSeconds,
		"precondition: %s must have a non-zero UTC offset for this test to exercise the bug",
		zoneEastOfUTC)

	keyID := uuid.New()
	_, err = db.ExecContext(ctx,
		`INSERT INTO api_keys (id, organization_id, agent_id, name, key_hash, prefix, expires_at, created_by)
		 VALUES ($1, $2, $3, 'naked-ts-expiry-key', $4, 'aim_live_naked', $5, $6)`,
		keyID, orgID, agentID, keyID.String(), expiresAt, userID)
	require.NoError(t, err)

	var readBack time.Time
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT expires_at FROM api_keys WHERE id = $1`, keyID).Scan(&readBack))

	require.Truef(t, readBack.Equal(intended),
		"expires_at came back as a different instant than it was written as.\n"+
			"  wrote  : %s (%s, offset %+d)\n"+
			"  intended: %s\n"+
			"  read back: %s\n"+
			"  drift  : %s\n"+
			"A key written east of UTC stays valid past its stated expiry by the offset, at the "+
			"`time.Now().After(*apiKey.ExpiresAt)` check in api_key_service.go.",
		expiresAt.Format(time.RFC3339), zoneEastOfUTC, offsetSeconds/3600,
		intended.Format(time.RFC3339), readBack.UTC().Format(time.RFC3339),
		readBack.Sub(intended))
}

// TestNakedTimestampDefaultNowRoundTripUnderNonUTCSession covers axis (b): no Go time value is
// involved at all. The column default supplies the value, and it is cast under the PostgreSQL
// *session* zone.
//
// cmd/server/main.go builds its DSN with no TimeZone parameter, so a self-hoster whose
// PostgreSQL defaults to a local zone hits this with entirely stock configuration. A `.UTC()`
// sweep at the Go write sites would not have moved this test.
func TestNakedTimestampDefaultNowRoundTripUnderNonUTCSession(t *testing.T) {
	db := openNakedTimestampTestDB(t)
	ctx := context.Background()

	// Pin one connection: SET TIME ZONE is per-session, and database/sql would otherwise
	// hand the INSERT to a different pooled connection than the SET.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	var restore string
	require.NoError(t, conn.QueryRowContext(ctx, `SHOW TimeZone`).Scan(&restore))
	_, err = conn.ExecContext(ctx, `SET TIME ZONE `+quoteLiteral(zoneEastOfUTC))
	require.NoError(t, err)
	defer func() { _, _ = conn.ExecContext(ctx, `SET TIME ZONE `+quoteLiteral(restore)) }()

	var sessionTZ string
	require.NoError(t, conn.QueryRowContext(ctx, `SHOW TimeZone`).Scan(&sessionTZ))
	require.Equalf(t, zoneEastOfUTC, sessionTZ,
		"precondition: the session zone must actually be %s for this test to exercise the bug",
		zoneEastOfUTC)

	orgID := uuid.New()
	suffix := orgID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	before := time.Now().UTC()
	// created_at is NOT in the column list: the DEFAULT NOW() supplies it.
	_, err = conn.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, updated_at)
		 VALUES ($1, $2, $3, NOW())`,
		orgID, "naked-ts-defaultnow-org-"+suffix, "naked-ts-defaultnow-"+suffix+".test")
	require.NoError(t, err)
	after := time.Now().UTC()

	var readBack time.Time
	require.NoError(t, conn.QueryRowContext(ctx,
		`SELECT created_at FROM organizations WHERE id = $1`, orgID).Scan(&readBack))

	require.Truef(t,
		!readBack.Before(before.Add(-time.Minute)) && !readBack.After(after.Add(time.Minute)),
		"created_at from DEFAULT NOW() came back outside the window in which it was written.\n"+
			"  session zone : %s\n"+
			"  window       : %s .. %s\n"+
			"  read back    : %s\n"+
			"  drift        : %s\n"+
			"No Go time value was involved — the column default was cast under the session zone. "+
			"This is the path a `.UTC()` sweep at the write sites cannot reach.",
		sessionTZ, before.Format(time.RFC3339), after.Format(time.RFC3339),
		readBack.UTC().Format(time.RFC3339), readBack.Sub(before))
}

// quoteLiteral wraps a zone name as a SQL string literal. SET TIME ZONE takes no placeholder,
// and the values here are test constants rather than user input, but doubling any quote keeps
// the helper from being a copy-paste injection footgun later.
func quoteLiteral(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(append(out, '\''))
}

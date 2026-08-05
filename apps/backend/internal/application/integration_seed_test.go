//go:build integration

package application

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Shared seeding for the integration tests in this package.
//
// These helpers exist because the same NOT NULL / foreign-key columns kept
// being omitted independently in each test file, and nothing caught it: the
// integration suite is excluded from `go test ./...` by its build tag, and CI
// had no database, so every one of these tests failed the moment it was
// actually run. Seeding through one path means a future column requirement is
// fixed once rather than in eleven places.
//
// The requirements as of migration 104:
//   - organizations.domain  VARCHAR(255) UNIQUE NOT NULL
//   - users.*               organization_id, email, name, password_hash, role,
//                           provider, provider_id
//   - agents.display_name   NOT NULL
//   - agents.created_by     NOT NULL, FK -> users(id)
//   - mcp_servers.created_by NOT NULL, FK -> users(id)

// seedOrgAndUser creates the organization and user rows that `agents` and
// `mcp_servers` require via NOT NULL / foreign key. Returns both IDs and
// registers cleanup.
//
// The domain is derived from the generated org UUID rather than from the
// caller's prefix, because the column is UNIQUE and prefixes repeat across
// tests in the same run.
func seedOrgAndUser(t *testing.T, db *sql.DB, ctx context.Context, prefix string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orgID := uuid.New()
	userID := uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, prefix+"-org-"+suffix, prefix+"-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users
		   (id, organization_id, email, name, password_hash, role, provider,
		    provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, prefix+"-"+suffix+"@example.com", prefix+"-user", "local-"+suffix)
	require.NoError(t, err)

	return orgID, userID
}

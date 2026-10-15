package repository

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Update writes the fields a caller edits and nothing about the server's
// verification: status, is_verified and last_verified_at are written by
// VerifyServer, UpdateMCPVerificationStatus and WithdrawConsensusVerification,
// each in one statement of its own. The snapshot below is one a caller read
// before a consensus withdrawal: saving it must not write the server back to
// verified, where consensus_verified_at NULL would keep a later recheck from
// withdrawing it again.
func TestMCPServerRepository_UpdateLeavesTheVerificationAlone(t *testing.T) {
	var statement string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
		func(_, actual string) error {
			statement = actual
			return nil
		})))
	require.NoError(t, err)
	defer db.Close()
	repo := NewMCPServerRepository(db)

	verifiedAt := time.Now().UTC().Add(-time.Hour)
	snapshot := &domain.MCPServer{
		ID:              uuid.New(),
		Name:            "files",
		Description:     "file access",
		URL:             "https://mcp.example.com",
		Version:         "1.2.0",
		PublicKey:       "FAKE-PUBLIC-KEY",
		Status:          domain.MCPServerStatusVerified,
		IsVerified:      true,
		LastVerifiedAt:  &verifiedAt,
		VerificationURL: "https://mcp.example.com/.well-known/mcp/verify",
		Capabilities:    []string{"tools"},
	}
	mock.ExpectQuery("UPDATE mcp_servers").
		WithArgs(snapshot.Name, snapshot.Description, snapshot.URL, snapshot.Version,
			snapshot.PublicKey, snapshot.VerificationURL, []byte(`["tools"]`),
			sqlmock.AnyArg(), snapshot.ID).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(time.Now().UTC()))

	require.NoError(t, repo.Update(snapshot))
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t,
		[]string{"name", "description", "url", "version", "public_key", "verification_url", "capabilities", "updated_at"},
		setColumns(t, statement))
}

// setColumns returns the columns an UPDATE statement's SET list writes, in order.
func setColumns(t *testing.T, statement string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?s)\bSET\b(.*?)\bWHERE\b`).FindStringSubmatch(statement)
	require.Len(t, m, 2, "an UPDATE with a SET list and a WHERE clause: %s", statement)
	var columns []string
	for _, assignment := range strings.Split(m[1], ",") {
		column, _, found := strings.Cut(assignment, "=")
		require.True(t, found, "an assignment: %q", assignment)
		columns = append(columns, strings.TrimSpace(column))
	}
	return columns
}

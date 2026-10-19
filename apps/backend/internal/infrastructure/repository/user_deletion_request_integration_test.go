//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A user asks for the deletion of their own account, held against PostgreSQL
// with every migration applied. UserRepository.RequestDeletion deactivates the
// account and records the request in users.deletion_requested_at (migration
// 139); PurgeDeletionRequests deletes the accounts whose request is due with
// DELETE FROM users.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserDeletion ./internal/infrastructure/repository/...

const userDeletionRequestMigration = "139_user_deletion_requests.sql"

// The account of a member who created an agent, an MCP server and an API key
// is deactivated by the request and kept until the request is due; the purge
// then deletes it, keeps the agent and the MCP server without a creator, and
// deletes the API key with the user.
func TestUserDeletionRequestDeactivatesTheAccountAndThePurgeDeletesItWhenDue(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-request")
	member := userDeleteSeedUser(t, db, ctx, f.orgID, &f.userID)
	name, email := userDeleteNameAndEmail(t, db, ctx, member)
	agentID := userDeleteCopiesAgent(t, db, ctx, NewAgentRepository(db), f.orgID, member, name, email)
	serverID := userDeleteCopiesServer(t, db, ctx, NewMCPServerRepository(db), f.orgID, member, name, email)
	keyID := userDeleteInsertAPIKey(t, db, ctx, f, member)

	at := time.Now().UTC().Add(-40 * 24 * time.Hour).Truncate(time.Microsecond)
	deletion, err := repo.RequestDeletion(ctx, member, at)
	require.NoError(t, err)
	assert.Equal(t, member, deletion.UserID)
	assert.Equal(t, f.orgID, deletion.OrganizationID)
	assert.True(t, deletion.RequestedAt.Equal(at), "requested at %s, want %s", deletion.RequestedAt, at)

	status, requestedAt := userDeletionState(t, db, ctx, member)
	assert.Equal(t, "deactivated", status)
	require.True(t, requestedAt.Valid, "the request must be recorded")
	assert.True(t, requestedAt.Time.Equal(at))

	state, err := repo.GetSessionState(ctx, member)
	require.NoError(t, err)
	assert.True(t, state.Deactivated, "the account's sessions must be refused")
	user, err := repo.GetByID(member)
	require.NoError(t, err)
	assert.False(t, user.CanHoldSession(), "refresh must refuse the account")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM api_keys WHERE id = $1 AND is_active`, keyID),
		"the request does not revoke the account's API keys")

	// A row of the member's own audit trail, as the request's audit row is.
	auditRow := &domain.AuditLog{
		OrganizationID: f.orgID,
		UserID:         &member,
		Action:         domain.AuditActionAccountDeletionRequested,
		ResourceType:   "user",
		ResourceID:     member,
		Metadata:       map[string]interface{}{},
	}
	require.NoError(t, NewAuditLogRepository(db).Create(auditRow))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE id = $1`, auditRow.ID) })

	notDue, err := repo.PurgeDeletionRequests(ctx, at.Add(-time.Microsecond), domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	assert.NotContains(t, userDeletionIDs(notDue), member, "a request made after requestedBy is not due")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, member))

	due, err := repo.PurgeDeletionRequests(ctx, at, domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	mine := userDeletionsOfOrg(due, f.orgID)
	require.Len(t, mine, 1, "the purge deletes the due request")
	assert.Equal(t, member, mine[0].UserID)
	assert.True(t, mine[0].RequestedAt.Equal(at))

	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, member), "the account is deleted")
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, f.userID), "the admin is kept")
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM api_keys WHERE id = $1`, keyID), "the API key goes with its user")
	assert.Equal(t, 0, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM audit_logs WHERE id = $1`, auditRow.ID),
		"the user's own audit rows go with the user, as at the admin's delete")

	agent, err := NewAgentRepository(db).GetByID(agentID)
	require.NoError(t, err, "the agent is the organization's and is kept")
	assert.Equal(t, uuid.Nil, agent.CreatedBy)
	assert.Equal(t, "", agent.CreatedByEmail)
	server, err := NewMCPServerRepository(db).GetByID(serverID)
	require.NoError(t, err, "the MCP server is the organization's and is kept")
	assert.Equal(t, uuid.Nil, server.CreatedBy)
	assert.Equal(t, "", server.CreatedByEmail)
}

// The organization's only active admin is refused while the organization has
// another user who is not deactivated and no other active admin; nothing is
// written then. Each case is an organization of its own whose first user is an
// active admin who asks.
func TestUserDeletionRequestOfTheOnlyActiveAdmin(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	type other struct{ role, status string }
	cases := []struct {
		name    string
		others  []other
		refused bool
	}{
		{"alone in the organization", nil, false},
		{"with an active member", []other{{"member", "active"}}, true},
		{"with a pending member", []other{{"member", "pending"}}, true},
		{"with a suspended member", []other{{"member", "suspended"}}, true},
		{"with a deactivated member only", []other{{"member", "deactivated"}}, false},
		{"with an active member and another active admin", []other{{"member", "active"}, {"admin", "active"}}, false},
		{"with an active member and a suspended admin", []other{{"member", "active"}, {"admin", "suspended"}}, true},
		{"with an active member and a pending admin", []other{{"member", "active"}, {"admin", "pending"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := seedTenant(t, db, ctx, "deletion-admin")
			for _, o := range c.others {
				userDeletionSeedUser(t, db, ctx, f.orgID, o.role, o.status)
			}

			_, err := repo.RequestDeletion(ctx, f.userID, time.Now().UTC())
			status, requestedAt := userDeletionState(t, db, ctx, f.userID)
			if c.refused {
				assert.True(t, errors.Is(err, domain.ErrLastActiveAdmin), "got %v", err)
				assert.Equal(t, "active", status, "a refused request changes nothing")
				assert.False(t, requestedAt.Valid, "a refused request records nothing")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "deactivated", status)
			assert.True(t, requestedAt.Valid)
		})
	}

	t.Run("a member is not held back by the admins", func(t *testing.T) {
		f := seedTenant(t, db, ctx, "deletion-member")
		member := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
		_, err := repo.RequestDeletion(ctx, member, time.Now().UTC())
		require.NoError(t, err)
	})
}

// Two active admins of an organization with a member ask at the same time:
// one request is recorded and the other is refused, so the organization keeps
// an active admin. Each round is an organization of its own.
func TestUserDeletionRequestsOfTwoAdminsAtOnceLeaveAnAdmin(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	for round := 0; round < 10; round++ {
		f := seedTenant(t, db, ctx, "deletion-race")
		second := userDeletionSeedUser(t, db, ctx, f.orgID, "admin", "active")
		userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		for i, id := range []uuid.UUID{f.userID, second} {
			wg.Add(1)
			go func(i int, id uuid.UUID) {
				defer wg.Done()
				<-start
				_, errs[i] = repo.RequestDeletion(ctx, id, time.Now().UTC())
			}(i, id)
		}
		close(start)
		wg.Wait()

		recorded, refused := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				recorded++
			case errors.Is(err, domain.ErrLastActiveAdmin):
				refused++
			default:
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		assert.Equal(t, 1, recorded, "round %d", round)
		assert.Equal(t, 1, refused, "round %d", round)
		assert.Equal(t, 1, userDeletionCount(t, db, ctx,
			`SELECT COUNT(*) FROM users WHERE organization_id = $1 AND role = 'admin' AND status = 'active'`, f.orgID),
			"round %d: the organization keeps an active admin", round)
	}
}

// An account that is deactivated already, and a user that does not exist, are
// refused.
func TestUserDeletionRequestRefusesADeactivatedOrUnknownAccount(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-refused")
	deactivated := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "deactivated")
	_, err := repo.RequestDeletion(ctx, deactivated, time.Now().UTC())
	assert.True(t, errors.Is(err, domain.ErrAccountDeactivated), "got %v", err)
	_, requestedAt := userDeletionState(t, db, ctx, deactivated)
	assert.False(t, requestedAt.Valid)

	_, err = repo.RequestDeletion(ctx, uuid.New(), time.Now().UTC())
	assert.True(t, errors.Is(err, domain.ErrUserNotFound), "got %v", err)
}

// An admin who activates the account withdraws the request: the trigger of
// migration 139 clears it when UserRepository.Update, the write
// AdminService.ActivateUser makes, sets the status to active. A later
// deactivation, through the same write as AuthService.DeactivateUser, which
// the admin's deactivate route calls, does not bring it back, and the purge
// keeps the account.
func TestUserDeletionRequestIsWithdrawnWhenTheAccountIsActivated(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-withdrawn")
	member := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
	at := time.Now().UTC().Add(-40 * 24 * time.Hour).Truncate(time.Microsecond)
	_, err := repo.RequestDeletion(ctx, member, at)
	require.NoError(t, err)

	user, err := repo.GetByID(member)
	require.NoError(t, err)
	user.Status = domain.UserStatusActive
	require.NoError(t, repo.Update(user))
	status, requestedAt := userDeletionState(t, db, ctx, member)
	assert.Equal(t, "active", status)
	assert.False(t, requestedAt.Valid, "activating the account withdraws the request")

	user.Status = domain.UserStatusDeactivated
	require.NoError(t, repo.Update(user))
	status, requestedAt = userDeletionState(t, db, ctx, member)
	assert.Equal(t, "deactivated", status)
	assert.False(t, requestedAt.Valid, "a later deactivation does not bring the request back")

	deleted, err := repo.PurgeDeletionRequests(ctx, time.Now().UTC(), domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	assert.NotContains(t, userDeletionIDs(deleted), member)
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, member))

	// The trigger keeps a request on a row that stays deactivated, and clears
	// one written beside any other status.
	_, err = db.ExecContext(ctx, `UPDATE users SET deletion_requested_at = $2 WHERE id = $1`, member, at)
	require.NoError(t, err)
	_, requestedAt = userDeletionState(t, db, ctx, member)
	assert.True(t, requestedAt.Valid, "a deactivated account keeps its request")
	_, err = db.ExecContext(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1`, member)
	require.NoError(t, err)
	_, requestedAt = userDeletionState(t, db, ctx, member)
	assert.False(t, requestedAt.Valid, "a suspended account holds no request")
}

// The purge deletes only accounts whose user asked: an account an admin
// deactivated is kept, however long ago.
func TestUserDeletionPurgeKeepsAnAccountAnAdminDeactivated(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-admin-deactivated")
	member := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
	user, err := repo.GetByID(member)
	require.NoError(t, err)
	user.Status = domain.UserStatusDeactivated
	require.NoError(t, repo.Update(user))
	_, err = db.ExecContext(ctx, `UPDATE users SET deleted_at = NOW() - INTERVAL '400 days' WHERE id = $1`, member)
	require.NoError(t, err)

	deleted, err := repo.PurgeDeletionRequests(ctx, time.Now().UTC(), domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	assert.NotContains(t, userDeletionIDs(deleted), member)
	assert.Equal(t, 1, userDeletionCount(t, db, ctx, `SELECT COUNT(*) FROM users WHERE id = $1`, member))
}

// A purge skips the accounts another purge holds locked, and deletes them
// once the lock is gone; with a limit it deletes the oldest requests first.
func TestUserDeletionPurgeSkipsLockedAccountsAndTakesTheOldestFirst(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-purge-order")
	base := time.Now().UTC().Add(-400 * 24 * time.Hour).Truncate(time.Microsecond)
	members := make([]uuid.UUID, 3)
	for i := range members {
		members[i] = userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
		_, err := repo.RequestDeletion(ctx, members[i], base.Add(time.Duration(i)*time.Hour))
		require.NoError(t, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, members[0])
	require.NoError(t, err)

	// Only the requests of this organization are older than base + 2h in a
	// database the other tests leave clean of requests that old, so the limit
	// of 1 picks among these three.
	first, err := repo.PurgeDeletionRequests(ctx, base.Add(2*time.Hour), 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, members[1], first[0].UserID, "the oldest request not held by another purge goes first")

	rest, err := repo.PurgeDeletionRequests(ctx, base.Add(2*time.Hour), domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{members[2]}, userDeletionIDs(userDeletionsOfOrg(rest, f.orgID)), "the locked account is skipped")

	require.NoError(t, tx.Rollback())
	last, err := repo.PurgeDeletionRequests(ctx, base.Add(2*time.Hour), domain.AccountDeletionPurgeBatch)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{members[0]}, userDeletionIDs(userDeletionsOfOrg(last, f.orgID)), "the account is deleted once the lock is gone")
}

// GetSessionState reads an account as deactivated when its status is
// deactivated and when it carries deleted_at, the two states sign-in refuses.
func TestUserDeletionSessionStateReadsDeactivation(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-session-state")
	cases := []struct {
		name, update string
		deactivated  bool
	}{
		{"active", ``, false},
		{"pending", `UPDATE users SET status = 'pending' WHERE id = $1`, false},
		{"suspended", `UPDATE users SET status = 'suspended' WHERE id = $1`, false},
		{"deactivated", `UPDATE users SET status = 'deactivated' WHERE id = $1`, true},
		{"active with deleted_at", `UPDATE users SET deleted_at = NOW() WHERE id = $1`, true},
	}
	for _, c := range cases {
		id := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
		if c.update != "" {
			_, err := db.ExecContext(ctx, c.update, id)
			require.NoError(t, err)
		}
		state, err := repo.GetSessionState(ctx, id)
		require.NoError(t, err, c.name)
		assert.Equal(t, c.deactivated, state.Deactivated, c.name)
	}
}

// Running migration 139 again on a migrated database keeps the column, the
// index and the requests, and leaves one trigger. It runs in a transaction
// that is rolled back, so the shared database is left as it was.
func TestUserDeletionMigrationRunsAgain(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	repo := NewUserRepository(db)

	f := seedTenant(t, db, ctx, "deletion-migration")
	member := userDeletionSeedUser(t, db, ctx, f.orgID, "member", "active")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, err := repo.RequestDeletion(ctx, member, at)
	require.NoError(t, err)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, userDeleteReadMigration(t, userDeletionRequestMigration))
	require.NoError(t, err, "the migration runs again")

	var triggers, indexes int
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pg_trigger WHERE tgrelid = 'users'::regclass AND tgname = 'trg_user_deletion_request_follows_status'`).Scan(&triggers))
	assert.Equal(t, 1, triggers)
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pg_indexes WHERE tablename = 'users' AND indexname = 'idx_users_deletion_requested_at'`).Scan(&indexes))
	assert.Equal(t, 1, indexes)
	var requestedAt sql.NullTime
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT deletion_requested_at FROM users WHERE id = $1`, member).Scan(&requestedAt))
	assert.True(t, requestedAt.Valid && requestedAt.Time.Equal(at), "the request is kept")
}

// userDeletionSeedUser adds a user with the role and status to org and returns
// its id. seedTenant's cleanup deletes it with the organization's users.
func userDeletionSeedUser(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID, role, status string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	suffix := id.String()[:8]
	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role, status,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, 'deletion request colleague', 'x', $4, $5, 'local', $6, NOW(), NOW())`,
		id, orgID, "deletion-"+suffix+"@example.com", role, status, "local-"+suffix)
	require.NoError(t, err)
	return id
}

// userDeletionState reads a user's status and deletion_requested_at.
func userDeletionState(t *testing.T, db *sql.DB, ctx context.Context, id uuid.UUID) (string, sql.NullTime) {
	t.Helper()

	var status string
	var requestedAt sql.NullTime
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status, deletion_requested_at FROM users WHERE id = $1`, id).Scan(&status, &requestedAt))
	return status, requestedAt
}

func userDeletionCount(t *testing.T, db *sql.DB, ctx context.Context, query string, args ...interface{}) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRowContext(ctx, query, args...).Scan(&n))
	return n
}

func userDeletionIDs(deleted []domain.AccountDeletion) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(deleted))
	for _, d := range deleted {
		ids = append(ids, d.UserID)
	}
	return ids
}

// userDeletionsOfOrg keeps the deletions of one organization: the purge reads
// every organization's requests, and another test's may be due too.
func userDeletionsOfOrg(deleted []domain.AccountDeletion, orgID uuid.UUID) []domain.AccountDeletion {
	var out []domain.AccountDeletion
	for _, d := range deleted {
		if d.OrganizationID == orgID {
			out = append(out, d)
		}
	}
	return out
}

//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Deleting a user deletes the registration requests filed with the user's email, and
// takes the user's email and name out of the audit rows an admin wrote about the user.
// Approving a registration request copies its email into the user it creates and keeps
// the request, with the names it was filed with and the password hash and signup
// answers when the registration gave them; an admin's role change writes the user's
// email into its audit row, and a registration approval the email and the name. None of these rows names the user by a foreign key, so deleting
// the user kept them. Migration 137 deletes and clears them in the statement that
// deletes the user.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run 'TestUserDelete(Trigger|Backfill)' ./internal/infrastructure/repository/...

const userDeleteRequestsMigration = "137_user_delete_removes_registration_requests_and_audit_copies.sql"

// Against every migration applied: deleting a user through UserRepository.Delete
// deletes the requests filed with the user's email that hold the user's organization or
// none, whatever the case of the email, and keeps a request with that email filed with
// another organization and another person's request; the user's role change and
// registration approval rows lose user_email and user_name and keep the rest of their
// metadata, and another user's rows and another organization's row stay as written.
func TestUserDeleteTriggerRemovesTheUsersRegistrationRequestsAndAuditCopies(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, ctx, "user-delete-requests")
	other := seedTenant(t, db, ctx, "user-delete-requests-other")
	admin := f.userID
	leavingEmail := userDeleteUniqueEmail("leaving")
	stayingEmail := userDeleteUniqueEmail("staying")
	leaving := userDeleteSeedNamedUser(t, db, ctx, f.orgID, leavingEmail, "Leaving Requester")
	staying := userDeleteSeedNamedUser(t, db, ctx, f.orgID, stayingEmail, "Staying Requester")

	// The request the leaving user was approved from, as the approval leaves it; an
	// earlier request of the same person, rejected, with the email in capitals; and a
	// pending request that holds the user's organization.
	approved := userDeleteFileRequest(t, db, ctx, leavingEmail, nil, domain.RegistrationStatusApproved, admin)
	rejected := userDeleteFileRequest(t, db, ctx, strings.ToUpper(leavingEmail), nil, domain.RegistrationStatusRejected, admin)
	pendingHere := userDeleteFileRequest(t, db, ctx, leavingEmail, &f.orgID, domain.RegistrationStatusPending, admin)
	// Kept: the same email's request filed with another organization, and another
	// person's request.
	pendingElsewhere := userDeleteFileRequest(t, db, ctx, leavingEmail, &other.orgID, domain.RegistrationStatusPending, admin)
	ofStaying := userDeleteFileRequest(t, db, ctx, stayingEmail, nil, domain.RegistrationStatusApproved, admin)

	// The rows the admin handlers write: UpdateUserRole and ApproveRegistrationRequest.
	roleChange := userDeleteWriteAuditRow(t, db, ctx, f.orgID, admin, "user_role", leaving,
		map[string]interface{}{"user_email": leavingEmail, "new_role": "manager"})
	approval := userDeleteWriteAuditRow(t, db, ctx, f.orgID, admin, "registration_approval", approved,
		map[string]interface{}{"action": "approved", "user_email": leavingEmail, "user_name": "Leaving Requester"})
	stayingRoleChange := userDeleteWriteAuditRow(t, db, ctx, f.orgID, admin, "user_role", staying,
		map[string]interface{}{"user_email": stayingEmail, "new_role": "viewer"})
	stayingApproval := userDeleteWriteAuditRow(t, db, ctx, f.orgID, admin, "registration_approval", ofStaying,
		map[string]interface{}{"action": "approved", "user_email": stayingEmail, "user_name": "Staying Requester"})
	otherOrgApproval := userDeleteWriteAuditRow(t, db, ctx, other.orgID, other.userID, "registration_approval", pendingElsewhere,
		map[string]interface{}{"action": "approved", "user_email": leavingEmail, "user_name": "Leaving Requester"})

	// Control: before the delete the rows hold the copies they were written with.
	require.Equal(t, leavingEmail, userDeleteAuditMetadata(t, db, roleChange)["user_email"],
		"control: the role change row must hold the user's email before the delete")
	for _, id := range []uuid.UUID{approved, rejected, pendingHere} {
		require.True(t, userDeleteRequestExists(t, db, ctx, id), "control: request %s must exist before the delete", id)
	}

	require.NoError(t, NewUserRepository(db).Delete(leaving), "deleting the user must succeed")

	assert.False(t, userDeleteRequestExists(t, db, ctx, approved),
		"the request the deleted user was approved from must be deleted")
	assert.False(t, userDeleteRequestExists(t, db, ctx, rejected),
		"a request filed with the deleted user's email in other case must be deleted")
	assert.False(t, userDeleteRequestExists(t, db, ctx, pendingHere),
		"a request that holds the deleted user's organization must be deleted")
	assert.True(t, userDeleteRequestExists(t, db, ctx, pendingElsewhere),
		"a request filed with another organization is that organization's, and must be kept")
	assert.True(t, userDeleteRequestExists(t, db, ctx, ofStaying), "another person's request must be kept")

	assert.Equal(t, map[string]interface{}{"new_role": "manager"}, userDeleteAuditMetadata(t, db, roleChange),
		"the role change row must keep its record and lose the deleted user's email")
	assert.Equal(t, map[string]interface{}{"action": "approved"}, userDeleteAuditMetadata(t, db, approval),
		"the registration approval row must keep its record and lose the deleted user's email and name")
	assert.Equal(t, map[string]interface{}{"user_email": stayingEmail, "new_role": "viewer"},
		userDeleteAuditMetadata(t, db, stayingRoleChange), "another user's role change row must be kept as written")
	assert.Equal(t, map[string]interface{}{"action": "approved", "user_email": stayingEmail, "user_name": "Staying Requester"},
		userDeleteAuditMetadata(t, db, stayingApproval), "another user's registration approval row must be kept as written")
	assert.Equal(t, map[string]interface{}{"action": "approved", "user_email": leavingEmail, "user_name": "Leaving Requester"},
		userDeleteAuditMetadata(t, db, otherOrgApproval), "another organization's row must be kept as written")
}

// A request that holds no organization cannot say which of two users with its email
// it was filed for, so it is kept while either remains, and deleted with the last one.
// Users of two organizations may share an email; the second one here has it in
// capitals.
func TestUserDeleteTriggerKeepsARequestWhileAnotherUserHasItsEmail(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, ctx, "user-delete-shared-email")
	other := seedTenant(t, db, ctx, "user-delete-shared-email-other")
	email := userDeleteUniqueEmail("shared")
	first := userDeleteSeedNamedUser(t, db, ctx, f.orgID, email, "Shared Email First")
	second := userDeleteSeedNamedUser(t, db, ctx, other.orgID, strings.ToUpper(email), "Shared Email Second")
	request := userDeleteFileRequest(t, db, ctx, email, nil, domain.RegistrationStatusApproved, f.userID)

	require.NoError(t, NewUserRepository(db).Delete(first), "deleting the first user must succeed")
	assert.True(t, userDeleteRequestExists(t, db, ctx, request),
		"a request that holds no organization must be kept while another user has its email")

	require.NoError(t, NewUserRepository(db).Delete(second), "deleting the second user must succeed")
	assert.False(t, userDeleteRequestExists(t, db, ctx, request),
		"the request must be deleted with the last user that has its email")
}

// Deleting an organization deletes its users in one statement, and with each of them
// the registration requests filed with its email that hold no organization, which the
// organization's own cascade does not reach.
func TestUserDeleteTriggerRemovesTheRequestsOfADeletedOrganizationsUsers(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, ctx, "user-delete-org")
	email := userDeleteUniqueEmail("member")
	member := userDeleteSeedNamedUser(t, db, ctx, f.orgID, email, "Organization Member")
	approved := userDeleteFileRequest(t, db, ctx, email, nil, domain.RegistrationStatusApproved, f.userID)
	held := userDeleteFileRequest(t, db, ctx, email, &f.orgID, domain.RegistrationStatusPending, f.userID)
	roleChange := userDeleteWriteAuditRow(t, db, ctx, f.orgID, f.userID, "user_role", member,
		map[string]interface{}{"user_email": email, "new_role": "manager"})
	approval := userDeleteWriteAuditRow(t, db, ctx, f.orgID, f.userID, "registration_approval", approved,
		map[string]interface{}{"action": "approved", "user_email": email, "user_name": "Organization Member"})

	_, err := db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, f.orgID)
	require.NoError(t, err, "deleting the organization must succeed")

	var users int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id IN ($1, $2)`, f.userID, member).Scan(&users))
	require.Equal(t, 0, users, "the organization's users must be deleted with it")
	assert.False(t, userDeleteRequestExists(t, db, ctx, approved),
		"a request with a member's email that holds no organization must be deleted with the organization's users")
	assert.False(t, userDeleteRequestExists(t, db, ctx, held), "a request that holds the organization must be deleted with it")
	var rows int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE id IN ($1, $2)`, roleChange, approval).Scan(&rows))
	assert.Equal(t, 0, rows, "the organization's audit rows must be deleted with it")
}

// Migration 137 deletes, once, the approved requests whose email no user has, and takes
// user_email and user_name out of the audit rows about users deleted before it ran:
// 'user' and 'user_role' rows whose resource_id names no user, among them the rows the
// admin's permanent delete wrote, and 'registration_approval' rows whose user_email no
// user of the row's organization has. The schema stands in for a database with every
// migration through 136 applied, with the columns migration 137 reads and writes. The
// migration runs from the file the repository ships, twice; then a delete of the
// remaining user deletes and clears that user's rows through the trigger it added.
func TestUserDeleteBackfillRemovesRequestsAndAuditCopiesThatOutlivedTheirUser(t *testing.T) {
	db := userDeleteStandInSchema(t)

	for _, ddl := range []string{
		`CREATE TABLE users (id UUID PRIMARY KEY, organization_id UUID NOT NULL, email VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE user_registration_requests (id UUID PRIMARY KEY, email VARCHAR(255) NOT NULL, organization_id UUID, status VARCHAR(50) NOT NULL)`,
		`CREATE TABLE audit_logs (id UUID PRIMARY KEY, organization_id UUID NOT NULL, resource_type VARCHAR(50) NOT NULL, resource_id UUID NOT NULL, metadata JSONB)`,
	} {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}

	org, otherOrg := uuid.New(), uuid.New()
	staying := uuid.New()
	gone := uuid.New() // an id no user has: the user it named was deleted
	_, err := db.Exec(`INSERT INTO users VALUES ($1, $2, 'staying@example.com', 'Staying User')`, staying, org)
	require.NoError(t, err)

	requests := map[string]uuid.UUID{}
	for _, r := range []struct{ name, email, status string }{
		{"approved, no user has its email", "deleted@example.com", "approved"},
		{"approved, a user has its email in other case", "Staying@Example.com", "approved"},
		{"pending, no user has its email", "deleted@example.com", "pending"},
		{"rejected, no user has its email", "deleted@example.com", "rejected"},
	} {
		requests[r.name] = uuid.New()
		_, err := db.Exec(`INSERT INTO user_registration_requests VALUES ($1, $2, NULL, $3)`, requests[r.name], r.email, r.status)
		require.NoError(t, err)
	}

	deletedUserCopies := `{"user_email": "deleted@example.com", "user_name": "Deleted User"}`
	stayingUserCopies := `{"user_email": "staying@example.com", "user_name": "Staying User"}`
	rows := []struct {
		name, org, resourceType string
		resourceID              uuid.UUID
		metadata                string
		want                    map[string]interface{}
	}{
		{"permanent delete row of a deleted user", "org", "user", gone,
			`{"action": "permanent_delete", "user_email": "deleted@example.com", "user_name": "Deleted User", "warning": "irreversible_hard_delete"}`,
			map[string]interface{}{"action": "permanent_delete", "warning": "irreversible_hard_delete"}},
		{"role change row of a deleted user", "org", "user_role", gone,
			`{"user_email": "deleted@example.com", "new_role": "manager"}`,
			map[string]interface{}{"new_role": "manager"}},
		{"approval row naming no user of its organization", "org", "registration_approval", uuid.New(),
			`{"action": "approved", "user_email": "deleted@example.com", "user_name": "Deleted User"}`,
			map[string]interface{}{"action": "approved"}},
		{"approval row naming a user of another organization", "otherOrg", "registration_approval", uuid.New(),
			`{"action": "approved", "user_email": "staying@example.com", "user_name": "Staying User"}`,
			map[string]interface{}{"action": "approved"}},
		{"role change row of the staying user", "org", "user_role", staying,
			`{"user_email": "staying@example.com", "new_role": "viewer"}`,
			map[string]interface{}{"user_email": "staying@example.com", "new_role": "viewer"}},
		{"approval row of the staying user", "org", "registration_approval", uuid.New(),
			`{"action": "approved", "user_email": "staying@example.com", "user_name": "Staying User"}`,
			map[string]interface{}{"action": "approved", "user_email": "staying@example.com", "user_name": "Staying User"}},
		{"row of another resource type", "org", "agent", gone, deletedUserCopies,
			map[string]interface{}{"user_email": "deleted@example.com", "user_name": "Deleted User"}},
		{"user row of the staying user", "org", "user", staying, stayingUserCopies,
			map[string]interface{}{"user_email": "staying@example.com", "user_name": "Staying User"}},
	}
	ids := make(map[string]uuid.UUID, len(rows))
	for _, r := range rows {
		ids[r.name] = uuid.New()
		rowOrg := org
		if r.org == "otherOrg" {
			rowOrg = otherOrg
		}
		_, err := db.Exec(`INSERT INTO audit_logs VALUES ($1, $2, $3, $4, $5::jsonb)`, ids[r.name], rowOrg, r.resourceType, r.resourceID, r.metadata)
		require.NoError(t, err)
	}

	// Control: the permanent delete row holds the copies before the migration runs.
	require.Equal(t, "deleted@example.com", userDeleteStandInMetadata(t, db, ids["permanent delete row of a deleted user"])["user_email"],
		"control: the permanent delete row must hold the deleted user's email before the migration")

	migration := userDeleteReadMigration(t, userDeleteRequestsMigration)
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply")
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply again")

	assert.False(t, userDeleteStandInRequestExists(t, db, requests["approved, no user has its email"]),
		"an approved request whose email no user has must be deleted")
	assert.True(t, userDeleteStandInRequestExists(t, db, requests["approved, a user has its email in other case"]),
		"an approved request whose email a user has, in any case, must be kept")
	assert.True(t, userDeleteStandInRequestExists(t, db, requests["pending, no user has its email"]),
		"a pending request names no user, and must be kept")
	assert.True(t, userDeleteStandInRequestExists(t, db, requests["rejected, no user has its email"]),
		"a rejected request names no user, and must be kept")
	for _, r := range rows {
		assert.Equal(t, r.want, userDeleteStandInMetadata(t, db, ids[r.name]), "%s", r.name)
	}

	_, err = db.Exec(`DELETE FROM users WHERE id = $1`, staying)
	require.NoError(t, err, "deleting the remaining user must succeed")
	assert.False(t, userDeleteStandInRequestExists(t, db, requests["approved, a user has its email in other case"]),
		"once its user is deleted, the request filed with the user's email must be deleted")
	assert.Equal(t, map[string]interface{}{"new_role": "viewer"},
		userDeleteStandInMetadata(t, db, ids["role change row of the staying user"]),
		"once its user is deleted, the role change row must lose the user's email")
	assert.Equal(t, map[string]interface{}{"action": "approved"},
		userDeleteStandInMetadata(t, db, ids["approval row of the staying user"]),
		"once its user is deleted, the approval row must lose the user's email and name")
	assert.Equal(t, map[string]interface{}{},
		userDeleteStandInMetadata(t, db, ids["user row of the staying user"]),
		"once its user is deleted, a user row about the user must lose the user's email and name")
}

// userDeleteUniqueEmail returns an email no other test uses.
func userDeleteUniqueEmail(label string) string {
	return "user-delete-" + label + "-" + uuid.NewString()[:8] + "@example.com"
}

// userDeleteSeedNamedUser adds a member with the given email and name to org and
// returns its id; the user is deleted when the test ends if it is still there.
func userDeleteSeedNamedUser(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID, email, name string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'member', 'local', $3, NOW(), NOW())`,
		id, orgID, email, name)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id
}

// userDeleteFileRequest files a registration request through
// OAuthRepositoryPostgres.CreateRegistrationRequest, as RegistrationService does, and
// for an approved or rejected status decides it through UpdateRegistrationRequest,
// reviewed by reviewer, as the service's approval and rejection do. The request is
// deleted when the test ends if it is still there.
func userDeleteFileRequest(t *testing.T, db *sql.DB, ctx context.Context, email string, orgID *uuid.UUID, status domain.RegistrationRequestStatus, reviewer uuid.UUID) uuid.UUID {
	t.Helper()

	repo := NewOAuthRepositoryPostgres(sqlx.NewDb(db, "postgres"))
	req := domain.NewUserRegistrationRequestManual(email, "Request", "Filer", "not-a-real-password-hash")
	req.OrganizationID = orgID
	req.Metadata = map[string]interface{}{"reason": "user delete test"}
	require.NoError(t, repo.CreateRegistrationRequest(ctx, req))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM user_registration_requests WHERE id = $1`, req.ID) })

	switch status {
	case domain.RegistrationStatusPending:
		return req.ID
	case domain.RegistrationStatusApproved:
		req.Approve(reviewer)
	case domain.RegistrationStatusRejected:
		req.Reject(reviewer, "user delete test")
	default:
		t.Fatalf("unknown registration status %q", status)
	}
	require.NoError(t, repo.UpdateRegistrationRequest(ctx, req))
	return req.ID
}

// userDeleteRequestExists reports whether the registration request is stored.
func userDeleteRequestExists(t *testing.T, db *sql.DB, ctx context.Context, id uuid.UUID) bool {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_registration_requests WHERE id = $1`, id).Scan(&n))
	return n == 1
}

// userDeleteWriteAuditRow writes an admin's audit row through AuditLogRepository.Create
// with the fields AuditService.LogAction gives it, and returns its id. The row is
// deleted when the test ends if it is still there.
func userDeleteWriteAuditRow(t *testing.T, db *sql.DB, ctx context.Context, orgID, adminID uuid.UUID, resourceType string, resourceID uuid.UUID, metadata map[string]interface{}) uuid.UUID {
	t.Helper()

	entry := &domain.AuditLog{
		OrganizationID: orgID,
		UserID:         &adminID,
		Action:         domain.AuditActionUpdate,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Metadata:       metadata,
	}
	require.NoError(t, NewAuditLogRepository(db).Create(entry))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE id = $1`, entry.ID) })
	return entry.ID
}

// userDeleteAuditMetadata reads an audit row's metadata through
// AuditLogRepository.GetByID; the row must exist.
func userDeleteAuditMetadata(t *testing.T, db *sql.DB, id uuid.UUID) map[string]interface{} {
	t.Helper()

	entry, err := NewAuditLogRepository(db).GetByID(id)
	require.NoError(t, err)
	require.NotNil(t, entry, "the audit row %s must be kept", id)
	return entry.Metadata
}

// userDeleteStandInRequestExists reports whether the stand-in schema holds the
// registration request.
func userDeleteStandInRequestExists(t *testing.T, db *sql.DB, id uuid.UUID) bool {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_registration_requests WHERE id = $1`, id).Scan(&n))
	return n == 1
}

// userDeleteStandInMetadata reads an audit row's metadata in the stand-in schema.
func userDeleteStandInMetadata(t *testing.T, db *sql.DB, id uuid.UUID) map[string]interface{} {
	t.Helper()

	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT metadata FROM audit_logs WHERE id = $1`, id).Scan(&raw))
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

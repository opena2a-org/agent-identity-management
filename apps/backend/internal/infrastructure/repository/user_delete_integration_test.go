//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A user named in another row can be deleted, held against PostgreSQL with every
// migration applied. Eighteen references to users(id) had no ON DELETE rule or had
// ON DELETE RESTRICT, so DELETE FROM users, which UserRepository.Delete runs for the
// admin's permanent delete, failed with a foreign-key violation for a user who had
// created an agent, an API key or an MCP server, among others; two more deleted the
// security policies and the tags the user had created. Migration 135 deletes the
// user's API keys and device codes with the user, and keeps the other rows of these
// twenty references with the reference set to NULL.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserDelete ./internal/infrastructure/repository/...

// userDeleteRules is the rule migration 135 gives each of the twenty references:
// cascade for a credential that acts as its user, SET NULL for a row the
// organization keeps.
var userDeleteRules = []struct {
	table, column string
	cascade       bool
}{
	{"api_keys", "created_by", true},
	{"device_codes", "user_id", true},
	{"agents", "created_by", false},
	{"mcp_servers", "created_by", false},
	{"webhooks", "created_by", false},
	{"compliance_evidence", "collected_by", false},
	{"capability_requests", "requested_by", false},
	{"users", "approved_by", false},
	{"alerts", "acknowledged_by", false},
	{"user_registration_requests", "reviewed_by", false},
	{"agent_capabilities", "granted_by", false},
	{"auth_lockouts", "unlocked_by", false},
	{"a2a_policies", "created_by", false},
	{"a2a_policies", "updated_by", false},
	{"a2a_revoked_agents", "revoked_by", false},
	{"a2a_security_settings", "created_by", false},
	{"a2a_security_settings", "updated_by", false},
	{"a2a_delegation", "created_by", false},
	{"security_policies", "created_by", false},
	{"tags", "created_by", false},
}

func userDeleteTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping user delete integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// Every reference to users(id) lets the user it names be deleted: its ON DELETE rule
// is CASCADE or SET NULL. A reference added with no rule (NO ACTION, the default) or
// with RESTRICT fails here. The twenty references migration 135 changed carry the
// rule it chose.
func TestUserDeleteEveryReferenceToUsersHasADeleteRule(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	rows, err := db.QueryContext(ctx, `
		SELECT c.conrelid::regclass::text, a.attname::text, c.confdeltype::text
		FROM pg_constraint c
		JOIN pg_attribute a
		  ON a.attrelid = c.conrelid
		 AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'f'
		  AND c.confrelid = 'users'::regclass
		ORDER BY 1, 2`)
	require.NoError(t, err)
	defer rows.Close()

	rules := make(map[string]string)
	var refusing []string
	for rows.Next() {
		var table, column, rule string
		require.NoError(t, rows.Scan(&table, &column, &rule))
		rules[table+"."+column] = rule
		if rule != "c" && rule != "n" {
			refusing = append(refusing, fmt.Sprintf("%s.%s (%s)", table, column, userDeleteRuleName(rule)))
		}
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, rules, "the catalog must list the references to users(id), or this test proves nothing")

	assert.Empty(t, refusing,
		"each of these references to users(id) refuses the delete of a user it names; give it ON DELETE CASCADE or ON DELETE SET NULL")
	for _, r := range userDeleteRules {
		want := "n"
		if r.cascade {
			want = "c"
		}
		assert.Equal(t, userDeleteRuleName(want), userDeleteRuleName(rules[r.table+"."+r.column]),
			"the ON DELETE rule of %s.%s", r.table, r.column)
	}
}

// Deleting a user named in all twenty references succeeds. The user's API key and
// device code are deleted with the user; every other row is kept, with its reference
// to the user set to NULL, and a kept tag stays on its agent; another user's API key
// and device code are kept.
func TestUserDeleteKeepsTheOrganizationsRowsAndDeletesTheUsersCredentials(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, ctx, "user-delete")
	deleted := f.userID
	colleague := userDeleteSeedUser(t, db, ctx, f.orgID, &deleted)
	revokedAgent := seedExtraAgent(t, db, ctx, f)
	delegateAgent := seedExtraAgent(t, db, ctx, f)
	suffix := f.orgID.String()[:8]

	type rowKey struct {
		keyColumn string
		key       uuid.UUID
	}
	named := map[string]rowKey{
		"agents.created_by": {"id", f.agentID},
		"users.approved_by": {"id", colleague},
	}
	name := func(table, column, keyColumn string, key uuid.UUID) {
		named[table+"."+column] = rowKey{keyColumn, key}
	}

	name("api_keys", "created_by", "id", userDeleteInsertAPIKey(t, db, ctx, f, deleted))
	colleagueKey := userDeleteInsertAPIKey(t, db, ctx, f, colleague)
	name("device_codes", "user_id", "id", userDeleteInsertDeviceCode(t, db, ctx, f.orgID, deleted))
	colleagueCode := userDeleteInsertDeviceCode(t, db, ctx, f.orgID, colleague)

	name("mcp_servers", "created_by", "id", userDeleteInsert(t, db, ctx, "mcp_servers", "id",
		`INSERT INTO mcp_servers (organization_id, name, url, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		f.orgID, "user-delete-mcp-"+suffix, "https://mcp-"+suffix+".example.com", deleted))
	name("webhooks", "created_by", "id", userDeleteInsert(t, db, ctx, "webhooks", "id",
		`INSERT INTO webhooks (organization_id, name, url, events, secret, created_by)
		 VALUES ($1, $2, 'https://hooks.example.com/user-delete', ARRAY['agent.created'], 'not-a-real-secret', $3)
		 RETURNING id`,
		f.orgID, "user-delete-webhook-"+suffix, deleted))
	name("compliance_evidence", "collected_by", "id", userDeleteInsert(t, db, ctx, "compliance_evidence", "id",
		`INSERT INTO compliance_evidence (organization_id, framework, check_name, evidence_type, title, collected_by)
		 VALUES ($1, 'soc2', 'user-delete', 'manual', 'collected by the user the test deletes', $2)
		 RETURNING id`,
		f.orgID, deleted))
	name("capability_requests", "requested_by", "id", userDeleteInsert(t, db, ctx, "capability_requests", "id",
		`INSERT INTO capability_requests (agent_id, capability_type, reason, requested_by)
		 VALUES ($1, 'file:read', 'requested by the user the test deletes', $2) RETURNING id`,
		f.agentID, deleted))
	name("alerts", "acknowledged_by", "id", userDeleteInsert(t, db, ctx, "alerts", "id",
		`INSERT INTO alerts (organization_id, alert_type, severity, title, is_acknowledged, acknowledged_by, acknowledged_at)
		 VALUES ($1, 'user_delete_test', 'low', 'acknowledged by the user the test deletes', true, $2, NOW())
		 RETURNING id`,
		f.orgID, deleted))
	name("user_registration_requests", "reviewed_by", "id", userDeleteInsert(t, db, ctx, "user_registration_requests", "id",
		`INSERT INTO user_registration_requests (email, first_name, last_name, organization_id, status, reviewed_by, reviewed_at)
		 VALUES ($1, 'Reviewed', 'Request', $2, 'approved', $3, NOW()) RETURNING id`,
		"user-delete-request-"+suffix+"@example.com", f.orgID, deleted))
	name("agent_capabilities", "granted_by", "id", userDeleteInsert(t, db, ctx, "agent_capabilities", "id",
		`INSERT INTO agent_capabilities (agent_id, capability_type, granted_by) VALUES ($1, 'file:read', $2) RETURNING id`,
		f.agentID, deleted))
	name("auth_lockouts", "unlocked_by", "id", userDeleteInsert(t, db, ctx, "auth_lockouts", "id",
		`INSERT INTO auth_lockouts (organization_id, email, locked_until, unlocked_at, unlocked_by)
		 VALUES ($1, $2, NOW() - INTERVAL '1 hour', NOW(), $3) RETURNING id`,
		f.orgID, "user-delete-lockout-"+suffix+"@example.com", deleted))
	policy := userDeleteInsert(t, db, ctx, "a2a_policies", "id",
		`INSERT INTO a2a_policies (organization_id, name, policy_yaml, created_by, updated_by)
		 VALUES ($1, $2, 'rules: []', $3, $3) RETURNING id`,
		f.orgID, "user-delete-policy-"+suffix, deleted)
	name("a2a_policies", "created_by", "id", policy)
	name("a2a_policies", "updated_by", "id", policy)
	name("a2a_revoked_agents", "revoked_by", "agent_id", userDeleteInsert(t, db, ctx, "a2a_revoked_agents", "agent_id",
		`INSERT INTO a2a_revoked_agents (agent_id, reason, revoked_by)
		 VALUES ($1, 'revoked by the user the test deletes', $2) RETURNING agent_id`,
		revokedAgent, deleted))
	settings := userDeleteInsert(t, db, ctx, "a2a_security_settings", "id",
		`INSERT INTO a2a_security_settings (organization_id, created_by, updated_by) VALUES ($1, $2, $2) RETURNING id`,
		f.orgID, deleted)
	name("a2a_security_settings", "created_by", "id", settings)
	name("a2a_security_settings", "updated_by", "id", settings)
	name("a2a_delegation", "created_by", "id", userDeleteInsert(t, db, ctx, "a2a_delegation", "id",
		`INSERT INTO a2a_delegation (organization_id, delegator_agent_id, delegate_agent_id, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		f.orgID, f.agentID, delegateAgent, deleted))
	name("security_policies", "created_by", "id", userDeleteInsert(t, db, ctx, "security_policies", "id",
		`INSERT INTO security_policies (organization_id, name, policy_type, enforcement_action, severity_threshold, created_by)
		 VALUES ($1, $2, 'mcp_unverified', 'alert_only', 'high', $3) RETURNING id`,
		f.orgID, "user-delete-policy-"+suffix, deleted))
	tag := userDeleteInsert(t, db, ctx, "tags", "id",
		`INSERT INTO tags (organization_id, key, value, category, created_by)
		 VALUES ($1, 'team', $2, 'custom', $3) RETURNING id`,
		f.orgID, "user-delete-"+suffix, deleted)
	name("tags", "created_by", "id", tag)
	_, err := db.ExecContext(ctx, `INSERT INTO agent_tags (agent_id, tag_id) VALUES ($1, $2)`, f.agentID, tag)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM agent_tags WHERE tag_id = $1`, tag) })

	var beforeDelete time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&beforeDelete))

	require.NoError(t, NewUserRepository(db).Delete(deleted),
		"deleting a user named in another row must succeed")

	assert.False(t, rowExists(t, db, ctx, "users", deleted), "the user row must be gone")
	for _, r := range userDeleteRules {
		k, ok := named[r.table+"."+r.column]
		if !assert.True(t, ok, "the fixture must name the deleted user in %s.%s", r.table, r.column) {
			continue
		}
		found, isNull := userDeleteReference(t, db, ctx, r.table, r.column, k.keyColumn, k.key)
		if r.cascade {
			assert.False(t, found, "the %s row whose %s is the deleted user must be deleted with the user", r.table, r.column)
			continue
		}
		if assert.True(t, found, "the %s row whose %s is the deleted user must be kept", r.table, r.column) {
			assert.True(t, isNull, "%s.%s must be NULL once the user it named is deleted", r.table, r.column)
		}
	}

	var tagged bool
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM agent_tags WHERE agent_id = $1 AND tag_id = $2)`, f.agentID, tag).Scan(&tagged))
	assert.True(t, tagged, "a tag the deleted user created must stay on its agent")

	assert.True(t, rowExists(t, db, ctx, "users", colleague), "another user must not be deleted")
	assert.True(t, rowExists(t, db, ctx, "api_keys", colleagueKey), "another user's API key must be kept")
	assert.True(t, rowExists(t, db, ctx, "device_codes", colleagueCode), "another user's device code must be kept")

	var updatedAt time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT updated_at FROM agents WHERE id = $1`, f.agentID).Scan(&updatedAt))
	assert.False(t, updatedAt.Before(beforeDelete),
		"setting agents.created_by to NULL updates the agent row, whose trigger sets updated_at (%s, delete began after %s)",
		updatedAt, beforeDelete)
}

// An agent whose creator was deleted keeps working. It reads back with CreatedBy
// uuid.Nil, and the handlers pass an agent's CreatedBy as the user of the rows its
// requests write: its audit rows, the MCP servers it registers, and its capability
// requests with their decision and grant. Each is stored with NULL for the user;
// the zero id is no user's and failed the foreign key. A capability request its
// creator filed before the delete is still read, with no requester, and can still
// be decided.
func TestUserDeleteAgentWhoseCreatorWasDeletedStillWritesItsRows(t *testing.T) {
	db := userDeleteTestDB(t)
	ctx := context.Background()
	xdb := sqlx.NewDb(db, "postgres")

	f := seedTenant(t, db, ctx, "user-delete-agent")
	reviewer := userDeleteSeedUser(t, db, ctx, f.orgID, nil)
	filedBefore := userDeleteInsert(t, db, ctx, "capability_requests", "id",
		`INSERT INTO capability_requests (agent_id, capability_type, reason, requested_by)
		 VALUES ($1, 'file:read', 'filed before its requester was deleted', $2) RETURNING id`,
		f.agentID, f.userID)

	require.NoError(t, NewUserRepository(db).Delete(f.userID))

	agent, err := NewAgentRepository(db).GetByID(f.agentID)
	require.NoError(t, err, "the agent must be kept")
	require.Equal(t, uuid.Nil, agent.CreatedBy, "the agent must read back with no creator")

	// Audit: the verification handler writes UserID: &agent.CreatedBy.
	audit := NewAuditLogRepository(db)
	var handed *domain.AuditLog
	audit.OnCreate(func(l *domain.AuditLog) { handed = l })
	entry := &domain.AuditLog{
		OrganizationID: f.orgID,
		UserID:         &agent.CreatedBy,
		AgentID:        &agent.ID,
		Action:         domain.AuditActionVerificationRefused,
		ResourceType:   "agent_action",
		ResourceID:     agent.ID,
		IPAddress:      "127.0.0.1",
		UserAgent:      "user-delete-test",
	}
	require.NoError(t, audit.Create(entry), "an audit row of an agent with no creator must be written")
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE id = $1`, entry.ID) })
	assert.Nil(t, entry.UserID, "the entry must say it has no user")
	require.NotNil(t, handed, "OnCreate must receive the entry")
	assert.Nil(t, handed.UserID, "the entry OnCreate receives must say it has no user")
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "audit_logs", "user_id", entry.ID), "audit_logs.user_id must be NULL")

	// MCP server: the MCP handler passes agent.CreatedBy as the creating user.
	server := &domain.MCPServer{
		ID:                uuid.New(),
		OrganizationID:    f.orgID,
		Name:              "user-delete-agent-mcp-" + f.orgID.String()[:8],
		URL:               "https://mcp-agent-" + f.orgID.String()[:8] + ".example.com",
		Status:            domain.MCPServerStatusVerified,
		IsVerified:        true,
		Capabilities:      []string{},
		RegisteredByAgent: &agent.ID,
		CreatedBy:         agent.CreatedBy,
	}
	require.NoError(t, NewMCPServerRepository(db).Create(server), "an MCP server an agent with no creator registers must be written")
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, server.ID) })
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "mcp_servers", "created_by", server.ID), "mcp_servers.created_by must be NULL")

	// Capability request, automatically approved in one transaction, as
	// CapabilityRequestService.autoApprove writes it: the requester is the reviewer and
	// the granter.
	requester := agent.CreatedBy
	inTx := &domain.CapabilityRequest{
		ID:             uuid.New(),
		AgentID:        agent.ID,
		CapabilityType: "network:outbound",
		Reason:         "requested by an agent with no creator",
		RequestedBy:    requester,
	}
	inTxGrant := &domain.AgentCapability{
		AgentID:        agent.ID,
		CapabilityType: inTx.CapabilityType,
		GrantedBy:      &requester,
		GrantedAt:      time.Now(),
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_requests WHERE id = $1`, inTx.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_capabilities WHERE id = $1`, inTxGrant.ID)
	})
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, CreateCapabilityRequestTx(ctx, tx, inTx))
	require.NoError(t, DecideCapabilityRequestTx(ctx, tx, inTx.ID, domain.CapabilityRequestStatusAutoApproved, requester))
	require.NoError(t, CreateCapabilityTx(ctx, tx, inTxGrant))
	require.NoError(t, tx.Commit(), "an approved capability request of an agent with no creator must be written")
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "capability_requests", "requested_by", inTx.ID))
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "capability_requests", "reviewed_by", inTx.ID))
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "agent_capabilities", "granted_by", inTxGrant.ID))

	// The same without a transaction, as CapabilityRequestService.CreateRequest writes
	// it when it records no transitions.
	requests := NewCapabilityRequestRepository(xdb)
	plain := &domain.CapabilityRequest{
		AgentID:        agent.ID,
		CapabilityType: "db:read",
		Reason:         "requested by an agent with no creator",
		RequestedBy:    requester,
	}
	plainGrant := &domain.AgentCapability{
		AgentID:        agent.ID,
		CapabilityType: plain.CapabilityType,
		GrantedBy:      &requester,
		GrantedAt:      time.Now(),
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM capability_requests WHERE id = $1`, plain.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_capabilities WHERE id = $1`, plainGrant.ID)
	})
	require.NoError(t, requests.Create(plain))
	require.NoError(t, requests.UpdateStatus(plain.ID, domain.CapabilityRequestStatusAutoApproved, requester))
	require.NoError(t, NewCapabilityRepository(xdb).CreateCapability(plainGrant))
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "capability_requests", "requested_by", plain.ID))
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "capability_requests", "reviewed_by", plain.ID))
	assert.True(t, userDeleteColumnIsNull(t, db, ctx, "agent_capabilities", "granted_by", plainGrant.ID))

	// The request filed before the delete is read with no requester.
	got, err := requests.GetByID(filedBefore)
	require.NoError(t, err, "a capability request whose requester was deleted must still be read")
	assert.Equal(t, uuid.Nil, got.RequestedBy)
	assert.Equal(t, "", got.RequestedByEmail)

	listed, err := requests.List(domain.CapabilityRequestFilter{OrganizationID: &f.orgID})
	require.NoError(t, err)
	ids := make(map[uuid.UUID]bool, len(listed))
	for _, r := range listed {
		ids[r.ID] = true
	}
	for name, id := range map[string]uuid.UUID{"filed before the delete": filedBefore, "in a transaction": inTx.ID, "without a transaction": plain.ID} {
		assert.True(t, ids[id], "the capability request %s must be listed", name)
	}

	require.NoError(t, requests.UpdateStatus(filedBefore, domain.CapabilityRequestStatusApproved, reviewer),
		"an admin must still be able to decide a request whose requester was deleted")
	decided, err := requests.GetByID(filedBefore)
	require.NoError(t, err)
	require.NotNil(t, decided.ReviewedBy)
	assert.Equal(t, reviewer, *decided.ReviewedBy)
}

// userDeleteSeedUser adds a member to org, approved by approvedBy when it is set, and
// returns its id. seedTenant's cleanup deletes it with the organization's users.
func userDeleteSeedUser(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID, approvedBy *uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	suffix := id.String()[:8]
	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, approved_by, approved_at, created_at, updated_at)
		 VALUES ($1, $2, $3, 'user delete colleague', 'x', 'member', 'local', $4, $5,
		         CASE WHEN $5::uuid IS NULL THEN NULL ELSE NOW() END, NOW(), NOW())`,
		id, orgID, "user-delete-"+suffix+"@example.com", "local-"+suffix, approvedBy)
	require.NoError(t, err)
	return id
}

// userDeleteInsertAPIKey adds an API key for f's agent, created by createdBy.
func userDeleteInsertAPIKey(t *testing.T, db *sql.DB, ctx context.Context, f tenantFixture, createdBy uuid.UUID) uuid.UUID {
	t.Helper()

	marker := uuid.New().String()
	return userDeleteInsert(t, db, ctx, "api_keys", "id",
		`INSERT INTO api_keys (organization_id, agent_id, name, key_hash, prefix, created_by)
		 VALUES ($1, $2, $3, $4, 'udel', $5) RETURNING id`,
		f.orgID, f.agentID, "user-delete-key-"+marker[:8], "not-a-real-hash-"+marker, createdBy)
}

// userDeleteInsertDeviceCode adds a CLI sign-in code that userID approved.
func userDeleteInsertDeviceCode(t *testing.T, db *sql.DB, ctx context.Context, orgID, userID uuid.UUID) uuid.UUID {
	t.Helper()

	marker := uuid.New().String()
	return userDeleteInsert(t, db, ctx, "device_codes", "id",
		`INSERT INTO device_codes (device_code, user_code, verification_uri, expires_at, status,
		                           user_id, organization_id, approved_at)
		 VALUES ($1, $2, 'https://aim.example.com/device', NOW() + INTERVAL '10 minutes', 'approved',
		         $3, $4, NOW())
		 RETURNING id`,
		"not-a-real-device-code-"+marker, "UD-"+marker[:8], userID, orgID)
}

// userDeleteInsert runs query, an INSERT that returns the new row's key, and deletes
// the row from table when the test ends if it is still there. table and keyColumn are
// always literals from this file.
func userDeleteInsert(t *testing.T, db *sql.DB, ctx context.Context, table, keyColumn, query string, args ...interface{}) uuid.UUID {
	t.Helper()

	var key uuid.UUID
	require.NoError(t, db.QueryRowContext(ctx, query, args...).Scan(&key), "insert into %s", table)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+keyColumn+` = $1`, key)
	})
	return key
}

// userDeleteReference reports whether table holds the row whose keyColumn is key, and
// whether its column is NULL. The names are always literals from this file.
func userDeleteReference(t *testing.T, db *sql.DB, ctx context.Context, table, column, keyColumn string, key uuid.UUID) (found, isNull bool) {
	t.Helper()

	err := db.QueryRowContext(ctx,
		`SELECT `+column+` IS NULL FROM `+table+` WHERE `+keyColumn+` = $1`, key).Scan(&isNull)
	if err == sql.ErrNoRows {
		return false, false
	}
	require.NoError(t, err)
	return true, isNull
}

// userDeleteColumnIsNull reports whether the row of table with the given id holds NULL
// in column; the row must exist.
func userDeleteColumnIsNull(t *testing.T, db *sql.DB, ctx context.Context, table, column string, id uuid.UUID) bool {
	t.Helper()

	found, isNull := userDeleteReference(t, db, ctx, table, column, "id", id)
	require.True(t, found, "%s must hold the row %s", table, id)
	return isNull
}

func userDeleteRuleName(code string) string {
	switch code {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	case "":
		return "no foreign key"
	}
	return code
}

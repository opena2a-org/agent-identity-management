//go:build integration

package application

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RecordConsent must refuse to write a consent record whose grantor the caller
// does not own.
//
// This is the write-path half of the A2A consent tenant-scoping fix, and it is
// the load-bearing half. The handler stamps organization_id from the caller's
// authenticated org, but both agent IDs arrive in the request body and the only
// constraint on them is a foreign key to agents(id) — which requires a real
// agent, not one of the caller's. Without this check organization_id and the
// grantor's org diverge on every such write, and the read-side predicate then
// scopes faithfully to a value the writer chose.
//
// A real database is required: the check reads the agent row to compare
// organization_id, and A2AService holds a concrete *repository.AgentRepository,
// so there is nothing to substitute.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestConsentGrantorOwnership ./internal/application/...

func consentOwnershipDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping consent grantor ownership test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

type consentOwnershipTenant struct {
	orgID   uuid.UUID
	agentID uuid.UUID
}

func seedConsentOwnershipTenant(t *testing.T, db *sql.DB, ctx context.Context, label string) consentOwnershipTenant {
	t.Helper()

	f := consentOwnershipTenant{orgID: uuid.New(), agentID: uuid.New()}
	userID := uuid.New()
	suffix := f.orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_consent_records WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, f.orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
		f.orgID, label+"-org-"+suffix, label+"-"+suffix+".invalid")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id)
		 VALUES ($1, $2, $3, $4, 'local', $5)`,
		userID, f.orgID, label+"-"+suffix+"@example.invalid", label+" user", userID.String())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id)
		 VALUES ($1, $2, $2, 'service', $3, $4)`,
		f.agentID, label+"-agent-"+suffix, userID, f.orgID)
	require.NoError(t, err)

	return f
}

// serviceForConsentOwnership builds an A2AService with only the two
// repositories RecordConsent touches. Same package, so the struct literal is
// available and the other fields stay nil deliberately — if a future change
// makes RecordConsent reach for one of them, this test panics loudly rather
// than silently exercising a different path.
func serviceForConsentOwnership(db *sql.DB) *A2AService {
	return &A2AService{
		consentRepo: repository.NewA2AConsentRepository(db),
		agentRepo:   repository.NewAgentRepository(db),
	}
}

func recordConsentReq(orgID, grantor, recipient uuid.UUID) RecordConsentRequest {
	return RecordConsentRequest{
		UserID:           "consent-ownership-user",
		OrganizationID:   &orgID,
		GrantorAgentID:   grantor,
		RecipientAgentID: recipient,
		Scope:            []string{"pii"},
		Purpose:          "regression test",
		DataTypes:        []string{"email"},
		ConsentMethod:    "api",
	}
}

// The attack: name another organization's agent as grantor while the record is
// stamped with the caller's own org.
func TestConsentGrantorOwnership_RejectsForeignGrantor(t *testing.T) {
	ctx := context.Background()
	db := consentOwnershipDB(t)
	svc := serviceForConsentOwnership(db)

	mine := seedConsentOwnershipTenant(t, db, ctx, "mine")
	theirs := seedConsentOwnershipTenant(t, db, ctx, "theirs")

	consent, err := svc.RecordConsent(ctx, recordConsentReq(mine.orgID, theirs.agentID, mine.agentID))

	require.Error(t, err, "a consent record naming another organization's agent as grantor was accepted")
	assert.True(t, errors.Is(err, ErrConsentGrantorNotOwned),
		"expected ErrConsentGrantorNotOwned, got %v", err)
	assert.Nil(t, consent)

	// The row must not exist. An error return that still wrote would leave the
	// divergence this check exists to prevent.
	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM a2a_consent_records WHERE grantor_agent_id = $1`,
		theirs.agentID).Scan(&count))
	assert.Equal(t, 0, count, "the rejected consent was written anyway")
}

// A grantor id that matches no agent at all must be refused the same way, and
// with the SAME error — distinguishing the two would answer "does this agent id
// exist" for agents the caller cannot see.
func TestConsentGrantorOwnership_RejectsUnknownGrantorWithTheSameError(t *testing.T) {
	ctx := context.Background()
	db := consentOwnershipDB(t)
	svc := serviceForConsentOwnership(db)

	mine := seedConsentOwnershipTenant(t, db, ctx, "mine")

	_, foreignErr := svc.RecordConsent(ctx,
		recordConsentReq(mine.orgID, seedConsentOwnershipTenant(t, db, ctx, "theirs").agentID, mine.agentID))
	_, unknownErr := svc.RecordConsent(ctx,
		recordConsentReq(mine.orgID, uuid.New(), mine.agentID))

	require.Error(t, unknownErr)
	assert.True(t, errors.Is(unknownErr, ErrConsentGrantorNotOwned))
	assert.Equal(t, foreignErr.Error(), unknownErr.Error(),
		"a nonexistent grantor and a foreign one must be indistinguishable, or the error is an enumeration oracle")
}

// The control, and the over-scoping guard. Owning the grantor is enough; the
// RECIPIENT may belong to another organization, because that is what an A2A
// consent grant is for. A fix that also constrained the recipient would break
// the feature while looking secure, so this test fails if someone adds that.
func TestConsentGrantorOwnership_AllowsCrossOrgRecipient(t *testing.T) {
	ctx := context.Background()
	db := consentOwnershipDB(t)
	svc := serviceForConsentOwnership(db)

	mine := seedConsentOwnershipTenant(t, db, ctx, "mine")
	theirs := seedConsentOwnershipTenant(t, db, ctx, "theirs")

	consent, err := svc.RecordConsent(ctx, recordConsentReq(mine.orgID, mine.agentID, theirs.agentID))

	require.NoError(t, err,
		"consent naming another organization's agent as RECIPIENT was refused; the scoping is too tight and cross-org consent is broken")
	require.NotNil(t, consent)
	require.NotNil(t, consent.OrganizationID)
	assert.Equal(t, mine.orgID, *consent.OrganizationID)
	assert.Equal(t, theirs.agentID, consent.RecipientAgentID)
}

// A record with no organization at all must be refused rather than written
// unowned. Before migration 097 this is exactly how NULL-org rows appeared.
func TestConsentGrantorOwnership_FailsClosedWithoutAnOrg(t *testing.T) {
	ctx := context.Background()
	db := consentOwnershipDB(t)
	svc := serviceForConsentOwnership(db)

	mine := seedConsentOwnershipTenant(t, db, ctx, "mine")

	req := recordConsentReq(mine.orgID, mine.agentID, mine.agentID)
	req.OrganizationID = nil

	_, err := svc.RecordConsent(ctx, req)
	require.Error(t, err, "a consent record with no organization must be refused, not written unowned")
	assert.True(t, errors.Is(err, ErrConsentGrantorNotOwned))
}

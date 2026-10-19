//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// Deleting a user removes the user's email from the admin access reviews the
// organization keeps as compliance evidence. CollectEvidence for admin_access_review
// stores an entry for each admin of the organization, with the admin's email, and
// the evidence stays when one of them is deleted. Migration 138 removes the email from
// the deleted user's entry in the statement that deletes the user.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserDeleteTrigger ./internal/application/...

func accessReviewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping access review user delete test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// With every migration applied: two access reviews collected through CollectEvidence,
// the first by the admin who is then deleted through UserRepository.Delete. Read back
// through GetEvidenceForCheck, both reviews keep both entries; the deleted admin's
// entry keeps its id, last sign-in and status and has no email, and the other admin's
// entry is kept as collected.
func TestUserDeleteTriggerRemovesTheAdminsEmailFromAccessReviewEvidence(t *testing.T) {
	db := accessReviewTestDB(t)
	ctx := context.Background()

	orgID, stayingID := seedOrgAndUser(t, db, ctx, "access-review-delete")
	leavingID, leavingEmail := seedAccessReviewAdmin(t, db, ctx, orgID, "leaving")

	users := repository.NewUserRepository(db)
	svc := NewComplianceServiceFull(nil, nil, users, nil, repository.NewComplianceEvidenceRepository(db), nil, nil)

	collectedByLeaving := collectAccessReview(t, db, ctx, svc, orgID, leavingID)
	collectedByStaying := collectAccessReview(t, db, ctx, svc, orgID, stayingID)

	before := accessReviewsByID(t, svc, ctx, orgID)
	require.Len(t, before, 2, "control: both access reviews must be stored")
	for _, id := range []uuid.UUID{collectedByLeaving, collectedByStaying} {
		require.Equal(t, leavingEmail, before[id].entries[leavingID.String()]["email"],
			"control: the review must hold the leaving admin's email before the delete")
	}

	require.NoError(t, users.Delete(leavingID), "deleting the admin must succeed")

	after := accessReviewsByID(t, svc, ctx, orgID)
	require.Len(t, after, 2, "the organization's access reviews must be kept")
	for _, id := range []uuid.UUID{collectedByLeaving, collectedByStaying} {
		review := after[id]
		assert.Equal(t, before[id].adminCount, review.adminCount, "the review must keep its admin count")
		assert.Len(t, review.entries, 2, "the review must keep an entry for each admin it was collected with")

		want := map[string]interface{}{}
		for k, v := range before[id].entries[leavingID.String()] {
			if k != "email" {
				want[k] = v
			}
		}
		assert.Equal(t, want, review.entries[leavingID.String()],
			"the deleted admin's entry must keep its id, last sign-in and status and lose the email")
		assert.Equal(t, before[id].entries[stayingID.String()], review.entries[stayingID.String()],
			"the other admin's entry must be kept as collected")
	}
	assert.Equal(t, uuid.Nil, after[collectedByLeaving].collectedBy,
		"the review the deleted admin collected must name no collector")
	assert.Equal(t, stayingID, after[collectedByStaying].collectedBy,
		"the review the other admin collected must keep its collector")
}

// Deleting an organization deletes its users and its evidence in one statement. With
// two access reviews that list its two admins, the delete succeeds, and no user or
// evidence of the organization is left.
func TestUserDeleteTriggerLetsAnOrganizationWithAccessReviewsBeDeleted(t *testing.T) {
	db := accessReviewTestDB(t)
	ctx := context.Background()

	orgID, adminID := seedOrgAndUser(t, db, ctx, "access-review-org-delete")
	otherAdminID, _ := seedAccessReviewAdmin(t, db, ctx, orgID, "other")
	svc := NewComplianceServiceFull(nil, nil, repository.NewUserRepository(db), nil,
		repository.NewComplianceEvidenceRepository(db), nil, nil)
	collectAccessReview(t, db, ctx, svc, orgID, adminID)
	collectAccessReview(t, db, ctx, svc, orgID, otherAdminID)

	_, err := db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	require.NoError(t, err, "deleting the organization must succeed")

	var users, evidence int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE organization_id = $1`, orgID).Scan(&users))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM compliance_evidence WHERE organization_id = $1`, orgID).Scan(&evidence))
	assert.Equal(t, 0, users, "the organization's users must be deleted with it")
	assert.Equal(t, 0, evidence, "the organization's evidence must be deleted with it")
}

// seedAccessReviewAdmin adds a user with the admin role to org and returns its id and
// email; the user is deleted when the test ends if it is still there.
func seedAccessReviewAdmin(t *testing.T, db *sql.DB, ctx context.Context, orgID uuid.UUID, label string) (uuid.UUID, string) {
	t.Helper()

	id := uuid.New()
	email := "access-review-" + label + "-" + id.String()[:8] + "@example.com"
	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $3, NOW(), NOW())`,
		id, orgID, email, "Access Review "+label)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id) })
	return id, email
}

// collectAccessReview collects admin_access_review evidence through CollectEvidence, as
// POST /api/v1/compliance/evidence/collect does, and returns its id. The evidence is
// deleted when the test ends if it is still there.
func collectAccessReview(t *testing.T, db *sql.DB, ctx context.Context, svc *ComplianceService, orgID, collectedBy uuid.UUID) uuid.UUID {
	t.Helper()

	evidence, err := svc.CollectEvidence(ctx, orgID, domain.FrameworkAIM, "admin_access_review", collectedBy)
	require.NoError(t, err)
	require.Equal(t, domain.EvidenceTypeAccessReview, evidence.EvidenceType,
		"control: CollectEvidence must have collected an access review")
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM compliance_evidence WHERE id = $1`, evidence.ID) })
	return evidence.ID
}

type storedAccessReview struct {
	collectedBy uuid.UUID
	adminCount  interface{}
	entries     map[string]map[string]interface{}
}

// accessReviewsByID reads the organization's admin_access_review evidence through
// GetEvidenceForCheck, with each review's adminUsers entries keyed by their id.
func accessReviewsByID(t *testing.T, svc *ComplianceService, ctx context.Context, orgID uuid.UUID) map[uuid.UUID]storedAccessReview {
	t.Helper()

	stored, err := svc.GetEvidenceForCheck(ctx, orgID, "admin_access_review")
	require.NoError(t, err)
	reviews := make(map[uuid.UUID]storedAccessReview, len(stored))
	for _, ev := range stored {
		list, ok := ev.Data["adminUsers"].([]interface{})
		require.True(t, ok, "the review %s must hold an adminUsers list, got %T", ev.ID, ev.Data["adminUsers"])
		review := storedAccessReview{
			collectedBy: ev.CollectedBy,
			adminCount:  ev.Data["adminCount"],
			entries:     make(map[string]map[string]interface{}, len(list)),
		}
		for _, item := range list {
			entry, ok := item.(map[string]interface{})
			require.True(t, ok, "an adminUsers entry of the review %s must be an object, got %T", ev.ID, item)
			id, _ := entry["id"].(string)
			review.entries[id] = entry
		}
		reviews[ev.ID] = review
	}
	return reviews
}

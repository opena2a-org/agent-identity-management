//go:build integration

package repository

import (
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A deleted webhook must not hold on to its name.
//
// WebhookRepository.Delete is a soft delete: it sets `deleted_at` and every read path
// filters on `deleted_at IS NULL`, so the row is gone as far as the organization can see.
// The per-organization name uniqueness, however, was a plain UNIQUE (organization_id, name)
// constraint that still counted those rows. Deleting a webhook therefore reserved its name
// forever: creating a new webhook with that name, or renaming another one to it, failed with
// a duplicate-key error the user had no way to resolve.
//
// Uniqueness now applies to live rows only. These tests pin both halves of that: a deleted
// webhook's name is free again, and two live webhooks still cannot share one.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestWebhookName ./internal/infrastructure/repository/...

func webhookNameTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping webhook name reuse test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedWebhookNameOrg inserts one organization and one admin user and returns their ids.
// Every webhook the test creates in that organization is removed on cleanup.
func seedWebhookNameOrg(t *testing.T, db *sql.DB) (orgID, userID uuid.UUID) {
	t.Helper()

	orgID, userID = uuid.New(), uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM webhooks WHERE organization_id = $1`, orgID)
		_, _ = db.Exec(`DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.Exec(
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "webhookname-org-"+suffix, "webhookname-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.Exec(
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "webhookname-"+suffix+"@example.com", "webhookname-user", "local-"+suffix)
	require.NoError(t, err)

	return orgID, userID
}

func newNamedWebhook(orgID, userID uuid.UUID, name string) *domain.Webhook {
	return &domain.Webhook{
		ID:                uuid.New(),
		OrganizationID:    orgID,
		Name:              name,
		URL:               "https://hooks.example.com/" + name,
		Events:            []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		Secret:            "test-secret",
		IsActive:          true,
		TimeoutSeconds:    30,
		MaxRetries:        3,
		RetryDelaySeconds: 60,
		CreatedBy:         userID,
	}
}

func TestWebhookNameIsFreeAfterDelete(t *testing.T) {
	db := webhookNameTestDB(t)
	repo := NewWebhookRepository(db)
	orgID, userID := seedWebhookNameOrg(t, db)

	first := newNamedWebhook(orgID, userID, "alerts")
	require.NoError(t, repo.Create(first))
	require.NoError(t, repo.Delete(first.ID))

	second := newNamedWebhook(orgID, userID, "alerts")
	require.NoError(t, repo.Create(second),
		"creating a webhook with the name of a deleted one must succeed")

	got, err := repo.GetByID(second.ID)
	require.NoError(t, err)
	assert.Equal(t, "alerts", got.Name)

	// The deleted row is kept, not overwritten: the name is shared by one live row
	// and one soft-deleted row.
	var total, live int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE deleted_at IS NULL)
		   FROM webhooks WHERE organization_id = $1 AND name = $2`,
		orgID, "alerts").Scan(&total, &live))
	assert.Equal(t, 2, total)
	assert.Equal(t, 1, live)

	// The name can be freed and reused more than once.
	require.NoError(t, repo.Delete(second.ID))
	require.NoError(t, repo.Create(newNamedWebhook(orgID, userID, "alerts")),
		"a name freed twice must be reusable twice")
}

func TestWebhookRenameToDeletedName(t *testing.T) {
	db := webhookNameTestDB(t)
	repo := NewWebhookRepository(db)
	orgID, userID := seedWebhookNameOrg(t, db)

	deleted := newNamedWebhook(orgID, userID, "alerts")
	require.NoError(t, repo.Create(deleted))
	require.NoError(t, repo.Delete(deleted.ID))

	other := newNamedWebhook(orgID, userID, "audit")
	require.NoError(t, repo.Create(other))

	other.Name = "alerts"
	require.NoError(t, repo.Update(other),
		"renaming a webhook to the name of a deleted one must succeed")

	got, err := repo.GetByID(other.ID)
	require.NoError(t, err)
	assert.Equal(t, "alerts", got.Name)
}

func TestWebhookNameStillUniqueAmongLiveWebhooks(t *testing.T) {
	db := webhookNameTestDB(t)
	repo := NewWebhookRepository(db)
	orgID, userID := seedWebhookNameOrg(t, db)

	require.NoError(t, repo.Create(newNamedWebhook(orgID, userID, "alerts")))

	err := repo.Create(newNamedWebhook(orgID, userID, "alerts"))
	require.Error(t, err, "two live webhooks in one organization must not share a name")
	var pqErr *pq.Error
	require.ErrorAs(t, err, &pqErr)
	assert.Equal(t, pq.ErrorCode("23505"), pqErr.Code, "expected a unique violation, got %v", err)

	other := newNamedWebhook(orgID, userID, "audit")
	require.NoError(t, repo.Create(other))
	other.Name = "alerts"
	err = repo.Update(other)
	require.Error(t, err, "renaming onto a live webhook's name must fail")
	require.ErrorAs(t, err, &pqErr)
	assert.Equal(t, pq.ErrorCode("23505"), pqErr.Code, "expected a unique violation, got %v", err)

	// A different organization may use the same name.
	otherOrgID, otherUserID := seedWebhookNameOrg(t, db)
	require.NoError(t, repo.Create(newNamedWebhook(otherOrgID, otherUserID, "alerts")))
}

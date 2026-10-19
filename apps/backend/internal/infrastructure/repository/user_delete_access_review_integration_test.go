//go:build integration

package repository

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 138 removes, once, the email from the admin access review entries of the
// users deleted before it ran. The test that deletes an admin with every migration
// applied is in internal/application, where the review is collected through
// ComplianceService.CollectEvidence.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestUserDeleteBackfill ./internal/infrastructure/repository/...

const userDeleteAccessReviewMigration = "138_user_delete_removes_email_from_access_review_evidence.sql"

// The schema stands in for a database with every migration through 137 applied, with
// the columns migration 138 reads and writes. The migration runs from the file the
// repository ships, twice. An entry whose id names no user loses its email, in a
// review of the remaining user's organization and in one of another; an entry of a
// remaining user, the evidence of another check and the values that are not entries
// are kept as written, and data that holds no list under adminUsers is kept. Then a
// delete of the remaining user removes its email through the trigger the migration
// added, in the reviews of its organization.
func TestUserDeleteBackfillRemovesAccessReviewEmailsThatOutlivedTheirUser(t *testing.T) {
	db := userDeleteStandInSchema(t)

	for _, ddl := range []string{
		`CREATE TABLE users (id UUID PRIMARY KEY, organization_id UUID NOT NULL, email VARCHAR(255) NOT NULL)`,
		`CREATE TABLE compliance_evidence (id UUID PRIMARY KEY, organization_id UUID NOT NULL, check_name VARCHAR(255) NOT NULL, data JSONB DEFAULT '{}')`,
	} {
		_, err := db.Exec(ddl)
		require.NoError(t, err)
	}

	org, otherOrg := uuid.New(), uuid.New()
	staying := uuid.New().String()
	gone := uuid.New().String() // an id no user has: the user it named was deleted
	_, err := db.Exec(`INSERT INTO users VALUES ($1, $2, 'staying@example.com')`, staying, org)
	require.NoError(t, err)

	goneEntry := `{"id": "` + gone + `", "email": "deleted@example.com", "lastLogin": "never", "status": "active"}`
	goneCleared := `{"id": "` + gone + `", "lastLogin": "never", "status": "active"}`
	stayingEntry := `{"id": "` + staying + `", "email": "staying@example.com", "lastLogin": "never", "status": "active"}`
	stayingCleared := `{"id": "` + staying + `", "lastLogin": "never", "status": "active"}`
	notEntries := `"email", 7, null, ["email"], {"email": "no-id@example.com"}, {"id": null, "email": "null-id@example.com"}`

	rows := []struct {
		name, org, checkName string
		data                 *string // nil stores NULL
		want                 *string // the data after the migration; nil: NULL
		wantAfterDelete      *string // the data after the remaining user is deleted; nil: as want
	}{
		{name: "review of a deleted and a remaining admin", org: "org", checkName: "admin_access_review",
			data:            userDeleteJSON(`{"adminCount": 2, "adminUsers": [` + goneEntry + `, ` + stayingEntry + `], "reviewDate": "2026-10-01T00:00:00Z"}`),
			want:            userDeleteJSON(`{"adminCount": 2, "adminUsers": [` + goneCleared + `, ` + stayingEntry + `], "reviewDate": "2026-10-01T00:00:00Z"}`),
			wantAfterDelete: userDeleteJSON(`{"adminCount": 2, "adminUsers": [` + goneCleared + `, ` + stayingCleared + `], "reviewDate": "2026-10-01T00:00:00Z"}`)},
		{name: "review of a remaining admin", org: "org", checkName: "admin_access_review",
			data:            userDeleteJSON(`{"adminCount": 1, "adminUsers": [` + stayingEntry + `]}`),
			want:            userDeleteJSON(`{"adminCount": 1, "adminUsers": [` + stayingEntry + `]}`),
			wantAfterDelete: userDeleteJSON(`{"adminCount": 1, "adminUsers": [` + stayingCleared + `]}`)},
		{name: "review of another organization of a deleted admin", org: "otherOrg", checkName: "admin_access_review",
			data: userDeleteJSON(`{"adminCount": 1, "adminUsers": [` + goneEntry + `]}`),
			want: userDeleteJSON(`{"adminCount": 1, "adminUsers": [` + goneCleared + `]}`)},
		{name: "review whose adminUsers holds values that are not entries", org: "org", checkName: "admin_access_review",
			data: userDeleteJSON(`{"adminUsers": [` + notEntries + `, {"id": "not-a-user-id", "email": "unknown@example.com"}, ` + goneEntry + `]}`),
			want: userDeleteJSON(`{"adminUsers": [` + notEntries + `, {"id": "not-a-user-id"}, ` + goneCleared + `]}`)},
		{name: "evidence of another check that holds the same entries", org: "org", checkName: "trust_score_degradation",
			data: userDeleteJSON(`{"adminUsers": [` + goneEntry + `, ` + stayingEntry + `]}`),
			want: userDeleteJSON(`{"adminUsers": [` + goneEntry + `, ` + stayingEntry + `]}`)},
		{name: "review whose adminUsers is an object", org: "org", checkName: "admin_access_review",
			data: userDeleteJSON(`{"adminUsers": ` + goneEntry + `}`),
			want: userDeleteJSON(`{"adminUsers": ` + goneEntry + `}`)},
		{name: "review whose data is a list", org: "org", checkName: "admin_access_review",
			data: userDeleteJSON(`[` + goneEntry + `]`),
			want: userDeleteJSON(`[` + goneEntry + `]`)},
		{name: "review with no data", org: "org", checkName: "admin_access_review"},
	}
	ids := make(map[string]uuid.UUID, len(rows))
	for _, r := range rows {
		ids[r.name] = uuid.New()
		rowOrg := org
		if r.org == "otherOrg" {
			rowOrg = otherOrg
		}
		_, err := db.Exec(`INSERT INTO compliance_evidence VALUES ($1, $2, $3, $4::jsonb)`, ids[r.name], rowOrg, r.checkName, r.data)
		require.NoError(t, err, "%s", r.name)
	}

	// Control: the review holds the deleted admin's email before the migration runs.
	userDeleteAssertEvidenceData(t, db, ids["review of a deleted and a remaining admin"], rows[0].data,
		"control: the review must hold the deleted admin's email before the migration")

	migration := userDeleteReadMigration(t, userDeleteAccessReviewMigration)
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply")
	_, err = db.Exec(migration)
	require.NoError(t, err, "the migration must apply again")

	for _, r := range rows {
		userDeleteAssertEvidenceData(t, db, ids[r.name], r.want, r.name)
	}

	_, err = db.Exec(`DELETE FROM users WHERE id = $1`, staying)
	require.NoError(t, err, "deleting the remaining admin must succeed")
	for _, r := range rows {
		want := r.want
		if r.wantAfterDelete != nil {
			want = r.wantAfterDelete
		}
		userDeleteAssertEvidenceData(t, db, ids[r.name], want, "once the remaining admin is deleted: "+r.name)
	}
}

// userDeleteJSON returns a pointer to s, for a row's stored data.
func userDeleteJSON(s string) *string { return &s }

// userDeleteAssertEvidenceData asserts that the stand-in schema's evidence row holds
// want as its data, compared as JSON; a nil want is NULL.
func userDeleteAssertEvidenceData(t *testing.T, db *sql.DB, id uuid.UUID, want *string, msg string) {
	t.Helper()

	var raw sql.NullString
	require.NoError(t, db.QueryRow(`SELECT data FROM compliance_evidence WHERE id = $1`, id).Scan(&raw), "%s", msg)
	if want == nil {
		assert.False(t, raw.Valid, "%s: data must be NULL, got %s", msg, raw.String)
		return
	}
	require.True(t, raw.Valid, "%s: data must not be NULL", msg)
	var got, wantValue interface{}
	require.NoError(t, json.Unmarshal([]byte(raw.String), &got), "%s", msg)
	require.NoError(t, json.Unmarshal([]byte(*want), &wantValue), "%s", msg)
	assert.Equal(t, wantValue, got, "%s", msg)
}

package main

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/seed"
)

// seedAdminFromEnv seeds from the ADMIN_* variables as the server reads them
// at startup.
func seedAdminFromEnv(db *sql.DB) error {
	return seedAdmin(db, config.LoadAdminSeed())
}

const seedTestPassword = "Seeded-Admin-Pass-42!"

// bcryptOf matches a password_hash argument that verifies against the
// password, so the test pins what is stored without fixing the salt.
type bcryptOf string

func (b bcryptOf) Match(v driver.Value) bool {
	hash, ok := v.(string)
	return ok && bcrypt.CompareHashAndPassword([]byte(hash), []byte(b)) == nil
}

func newSeedMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("database calls: %v", err)
		}
	})
	return db, mock
}

func expectAdminExists(mock sqlmock.Sqlmock, exists bool) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM users WHERE role = 'admin' OR LOWER(email) = LOWER($1))`)).
		WithArgs("admin@opena2a.org").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

// sameAdminID matches the first value it is compared with and, after that,
// only an equal one: the id the admin INSERT carries, then the created_by of
// each default policy.
type sameAdminID struct{ seen *driver.Value }

func newSameAdminID() sameAdminID { return sameAdminID{seen: new(driver.Value)} }

func (s sameAdminID) Match(v driver.Value) bool {
	if *s.seen == nil {
		*s.seen = v
		return true
	}
	return *s.seen == v
}

// expectDefaultPoliciesCommitted queues what follows the admin INSERT on the
// start that creates the administrator: an INSERT of each
// seed.DefaultPolicies row into the organization orgID, created by the
// administrator admin matches, then the commit. The last present of those
// INSERTs answer that the organization already had the policy.
func expectDefaultPoliciesCommitted(mock sqlmock.Sqlmock, orgID string, admin sqlmock.Argument, present int) {
	for i, p := range seed.DefaultPolicies {
		var added int64 = 1
		if i >= len(seed.DefaultPolicies)-present {
			added = 0
		}
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_policies`)).
			WithArgs(orgID, p.Name, p.Description, p.PolicyType, p.EnforcementAction,
				p.SeverityThreshold, p.Rules, p.IsEnabled, p.Priority, admin).
			WillReturnResult(sqlmock.NewResult(0, added))
	}
	mock.ExpectCommit()
}

// seedLogLines returns the lines the standard logger writes while fn runs.
func seedLogLines(fn func()) []string {
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetFlags(flags)
		log.SetOutput(out)
	}()
	fn()
	if buf.Len() == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// A restart with ADMIN_PASSWORD still set must not touch an administrator
// that already exists: the password the operator changed in the dashboard is
// the one that signs in after the restart.
func TestSeedAdminFromEnv_ExistingAdminKeepsItsPassword(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, true)

	if err := seedAdminFromEnv(db); err != nil {
		t.Fatalf("seedAdminFromEnv with an existing admin: %v", err)
	}
	// No UPDATE or INSERT is expected; sqlmock fails any statement it was not told about.
}

// With no administrator yet, ADMIN_PASSWORD seeds one in the default admin
// organization, with the same flags aim-bootstrap --default sets. In the
// transaction that inserts it, the seed adds each default security policy to
// that organization, created by that administrator, as aim-bootstrap
// --default does.
func TestSeedAdminFromEnv_NoAdminSeedsOne(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_NAME", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs("admin.opena2a.org").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	admin := newSameAdminID()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(
			admin, // id
			"11111111-1111-1111-1111-111111111111",
			"admin@opena2a.org",
			"System Administrator",
			sqlmock.AnyArg(), // provider_id
			bcryptOf(seedTestPassword),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectDefaultPoliciesCommitted(mock, "11111111-1111-1111-1111-111111111111", admin, 0)

	if err := seedAdminFromEnv(db); err != nil {
		t.Fatalf("seedAdminFromEnv with no admin: %v", err)
	}
}

// The start that seeds the administrator logs how many default policies it
// added, after the line that names the administrator. A policy the
// organization already has under the same name is left as it is and counted
// apart.
func TestSeedAdminFromEnv_LogsTheDefaultPoliciesItAdded(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	all := len(seed.DefaultPolicies)
	for _, tc := range []struct {
		name    string
		present int
		want    string
	}{
		{"organization without policies", 0,
			fmt.Sprintf("Added %d default security policies to the admin.opena2a.org organization", all)},
		{"organization with two of them", 2,
			fmt.Sprintf("Added %d default security policies to the admin.opena2a.org organization; 2 it already had are left unchanged", all-2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSeedMock(t)
			expectAdminExists(mock, false)
			expectAdminOrganization(mock)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnResult(sqlmock.NewResult(0, 1))
			expectDefaultPoliciesCommitted(mock, "11111111-1111-1111-1111-111111111111", sqlmock.AnyArg(), tc.present)

			var err error
			lines := seedLogLines(func() { err = seedAdminFromEnv(db) })
			if err != nil {
				t.Fatalf("seedAdminFromEnv: %v", err)
			}
			want := []string{
				"Seeded administrator admin@opena2a.org from ADMIN_PASSWORD (change the password at first sign-in)",
				tc.want,
			}
			if !slices.Equal(lines, want) {
				t.Errorf("seedAdminFromEnv logged:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// A policy the database refuses fails the seed and rolls its transaction
// back: the administrator it inserted is not committed, so the next start
// seeds it again, and no line says it was seeded.
func TestSeedAdminFromEnv_RefusedDefaultPolicyRollsTheAdministratorBack(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	expectAdminOrganization(mock)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_policies`)).
		WillReturnError(errors.New("refused by the test"))
	mock.ExpectRollback()

	var err error
	lines := seedLogLines(func() { err = seedAdminFromEnv(db) })
	if err == nil || !strings.Contains(err.Error(), "failed to add the default security policies to the admin.opena2a.org organization") {
		t.Fatalf("seedAdminFromEnv returned %v, want the refused policy's error", err)
	}
	if len(lines) > 0 {
		t.Errorf("seedAdminFromEnv logged %q for a seed it rolled back", lines)
	}
}

// Only the start that inserts the administrator adds policies. When the
// INSERT adds no row, because an account for that email reached the
// organization after the existence check, the start adds none and logs
// nothing.
func TestSeedAdminFromEnv_NoAdministratorInsertedAddsNoPolicy(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	expectAdminOrganization(mock)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	var err error
	lines := seedLogLines(func() { err = seedAdminFromEnv(db) })
	if err != nil {
		t.Fatalf("seedAdminFromEnv: %v", err)
	}
	if len(lines) > 0 {
		t.Errorf("seedAdminFromEnv logged %q when it inserted no administrator", lines)
	}
}

func TestSeedAdminFromEnv_UnsetDoesNothing(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "")
	db, _ := newSeedMock(t)

	if err := seedAdminFromEnv(db); err != nil {
		t.Fatalf("seedAdminFromEnv with ADMIN_PASSWORD unset: %v", err)
	}
}

// A seed password is held to the rule every other AIM password meets; a weak
// one creates no account.
func TestSeedAdminFromEnv_WeakPasswordSeedsNothing(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("ADMIN_EMAIL", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)

	if err := seedAdminFromEnv(db); err == nil {
		t.Fatal("seedAdminFromEnv accepted a password without upper case, digit or special character")
	}
}

func TestSeedAdminFromEnv_MissingAdminOrgIsAnError(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs("admin.opena2a.org").
		WillReturnError(errors.New("sql: no rows in result set"))

	if err := seedAdminFromEnv(db); err == nil {
		t.Fatal("seedAdminFromEnv returned no error when the admin organization is missing")
	}
}

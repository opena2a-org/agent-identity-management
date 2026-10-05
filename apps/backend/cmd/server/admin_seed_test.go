package main

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

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
// organization, with the same flags aim-bootstrap --default sets.
func TestSeedAdminFromEnv_NoAdminSeedsOne(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", seedTestPassword)
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_NAME", "")
	db, mock := newSeedMock(t)
	expectAdminExists(mock, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs("admin.opena2a.org").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(
			sqlmock.AnyArg(), // id
			"11111111-1111-1111-1111-111111111111",
			"admin@opena2a.org",
			"System Administrator",
			sqlmock.AnyArg(), // provider_id
			bcryptOf(seedTestPassword),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := seedAdminFromEnv(db); err != nil {
		t.Fatalf("seedAdminFromEnv with no admin: %v", err)
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

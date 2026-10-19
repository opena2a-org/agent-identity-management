package main

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/seed"
)

var uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// withoutIDs replaces every UUID in s with <id>, since runBootstrap generates
// the IDs it prints.
func withoutIDs(s string) string {
	return uuidPattern.ReplaceAllString(s, "<id>")
}

// expectBootstrapTransaction queues the statements runBootstrap issues, in
// order. orgID is the ID the organization lookup answers, empty for no
// organization, and usersInserted the row count of the admin INSERT.
func expectBootstrapTransaction(mock sqlmock.Sqlmock, orgDomain, orgID string, usersInserted int64) {
	orgRows := sqlmock.NewRows([]string{"id"})
	if orgID != "" {
		orgRows.AddRow(orgID)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(orgDomain).
		WillReturnRows(orgRows)
	if orgID == "" {
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO organizations`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WillReturnResult(sqlmock.NewResult(0, usersInserted))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO system_config`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

// guideBootstrapSteps returns the step lines of the sample run in
// docs/guides/BOOTSTRAP_GUIDE.md: the lines after "Starting bootstrap
// process..." and before "Bootstrap completed successfully!", trailing blank
// lines dropped.
func guideBootstrapSteps(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "guides", "BOOTSTRAP_GUIDE.md"))
	if err != nil {
		t.Fatalf("read the bootstrap guide: %v", err)
	}
	guide := string(raw)
	const start = "Starting bootstrap process...\n"
	const end = "Bootstrap completed successfully!"
	i := strings.Index(guide, start)
	if i < 0 {
		t.Fatalf("the bootstrap guide has no %q line", strings.TrimSpace(start))
	}
	rest := guide[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("the bootstrap guide has no %q line after the steps", end)
	}
	return strings.TrimRight(rest[:j], "\n") + "\n"
}

// A run against a database with no organization and no admin prints the step
// lines the guide's sample run shows: a number, a space and the step, then the
// step's result indented three spaces under it.
func TestRunBootstrap_FreshDatabasePrintsTheGuidesSteps(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// The configuration of the guide's Step 2 command.
	cfg := &BootstrapConfig{
		AdminEmail:    "admin@company.com",
		AdminPassword: "FAKE-pass-1234",
		AdminName:     "System Administrator",
		OrgName:       "My Company",
		OrgDomain:     "localhost",
		MaxUsers:      100,
		MaxAgents:     1000,
	}
	expectBootstrapTransaction(mock, cfg.OrgDomain, "", 1)

	var out bytes.Buffer
	if err := runBootstrap(context.Background(), &out, db, cfg); err != nil {
		t.Fatalf("runBootstrap: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("statements: %v", err)
	}

	want := withoutIDs(guideBootstrapSteps(t))
	if got := withoutIDs(out.String()); got != want {
		t.Errorf("runBootstrap printed:\n%s\nthe bootstrap guide shows:\n%s", got, want)
	}
}

// A --default re-run that finds the organization and the admin already in
// place indents those results under their steps the same way.
func TestRunBootstrap_ExistingOrganizationAndAdminAreIndentedUnderTheirSteps(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	cfg := &BootstrapConfig{
		AdminEmail:    defaultAdminEmail,
		AdminPassword: "FAKE-pass-1234",
		AdminName:     "System Administrator",
		OrgName:       defaultOrgName,
		OrgDomain:     defaultOrgDomain,
		MaxUsers:      100,
		MaxAgents:     1000,
		Default:       true,
	}
	const existingOrgID = "6f1c2d3e-4a5b-4c6d-8e7f-0a1b2c3d4e5f"
	expectBootstrapTransaction(mock, cfg.OrgDomain, existingOrgID, 0)

	var out bytes.Buffer
	if err := runBootstrap(context.Background(), &out, db, cfg); err != nil {
		t.Fatalf("runBootstrap: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("statements: %v", err)
	}

	want := strings.Join([]string{
		"1. Checking organization...",
		"   Organization exists (ID: " + existingOrgID + ")",
		"2. Hashing password...",
		"   Password hashed",
		"3. Creating admin user...",
		"   Admin user already exists for " + defaultAdminEmail + " in this org — leaving stored password unchanged",
		"4. Updating system configuration...",
		"   System configuration updated",
	}, "\n") + "\n"
	if got := out.String(); got != want {
		t.Errorf("runBootstrap printed:\n%s\nwant:\n%s", got, want)
	}
}

// sameValue matches the first value it is compared with and, after that, only
// a value equal to it, so that statements can be required to carry the ID
// runBootstrap generates for the admin.
type sameValue struct{ seen *driver.Value }

func newSameValue() sameValue { return sameValue{seen: new(driver.Value)} }

func (s sameValue) Match(v driver.Value) bool {
	if *s.seen == nil {
		*s.seen = v
		return true
	}
	return *s.seen == v
}

// canonicalDefaultConfig is the configuration `aim-bootstrap --default` runs
// with when no flag overrides a canonical value.
func canonicalDefaultConfig() *BootstrapConfig {
	return &BootstrapConfig{
		AdminEmail:    defaultAdminEmail,
		AdminPassword: "FAKE-pass-1234",
		AdminName:     defaultAdminName,
		OrgName:       defaultOrgName,
		OrgDomain:     defaultOrgDomain,
		MaxUsers:      defaultMaxUsers,
		MaxAgents:     defaultMaxAgents,
		Default:       true,
	}
}

// expectDefaultRunCreatingTheAdmin queues the statements of a --default run
// that finds the organization orgID and creates the admin. Each default
// policy INSERT must carry orgID and the ID the admin INSERT carried; the
// last present of them answer that they added no row.
func expectDefaultRunCreatingTheAdmin(mock sqlmock.Sqlmock, orgDomain, orgID string, present int) {
	admin := newSameValue()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(orgDomain).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orgID))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(admin, orgID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
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
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO system_config`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

// A --default run that creates the admin adds the default security policies
// to the organization it found, as created by that admin, in the transaction
// and before the bootstrap_completed marker, and prints how many it added. A
// policy the organization already had is counted apart.
func TestRunBootstrap_DefaultRunThatCreatesTheAdminAddsTheDefaultPolicies(t *testing.T) {
	t.Parallel()

	const orgID = "6f1c2d3e-4a5b-4c6d-8e7f-0a1b2c3d4e5f"
	all := len(seed.DefaultPolicies)
	for _, tc := range []struct {
		name    string
		present int
		result  string
	}{
		{"organization without policies", 0, fmt.Sprintf("   Added %d default security policies", all)},
		{"organization with two of them", 2, fmt.Sprintf("   Added %d default security policies; 2 the organization already had are left unchanged", all-2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock.New: %v", err)
			}
			defer db.Close()

			cfg := canonicalDefaultConfig()
			expectDefaultRunCreatingTheAdmin(mock, cfg.OrgDomain, orgID, tc.present)

			var out bytes.Buffer
			if err := runBootstrap(context.Background(), &out, db, cfg); err != nil {
				t.Fatalf("runBootstrap: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("statements: %v", err)
			}

			want := withoutIDs(strings.Join([]string{
				"1. Checking organization...",
				"   Organization exists (ID: " + orgID + ")",
				"2. Hashing password...",
				"   Password hashed",
				"3. Creating admin user...",
				"   Admin user created (ID: <id>)",
				"4. Adding default security policies...",
				tc.result,
				"5. Updating system configuration...",
				"   System configuration updated",
			}, "\n") + "\n")
			if got := withoutIDs(out.String()); got != want {
				t.Errorf("runBootstrap printed:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// When the database refuses a default policy, the run fails and its
// transaction is rolled back: the admin it inserted is not committed and the
// bootstrap_completed marker is not set.
func TestRunBootstrap_RefusedDefaultPolicyRollsTheRunBack(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	cfg := canonicalDefaultConfig()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(cfg.OrgDomain).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("6f1c2d3e-4a5b-4c6d-8e7f-0a1b2c3d4e5f"))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_policies`)).
		WillReturnError(errors.New("refused by the test"))
	mock.ExpectRollback()

	var out bytes.Buffer
	err = runBootstrap(context.Background(), &out, db, cfg)
	if err == nil || !strings.Contains(err.Error(), "failed to add default security policies") {
		t.Fatalf("runBootstrap returned %v, want the refused policy's error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("statements: %v", err)
	}
}

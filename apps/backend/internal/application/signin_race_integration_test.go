//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/require"
)

// A password sign-in reads the user row, spends a bcrypt comparison on the hash
// it read, and then records the sign-in. A change another request commits to
// that row inside the window (a password change, a deactivation, a role change,
// a reset-token use or request) has to survive the sign-in's write. Each case
// holds the sign-in between its read and its write, commits the other change
// through the service that owns it, then lets the sign-in finish and reads the
// row back.
//
// The statements are what is under test, so these drive the real
// UserRepository against a real Postgres rather than a mock.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSignInRace ./internal/application/...

const (
	signInRaceOldPassword = "SignInRace-0ld-Passw0rd!"
	signInRaceNewPassword = "SignInRace-N3w-Passw0rd!"
)

func signInRaceDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping sign-in race test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// signInHold wraps the repository the sign-in reads through. Its first
// GetByEmail reads the row, signals, and waits to be released before returning
// it, so the competing change commits while the sign-in holds an earlier read.
type signInHold struct {
	domain.UserRepository
	once    sync.Once
	read    chan struct{}
	release chan struct{}
}

func (h *signInHold) GetByEmail(email string) (*domain.User, error) {
	u, err := h.UserRepository.GetByEmail(email)
	h.once.Do(func() {
		close(h.read)
		<-h.release
	})
	return u, err
}

// discardAuditLog accepts the audit entry ResetPassword writes; the audit trail
// is not what these cases check.
type discardAuditLog struct{ domain.AuditLogRepository }

func (discardAuditLog) Create(*domain.AuditLog) error { return nil }

type signInRaceFixture struct {
	db     *sql.DB
	auth   *AuthService
	reset  *RegistrationService
	orgID  uuid.UUID
	userID uuid.UUID
	email  string
	users  *repository.UserRepository
	token  string
}

// signInRaceRow is the stored row, read with SQL so no repository read path
// decides which columns the assertions can see.
type signInRaceRow struct {
	role         string
	status       string
	passwordHash string
	resetToken   sql.NullString
	lastLoginAt  sql.NullTime
}

func newSignInRaceFixture(t *testing.T, db *sql.DB) *signInRaceFixture {
	t.Helper()
	ctx := context.Background()
	orgID, userID := seedOrgAndUser(t, db, ctx, "signin-race")

	hash, err := auth.NewPasswordHasher().HashPassword(signInRaceOldPassword)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, status = 'active', role = 'admin', last_login_at = NULL
		 WHERE id = $2`, hash, userID)
	require.NoError(t, err)

	var email string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email))

	users := repository.NewUserRepository(db)
	return &signInRaceFixture{
		db:     db,
		auth:   NewAuthService(users, nil, nil, nil, nil, nil),
		reset:  NewRegistrationService(nil, users, nil, NewAuditService(discardAuditLog{}), nil),
		orgID:  orgID,
		userID: userID,
		email:  email,
		users:  users,
	}
}

func (f *signInRaceFixture) row(t *testing.T) signInRaceRow {
	t.Helper()
	var r signInRaceRow
	var status sql.NullString
	var hash sql.NullString
	require.NoError(t, f.db.QueryRow(
		`SELECT role, status, password_hash, password_reset_token, last_login_at
		 FROM users WHERE id = $1`, f.userID,
	).Scan(&r.role, &status, &hash, &r.resetToken, &r.lastLoginAt))
	r.status, r.passwordHash = status.String, hash.String
	return r
}

func signInRacePasswordIs(r signInRaceRow, password string) bool {
	return auth.NewPasswordHasher().VerifyPassword(password, r.passwordHash) == nil
}

type signInRaceCase struct {
	name string
	// setup runs before the sign-in starts.
	setup func(t *testing.T, f *signInRaceFixture)
	// commit makes the competing change through the service that owns it.
	commit func(ctx context.Context, f *signInRaceFixture) error
	// holds reports whether the stored row carries the competing change.
	holds func(r signInRaceRow) bool
}

func signInRaceCases() []signInRaceCase {
	return []signInRaceCase{
		{
			name: "password change",
			commit: func(ctx context.Context, f *signInRaceFixture) error {
				return f.auth.ChangePassword(ctx, f.userID, signInRaceOldPassword, signInRaceNewPassword)
			},
			holds: func(r signInRaceRow) bool { return signInRacePasswordIs(r, signInRaceNewPassword) },
		},
		{
			name: "deactivation",
			commit: func(ctx context.Context, f *signInRaceFixture) error {
				return f.auth.DeactivateUser(ctx, f.userID, f.orgID, uuid.New())
			},
			holds: func(r signInRaceRow) bool { return r.status == string(domain.UserStatusDeactivated) },
		},
		{
			name: "role lowered",
			commit: func(ctx context.Context, f *signInRaceFixture) error {
				_, err := f.auth.UpdateUserRole(ctx, f.userID, f.orgID, domain.RoleViewer, uuid.New())
				return err
			},
			holds: func(r signInRaceRow) bool { return r.role == string(domain.RoleViewer) },
		},
		{
			name: "reset token used",
			setup: func(t *testing.T, f *signInRaceFixture) {
				require.NoError(t, f.reset.RequestPasswordReset(context.Background(), f.email))
				issued := f.row(t).resetToken
				require.True(t, issued.Valid, "RequestPasswordReset did not store a token")
				f.token = issued.String
			},
			commit: func(ctx context.Context, f *signInRaceFixture) error {
				return f.reset.ResetPassword(ctx, f.token, signInRaceNewPassword, signInRaceNewPassword)
			},
			holds: func(r signInRaceRow) bool {
				return !r.resetToken.Valid && signInRacePasswordIs(r, signInRaceNewPassword)
			},
		},
		{
			name: "reset requested",
			commit: func(ctx context.Context, f *signInRaceFixture) error {
				return f.reset.RequestPasswordReset(ctx, f.email)
			},
			holds: func(r signInRaceRow) bool { return r.resetToken.Valid },
		},
	}
}

func TestSignInRace_ChangeCommittedDuringSignInStaysCommitted(t *testing.T) {
	db := signInRaceDB(t)

	for _, c := range signInRaceCases() {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			f := newSignInRaceFixture(t, db)
			if c.setup != nil {
				c.setup(t, f)
			}

			hold := &signInHold{UserRepository: f.users, read: make(chan struct{}), release: make(chan struct{})}
			release := sync.OnceFunc(func() { close(hold.release) })
			t.Cleanup(release)
			signIn := NewAuthService(hold, nil, nil, nil, nil, nil)

			type outcome struct {
				user *domain.User
				err  error
			}
			done := make(chan outcome, 1)
			go func() {
				u, err := signIn.LoginWithPassword(ctx, f.email, signInRaceOldPassword)
				done <- outcome{u, err}
			}()

			select {
			case <-hold.read:
			case <-time.After(30 * time.Second):
				t.Fatal("the sign-in never read the user")
			}

			// Positive control: the change is absent when the sign-in reads,
			// and committed before the sign-in writes.
			require.False(t, c.holds(f.row(t)), "the %s was in the row before the sign-in read it", c.name)
			require.NoError(t, c.commit(ctx, f))
			require.True(t, c.holds(f.row(t)), "the %s did not commit while the sign-in held its read", c.name)

			release()
			var out outcome
			select {
			case out = <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("the sign-in did not finish after release")
			}
			require.NoError(t, out.err, "the sign-in read the row before the change, so it completes")

			after := f.row(t)
			require.True(t, after.lastLoginAt.Valid, "the sign-in did not record last_login_at")
			require.True(t, c.holds(after), "the sign-in wrote its earlier read back over the %s", c.name)
		})
	}
}

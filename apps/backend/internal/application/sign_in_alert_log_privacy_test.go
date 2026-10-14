package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Password sign-in, and the auth-failure and lock alerts it raises, log the
// outcome and the organization, never the account's email address: a server
// log is kept far longer than any reason to hold the address, and a
// database's error text can echo it.

const (
	signInAlertLogProbeLocal   = "planted.account.probe"
	signInAlertLogProbeAddress = signInAlertLogProbeLocal + "@example.test"
	signInAlertLogProbeSecret  = "Planted-Pass-Phrase-91!"
)

// signInAlertLogEchoError stands in for a database error that repeats the
// address it was given.
func signInAlertLogEchoError(email string) error {
	return fmt.Errorf("550 5.1.1 <%s>: rejected", email)
}

type signInAlertLogUserRepo struct {
	domain.UserRepository
	user *domain.User
}

func (r *signInAlertLogUserRepo) GetByEmail(email string) (*domain.User, error) {
	if r.user != nil && r.user.Email == email {
		return r.user, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *signInAlertLogUserRepo) Update(*domain.User) error { return nil }

func (r *signInAlertLogUserRepo) UpdateLastLogin(uuid.UUID, time.Time) error { return nil }

// signInAlertLogFailures fails the auth-failure calls named in fail, each with
// an error that echoes the address it was asked about.
type signInAlertLogFailures struct {
	domain.AuthFailureRepository
	fail  map[string]bool
	count int
}

func (r *signInAlertLogFailures) err(method, email string) error {
	if r.fail[method] {
		return signInAlertLogEchoError(email)
	}
	return nil
}

func (r *signInAlertLogFailures) GetActiveLockout(email string) (*domain.AuthLockout, error) {
	return nil, r.err("GetActiveLockout", email)
}

func (r *signInAlertLogFailures) RecordFailure(f *domain.AuthFailure) error {
	return r.err("RecordFailure", f.Email)
}

func (r *signInAlertLogFailures) CountRecentFailures(email string, _ int) (int, error) {
	return r.count, r.err("CountRecentFailures", email)
}

func (r *signInAlertLogFailures) CreateLockout(l *domain.AuthLockout) error {
	return r.err("CreateLockout", l.Email)
}

func (r *signInAlertLogFailures) ClearFailures(email string) error {
	return r.err("ClearFailures", email)
}

// signInAlertLogAlerts accepts every alert and reports none on file, so the
// policy service reaches its "alert created" lines.
type signInAlertLogAlerts struct{ domain.AlertRepository }

func (signInAlertLogAlerts) Create(*domain.Alert) error { return nil }

func (signInAlertLogAlerts) GetByOrganization(uuid.UUID, int, int) ([]*domain.Alert, error) {
	return nil, nil
}

// captureSignInAlertLogOutput runs fn with the standard logger, stdout and
// stderr all writing to one pipe and returns everything written.
func captureSignInAlertLogOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut, oldErr, oldLog := os.Stdout, os.Stderr, log.Writer()
	os.Stdout, os.Stderr = w, w
	log.SetOutput(w)
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() {
			os.Stdout, os.Stderr = oldOut, oldErr
			log.SetOutput(oldLog)
			_ = w.Close()
		}()
		fn()
	}()
	return <-done
}

func requireSignInAlertLogNoAddress(t *testing.T, site, output string) {
	t.Helper()
	require.NotEmpty(t, output, "%s logged nothing, so the check would pass vacuously", site)
	require.NotContains(t, output, signInAlertLogProbeLocal, "%s wrote the address (or its local part) to the log:\n%s", site, output)
}

func TestPasswordSignInFailureAndAlertLogsCarryNoEmailAddress(t *testing.T) {
	ctx := context.Background()
	hash, err := auth.NewPasswordHasher().HashPassword(signInAlertLogProbeSecret)
	require.NoError(t, err)
	member := &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          signInAlertLogProbeAddress,
		Status:         domain.UserStatusActive,
		PasswordHash:   &hash,
	}

	// The server wires a policy service with an alert repository into the
	// sign-in service, so the alert lines it prints on the third and fifth
	// wrong password for an existing account are part of the sign-in log.
	alerts := NewSecurityPolicyService(nil, signInAlertLogAlerts{}, nil)
	cfg := domain.DefaultAuthFailureConfig()

	cases := []struct {
		name     string
		users    *signInAlertLogUserRepo
		password string
		failures *signInAlertLogFailures
		policy   *SecurityPolicyService
	}{
		{"lockout lookup fails", &signInAlertLogUserRepo{}, "wrong",
			&signInAlertLogFailures{fail: map[string]bool{"GetActiveLockout": true, "RecordFailure": true}}, nil},
		{"failure count fails", &signInAlertLogUserRepo{}, "wrong",
			&signInAlertLogFailures{fail: map[string]bool{"CountRecentFailures": true}}, nil},
		{"lockout insert fails", &signInAlertLogUserRepo{}, "wrong",
			&signInAlertLogFailures{fail: map[string]bool{"CreateLockout": true}, count: 1000}, nil},
		{"clearing failures after sign-in fails", &signInAlertLogUserRepo{user: member}, signInAlertLogProbeSecret,
			&signInAlertLogFailures{fail: map[string]bool{"ClearFailures": true}}, nil},
		{"failure pattern alert for an existing account", &signInAlertLogUserRepo{user: member}, "wrong",
			&signInAlertLogFailures{count: cfg.AlertThreshold}, alerts},
		{"lockout alert for an existing account", &signInAlertLogUserRepo{user: member}, "wrong",
			&signInAlertLogFailures{count: cfg.MaxAttempts}, alerts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewAuthService(tc.users, nil, nil, tc.failures, tc.policy, nil)
			out := captureSignInAlertLogOutput(t, func() {
				_, _ = svc.LoginWithPasswordExtended(ctx, signInAlertLogProbeAddress, tc.password, "", "")
			})
			requireSignInAlertLogNoAddress(t, tc.name, out)
		})
	}
}

//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An approved device code mints one pair. Consume carries the status test in
// the same UPDATE that changes it, and holds the row lock while the pair is
// minted. Whether a second poll is refused depends on how Postgres re-checks
// that WHERE clause after the first transaction commits, so a mocked
// repository cannot exercise it. These tests drive the REAL repository against
// a real Postgres.
//
// Build-tag gated; requires the AIM schema:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestDeviceCodeConsume ./internal/infrastructure/repository/...

func deviceConsumeDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping device code consume integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedApprovedDeviceCode inserts its own approved code. user_id and
// organization_id are nullable and Consume does not read them, so the fixture
// needs no user or organization.
func seedApprovedDeviceCode(t *testing.T, db *sql.DB) string {
	t.Helper()
	deviceCode := "consume-" + uuid.NewString()
	userCode := uuid.NewString()[:8]
	_, err := db.Exec(`
        INSERT INTO device_codes
            (id, device_code, user_code, client_id, scope, verification_uri,
             expires_at, interval_seconds, status, approved_at, created_at)
        VALUES ($1, $2, $3, 'aim-sdk', '', 'http://localhost:3000/device',
                $4, 10, 'approved', NOW(), NOW())`,
		uuid.New(), deviceCode, userCode, time.Now().Add(10*time.Minute))
	require.NoError(t, err, "seed approved device code")
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM device_codes WHERE device_code = $1`, deviceCode) })
	return deviceCode
}

func deviceCodeStatus(t *testing.T, db *sql.DB, deviceCode string) string {
	t.Helper()
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM device_codes WHERE device_code = $1`, deviceCode).Scan(&status))
	return status
}

// waitForLockWaiter returns once another backend in this database is waiting
// on a lock inside a device_codes UPDATE, or after the timeout.
func waitForLockWaiter(db *sql.DB, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var waiting int
		err := db.QueryRow(`
            SELECT count(*) FROM pg_stat_activity
            WHERE datname = current_database() AND wait_event_type = 'Lock'
              AND query LIKE '%UPDATE device_codes%'
              AND pid <> pg_backend_pid()`).Scan(&waiting)
		if err == nil && waiting > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// Two polls on one approved code yield exactly one pair. The second poll is
// started while the first is inside its mint, and the first commits only once
// the second is waiting on the row, so the overlap is forced on every run
// rather than left to scheduling. A read of the status followed by a separate
// UPDATE passes its read here, then updates after the first commits, and mints
// a second pair.
func TestDeviceCodeConsume_TwoConcurrentPolls_YieldExactlyOnePair(t *testing.T) {
	db := deviceConsumeDB(t)
	repo := NewDeviceCodeRepository(db)
	deviceCode := seedApprovedDeviceCode(t, db)
	ctx := context.Background()

	firstInMint := make(chan struct{})
	firstDone := make(chan error, 1)
	issued := make(chan string, 2)

	go func() {
		firstDone <- repo.Consume(ctx, deviceCode, func() error {
			close(firstInMint)
			// Hold the transaction open until the second poll is blocked on the
			// row, then commit.
			waitForLockWaiter(db, 5*time.Second)
			issued <- "first"
			return nil
		})
	}()

	<-firstInMint
	secondErr := repo.Consume(ctx, deviceCode, func() error {
		issued <- "second"
		return nil
	})
	firstErr := <-firstDone
	close(issued)

	var minted []string
	for who := range issued {
		minted = append(minted, who)
	}
	require.NoError(t, firstErr)
	assert.ErrorIs(t, secondErr, domain.ErrDeviceCodeNotApproved)
	assert.Equal(t, []string{"first"}, minted, "one approval mints one pair")
	assert.Equal(t, string(domain.DeviceCodeStatusConsumed), deviceCodeStatus(t, db, deviceCode))
}

// A poll on a consumed code mints nothing.
func TestDeviceCodeConsume_ConsumedCode_MintsNothing(t *testing.T) {
	db := deviceConsumeDB(t)
	repo := NewDeviceCodeRepository(db)
	deviceCode := seedApprovedDeviceCode(t, db)
	ctx := context.Background()

	require.NoError(t, repo.Consume(ctx, deviceCode, func() error { return nil }))

	issued := 0
	err := repo.Consume(ctx, deviceCode, func() error { issued++; return nil })
	assert.ErrorIs(t, err, domain.ErrDeviceCodeNotApproved)
	assert.Equal(t, 0, issued)
}

// A pair that fails to mint leaves the code approved, and the next poll
// receives the pair.
func TestDeviceCodeConsume_MintFails_CodeStaysApproved(t *testing.T) {
	db := deviceConsumeDB(t)
	repo := NewDeviceCodeRepository(db)
	deviceCode := seedApprovedDeviceCode(t, db)
	ctx := context.Background()

	mintErr := errors.New("signing unavailable")
	err := repo.Consume(ctx, deviceCode, func() error { return mintErr })
	assert.ErrorIs(t, err, mintErr)
	assert.Equal(t, string(domain.DeviceCodeStatusApproved), deviceCodeStatus(t, db, deviceCode))

	require.NoError(t, repo.Consume(ctx, deviceCode, func() error { return nil }))
	assert.Equal(t, string(domain.DeviceCodeStatusConsumed), deviceCodeStatus(t, db, deviceCode))
}

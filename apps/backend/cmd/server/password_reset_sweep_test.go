package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// The statement the cleanup job runs against users. It clears only the two
// reset columns, does not set updated_at, and reads nothing back, so the job
// can log a count and never a token or an address.
const wantClearResetTokensSQL = `
	UPDATE users
	SET password_reset_token = NULL,
	    password_reset_expires_at = NULL
	WHERE password_reset_expires_at <= NOW()
	   OR (password_reset_token IS NOT NULL AND password_reset_expires_at IS NULL)
`

func expectVerificationSweep(mock sqlmock.Sqlmock) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`UPDATE verification_events`)
}

func expectResetTokenSweep(mock sqlmock.Sqlmock) *sqlmock.ExpectedExec {
	return mock.ExpectExec(`^` + regexp.QuoteMeta(strings.TrimSpace(wantClearResetTokensSQL)) + `$`).WithArgs()
}

// Every tick of the five-minute job clears reset tokens that can no longer
// be redeemed, and logs how many it cleared.
func TestExpirationCleanup_ClearsExpiredPasswordResetTokens(t *testing.T) {
	db, mock := newSeedMock(t)
	logs := captureLog(t)
	expectVerificationSweep(mock).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	expectResetTokenSweep(mock).WillReturnResult(sqlmock.NewResult(0, 2))

	runExpirationCleanup(db)

	if !strings.Contains(logs.String(), "Cleared 2 expired password reset token(s)") {
		t.Errorf("log does not report the cleared count:\n%s", logs.String())
	}
}

// A tick that finds nothing to clear stays quiet.
func TestExpirationCleanup_NothingToClearLogsNothing(t *testing.T) {
	db, mock := newSeedMock(t)
	logs := captureLog(t)
	expectVerificationSweep(mock).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	expectResetTokenSweep(mock).WillReturnResult(sqlmock.NewResult(0, 0))

	runExpirationCleanup(db)

	if strings.Contains(logs.String(), "password reset") {
		t.Errorf("a tick with nothing to clear wrote a reset token line:\n%s", logs.String())
	}
}

// A failure in the verification sweep does not skip the reset token sweep,
// and a failure in the reset token sweep is logged, not fatal.
func TestExpirationCleanup_EachSweepRunsWhenTheOtherFails(t *testing.T) {
	db, mock := newSeedMock(t)
	logs := captureLog(t)
	expectVerificationSweep(mock).WillReturnError(errors.New("verification sweep down"))
	expectResetTokenSweep(mock).WillReturnError(errors.New("reset sweep down"))

	runExpirationCleanup(db)

	for _, want := range []string{"verification sweep down", "reset sweep down"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log does not carry %q:\n%s", want, logs.String())
		}
	}
}

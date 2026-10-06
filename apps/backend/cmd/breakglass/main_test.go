package main

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	recordstore "github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

func envOf(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// Usage and configuration errors exit 2 before any database is opened: none
// of these invocations names a database, so a read would fail with exit 1.
func TestUsageErrorsExitTwoAndOpenNoDatabase(t *testing.T) {
	const org = "1b4e28ba-2fa1-11d2-883f-0016d3cca427"
	for _, tc := range []struct {
		name   string
		args   []string
		env    map[string]string
		stderr string
	}{
		{"no command", nil, nil, "Usage: aim-breakglass"},
		{"unknown command", []string{"agent"}, nil, `unknown command "agent"`},
		{"chain without a subcommand", []string{"chain"}, nil, "name a subcommand"},
		{"unknown chain subcommand", []string{"chain", "verify"}, nil, `unknown subcommand "verify"`},
		{"no organization", []string{"chain", "status"}, nil, "--organization is required"},
		{"organization not a UUID", []string{"chain", "status", "--organization", "acme"}, nil, `--organization "acme" is not a UUID`},
		{"unknown flag", []string{"chain", "status", "--org", org}, nil, "flag provided but not defined: -org"},
		{"extra argument", []string{"chain", "status", "--organization", org, "now"}, nil, `unexpected argument "now"`},
		{"no database configured", []string{"chain", "status", "--organization", org}, nil, "no database is configured"},
		{"POSTGRES_HOST without its user and database", []string{"chain", "status", "--organization", org},
			map[string]string{"POSTGRES_HOST": "db"}, "POSTGRES_HOST is set but POSTGRES_USER and POSTGRES_DB are not"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tc.args, envOf(tc.env), &stdout, &stderr)
			require.Equal(t, exitUsage, code, stderr.String())
			require.Contains(t, stderr.String(), tc.stderr)
			require.Empty(t, stdout.String())
		})
	}
}

func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"chain", "help"}} {
		var stdout, stderr bytes.Buffer
		require.Equal(t, exitOK, run(context.Background(), args, envOf(nil), &stdout, &stderr), args)
		require.Contains(t, stdout.String(), "chain status --organization <id> [--json]", args)
	}
	var stdout, stderr bytes.Buffer
	require.Equal(t, exitOK, run(context.Background(), []string{"chain", "status", "-h"}, envOf(nil), &stdout, &stderr))
	require.Contains(t, stderr.String(), "-organization")
	require.Contains(t, stderr.String(), "-json")
}

// The command reads the server's database: the server's POSTGRES_* variables
// win over DATABASE_URL, and a password with characters that need quoting
// reaches the driver unchanged.
func TestDatabaseDSN(t *testing.T) {
	const password = `p'a\ss w@rd:/?#%`
	dsn, err := databaseDSN(envOf(map[string]string{
		"POSTGRES_HOST":     "db.internal",
		"POSTGRES_PORT":     "6543",
		"POSTGRES_USER":     "aim",
		"POSTGRES_PASSWORD": password,
		"POSTGRES_DB":       "aim",
		"POSTGRES_SSL_MODE": "require",
		"DATABASE_URL":      "postgres://other@elsewhere/other",
	}))
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, "db.internal:6543", u.Host)
	require.Equal(t, "/aim", u.Path)
	require.Equal(t, "require", u.Query().Get("sslmode"))
	require.Equal(t, "aim", u.User.Username())
	got, ok := u.User.Password()
	require.True(t, ok)
	require.Equal(t, password, got)
	_, err = pq.ParseURL(dsn)
	require.NoError(t, err, "the driver accepts the connection string")

	dsn, err = databaseDSN(envOf(map[string]string{"POSTGRES_HOST": "db", "POSTGRES_USER": "aim", "POSTGRES_DB": "aim"}))
	require.NoError(t, err)
	require.Equal(t, "postgres://aim@db:5432/aim?sslmode=disable", dsn, "the server's defaults: port 5432, sslmode disable, no password")

	dsn, err = databaseDSN(envOf(map[string]string{"DATABASE_URL": "postgres://aim@db/aim"}))
	require.NoError(t, err)
	require.Equal(t, "postgres://aim@db/aim", dsn)
}

// A configuration error names the variable, never a value it holds.
func TestDatabaseDSNErrorsCarryNoValue(t *testing.T) {
	const secret = "s3cr3t-value"
	for _, env := range []map[string]string{
		{"POSTGRES_HOST": "db", "POSTGRES_PASSWORD": secret},
		{"POSTGRES_HOST": "db", "POSTGRES_USER": "aim", "POSTGRES_DB": "aim", "POSTGRES_PORT": secret},
		{"DATABASE_URL": "postgres://aim:" + secret + "@db:port/aim"},
	} {
		_, err := databaseDSN(envOf(env))
		require.Error(t, err)
		require.NotContains(t, err.Error(), secret)
	}
}

func TestWriteChainStatus(t *testing.T) {
	const org = "1b4e28ba-2fa1-11d2-883f-0016d3cca427"
	chainID := "6f1c3a52-9d0e-4b7a-8c2f-3e5d7a9b1c04"
	hash := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		status recordstore.ChainStatus
		want   string
	}{
		{recordstore.ChainStatus{State: record.ChainNotStarted}, `organization      1b4e28ba-2fa1-11d2-883f-0016d3cca427
chainState        notStarted (no genesis record)
chainId           none
head              none
latestCheckpoint  none
`},
		{recordstore.ChainStatus{State: record.ChainExtendable, ChainID: &chainID, Head: &recordstore.HeadView{Seq: 41, Hash: hash}},
			`organization      1b4e28ba-2fa1-11d2-883f-0016d3cca427
chainState        extendable (the next record can link to the stored head)
chainId           6f1c3a52-9d0e-4b7a-8c2f-3e5d7a9b1c04
head              seq 41, hash ` + hash + `
latestCheckpoint  none
`},
		{recordstore.ChainStatus{State: record.ChainNotExtendable, ChainID: &chainID, Head: &recordstore.HeadView{Seq: 0, Hash: hash},
			Reason: recordstore.NotExtendableHeadMismatch},
			`organization      1b4e28ba-2fa1-11d2-883f-0016d3cca427
chainState        notExtendable (the chain started, but its stored head cannot be linked to)
chainId           6f1c3a52-9d0e-4b7a-8c2f-3e5d7a9b1c04
head              seq 0, hash ` + hash + `
latestCheckpoint  none
reason            head_mismatch (the stored head does not name the chain's newest record)
`},
	} {
		var out bytes.Buffer
		writeChainStatus(&out, org, tc.status)
		require.Equal(t, tc.want, out.String())
	}
}

// Every chain state and every reason the record store defines has a meaning
// to print.
func TestEveryStateAndReasonHasAMeaning(t *testing.T) {
	for _, state := range []record.ChainState{record.ChainNotStarted, record.ChainExtendable, record.ChainNotExtendable} {
		require.NotEmpty(t, stateMeaning[state], state)
	}
	for _, reason := range []recordstore.NotExtendableReason{
		recordstore.NotExtendableNoRecords, recordstore.NotExtendableHeadMismatch, recordstore.NotExtendableRecordModified,
	} {
		require.NotEmpty(t, reasonMeaning[reason], reason)
	}
}

// A connection string the driver refuses exits 2 without echoing it: the
// driver's own error would quote it, password included.
func TestARefusedConnectionStringIsNotEchoed(t *testing.T) {
	const secret = "s3cr3t-value"
	var stdout, stderr bytes.Buffer
	code := run(context.Background(),
		[]string{"chain", "status", "--organization", "1b4e28ba-2fa1-11d2-883f-0016d3cca427"},
		envOf(map[string]string{"DATABASE_URL": "mysql://aim:" + secret + "@db/aim"}), &stdout, &stderr)
	require.Equal(t, exitUsage, code, stderr.String())
	require.Contains(t, stderr.String(), "not one the driver accepts")
	require.NotContains(t, stderr.String(), secret)
	require.Empty(t, stdout.String())
}

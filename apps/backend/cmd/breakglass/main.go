// Command aim-breakglass is the deployment operator's command. It runs next to
// the server, against the server's database, for an operator who cannot or
// should not go through the API.
//
//	aim-breakglass chain status --organization <id> [--json]
//
// chain status reports one organization's audit record chain through the
// record store's one chain-state read, the same read the admin route
// GET /api/v1/admin/audit-logs/chain/head answers with. --json prints the
// record store's ChainReport of that read, which is the route's body.
//
// The database is the server's: POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER,
// POSTGRES_PASSWORD, POSTGRES_DB and POSTGRES_SSL_MODE when POSTGRES_HOST is
// set, and DATABASE_URL otherwise.
//
// Exit codes: 0 when the command did what it was asked, whatever the chain's
// state; 1 when it could not (database unreachable, unknown organization,
// failed read); 2 for a usage or configuration error.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	recordstore "github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// readTimeout bounds one invocation's database work, connection included.
const readTimeout = 30 * time.Second

const usage = `Usage: aim-breakglass <command> <subcommand> [flags]

Commands:
  chain status --organization <id> [--json]
      Report an organization's audit record chain: chainState (notStarted,
      extendable or notExtendable), chainId, head, latestCheckpoint, and the
      reason when the chain cannot be extended.

The database is the server's: POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER,
POSTGRES_PASSWORD, POSTGRES_DB and POSTGRES_SSL_MODE when POSTGRES_HOST is
set, and DATABASE_URL otherwise.

Exit codes: 0 done, 1 failed, 2 usage or configuration error.
`

const chainStatusUsage = `Usage: aim-breakglass chain status --organization <id> [--json]

Reports an organization's audit record chain, as GET
/api/v1/admin/audit-logs/chain/head reports it to the organization's admins.

Flags:
`

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

// run executes one invocation and returns its exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "chain":
	default:
		fmt.Fprintf(stderr, "aim-breakglass: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
	if len(args) < 2 {
		fmt.Fprintf(stderr, "aim-breakglass chain: name a subcommand\n\n%s", usage)
		return exitUsage
	}
	switch args[1] {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "status":
		return chainStatus(ctx, args[2:], getenv, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "aim-breakglass chain: unknown subcommand %q\n\n%s", args[1], usage)
		return exitUsage
	}
}

// chainStatus runs chain status.
func chainStatus(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("aim-breakglass chain status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	organization := fs.String("organization", "", "the organization's id (a UUID)")
	asJSON := fs.Bool("json", false, "print the chain-state read's value as JSON, as the admin route answers it")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), chainStatusUsage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "aim-breakglass chain status: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}
	if *organization == "" {
		fmt.Fprintln(stderr, "aim-breakglass chain status: --organization is required")
		return exitUsage
	}
	orgID, err := uuid.Parse(*organization)
	if err != nil {
		fmt.Fprintf(stderr, "aim-breakglass chain status: --organization %q is not a UUID\n", *organization)
		return exitUsage
	}
	dsn, err := databaseDSN(getenv)
	if err != nil {
		fmt.Fprintf(stderr, "aim-breakglass: %v\n", err)
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	// The driver parses a connection string at its first use otherwise, and
	// its parse error quotes the string, password included. Parsing it here
	// keeps that error out of every later one, and it is not passed on.
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		fmt.Fprintln(stderr, "aim-breakglass: the database connection string is not one the driver accepts")
		return exitUsage
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(1)

	var exists bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM organizations WHERE id = $1)`, orgID.String()).Scan(&exists); err != nil {
		fmt.Fprintf(stderr, "aim-breakglass: read the organization: %v\n", err)
		return exitFailure
	}
	if !exists {
		fmt.Fprintf(stderr, "aim-breakglass chain status: no organization has id %s\n", orgID)
		return exitFailure
	}
	status, err := recordstore.ReadChainState(ctx, db, orgID.String())
	if err != nil {
		fmt.Fprintf(stderr, "aim-breakglass: %v\n", err)
		return exitFailure
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(recordstore.ChainReport{ChainStatus: status}); err != nil {
			fmt.Fprintf(stderr, "aim-breakglass: write the chain state: %v\n", err)
			return exitFailure
		}
		return exitOK
	}
	writeChainStatus(stdout, orgID.String(), status)
	return exitOK
}

// stateMeaning says what each chain state means, in the words of its
// definition in the record package.
var stateMeaning = map[record.ChainState]string{
	record.ChainNotStarted:    "no genesis record",
	record.ChainExtendable:    "the next record can link to the stored head",
	record.ChainNotExtendable: "the chain started, but its stored head cannot be linked to",
}

// reasonMeaning says what each reason a chain is not extendable means.
var reasonMeaning = map[recordstore.NotExtendableReason]string{
	recordstore.NotExtendableNoRecords:      "the chain holds no record, its genesis included",
	recordstore.NotExtendableHeadMismatch:   "the stored head does not name the chain's newest record",
	recordstore.NotExtendableRecordModified: "the newest record's stored bytes do not hash to its stored hash",
}

// writeChainStatus prints a chain state for a person to read.
func writeChainStatus(w io.Writer, organizationID string, status recordstore.ChainStatus) {
	line := func(name, value string) { fmt.Fprintf(w, "%-18s%s\n", name, value) }
	line("organization", organizationID)
	state := string(status.State)
	if meaning, ok := stateMeaning[status.State]; ok {
		state += " (" + meaning + ")"
	}
	line("chainState", state)
	chainID, head := "none", "none"
	if status.ChainID != nil {
		chainID = *status.ChainID
	}
	if status.Head != nil {
		head = fmt.Sprintf("seq %d, hash %s", status.Head.Seq, status.Head.Hash)
	}
	line("chainId", chainID)
	line("head", head)
	line("latestCheckpoint", "none")
	if status.Reason != "" {
		reason := string(status.Reason)
		if meaning, ok := reasonMeaning[status.Reason]; ok {
			reason += " (" + meaning + ")"
		}
		line("reason", reason)
	}
}

// databaseDSN returns the connection string for the server's database: from
// the server's POSTGRES_* variables when POSTGRES_HOST is set, and from
// DATABASE_URL otherwise. Its errors name variables, never their values.
func databaseDSN(getenv func(string) string) (string, error) {
	if host := getenv("POSTGRES_HOST"); host != "" {
		var missing []string
		user, dbname := getenv("POSTGRES_USER"), getenv("POSTGRES_DB")
		if user == "" {
			missing = append(missing, "POSTGRES_USER")
		}
		if dbname == "" {
			missing = append(missing, "POSTGRES_DB")
		}
		switch len(missing) {
		case 1:
			return "", fmt.Errorf("POSTGRES_HOST is set but %s is not", missing[0])
		case 2:
			return "", fmt.Errorf("POSTGRES_HOST is set but %s are not", strings.Join(missing, " and "))
		}
		port := getenv("POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", errors.New("POSTGRES_PORT is not a port number")
		}
		sslMode := getenv("POSTGRES_SSL_MODE")
		if sslMode == "" {
			sslMode = "disable"
		}
		u := url.URL{
			Scheme:   "postgres",
			Host:     net.JoinHostPort(host, port),
			Path:     "/" + dbname,
			RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
		}
		if password := getenv("POSTGRES_PASSWORD"); password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
		return u.String(), nil
	}
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return "", errors.New("no database is configured: set POSTGRES_HOST, POSTGRES_USER and POSTGRES_DB as the server reads them, or DATABASE_URL")
	}
	// The parse error would quote the URL, password included, so it is not
	// passed on.
	if _, err := url.Parse(dsn); err != nil {
		return "", errors.New("DATABASE_URL is not a valid URL")
	}
	return dsn, nil
}

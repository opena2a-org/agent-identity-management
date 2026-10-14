package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Tenant scoping on the database connection.
//
// This file is the Go side of a per-connection tenant identity: it writes the
// identity, or a declared cross-tenant service scope, into a PostgreSQL session
// variable on the connection a query runs on. No migration in this repository
// enables row-level security, and no policy here reads these variables yet;
// until one does, setting them changes what no query returns.
//
// The backend sets no per-connection tenant identity on its own. The identity
// exists only in the request: the auth middleware writes
// c.Locals("organization_id") and the API-key middleware does the same, and
// every repository then shares one pooled *sql.DB with nothing on it. The
// primitives below carry that value the last hop, onto the connection the
// query actually runs on.
//
// SCOPE, STATED PLAINLY. This is the primitive, not the rollout. Repositories
// still hold a bare *sql.DB and still scope in their own SQL; converting them to
// take their connection from WithTenant is separate work. A policy that reads
// these variables must not be applied to an environment whose request path has
// not been converted, because a connection with no identity declared would
// then read zero rows -- that is the fail-closed behaviour such a policy is for.

const (
	// TenantGUC is the PostgreSQL session variable that carries the tenant
	// identity, for a row-level security policy to read. No migration in this
	// repository defines such a policy yet.
	//
	// The app.* prefix is not invented here: migration 032's
	// log_trust_score_change() already reads app.current_user_id, and this
	// follows that convention.
	TenantGUC = "app.current_organization_id"

	// ServiceScopeGUC is the session variable that declares a unit of work
	// cross-tenant. A policy that reads TenantGUC is expected to treat it as
	// the only named way past tenant isolation. The migration runners already
	// set it on every migration transaction.
	ServiceScopeGUC = "app.rls_service_scope"

	// serviceScopeOn is the single value that declares the service scope.
	// Anything else, including unset, reads as off.
	serviceScopeOn = "on"
)

// ErrNoTenantIdentity is returned rather than setting an empty tenant identity.
//
// A uuid.Nil organization is not a tenant; writing it to the session variable
// would produce a connection that quietly matches nothing while looking scoped,
// which is a worse failure than refusing up front. Callers that genuinely need
// to cross tenants say so with WithServiceScope.
var ErrNoTenantIdentity = errors.New("database: refusing to scope a connection to the nil organization; use WithServiceScope for cross-tenant work")

// ErrNoServiceScopeReason is returned by WithServiceScope when the caller does
// not say why it is crossing tenants.
//
// The reason is the whole control. A cross-tenant escape hatch that anyone can
// take without writing down why is how the app-layer scoping this sits under
// ended up with an allowlist of handlers nobody could account for.
var ErrNoServiceScopeReason = errors.New("database: WithServiceScope requires a reason describing why this work crosses tenants")

// Execer is the subset of *sql.DB, *sql.Conn and *sql.Tx these helpers need.
// Taking the interface lets a caller set the identity on a transaction, on a
// pinned connection, or (in tests) on a connection it manages itself.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TenantScopedTable records one tenant-scoped table and how its rows are bound
// to a tenant.
//
// Nothing in this repository yet ties this manifest to a row-level security
// policy: no migration here enables one, and no test compares the list to a
// migration. Its one reader today is the row security census, which counts
// statements naming these tables. A policy that is added later should be
// checked against this list, because a tenant-owned table in one and not the
// other is exactly the drift worth failing on -- a new tenant-owned table with
// no policy is an unprotected table.
type TenantScopedTable struct {
	// Table is the table name in the public schema.
	Table string

	// TenantColumn is the column carrying the tenant identity directly, or ""
	// when the table is tenant-owned through AgentColumn instead.
	TenantColumn string

	// AgentColumn is the column pointing at the owning agent, for tables that
	// carry no organization of their own. Empty when TenantColumn is set.
	AgentColumn string

	// LintParamKeys are names from cmd/tenantscope-lint's paramKeys that this
	// manifest pairs with the table. paramKeys is a set of URL parameter
	// names, not of tables, and the lint does not read this field.
	LintParamKeys []string
}

// TenantScopedTables lists the eleven tables the row security census looks
// for: it counts the statements that name one of them. It is not every table
// whose rows belong to one tenant: webhooks, sdk_tokens and security_policies,
// for example, each carry an organization_id column and are not in it.
var TenantScopedTables = []TenantScopedTable{
	{Table: "organizations", TenantColumn: "id", LintParamKeys: []string{"orgId"}},
	{Table: "users", TenantColumn: "organization_id", LintParamKeys: []string{"userId"}},
	{Table: "agents", TenantColumn: "organization_id", LintParamKeys: []string{"id", "agent_id", "agent-id", "agentId", "identifier"}},
	{Table: "api_keys", TenantColumn: "organization_id", LintParamKeys: []string{"id"}},
	{Table: "mcp_servers", TenantColumn: "organization_id", LintParamKeys: []string{"mcp_id"}},
	{Table: "audit_logs", TenantColumn: "organization_id", LintParamKeys: []string{"audit_id"}},
	{Table: "tags", TenantColumn: "organization_id", LintParamKeys: []string{"tagId"}},
	{Table: "agent_capabilities", AgentColumn: "agent_id", LintParamKeys: []string{"capabilityId"}},
	{Table: "mcp_attestations", AgentColumn: "agent_id", LintParamKeys: []string{"attestation_id"}},
	{Table: "a2a_peer_trust", AgentColumn: "agent_id", LintParamKeys: []string{"peer_id"}},
	{Table: "a2a_skills", AgentColumn: "agent_id", LintParamKeys: []string{"skillId"}},
}

// SetTenantIdentity declares orgID as the tenant for work running on e.
//
// local mirrors set_config's is_local argument: true confines the setting to the
// surrounding transaction, false to the session. Prefer local=true on anything
// drawn from a pool -- a session-scoped setting outlives the work that set it
// and is handed to whoever gets the connection next.
func SetTenantIdentity(ctx context.Context, e Execer, orgID uuid.UUID, local bool) error {
	if orgID == uuid.Nil {
		return ErrNoTenantIdentity
	}
	if _, err := e.ExecContext(ctx, `SELECT set_config($1, $2, $3)`, TenantGUC, orgID.String(), local); err != nil {
		return fmt.Errorf("set tenant identity %s: %w", orgID, err)
	}
	return nil
}

// SetServiceScope declares that work running on e crosses tenants, and why.
//
// reason is not logged by this function; it is required so the justification is
// written at the call site, where a reviewer reading that code can see it.
func SetServiceScope(ctx context.Context, e Execer, reason string, local bool) error {
	if reason == "" {
		return ErrNoServiceScopeReason
	}
	if _, err := e.ExecContext(ctx, `SELECT set_config($1, $2, $3)`, ServiceScopeGUC, serviceScopeOn, local); err != nil {
		return fmt.Errorf("enter service scope (%s): %w", reason, err)
	}
	return nil
}

// ClearScope removes both settings from e, returning it to the fail-closed
// state where it reads no tenant-scoped rows at all.
//
// Only needed for a connection whose settings were made session-scoped
// (local=false); a transaction-local setting is discarded at commit or rollback.
func ClearScope(ctx context.Context, e Execer, local bool) error {
	if _, err := e.ExecContext(ctx, `SELECT set_config($1, '', $2)`, TenantGUC, local); err != nil {
		return fmt.Errorf("clear tenant identity: %w", err)
	}
	if _, err := e.ExecContext(ctx, `SELECT set_config($1, '', $2)`, ServiceScopeGUC, local); err != nil {
		return fmt.Errorf("clear service scope: %w", err)
	}
	return nil
}

// WithTenant runs fn in a transaction scoped to orgID.
//
// The identity is transaction-local, so it cannot leak to the next user of the
// pooled connection: whether fn commits or rolls back, the setting goes with the
// transaction. A non-nil error from fn rolls back.
func WithTenant(ctx context.Context, db *sql.DB, orgID uuid.UUID, fn func(*sql.Tx) error) error {
	if orgID == uuid.Nil {
		return ErrNoTenantIdentity
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if err := SetTenantIdentity(ctx, tx, orgID, true); err != nil {
			return err
		}
		return fn(tx)
	})
}

// WithServiceScope runs fn in a transaction that is exempt from tenant
// isolation, for work that legitimately spans tenants.
//
// The legitimate callers are narrow and worth naming: migrations that backfill,
// platform-admin reads, and the pre-authentication lookups that resolve an email
// or a domain to an organization before any tenant is known. Anything else
// belongs in WithTenant.
func WithServiceScope(ctx context.Context, db *sql.DB, reason string, fn func(*sql.Tx) error) error {
	if reason == "" {
		return ErrNoServiceScopeReason
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if err := SetServiceScope(ctx, tx, reason, true); err != nil {
			return err
		}
		return fn(tx)
	})
}

// inTx runs fn inside a transaction, rolling back on error or panic.
func inTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tenant-scoped transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tenant-scoped transaction: %w", err)
	}
	committed = true
	return nil
}

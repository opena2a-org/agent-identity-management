package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AIM-08: contextRules.onUnavailable, fail closed by default.
//
// Step 3 (context check) used to fail OPEN twice over: an unparseable
// context_rules document was warned about and allowed, and a missing ASC risk
// summary skipped the rules entirely. These tests pin the AIM-08 ruling: an
// unevaluable rule is not satisfied. The decision when the summary cannot be
// evaluated follows contextRules.onUnavailable ("deny" | "allow"), with deny
// as the default for an absent key, an unknown value, or an unparseable
// document — and rules that evaluate normally are decided exactly as before.

// aim08ASCColumns matches the SELECT in fetchASCRiskSummary.
var aim08ASCColumns = []string{"overall_risk", "drift_score", "active_alerts", "coalesce", "scan_verdict"}

func aim08HealthyASCRow() *sqlmock.Rows {
	return sqlmock.NewRows(aim08ASCColumns).AddRow("low", 0.2, 0, 5, "clean")
}

// aim08ContextEngine builds an FGAEngine good enough for direct checkContext
// calls: a sqlmock DB (nil rows = the ASC summary query fails, i.e. the
// summary is unavailable) and a real logger. No tracer/meter needed on this
// path.
func aim08ContextEngine(t *testing.T, ascRows *sqlmock.Rows) *FGAEngine {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	if ascRows != nil {
		mock.ExpectQuery("FROM agent_security_contexts").WillReturnRows(ascRows)
	}
	return &FGAEngine{db: db, logger: slog.Default()}
}

func aim08Policy(contextRules string) *FGAPolicy {
	return &FGAPolicy{ContextRules: json.RawMessage(contextRules)}
}

func TestAIM08CheckContextOnUnavailable(t *testing.T) {
	req := &FGARequest{AgentID: uuid.New(), Capability: "file:read"}

	t.Run("AIM-08.AC1 absent onUnavailable denies when the summary is unavailable", func(t *testing.T) {
		engine := aim08ContextEngine(t, nil)
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{"maxDriftScore":0.5}`))
		assert.True(t, denied, "absent onUnavailable must default to deny")
		assert.Equal(t, contextUnavailableReason, reason)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusUnavailable, check.Status)
		assert.Equal(t, contextOnUnavailableDeny, check.OnUnavailable)
		assert.True(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 explicit deny denies when the summary is unavailable", func(t *testing.T) {
		engine := aim08ContextEngine(t, nil)
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{"maxDriftScore":0.5,"onUnavailable":"deny"}`))
		assert.True(t, denied)
		assert.Equal(t, contextUnavailableReason, reason)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusUnavailable, check.Status)
		assert.Equal(t, contextOnUnavailableDeny, check.OnUnavailable)
		assert.True(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 explicit allow allows when the summary is unavailable", func(t *testing.T) {
		engine := aim08ContextEngine(t, nil)
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{"maxDriftScore":0.5,"onUnavailable":"allow"}`))
		assert.False(t, denied, "an explicit allow opt-in must preserve the pre-AIM-08 fail-open")
		assert.Empty(t, reason)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusUnavailable, check.Status)
		assert.Equal(t, contextOnUnavailableAllow, check.OnUnavailable)
		assert.False(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 an unknown onUnavailable value denies when the summary is unavailable", func(t *testing.T) {
		engine := aim08ContextEngine(t, nil)
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{"onUnavailable":"skip"}`))
		assert.True(t, denied, "only 'deny' and 'allow' are admitted; anything else fails closed")
		assert.Equal(t, contextUnavailableReason, reason)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusInvalid, check.Status)
		assert.Equal(t, contextOnUnavailableDeny, check.OnUnavailable)
		assert.True(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 unparseable context rules deny", func(t *testing.T) {
		// Pre-AIM-08 this logged a warning and allowed, silently disabling
		// the context check for a malformed document.
		engine := aim08ContextEngine(t, nil)
		denied, _, check := engine.checkContext(context.Background(), req, aim08Policy(`{"onUnavailable":`))
		assert.True(t, denied)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusInvalid, check.Status)
		assert.Equal(t, contextOnUnavailableDeny, check.OnUnavailable)
		assert.True(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 a present summary evaluates the rules exactly as at base", func(t *testing.T) {
		// Passing rule: drift 0.2 under the 0.5 ceiling.
		engine := aim08ContextEngine(t, aim08HealthyASCRow())
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{"maxDriftScore":0.5}`))
		assert.False(t, denied)
		assert.Empty(t, reason)
		require.NotNil(t, check)
		assert.Equal(t, contextStatusEvaluated, check.Status)
		assert.Equal(t, contextOnUnavailableDeny, check.OnUnavailable)
		assert.False(t, check.Blocked)

		// Failing rule: drift 0.2 over a 0.1 ceiling — same deny and reason
		// shape as base, with the evaluated status attached.
		engine = aim08ContextEngine(t, aim08HealthyASCRow())
		denied, reason, check = engine.checkContext(context.Background(), req, aim08Policy(`{"maxDriftScore":0.1}`))
		assert.True(t, denied)
		assert.Contains(t, reason, "Drift score")
		require.NotNil(t, check)
		assert.Equal(t, contextStatusEvaluated, check.Status)
		assert.True(t, check.Blocked)
	})

	t.Run("AIM-08.AC1 empty rules still skip the step", func(t *testing.T) {
		engine := aim08ContextEngine(t, nil)
		denied, reason, check := engine.checkContext(context.Background(), req, aim08Policy(`{}`))
		assert.False(t, denied)
		assert.Empty(t, reason)
		assert.Nil(t, check, "empty rules must not produce a contextCheck result")

		denied, _, check = engine.checkContext(context.Background(), req, &FGAPolicy{})
		assert.False(t, denied)
		assert.Nil(t, check)
	})
}

// --- Authorize-level harness -------------------------------------------------

// aim08CapabilityRepo grants exactly one capability so Step 1 passes.
type aim08CapabilityRepo struct {
	domain.CapabilityRepository
	capability string
}

func (r *aim08CapabilityRepo) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	return []*domain.AgentCapability{{AgentID: agentID, CapabilityType: r.capability}}, nil
}

// aim08AgentRepo fails GetByID so the span-finalizer enrichment (which is
// fail-silent by design) stays out of the way.
type aim08AgentRepo struct {
	domain.AgentRepository
}

func (r *aim08AgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return nil, errors.New("aim08 test: agent enrichment not under test")
}

var aim08PolicyColumns = []string{
	"id", "agent_id", "capability",
	"allowed_attributes", "denied_attributes", "allowed_objects", "allowed_actions",
	"row_predicate", "context_rules", "chain_rules", "intent_check", "risk_level",
}

// aim08AuthorizeEngine wires a full engine over sqlmock: loadPolicy returns a
// LOW-risk policy carrying contextRules, and the ASC summary query either
// returns ascRows or (nil) fails, making the summary unavailable. Instruments
// bind to whatever meter provider is installed when this runs.
func aim08AuthorizeEngine(t *testing.T, contextRules string, ascRows *sqlmock.Rows) (*FGAEngine, *FGARequest) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	req := &FGARequest{AgentID: uuid.New(), Capability: "file:read", Resource: "/tmp/aim08"}
	mock.ExpectQuery("FROM fga_policies").WillReturnRows(
		sqlmock.NewRows(aim08PolicyColumns).AddRow(
			uuid.New().String(), req.AgentID.String(), req.Capability,
			"{}", "{}", "{}", "{}",
			[]byte(`{}`), []byte(contextRules), []byte(`{}`), []byte(`{}`), "LOW",
		),
	)
	if ascRows != nil {
		mock.ExpectQuery("FROM agent_security_contexts").WillReturnRows(ascRows)
	}
	// Every other DB touch (attestation/tool-call inserts, the finalizer's
	// second ASC read) is best-effort in the engine; sqlmock's "unexpected
	// call" errors exercise exactly those fail-silent paths.

	agentSvc := &AgentService{
		capabilityRepo: &aim08CapabilityRepo{capability: req.Capability},
		agentRepo:      &aim08AgentRepo{},
	}
	engine := NewFGAEngine(db, agentSvc, slog.Default())
	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })
	return engine, req
}

func TestAIM08AuthorizeContextCheckResult(t *testing.T) {
	t.Run("AIM-08.AC2 unavailable deny sets DENY_CONTEXT with the pinned reason", func(t *testing.T) {
		engine, req := aim08AuthorizeEngine(t, `{"maxDriftScore":0.5}`, nil)
		result, err := engine.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, "DENY_CONTEXT", result.Outcome)
		assert.Equal(t, "context_check", result.DeniedBy)
		assert.Equal(t, "ASC summary unavailable", result.DeniedReason)
		require.NotNil(t, result.ContextCheck)
		assert.Equal(t, contextStatusUnavailable, result.ContextCheck.Status)
		assert.Equal(t, contextOnUnavailableDeny, result.ContextCheck.OnUnavailable)
		assert.True(t, result.ContextCheck.Blocked)
	})

	t.Run("AIM-08.AC2 unavailable allow returns ALLOW with an unblocked contextCheck", func(t *testing.T) {
		engine, req := aim08AuthorizeEngine(t, `{"maxDriftScore":0.5,"onUnavailable":"allow"}`, nil)
		result, err := engine.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, "ALLOW", result.Outcome)
		require.NotNil(t, result.ContextCheck)
		assert.Equal(t, contextStatusUnavailable, result.ContextCheck.Status)
		assert.Equal(t, contextOnUnavailableAllow, result.ContextCheck.OnUnavailable)
		assert.False(t, result.ContextCheck.Blocked)
	})

	t.Run("AIM-08.AC2 contextCheck serialises under contextCheck and omits when nil", func(t *testing.T) {
		withCheck, err := json.Marshal(&FGAResult{
			Outcome: "DENY_CONTEXT",
			ContextCheck: &ContextCheckResult{
				Status:        contextStatusUnavailable,
				OnUnavailable: contextOnUnavailableDeny,
				Blocked:       true,
			},
		})
		require.NoError(t, err)
		assert.Contains(t, string(withCheck), `"contextCheck":{"status":"unavailable","onUnavailable":"deny","blocked":true}`)

		withoutCheck, err := json.Marshal(&FGAResult{Outcome: "ALLOW"})
		require.NoError(t, err)
		assert.NotContains(t, string(withoutCheck), "contextCheck")
	})
}

func TestAIM08ContextStatusMetric(t *testing.T) {
	// The exported-metrics pattern of fga_engine_step5_metrics_test.go: a
	// manual reader installed before the engine binds its instruments, then
	// collectCounter over the reader.
	install := func(t *testing.T) sdkmetric.Reader {
		t.Helper()
		reader := sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		prev := otel.GetMeterProvider()
		otel.SetMeterProvider(mp)
		t.Cleanup(func() { otel.SetMeterProvider(prev) })
		return reader
	}

	findStatus := func(t *testing.T, reader sdkmetric.Reader, status string) int64 {
		t.Helper()
		dps := collectCounter(t, reader, "fga.context_status")
		for i := range dps {
			if attrStr(dps[i].Attributes, "fga.context_status") == status {
				return dps[i].Value
			}
		}
		t.Fatalf("no fga.context_status datapoint with status %q", status)
		return 0
	}

	t.Run("AIM-08.AC2 the unavailable cell is measured as fga.context_status", func(t *testing.T) {
		reader := install(t)
		engine, req := aim08AuthorizeEngine(t, `{"maxDriftScore":0.5}`, nil)
		result, err := engine.Authorize(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, "DENY_CONTEXT", result.Outcome)
		assert.Equal(t, int64(1), findStatus(t, reader, contextStatusUnavailable))
	})

	t.Run("AIM-08.AC2 the evaluated cell is measured as fga.context_status", func(t *testing.T) {
		reader := install(t)
		engine, req := aim08AuthorizeEngine(t, `{"maxDriftScore":0.5}`, aim08HealthyASCRow())
		result, err := engine.Authorize(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, "ALLOW", result.Outcome)
		assert.Equal(t, int64(1), findStatus(t, reader, contextStatusEvaluated))
	})
}

func TestAIM08FailClosedCannotBeWidenedByData(t *testing.T) {
	// AC4: no value an operator (or attacker with policy-write access) can
	// place in context_rules turns an unavailable summary into an allow,
	// other than the one admitted opt-in ("allow"). Every malformed shape is
	// DENY_CONTEXT with contextCheck.status "invalid".
	cases := []struct {
		name  string
		rules string
	}{
		{"AIM-08.AC4 unknown value skip denies as invalid", `{"onUnavailable":"skip"}`},
		{"AIM-08.AC4 case-mangled ALLOW denies as invalid", `{"onUnavailable":"ALLOW"}`},
		{"AIM-08.AC4 boolean true denies as invalid", `{"onUnavailable":true}`},
		{"AIM-08.AC4 truncated document denies as invalid", `{"onUnavailable":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine, req := aim08AuthorizeEngine(t, tc.rules, nil)
			result, err := engine.Authorize(context.Background(), req)
			require.NoError(t, err)
			assert.False(t, result.Allowed, "rules %s must never widen the fail-closed default", tc.rules)
			assert.Equal(t, "DENY_CONTEXT", result.Outcome)
			assert.Equal(t, "context_check", result.DeniedBy)
			require.NotNil(t, result.ContextCheck)
			assert.Equal(t, contextStatusInvalid, result.ContextCheck.Status)
			assert.Equal(t, contextOnUnavailableDeny, result.ContextCheck.OnUnavailable)
			assert.True(t, result.ContextCheck.Blocked)
		})
	}

	t.Run("AIM-08.AC4 a base-passing evaluated policy still passes", func(t *testing.T) {
		engine, req := aim08AuthorizeEngine(t, `{"maxDriftScore":0.5,"requireScanClean":true}`, aim08HealthyASCRow())
		result, err := engine.Authorize(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "an evaluated decision must not change")
		assert.Equal(t, "ALLOW", result.Outcome)
		require.NotNil(t, result.ContextCheck)
		assert.Equal(t, contextStatusEvaluated, result.ContextCheck.Status)
		assert.False(t, result.ContextCheck.Blocked)
	})
}

func TestAIM08Migration110(t *testing.T) {
	root := aim02RepoRoot(t)
	migrationsDir := filepath.Join(root, "apps", "backend", "migrations")

	t.Run("AIM-08.AC3 migration 110 carries the CHECK and documents the camelCase keys", func(t *testing.T) {
		sql := aim08ReadFile(t, filepath.Join(migrationsDir, "110_fga_context_rules_on_unavailable.sql"))
		assert.Contains(t, sql,
			"CHECK (context_rules->>'onUnavailable' IS NULL OR context_rules->>'onUnavailable' IN ('deny','allow'))",
			"the constraint must admit exactly NULL, 'deny' and 'allow'")
		assert.Contains(t, sql, "fga_policies", "the constraint must target fga_policies")
		for _, key := range []string{"maxDriftScore", "requireScanClean", "maxAlerts", "minTrustLevel", "onUnavailable"} {
			assert.Contains(t, sql, key,
				"the header comment must document the camelCase key %q the engine reads", key)
		}
	})

	t.Run("AIM-08.AC3 110 is the only migration this change adds and 109 stays free for aim-cloud", func(t *testing.T) {
		entries, err := os.ReadDir(migrationsDir)
		require.NoError(t, err)
		numRe := regexp.MustCompile(`^(\d+)_`)
		highest := 0
		count110 := 0
		for _, entry := range entries {
			m := numRe.FindStringSubmatch(entry.Name())
			if m == nil {
				continue
			}
			n, err := strconv.Atoi(m[1])
			require.NoError(t, err)
			if n > highest {
				highest = n
			}
			switch n {
			case 109:
				t.Errorf("migration %s occupies 109, which belongs to aim-cloud's row-level-security migration", entry.Name())
			case 110:
				count110++
			}
		}
		assert.Equal(t, 110, highest, "110 must be the highest migration in this tree")
		assert.Equal(t, 1, count110, "exactly one migration file may carry number 110")
	})
}

func aim08ReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err, "expected %s to exist", path)
	return string(b)
}

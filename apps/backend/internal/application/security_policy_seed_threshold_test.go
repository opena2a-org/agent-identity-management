package application

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A trust_score_low policy is evaluated against rules["trust_threshold"] on
// the trust score's 0-1 scale. Migration 015 seeded {"threshold": 50} for
// 'Critical Trust Score Block' and {"threshold": 70} for 'Low Trust Score
// Alert': the evaluator never read either value and applied 0.3 to both, and
// an operator who edited the seeded value changed nothing. The seed script
// and the policy backfill wrote {"trust_threshold": 70.0}, a percent compared
// to a 0-1 score. These tests read every place that seeds a trust_score_low
// policy and fail when its rules put a threshold anywhere the evaluator does
// not read it.

var (
	sqlJSONLiteral   = regexp.MustCompile(`'(\{[^']*\})'`)
	goTrustScoreLow  = regexp.MustCompile("PolicyType:\\s*\"trust_score_low\",[^`]*?Rules:\\s*`([^`]*)`")
	sqlInsertKeyword = regexp.MustCompile(`(?i)INSERT\s+INTO\s+security_policies`)
)

// trustScoreLowRulesInSQL returns the rules of every security_policies INSERT
// in sql whose policy_type is 'trust_score_low'.
func trustScoreLowRulesInSQL(t *testing.T, sql string) []map[string]interface{} {
	t.Helper()
	var found []map[string]interface{}
	for _, stmt := range sqlInsertKeyword.Split(sql, -1)[1:] {
		if !strings.Contains(stmt, "'trust_score_low'") {
			continue
		}
		literals := sqlJSONLiteral.FindAllStringSubmatch(stmt, -1)
		require.Len(t, literals, 1, "a trust_score_low INSERT carries exactly one JSON rules literal:\n%s", stmt)
		found = append(found, decodeRules(t, literals[0][1]))
	}
	return found
}

// trustScoreLowRulesInGo returns the Rules literal of every trust_score_low
// entry in a Go source that declares policies as struct literals.
func trustScoreLowRulesInGo(t *testing.T, src string) []map[string]interface{} {
	t.Helper()
	var found []map[string]interface{}
	for _, m := range goTrustScoreLow.FindAllStringSubmatch(src, -1) {
		found = append(found, decodeRules(t, m[1]))
	}
	return found
}

func decodeRules(t *testing.T, literal string) map[string]interface{} {
	t.Helper()
	var rules map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(literal), &rules), "rules literal must be a JSON object: %s", literal)
	return rules
}

// assertEvaluatorReadsThreshold fails when rules carry their threshold under a
// key EvaluateTrustScoreLow does not read, or off the 0-1 scale.
func assertEvaluatorReadsThreshold(t *testing.T, rules map[string]interface{}) {
	t.Helper()
	for key := range rules {
		if key != TrustThresholdRuleKey && strings.Contains(strings.ToLower(key), "threshold") {
			t.Errorf("rules %v carry a threshold under %q; the evaluator reads only %q", rules, key, TrustThresholdRuleKey)
		}
	}
	value, ok := rules[TrustThresholdRuleKey].(float64)
	if !assert.True(t, ok, "rules %v carry no numeric %q; the evaluator would apply %.2f", rules, TrustThresholdRuleKey, DefaultTrustScorePolicyThreshold) {
		return
	}
	assert.True(t, value >= 0 && value <= 1,
		"rules %v put %q at %v; the trust score it is compared to runs 0-1", rules, TrustThresholdRuleKey, value)
	assert.Equal(t, value, trustScorePolicyThreshold(rules), "the evaluator must apply the seeded value")
}

func TestSeededTrustScoreLowPoliciesCarryTheThresholdTheEvaluatorReads(t *testing.T) {
	backend := filepath.Join(repoRoot(t), "apps", "backend")

	sources := []struct {
		name    string
		path    string
		extract func(*testing.T, string) []map[string]interface{}
		count   int
	}{
		{"migration 015", "migrations/015_add_default_security_policies.sql", trustScoreLowRulesInSQL, 2},
		{"seed script", "seed/default_security_policies.sql", trustScoreLowRulesInSQL, 1},
		{"policy backfill", "cmd/backfill_policies/main.go", trustScoreLowRulesInGo, 1},
	}
	for _, src := range sources {
		t.Run(src.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(backend, filepath.FromSlash(src.path)))
			require.NoError(t, err)
			all := src.extract(t, string(content))
			require.Len(t, all, src.count, "trust_score_low policies seeded by %s", src.path)
			for _, rules := range all {
				assertEvaluatorReadsThreshold(t, rules)
			}
		})
	}

	t.Run("CreateDefaultPolicies", func(t *testing.T) {
		repo := new(MockSecurityPolicyRepository)
		var created []*domain.SecurityPolicy
		repo.On("Create", mock.Anything).Run(func(args mock.Arguments) {
			created = append(created, args.Get(0).(*domain.SecurityPolicy))
		}).Return(nil)
		service := NewSecurityPolicyService(repo, nil, nil)

		require.NoError(t, service.CreateDefaultPolicies(context.Background(), uuid.New(), uuid.New()))
		count := 0
		for _, policy := range created {
			if policy.PolicyType == domain.PolicyTypeTrustScoreLow {
				count++
				assertEvaluatorReadsThreshold(t, policy.Rules)
			}
		}
		assert.Equal(t, 1, count, "trust_score_low policies created for a new organization")
	})
}

// seededCriticalTrustScoreBlock returns the 'Critical Trust Score Block'
// policy as migration 015 writes it.
func seededCriticalTrustScoreBlock(t *testing.T) *domain.SecurityPolicy {
	t.Helper()
	path := filepath.Join(repoRoot(t), "apps", "backend", "migrations", "015_add_default_security_policies.sql")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, stmt := range sqlInsertKeyword.Split(string(content), -1)[1:] {
		if !strings.Contains(stmt, "'Critical Trust Score Block'") {
			continue
		}
		literals := sqlJSONLiteral.FindAllStringSubmatch(stmt, -1)
		require.Len(t, literals, 1)
		return &domain.SecurityPolicy{
			ID:                uuid.New(),
			Name:              "Critical Trust Score Block",
			PolicyType:        domain.PolicyTypeTrustScoreLow,
			EnforcementAction: domain.EnforcementBlockAndAlert,
			AppliesTo:         "all",
			IsEnabled:         true,
			Rules:             decodeRules(t, literals[0][1]),
		}
	}
	t.Fatalf("migration 015 seeds no 'Critical Trust Score Block' policy")
	return nil
}

// The seeded block and the update-time suspension describe one critical
// score. They are two rules, so the test keeps their values equal.
func TestSeededCriticalTrustScoreBlockMatchesTheUpdateTimeSuspension(t *testing.T) {
	policy := seededCriticalTrustScoreBlock(t)
	assert.Equal(t, TrustScoreThresholdCritical, trustScorePolicyThreshold(policy.Rules),
		"the seeded block threshold and the suspension threshold must agree")
}

// An evaluable agent at 0.40 is below the seeded 0.50 and is blocked. Before
// the seed carried trust_threshold, the evaluator applied 0.3 and allowed it.
func TestSeededCriticalTrustScoreBlockBlocksBelowItsSeededThreshold(t *testing.T) {
	repo := new(MockSecurityPolicyRepository)
	service := NewSecurityPolicyService(repo, nil, nil)
	agent := neverAllowedAgent()
	agent.TrustScore = 0.40
	service.SetVerificationEventRepo(statsMock(agent.ID, 10, 6))
	repo.On("GetByType", agent.OrganizationID, domain.PolicyTypeTrustScoreLow).
		Return([]*domain.SecurityPolicy{seededCriticalTrustScoreBlock(t)}, nil)

	blocked, alert, name, err := service.EvaluateTrustScoreLow(context.Background(), agent, "db:read", "", uuid.New())
	require.NoError(t, err)
	assert.True(t, blocked, "0.40 is below the seeded 0.50")
	assert.True(t, alert)
	assert.Equal(t, "Critical Trust Score Block", name)
}

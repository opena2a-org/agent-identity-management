package application

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The guides quote trust scores and factor weights. Each score a page quotes is
// computed here by the calculator, in the state the page describes, and looked
// up in that page, so a change to a weight or a factor that moves a quoted
// figure fails next to the page that would then be wrong. The guides kept the
// 8-factor weights, and a new pending agent at 68%, after the ninth factor was
// added (#675).

// docsTrustFactors are the nine factors in the order and with the weights
// Calculate uses: the display name the pages use, the key the SDK guides'
// breakdowns use, and the weight in percent.
var docsTrustFactors = []struct {
	name   string
	key    string
	weight int
}{
	{"Verification Status", "verification_status", 25},
	{"Uptime & Availability", "uptime", 15},
	{"Action Success Rate", "success_rate", 15},
	{"Security Alerts", "security_alerts", 15},
	{"Compliance Score", "compliance", 10},
	{"Age & History", "age", 5},
	{"Drift Detection", "drift_detection", 3},
	{"User Feedback", "user_feedback", 2},
	{"Execution Isolation", "execution_isolation", 10},
}

// docsFactorValues returns the nine factor values of score in docsTrustFactors
// order. An excluded factor carries its neutral placeholder.
func docsFactorValues(score *domain.TrustScore) []float64 {
	f := score.Factors
	return []float64{
		f.VerificationStatus, f.Uptime, f.SuccessRate, f.SecurityAlerts, f.Compliance,
		f.Age, f.DriftDetection, f.UserFeedback, f.ExecutionIsolation,
	}
}

// docsDecimal formats v with at most four decimals and no trailing zeros.
func docsDecimal(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
}

// docsLine returns the one line of page that contains needle.
func docsLine(t *testing.T, page, rel, needle string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(page, "\n") {
		if strings.Contains(line, needle) {
			found = append(found, line)
		}
	}
	require.Len(t, found, 1, "%s must have exactly one line containing %q", rel, needle)
	return found[0]
}

// measuredAgentCalculator returns a calculator and a verified agent of the
// given age for which every factor has data: 98% of its verifications in the
// last 30 days succeeded, the latest an hour ago; its organization has an AIM
// compliance snapshot of 100; it has more than ten positive ratings; and it
// reported an isolation posture that is not verified.
func measuredAgentCalculator(t *testing.T, age time.Duration) (*TrustCalculator, *domain.Agent) {
	t.Helper()
	calculator, agent := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{
		TotalVerifications: 50, SuccessCount: 49, FailedCount: 1, SuccessRate: 0.98,
		LastVerification: time.Now().Add(-time.Hour),
	})
	agent.CreatedAt = time.Now().Add(-age)
	calculator.SetSnapshotRepo(&stubSnapshotRepo{snapshot: &domain.ComplianceSnapshot{Score: 100}})
	feedbackRepo := new(MockUserFeedbackRepository)
	feedbackRepo.On("GetStats", mock.Anything).Return(&domain.UserFeedbackStats{Total: 12, PositiveCount: 11, NegativeCount: 1}, nil)
	calculator.SetUserFeedbackRepo(feedbackRepo)
	isolationRepo := new(MockIsolationAttestationRepository)
	isolationRepo.On("GetLatest", mock.Anything).Return(&domain.IsolationAttestation{
		ID: uuid.New(), AgentID: agent.ID, Score: 0.80, ReportedAt: time.Now(), Verified: false,
	}, nil)
	calculator.SetIsolationRepo(isolationRepo)
	return calculator, agent
}

func TestTrustScoreDocs_PagesQuoteTheNineFactorScore(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := repoRoot(t)

	t.Run("no page describes an 8-factor score", func(t *testing.T) {
		eightFactor := regexp.MustCompile(`(?i)\b(8|eight)[- ]factors?\b`)
		for _, rel := range []string{
			"docs/quick-start.md",
			"docs/sdk/python.md",
			"docs/sdk/java.md",
			"docs/sdk/authentication.md",
			"docs/sdk/auto-detection.md",
			"docs/DEMO_SCRIPT_ECHOLEAK.md",
			"docs/DOCUMENTATION_INDEX.md",
			"docs/examples/weather-agent.md",
		} {
			page := aim02ReadFile(t, root, rel)
			assert.Empty(t, eightFactor.FindAllString(page, -1), "%s describes an 8-factor trust score", rel)
		}
	})

	t.Run("the demo script quotes a new agent's starting score", func(t *testing.T) {
		const rel = "docs/DEMO_SCRIPT_ECHOLEAK.md"
		page := aim02ReadFile(t, root, rel)

		// A wired verification repository with no rows, as for an agent that
		// has just registered.
		calculator, agent := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{})
		agent.Status = domain.AgentStatusPending
		pending, err := calculator.Calculate(agent)
		require.NoError(t, err)
		require.Equal(t, []string{"compliance", "user_feedback"}, pending.ExcludedFactors)
		pct := fmt.Sprintf("%.0f%%", pending.Score*100)

		values := docsFactorValues(pending)
		contributions := make([]string, len(values))
		var imputed float64
		for i, f := range docsTrustFactors {
			c := values[i] * float64(f.weight) / 100
			imputed += c
			contributions[i] = docsDecimal(c)
			// The factor table under "Why ... Instead of 100%?" and the
			// algorithm reference table each carry the factor and its weight.
			row := fmt.Sprintf("| %s | %d%% |", f.name, f.weight)
			assert.Equal(t, 2, strings.Count(page, row), "%s must carry the row %q in both factor tables", rel, row)
			assert.True(t, strings.HasSuffix(docsLine(t, page, rel, row+" "+docsPercentPrefix(values[i], f.key)), " "+contributions[i]+" |"),
				"%s: the %s row must end with its contribution %s", rel, f.name, contributions[i])
		}
		// The anti-gaming cap binds for a new agent: the published score is
		// the sum with the excluded factors at their neutral placeholders.
		require.InDelta(t, imputed, pending.Score, 1e-9)

		for _, want := range []string{
			fmt.Sprintf("Trust Score: %s (9-factor algorithm)", pct),
			fmt.Sprintf("Why %s Instead of 100%%?", pct),
			fmt.Sprintf("**Calculation:** %s = **%s ≈ %s**", strings.Join(contributions, " + "), docsDecimal(pending.Score), pct),
			fmt.Sprintf("* **Trust Score:** ~%s", pct),
			fmt.Sprintf("* **Started:** %s (new pending agent)", pct),
			fmt.Sprintf("- **New pending agent:** ~%s", pct),
			fmt.Sprintf("(Trust Score **%s**, status **pending**)", pct),
			fmt.Sprintf("resulting in ~%s initial trust", pct),
		} {
			assert.Contains(t, page, want, "%s must quote the starting score %s", rel, pct)
		}
		assert.NotContains(t, page, "68%", "%s still quotes the 8-factor starting score", rel)

		calculator, agent = neverAllowedCalculator(t, &domain.AgentVerificationStatistics{})
		verified, err := calculator.Calculate(agent)
		require.NoError(t, err)
		assert.Contains(t, page, fmt.Sprintf("- **New verified agent (no verifications recorded yet):** ~%.0f%%", verified.Score*100))
	})

	t.Run("the quick-start and weather example quote a verified agent's first-day score", func(t *testing.T) {
		calculator, agent := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{
			TotalVerifications: 2, SuccessCount: 2, SuccessRate: 1.0,
			LastVerification: time.Now().Add(-time.Minute),
		})
		score, err := calculator.Calculate(agent)
		require.NoError(t, err)
		require.Equal(t, []string{"compliance", "user_feedback"}, score.ExcludedFactors)

		values := docsFactorValues(score)
		var includedSum float64
		var includedWeight int
		for i, f := range docsTrustFactors {
			if f.key == "compliance" || f.key == "user_feedback" {
				continue
			}
			includedSum += values[i] * float64(f.weight) / 100
			includedWeight += f.weight
		}
		renormalized := includedSum / (float64(includedWeight) / 100)

		pending, err := func() (*domain.TrustScore, error) {
			c, a := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{})
			a.Status = domain.AgentStatusPending
			return c.Calculate(a)
		}()
		require.NoError(t, err)

		for _, page := range []struct {
			rel    string
			weight string
		}{
			{"docs/quick-start.md", "[%2d%%]"},
			{"docs/examples/weather-agent.md", "[Weight: %2d%%]"},
		} {
			text := aim02ReadFile(t, root, page.rel)
			assert.Contains(t, text, fmt.Sprintf("Trust Score: %.3f (Good)", score.Score), page.rel)
			assert.Contains(t, text, fmt.Sprintf("Overall Trust Score: %.3f / 1.00", score.Score), page.rel)
			assert.Contains(t, text, fmt.Sprintf("still pending starts at %s.", docsDecimal(pending.Score)), page.rel)
			for i, f := range docsTrustFactors {
				line := docsLine(t, text, page.rel, f.name+":")
				assert.True(t, strings.HasSuffix(strings.TrimSpace(line), fmt.Sprintf(page.weight, f.weight)),
					"%s: the %s line must end with weight %d%%: %q", page.rel, f.name, f.weight, line)
				if f.key == "compliance" || f.key == "user_feedback" {
					assert.Contains(t, line, "(excluded)", "%s: %s has no data and is excluded", page.rel, f.name)
				} else {
					assert.Contains(t, line, fmt.Sprintf("(%.2f)", values[i]), "%s: %s value", page.rel, f.name)
				}
			}
		}
		quickStart := aim02ReadFile(t, root, "docs/quick-start.md")
		assert.Contains(t, quickStart, fmt.Sprintf("seven\nfactors (%.3f)", renormalized))
	})

	t.Run("the SDK guides' breakdowns carry the nine factors and their overall", func(t *testing.T) {
		for _, page := range []struct {
			rel     string
			age     time.Duration
			overall []string
			factor  string
		}{
			{"docs/sdk/python.md", 20 * 24 * time.Hour,
				[]string{"#   \"overall\": %.2f,", "print(f\"Trust Score: {score}\")  # %.2f\n"}, "#     %q: %.2f"},
			{"docs/sdk/trust-scoring.md", 45 * 24 * time.Hour,
				[]string{"Overall Trust Score: %.2f\n"}, "  %s: %.2f\n"},
		} {
			calculator, agent := measuredAgentCalculator(t, page.age)
			score, err := calculator.Calculate(agent)
			require.NoError(t, err)
			require.Empty(t, score.ExcludedFactors, "every factor has data in this state")

			text := aim02ReadFile(t, root, page.rel)
			for _, overall := range page.overall {
				assert.Contains(t, text, fmt.Sprintf(overall, score.Score), page.rel)
			}
			for i, f := range docsTrustFactors {
				assert.Contains(t, text, fmt.Sprintf(page.factor, f.key, docsFactorValues(score)[i]),
					"%s must list %s in its breakdown", page.rel, f.key)
			}
		}

		// The report example in the trust scoring guide weighs each factor.
		guide := aim02ReadFile(t, root, "docs/sdk/trust-scoring.md")
		for _, f := range docsTrustFactors {
			assert.Contains(t, guide, fmt.Sprintf("'%s': %.2f", f.key, float64(f.weight)/100),
				"the weight table in docs/sdk/trust-scoring.md must weigh %s at %d%%", f.key, f.weight)
		}
	})
}

// docsPercentPrefix is the start of the demo script's "New Pending Agent Score"
// cell for a factor: its value in percent, or the note for an excluded factor.
func docsPercentPrefix(value float64, key string) string {
	if key == "compliance" || key == "user_feedback" {
		return "no "
	}
	return fmt.Sprintf("%.0f%% ", value*100)
}

package application

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// excludedFactors names the factors with no data. Calculate leaves them out of
// its weighted mean and caps the result at the 9-factor sum in which each of
// them counts at its placeholder, so when the cap applies they count in the
// published score (#792). The texts that describe the field say so, and none
// says the listed factors are left out of the score.
func TestTrustScoreDocs_ListedFactorsCountThroughTheCap(t *testing.T) {
	requirePublicRepositoryFiles(t)
	root := repoRoot(t)

	// A verified agent under seven days old whose two verifications succeeded:
	// its organization has no compliance snapshot, and it has no user feedback
	// and no isolation attestation.
	calculator, agent := neverAllowedCalculator(t, &domain.AgentVerificationStatistics{
		TotalVerifications: 2, SuccessCount: 2, SuccessRate: 1.0,
		LastVerification: time.Now().Add(-time.Minute),
	})
	score, err := calculator.Calculate(agent)
	require.NoError(t, err)
	require.Equal(t, []string{"compliance", "user_feedback"}, score.ExcludedFactors)

	values := docsFactorValues(score)
	var sum, measuredSum, measuredWeight float64
	for i, f := range docsTrustFactors {
		weight := float64(f.weight) / 100
		sum += values[i] * weight
		if f.key == "compliance" || f.key == "user_feedback" {
			require.Equal(t, 0.5, values[i], "%s carries its placeholder", f.key)
			continue
		}
		measuredSum += values[i] * weight
		measuredWeight += weight
	}
	mean := measuredSum / measuredWeight
	require.Greater(t, mean, sum, "the cap applies in this state")
	require.InDelta(t, sum, score.Score, 1e-9, "the published score counts the listed factors at their placeholders")

	guide := aim02ReadFile(t, root, "docs/sdk/trust-scoring.md")
	assert.Contains(t, guide, fmt.Sprintf("the mean of the seven measured factors is %.3f\n", mean))
	assert.Contains(t, guide, fmt.Sprintf("the 9-factor sum with compliance and user feedback at 0.5 is %.3f.\n", sum))
	assert.Contains(t, guide, fmt.Sprintf("The published score is %.3f: the two listed factors count in it at 0.5.\n", score.Score))

	for _, rel := range []string{
		"docs/sdk/trust-scoring.md",
		"docs/API.md",
		"sdk/python/aim_sdk/client.py",
	} {
		text := strings.ToLower(aim02ReadFile(t, root, rel))
		for _, claim := range []string{
			"excluded from the composite",
			"excluded from the score",
			"never contributes a fabricated neutral value",
		} {
			assert.NotContains(t, text, claim, "%s says the factors excludedFactors lists are left out of the score", rel)
		}
	}
}

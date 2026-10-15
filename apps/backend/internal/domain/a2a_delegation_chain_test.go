package domain

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var delegationTestNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// delegationChainFixture is a chain of n delegations through n+1 verified
// agents, each delegation carrying the column defaults and letting its
// delegate delegate further.
func delegationChainFixture(n int, rootTrust float64) ([]*A2ADelegation, map[uuid.UUID]*Agent) {
	agents := map[uuid.UUID]*Agent{}
	ids := make([]uuid.UUID, n+1)
	for i := range ids {
		ids[i] = uuid.New()
		agents[ids[i]] = &Agent{ID: ids[i], Status: AgentStatusVerified, TrustScore: 0.5}
	}
	agents[ids[0]].TrustScore = rootTrust
	chain := make([]*A2ADelegation, n)
	for i := range chain {
		chain[i] = &A2ADelegation{
			ID:                     uuid.New(),
			DelegatorAgentID:       ids[i],
			DelegateAgentID:        ids[i+1],
			MaxChainDepth:          MaxA2ADelegationChainLength,
			TrustAttenuationFactor: DefaultA2ADelegationTrustAttenuationFactor,
			MinDelegatedTrustScore: DefaultA2ADelegationMinTrustScore,
			CanDelegateFurther:     true,
			IsActive:               true,
			ValidFrom:              delegationTestNow.Add(-time.Hour),
		}
	}
	return chain, agents
}

func hopTrust(v *A2ADelegationChainVerification) []float64 {
	out := make([]float64, len(v.Hops))
	for i, h := range v.Hops {
		out[i] = h.EffectiveTrust
	}
	return out
}

func hopErrors(v *A2ADelegationChainVerification) []string {
	out := make([]string, len(v.Hops))
	for i, h := range v.Hops {
		out[i] = h.Error
	}
	return out
}

func TestVerifyA2ADelegationChain_EveryDelegationMultipliesTheRootDelegatorsTrust(t *testing.T) {
	chain, agents := delegationChainFixture(3, 0.9)

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.True(t, v.Valid)
	assert.Equal(t, chain[0].DelegatorAgentID, v.RootAgentID)
	assert.Equal(t, 0.9, v.RootTrustScore)
	assert.Equal(t, []float64{0.72, 0.576, 0.4608}, hopTrust(v))
	assert.Equal(t, 0.4608, v.EffectiveTrust)
	assert.Equal(t, []string{"", "", ""}, hopErrors(v))
	for i, h := range v.Hops {
		assert.Equal(t, i, h.Index)
		assert.Equal(t, chain[i].ID, h.DelegationID)
		assert.Equal(t, 0.8, h.TrustAttenuationFactor)
		assert.Equal(t, 0.3, h.TrustFloor)
	}
}

func TestVerifyA2ADelegationChain_EachDelegationAppliesItsOwnFactor(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	chain[0].TrustAttenuationFactor = 0.9
	chain[1].TrustAttenuationFactor = 0.5

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.True(t, v.Valid)
	assert.Equal(t, []float64{0.9, 0.45}, hopTrust(v))
}

func TestVerifyA2ADelegationChain_ADelegationBelowTheFloorFails(t *testing.T) {
	chain, agents := delegationChainFixture(3, 0.5)

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, []float64{0.4, 0.32, 0.256}, hopTrust(v))
	assert.Equal(t, []string{"", "", "effective trust 0.256 is below the trust floor 0.3"}, hopErrors(v))
}

func TestVerifyA2ADelegationChain_ALaterDelegationCannotLowerTheFloor(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	chain[0].MinDelegatedTrustScore = 0.5
	chain[1].MinDelegatedTrustScore = 0.1
	chain[1].TrustAttenuationFactor = 0.6

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, []float64{0.8, 0.48}, hopTrust(v))
	assert.Equal(t, 0.5, v.Hops[1].TrustFloor)
	assert.Equal(t, "effective trust 0.48 is below the trust floor 0.5", v.Hops[1].Error)
}

func TestVerifyA2ADelegationChain_AFactorOutsideZeroToOneFailsAndZeroesTheTrust(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	chain[0].TrustAttenuationFactor = 1.5

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, []float64{0, 0}, hopTrust(v))
	assert.Equal(t, "trustAttenuationFactor 1.5 is outside 0 to 1", v.Hops[0].Error)
	assert.Equal(t, "effective trust 0 is below the trust floor 0.3", v.Hops[1].Error)
}

func TestVerifyA2ADelegationChain_AFloorOutsideZeroToOneFails(t *testing.T) {
	chain, agents := delegationChainFixture(1, 1.0)
	chain[0].MinDelegatedTrustScore = -0.1

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, "minDelegatedTrustScore -0.1 is outside 0 to 1", v.Hops[0].Error)
}

func TestVerifyA2ADelegationChain_ValidityAndRevocation(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(d *A2ADelegation)
		want   string
	}{
		"revoked": {
			mutate: func(d *A2ADelegation) { d.IsActive = false },
			want:   "the delegation is revoked",
		},
		"not yet valid": {
			mutate: func(d *A2ADelegation) { d.ValidFrom = delegationTestNow.Add(time.Hour) },
			want:   "the delegation is not valid before 2026-10-15T13:00:00Z",
		},
		"expired": {
			mutate: func(d *A2ADelegation) { until := delegationTestNow; d.ValidUntil = &until },
			want:   "the delegation expired at 2026-10-15T12:00:00Z",
		},
	} {
		t.Run(name, func(t *testing.T) {
			chain, agents := delegationChainFixture(2, 1.0)
			tc.mutate(chain[1])

			v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

			assert.False(t, v.Valid)
			assert.Equal(t, []string{"", tc.want}, hopErrors(v))
		})
	}
}

func TestVerifyA2ADelegationChain_ABrokenChainFails(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	stranger := uuid.New()
	agents[stranger] = &Agent{ID: stranger, Status: AgentStatusVerified}
	chain[1].DelegatorAgentID = stranger

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, "the chain is broken: the delegator is not the previous delegation's delegate", v.Hops[1].Error)
}

func TestVerifyA2ADelegationChain_ADelegateThatMayNotDelegateFurtherEndsTheChain(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	chain[0].CanDelegateFurther = false

	v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Equal(t, []string{"", "the previous delegation does not let its delegate delegate further"}, hopErrors(v))
}

func TestVerifyA2ADelegationChain_MaxChainDepthCountsFromEachDelegation(t *testing.T) {
	t.Run("the root's depth", func(t *testing.T) {
		chain, agents := delegationChainFixture(3, 1.0)
		chain[0].MaxChainDepth = 2

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, []string{"", "", "delegation 0 has maxChainDepth 2, and this is delegation 3 counted from it"}, hopErrors(v))
	})
	t.Run("a later delegation's depth", func(t *testing.T) {
		chain, agents := delegationChainFixture(3, 1.0)
		chain[1].MaxChainDepth = 1

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, []string{"", "", "delegation 1 has maxChainDepth 1, and this is delegation 2 counted from it"}, hopErrors(v))
	})
	t.Run("a depth of zero", func(t *testing.T) {
		chain, agents := delegationChainFixture(1, 1.0)
		chain[0].MaxChainDepth = 0

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, "delegation 0 has maxChainDepth 0, and this is delegation 1 counted from it", v.Hops[0].Error)
	})
}

func TestVerifyA2ADelegationChain_EveryAgentMustBeInTheOrganizationAndPermitted(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		chain, agents := delegationChainFixture(2, 1.0)
		delete(agents, chain[1].DelegateAgentID)

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, "agent "+chain[1].DelegateAgentID.String()+" is not in this organization", v.Hops[1].Error)
	})
	t.Run("suspended", func(t *testing.T) {
		chain, agents := delegationChainFixture(2, 1.0)
		agents[chain[0].DelegateAgentID].Status = AgentStatusSuspended

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		want := "agent " + chain[0].DelegateAgentID.String() + " is suspended"
		assert.Equal(t, []string{want, want}, hopErrors(v))
	})
	t.Run("the root delegator missing", func(t *testing.T) {
		chain, agents := delegationChainFixture(1, 1.0)
		delete(agents, chain[0].DelegatorAgentID)

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, 0.0, v.RootTrustScore)
		assert.Equal(t, []float64{0}, hopTrust(v))
		assert.Equal(t, "agent "+chain[0].DelegatorAgentID.String()+" is not in this organization", v.Hops[0].Error)
	})
	t.Run("the root delegator's trust outside 0 to 1", func(t *testing.T) {
		chain, agents := delegationChainFixture(1, 1.2)

		v := VerifyA2ADelegationChain(chain, agents, "", delegationTestNow)

		assert.False(t, v.Valid)
		assert.Equal(t, 1.2, v.RootTrustScore)
		assert.Equal(t, []float64{0}, hopTrust(v))
		assert.Equal(t, "the root delegator's trust score 1.2 is outside 0 to 1", v.Hops[0].Error)
	})
}

func TestVerifyA2ADelegationChain_ASkillMustBeCoveredByEveryDelegation(t *testing.T) {
	chain, agents := delegationChainFixture(2, 1.0)
	chain[0].SkillIDs = []string{"search", "summarize"}
	chain[1].SkillIDs = []string{"search"}

	covered := VerifyA2ADelegationChain(chain, agents, "search", delegationTestNow)
	assert.True(t, covered.Valid)
	assert.Equal(t, "search", covered.SkillID)

	uncovered := VerifyA2ADelegationChain(chain, agents, "summarize", delegationTestNow)
	assert.False(t, uncovered.Valid)
	assert.Equal(t, []string{"", `the delegation does not cover skill "summarize"`}, hopErrors(uncovered))

	chain[1].SkillIDs = nil
	everySkill := VerifyA2ADelegationChain(chain, agents, "summarize", delegationTestNow)
	assert.True(t, everySkill.Valid, "an empty skill list covers every skill")
}

func TestVerifyA2ADelegationChain_AnEmptyChainIsNotValid(t *testing.T) {
	v := VerifyA2ADelegationChain(nil, map[uuid.UUID]*Agent{}, "", delegationTestNow)

	assert.False(t, v.Valid)
	assert.Empty(t, v.Hops)
}

// The defaults a delegation is created with are the a2a_delegation table's
// column defaults.
func TestA2ADelegationDefaultsAreTheColumnDefaults(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		return string(b)
	}
	column := func(src, pattern string) float64 {
		m := regexp.MustCompile(pattern).FindStringSubmatch(src)
		require.Len(t, m, 2, "no match for %s", pattern)
		v, err := strconv.ParseFloat(m[1], 64)
		require.NoError(t, err)
		return v
	}
	attenuation := read("077_add_trust_attenuation_to_a2a_delegation.sql")
	assert.Equal(t, DefaultA2ADelegationTrustAttenuationFactor,
		column(attenuation, `trust_attenuation_factor DOUBLE PRECISION NOT NULL DEFAULT (\d+(?:\.\d+)?);`))
	assert.Equal(t, DefaultA2ADelegationMinTrustScore,
		column(attenuation, `min_delegated_trust_score DOUBLE PRECISION NOT NULL DEFAULT (\d+(?:\.\d+)?);`))
	assert.Equal(t, float64(DefaultA2ADelegationMaxChainDepth),
		column(read("070_add_a2a_supply_chain.sql"), `max_chain_depth INT DEFAULT (\d+),`))
}

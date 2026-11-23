package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// FGA Step 5 (the NanoMind intent check) is counted on /metrics in
//
//   - aim_fga_intent_checks_total{risk_tier, status, agreement}
//   - aim_fga_intent_skipped_total{risk_tier}
//
// The per-agent breakdown of the same counts is exported over OTel as
// fga.intent_checks and fga.intent_skipped with an agent.id attribute. This
// endpoint carries no label that identifies an agent, so these are the totals
// across agents. Every label takes its values from a closed set, and every
// series is created at start, at 0, so a scrape lists a tier that has seen no
// check.

// Step 5 risk tiers as labels. A policy risk level outside HIGH, MEDIUM and LOW
// is recorded as FGAIntentRiskTierOther, so a stored value cannot become a label.
const (
	FGAIntentRiskTierHigh   = "high"
	FGAIntentRiskTierMedium = "medium"
	FGAIntentRiskTierLow    = "low"
	FGAIntentRiskTierOther  = "other"
)

// Step 5 outcome statuses as labels, as IntentCheckResult.Status names them.
const (
	FGAIntentStatusClassified = "classified"
	FGAIntentStatusAbstain    = "abstain"
	FGAIntentStatusFailOpen   = "fail_open"
)

// Step 5 agreement as labels: whether the classifier's verdict agrees with the
// allow steps 1 to 4 reached. Step 5 runs only after they allow, so a confident
// benign verdict agrees and an attack class disagrees. An abstain or fail_open
// gives no verdict and is FGAIntentAgreementNone.
const (
	FGAIntentAgreementAgree    = "agree"
	FGAIntentAgreementDisagree = "disagree"
	FGAIntentAgreementNone     = "none"
)

var fgaIntentRiskTiers = []string{FGAIntentRiskTierHigh, FGAIntentRiskTierMedium, FGAIntentRiskTierLow, FGAIntentRiskTierOther}

// fgaIntentOutcomes are the (status, agreement) pairs a check can record.
var fgaIntentOutcomes = [][2]string{
	{FGAIntentStatusClassified, FGAIntentAgreementAgree},
	{FGAIntentStatusClassified, FGAIntentAgreementDisagree},
	{FGAIntentStatusAbstain, FGAIntentAgreementNone},
	{FGAIntentStatusFailOpen, FGAIntentAgreementNone},
}

var fgaIntentChecksTotal = func() *prometheus.CounterVec {
	vec := factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_fga_intent_checks_total",
			Help: "FGA Step 5 intent evaluations that reached the NanoMind daemon, by risk tier, outcome status and agreement with the allow of steps 1 to 4",
		},
		[]string{"risk_tier", "status", "agreement"},
	)
	for _, tier := range fgaIntentRiskTiers {
		for _, o := range fgaIntentOutcomes {
			vec.WithLabelValues(tier, o[0], o[1])
		}
	}
	return vec
}()

var fgaIntentSkippedTotal = func() *prometheus.CounterVec {
	vec := factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_fga_intent_skipped_total",
			Help: "FGA authorizations where the Step 5 intent check was skipped, by risk tier",
		},
		[]string{"risk_tier"},
	)
	for _, tier := range fgaIntentRiskTiers {
		vec.WithLabelValues(tier)
	}
	return vec
}()

// FGAIntentRiskTier maps a policy risk level (HIGH, MEDIUM, LOW, any case) to
// its risk_tier label, and anything else to FGAIntentRiskTierOther.
func FGAIntentRiskTier(policyRiskLevel string) string {
	switch strings.ToUpper(strings.TrimSpace(policyRiskLevel)) {
	case "HIGH":
		return FGAIntentRiskTierHigh
	case "MEDIUM":
		return FGAIntentRiskTierMedium
	case "LOW":
		return FGAIntentRiskTierLow
	default:
		return FGAIntentRiskTierOther
	}
}

// FGAIntentAgreement is the agreement label of a Step 5 outcome: agree for a
// classified benign verdict (empty attack class), disagree for a classified
// attack class, none for any other status.
func FGAIntentAgreement(status, attackClass string) string {
	if status != FGAIntentStatusClassified {
		return FGAIntentAgreementNone
	}
	if attackClass == "" {
		return FGAIntentAgreementAgree
	}
	return FGAIntentAgreementDisagree
}

// RecordFGAIntentCheck counts one Step 5 evaluation. policyRiskLevel is the
// policy risk level that triggered it, status its outcome status and
// attackClass the class the daemon returned. A status outside the closed set
// is recorded as fail_open, the outcome that carries no verdict.
func RecordFGAIntentCheck(policyRiskLevel, status, attackClass string) {
	switch status {
	case FGAIntentStatusClassified, FGAIntentStatusAbstain, FGAIntentStatusFailOpen:
	default:
		status = FGAIntentStatusFailOpen
	}
	fgaIntentChecksTotal.WithLabelValues(
		FGAIntentRiskTier(policyRiskLevel), status, FGAIntentAgreement(status, attackClass),
	).Inc()
}

// RecordFGAIntentSkipped counts one authorization where Step 5 was skipped.
func RecordFGAIntentSkipped(policyRiskLevel string) {
	fgaIntentSkippedTotal.WithLabelValues(FGAIntentRiskTier(policyRiskLevel)).Inc()
}

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const (
	trustReevaluationIntervalEnv = "TRUST_REEVALUATION_INTERVAL"
	// defaultTrustReevaluationInterval applies when TRUST_REEVALUATION_INTERVAL is unset or unusable.
	defaultTrustReevaluationInterval = time.Hour
	// minTrustReevaluationInterval is the floor: each run recalculates every
	// agent that is not revoked, several queries for each, so a shorter
	// interval is raised to this.
	minTrustReevaluationInterval = 5 * time.Minute
	// trustScoreStep is the precision trust_scores.score is stored at. A
	// recalculated score closer than this to the last one on record is the
	// same score and is not recorded again.
	trustScoreStep = 0.001
)

// trustReevaluationInterval returns how often the trust re-evaluation job
// runs, read from TRUST_REEVALUATION_INTERVAL as a Go duration ("30m", "2h").
// Unset uses the one-hour default. A value that does not parse or is not
// positive also uses the default, and a value under five minutes runs at five
// minutes; both return a warning naming the variable and the interval in
// effect. The warning is empty when the value is used as given.
func trustReevaluationInterval(getenv func(string) string) (time.Duration, string) {
	return backgroundJobInterval(getenv, trustReevaluationIntervalEnv,
		defaultTrustReevaluationInterval, minTrustReevaluationInterval)
}

// trustReevaluationAgents is the agent row as the job uses it. The score is
// written with the narrow UpdateTrustScore, never the whole-row Update, which
// would write back every column as it was read a moment earlier.
type trustReevaluationAgents interface {
	GetByID(id uuid.UUID) (*domain.Agent, error)
	UpdateTrustScore(id uuid.UUID, newScore float64) error
}

// trustReevaluationScores is the trust_scores history as the job uses it.
type trustReevaluationScores interface {
	GetLatest(agentID uuid.UUID) (*domain.TrustScore, error)
	Create(score *domain.TrustScore) error
}

// trustReevaluationCalculator calculates an agent's score without storing it.
type trustReevaluationCalculator interface {
	Calculate(agent *domain.Agent) (*domain.TrustScore, error)
}

// trustReevaluationWebhooks sends trust_score.changed to an organization's
// webhooks. The agent service compares the two scores at the precision
// agents.trust_score stores and sends nothing when they are the same.
type trustReevaluationWebhooks interface {
	NotifyTrustScoreChanged(agent *domain.Agent, previousScore, newScore float64)
}

// trustReevaluation recalculates agent trust scores on a schedule, so a score
// moves when its inputs do (a verification event leaving the 30-day window, an
// agent going quiet, an alert acknowledged, an agent growing older) and not
// only when someone asks for a recalculation.
//
// A score that moved is recorded the way POST
// /api/v1/agents/:id/trust-score/recalculate records it, a trust_scores row
// and the agent's trust_score, and trust_score.changed is sent to the
// organization's webhooks. A score that has not moved since the last one on
// record is left alone, so a pass adds no history and does not touch an
// agent's updated_at unless something changed. Every server instance runs the
// job; a pass that finds nothing changed writes nothing.
type trustReevaluation struct {
	listAgentIDs func(ctx context.Context) ([]uuid.UUID, error)
	agents       trustReevaluationAgents
	scores       trustReevaluationScores
	calculator   trustReevaluationCalculator
	webhooks     trustReevaluationWebhooks
}

// trustReevaluationPass is what one pass over the agents did.
type trustReevaluationPass struct {
	Evaluated int   // agents whose score was recalculated
	Changed   int   // of those, scores that moved and were recorded
	Failed    int   // agents that could not be re-evaluated
	FirstErr  error // the first failure, naming its agent
}

func (p trustReevaluationPass) String() string {
	summary := fmt.Sprintf("re-evaluated %d agent(s), %d score(s) changed", p.Evaluated, p.Changed)
	if p.Failed > 0 {
		summary += fmt.Sprintf(", %d failed (first: %v)", p.Failed, p.FirstErr)
	}
	return summary
}

// run re-evaluates every listed agent once. An agent that fails is counted and
// the pass goes on to the next; only a failed listing or a cancelled context
// ends it early.
func (j *trustReevaluation) run(ctx context.Context) (trustReevaluationPass, error) {
	var pass trustReevaluationPass

	ids, err := j.listAgentIDs(ctx)
	if err != nil {
		return pass, fmt.Errorf("list agents: %w", err)
	}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return pass, err
		}
		evaluated, changed, err := j.reevaluate(id)
		if err != nil {
			pass.Failed++
			if pass.FirstErr == nil {
				pass.FirstErr = fmt.Errorf("agent %s: %w", id, err)
			}
			continue
		}
		if evaluated {
			pass.Evaluated++
		}
		if changed {
			pass.Changed++
		}
	}

	return pass, nil
}

// reevaluate recalculates one agent's score and records it if it moved. An
// agent revoked since the listing is not evaluated.
//
// The comparison is with the last calculated score on record, not with the
// agent row's score, so a score an administrator set by hand stays as set
// until the calculation itself moves. Once the agent row carries the new
// score, trust_score.changed is sent from the score the row carried before.
func (j *trustReevaluation) reevaluate(id uuid.UUID) (evaluated, changed bool, err error) {
	agent, err := j.agents.GetByID(id)
	if err != nil {
		return false, false, fmt.Errorf("read agent: %w", err)
	}
	if agent.Status == domain.AgentStatusRevoked {
		return false, false, nil
	}

	score, err := j.calculator.Calculate(agent)
	if err != nil {
		return false, false, fmt.Errorf("calculate score: %w", err)
	}

	latest, err := j.scores.GetLatest(id)
	if err != nil {
		return false, false, fmt.Errorf("read last recorded score: %w", err)
	}
	if latest != nil && math.Abs(score.Score-latest.Score) < trustScoreStep {
		return true, false, nil
	}

	if err := j.scores.Create(score); err != nil {
		return false, false, fmt.Errorf("record score: %w", err)
	}
	if err := j.agents.UpdateTrustScore(id, score.Score); err != nil {
		return false, false, fmt.Errorf("update agent score: %w", err)
	}
	j.webhooks.NotifyTrustScoreChanged(agent, agent.TrustScore, score.Score)
	return true, true, nil
}

// loop runs a pass on every tick until ctx is cancelled. Cancelling ctx also
// ends a pass that is under way, before its next agent.
func (j *trustReevaluation) loop(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ticks:
			started := time.Now()
			pass, err := j.run(ctx)
			switch {
			case ctx.Err() != nil:
				// Stopped mid-pass: the agents not reached keep the score they had.
				return
			case err != nil:
				log.Printf("Trust re-evaluation failed: %v", err)
			default:
				log.Printf("Trust re-evaluation: %s in %s", pass, time.Since(started).Round(time.Millisecond))
			}
		case <-ctx.Done():
			return
		}
	}
}

// startTrustReevaluationJob runs a trust re-evaluation pass every interval.
// The returned stop function ends a pass under way before its next agent,
// stops the job and returns once the job's goroutine has exited. Calling it
// again does nothing.
func startTrustReevaluationJob(job *trustReevaluation, interval time.Duration) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	log.Printf("Trust re-evaluation job started (runs every %s)", interval)

	go func() {
		defer close(done)
		job.loop(ctx, ticker.C)
		ticker.Stop()
		log.Println("Trust re-evaluation job stopped")
	}()

	return func() {
		cancel()
		<-done
	}
}

// reevaluableAgentIDs lists the agents whose score the job keeps current: every
// agent in every organization that is not revoked. A revoked agent's score is
// settled when it is revoked. It is a system task's listing, never a tenant
// request's. One query rather than LIMIT/OFFSET pages, because a page boundary
// between agents with the same created_at can skip an agent.
func reevaluableAgentIDs(db *sql.DB) func(ctx context.Context) ([]uuid.UUID, error) {
	return func(ctx context.Context) ([]uuid.UUID, error) {
		rows, err := db.QueryContext(ctx,
			`SELECT id FROM agents WHERE status <> $1 ORDER BY created_at, id`,
			string(domain.AgentStatusRevoked))
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		ids := make([]uuid.UUID, 0)
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return ids, nil
	}
}

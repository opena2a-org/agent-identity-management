package application

import (
	"testing"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The A2A composite once started every agent at 0.5 and added 0.1 for missing
// response data and 0.1 for account age unconditionally, so an agent with no
// task at all read 0.7 while an agent with a perfect task record, no peers and
// no response figures read 0.6: no data outranked perfect data. The ruled
// behaviour: an agent without task data is UNSCORED (no number), and a
// measured agent always outranks an unscored one.

func f(x float64) *float64 { return &x }

func TestComposeA2ATrustScore_NoTaskDataIsUnscored(t *testing.T) {
	noData := &domain.A2ATrustScore{AgentID: uuid.New()}
	if got := composeA2ATrustScore(noData, 0); got != nil {
		t.Fatalf("an agent with no task data must be unscored (nil), got %.3f", *got)
	}
	// Peer opinions or response figures without a single task are still no
	// task data: the composite's first term has nothing to measure.
	withPeers := &domain.A2ATrustScore{AgentID: uuid.New(), AvgResponseTimeMs: intp(800)}
	if got := composeA2ATrustScore(withPeers, 0.9); got != nil {
		t.Fatalf("no tasks plus peer/response figures must still be unscored, got %.3f", *got)
	}
}

func TestComposeA2ATrustScore_MeasuredAlwaysOutranksUnscored(t *testing.T) {
	perfect := &domain.A2ATrustScore{AgentID: uuid.New(), TasksCompleted: 10, TasksFailed: 0}
	got := composeA2ATrustScore(perfect, 0)
	if got == nil {
		t.Fatal("an agent with task data must carry a score")
	}
	noData := composeA2ATrustScore(&domain.A2ATrustScore{AgentID: uuid.New()}, 0)
	if noData != nil && *noData > *got {
		t.Fatalf("no data (%.3f) outranks perfect data (%.3f)", *noData, *got)
	}
	// The worst measured agent is still a measured agent: it must read as a
	// number, and never as high as the fabricated 0.7 the old default produced.
	worst := composeA2ATrustScore(&domain.A2ATrustScore{AgentID: uuid.New(), TasksCompleted: 0, TasksFailed: 10}, 0)
	if worst == nil {
		t.Fatal("a fully failed agent is measured, not unscored")
	}
	if *worst >= 0.7 {
		t.Fatalf("a fully failed agent reads %.3f, the old no-data default", *worst)
	}
}

func TestComposeA2ATrustScore_MeasuredComponents(t *testing.T) {
	// 10/10 tasks, peer average 0.5, 1000 ms responses: 0.4*1 + 0.3*0.5 + 0.2*(1-0.2) + 0.1 = 0.81
	s := &domain.A2ATrustScore{AgentID: uuid.New(), TasksCompleted: 10, TasksFailed: 0, AvgResponseTimeMs: intp(1000)}
	got := composeA2ATrustScore(s, 0.5)
	if got == nil || (*got < 0.809 || *got > 0.811) {
		t.Fatalf("measured composite = %v, want 0.81", got)
	}
	// Missing response data contributes nothing, not a neutral credit.
	noResp := composeA2ATrustScore(&domain.A2ATrustScore{AgentID: uuid.New(), TasksCompleted: 10}, 0.5)
	if noResp == nil || (*noResp < 0.649 || *noResp > 0.651) {
		t.Fatalf("composite without response data = %v, want 0.65 (0.4 + 0.15 + 0 + 0.1)", noResp)
	}
	_ = f
}

func intp(v int) *int { return &v }

package application

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// MCPTrustRescoreMarkerKey is the system_config key that records that the
// one-time MCP rescoring pass has completed.
//
// Migration 104 moved `mcp_servers.trust_score` to the [0,1] scale by dividing
// stored values above 1.0 by 100. That rescales a placeholder; it does not
// replace it. Before the calculator was wired into registration and
// verification, the column only ever held a literal: 75.0 for an SDK-registered
// or verified server, 0.0 for a manually registered one. After 104 those read
// 0.75 and 0.0, indistinguishable from a calculated score, and they stay that
// way until the server is re-verified, because registration and verification
// are the only occasions that score a server.
//
// The pass replaces every such value with a calculated one, once.
const MCPTrustRescoreMarkerKey = "mcp_trust_rescored_after_scale_migration"

const mcpTrustRescoreMarkerDescription = "Every MCP server was scored by the 8-factor calculator once after migration 104 rescaled the placeholder trust scores"

// SystemMarkerStore reads and writes completion markers for one-time tasks.
type SystemMarkerStore interface {
	IsMarkerSet(ctx context.Context, key string) (bool, error)
	SetMarker(ctx context.Context, key, description string) error
}

// MCPServerIDLister returns the ID of every MCP server, across all
// organizations. The rescoring pass is a system task, not a tenant request.
type MCPServerIDLister interface {
	ListAllIDs(ctx context.Context) ([]uuid.UUID, error)
}

// MCPTrustRescoreResult reports what one run of RescoreMCPServersOnce did.
type MCPTrustRescoreResult struct {
	// AlreadyDone is true when the marker was set before this run, so no
	// server was scored.
	AlreadyDone bool
	Scored      int
	// Failed lists the servers whose scoring failed. Their rows are unchanged.
	Failed []uuid.UUID
}

// RescoreMCPServersOnce scores every MCP server through the trust calculator,
// unless the pass has already completed on this database.
//
// A server whose scoring fails is left unchanged and logged; the pass goes on
// to the next server. The calculator writes nothing for a failed server: the
// score row is inserted only after the calculation succeeds, and that insert
// is what updates `mcp_servers.trust_score` (migration 094 trigger).
//
// The marker is set once every server has been attempted. It is not set when
// the marker cannot be read, the servers cannot be listed, or the context is
// cancelled part-way, so the next start runs the pass again. Running it twice
// is harmless: each run records a fresh calculation for each server.
func RescoreMCPServersOnce(ctx context.Context, markers SystemMarkerStore, servers MCPServerIDLister, calculator *MCPTrustCalculator) (*MCPTrustRescoreResult, error) {
	done, err := markers.IsMarkerSet(ctx, MCPTrustRescoreMarkerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP rescore marker: %w", err)
	}
	if done {
		return &MCPTrustRescoreResult{AlreadyDone: true}, nil
	}
	if calculator == nil {
		return nil, fmt.Errorf("no MCP trust calculator configured")
	}

	ids, err := servers.ListAllIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP servers for rescoring: %w", err)
	}

	result := &MCPTrustRescoreResult{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("MCP rescoring stopped after %d of %d servers: %w", result.Scored+len(result.Failed), len(ids), err)
		}
		if _, err := calculator.CalculateTrustScore(ctx, id); err != nil {
			log.Printf("MCP rescoring: failed to score MCP server %s, trust score left unchanged: %v", id, err)
			result.Failed = append(result.Failed, id)
			continue
		}
		result.Scored++
	}

	if err := markers.SetMarker(ctx, MCPTrustRescoreMarkerKey, mcpTrustRescoreMarkerDescription); err != nil {
		return result, fmt.Errorf("failed to record MCP rescore marker: %w", err)
	}
	return result, nil
}

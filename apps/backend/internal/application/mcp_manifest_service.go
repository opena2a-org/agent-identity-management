package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

func sortedStrings(s []string) []string {
	sort.Strings(s)
	return s
}

// genericCapabilityCategories are the MCP capability-category tokens an attestation may report instead
// of (or alongside) concrete tool names. They are not tool names and must not enter the tool manifest.
// Kept in sync with MCPServerCapabilityRepository.UpsertFromAttestation, which skips the same tokens.
var genericCapabilityCategories = map[string]struct{}{
	"tools": {}, "resources": {}, "prompts": {},
}

// filterAttestedToolNames drops generic capability-category tokens from an attestation's reported names,
// leaving only concrete tool names for manifest diffing.
func filterAttestedToolNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if _, generic := genericCapabilityCategories[n]; generic {
			continue
		}
		out = append(out, n)
	}
	return out
}

// MCPManifestService maintains a per-server manifest baseline and records drift when a server's
// tool surface changes after it was trusted. Two sources feed it:
//
//   - well_known: the server's own /.well-known/mcp/capabilities declaration. Authoritative; every
//     change updates the baseline and records drift.
//   - attestation: tool names observed by attesting agents. Used to catch a server that declares one
//     thing but behaves differently to agents. Baseline movement and drift recording from this source
//     require multi-agent consensus, so a single rogue or misconfigured agent cannot poison the baseline.
type MCPManifestService struct {
	capabilityRepo *repository.MCPServerCapabilityRepository
	manifestRepo   *repository.MCPManifestRepository
	alertRepo      *repository.AlertRepository
	mcpRepo        *repository.MCPServerRepository
}

func NewMCPManifestService(
	capabilityRepo *repository.MCPServerCapabilityRepository,
	manifestRepo *repository.MCPManifestRepository,
) *MCPManifestService {
	return &MCPManifestService{capabilityRepo: capabilityRepo, manifestRepo: manifestRepo}
}

// SetAlerting wires the alert + MCP-server repositories so a recorded drift raises an operator-facing
// alert (issue #275). Optional and injected via a setter so the existing constructor, its callers, and
// the drift integration tests stay unchanged; when unset, drift is still recorded and logged but no
// alert is raised. The MCP-server repo is needed to resolve the server's organization and name for the
// alert. Best-effort: see raiseDriftAlert.
func (s *MCPManifestService) SetAlerting(alertRepo *repository.AlertRepository, mcpRepo *repository.MCPServerRepository) {
	s.alertRepo = alertRepo
	s.mcpRepo = mcpRepo
}

// RecomputeFromWellKnown rebuilds the manifest from the server's stored capabilities (populated by a
// /.well-known capability fetch) and compares it to the current baseline. On first capture it seeds
// the baseline and records no drift. On a hash change it records drift and advances the baseline.
// Returns the drift record when one was created, otherwise nil.
func (s *MCPManifestService) RecomputeFromWellKnown(ctx context.Context, serverID uuid.UUID) (*domain.MCPManifestDrift, error) {
	caps, err := s.capabilityRepo.GetByServerID(serverID)
	if err != nil {
		return nil, fmt.Errorf("load capabilities: %w", err)
	}
	entries, hash, tools, resources, prompts := domain.BuildManifest(caps)

	newBaseline := &domain.MCPManifestBaseline{
		MCPServerID:   serverID,
		ManifestHash:  hash,
		ToolCount:     tools,
		ResourceCount: resources,
		PromptCount:   prompts,
		CapturedFrom:  domain.ManifestSourceWellKnown,
	}

	// The baseline read, drift insert, and baseline replace run in a single transaction under a
	// per-server advisory lock (issue #276), so two concurrent well-known recomputes for the same
	// server serialize instead of interleaving into a duplicate or orphaned drift row. The diff against
	// the locked baseline is computed in the callback; returning nil means the manifest is unchanged
	// (no drift, no baseline move). First capture (prev == nil) seeds the baseline without calling it.
	drift, err := s.manifestRepo.AtomicRecomputeWellKnown(serverID, newBaseline, entries,
		func(prev *domain.MCPManifestBaseline, prevEntries []domain.ManifestEntry) *domain.MCPManifestDrift {
			if prev.ManifestHash == hash {
				return nil
			}
			added, removed, changed := domain.DiffManifests(prevEntries, entries)
			return &domain.MCPManifestDrift{
				MCPServerID:  serverID,
				PrevHash:     prev.ManifestHash,
				NewHash:      hash,
				AddedTools:   added,
				RemovedTools: removed,
				ChangedTools: changed,
				Severity:     domain.ClassifyDriftSeverity(added, removed, changed),
				Source:       domain.ManifestSourceWellKnown,
			}
		})
	if err != nil {
		return nil, err
	}
	if drift != nil {
		s.raiseDriftAlert(serverID, drift)
	}
	return drift, nil
}

// raiseDriftAlert surfaces a recorded manifest drift to operators via the alert stream. It is the
// single choke point for both the well-known and attestation drift sources, so an operator sees a
// trusted server's tool surface changing (e.g. a verified server silently adding an exec_* tool)
// regardless of which feed detected it (issue #275). Severity maps the drift grade onto the alert
// scale: high -> high, medium -> warning, low -> info.
//
// Best-effort and nil-safe: alerting is optional (wired via SetAlerting), and an alert-write failure
// must never fail capability detection or attestation, so every failure path logs and returns rather
// than propagating an error.
func (s *MCPManifestService) raiseDriftAlert(serverID uuid.UUID, drift *domain.MCPManifestDrift) {
	if s.alertRepo == nil || s.mcpRepo == nil {
		return
	}
	server, err := s.mcpRepo.GetByID(serverID)
	if err != nil || server == nil {
		fmt.Printf("⚠️  Failed to load MCP server %s for drift alert: %v\n", serverID, err)
		return
	}

	var severity domain.AlertSeverity
	switch drift.Severity {
	case domain.DriftSeverityHigh:
		severity = domain.AlertSeverityHigh
	case domain.DriftSeverityMedium:
		severity = domain.AlertSeverityWarning
	default:
		severity = domain.AlertSeverityInfo
	}

	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: server.OrganizationID,
		AlertType:      domain.AlertMCPManifestDrift,
		Severity:       severity,
		Title:          fmt.Sprintf("MCP manifest drift detected: %s", server.Name),
		Description: fmt.Sprintf(
			"The tool manifest for MCP server '%s' changed after it was trusted (source: %s, severity: %s): +%d added, -%d removed, ~%d changed.",
			server.Name, drift.Source, drift.Severity,
			len(drift.AddedTools), len(drift.RemovedTools), len(drift.ChangedTools)),
		ResourceType: "mcp_server",
		ResourceID:   serverID,
		Metadata: map[string]interface{}{
			"mcpServerId":   serverID.String(),
			"mcpServerName": server.Name,
			"source":        string(drift.Source),
			"driftSeverity": string(drift.Severity),
			"prevHash":      drift.PrevHash,
			"newHash":       drift.NewHash,
			"addedTools":    orEmpty(drift.AddedTools),
			"removedTools":  orEmpty(drift.RemovedTools),
			"changedTools":  orEmpty(drift.ChangedTools),
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := s.alertRepo.Create(alert); err != nil {
		fmt.Printf("⚠️  Failed to create MCP manifest drift alert for %s: %v\n", server.Name, err)
		return
	}
	fmt.Printf("🚨 Created MCP manifest drift alert for %s (severity=%s)\n", server.Name, severity)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// RecomputeFromAttestation compares the tool names an agent reported against the current baseline's
// tool set and records drift when they diverge. It deliberately NEVER mutates the baseline: the
// /.well-known feed is the sole baseline author. This is the baseline-poisoning guard — the consensus
// signal available here ("has this server ever reached multi-agent verification?") is a server-lifetime
// latch, not per-manifest corroboration, so allowing it to advance the baseline would let a single
// trusted in-org agent silently rebaseline a verified server to an arbitrary tool set. Instead,
// consensus-confirmed divergence is recorded as a drift observation against the unchanged baseline;
// the server's own next /.well-known capture is what legitimately moves the baseline.
//
// consensusMet still gates RECORDING (below consensus, a lone agent's divergence is ignored to avoid
// false positives). Divergence is deduplicated against recent unacknowledged attestation drift so the
// same discrepancy is not re-recorded on every subsequent attestation. Only tool-type entries are
// attested (resources/prompts are not), so the diff is over tool names. Returns the drift record when
// one was created, otherwise nil.
func (s *MCPManifestService) RecomputeFromAttestation(ctx context.Context, serverID uuid.UUID, observedNames []string, consensusMet bool) (*domain.MCPManifestDrift, error) {
	// Attestation payloads mix real tool names with the generic MCP capability-category tokens
	// ("tools"/"resources"/"prompts"). The capability sync (UpsertFromAttestation) skips those tokens
	// because they are not tool names; the manifest diff must skip them too, or every consensus-met
	// attestation that lists a category would record spurious "added tool: tools" drift.
	observedNames = filterAttestedToolNames(observedNames)
	if len(observedNames) == 0 {
		// Empty report (or one carrying only category tokens) is treated as missing data, never as
		// "all tools removed".
		return nil, nil
	}

	baseline, prevEntries, err := s.manifestRepo.GetCurrentBaseline(serverID)
	if err != nil {
		return nil, err
	}
	if baseline == nil {
		// No /.well-known baseline yet: there is nothing authoritative to diverge from, and attestation
		// must not seed a baseline (well-known is the sole author). Nothing to record.
		return nil, nil
	}

	observedEntries, _ := domain.BuildManifestFromNames(observedNames)
	observedSet := make(map[string]struct{}, len(observedEntries))
	for _, e := range observedEntries {
		observedSet[e.Name] = struct{}{}
	}

	// Diff observed tool names against the baseline's tool names (only tool-type entries are attested).
	baselineTools := make(map[string]struct{})
	for _, e := range prevEntries {
		if e.Type == domain.MCPCapabilityTypeTool {
			baselineTools[e.Name] = struct{}{}
		}
	}

	added := make([]string, 0)
	removed := make([]string, 0)
	for name := range observedSet {
		if _, ok := baselineTools[name]; !ok {
			added = append(added, "tool:"+name)
		}
	}
	for name := range baselineTools {
		if _, ok := observedSet[name]; !ok {
			removed = append(removed, "tool:"+name)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return nil, nil
	}
	if !consensusMet {
		// Unconfirmed divergence from a sub-threshold agent set is not recorded as server drift.
		return nil, nil
	}

	// Hash of what the agents observed, used both as the drift's NewHash (the baseline's hash is the
	// unchanged PrevHash) and as the dedup key.
	observedHash, _, _, _ := domain.HashManifestEntries(observedEntries)

	// Dedup: if this exact observed divergence is already recorded and unacknowledged, do not record it
	// again. Without this, every attestation carrying the same divergence would add a duplicate row,
	// because the baseline (correctly) never moves to absorb it. Exact existence check, so a repeat is
	// suppressed regardless of how many other drift rows accumulated in between.
	dup, err := s.manifestRepo.HasUnacknowledgedDriftWithHash(serverID, domain.ManifestSourceAttestation, observedHash)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, nil
	}

	drift := &domain.MCPManifestDrift{
		MCPServerID:  serverID,
		PrevHash:     baseline.ManifestHash,
		NewHash:      observedHash,
		AddedTools:   sortedStrings(added),
		RemovedTools: sortedStrings(removed),
		ChangedTools: []string{},
		Severity:     domain.ClassifyDriftSeverity(added, removed, nil),
		Source:       domain.ManifestSourceAttestation,
	}
	if err := s.manifestRepo.InsertDrift(drift); err != nil {
		return nil, err
	}
	s.raiseDriftAlert(serverID, drift)
	return drift, nil
}

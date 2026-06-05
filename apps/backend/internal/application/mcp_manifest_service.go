package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

func sortedStrings(s []string) []string {
	sort.Strings(s)
	return s
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
}

func NewMCPManifestService(
	capabilityRepo *repository.MCPServerCapabilityRepository,
	manifestRepo *repository.MCPManifestRepository,
) *MCPManifestService {
	return &MCPManifestService{capabilityRepo: capabilityRepo, manifestRepo: manifestRepo}
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

	baseline, prevEntries, err := s.manifestRepo.GetCurrentBaseline(serverID)
	if err != nil {
		return nil, err
	}

	newBaseline := &domain.MCPManifestBaseline{
		MCPServerID:   serverID,
		ManifestHash:  hash,
		ToolCount:     tools,
		ResourceCount: resources,
		PromptCount:   prompts,
		CapturedFrom:  domain.ManifestSourceWellKnown,
	}

	if baseline == nil {
		// First capture: establish baseline, no drift.
		return nil, s.manifestRepo.ReplaceBaseline(newBaseline, entries)
	}
	if baseline.ManifestHash == hash {
		return nil, nil
	}

	added, removed, changed := domain.DiffManifests(prevEntries, entries)
	drift := &domain.MCPManifestDrift{
		MCPServerID:  serverID,
		PrevHash:     baseline.ManifestHash,
		NewHash:      hash,
		AddedTools:   added,
		RemovedTools: removed,
		ChangedTools: changed,
		Severity:     domain.ClassifyDriftSeverity(added, removed, changed),
		Source:       domain.ManifestSourceWellKnown,
	}
	if err := s.manifestRepo.InsertDrift(drift); err != nil {
		return nil, err
	}
	if err := s.manifestRepo.ReplaceBaseline(newBaseline, entries); err != nil {
		return nil, err
	}
	return drift, nil
}

// RecomputeFromAttestation compares the tool names an agent reported against the current baseline's
// tool set. Divergence (a tool agents see that the server never declared, or vice versa) is only
// treated as server drift when consensusMet is true; below consensus it is ignored to avoid
// single-agent false positives. Resources and prompts are not attested, so non-tool baseline entries
// are carried forward unchanged. Returns the drift record when one was created, otherwise nil.
func (s *MCPManifestService) RecomputeFromAttestation(ctx context.Context, serverID uuid.UUID, observedNames []string, consensusMet bool) (*domain.MCPManifestDrift, error) {
	if len(observedNames) == 0 {
		// Empty report is treated as missing data, never as "all tools removed".
		return nil, nil
	}

	baseline, prevEntries, err := s.manifestRepo.GetCurrentBaseline(serverID)
	if err != nil {
		return nil, err
	}

	observedEntries, _ := domain.BuildManifestFromNames(observedNames)
	observedSet := make(map[string]struct{}, len(observedEntries))
	for _, e := range observedEntries {
		observedSet[e.Name] = struct{}{}
	}

	if baseline == nil {
		// Only a consensus-backed attestation may establish a baseline from observation alone.
		if !consensusMet {
			return nil, nil
		}
		hash, tools, res, prm := domain.HashManifestEntries(observedEntries)
		return nil, s.manifestRepo.ReplaceBaseline(&domain.MCPManifestBaseline{
			MCPServerID: serverID, ManifestHash: hash, ToolCount: tools,
			ResourceCount: res, PromptCount: prm, CapturedFrom: domain.ManifestSourceAttestation,
		}, observedEntries)
	}

	// Diff observed tool names against the baseline's tool names (only tool-type entries are attested).
	baselineTools := make(map[string]domain.ManifestEntry)
	var carried []domain.ManifestEntry // non-tool entries preserved across an attestation baseline move
	for _, e := range prevEntries {
		if e.Type == domain.MCPCapabilityTypeTool {
			baselineTools[e.Name] = e
		} else {
			carried = append(carried, e)
		}
	}

	var added, removed []string
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
		// Unconfirmed divergence from a single agent is not recorded as server drift.
		return nil, nil
	}

	// Consensus-confirmed: record drift and advance the baseline. Carry forward known tools' schema
	// hashes so a later well-known capture can still detect a schema change on a surviving tool.
	newEntries := append([]domain.ManifestEntry{}, carried...)
	for name := range observedSet {
		if prev, ok := baselineTools[name]; ok {
			newEntries = append(newEntries, prev)
		} else {
			newEntries = append(newEntries, domain.ManifestEntry{Type: domain.MCPCapabilityTypeTool, Name: name})
		}
	}
	newHash, tools, res, prm := domain.HashManifestEntries(newEntries)

	drift := &domain.MCPManifestDrift{
		MCPServerID:  serverID,
		PrevHash:     baseline.ManifestHash,
		NewHash:      newHash,
		AddedTools:   sortedStrings(added),
		RemovedTools: sortedStrings(removed),
		ChangedTools: []string{},
		Severity:     domain.ClassifyDriftSeverity(added, removed, nil),
		Source:       domain.ManifestSourceAttestation,
	}
	if err := s.manifestRepo.InsertDrift(drift); err != nil {
		return nil, err
	}
	if err := s.manifestRepo.ReplaceBaseline(&domain.MCPManifestBaseline{
		MCPServerID: serverID, ManifestHash: newHash, ToolCount: tools,
		ResourceCount: res, PromptCount: prm, CapturedFrom: domain.ManifestSourceAttestation,
	}, newEntries); err != nil {
		return nil, err
	}
	return drift, nil
}

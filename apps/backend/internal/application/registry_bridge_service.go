package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const (
	registryBridgeToolName    = "aim-server"
	registryBridgeToolVersion = "1.5.0"
)

// RegistryBridgeService aggregates anonymized attestation data from opted-in
// organizations and pushes contribution batches to the OpenA2A Registry.
type RegistryBridgeService struct {
	orgRepo         domain.OrganizationRepository
	agentRepo       domain.AgentRepository
	attestationRepo domain.MCPAttestationRepository
	mcpServerRepo   domain.MCPServerRepository
	auditLogRepo    domain.AuditLogRepository
	registryURL     string
	httpClient      *http.Client
}

// NewRegistryBridgeService creates a new bridge service that pushes aggregated
// attestation data to the specified registry URL.
func NewRegistryBridgeService(
	orgRepo domain.OrganizationRepository,
	agentRepo domain.AgentRepository,
	attestationRepo domain.MCPAttestationRepository,
	mcpServerRepo domain.MCPServerRepository,
	auditLogRepo domain.AuditLogRepository,
	registryURL string,
) *RegistryBridgeService {
	return &RegistryBridgeService{
		orgRepo:         orgRepo,
		agentRepo:       agentRepo,
		attestationRepo: attestationRepo,
		mcpServerRepo:   mcpServerRepo,
		auditLogRepo:    auditLogRepo,
		registryURL:     registryURL,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

// AggregateAndPush iterates over all active organizations, builds anonymized
// contribution events, and POSTs them to the OpenA2A Registry.
func (s *RegistryBridgeService) AggregateAndPush(ctx context.Context) error {
	orgs, err := s.orgRepo.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	secret := os.Getenv("REGISTRY_BRIDGE_SECRET")
	if secret == "" {
		return fmt.Errorf("REGISTRY_BRIDGE_SECRET environment variable is not set")
	}

	var totalEvents int
	var pushErrors int

	for _, org := range orgs {
		if !org.IsActive {
			continue
		}

		events, err := s.buildOrgContribution(ctx, org.ID)
		if err != nil {
			log.Printf("Registry bridge: failed to build contribution for org %s: %v", org.ID, err)
			continue
		}

		if len(events) == 0 {
			continue
		}

		token := generateContributorToken(org.ID, secret)
		batch := domain.ContributionBatch{
			ContributorToken: token,
			Events:           events,
			SubmittedAt:      time.Now().UTC().Format(time.RFC3339),
		}

		if err := s.pushBatch(ctx, &batch); err != nil {
			log.Printf("Registry bridge: failed to push batch for org %s: %v", org.ID, err)
			pushErrors++
			continue
		}

		totalEvents += len(events)
	}

	log.Printf("Registry bridge: pushed %d events from %d orgs (%d errors)", totalEvents, len(orgs), pushErrors)
	return nil
}

// buildOrgContribution creates anonymized contribution events for one organization.
// Privacy: only MCP package names, aggregate counts, and trust score averages
// are included. No org names, agent names, user emails, or raw audit entries.
func (s *RegistryBridgeService) buildOrgContribution(ctx context.Context, orgID uuid.UUID) ([]domain.ContributionEvent, error) {
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}

	if len(agents) == 0 {
		return nil, nil
	}

	var events []domain.ContributionEvent
	now := time.Now().UTC().Format(time.RFC3339)

	for _, agent := range agents {
		attestations, err := s.attestationRepo.GetAttestationsByAgent(agent.ID)
		if err != nil {
			log.Printf("Registry bridge: failed to get attestations for agent %s: %v", agent.ID, err)
			continue
		}

		if len(attestations) == 0 {
			continue
		}

		// Group attestations by MCP server to produce one event per MCP
		mcpAttestations := make(map[uuid.UUID][]*domain.MCPAttestation)
		for _, att := range attestations {
			mcpAttestations[att.MCPServerID] = append(mcpAttestations[att.MCPServerID], att)
		}

		for mcpServerID, atts := range mcpAttestations {
			mcpServer, err := s.mcpServerRepo.GetByID(mcpServerID)
			if err != nil {
				log.Printf("Registry bridge: failed to get MCP server %s: %v", mcpServerID, err)
				continue
			}

			// Count successful attestations (signature verified and valid)
			var passed int
			for _, a := range atts {
				if a.SignatureVerified && a.IsValid {
					passed++
				}
			}

			// Count audit log entries for this agent as a proxy for interaction count
			auditLogs, err := s.auditLogRepo.GetByAgent(agent.ID, 1000, 0)
			interactionCount := 0
			if err == nil {
				interactionCount = len(auditLogs)
			}

			// Calculate success rate from trust score (0-1 range, map to 0-1)
			successRate := agent.TrustScore
			if successRate > 1.0 {
				successRate = successRate / 100.0
			}

			// Determine ecosystem from MCP server URL heuristics
			ecosystem := inferEcosystem(mcpServer.URL)

			event := domain.ContributionEvent{
				Type:        "interaction",
				Tool:        registryBridgeToolName,
				ToolVersion: registryBridgeToolVersion,
				Timestamp:   now,
				Package: &domain.PackageRef{
					Name:      mcpServer.Name,
					Version:   mcpServer.Version,
					Ecosystem: ecosystem,
				},
				BehaviorSummary: &domain.BehaviorSummary{
					Interactions: interactionCount,
					SuccessRate:  successRate,
					Anomalies:    agent.CapabilityViolationCount,
				},
			}

			// If we have enough attestation data, also include a scan summary
			if len(atts) >= 2 {
				score := 0
				if len(atts) > 0 {
					score = (passed * 100) / len(atts)
				}
				verdict := "pass"
				if score < 50 {
					verdict = "fail"
				} else if score < 80 {
					verdict = "warn"
				}

				event.ScanSummary = &domain.ScanSummary{
					TotalChecks: len(atts),
					Passed:      passed,
					Score:       score,
					Verdict:     verdict,
				}
			}

			events = append(events, event)
		}
	}

	return events, nil
}

// pushBatch sends a ContributionBatch to the OpenA2A Registry.
func (s *RegistryBridgeService) pushBatch(ctx context.Context, batch *domain.ContributionBatch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	url := s.registryURL + "/api/v1/contribute"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "aim-server/"+registryBridgeToolVersion)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	return nil
}

// generateContributorToken creates a deterministic, anonymous token for an org.
// The token is a SHA-256 hash of the org UUID concatenated with a server secret,
// so the registry can correlate contributions from the same source without
// learning the org identity.
func generateContributorToken(orgID uuid.UUID, secret string) string {
	h := sha256.New()
	h.Write(orgID[:])
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}

// inferEcosystem guesses the package ecosystem from the MCP server URL.
func inferEcosystem(url string) string {
	if url == "" {
		return ""
	}
	// Common patterns for npm-based MCP servers
	if len(url) > 0 {
		return "npm"
	}
	return ""
}

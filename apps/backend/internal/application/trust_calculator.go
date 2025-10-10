package application

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// TrustCalculator implements domain.TrustScoreCalculator
type TrustCalculator struct {
	trustScoreRepo   domain.TrustScoreRepository
	apiKeyRepo       domain.APIKeyRepository
	auditRepo        domain.AuditLogRepository
	capabilityRepo   domain.CapabilityRepository
}

// NewTrustCalculator creates a new trust calculator
func NewTrustCalculator(
	trustScoreRepo domain.TrustScoreRepository,
	apiKeyRepo domain.APIKeyRepository,
	auditRepo domain.AuditLogRepository,
	capabilityRepo domain.CapabilityRepository,
) *TrustCalculator {
	return &TrustCalculator{
		trustScoreRepo:   trustScoreRepo,
		apiKeyRepo:       apiKeyRepo,
		auditRepo:        auditRepo,
		capabilityRepo:   capabilityRepo,
	}
}

// Calculate calculates trust score for an agent
func (c *TrustCalculator) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	factors, err := c.CalculateFactors(agent)
	if err != nil {
		return nil, err
	}

	// Weighted average of factors (9 factors totaling 100%)
	weights := map[string]float64{
		"verification":    0.18, // Identity verification (reduced from 0.20)
		"certificate":     0.12, // Certificate validity (reduced from 0.15)
		"repository":      0.12, // Repository quality (reduced from 0.15)
		"documentation":   0.08, // Documentation score (reduced from 0.10)
		"community":       0.08, // Community trust (reduced from 0.10)
		"security":        0.12, // Security audit (reduced from 0.15)
		"updates":         0.08, // Update frequency (reduced from 0.10)
		"age":             0.05, // Agent age (unchanged)
		"capability_risk": 0.17, // Capability risk (NEW - high importance)
	}

	score := factors.VerificationStatus*weights["verification"] +
		factors.CertificateValidity*weights["certificate"] +
		factors.RepositoryQuality*weights["repository"] +
		factors.DocumentationScore*weights["documentation"] +
		factors.CommunityTrust*weights["community"] +
		factors.SecurityAudit*weights["security"] +
		factors.UpdateFrequency*weights["updates"] +
		factors.AgeScore*weights["age"] +
		factors.CapabilityRisk*weights["capability_risk"]

	// Calculate confidence based on available data
	confidence := c.calculateConfidence(agent, factors)

	return &domain.TrustScore{
		ID:             uuid.New(),
		AgentID:        agent.ID,
		Score:          score,
		Factors:        *factors,
		Confidence:     confidence,
		LastCalculated: time.Now(),
		CreatedAt:      time.Now(),
	}, nil
}

// CalculateFactors calculates individual trust factors
func (c *TrustCalculator) CalculateFactors(agent *domain.Agent) (*domain.TrustScoreFactors, error) {
	factors := &domain.TrustScoreFactors{}

	// 1. Verification Status (0-1)
	factors.VerificationStatus = c.calculateVerificationStatus(agent)

	// 2. Certificate Validity (0-1)
	factors.CertificateValidity = c.calculateCertificateValidity(agent)

	// 3. Repository Quality (0-1)
	factors.RepositoryQuality = c.calculateRepositoryQuality(agent)

	// 4. Documentation Score (0-1)
	factors.DocumentationScore = c.calculateDocumentationScore(agent)

	// 5. Community Trust (0-1)
	factors.CommunityTrust = c.calculateCommunityTrust(agent)

	// 6. Security Audit (0-1)
	factors.SecurityAudit = c.calculateSecurityAudit(agent)

	// 7. Update Frequency (0-1)
	factors.UpdateFrequency = c.calculateUpdateFrequency(agent)

	// 8. Age Score (0-1)
	factors.AgeScore = c.calculateAgeScore(agent)

	// 9. Capability Risk (0-1, where 1 = low risk, 0 = high risk)
	factors.CapabilityRisk = c.calculateCapabilityRisk(agent)

	return factors, nil
}

func (c *TrustCalculator) calculateVerificationStatus(agent *domain.Agent) float64 {
	switch agent.Status {
	case domain.AgentStatusVerified:
		return 1.0
	case domain.AgentStatusPending:
		return 0.3
	case domain.AgentStatusSuspended:
		return 0.1
	case domain.AgentStatusRevoked:
		return 0.0
	default:
		return 0.3
	}
}

func (c *TrustCalculator) calculateCertificateValidity(agent *domain.Agent) float64 {
	// Check if certificate URL is provided
	if agent.CertificateURL == "" {
		return 0.0
	}

	// Check if public key is provided
	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return 0.3
	}

	// Try to parse the public key
	block, _ := pem.Decode([]byte(*agent.PublicKey))
	if block == nil {
		return 0.3
	}

	// Try to parse as X.509 certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0.5
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return 0.2
	}

	// Certificate is valid
	return 1.0
}

func (c *TrustCalculator) calculateRepositoryQuality(agent *domain.Agent) float64 {
	if agent.RepositoryURL == "" {
		return 0.0
	}

	// Check if URL is valid
	parsedURL, err := url.Parse(agent.RepositoryURL)
	if err != nil {
		return 0.0
	}

	score := 0.0

	// Check if it's a known repository hosting service
	host := strings.ToLower(parsedURL.Host)
	if strings.Contains(host, "github.com") ||
		strings.Contains(host, "gitlab.com") ||
		strings.Contains(host, "bitbucket.org") {
		score += 0.5
	}

	// Check if URL is accessible
	resp, err := http.Head(agent.RepositoryURL)
	if err == nil && resp.StatusCode == 200 {
		score += 0.5
	}

	return math.Min(score, 1.0)
}

func (c *TrustCalculator) calculateDocumentationScore(agent *domain.Agent) float64 {
	score := 0.0

	// Has description
	if agent.Description != "" && len(agent.Description) > 50 {
		score += 0.3
	}

	// Has documentation URL
	if agent.DocumentationURL != "" {
		score += 0.3

		// Check if documentation is accessible
		resp, err := http.Head(agent.DocumentationURL)
		if err == nil && resp.StatusCode == 200 {
			score += 0.4
		}
	}

	return math.Min(score, 1.0)
}

func (c *TrustCalculator) calculateCommunityTrust(agent *domain.Agent) float64 {
	// This would integrate with external reputation systems
	// For MVP, return a baseline score
	return 0.5
}

func (c *TrustCalculator) calculateSecurityAudit(agent *domain.Agent) float64 {
	// This would check for security audit reports
	// For MVP, return a baseline score
	return 0.5
}

func (c *TrustCalculator) calculateUpdateFrequency(agent *domain.Agent) float64 {
	// Check how recently the agent was updated
	daysSinceUpdate := time.Since(agent.UpdatedAt).Hours() / 24

	if daysSinceUpdate < 30 {
		return 1.0
	} else if daysSinceUpdate < 90 {
		return 0.7
	} else if daysSinceUpdate < 180 {
		return 0.5
	} else if daysSinceUpdate < 365 {
		return 0.3
	}
	return 0.1
}

func (c *TrustCalculator) calculateAgeScore(agent *domain.Agent) float64 {
	// Older agents are more established
	daysSinceCreation := time.Since(agent.CreatedAt).Hours() / 24

	if daysSinceCreation < 7 {
		return 0.2
	} else if daysSinceCreation < 30 {
		return 0.4
	} else if daysSinceCreation < 90 {
		return 0.6
	} else if daysSinceCreation < 180 {
		return 0.8
	}
	return 1.0
}

func (c *TrustCalculator) calculateCapabilityRisk(agent *domain.Agent) float64 {
	// Start with baseline score (no capabilities detected = neutral risk)
	score := 0.7 // Neutral baseline

	// Get active capabilities for the agent
	capabilities, err := c.capabilityRepo.GetActiveCapabilitiesByAgentID(agent.ID)
	if err != nil || len(capabilities) == 0 {
		return score // No capabilities data = neutral score
	}

	// Define high-risk capability types
	highRiskCapabilities := map[string]float64{
		domain.CapabilityFileDelete:      -0.15, // File deletion is high risk
		domain.CapabilitySystemAdmin:     -0.20, // System admin is very high risk
		domain.CapabilityUserImpersonate: -0.20, // Impersonation is very high risk
		domain.CapabilityDataExport:      -0.10, // Data export is moderate risk
	}

	mediumRiskCapabilities := map[string]float64{
		domain.CapabilityFileWrite:   -0.08,
		domain.CapabilityDBWrite:     -0.08,
		domain.CapabilityAPICall:     -0.05,
	}

	lowRiskCapabilities := map[string]float64{
		domain.CapabilityFileRead:    -0.03,
		domain.CapabilityDBQuery:     -0.03,
		domain.CapabilityMCPToolUse:  -0.02,
	}

	// Calculate risk based on capabilities
	for _, cap := range capabilities {
		// Check high-risk capabilities
		if penalty, exists := highRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		} else if penalty, exists := mediumRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		} else if penalty, exists := lowRiskCapabilities[cap.CapabilityType]; exists {
			score += penalty
		}
	}

	// Get recent violations (last 30 days)
	violations, _, err := c.capabilityRepo.GetViolationsByAgentID(agent.ID, 100, 0)
	if err == nil && len(violations) > 0 {
		// Recent violations significantly impact trust
		recentViolations := 0
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

		for _, violation := range violations {
			if violation.CreatedAt.After(thirtyDaysAgo) {
				recentViolations++

				// Additional penalty based on violation severity
				switch violation.Severity {
				case domain.ViolationSeverityCritical:
					score -= 0.15
				case domain.ViolationSeverityHigh:
					score -= 0.10
				case domain.ViolationSeverityMedium:
					score -= 0.05
				case domain.ViolationSeverityLow:
					score -= 0.02
				}
			}
		}

		// Cap violations penalty
		if recentViolations > 10 {
			score -= 0.20 // Significant violation history
		} else if recentViolations > 5 {
			score -= 0.10
		}
	}

	// Ensure score stays within bounds [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

func (c *TrustCalculator) calculateConfidence(agent *domain.Agent, factors *domain.TrustScoreFactors) float64 {
	// Calculate confidence based on available data
	dataPoints := 0.0
	total := 0.0

	if agent.Status != "" {
		dataPoints++
	}
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		dataPoints++
	}
	if agent.CertificateURL != "" {
		dataPoints++
	}
	if agent.RepositoryURL != "" {
		dataPoints++
	}
	if agent.DocumentationURL != "" {
		dataPoints++
	}
	if agent.Description != "" {
		dataPoints++
	}
	if agent.Version != "" {
		dataPoints++
	}

	total = 7.0 // Total possible data points

	return dataPoints / total
}

// CalculateTrustScore calculates and stores trust score for an agent
func (c *TrustCalculator) CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	// This would normally fetch the agent first
	// For MVP, we'll create a placeholder implementation
	// In production, would fetch agent and call Calculate()

	// For now, return a simple calculated score
	score := &domain.TrustScore{
		ID:             uuid.New(),
		AgentID:        agentID,
		Score:          0.75,
		Confidence:     0.8,
		LastCalculated: time.Now(),
		CreatedAt:      time.Now(),
	}

	// Store the score
	if err := c.trustScoreRepo.Create(score); err != nil {
		return nil, err
	}

	return score, nil
}

// GetLatestTrustScore retrieves the latest trust score for an agent
func (c *TrustCalculator) GetLatestTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	return c.trustScoreRepo.GetLatest(agentID)
}

// GetTrustScoreHistory retrieves trust score history for an agent
func (c *TrustCalculator) GetTrustScoreHistory(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	return c.trustScoreRepo.GetHistory(agentID, limit)
}

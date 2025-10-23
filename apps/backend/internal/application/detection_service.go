package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a/identity/backend/internal/domain"
)

// DetectionService handles MCP detection business logic
type DetectionService struct {
	db                    *sql.DB
	trustCalculator       domain.TrustScoreCalculator // ✅ NEW: For proper trust score calculation
	agentRepo             domain.AgentRepository      // ✅ NEW: For fetching agent data
	deduplicationWindow   time.Duration
}

// NewDetectionService creates a new detection service
func NewDetectionService(
	db *sql.DB,
	trustCalculator domain.TrustScoreCalculator,
	agentRepo domain.AgentRepository,
) *DetectionService {
	// Configure server-side deduplication window based on environment
	// Production: 24 hours (avoid spam, focus on significant changes)
	// Development: 5 minutes (rapid testing and iteration)
	deduplicationWindow := 24 * time.Hour
	if env := os.Getenv("ENVIRONMENT"); env == "development" || env == "dev" {
		deduplicationWindow = 5 * time.Minute
	}

	return &DetectionService{
		db:                    db,
		trustCalculator:       trustCalculator,
		agentRepo:             agentRepo,
		deduplicationWindow:   deduplicationWindow,
	}
}

// ReportDetections processes detection events from SDK or Direct API
//
// Server-Side Intelligent Deduplication Architecture:
// 1. Store EVERY detection in immutable audit table (detections)
// 2. Determine if detection is "significant" based on time window
// 3. Only update aggregated state (agent_mcp_detections) if significant
// 4. Only trigger trust score updates/webhooks/alerts if significant
// 5. Maintain full audit trail for compliance and analytics
func (s *DetectionService) ReportDetections(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
	req *domain.DetectionReportRequest,
) (*domain.DetectionReportResponse, error) {
	// 1. Validate agent belongs to organization
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)`,
		agentID, orgID,
	).Scan(&exists)

	if err != nil || !exists {
		return nil, fmt.Errorf("agent not found or unauthorized")
	}

	newMCPs := []string{}
	existingMCPs := []string{}
	totalProcessed := 0
	significantCount := 0

	// 2. Process each detection
	for _, detection := range req.Detections {
		// Validate detection
		if detection.MCPServer == "" {
			continue // Skip empty server names
		}

		if detection.Confidence < 0 || detection.Confidence > 100 {
			continue // Skip invalid confidence scores
		}

		detailsJSON, _ := json.Marshal(detection.Details)

		// 3. ALWAYS store in audit table (immutable, full trail)
		// This ensures compliance, analytics, and forensic capabilities
		var detectionID uuid.UUID
		err := s.db.QueryRowContext(ctx, `
			INSERT INTO detections (
				agent_id, mcp_server_name, detection_method,
				confidence_score, details, sdk_version,
				is_significant, detected_at
			) VALUES ($1, $2, $3, $4, $5, $6, FALSE, NOW())
			RETURNING id
		`, agentID, detection.MCPServer, detection.DetectionMethod,
			detection.Confidence, detailsJSON, detection.SDKVersion).Scan(&detectionID)

		if err != nil {
			fmt.Printf("Warning: failed to store audit detection for %s: %v\n", detection.MCPServer, err)
			continue
		}

		totalProcessed++

		// 4. Check if this detection is "significant" (server-side deduplication)
		// Query last significant detection for this agent+mcp+method combination
		var lastSignificantAt sql.NullTime
		err = s.db.QueryRowContext(ctx, `
			SELECT detected_at
			FROM detections
			WHERE agent_id = $1
			  AND mcp_server_name = $2
			  AND detection_method = $3
			  AND is_significant = TRUE
			ORDER BY detected_at DESC
			LIMIT 1
		`, agentID, detection.MCPServer, detection.DetectionMethod).Scan(&lastSignificantAt)

		// Determine if this detection is significant
		isSignificant := false
		if err == sql.ErrNoRows {
			// First detection ever - always significant
			isSignificant = true
		} else if err == nil && lastSignificantAt.Valid {
			// Check if enough time has passed since last significant detection
			timeSinceLastSignificant := time.Since(lastSignificantAt.Time)
			if timeSinceLastSignificant >= s.deduplicationWindow {
				isSignificant = true
			}
		} else if err != nil {
			// Query error - be conservative, treat as significant
			fmt.Printf("Warning: failed to check last significant detection: %v\n", err)
			isSignificant = true
		}

		// 5. If significant, mark in audit table and update aggregated state
		if isSignificant {
			// Mark as significant in audit table
			s.db.ExecContext(ctx, `
				UPDATE detections SET is_significant = TRUE WHERE id = $1
			`, detectionID)

			significantCount++

			// Update aggregated state table (agent_mcp_detections)
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO agent_mcp_detections (
					agent_id, mcp_server_name, detection_method,
					confidence_score, details, sdk_version,
					first_detected_at, last_seen_at
				) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
				ON CONFLICT (agent_id, mcp_server_name, detection_method)
				DO UPDATE SET
					last_seen_at = NOW(),
					confidence_score = EXCLUDED.confidence_score,
					details = EXCLUDED.details,
					sdk_version = COALESCE(EXCLUDED.sdk_version, agent_mcp_detections.sdk_version)
			`, agentID, detection.MCPServer, detection.DetectionMethod,
				detection.Confidence, detailsJSON, detection.SDKVersion)

			if err != nil {
				fmt.Printf("Warning: failed to update aggregated state for %s: %v\n", detection.MCPServer, err)
				continue
			}

			// 6. Check if MCP is already in agent's talks_to
			var talksToJSON []byte
			err = s.db.QueryRowContext(ctx,
				`SELECT talks_to FROM agents WHERE id = $1`, agentID,
			).Scan(&talksToJSON)

			if err != nil {
				fmt.Printf("Warning: failed to get agent talks_to: %v\n", err)
				continue
			}

			var talksTo []string
			if len(talksToJSON) > 0 {
				json.Unmarshal(talksToJSON, &talksTo)
			}

			// 7. Add to talks_to if not present
			found := false
			for _, mcp := range talksTo {
				if mcp == detection.MCPServer {
					found = true
					existingMCPs = append(existingMCPs, detection.MCPServer)
					break
				}
			}

			if !found {
				talksTo = append(talksTo, detection.MCPServer)
				updatedJSON, _ := json.Marshal(talksTo)

				_, err = s.db.ExecContext(ctx,
					`UPDATE agents SET talks_to = $1, updated_at = NOW() WHERE id = $2`,
					updatedJSON, agentID)

				if err == nil {
					newMCPs = append(newMCPs, detection.MCPServer)
				} else {
					fmt.Printf("Warning: failed to update talks_to for %s: %v\n", detection.MCPServer, err)
				}
			}

			// 8. Update SDK installation heartbeat if SDK detection
			if detection.SDKVersion != "" {
				s.updateSDKHeartbeat(ctx, agentID, detection.SDKVersion)
			}
		}
	}

	// Deduplicate newMCPs and existingMCPs
	newMCPs = deduplicateSlice(newMCPs)
	existingMCPs = deduplicateSlice(existingMCPs)

	return &domain.DetectionReportResponse{
		Success:             true,
		DetectionsProcessed: totalProcessed,
		NewMCPs:             newMCPs,
		ExistingMCPs:        existingMCPs,
		Message:             fmt.Sprintf("Processed %d detections (%d significant, %d filtered)", totalProcessed, significantCount, totalProcessed-significantCount),
	}, nil
}

// updateSDKHeartbeat updates the SDK installation heartbeat timestamp
func (s *DetectionService) updateSDKHeartbeat(ctx context.Context, agentID uuid.UUID, sdkVersion string) {
	// Try to update existing SDK installation
	result, err := s.db.ExecContext(ctx, `
		UPDATE sdk_installations
		SET last_heartbeat_at = NOW(), updated_at = NOW()
		WHERE agent_id = $1
	`, agentID)

	if err != nil {
		return // Silent failure
	}

	// If no rows updated, insert new SDK installation
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Extract language from SDK version if possible (or default to "unknown")
		sdkLanguage := "javascript" // Default, can be improved with version parsing

		s.db.ExecContext(ctx, `
			INSERT INTO sdk_installations (
				agent_id, sdk_language, sdk_version,
				installed_at, last_heartbeat_at, auto_detect_enabled
			) VALUES ($1, $2, $3, NOW(), NOW(), TRUE)
			ON CONFLICT (agent_id) DO UPDATE SET
				last_heartbeat_at = NOW(),
				sdk_version = EXCLUDED.sdk_version,
				updated_at = NOW()
		`, agentID, sdkLanguage, sdkVersion)
	}
}

// GetDetectionStatus returns the current detection status for an agent
func (s *DetectionService) GetDetectionStatus(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
) (*domain.DetectionStatusResponse, error) {
	// 1. Validate agent belongs to organization
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)`,
		agentID, orgID,
	).Scan(&exists)

	if err != nil || !exists {
		return nil, fmt.Errorf("agent not found or unauthorized")
	}

	response := &domain.DetectionStatusResponse{
		AgentID:      agentID,
		SDKInstalled: false,
		DetectedMCPs: []domain.DetectedMCPSummary{},
	}

	// 2. Check SDK installation
	var sdk domain.SDKInstallation
	err = s.db.QueryRowContext(ctx, `
		SELECT sdk_version, auto_detect_enabled, last_heartbeat_at
		FROM sdk_installations
		WHERE agent_id = $1
	`, agentID).Scan(&sdk.SDKVersion, &sdk.AutoDetectEnabled, &sdk.LastHeartbeatAt)

	if err == nil {
		response.SDKInstalled = true
		response.SDKVersion = sdk.SDKVersion
		response.AutoDetectEnabled = sdk.AutoDetectEnabled
		response.LastReportedAt = &sdk.LastHeartbeatAt
	}

	// 3. Get the most recent protocol from verification events
	// SDK auto-detects protocol and sends it with each verification request
	var protocol sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT protocol
		FROM verification_events
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, agentID).Scan(&protocol)

	if err == nil && protocol.Valid {
		response.Protocol = protocol.String
	}

	// 4. Get ALL connected MCPs from talks_to with their detection metadata
	// This query shows all servers in Connections tab, enriched with detection data
	rows, err := s.db.QueryContext(ctx, `
		WITH connected_mcps AS (
			SELECT jsonb_array_elements_text(talks_to) as mcp_name
			FROM agents WHERE id = $1
		)
		SELECT
			t.mcp_name,
			COALESCE(ARRAY_AGG(DISTINCT d.detection_method::text) FILTER (WHERE d.detection_method IS NOT NULL), ARRAY['manual']::text[]) as methods,
			COALESCE(AVG(d.confidence_score), 0) as avg_confidence,
			MIN(d.first_detected_at) as first_detected,
			MAX(d.last_seen_at) as last_seen,
			CASE WHEN COUNT(d.mcp_server_name) = 0 THEN true ELSE false END as is_manual
		FROM connected_mcps t
		LEFT JOIN agent_mcp_detections d
			ON d.agent_id = $1 AND d.mcp_server_name = t.mcp_name
		GROUP BY t.mcp_name
		ORDER BY is_manual ASC, last_seen DESC NULLS LAST
	`, agentID)

	if err != nil {
		return response, nil // Return partial response
	}
	defer rows.Close()

	for rows.Next() {
		var mcp domain.DetectedMCPSummary
		var methods []string
		var isManual bool
		var firstDetectedNull, lastSeenNull sql.NullTime

		err := rows.Scan(&mcp.Name, pq.Array(&methods), &mcp.ConfidenceScore,
			&firstDetectedNull, &lastSeenNull, &isManual)
		if err != nil {
			continue
		}

		// Handle nullable timestamps
		if firstDetectedNull.Valid {
			mcp.FirstDetected = firstDetectedNull.Time
		}
		if lastSeenNull.Valid {
			mcp.LastSeen = lastSeenNull.Time
		}

		// Convert methods to DetectionMethod type
		for _, m := range methods {
			mcp.DetectedBy = append(mcp.DetectedBy, domain.DetectionMethod(m))
		}

		// Boost confidence if multiple detection methods (only for auto-detected)
		if !isManual {
			methodCount := len(mcp.DetectedBy)
			if methodCount >= 2 {
				mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore+10)
			}
			if methodCount >= 3 {
				mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore+20)
			}
		}

		response.DetectedMCPs = append(response.DetectedMCPs, mcp)
	}

	return response, nil
}

// ReportCapabilities processes agent capability detection reports from SDK
func (s *DetectionService) ReportCapabilities(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
	req *domain.AgentCapabilityReport,
) (*domain.CapabilityReportResponse, error) {
	// 1. Validate agent belongs to organization
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)`,
		agentID, orgID,
	).Scan(&exists)

	if err != nil || !exists {
		return nil, fmt.Errorf("agent not found or unauthorized")
	}

	// 2. Fetch full agent entity for comprehensive trust calculation
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent: %v", err)
	}

	// 3. Calculate trust score using proper 9-factor algorithm (includes capability risk)
	// This replaces the naive addition/subtraction with comprehensive risk assessment
	trustScore, err := s.trustCalculator.Calculate(agent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trust score: %v", err)
	}

	// Trust score is already on 0-1 scale (0.0-1.0), use it directly
	// Database constraints enforce 0.0-1.0 range
	newTrustScore := trustScore.Score

	// 4. Convert capability report to JSON
	envJSON, _ := json.Marshal(req.Environment)
	aiModelsJSON, _ := json.Marshal(req.AIModels)
	capabilitiesJSON, _ := json.Marshal(req.Capabilities)
	riskAssessmentJSON, _ := json.Marshal(req.RiskAssessment)

	// 5. Store capability report in database
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_capability_reports (
			agent_id, detected_at, environment, ai_models,
			capabilities, risk_assessment, risk_level,
			overall_risk_score, trust_score_impact
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, agentID, req.DetectedAt, envJSON, aiModelsJSON,
		capabilitiesJSON, riskAssessmentJSON, req.RiskAssessment.RiskLevel,
		req.RiskAssessment.OverallRiskScore, req.RiskAssessment.TrustScoreImpact)

	if err != nil {
		return nil, fmt.Errorf("failed to store capability report: %v", err)
	}

	// 6. Store trust score in trust_scores table for historical tracking
	factorsJSON, _ := json.Marshal(trustScore.Factors)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO trust_scores (
			agent_id, score, factors, confidence, last_calculated
		) VALUES ($1, $2, $3, $4, NOW())
	`, agentID, trustScore.Score, factorsJSON, trustScore.Confidence)

	if err != nil {
		return nil, fmt.Errorf("failed to store trust score: %v", err)
	}

	// 7. Update agent trust score (keep agents table in sync)
	_, err = s.db.ExecContext(ctx, `
		UPDATE agents
		SET trust_score = $1, updated_at = NOW()
		WHERE id = $2
	`, newTrustScore, agentID)

	if err != nil {
		return nil, fmt.Errorf("failed to update agent trust score: %v", err)
	}

	// 8. Create security alerts for CRITICAL and HIGH severity issues
	for _, alert := range req.RiskAssessment.Alerts {
		if alert.Severity == "CRITICAL" || alert.Severity == "HIGH" {
			// Store alert in database
			s.db.ExecContext(ctx, `
				INSERT INTO security_alerts (
					organization_id, agent_id, severity,
					alert_type, message, metadata, acknowledged
				) VALUES ($1, $2, $3, $4, $5, $6, FALSE)
			`, orgID, agentID, alert.Severity, "capability_risk",
				alert.Message, fmt.Sprintf(`{"capability": "%s", "recommendation": "%s"}`, alert.Capability, alert.Recommendation))
		}
	}

	return &domain.CapabilityReportResponse{
		Success:            true,
		AgentID:            agentID,
		RiskLevel:          req.RiskAssessment.RiskLevel,
		TrustScoreImpact:   req.RiskAssessment.TrustScoreImpact,
		NewTrustScore:      newTrustScore,
		SecurityAlertsCount: countHighSeverityAlerts(req.RiskAssessment.Alerts),
		Message:            fmt.Sprintf("Capability report processed. Risk: %s, Trust impact: %d", req.RiskAssessment.RiskLevel, req.RiskAssessment.TrustScoreImpact),
	}, nil
}

// GetLatestCapabilityReport fetches the most recent capability report for an agent
func (s *DetectionService) GetLatestCapabilityReport(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
) (*domain.AgentCapabilityReport, error) {
	// Verify agent exists and belongs to organization
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agents
		WHERE id = $1 AND organization_id = $2
	`, agentID, orgID).Scan(&count)

	if err != nil {
		return nil, fmt.Errorf("failed to verify agent: %w", err)
	}
	if count == 0 {
		return nil, fmt.Errorf("agent not found")
	}

	// Query latest capability report
	var (
		detectedAt        time.Time
		environmentJSON   []byte
		aiModelsJSON      []byte
		capabilitiesJSON  []byte
		riskAssessmentJSON []byte
	)

	query := `
		SELECT
			detected_at,
			environment,
			ai_models,
			capabilities,
			risk_assessment
		FROM agent_capability_reports
		WHERE agent_id = $1
		ORDER BY detected_at DESC
		LIMIT 1
	`

	err = s.db.QueryRowContext(ctx, query, agentID).Scan(
		&detectedAt,
		&environmentJSON,
		&aiModelsJSON,
		&capabilitiesJSON,
		&riskAssessmentJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no capability reports found for this agent")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch capability report: %w", err)
	}

	// Parse JSON fields
	var environment domain.ProgrammingEnvironment
	if err := json.Unmarshal(environmentJSON, &environment); err != nil {
		return nil, fmt.Errorf("failed to parse environment: %w", err)
	}

	var aiModels []domain.AIModelUsage
	if err := json.Unmarshal(aiModelsJSON, &aiModels); err != nil {
		return nil, fmt.Errorf("failed to parse ai models: %w", err)
	}

	var capabilities domain.AgentCapabilities
	if err := json.Unmarshal(capabilitiesJSON, &capabilities); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities: %w", err)
	}

	var riskAssessment domain.RiskAssessment
	if err := json.Unmarshal(riskAssessmentJSON, &riskAssessment); err != nil {
		return nil, fmt.Errorf("failed to parse risk assessment: %w", err)
	}

	return &domain.AgentCapabilityReport{
		DetectedAt:     detectedAt.Format(time.RFC3339),
		Environment:    environment,
		AIModels:       aiModels,
		Capabilities:   capabilities,
		RiskAssessment: riskAssessment,
	}, nil
}

// countHighSeverityAlerts counts CRITICAL and HIGH severity alerts
func countHighSeverityAlerts(alerts []domain.SecurityAlert) int {
	count := 0
	for _, alert := range alerts {
		if alert.Severity == "CRITICAL" || alert.Severity == "HIGH" {
			count++
		}
	}
	return count
}

// Helper functions

func deduplicateSlice(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

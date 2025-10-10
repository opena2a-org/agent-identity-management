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
	deduplicationWindow   time.Duration
}

// NewDetectionService creates a new detection service
func NewDetectionService(db *sql.DB) *DetectionService {
	// Configure server-side deduplication window based on environment
	// Production: 24 hours (avoid spam, focus on significant changes)
	// Development: 5 minutes (rapid testing and iteration)
	deduplicationWindow := 24 * time.Hour
	if env := os.Getenv("ENVIRONMENT"); env == "development" || env == "dev" {
		deduplicationWindow = 5 * time.Minute
	}

	return &DetectionService{
		db:                    db,
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

			// TODO: Trigger trust score recalculation for significant detections
			// TODO: Send webhooks for significant detections
			// TODO: Create security alerts for anomalous detections
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

	// 3. Get detected MCPs with aggregated confidence
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			mcp_server_name,
			ARRAY_AGG(DISTINCT detection_method::text) as methods,
			AVG(confidence_score) as avg_confidence,
			MIN(first_detected_at) as first_detected,
			MAX(last_seen_at) as last_seen
		FROM agent_mcp_detections
		WHERE agent_id = $1
		GROUP BY mcp_server_name
		ORDER BY last_seen DESC
	`, agentID)

	if err != nil {
		return response, nil // Return partial response
	}
	defer rows.Close()

	for rows.Next() {
		var mcp domain.DetectedMCPSummary
		var methods []string

		err := rows.Scan(&mcp.Name, pq.Array(&methods), &mcp.ConfidenceScore,
			&mcp.FirstDetected, &mcp.LastSeen)
		if err != nil {
			continue
		}

		// Convert methods to DetectionMethod type
		for _, m := range methods {
			mcp.DetectedBy = append(mcp.DetectedBy, domain.DetectionMethod(m))
		}

		// Boost confidence if multiple methods
		methodCount := len(mcp.DetectedBy)
		if methodCount >= 2 {
			mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore+10)
		}
		if methodCount >= 3 {
			mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore+20)
		}

		response.DetectedMCPs = append(response.DetectedMCPs, mcp)
	}

	return response, nil
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

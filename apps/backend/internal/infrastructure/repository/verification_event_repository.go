package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationEventRepositorySimple implements the VerificationEventRepository interface using standard sql.DB
type VerificationEventRepositorySimple struct {
	db *sql.DB
}

// NewVerificationEventRepository creates a new verification event repository
func NewVerificationEventRepository(db *sql.DB) *VerificationEventRepositorySimple {
	return &VerificationEventRepositorySimple{db: db}
}

// Create inserts a new verification event
func (r *VerificationEventRepositorySimple) Create(event *domain.VerificationEvent) error {
	query := `
		INSERT INTO verification_events (
			organization_id, agent_id, agent_name, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, details, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
		) RETURNING id, created_at`

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return r.db.QueryRow(
		query,
		event.OrganizationID, event.AgentID, event.AgentName, event.Protocol, event.VerificationType,
		event.Status, event.Result, event.Signature, event.MessageHash, event.Nonce, event.PublicKey,
		event.Confidence, event.TrustScore, event.DurationMs, event.ErrorCode, event.ErrorReason,
		event.InitiatorType, event.InitiatorID, event.InitiatorName, event.InitiatorIP,
		event.Action, event.ResourceType, event.ResourceID, event.Location,
		event.StartedAt, event.CompletedAt, event.Details, metadataJSON,
	).Scan(&event.ID, &event.CreatedAt)
}

// GetByID retrieves a verification event by ID
func (r *VerificationEventRepositorySimple) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	query := `
		SELECT id, organization_id, agent_id, agent_name, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, created_at, details, metadata
		FROM verification_events WHERE id = $1`

	event := &domain.VerificationEvent{}
	var agentID uuid.NullUUID
	var agentName sql.NullString
	var resultStr, signature, messageHash, nonce, publicKey, errorCode, errorReason sql.NullString
	var initiatorType sql.NullString
	var initiatorID uuid.NullUUID
	var initiatorName, initiatorIP, action, resourceType, resourceID, location, details sql.NullString
	var completedAt sql.NullTime
	var metadataJSON []byte

	err := r.db.QueryRow(query, id).Scan(
		&event.ID, &event.OrganizationID, &agentID, &agentName,
		&event.Protocol, &event.VerificationType, &event.Status, &resultStr,
		&signature, &messageHash, &nonce, &publicKey,
		&event.Confidence, &event.TrustScore, &event.DurationMs, &errorCode,
		&errorReason, &initiatorType, &initiatorID, &initiatorName,
		&initiatorIP, &action, &resourceType, &resourceID,
		&location, &event.StartedAt, &completedAt, &event.CreatedAt,
		&details, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	// Convert nullable fields
	if agentID.Valid {
		event.AgentID = &agentID.UUID
	}
	if agentName.Valid {
		event.AgentName = &agentName.String
	}
	// Set InitiatorType with default if NULL
	if initiatorType.Valid {
		event.InitiatorType = domain.InitiatorType(initiatorType.String)
	} else {
		event.InitiatorType = domain.InitiatorTypeSystem // Default for NULL values
	}
	if resultStr.Valid {
		result := domain.VerificationResult(resultStr.String)
		event.Result = &result
	}
	if signature.Valid {
		event.Signature = &signature.String
	}
	if messageHash.Valid {
		event.MessageHash = &messageHash.String
	}
	if nonce.Valid {
		event.Nonce = &nonce.String
	}
	if publicKey.Valid {
		event.PublicKey = &publicKey.String
	}
	if errorCode.Valid {
		event.ErrorCode = &errorCode.String
	}
	if errorReason.Valid {
		event.ErrorReason = &errorReason.String
	}
	if initiatorID.Valid {
		event.InitiatorID = &initiatorID.UUID
	}
	if initiatorName.Valid {
		event.InitiatorName = &initiatorName.String
	}
	if initiatorIP.Valid {
		event.InitiatorIP = &initiatorIP.String
	}
	if action.Valid {
		event.Action = &action.String
	}
	if resourceType.Valid {
		event.ResourceType = &resourceType.String
	}
	if resourceID.Valid {
		event.ResourceID = &resourceID.String
	}
	if location.Valid {
		event.Location = &location.String
	}
	if completedAt.Valid {
		event.CompletedAt = &completedAt.Time
	}
	if details.Valid {
		event.Details = &details.String
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return event, nil
}

// GetByOrganization retrieves verification events for an organization with pagination
func (r *VerificationEventRepositorySimple) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM verification_events WHERE organization_id = $1`
	if err := r.db.QueryRow(countQuery, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get events
	query := `
		SELECT id, organization_id, agent_id, agent_name, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, created_at, details, metadata
		FROM verification_events
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*domain.VerificationEvent
	for rows.Next() {
		event := &domain.VerificationEvent{}
		var agentID uuid.NullUUID
		var agentName sql.NullString
		var resultStr, signature, messageHash, nonce, publicKey, errorCode, errorReason sql.NullString
		var initiatorType sql.NullString
		var initiatorID uuid.NullUUID
		var initiatorName, initiatorIP, action, resourceType, resourceID, location, details sql.NullString
		var completedAt sql.NullTime
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.OrganizationID, &agentID, &agentName,
			&event.Protocol, &event.VerificationType, &event.Status, &resultStr,
			&signature, &messageHash, &nonce, &publicKey,
			&event.Confidence, &event.TrustScore, &event.DurationMs, &errorCode,
			&errorReason, &initiatorType, &initiatorID, &initiatorName,
			&initiatorIP, &action, &resourceType, &resourceID,
			&location, &event.StartedAt, &completedAt, &event.CreatedAt,
			&details, &metadataJSON,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert nullable fields (same as GetByID)
		if agentID.Valid {
			event.AgentID = &agentID.UUID
		}
		if agentName.Valid {
			event.AgentName = &agentName.String
		}
		if initiatorType.Valid {
			event.InitiatorType = domain.InitiatorType(initiatorType.String)
		} else {
			event.InitiatorType = domain.InitiatorTypeSystem
		}
		if resultStr.Valid {
			result := domain.VerificationResult(resultStr.String)
			event.Result = &result
		}
		if signature.Valid {
			event.Signature = &signature.String
		}
		if messageHash.Valid {
			event.MessageHash = &messageHash.String
		}
		if nonce.Valid {
			event.Nonce = &nonce.String
		}
		if publicKey.Valid {
			event.PublicKey = &publicKey.String
		}
		if errorCode.Valid {
			event.ErrorCode = &errorCode.String
		}
		if errorReason.Valid {
			event.ErrorReason = &errorReason.String
		}
		if initiatorID.Valid {
			event.InitiatorID = &initiatorID.UUID
		}
		if initiatorName.Valid {
			event.InitiatorName = &initiatorName.String
		}
		if initiatorIP.Valid {
			event.InitiatorIP = &initiatorIP.String
		}
		if action.Valid {
			event.Action = &action.String
		}
		if resourceType.Valid {
			event.ResourceType = &resourceType.String
		}
		if resourceID.Valid {
			event.ResourceID = &resourceID.String
		}
		if location.Valid {
			event.Location = &location.String
		}
		if completedAt.Valid {
			event.CompletedAt = &completedAt.Time
		}
		if details.Valid {
			event.Details = &details.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	return events, total, rows.Err()
}

// GetByAgent retrieves verification events for a specific agent
func (r *VerificationEventRepositorySimple) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM verification_events WHERE agent_id = $1`
	if err := r.db.QueryRow(countQuery, agentID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get events
	query := `
		SELECT id, organization_id, agent_id, agent_name, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, created_at, details, metadata
		FROM verification_events
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, agentID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*domain.VerificationEvent
	for rows.Next() {
		event := &domain.VerificationEvent{}
		var agentID uuid.NullUUID
		var agentName sql.NullString
		var resultStr, signature, messageHash, nonce, publicKey, errorCode, errorReason sql.NullString
		var initiatorType sql.NullString
		var initiatorID uuid.NullUUID
		var initiatorName, initiatorIP, action, resourceType, resourceID, location, details sql.NullString
		var completedAt sql.NullTime
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.OrganizationID, &agentID, &agentName,
			&event.Protocol, &event.VerificationType, &event.Status, &resultStr,
			&signature, &messageHash, &nonce, &publicKey,
			&event.Confidence, &event.TrustScore, &event.DurationMs, &errorCode,
			&errorReason, &initiatorType, &initiatorID, &initiatorName,
			&initiatorIP, &action, &resourceType, &resourceID,
			&location, &event.StartedAt, &completedAt, &event.CreatedAt,
			&details, &metadataJSON,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert nullable fields (same pattern)
		if agentID.Valid {
			event.AgentID = &agentID.UUID
		}
		if agentName.Valid {
			event.AgentName = &agentName.String
		}
		if initiatorType.Valid {
			event.InitiatorType = domain.InitiatorType(initiatorType.String)
		} else {
			event.InitiatorType = domain.InitiatorTypeSystem
		}
		if resultStr.Valid {
			result := domain.VerificationResult(resultStr.String)
			event.Result = &result
		}
		if signature.Valid {
			event.Signature = &signature.String
		}
		if messageHash.Valid {
			event.MessageHash = &messageHash.String
		}
		if nonce.Valid {
			event.Nonce = &nonce.String
		}
		if publicKey.Valid {
			event.PublicKey = &publicKey.String
		}
		if errorCode.Valid {
			event.ErrorCode = &errorCode.String
		}
		if errorReason.Valid {
			event.ErrorReason = &errorReason.String
		}
		if initiatorID.Valid {
			event.InitiatorID = &initiatorID.UUID
		}
		if initiatorName.Valid {
			event.InitiatorName = &initiatorName.String
		}
		if initiatorIP.Valid {
			event.InitiatorIP = &initiatorIP.String
		}
		if action.Valid {
			event.Action = &action.String
		}
		if resourceType.Valid {
			event.ResourceType = &resourceType.String
		}
		if resourceID.Valid {
			event.ResourceID = &resourceID.String
		}
		if location.Valid {
			event.Location = &location.String
		}
		if completedAt.Valid {
			event.CompletedAt = &completedAt.Time
		}
		if details.Valid {
			event.Details = &details.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	return events, total, rows.Err()
}

// GetByMCPServer retrieves all verification events for an MCP server with pagination
func (r *VerificationEventRepositorySimple) GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM verification_events WHERE mcp_server_id = $1`
	if err := r.db.QueryRow(countQuery, mcpServerID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get events
	query := `
		SELECT id, organization_id, agent_id, agent_name, mcp_server_id, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, created_at, details, metadata
		FROM verification_events
		WHERE mcp_server_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, mcpServerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*domain.VerificationEvent
	for rows.Next() {
		event := &domain.VerificationEvent{}
		var agentID, mcpServerID uuid.NullUUID
		var agentName sql.NullString
		var resultStr, signature, messageHash, nonce, publicKey, errorCode, errorReason sql.NullString
		var initiatorType sql.NullString
		var initiatorID uuid.NullUUID
		var initiatorName, initiatorIP, action, resourceType, resourceID, location, details sql.NullString
		var completedAt sql.NullTime
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.OrganizationID, &agentID, &agentName, &mcpServerID,
			&event.Protocol, &event.VerificationType, &event.Status, &resultStr,
			&signature, &messageHash, &nonce, &publicKey,
			&event.Confidence, &event.TrustScore, &event.DurationMs, &errorCode,
			&errorReason, &initiatorType, &initiatorID, &initiatorName,
			&initiatorIP, &action, &resourceType, &resourceID,
			&location, &event.StartedAt, &completedAt, &event.CreatedAt,
			&details, &metadataJSON,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert nullable fields
		if agentID.Valid {
			event.AgentID = &agentID.UUID
		}
		if agentName.Valid {
			event.AgentName = &agentName.String
		}
		if mcpServerID.Valid {
			event.MCPServerID = &mcpServerID.UUID
		}
		if initiatorType.Valid {
			event.InitiatorType = domain.InitiatorType(initiatorType.String)
		} else {
			event.InitiatorType = domain.InitiatorTypeSystem
		}
		if resultStr.Valid {
			result := domain.VerificationResult(resultStr.String)
			event.Result = &result
		}
		if signature.Valid {
			event.Signature = &signature.String
		}
		if messageHash.Valid {
			event.MessageHash = &messageHash.String
		}
		if nonce.Valid {
			event.Nonce = &nonce.String
		}
		if publicKey.Valid {
			event.PublicKey = &publicKey.String
		}
		if errorCode.Valid {
			event.ErrorCode = &errorCode.String
		}
		if errorReason.Valid {
			event.ErrorReason = &errorReason.String
		}
		if initiatorID.Valid {
			event.InitiatorID = &initiatorID.UUID
		}
		if initiatorName.Valid {
			event.InitiatorName = &initiatorName.String
		}
		if initiatorIP.Valid {
			event.InitiatorIP = &initiatorIP.String
		}
		if action.Valid {
			event.Action = &action.String
		}
		if resourceType.Valid {
			event.ResourceType = &resourceType.String
		}
		if resourceID.Valid {
			event.ResourceID = &resourceID.String
		}
		if location.Valid {
			event.Location = &location.String
		}
		if completedAt.Valid {
			event.CompletedAt = &completedAt.Time
		}
		if details.Valid {
			event.Details = &details.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	return events, total, rows.Err()
}

// GetRecentEvents retrieves events from the last N minutes
func (r *VerificationEventRepositorySimple) GetRecentEvents(orgID uuid.UUID, minutes int) ([]*domain.VerificationEvent, error) {
	query := `
		SELECT id, organization_id, agent_id, agent_name, protocol, verification_type,
			status, result, signature, message_hash, nonce, public_key,
			confidence, trust_score, duration_ms, error_code, error_reason,
			initiator_type, initiator_id, initiator_name, initiator_ip,
			action, resource_type, resource_id, location,
			started_at, completed_at, created_at, details, metadata
		FROM verification_events
		WHERE organization_id = $1
		AND created_at >= NOW() - INTERVAL '1 minute' * $2
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, orgID, minutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*domain.VerificationEvent
	for rows.Next() {
		event := &domain.VerificationEvent{}
		var agentID uuid.NullUUID
		var agentName sql.NullString
		var resultStr, signature, messageHash, nonce, publicKey, errorCode, errorReason sql.NullString
		var initiatorType sql.NullString
		var initiatorID uuid.NullUUID
		var initiatorName, initiatorIP, action, resourceType, resourceID, location, details sql.NullString
		var completedAt sql.NullTime
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.OrganizationID, &agentID, &agentName,
			&event.Protocol, &event.VerificationType, &event.Status, &resultStr,
			&signature, &messageHash, &nonce, &publicKey,
			&event.Confidence, &event.TrustScore, &event.DurationMs, &errorCode,
			&errorReason, &initiatorType, &initiatorID, &initiatorName,
			&initiatorIP, &action, &resourceType, &resourceID,
			&location, &event.StartedAt, &completedAt, &event.CreatedAt,
			&details, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullable fields
		if agentID.Valid {
			event.AgentID = &agentID.UUID
		}
		if agentName.Valid {
			event.AgentName = &agentName.String
		}
		if initiatorType.Valid {
			event.InitiatorType = domain.InitiatorType(initiatorType.String)
		} else {
			event.InitiatorType = domain.InitiatorTypeSystem
		}
		if resultStr.Valid {
			result := domain.VerificationResult(resultStr.String)
			event.Result = &result
		}
		if signature.Valid {
			event.Signature = &signature.String
		}
		if messageHash.Valid {
			event.MessageHash = &messageHash.String
		}
		if nonce.Valid {
			event.Nonce = &nonce.String
		}
		if publicKey.Valid {
			event.PublicKey = &publicKey.String
		}
		if errorCode.Valid {
			event.ErrorCode = &errorCode.String
		}
		if errorReason.Valid {
			event.ErrorReason = &errorReason.String
		}
		if initiatorID.Valid {
			event.InitiatorID = &initiatorID.UUID
		}
		if initiatorName.Valid {
			event.InitiatorName = &initiatorName.String
		}
		if initiatorIP.Valid {
			event.InitiatorIP = &initiatorIP.String
		}
		if action.Valid {
			event.Action = &action.String
		}
		if resourceType.Valid {
			event.ResourceType = &resourceType.String
		}
		if resourceID.Valid {
			event.ResourceID = &resourceID.String
		}
		if location.Valid {
			event.Location = &location.String
		}
		if completedAt.Valid {
			event.CompletedAt = &completedAt.Time
		}
		if details.Valid {
			event.Details = &details.String
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetStatistics calculates aggregated statistics for a time range
func (r *VerificationEventRepositorySimple) GetStatistics(orgID uuid.UUID, startTime, endTime time.Time) (*domain.VerificationStatistics, error) {
	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed_count,
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending_count,
			COALESCE(SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END), 0) as timeout_count,
			AVG(duration_ms) as avg_duration,
			AVG(confidence) as avg_confidence,
			AVG(trust_score) as avg_trust_score,
			COUNT(DISTINCT agent_id) as unique_agents
		FROM verification_events
		WHERE organization_id = $1
		AND created_at BETWEEN $2 AND $3`

	var total, successCount, failedCount, pendingCount, timeoutCount, uniqueAgents int
	var avgDuration, avgConfidence, avgTrustScore sql.NullFloat64

	err := r.db.QueryRow(query, orgID, startTime, endTime).Scan(
		&total, &successCount, &failedCount, &pendingCount, &timeoutCount,
		&avgDuration, &avgConfidence, &avgTrustScore, &uniqueAgents,
	)
	if err != nil {
		return nil, err
	}

	// Convert nullable averages to float64 (default to 0 if NULL)
	avgDurationVal := 0.0
	if avgDuration.Valid {
		avgDurationVal = avgDuration.Float64
	}
	avgConfidenceVal := 0.0
	if avgConfidence.Valid {
		avgConfidenceVal = avgConfidence.Float64
	}
	avgTrustScoreVal := 0.0
	if avgTrustScore.Valid {
		avgTrustScoreVal = avgTrustScore.Float64
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(successCount) / float64(total) * 100
	}

	duration := endTime.Sub(startTime).Minutes()
	verificationsPerMinute := 0.0
	if duration > 0 {
		verificationsPerMinute = float64(total) / duration
	}

	// Get protocol distribution
	protocolDist := make(map[string]int)
	protocolQuery := `
		SELECT protocol, COUNT(*) as count
		FROM verification_events
		WHERE organization_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY protocol`

	protocolRows, err := r.db.Query(protocolQuery, orgID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer protocolRows.Close()

	for protocolRows.Next() {
		var protocol string
		var count int
		if err := protocolRows.Scan(&protocol, &count); err != nil {
			return nil, err
		}
		protocolDist[protocol] = count
	}

	// Get type distribution
	typeDist := make(map[string]int)
	typeQuery := `
		SELECT verification_type, COUNT(*) as count
		FROM verification_events
		WHERE organization_id = $1 AND created_at BETWEEN $2 AND $3
		GROUP BY verification_type`

	typeRows, err := r.db.Query(typeQuery, orgID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var vtype string
		var count int
		if err := typeRows.Scan(&vtype, &count); err != nil {
			return nil, err
		}
		typeDist[vtype] = count
	}

	// Get initiator distribution
	initiatorDist := make(map[string]int)
	initiatorQuery := `
		SELECT initiator_type, COUNT(*) as count
		FROM verification_events
		WHERE organization_id = $1 AND created_at BETWEEN $2 AND $3
		AND initiator_type IS NOT NULL
		GROUP BY initiator_type`

	initiatorRows, err := r.db.Query(initiatorQuery, orgID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer initiatorRows.Close()

	for initiatorRows.Next() {
		var initiator string
		var count int
		if err := initiatorRows.Scan(&initiator, &count); err != nil {
			return nil, err
		}
		initiatorDist[initiator] = count
	}

	return &domain.VerificationStatistics{
		TotalVerifications:     total,
		SuccessCount:           successCount,
		FailedCount:            failedCount,
		PendingCount:           pendingCount,
		TimeoutCount:           timeoutCount,
		SuccessRate:            successRate,
		AvgDurationMs:          avgDurationVal,
		AvgConfidence:          avgConfidenceVal,
		AvgTrustScore:          avgTrustScoreVal,
		VerificationsPerMinute: verificationsPerMinute,
		UniqueAgentsVerified:   uniqueAgents,
		ProtocolDistribution:   protocolDist,
		TypeDistribution:       typeDist,
		InitiatorDistribution:  initiatorDist,
	}, nil
}

// UpdateResult updates the result of a verification event
func (r *VerificationEventRepositorySimple) UpdateResult(id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
	// Merge new metadata with existing metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	resultStr := string(result)
	status := "failed"
	if resultStr == string(domain.VerificationResultVerified) {
		status = "success"
	} else if resultStr == string(domain.VerificationResultDenied) {
		status = "failed"
	}

	query := `
    UPDATE verification_events
    SET
        result = $1,
        status = $2,
        error_reason = COALESCE($3, error_reason),
        metadata = COALESCE($4::jsonb, metadata),
        completed_at = CASE
            WHEN completed_at IS NULL THEN NOW()
            ELSE completed_at
        END
    WHERE id = $5`

	execResult, err := r.db.Exec(query, resultStr, status, reason, metadataJSON, id)
	if err != nil {
		return err
	}

	rowsAffected, err := execResult.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("verification event not found")
	}

	return nil
}

// Delete removes a verification event
func (r *VerificationEventRepositorySimple) Delete(id uuid.UUID) error {
	query := `DELETE FROM verification_events WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

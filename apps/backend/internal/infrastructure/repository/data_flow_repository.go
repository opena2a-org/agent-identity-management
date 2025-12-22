package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a/identity/backend/internal/domain"
)

// DataFlowRepository implements domain.DataFlowRepository with PostgreSQL
type DataFlowRepository struct {
	db *sql.DB
}

// NewDataFlowRepository creates a new DataFlowRepository
func NewDataFlowRepository(db *sql.DB) *DataFlowRepository {
	return &DataFlowRepository{db: db}
}

// ============================================================================
// DATA FLOWS
// ============================================================================

// CreateFlow inserts a new data flow event
func (r *DataFlowRepository) CreateFlow(flow *domain.DataFlow) error {
	sourceDetails, _ := json.Marshal(flow.SourceDetails)
	destDetails, _ := json.Marshal(flow.DestinationDetails)
	metadata, _ := json.Marshal(flow.Metadata)

	query := `
		INSERT INTO data_flows (
			id, organization_id, agent_id, agent_name,
			flow_id, parent_flow_id, hop_number,
			source_id, source_type, source_name, source_details,
			destination_type, destination_name, destination_endpoint, destination_details,
			classification_category, classification_type, classification_tier, classification_confidence,
			data_size_bytes, record_count, field_count, fields_detected,
			policy_id, policy_action, policy_reason,
			request_id, session_id, user_id, client_ip,
			verification_event_id, capability,
			started_at, completed_at, duration_ms,
			status, error_message, metadata, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22, $23,
			$24, $25, $26,
			$27, $28, $29, $30,
			$31, $32,
			$33, $34, $35,
			$36, $37, $38, $39
		)`

	_, err := r.db.Exec(query,
		flow.ID, flow.OrganizationID, flow.AgentID, flow.AgentName,
		flow.FlowID, nullString(flow.ParentFlowID), flow.HopNumber,
		flow.SourceID, flow.SourceType, flow.SourceName, sourceDetails,
		flow.DestinationType, flow.DestinationName, nullString(flow.DestinationEndpoint), destDetails,
		nullString(string(flow.ClassificationCategory)), nullString(flow.ClassificationType), flow.ClassificationTier, flow.ClassificationConfidence,
		nullInt64(flow.DataSizeBytes), nullInt(flow.RecordCount), nullInt(flow.FieldCount), pq.Array(flow.FieldsDetected),
		flow.PolicyID, flow.PolicyAction, nullString(flow.PolicyReason),
		nullString(flow.RequestID), nullString(flow.SessionID), nullString(flow.UserID), nullString(flow.ClientIP),
		flow.VerificationEventID, nullString(flow.Capability),
		flow.StartedAt, flow.CompletedAt, nullInt(flow.DurationMs),
		flow.Status, nullString(flow.ErrorMessage), metadata, flow.CreatedAt,
	)
	return err
}

// GetFlowByID retrieves a data flow by ID
func (r *DataFlowRepository) GetFlowByID(id uuid.UUID) (*domain.DataFlow, error) {
	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			flow_id, parent_flow_id, hop_number,
			source_id, source_type, source_name, source_details,
			destination_type, destination_name, destination_endpoint, destination_details,
			classification_category, classification_type, classification_tier, classification_confidence,
			data_size_bytes, record_count, field_count, fields_detected,
			policy_id, policy_action, policy_reason,
			request_id, session_id, user_id, client_ip,
			verification_event_id, capability,
			started_at, completed_at, duration_ms,
			status, error_message, metadata, created_at
		FROM data_flows
		WHERE id = $1`

	flow := &domain.DataFlow{}
	var sourceDetails, destDetails, metadata []byte
	var parentFlowID, destEndpoint, classCategory, classType sql.NullString
	var policyReason, requestID, sessionID, userID, clientIP, capability, errorMessage sql.NullString
	var sourceID, policyID, verificationEventID sql.NullString
	var dataSizeBytes sql.NullInt64
	var recordCount, fieldCount, durationMs sql.NullInt32
	var completedAt sql.NullTime
	var fieldsDetected pq.StringArray

	err := r.db.QueryRow(query, id).Scan(
		&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
		&flow.FlowID, &parentFlowID, &flow.HopNumber,
		&sourceID, &flow.SourceType, &flow.SourceName, &sourceDetails,
		&flow.DestinationType, &flow.DestinationName, &destEndpoint, &destDetails,
		&classCategory, &classType, &flow.ClassificationTier, &flow.ClassificationConfidence,
		&dataSizeBytes, &recordCount, &fieldCount, &fieldsDetected,
		&policyID, &flow.PolicyAction, &policyReason,
		&requestID, &sessionID, &userID, &clientIP,
		&verificationEventID, &capability,
		&flow.StartedAt, &completedAt, &durationMs,
		&flow.Status, &errorMessage, &metadata, &flow.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	flow.ParentFlowID = parentFlowID.String
	flow.DestinationEndpoint = destEndpoint.String
	flow.ClassificationCategory = classCategory.String
	flow.ClassificationType = classType.String
	flow.PolicyReason = policyReason.String
	flow.RequestID = requestID.String
	flow.SessionID = sessionID.String
	flow.UserID = userID.String
	flow.ClientIP = clientIP.String
	flow.Capability = capability.String
	flow.ErrorMessage = errorMessage.String
	flow.FieldsDetected = fieldsDetected

	if sourceID.Valid {
		id, _ := uuid.Parse(sourceID.String)
		flow.SourceID = &id
	}
	if policyID.Valid {
		id, _ := uuid.Parse(policyID.String)
		flow.PolicyID = &id
	}
	if verificationEventID.Valid {
		id, _ := uuid.Parse(verificationEventID.String)
		flow.VerificationEventID = &id
	}
	if dataSizeBytes.Valid {
		flow.DataSizeBytes = dataSizeBytes.Int64
	}
	if recordCount.Valid {
		flow.RecordCount = int(recordCount.Int32)
	}
	if fieldCount.Valid {
		flow.FieldCount = int(fieldCount.Int32)
	}
	if durationMs.Valid {
		flow.DurationMs = int(durationMs.Int32)
	}
	if completedAt.Valid {
		flow.CompletedAt = &completedAt.Time
	}

	json.Unmarshal(sourceDetails, &flow.SourceDetails)
	json.Unmarshal(destDetails, &flow.DestinationDetails)
	json.Unmarshal(metadata, &flow.Metadata)

	return flow, nil
}

// GetFlowsByOrganization retrieves flows for an organization with pagination
func (r *DataFlowRepository) GetFlowsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.DataFlow, int, error) {
	// Get total count first
	var total int
	countQuery := `SELECT COUNT(*) FROM data_flows WHERE organization_id = $1`
	if err := r.db.QueryRow(countQuery, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			flow_id, parent_flow_id, hop_number,
			source_type, source_name,
			destination_type, destination_name, destination_endpoint,
			classification_category, classification_type, classification_tier, classification_confidence,
			policy_action, policy_reason,
			started_at, completed_at, duration_ms,
			status, created_at
		FROM data_flows
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flows []*domain.DataFlow
	for rows.Next() {
		flow := &domain.DataFlow{}
		var parentFlowID, destEndpoint, classCategory, classType, policyReason sql.NullString
		var durationMs sql.NullInt32
		var completedAt sql.NullTime

		err := rows.Scan(
			&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
			&flow.FlowID, &parentFlowID, &flow.HopNumber,
			&flow.SourceType, &flow.SourceName,
			&flow.DestinationType, &flow.DestinationName, &destEndpoint,
			&classCategory, &classType, &flow.ClassificationTier, &flow.ClassificationConfidence,
			&flow.PolicyAction, &policyReason,
			&flow.StartedAt, &completedAt, &durationMs,
			&flow.Status, &flow.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		flow.ParentFlowID = parentFlowID.String
		flow.DestinationEndpoint = destEndpoint.String
		flow.ClassificationCategory = classCategory.String
		flow.ClassificationType = classType.String
		flow.PolicyReason = policyReason.String
		if durationMs.Valid {
			flow.DurationMs = int(durationMs.Int32)
		}
		if completedAt.Valid {
			flow.CompletedAt = &completedAt.Time
		}

		flows = append(flows, flow)
	}

	return flows, total, nil
}

// GetFlowsByAgent retrieves flows for a specific agent
func (r *DataFlowRepository) GetFlowsByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.DataFlow, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM data_flows WHERE agent_id = $1`
	if err := r.db.QueryRow(countQuery, agentID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			flow_id, hop_number,
			source_type, source_name,
			destination_type, destination_name, destination_endpoint,
			classification_category, classification_type, classification_tier,
			policy_action, status, duration_ms, created_at
		FROM data_flows
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, agentID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flows []*domain.DataFlow
	for rows.Next() {
		flow := &domain.DataFlow{}
		var destEndpoint, classCategory, classType sql.NullString
		var durationMs sql.NullInt32

		err := rows.Scan(
			&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
			&flow.FlowID, &flow.HopNumber,
			&flow.SourceType, &flow.SourceName,
			&flow.DestinationType, &flow.DestinationName, &destEndpoint,
			&classCategory, &classType, &flow.ClassificationTier,
			&flow.PolicyAction, &flow.Status, &durationMs, &flow.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		flow.DestinationEndpoint = destEndpoint.String
		flow.ClassificationCategory = classCategory.String
		flow.ClassificationType = classType.String
		if durationMs.Valid {
			flow.DurationMs = int(durationMs.Int32)
		}

		flows = append(flows, flow)
	}

	return flows, total, nil
}

// GetFlowsByClassification retrieves flows by classification category/type
func (r *DataFlowRepository) GetFlowsByClassification(orgID uuid.UUID, category, dataType string, limit, offset int) ([]*domain.DataFlow, int, error) {
	var total int
	countQuery := `
		SELECT COUNT(*) FROM data_flows
		WHERE organization_id = $1
		AND ($2 = '' OR classification_category = $2)
		AND ($3 = '' OR classification_type = $3)`
	if err := r.db.QueryRow(countQuery, orgID, category, dataType).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			source_type, source_name,
			destination_type, destination_name, destination_endpoint,
			classification_category, classification_type, classification_tier,
			policy_action, status, created_at
		FROM data_flows
		WHERE organization_id = $1
		AND ($2 = '' OR classification_category = $2)
		AND ($3 = '' OR classification_type = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.db.Query(query, orgID, category, dataType, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flows []*domain.DataFlow
	for rows.Next() {
		flow := &domain.DataFlow{}
		var destEndpoint, classCategory, classType sql.NullString

		err := rows.Scan(
			&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
			&flow.SourceType, &flow.SourceName,
			&flow.DestinationType, &flow.DestinationName, &destEndpoint,
			&classCategory, &classType, &flow.ClassificationTier,
			&flow.PolicyAction, &flow.Status, &flow.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		flow.DestinationEndpoint = destEndpoint.String
		flow.ClassificationCategory = classCategory.String
		flow.ClassificationType = classType.String
		flows = append(flows, flow)
	}

	return flows, total, nil
}

// GetFlowChain retrieves all flows in a chain by flow_id
func (r *DataFlowRepository) GetFlowChain(flowID string) ([]*domain.DataFlow, error) {
	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			flow_id, parent_flow_id, hop_number,
			source_type, source_name,
			destination_type, destination_name,
			classification_category, classification_type,
			policy_action, status, created_at
		FROM data_flows
		WHERE flow_id = $1
		ORDER BY hop_number ASC`

	rows, err := r.db.Query(query, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []*domain.DataFlow
	for rows.Next() {
		flow := &domain.DataFlow{}
		var parentFlowID, classCategory, classType sql.NullString

		err := rows.Scan(
			&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
			&flow.FlowID, &parentFlowID, &flow.HopNumber,
			&flow.SourceType, &flow.SourceName,
			&flow.DestinationType, &flow.DestinationName,
			&classCategory, &classType,
			&flow.PolicyAction, &flow.Status, &flow.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		flow.ParentFlowID = parentFlowID.String
		flow.ClassificationCategory = classCategory.String
		flow.ClassificationType = classType.String
		flows = append(flows, flow)
	}

	return flows, nil
}

// UpdateFlowStatus updates the status of a flow
func (r *DataFlowRepository) UpdateFlowStatus(id uuid.UUID, status domain.DataFlowStatus, completedAt time.Time, durationMs int) error {
	query := `
		UPDATE data_flows
		SET status = $2, completed_at = $3, duration_ms = $4
		WHERE id = $1`
	_, err := r.db.Exec(query, id, status, completedAt, durationMs)
	return err
}

// GetRecentFlows retrieves flows from the last N minutes
func (r *DataFlowRepository) GetRecentFlows(orgID uuid.UUID, minutes int) ([]*domain.DataFlow, error) {
	query := `
		SELECT
			id, organization_id, agent_id, agent_name,
			source_type, source_name,
			destination_type, destination_name,
			classification_category, classification_type,
			policy_action, status, created_at
		FROM data_flows
		WHERE organization_id = $1
		AND created_at > NOW() - INTERVAL '1 minute' * $2
		ORDER BY created_at DESC
		LIMIT 1000`

	rows, err := r.db.Query(query, orgID, minutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []*domain.DataFlow
	for rows.Next() {
		flow := &domain.DataFlow{}
		var classCategory, classType sql.NullString

		err := rows.Scan(
			&flow.ID, &flow.OrganizationID, &flow.AgentID, &flow.AgentName,
			&flow.SourceType, &flow.SourceName,
			&flow.DestinationType, &flow.DestinationName,
			&classCategory, &classType,
			&flow.PolicyAction, &flow.Status, &flow.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		flow.ClassificationCategory = classCategory.String
		flow.ClassificationType = classType.String
		flows = append(flows, flow)
	}

	return flows, nil
}

// ============================================================================
// DATA SOURCES
// ============================================================================

// CreateSource creates a new data source
func (r *DataFlowRepository) CreateSource(source *domain.DataSource) error {
	metadata, _ := json.Marshal(source.Metadata)

	query := `
		INSERT INTO data_sources (
			id, organization_id, source_type, source_name, source_identifier,
			database_type, schema_name, table_name, column_name,
			api_endpoint, api_method, response_path,
			classification_tier, classification_type_id, classification_confidence,
			is_manually_classified, classified_by, classified_at,
			description, metadata, is_active,
			discovered_at, last_seen_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20, $21,
			$22, $23, $24, $25
		)
		ON CONFLICT (organization_id, source_type, source_identifier)
		DO UPDATE SET last_seen_at = NOW(), updated_at = NOW()`

	_, err := r.db.Exec(query,
		source.ID, source.OrganizationID, source.SourceType, source.SourceName, source.SourceIdentifier,
		nullString(source.DatabaseType), nullString(source.SchemaName), nullString(source.TableName), nullString(source.ColumnName),
		nullString(source.APIEndpoint), nullString(source.APIMethod), nullString(source.ResponsePath),
		source.ClassificationTier, source.ClassificationTypeID, source.ClassificationConfidence,
		source.IsManuallyClassified, source.ClassifiedBy, source.ClassifiedAt,
		nullString(source.Description), metadata, source.IsActive,
		source.DiscoveredAt, source.LastSeenAt, source.CreatedAt, source.UpdatedAt,
	)
	return err
}

// GetSourceByID retrieves a data source by ID
func (r *DataFlowRepository) GetSourceByID(id uuid.UUID) (*domain.DataSource, error) {
	query := `
		SELECT
			ds.id, ds.organization_id, ds.source_type, ds.source_name, ds.source_identifier,
			ds.database_type, ds.schema_name, ds.table_name, ds.column_name,
			ds.api_endpoint, ds.api_method, ds.response_path,
			ds.classification_tier, ds.classification_type_id, ds.classification_confidence,
			ds.is_manually_classified, ds.classified_by, ds.classified_at,
			ds.description, ds.metadata, ds.is_active,
			ds.discovered_at, ds.last_seen_at, ds.created_at, ds.updated_at,
			dct.name, dcc.name
		FROM data_sources ds
		LEFT JOIN data_classification_types dct ON ds.classification_type_id = dct.id
		LEFT JOIN data_classification_categories dcc ON dct.category_id = dcc.id
		WHERE ds.id = $1`

	source := &domain.DataSource{}
	var dbType, schemaName, tableName, colName sql.NullString
	var apiEndpoint, apiMethod, responsePath, description sql.NullString
	var classTypeID, classifiedBy sql.NullString
	var classifiedAt sql.NullTime
	var metadata []byte
	var classTypeName, classCategoryName sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&source.ID, &source.OrganizationID, &source.SourceType, &source.SourceName, &source.SourceIdentifier,
		&dbType, &schemaName, &tableName, &colName,
		&apiEndpoint, &apiMethod, &responsePath,
		&source.ClassificationTier, &classTypeID, &source.ClassificationConfidence,
		&source.IsManuallyClassified, &classifiedBy, &classifiedAt,
		&description, &metadata, &source.IsActive,
		&source.DiscoveredAt, &source.LastSeenAt, &source.CreatedAt, &source.UpdatedAt,
		&classTypeName, &classCategoryName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	source.DatabaseType = dbType.String
	source.SchemaName = schemaName.String
	source.TableName = tableName.String
	source.ColumnName = colName.String
	source.APIEndpoint = apiEndpoint.String
	source.APIMethod = apiMethod.String
	source.ResponsePath = responsePath.String
	source.Description = description.String
	source.ClassificationTypeName = classTypeName.String
	source.ClassificationCategory = classCategoryName.String

	if classTypeID.Valid {
		id, _ := uuid.Parse(classTypeID.String)
		source.ClassificationTypeID = &id
	}
	if classifiedBy.Valid {
		id, _ := uuid.Parse(classifiedBy.String)
		source.ClassifiedBy = &id
	}
	if classifiedAt.Valid {
		source.ClassifiedAt = &classifiedAt.Time
	}

	json.Unmarshal(metadata, &source.Metadata)
	return source, nil
}

// GetSourceByIdentifier finds a source by its unique identifier
func (r *DataFlowRepository) GetSourceByIdentifier(orgID uuid.UUID, sourceType domain.DataSourceType, identifier string) (*domain.DataSource, error) {
	query := `
		SELECT
			ds.id, ds.organization_id, ds.source_type, ds.source_name, ds.source_identifier,
			ds.classification_tier, ds.classification_type_id, ds.classification_confidence,
			dct.name, dcc.name
		FROM data_sources ds
		LEFT JOIN data_classification_types dct ON ds.classification_type_id = dct.id
		LEFT JOIN data_classification_categories dcc ON dct.category_id = dcc.id
		WHERE ds.organization_id = $1 AND ds.source_type = $2 AND ds.source_identifier = $3`

	source := &domain.DataSource{}
	var classTypeID sql.NullString
	var classTypeName, classCategoryName sql.NullString

	err := r.db.QueryRow(query, orgID, sourceType, identifier).Scan(
		&source.ID, &source.OrganizationID, &source.SourceType, &source.SourceName, &source.SourceIdentifier,
		&source.ClassificationTier, &classTypeID, &source.ClassificationConfidence,
		&classTypeName, &classCategoryName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if classTypeID.Valid {
		id, _ := uuid.Parse(classTypeID.String)
		source.ClassificationTypeID = &id
	}
	source.ClassificationTypeName = classTypeName.String
	source.ClassificationCategory = classCategoryName.String

	return source, nil
}

// GetSourcesByOrganization retrieves all sources for an organization
func (r *DataFlowRepository) GetSourcesByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.DataSource, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM data_sources WHERE organization_id = $1`
	if err := r.db.QueryRow(countQuery, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			ds.id, ds.source_type, ds.source_name,
			ds.table_name, ds.column_name,
			ds.classification_tier, dct.name, dcc.name,
			ds.is_active, ds.last_seen_at
		FROM data_sources ds
		LEFT JOIN data_classification_types dct ON ds.classification_type_id = dct.id
		LEFT JOIN data_classification_categories dcc ON dct.category_id = dcc.id
		WHERE ds.organization_id = $1
		ORDER BY ds.last_seen_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sources []*domain.DataSource
	for rows.Next() {
		s := &domain.DataSource{OrganizationID: orgID}
		var tableName, colName, classTypeName, classCategoryName sql.NullString

		err := rows.Scan(
			&s.ID, &s.SourceType, &s.SourceName,
			&tableName, &colName,
			&s.ClassificationTier, &classTypeName, &classCategoryName,
			&s.IsActive, &s.LastSeenAt,
		)
		if err != nil {
			return nil, 0, err
		}

		s.TableName = tableName.String
		s.ColumnName = colName.String
		s.ClassificationTypeName = classTypeName.String
		s.ClassificationCategory = classCategoryName.String
		sources = append(sources, s)
	}

	return sources, total, nil
}

// UpdateSourceClassification updates the classification of a source
func (r *DataFlowRepository) UpdateSourceClassification(id uuid.UUID, typeID uuid.UUID, tier domain.ClassificationTier, confidence float64) error {
	query := `
		UPDATE data_sources
		SET classification_type_id = $2, classification_tier = $3, classification_confidence = $4, updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(query, id, typeID, tier, confidence)
	return err
}

// UpdateSourceLastSeen updates the last_seen_at timestamp
func (r *DataFlowRepository) UpdateSourceLastSeen(id uuid.UUID) error {
	query := `UPDATE data_sources SET last_seen_at = NOW() WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

// ============================================================================
// POLICIES
// ============================================================================

// CreatePolicy creates a new data flow policy
func (r *DataFlowRepository) CreatePolicy(policy *domain.DataFlowPolicy) error {
	redactionConfig, _ := json.Marshal(policy.RedactionConfig)

	query := `
		INSERT INTO data_flow_policies (
			id, organization_id, name, description, priority,
			classification_categories, classification_types, min_classification_tier,
			source_types, source_patterns,
			destination_types, destination_patterns, blocked_destinations, allowed_destinations,
			agent_ids, agent_types,
			action, alert_severity, alert_message, notify_channels,
			redaction_strategy, redaction_config,
			is_enabled, is_builtin, created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10,
			$11, $12, $13, $14,
			$15, $16,
			$17, $18, $19, $20,
			$21, $22,
			$23, $24, $25, $26, $27
		)`

	_, err := r.db.Exec(query,
		policy.ID, policy.OrganizationID, policy.Name, policy.Description, policy.Priority,
		pq.Array(policy.ClassificationCategories), pq.Array(policy.ClassificationTypes), policy.MinClassificationTier,
		pq.Array(policy.SourceTypes), pq.Array(policy.SourcePatterns),
		pq.Array(policy.DestinationTypes), pq.Array(policy.DestinationPatterns), pq.Array(policy.BlockedDestinations), pq.Array(policy.AllowedDestinations),
		pq.Array(uuidsToStrings(policy.AgentIDs)), pq.Array(policy.AgentTypes),
		policy.Action, nullString(policy.AlertSeverity), nullString(policy.AlertMessage), pq.Array(policy.NotifyChannels),
		nullString(policy.RedactionStrategy), redactionConfig,
		policy.IsEnabled, policy.IsBuiltin, policy.CreatedBy, policy.CreatedAt, policy.UpdatedAt,
	)
	return err
}

// GetPolicyByID retrieves a policy by ID
func (r *DataFlowRepository) GetPolicyByID(id uuid.UUID) (*domain.DataFlowPolicy, error) {
	query := `
		SELECT
			id, organization_id, name, description, priority,
			classification_categories, classification_types, min_classification_tier,
			source_types, source_patterns,
			destination_types, destination_patterns, blocked_destinations, allowed_destinations,
			agent_ids, agent_types,
			action, alert_severity, alert_message, notify_channels,
			redaction_strategy, redaction_config,
			is_enabled, is_builtin, created_by, created_at, updated_at
		FROM data_flow_policies
		WHERE id = $1`

	policy := &domain.DataFlowPolicy{}
	var description, alertSeverity, alertMessage, redactionStrategy sql.NullString
	var minTier sql.NullInt32
	var createdBy sql.NullString
	var redactionConfig []byte
	var classCategories, classTypes, sourceTypes, sourcePatterns pq.StringArray
	var destTypes, destPatterns, blockedDests, allowedDests pq.StringArray
	var agentIDs, agentTypes, notifyChannels pq.StringArray

	err := r.db.QueryRow(query, id).Scan(
		&policy.ID, &policy.OrganizationID, &policy.Name, &description, &policy.Priority,
		&classCategories, &classTypes, &minTier,
		&sourceTypes, &sourcePatterns,
		&destTypes, &destPatterns, &blockedDests, &allowedDests,
		&agentIDs, &agentTypes,
		&policy.Action, &alertSeverity, &alertMessage, &notifyChannels,
		&redactionStrategy, &redactionConfig,
		&policy.IsEnabled, &policy.IsBuiltin, &createdBy, &policy.CreatedAt, &policy.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	policy.Description = description.String
	policy.AlertSeverity = alertSeverity.String
	policy.AlertMessage = alertMessage.String
	policy.RedactionStrategy = redactionStrategy.String
	policy.ClassificationCategories = classCategories
	policy.ClassificationTypes = classTypes
	policy.SourceTypes = sourceTypes
	policy.SourcePatterns = sourcePatterns
	policy.DestinationTypes = destTypes
	policy.DestinationPatterns = destPatterns
	policy.BlockedDestinations = blockedDests
	policy.AllowedDestinations = allowedDests
	policy.AgentTypes = agentTypes
	policy.NotifyChannels = notifyChannels
	policy.AgentIDs = stringsToUUIDs(agentIDs)

	if minTier.Valid {
		tier := int(minTier.Int32)
		policy.MinClassificationTier = &tier
	}
	if createdBy.Valid {
		id, _ := uuid.Parse(createdBy.String)
		policy.CreatedBy = &id
	}

	json.Unmarshal(redactionConfig, &policy.RedactionConfig)
	return policy, nil
}

// GetPoliciesByOrganization retrieves all policies for an organization
func (r *DataFlowRepository) GetPoliciesByOrganization(orgID uuid.UUID) ([]*domain.DataFlowPolicy, error) {
	query := `
		SELECT
			id, name, description, priority,
			classification_categories, destination_types,
			action, alert_severity,
			is_enabled, is_builtin, created_at
		FROM data_flow_policies
		WHERE organization_id = $1
		ORDER BY priority ASC`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.DataFlowPolicy
	for rows.Next() {
		p := &domain.DataFlowPolicy{OrganizationID: orgID}
		var description, alertSeverity sql.NullString
		var classCategories, destTypes pq.StringArray

		err := rows.Scan(
			&p.ID, &p.Name, &description, &p.Priority,
			&classCategories, &destTypes,
			&p.Action, &alertSeverity,
			&p.IsEnabled, &p.IsBuiltin, &p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		p.Description = description.String
		p.AlertSeverity = alertSeverity.String
		p.ClassificationCategories = classCategories
		p.DestinationTypes = destTypes
		policies = append(policies, p)
	}

	return policies, nil
}

// GetEnabledPoliciesByPriority retrieves enabled policies in priority order for policy evaluation
func (r *DataFlowRepository) GetEnabledPoliciesByPriority(orgID uuid.UUID) ([]*domain.DataFlowPolicy, error) {
	query := `
		SELECT
			id, name, priority,
			classification_categories, classification_types, min_classification_tier,
			source_types, source_patterns,
			destination_types, destination_patterns, blocked_destinations, allowed_destinations,
			agent_ids, agent_types,
			action, alert_severity, alert_message
		FROM data_flow_policies
		WHERE organization_id = $1 AND is_enabled = TRUE
		ORDER BY priority ASC`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.DataFlowPolicy
	for rows.Next() {
		p := &domain.DataFlowPolicy{OrganizationID: orgID, IsEnabled: true}
		var alertSeverity, alertMessage sql.NullString
		var minTier sql.NullInt32
		var classCategories, classTypes, sourceTypes, sourcePatterns pq.StringArray
		var destTypes, destPatterns, blockedDests, allowedDests pq.StringArray
		var agentIDs, agentTypes pq.StringArray

		err := rows.Scan(
			&p.ID, &p.Name, &p.Priority,
			&classCategories, &classTypes, &minTier,
			&sourceTypes, &sourcePatterns,
			&destTypes, &destPatterns, &blockedDests, &allowedDests,
			&agentIDs, &agentTypes,
			&p.Action, &alertSeverity, &alertMessage,
		)
		if err != nil {
			return nil, err
		}

		p.AlertSeverity = alertSeverity.String
		p.AlertMessage = alertMessage.String
		p.ClassificationCategories = classCategories
		p.ClassificationTypes = classTypes
		p.SourceTypes = sourceTypes
		p.SourcePatterns = sourcePatterns
		p.DestinationTypes = destTypes
		p.DestinationPatterns = destPatterns
		p.BlockedDestinations = blockedDests
		p.AllowedDestinations = allowedDests
		p.AgentTypes = agentTypes
		p.AgentIDs = stringsToUUIDs(agentIDs)

		if minTier.Valid {
			tier := int(minTier.Int32)
			p.MinClassificationTier = &tier
		}

		policies = append(policies, p)
	}

	return policies, nil
}

// UpdatePolicy updates an existing policy
func (r *DataFlowRepository) UpdatePolicy(policy *domain.DataFlowPolicy) error {
	redactionConfig, _ := json.Marshal(policy.RedactionConfig)

	query := `
		UPDATE data_flow_policies SET
			name = $2, description = $3, priority = $4,
			classification_categories = $5, classification_types = $6, min_classification_tier = $7,
			source_types = $8, source_patterns = $9,
			destination_types = $10, destination_patterns = $11, blocked_destinations = $12, allowed_destinations = $13,
			agent_ids = $14, agent_types = $15,
			action = $16, alert_severity = $17, alert_message = $18, notify_channels = $19,
			redaction_strategy = $20, redaction_config = $21,
			is_enabled = $22, updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(query,
		policy.ID, policy.Name, policy.Description, policy.Priority,
		pq.Array(policy.ClassificationCategories), pq.Array(policy.ClassificationTypes), policy.MinClassificationTier,
		pq.Array(policy.SourceTypes), pq.Array(policy.SourcePatterns),
		pq.Array(policy.DestinationTypes), pq.Array(policy.DestinationPatterns), pq.Array(policy.BlockedDestinations), pq.Array(policy.AllowedDestinations),
		pq.Array(uuidsToStrings(policy.AgentIDs)), pq.Array(policy.AgentTypes),
		policy.Action, nullString(policy.AlertSeverity), nullString(policy.AlertMessage), pq.Array(policy.NotifyChannels),
		nullString(policy.RedactionStrategy), redactionConfig,
		policy.IsEnabled,
	)
	return err
}

// DeletePolicy deletes a policy
func (r *DataFlowRepository) DeletePolicy(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM data_flow_policies WHERE id = $1`, id)
	return err
}

// ============================================================================
// VIOLATIONS
// ============================================================================

// CreateViolation creates a new violation record
func (r *DataFlowRepository) CreateViolation(v *domain.DataFlowViolation) error {
	metadata, _ := json.Marshal(v.Metadata)

	query := `
		INSERT INTO data_flow_violations (
			id, organization_id, data_flow_id, policy_id, agent_id,
			violation_type, severity,
			classification_category, classification_type,
			source_name, destination_name, destination_endpoint,
			action_taken, action_details,
			status, alert_id, metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17, $18
		)`

	_, err := r.db.Exec(query,
		v.ID, v.OrganizationID, v.DataFlowID, v.PolicyID, v.AgentID,
		v.ViolationType, v.Severity,
		v.ClassificationCategory, nullString(v.ClassificationType),
		v.SourceName, v.DestinationName, nullString(v.DestinationEndpoint),
		v.ActionTaken, nullString(v.ActionDetails),
		v.Status, v.AlertID, metadata, v.CreatedAt,
	)
	return err
}

// GetViolationByID retrieves a violation by ID
func (r *DataFlowRepository) GetViolationByID(id uuid.UUID) (*domain.DataFlowViolation, error) {
	query := `
		SELECT
			id, organization_id, data_flow_id, policy_id, agent_id,
			violation_type, severity,
			classification_category, classification_type,
			source_name, destination_name, destination_endpoint,
			action_taken, action_details,
			status, resolved_by, resolved_at, resolution_notes,
			alert_id, metadata, created_at
		FROM data_flow_violations
		WHERE id = $1`

	v := &domain.DataFlowViolation{}
	var classType, destEndpoint, actionDetails, resolutionNotes sql.NullString
	var resolvedBy, alertID sql.NullString
	var resolvedAt sql.NullTime
	var metadata []byte

	err := r.db.QueryRow(query, id).Scan(
		&v.ID, &v.OrganizationID, &v.DataFlowID, &v.PolicyID, &v.AgentID,
		&v.ViolationType, &v.Severity,
		&v.ClassificationCategory, &classType,
		&v.SourceName, &v.DestinationName, &destEndpoint,
		&v.ActionTaken, &actionDetails,
		&v.Status, &resolvedBy, &resolvedAt, &resolutionNotes,
		&alertID, &metadata, &v.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	v.ClassificationType = classType.String
	v.DestinationEndpoint = destEndpoint.String
	v.ActionDetails = actionDetails.String
	v.ResolutionNotes = resolutionNotes.String

	if resolvedBy.Valid {
		id, _ := uuid.Parse(resolvedBy.String)
		v.ResolvedBy = &id
	}
	if resolvedAt.Valid {
		v.ResolvedAt = &resolvedAt.Time
	}
	if alertID.Valid {
		id, _ := uuid.Parse(alertID.String)
		v.AlertID = &id
	}

	json.Unmarshal(metadata, &v.Metadata)
	return v, nil
}

// GetViolationsByOrganization retrieves violations for an organization
func (r *DataFlowRepository) GetViolationsByOrganization(orgID uuid.UUID, status domain.ViolationStatus, limit, offset int) ([]*domain.DataFlowViolation, int, error) {
	var total int
	countQuery := `
		SELECT COUNT(*) FROM data_flow_violations
		WHERE organization_id = $1 AND ($2 = '' OR status = $2)`
	if err := r.db.QueryRow(countQuery, orgID, status).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, data_flow_id, policy_id, agent_id,
			violation_type, severity,
			classification_category, classification_type,
			source_name, destination_name,
			action_taken, status, created_at
		FROM data_flow_violations
		WHERE organization_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(query, orgID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var violations []*domain.DataFlowViolation
	for rows.Next() {
		v := &domain.DataFlowViolation{OrganizationID: orgID}
		var classType sql.NullString

		err := rows.Scan(
			&v.ID, &v.DataFlowID, &v.PolicyID, &v.AgentID,
			&v.ViolationType, &v.Severity,
			&v.ClassificationCategory, &classType,
			&v.SourceName, &v.DestinationName,
			&v.ActionTaken, &v.Status, &v.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		v.ClassificationType = classType.String
		violations = append(violations, v)
	}

	return violations, total, nil
}

// UpdateViolationStatus updates the status of a violation
func (r *DataFlowRepository) UpdateViolationStatus(id uuid.UUID, status domain.ViolationStatus, resolvedBy *uuid.UUID, notes string) error {
	query := `
		UPDATE data_flow_violations
		SET status = $2, resolved_by = $3, resolved_at = NOW(), resolution_notes = $4
		WHERE id = $1`
	_, err := r.db.Exec(query, id, status, resolvedBy, notes)
	return err
}

// ============================================================================
// STATISTICS
// ============================================================================

// GetSummary returns a high-level summary of data flows
func (r *DataFlowRepository) GetSummary(orgID uuid.UUID) (*domain.DataFlowSummary, error) {
	query := `
		SELECT
			COUNT(*) as total_flows,
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') as flows_24h,
			COUNT(*) FILTER (WHERE policy_action = 'block') as blocked,
			COUNT(*) FILTER (WHERE policy_action = 'alert') as alerted,
			COUNT(*) FILTER (WHERE classification_category = 'PII') as pii,
			COUNT(*) FILTER (WHERE classification_category = 'PHI') as phi,
			COUNT(*) FILTER (WHERE classification_category = 'PCI') as pci,
			COUNT(DISTINCT agent_id) as unique_agents,
			COUNT(DISTINCT destination_name) as unique_destinations,
			COUNT(DISTINCT source_name) as unique_sources
		FROM data_flows
		WHERE organization_id = $1`

	summary := &domain.DataFlowSummary{}
	err := r.db.QueryRow(query, orgID).Scan(
		&summary.TotalFlows,
		&summary.FlowsLast24h,
		&summary.BlockedFlows,
		&summary.AlertedFlows,
		&summary.PIIFlows,
		&summary.PHIFlows,
		&summary.PCIFlows,
		&summary.UniqueAgents,
		&summary.UniqueDestinations,
		&summary.UniqueSources,
	)
	if err != nil {
		return nil, err
	}

	// Get open violations count
	violationQuery := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'open') as open,
			COUNT(*) FILTER (WHERE status = 'open' AND severity = 'critical') as critical
		FROM data_flow_violations
		WHERE organization_id = $1`
	r.db.QueryRow(violationQuery, orgID).Scan(&summary.OpenViolations, &summary.CriticalViolations)

	// Calculate avg flows per hour (last 24h)
	if summary.FlowsLast24h > 0 {
		summary.AvgFlowsPerHour = float64(summary.FlowsLast24h) / 24.0
	}

	return summary, nil
}

// GetStatisticsByTimeRange retrieves pre-aggregated statistics
func (r *DataFlowRepository) GetStatisticsByTimeRange(orgID uuid.UUID, start, end time.Time) ([]*domain.DataFlowStatistics, error) {
	query := `
		SELECT
			id, organization_id, bucket_time,
			agent_id, classification_category, destination_type,
			total_flows, allowed_flows, blocked_flows, alerted_flows, redacted_flows,
			total_bytes, total_records,
			avg_duration_ms, p95_duration_ms,
			unique_destinations, unique_sources,
			computed_at
		FROM data_flow_statistics
		WHERE organization_id = $1 AND bucket_time >= $2 AND bucket_time <= $3
		ORDER BY bucket_time ASC`

	rows, err := r.db.Query(query, orgID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*domain.DataFlowStatistics
	for rows.Next() {
		s := &domain.DataFlowStatistics{}
		var agentID sql.NullString
		var classCategory, destType sql.NullString

		err := rows.Scan(
			&s.ID, &s.OrganizationID, &s.BucketTime,
			&agentID, &classCategory, &destType,
			&s.TotalFlows, &s.AllowedFlows, &s.BlockedFlows, &s.AlertedFlows, &s.RedactedFlows,
			&s.TotalBytes, &s.TotalRecords,
			&s.AvgDurationMs, &s.P95DurationMs,
			&s.UniqueDestinations, &s.UniqueSources,
			&s.ComputedAt,
		)
		if err != nil {
			return nil, err
		}

		if agentID.Valid {
			id, _ := uuid.Parse(agentID.String)
			s.AgentID = &id
		}
		s.ClassificationCategory = classCategory.String
		s.DestinationType = destType.String
		stats = append(stats, s)
	}

	return stats, nil
}

// AggregateStatistics aggregates flows into the statistics table for a time bucket
func (r *DataFlowRepository) AggregateStatistics(orgID uuid.UUID, bucketTime time.Time) error {
	query := `
		INSERT INTO data_flow_statistics (
			id, organization_id, bucket_time,
			agent_id, classification_category, destination_type,
			total_flows, allowed_flows, blocked_flows, alerted_flows, redacted_flows,
			total_bytes, total_records,
			avg_duration_ms, p95_duration_ms,
			unique_destinations, unique_sources,
			computed_at
		)
		SELECT
			gen_random_uuid(), $1, $2,
			agent_id, classification_category, destination_type,
			COUNT(*),
			COUNT(*) FILTER (WHERE policy_action = 'allow'),
			COUNT(*) FILTER (WHERE policy_action = 'block'),
			COUNT(*) FILTER (WHERE policy_action = 'alert'),
			COUNT(*) FILTER (WHERE policy_action = 'redact'),
			COALESCE(SUM(data_size_bytes), 0),
			COALESCE(SUM(record_count), 0),
			COALESCE(AVG(duration_ms), 0),
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0),
			COUNT(DISTINCT destination_name),
			COUNT(DISTINCT source_name),
			NOW()
		FROM data_flows
		WHERE organization_id = $1
		AND created_at >= $2
		AND created_at < $2 + INTERVAL '1 hour'
		GROUP BY agent_id, classification_category, destination_type
		ON CONFLICT (organization_id, bucket_time, agent_id, classification_category, destination_type)
		DO UPDATE SET
			total_flows = EXCLUDED.total_flows,
			allowed_flows = EXCLUDED.allowed_flows,
			blocked_flows = EXCLUDED.blocked_flows,
			alerted_flows = EXCLUDED.alerted_flows,
			redacted_flows = EXCLUDED.redacted_flows,
			total_bytes = EXCLUDED.total_bytes,
			total_records = EXCLUDED.total_records,
			avg_duration_ms = EXCLUDED.avg_duration_ms,
			p95_duration_ms = EXCLUDED.p95_duration_ms,
			unique_destinations = EXCLUDED.unique_destinations,
			unique_sources = EXCLUDED.unique_sources,
			computed_at = NOW()`

	_, err := r.db.Exec(query, orgID, bucketTime)
	return err
}

// ============================================================================
// CLASSIFICATION TYPES
// ============================================================================

// GetClassificationCategories retrieves all classification categories
func (r *DataFlowRepository) GetClassificationCategories() ([]*domain.DataClassificationCategory, error) {
	query := `
		SELECT
			id, name, display_name, description, risk_level,
			regulatory_frameworks, color, icon, sort_order,
			created_at, updated_at
		FROM data_classification_categories
		ORDER BY sort_order ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*domain.DataClassificationCategory
	for rows.Next() {
		c := &domain.DataClassificationCategory{}
		var description, icon sql.NullString
		var frameworks pq.StringArray

		err := rows.Scan(
			&c.ID, &c.Name, &c.DisplayName, &description, &c.RiskLevel,
			&frameworks, &c.Color, &icon, &c.SortOrder,
			&c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		c.Description = description.String
		c.Icon = icon.String
		c.RegulatoryFrameworks = frameworks
		categories = append(categories, c)
	}

	return categories, nil
}

// GetClassificationTypes retrieves classification types, optionally filtered by category
func (r *DataFlowRepository) GetClassificationTypes(categoryID *uuid.UUID) ([]*domain.DataClassificationType, error) {
	query := `
		SELECT
			dct.id, dct.category_id, dcc.name,
			dct.name, dct.display_name, dct.description,
			dct.column_patterns, dct.table_patterns,
			dct.content_patterns, dct.content_pattern_confidence,
			dct.examples, dct.is_builtin,
			dct.created_at, dct.updated_at
		FROM data_classification_types dct
		JOIN data_classification_categories dcc ON dct.category_id = dcc.id
		WHERE ($1::uuid IS NULL OR dct.category_id = $1)
		ORDER BY dcc.sort_order ASC, dct.display_name ASC`

	rows, err := r.db.Query(query, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []*domain.DataClassificationType
	for rows.Next() {
		t := &domain.DataClassificationType{}
		var description sql.NullString
		var columnPatterns, tablePatterns, contentPatterns, examples pq.StringArray

		err := rows.Scan(
			&t.ID, &t.CategoryID, &t.CategoryName,
			&t.Name, &t.DisplayName, &description,
			&columnPatterns, &tablePatterns,
			&contentPatterns, &t.ContentPatternConfidence,
			&examples, &t.IsBuiltin,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		t.Description = description.String
		t.ColumnPatterns = columnPatterns
		t.TablePatterns = tablePatterns
		t.ContentPatterns = contentPatterns
		t.Examples = examples
		types = append(types, t)
	}

	return types, nil
}

// GetClassificationTypeByName retrieves a classification type by category and name
func (r *DataFlowRepository) GetClassificationTypeByName(category, typeName string) (*domain.DataClassificationType, error) {
	query := `
		SELECT
			dct.id, dct.category_id, dcc.name,
			dct.name, dct.display_name, dct.description,
			dct.column_patterns, dct.content_patterns, dct.content_pattern_confidence
		FROM data_classification_types dct
		JOIN data_classification_categories dcc ON dct.category_id = dcc.id
		WHERE dcc.name = $1 AND dct.name = $2`

	t := &domain.DataClassificationType{}
	var description sql.NullString
	var columnPatterns, contentPatterns pq.StringArray

	err := r.db.QueryRow(query, category, typeName).Scan(
		&t.ID, &t.CategoryID, &t.CategoryName,
		&t.Name, &t.DisplayName, &description,
		&columnPatterns, &contentPatterns, &t.ContentPatternConfidence,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	t.Description = description.String
	t.ColumnPatterns = columnPatterns
	t.ContentPatterns = contentPatterns
	return t, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(i int) sql.NullInt32 {
	if i == 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(i), Valid: true}
}

func nullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

func uuidsToStrings(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}

func stringsToUUIDs(strs []string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		if id, err := uuid.Parse(s); err == nil {
			result = append(result, id)
		}
	}
	return result
}

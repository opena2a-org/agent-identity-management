package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type BehaviorBaselineRepository struct {
	db *sql.DB
}

func NewBehaviorBaselineRepository(db *sql.DB) *BehaviorBaselineRepository {
	return &BehaviorBaselineRepository{db: db}
}

// GetByAgentID returns the baseline for an agent (nil if not exists)
func (r *BehaviorBaselineRepository) GetByAgentID(agentID uuid.UUID) (*domain.AgentBehaviorBaseline, error) {
	query := `
		SELECT
			id, agent_id, organization_id,
			baseline_start_at, baseline_end_at, samples_count,
			avg_calls_per_hour, stddev_calls_per_hour, max_calls_per_hour,
			capability_usage, resource_patterns, action_sequences,
			avg_risk_score, stddev_risk_score,
			is_mature, maturity_threshold, last_updated_at, created_at
		FROM agent_behavior_baselines
		WHERE agent_id = $1
	`

	baseline := &domain.AgentBehaviorBaseline{}
	var capabilityUsageJSON, resourcePatternsJSON, actionSequencesJSON []byte

	err := r.db.QueryRow(query, agentID).Scan(
		&baseline.ID,
		&baseline.AgentID,
		&baseline.OrganizationID,
		&baseline.BaselineStartAt,
		&baseline.BaselineEndAt,
		&baseline.SamplesCount,
		&baseline.AvgCallsPerHour,
		&baseline.StddevCallsPerHour,
		&baseline.MaxCallsPerHour,
		&capabilityUsageJSON,
		&resourcePatternsJSON,
		&actionSequencesJSON,
		&baseline.AvgRiskScore,
		&baseline.StddevRiskScore,
		&baseline.IsMature,
		&baseline.MaturityThreshold,
		&baseline.LastUpdatedAt,
		&baseline.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if len(capabilityUsageJSON) > 0 {
		if err := json.Unmarshal(capabilityUsageJSON, &baseline.CapabilityUsage); err != nil {
			baseline.CapabilityUsage = make(map[string]int)
		}
	} else {
		baseline.CapabilityUsage = make(map[string]int)
	}

	if len(resourcePatternsJSON) > 0 {
		if err := json.Unmarshal(resourcePatternsJSON, &baseline.ResourcePatterns); err != nil {
			baseline.ResourcePatterns = make(map[string]int)
		}
	} else {
		baseline.ResourcePatterns = make(map[string]int)
	}

	if len(actionSequencesJSON) > 0 {
		if err := json.Unmarshal(actionSequencesJSON, &baseline.ActionSequences); err != nil {
			baseline.ActionSequences = []domain.ActionSequence{}
		}
	} else {
		baseline.ActionSequences = []domain.ActionSequence{}
	}

	return baseline, nil
}

// Upsert creates or updates a baseline
func (r *BehaviorBaselineRepository) Upsert(baseline *domain.AgentBehaviorBaseline) error {
	if baseline.ID == uuid.Nil {
		baseline.ID = uuid.New()
	}
	if baseline.CreatedAt.IsZero() {
		baseline.CreatedAt = time.Now()
	}
	baseline.LastUpdatedAt = time.Now()

	// Initialize maps if nil
	if baseline.CapabilityUsage == nil {
		baseline.CapabilityUsage = make(map[string]int)
	}
	if baseline.ResourcePatterns == nil {
		baseline.ResourcePatterns = make(map[string]int)
	}
	if baseline.ActionSequences == nil {
		baseline.ActionSequences = []domain.ActionSequence{}
	}

	// Serialize JSON fields
	capabilityUsageJSON, err := json.Marshal(baseline.CapabilityUsage)
	if err != nil {
		return err
	}
	resourcePatternsJSON, err := json.Marshal(baseline.ResourcePatterns)
	if err != nil {
		return err
	}
	actionSequencesJSON, err := json.Marshal(baseline.ActionSequences)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO agent_behavior_baselines (
			id, agent_id, organization_id,
			baseline_start_at, baseline_end_at, samples_count,
			avg_calls_per_hour, stddev_calls_per_hour, max_calls_per_hour,
			capability_usage, resource_patterns, action_sequences,
			avg_risk_score, stddev_risk_score,
			is_mature, maturity_threshold, last_updated_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (agent_id) DO UPDATE SET
			baseline_start_at = EXCLUDED.baseline_start_at,
			baseline_end_at = EXCLUDED.baseline_end_at,
			samples_count = EXCLUDED.samples_count,
			avg_calls_per_hour = EXCLUDED.avg_calls_per_hour,
			stddev_calls_per_hour = EXCLUDED.stddev_calls_per_hour,
			max_calls_per_hour = EXCLUDED.max_calls_per_hour,
			capability_usage = EXCLUDED.capability_usage,
			resource_patterns = EXCLUDED.resource_patterns,
			action_sequences = EXCLUDED.action_sequences,
			avg_risk_score = EXCLUDED.avg_risk_score,
			stddev_risk_score = EXCLUDED.stddev_risk_score,
			is_mature = EXCLUDED.is_mature,
			maturity_threshold = EXCLUDED.maturity_threshold,
			last_updated_at = EXCLUDED.last_updated_at
	`

	_, err = r.db.Exec(query,
		baseline.ID,
		baseline.AgentID,
		baseline.OrganizationID,
		baseline.BaselineStartAt,
		baseline.BaselineEndAt,
		baseline.SamplesCount,
		baseline.AvgCallsPerHour,
		baseline.StddevCallsPerHour,
		baseline.MaxCallsPerHour,
		capabilityUsageJSON,
		resourcePatternsJSON,
		actionSequencesJSON,
		baseline.AvgRiskScore,
		baseline.StddevRiskScore,
		baseline.IsMature,
		baseline.MaturityThreshold,
		baseline.LastUpdatedAt,
		baseline.CreatedAt,
	)
	return err
}

// GetMatureByOrganization returns all mature baselines for an organization
func (r *BehaviorBaselineRepository) GetMatureByOrganization(orgID uuid.UUID) ([]*domain.AgentBehaviorBaseline, error) {
	query := `
		SELECT
			id, agent_id, organization_id,
			baseline_start_at, baseline_end_at, samples_count,
			avg_calls_per_hour, stddev_calls_per_hour, max_calls_per_hour,
			capability_usage, resource_patterns, action_sequences,
			avg_risk_score, stddev_risk_score,
			is_mature, maturity_threshold, last_updated_at, created_at
		FROM agent_behavior_baselines
		WHERE organization_id = $1 AND is_mature = true
		ORDER BY last_updated_at DESC
	`

	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var baselines []*domain.AgentBehaviorBaseline
	for rows.Next() {
		baseline := &domain.AgentBehaviorBaseline{}
		var capabilityUsageJSON, resourcePatternsJSON, actionSequencesJSON []byte

		err := rows.Scan(
			&baseline.ID,
			&baseline.AgentID,
			&baseline.OrganizationID,
			&baseline.BaselineStartAt,
			&baseline.BaselineEndAt,
			&baseline.SamplesCount,
			&baseline.AvgCallsPerHour,
			&baseline.StddevCallsPerHour,
			&baseline.MaxCallsPerHour,
			&capabilityUsageJSON,
			&resourcePatternsJSON,
			&actionSequencesJSON,
			&baseline.AvgRiskScore,
			&baseline.StddevRiskScore,
			&baseline.IsMature,
			&baseline.MaturityThreshold,
			&baseline.LastUpdatedAt,
			&baseline.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if len(capabilityUsageJSON) > 0 {
			json.Unmarshal(capabilityUsageJSON, &baseline.CapabilityUsage)
		}
		if baseline.CapabilityUsage == nil {
			baseline.CapabilityUsage = make(map[string]int)
		}

		if len(resourcePatternsJSON) > 0 {
			json.Unmarshal(resourcePatternsJSON, &baseline.ResourcePatterns)
		}
		if baseline.ResourcePatterns == nil {
			baseline.ResourcePatterns = make(map[string]int)
		}

		if len(actionSequencesJSON) > 0 {
			json.Unmarshal(actionSequencesJSON, &baseline.ActionSequences)
		}
		if baseline.ActionSequences == nil {
			baseline.ActionSequences = []domain.ActionSequence{}
		}

		baselines = append(baselines, baseline)
	}

	return baselines, nil
}

// Delete removes a baseline for an agent
func (r *BehaviorBaselineRepository) Delete(agentID uuid.UUID) error {
	query := `DELETE FROM agent_behavior_baselines WHERE agent_id = $1`
	_, err := r.db.Exec(query, agentID)
	return err
}

// RecordAnomaly stores a detected anomaly
func (r *BehaviorBaselineRepository) RecordAnomaly(anomaly *domain.BehavioralAnomaly) error {
	if anomaly.ID == uuid.Nil {
		anomaly.ID = uuid.New()
	}
	if anomaly.DetectedAt.IsZero() {
		anomaly.DetectedAt = time.Now()
	}
	if anomaly.Metadata == nil {
		anomaly.Metadata = make(map[string]interface{})
	}

	metadataJSON, err := json.Marshal(anomaly.Metadata)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO behavioral_anomalies (
			id, agent_id, organization_id,
			anomaly_type, severity, description,
			expected_value, actual_value, deviation_sigma,
			trigger_action, trigger_resource, trigger_capability,
			metadata, detected_at, alert_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err = r.db.Exec(query,
		anomaly.ID,
		anomaly.AgentID,
		anomaly.OrganizationID,
		anomaly.AnomalyType,
		anomaly.Severity,
		anomaly.Description,
		anomaly.ExpectedValue,
		anomaly.ActualValue,
		anomaly.DeviationSigma,
		anomaly.TriggerAction,
		anomaly.TriggerResource,
		anomaly.TriggerCapability,
		metadataJSON,
		anomaly.DetectedAt,
		anomaly.AlertID,
	)
	return err
}

// GetRecentAnomalies returns recent anomalies for an agent
func (r *BehaviorBaselineRepository) GetRecentAnomalies(agentID uuid.UUID, limit int) ([]*domain.BehavioralAnomaly, error) {
	query := `
		SELECT
			id, agent_id, organization_id,
			anomaly_type, severity, description,
			expected_value, actual_value, deviation_sigma,
			trigger_action, trigger_resource, trigger_capability,
			metadata, detected_at, alert_id
		FROM behavioral_anomalies
		WHERE agent_id = $1
		ORDER BY detected_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAnomalies(rows)
}

// GetAnomaliesByType returns anomalies by type for an organization
func (r *BehaviorBaselineRepository) GetAnomaliesByType(orgID uuid.UUID, anomalyType domain.BehavioralAnomalyType, limit int) ([]*domain.BehavioralAnomaly, error) {
	query := `
		SELECT
			id, agent_id, organization_id,
			anomaly_type, severity, description,
			expected_value, actual_value, deviation_sigma,
			trigger_action, trigger_resource, trigger_capability,
			metadata, detected_at, alert_id
		FROM behavioral_anomalies
		WHERE organization_id = $1 AND anomaly_type = $2
		ORDER BY detected_at DESC
		LIMIT $3
	`

	rows, err := r.db.Query(query, orgID, anomalyType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAnomalies(rows)
}

// scanAnomalies is a helper to scan anomaly rows
func (r *BehaviorBaselineRepository) scanAnomalies(rows *sql.Rows) ([]*domain.BehavioralAnomaly, error) {
	var anomalies []*domain.BehavioralAnomaly

	for rows.Next() {
		anomaly := &domain.BehavioralAnomaly{}
		var metadataJSON []byte

		err := rows.Scan(
			&anomaly.ID,
			&anomaly.AgentID,
			&anomaly.OrganizationID,
			&anomaly.AnomalyType,
			&anomaly.Severity,
			&anomaly.Description,
			&anomaly.ExpectedValue,
			&anomaly.ActualValue,
			&anomaly.DeviationSigma,
			&anomaly.TriggerAction,
			&anomaly.TriggerResource,
			&anomaly.TriggerCapability,
			&metadataJSON,
			&anomaly.DetectedAt,
			&anomaly.AlertID,
		)
		if err != nil {
			return nil, err
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &anomaly.Metadata)
		}
		if anomaly.Metadata == nil {
			anomaly.Metadata = make(map[string]interface{})
		}

		anomalies = append(anomalies, anomaly)
	}

	return anomalies, nil
}

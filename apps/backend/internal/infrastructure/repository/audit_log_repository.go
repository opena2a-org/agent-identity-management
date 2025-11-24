package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type AuditLogRepository struct {
	db *sql.DB
}

func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

func (r *AuditLogRepository) Create(log *domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(query,
		log.ID,
		log.OrganizationID,
		log.UserID,
		log.Action,
		log.ResourceType,
		log.ResourceID,
		log.IPAddress,
		log.UserAgent,
		metadataJSON,
		log.Timestamp,
	)
	return err
}

func (r *AuditLogRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE organization_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *AuditLogRepository) GetByUser(userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *AuditLogRepository) GetByResource(resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(query, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *AuditLogRepository) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	// This would integrate with Elasticsearch for full-text search
	// For now, implement basic SQL search
	sqlQuery := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE action LIKE $1 OR resource_type LIKE $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.Query(sqlQuery, searchPattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

func (r *AuditLogRepository) scanLogs(rows *sql.Rows) ([]*domain.AuditLog, error) {
	var logs []*domain.AuditLog

	for rows.Next() {
		log := &domain.AuditLog{}
		var metadataJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.OrganizationID,
			&log.UserID,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&log.IPAddress,
			&log.UserAgent,
			&metadataJSON,
			&log.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
				return nil, err
			}
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// CountActionsByAgentInTimeWindow counts how many times an agent performed an action in the last N minutes
func (r *AuditLogRepository) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE resource_id = $1
		  AND resource_type = 'agent'
		  AND action = $2
		  AND timestamp >= NOW() - INTERVAL '1 minute' * $3
	`

	var count int
	err := r.db.QueryRow(query, agentID, action, windowMinutes).Scan(&count)
	return count, err
}

// GetRecentActionsByAgent gets the most recent actions by an agent
func (r *AuditLogRepository) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE resource_id = $1 AND resource_type = 'agent'
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

// GetAgentActionsByIPAddress gets actions by an agent from a specific IP address
func (r *AuditLogRepository) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	query := `
		SELECT id, organization_id, user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, timestamp
		FROM audit_logs
		WHERE resource_id = $1
		  AND resource_type = 'agent'
		  AND ip_address = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.Query(query, agentID, ipAddress, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanLogs(rows)
}

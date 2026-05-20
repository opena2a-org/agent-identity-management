package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// DataTransferRepository implements domain.DataTransferRepository
type DataTransferRepository struct {
	db *sql.DB
}

// NewDataTransferRepository creates a new data transfer repository
func NewDataTransferRepository(db *sql.DB) *DataTransferRepository {
	return &DataTransferRepository{db: db}
}

// RecordTransfer records a data transfer
func (r *DataTransferRepository) RecordTransfer(transfer *domain.DataTransfer) error {
	query := `
		INSERT INTO data_transfers (id, agent_id, organization_id, action_type, resource, destination_ip, destination_domain, request_size, response_size, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	if transfer.ID == uuid.Nil {
		transfer.ID = uuid.New()
	}
	if transfer.CreatedAt.IsZero() {
		transfer.CreatedAt = time.Now()
	}

	metadataJSON := make([]byte, 0)
	var err error
	if transfer.Metadata != nil {
		metadataJSON, err = json.Marshal(transfer.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}

	_, err = r.db.Exec(query,
		transfer.ID,
		transfer.AgentID,
		transfer.OrganizationID,
		transfer.ActionType,
		transfer.Resource,
		transfer.DestinationIP,
		transfer.DestinationDomain,
		transfer.RequestSize,
		transfer.ResponseSize,
		metadataJSON,
		transfer.CreatedAt,
	)
	return err
}

// GetRecentTransfers gets recent transfers for an agent
func (r *DataTransferRepository) GetRecentTransfers(agentID uuid.UUID, limit int) ([]*domain.DataTransfer, error) {
	query := `
		SELECT id, agent_id, organization_id, action_type, resource, destination_ip, destination_domain, request_size, response_size, COALESCE(metadata, '{}'), created_at
		FROM data_transfers
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTransfers(rows)
}

// GetTransfersInWindow gets transfers within a time window
func (r *DataTransferRepository) GetTransfersInWindow(agentID uuid.UUID, windowMinutes int) ([]*domain.DataTransfer, error) {
	query := `
		SELECT id, agent_id, organization_id, action_type, resource, destination_ip, destination_domain, request_size, response_size, COALESCE(metadata, '{}'), created_at
		FROM data_transfers
		WHERE agent_id = $1 AND created_at > NOW() - INTERVAL '1 minute' * $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, agentID, windowMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTransfers(rows)
}

// GetCumulativeSize gets cumulative transfer size for an agent within a time window
func (r *DataTransferRepository) GetCumulativeSize(agentID uuid.UUID, windowMinutes int) (requestBytes, responseBytes int64, err error) {
	query := `
		SELECT COALESCE(SUM(request_size), 0), COALESCE(SUM(response_size), 0)
		FROM data_transfers
		WHERE agent_id = $1 AND created_at > NOW() - INTERVAL '1 minute' * $2
	`

	err = r.db.QueryRow(query, agentID, windowMinutes).Scan(&requestBytes, &responseBytes)
	return
}

// GetOrCreateCurrentAggregate gets or creates the aggregate for the current hour
func (r *DataTransferRepository) GetOrCreateCurrentAggregate(agentID, orgID uuid.UUID) (*domain.DataTransferAggregate, error) {
	now := time.Now()
	bucketStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	bucketEnd := bucketStart.Add(time.Hour)

	// Try to get existing aggregate
	query := `
		SELECT id, agent_id, organization_id, bucket_start, bucket_end, total_request_bytes, total_response_bytes, transfer_count, unique_destinations, COALESCE(top_destinations, '[]'), created_at, updated_at
		FROM data_transfer_aggregates
		WHERE agent_id = $1 AND bucket_start = $2
	`

	aggregate := &domain.DataTransferAggregate{}
	topDestJSON := make([]byte, 0)

	err := r.db.QueryRow(query, agentID, bucketStart).Scan(
		&aggregate.ID,
		&aggregate.AgentID,
		&aggregate.OrganizationID,
		&aggregate.BucketStart,
		&aggregate.BucketEnd,
		&aggregate.TotalRequestBytes,
		&aggregate.TotalResponseBytes,
		&aggregate.TransferCount,
		&aggregate.UniqueDestinations,
		&topDestJSON,
		&aggregate.CreatedAt,
		&aggregate.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create new aggregate
		aggregate = &domain.DataTransferAggregate{
			ID:             uuid.New(),
			AgentID:        agentID,
			OrganizationID: orgID,
			BucketStart:    bucketStart,
			BucketEnd:      bucketEnd,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		insertQuery := `
			INSERT INTO data_transfer_aggregates (id, agent_id, organization_id, bucket_start, bucket_end, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err = r.db.Exec(insertQuery,
			aggregate.ID,
			aggregate.AgentID,
			aggregate.OrganizationID,
			aggregate.BucketStart,
			aggregate.BucketEnd,
			aggregate.CreatedAt,
			aggregate.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create aggregate: %w", err)
		}
		return aggregate, nil
	}

	if err != nil {
		return nil, err
	}

	// Parse top destinations
	if len(topDestJSON) > 0 {
		json.Unmarshal(topDestJSON, &aggregate.TopDestinations)
	}

	return aggregate, nil
}

// UpdateAggregate updates an aggregate with new transfer data
func (r *DataTransferRepository) UpdateAggregate(aggregate *domain.DataTransferAggregate) error {
	topDestJSON, err := json.Marshal(aggregate.TopDestinations)
	if err != nil {
		topDestJSON = []byte("[]")
	}

	query := `
		UPDATE data_transfer_aggregates
		SET total_request_bytes = $2, total_response_bytes = $3, transfer_count = $4, unique_destinations = $5, top_destinations = $6, updated_at = NOW()
		WHERE id = $1
	`

	_, err = r.db.Exec(query,
		aggregate.ID,
		aggregate.TotalRequestBytes,
		aggregate.TotalResponseBytes,
		aggregate.TransferCount,
		aggregate.UniqueDestinations,
		topDestJSON,
	)
	return err
}

// GetAggregates gets aggregates for an agent within a time range
func (r *DataTransferRepository) GetAggregates(agentID uuid.UUID, since time.Time) ([]*domain.DataTransferAggregate, error) {
	query := `
		SELECT id, agent_id, organization_id, bucket_start, bucket_end, total_request_bytes, total_response_bytes, transfer_count, unique_destinations, COALESCE(top_destinations, '[]'), created_at, updated_at
		FROM data_transfer_aggregates
		WHERE agent_id = $1 AND bucket_start >= $2
		ORDER BY bucket_start DESC
	`

	rows, err := r.db.Query(query, agentID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	aggregates := make([]*domain.DataTransferAggregate, 0)
	for rows.Next() {
		agg := &domain.DataTransferAggregate{}
		topDestJSON := make([]byte, 0)

		err := rows.Scan(
			&agg.ID,
			&agg.AgentID,
			&agg.OrganizationID,
			&agg.BucketStart,
			&agg.BucketEnd,
			&agg.TotalRequestBytes,
			&agg.TotalResponseBytes,
			&agg.TransferCount,
			&agg.UniqueDestinations,
			&topDestJSON,
			&agg.CreatedAt,
			&agg.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(topDestJSON) > 0 {
			json.Unmarshal(topDestJSON, &agg.TopDestinations)
		}

		aggregates = append(aggregates, agg)
	}

	return aggregates, nil
}

// GetTopTransferringAgents gets top transferring agents for an organization
func (r *DataTransferRepository) GetTopTransferringAgents(orgID uuid.UUID, windowHours int, limit int) ([]domain.AgentTransferSummary, error) {
	query := `
		SELECT
			dt.agent_id,
			a.name,
			SUM(dt.response_size) as total_bytes,
			COUNT(*) as transfer_count,
			MODE() WITHIN GROUP (ORDER BY COALESCE(dt.destination_domain, dt.destination_ip)) as top_destination
		FROM data_transfers dt
		JOIN agents a ON dt.agent_id = a.id
		WHERE dt.organization_id = $1 AND dt.created_at > NOW() - INTERVAL '1 hour' * $2
		GROUP BY dt.agent_id, a.name
		ORDER BY total_bytes DESC
		LIMIT $3
	`

	rows, err := r.db.Query(query, orgID, windowHours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]domain.AgentTransferSummary, 0)
	for rows.Next() {
		var s domain.AgentTransferSummary
		var topDest sql.NullString

		err := rows.Scan(&s.AgentID, &s.AgentName, &s.TotalBytes, &s.TransferCount, &topDest)
		if err != nil {
			return nil, err
		}

		if topDest.Valid {
			s.TopDestination = topDest.String
		}

		summaries = append(summaries, s)
	}

	return summaries, nil
}

// CleanupOldRecords removes records older than the specified duration
func (r *DataTransferRepository) CleanupOldRecords(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	// Delete old transfers
	result, err := r.db.Exec("DELETE FROM data_transfers WHERE created_at < $1", cutoff)
	if err != nil {
		return 0, err
	}

	transfersDeleted, _ := result.RowsAffected()

	// Delete old aggregates
	result, err = r.db.Exec("DELETE FROM data_transfer_aggregates WHERE bucket_end < $1", cutoff)
	if err != nil {
		return transfersDeleted, err
	}

	aggDeleted, _ := result.RowsAffected()

	return transfersDeleted + aggDeleted, nil
}

// scanTransfers scans rows into DataTransfer slice
func (r *DataTransferRepository) scanTransfers(rows *sql.Rows) ([]*domain.DataTransfer, error) {
	transfers := make([]*domain.DataTransfer, 0)

	for rows.Next() {
		t := &domain.DataTransfer{}
		metadataJSON := make([]byte, 0)
		var resource, destIP, destDomain sql.NullString

		err := rows.Scan(
			&t.ID,
			&t.AgentID,
			&t.OrganizationID,
			&t.ActionType,
			&resource,
			&destIP,
			&destDomain,
			&t.RequestSize,
			&t.ResponseSize,
			&metadataJSON,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if resource.Valid {
			t.Resource = resource.String
		}
		if destIP.Valid {
			t.DestinationIP = destIP.String
		}
		if destDomain.Valid {
			t.DestinationDomain = destDomain.String
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &t.Metadata)
		}

		transfers = append(transfers, t)
	}

	return transfers, nil
}

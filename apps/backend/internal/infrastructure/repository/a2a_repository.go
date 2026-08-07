package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ============================================================================
// A2A Agent Card Repository
// ============================================================================

// A2AAgentCardRepository implements domain.A2AAgentCardRepository
type A2AAgentCardRepository struct {
	db *sql.DB
}

// NewA2AAgentCardRepository creates a new A2AAgentCardRepository
func NewA2AAgentCardRepository(db *sql.DB) *A2AAgentCardRepository {
	return &A2AAgentCardRepository{db: db}
}

func (r *A2AAgentCardRepository) Create(ctx context.Context, card *domain.A2AAgentCard) error {
	query := `
		INSERT INTO a2a_agent_cards (
			id, agent_id, card_url, card_data, card_hash, protocol_version,
			attestation_signature, attestation_issued_at, attestation_expires_at,
			is_valid, validation_error, last_fetched_at, fetch_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (agent_id) DO UPDATE SET
			card_url = EXCLUDED.card_url,
			card_data = EXCLUDED.card_data,
			card_hash = EXCLUDED.card_hash,
			protocol_version = EXCLUDED.protocol_version,
			attestation_signature = EXCLUDED.attestation_signature,
			attestation_issued_at = EXCLUDED.attestation_issued_at,
			attestation_expires_at = EXCLUDED.attestation_expires_at,
			is_valid = EXCLUDED.is_valid,
			validation_error = EXCLUDED.validation_error,
			last_fetched_at = EXCLUDED.last_fetched_at,
			fetch_count = a2a_agent_cards.fetch_count + 1,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at, fetch_count
	`

	if card.ID == uuid.Nil {
		card.ID = uuid.New()
	}
	now := time.Now().UTC()

	err := r.db.QueryRowContext(ctx, query,
		card.ID,
		card.AgentID,
		card.CardURL,
		card.CardData,
		card.CardHash,
		card.ProtocolVersion,
		nullString(card.AttestationSignature),
		nullTime(card.AttestationIssuedAt),
		nullTime(card.AttestationExpiresAt),
		card.IsValid,
		nullString(card.ValidationError),
		nullTime(card.LastFetchedAt),
		card.FetchCount,
		now,
		now,
	).Scan(&card.ID, &card.CreatedAt, &card.UpdatedAt, &card.FetchCount)

	if err != nil {
		return fmt.Errorf("failed to create A2A agent card: %w", err)
	}

	return nil
}

func (r *A2AAgentCardRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*domain.A2AAgentCard, error) {
	query := `
		SELECT
			id, agent_id, card_url, card_data, card_hash, protocol_version,
			attestation_signature, attestation_issued_at, attestation_expires_at,
			is_valid, validation_error, last_fetched_at, fetch_count, created_at, updated_at
		FROM a2a_agent_cards
		WHERE agent_id = $1
	`

	card := &domain.A2AAgentCard{}
	var attestSig, validationErr sql.NullString
	var attestIssued, attestExpires, lastFetched sql.NullTime

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&card.ID,
		&card.AgentID,
		&card.CardURL,
		&card.CardData,
		&card.CardHash,
		&card.ProtocolVersion,
		&attestSig,
		&attestIssued,
		&attestExpires,
		&card.IsValid,
		&validationErr,
		&lastFetched,
		&card.FetchCount,
		&card.CreatedAt,
		&card.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get A2A agent card: %w", err)
	}

	card.AttestationSignature = attestSig.String
	card.ValidationError = validationErr.String
	if attestIssued.Valid {
		card.AttestationIssuedAt = &attestIssued.Time
	}
	if attestExpires.Valid {
		card.AttestationExpiresAt = &attestExpires.Time
	}
	if lastFetched.Valid {
		card.LastFetchedAt = &lastFetched.Time
	}

	return card, nil
}

func (r *A2AAgentCardRepository) Update(ctx context.Context, card *domain.A2AAgentCard) error {
	query := `
		UPDATE a2a_agent_cards SET
			card_url = $1,
			card_data = $2,
			card_hash = $3,
			protocol_version = $4,
			attestation_signature = $5,
			attestation_issued_at = $6,
			attestation_expires_at = $7,
			is_valid = $8,
			validation_error = $9,
			last_fetched_at = $10,
			fetch_count = $11,
			updated_at = $12
		WHERE id = $13
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		card.CardURL,
		card.CardData,
		card.CardHash,
		card.ProtocolVersion,
		nullString(card.AttestationSignature),
		nullTime(card.AttestationIssuedAt),
		nullTime(card.AttestationExpiresAt),
		card.IsValid,
		nullString(card.ValidationError),
		nullTime(card.LastFetchedAt),
		card.FetchCount,
		time.Now().UTC(),
		card.ID,
	).Scan(&card.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update A2A agent card: %w", err)
	}

	return nil
}

func (r *A2AAgentCardRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM a2a_agent_cards WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete A2A agent card: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("A2A agent card not found")
	}
	return nil
}

func (r *A2AAgentCardRepository) GetValidCards(ctx context.Context, limit, offset int) ([]*domain.A2AAgentCard, error) {
	query := `
		SELECT
			id, agent_id, card_url, card_data, card_hash, protocol_version,
			attestation_signature, attestation_issued_at, attestation_expires_at,
			is_valid, validation_error, last_fetched_at, fetch_count, created_at, updated_at
		FROM a2a_agent_cards
		WHERE is_valid = TRUE
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`
	return r.queryCards(ctx, query, limit, offset)
}

func (r *A2AAgentCardRepository) GetExpiredCards(ctx context.Context) ([]*domain.A2AAgentCard, error) {
	query := `
		SELECT
			id, agent_id, card_url, card_data, card_hash, protocol_version,
			attestation_signature, attestation_issued_at, attestation_expires_at,
			is_valid, validation_error, last_fetched_at, fetch_count, created_at, updated_at
		FROM a2a_agent_cards
		WHERE attestation_expires_at < NOW() AND is_valid = TRUE
		ORDER BY attestation_expires_at
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired cards: %w", err)
	}
	defer rows.Close()
	return r.scanCards(rows)
}

func (r *A2AAgentCardRepository) queryCards(ctx context.Context, query string, args ...interface{}) ([]*domain.A2AAgentCard, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query cards: %w", err)
	}
	defer rows.Close()
	return r.scanCards(rows)
}

func (r *A2AAgentCardRepository) scanCards(rows *sql.Rows) ([]*domain.A2AAgentCard, error) {
	cards := make([]*domain.A2AAgentCard, 0)
	for rows.Next() {
		card := &domain.A2AAgentCard{}
		var attestSig, validationErr sql.NullString
		var attestIssued, attestExpires, lastFetched sql.NullTime

		err := rows.Scan(
			&card.ID,
			&card.AgentID,
			&card.CardURL,
			&card.CardData,
			&card.CardHash,
			&card.ProtocolVersion,
			&attestSig,
			&attestIssued,
			&attestExpires,
			&card.IsValid,
			&validationErr,
			&lastFetched,
			&card.FetchCount,
			&card.CreatedAt,
			&card.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan card: %w", err)
		}

		card.AttestationSignature = attestSig.String
		card.ValidationError = validationErr.String
		if attestIssued.Valid {
			card.AttestationIssuedAt = &attestIssued.Time
		}
		if attestExpires.Valid {
			card.AttestationExpiresAt = &attestExpires.Time
		}
		if lastFetched.Valid {
			card.LastFetchedAt = &lastFetched.Time
		}

		cards = append(cards, card)
	}
	return cards, nil
}

// ============================================================================
// A2A Skill Repository
// ============================================================================

// A2ASkillRepository implements domain.A2ASkillRepository
type A2ASkillRepository struct {
	db *sql.DB
}

// NewA2ASkillRepository creates a new A2ASkillRepository
func NewA2ASkillRepository(db *sql.DB) *A2ASkillRepository {
	return &A2ASkillRepository{db: db}
}

func (r *A2ASkillRepository) Create(ctx context.Context, skill *domain.A2ASkill) error {
	query := `
		INSERT INTO a2a_skills (
			id, agent_id, skill_id, name, description, tags, input_modes, output_modes,
			examples, invocation_count, success_count, failure_count, avg_duration_ms,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (agent_id, skill_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			tags = EXCLUDED.tags,
			input_modes = EXCLUDED.input_modes,
			output_modes = EXCLUDED.output_modes,
			examples = EXCLUDED.examples,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	if skill.ID == uuid.Nil {
		skill.ID = uuid.New()
	}
	now := time.Now().UTC()

	tagsJSON, _ := json.Marshal(skill.Tags)
	inputModesJSON, _ := json.Marshal(skill.InputModes)
	outputModesJSON, _ := json.Marshal(skill.OutputModes)
	examplesJSON, _ := json.Marshal(skill.Examples)

	err := r.db.QueryRowContext(ctx, query,
		skill.ID,
		skill.AgentID,
		skill.SkillID,
		skill.Name,
		skill.Description,
		tagsJSON,
		inputModesJSON,
		outputModesJSON,
		examplesJSON,
		skill.InvocationCount,
		skill.SuccessCount,
		skill.FailureCount,
		nullInt(skill.AvgDurationMs),
		now,
		now,
	).Scan(&skill.ID, &skill.CreatedAt, &skill.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create A2A skill: %w", err)
	}

	return nil
}

func (r *A2ASkillRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]*domain.A2ASkill, error) {
	query := `
		SELECT
			id, agent_id, skill_id, name, description, tags, input_modes, output_modes,
			examples, invocation_count, success_count, failure_count, avg_duration_ms,
			created_at, updated_at
		FROM a2a_skills
		WHERE agent_id = $1
		ORDER BY name
	`
	return r.querySkills(ctx, query, agentID)
}

func (r *A2ASkillRepository) GetBySkillID(ctx context.Context, agentID uuid.UUID, skillID string) (*domain.A2ASkill, error) {
	query := `
		SELECT
			id, agent_id, skill_id, name, description, tags, input_modes, output_modes,
			examples, invocation_count, success_count, failure_count, avg_duration_ms,
			created_at, updated_at
		FROM a2a_skills
		WHERE agent_id = $1 AND skill_id = $2
	`

	skill := &domain.A2ASkill{}
	var tagsJSON, inputModesJSON, outputModesJSON, examplesJSON []byte
	var avgDuration sql.NullInt32

	err := r.db.QueryRowContext(ctx, query, agentID, skillID).Scan(
		&skill.ID,
		&skill.AgentID,
		&skill.SkillID,
		&skill.Name,
		&skill.Description,
		&tagsJSON,
		&inputModesJSON,
		&outputModesJSON,
		&examplesJSON,
		&skill.InvocationCount,
		&skill.SuccessCount,
		&skill.FailureCount,
		&avgDuration,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get A2A skill: %w", err)
	}

	_ = json.Unmarshal(tagsJSON, &skill.Tags)
	_ = json.Unmarshal(inputModesJSON, &skill.InputModes)
	_ = json.Unmarshal(outputModesJSON, &skill.OutputModes)
	_ = json.Unmarshal(examplesJSON, &skill.Examples)
	if avgDuration.Valid {
		skill.AvgDurationMs = int(avgDuration.Int32)
	}

	return skill, nil
}

func (r *A2ASkillRepository) Update(ctx context.Context, skill *domain.A2ASkill) error {
	query := `
		UPDATE a2a_skills SET
			name = $1,
			description = $2,
			tags = $3,
			input_modes = $4,
			output_modes = $5,
			examples = $6,
			invocation_count = $7,
			success_count = $8,
			failure_count = $9,
			avg_duration_ms = $10,
			updated_at = $11
		WHERE id = $12
		RETURNING updated_at
	`

	tagsJSON, _ := json.Marshal(skill.Tags)
	inputModesJSON, _ := json.Marshal(skill.InputModes)
	outputModesJSON, _ := json.Marshal(skill.OutputModes)
	examplesJSON, _ := json.Marshal(skill.Examples)

	err := r.db.QueryRowContext(ctx, query,
		skill.Name,
		skill.Description,
		tagsJSON,
		inputModesJSON,
		outputModesJSON,
		examplesJSON,
		skill.InvocationCount,
		skill.SuccessCount,
		skill.FailureCount,
		nullInt(skill.AvgDurationMs),
		time.Now().UTC(),
		skill.ID,
	).Scan(&skill.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update A2A skill: %w", err)
	}

	return nil
}

func (r *A2ASkillRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM a2a_skills WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete A2A skill: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("A2A skill not found")
	}
	return nil
}

func (r *A2ASkillRepository) DeleteByAgentID(ctx context.Context, agentID uuid.UUID) error {
	query := `DELETE FROM a2a_skills WHERE agent_id = $1`
	_, err := r.db.ExecContext(ctx, query, agentID)
	if err != nil {
		return fmt.Errorf("failed to delete A2A skills: %w", err)
	}
	return nil
}

func (r *A2ASkillRepository) Search(ctx context.Context, query string, limit int) ([]*domain.A2ASkill, error) {
	sql := `
		SELECT
			id, agent_id, skill_id, name, description, tags, input_modes, output_modes,
			examples, invocation_count, success_count, failure_count, avg_duration_ms,
			created_at, updated_at
		FROM a2a_skills
		WHERE name ILIKE $1 OR description ILIKE $1 OR skill_id ILIKE $1
		ORDER BY invocation_count DESC
		LIMIT $2
	`
	return r.querySkills(ctx, sql, "%"+query+"%", limit)
}

// SearchByIntent performs intent-based agent discovery using PostgreSQL full-text search.
// Returns agents with matching skills, ranked by relevance and trust score.
func (r *A2ASkillRepository) SearchByIntent(ctx context.Context, intent string, minTrustScore float64, limit int) ([]*domain.RoutedAgent, error) {
	query := `
		SELECT
			s.id as skill_uuid,
			s.agent_id,
			s.skill_id,
			s.name as skill_name,
			s.description as skill_description,
			a.name as agent_name,
			a.status as agent_status,
			COALESCE(t.a2a_trust_score, 0.5) as trust_score,
			ts_rank(s.search_vector, plainto_tsquery('english', $1)) as relevance
		FROM a2a_skills s
		JOIN agents a ON a.id = s.agent_id
		LEFT JOIN a2a_trust_scores t ON t.agent_id = s.agent_id
		WHERE s.search_vector @@ plainto_tsquery('english', $1)
		  AND COALESCE(t.a2a_trust_score, 0.5) >= $2
		  AND a.status = 'verified'
		ORDER BY ts_rank(s.search_vector, plainto_tsquery('english', $1))
		       * COALESCE(t.a2a_trust_score, 0.5) DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, intent, minTrustScore, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search by intent: %w", err)
	}
	defer rows.Close()

	results := make([]*domain.RoutedAgent, 0)
	for rows.Next() {
		ra := &domain.RoutedAgent{}
		err := rows.Scan(
			&ra.SkillUUID,
			&ra.AgentID,
			&ra.SkillID,
			&ra.SkillName,
			&ra.SkillDescription,
			&ra.AgentName,
			&ra.AgentStatus,
			&ra.TrustScore,
			&ra.Relevance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan routed agent: %w", err)
		}
		results = append(results, ra)
	}

	return results, nil
}

// CountByIntent returns the number of agents matching an intent
func (r *A2ASkillRepository) CountByIntent(ctx context.Context, intent string, minTrustScore float64) (int, error) {
	query := `
		SELECT COUNT(DISTINCT s.agent_id)
		FROM a2a_skills s
		JOIN agents a ON a.id = s.agent_id
		LEFT JOIN a2a_trust_scores t ON t.agent_id = s.agent_id
		WHERE s.search_vector @@ plainto_tsquery('english', $1)
		  AND COALESCE(t.a2a_trust_score, 0.5) >= $2
		  AND a.status = 'verified'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, intent, minTrustScore).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count by intent: %w", err)
	}
	return count, nil
}

func (r *A2ASkillRepository) IncrementUsage(ctx context.Context, id uuid.UUID, success bool, durationMs int) error {
	var query string
	if success {
		query = `
			UPDATE a2a_skills SET
				invocation_count = invocation_count + 1,
				success_count = success_count + 1,
				avg_duration_ms = COALESCE((avg_duration_ms * invocation_count + $2) / (invocation_count + 1), $2),
				updated_at = $3
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE a2a_skills SET
				invocation_count = invocation_count + 1,
				failure_count = failure_count + 1,
				avg_duration_ms = COALESCE((avg_duration_ms * invocation_count + $2) / (invocation_count + 1), $2),
				updated_at = $3
			WHERE id = $1
		`
	}

	_, err := r.db.ExecContext(ctx, query, id, durationMs, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to increment skill usage: %w", err)
	}
	return nil
}

// MarkVerified marks a skill as verified after consensus is reached
func (r *A2ASkillRepository) MarkVerified(ctx context.Context, agentID uuid.UUID, skillID string) error {
	query := `
		UPDATE a2a_skills
		SET is_verified = TRUE, verified_at = NOW(), attestation_count = COALESCE(attestation_count, 0) + 1, updated_at = NOW()
		WHERE agent_id = $1 AND skill_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, agentID, skillID)
	return err
}

func (r *A2ASkillRepository) querySkills(ctx context.Context, query string, args ...interface{}) ([]*domain.A2ASkill, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*domain.A2ASkill, 0)
	for rows.Next() {
		skill := &domain.A2ASkill{}
		var tagsJSON, inputModesJSON, outputModesJSON, examplesJSON []byte
		var avgDuration sql.NullInt32

		err := rows.Scan(
			&skill.ID,
			&skill.AgentID,
			&skill.SkillID,
			&skill.Name,
			&skill.Description,
			&tagsJSON,
			&inputModesJSON,
			&outputModesJSON,
			&examplesJSON,
			&skill.InvocationCount,
			&skill.SuccessCount,
			&skill.FailureCount,
			&avgDuration,
			&skill.CreatedAt,
			&skill.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}

		_ = json.Unmarshal(tagsJSON, &skill.Tags)
		_ = json.Unmarshal(inputModesJSON, &skill.InputModes)
		_ = json.Unmarshal(outputModesJSON, &skill.OutputModes)
		_ = json.Unmarshal(examplesJSON, &skill.Examples)
		if avgDuration.Valid {
			skill.AvgDurationMs = int(avgDuration.Int32)
		}

		skills = append(skills, skill)
	}
	return skills, nil
}

// ============================================================================
// A2A Task Repository
// ============================================================================

// A2ATaskRepository implements domain.A2ATaskRepository
type A2ATaskRepository struct {
	db *sql.DB
}

// NewA2ATaskRepository creates a new A2ATaskRepository
func NewA2ATaskRepository(db *sql.DB) *A2ATaskRepository {
	return &A2ATaskRepository{db: db}
}

func (r *A2ATaskRepository) Create(ctx context.Context, task *domain.A2ATask) error {
	query := `
		INSERT INTO a2a_tasks (
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, created_at
	`

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}

	err := r.db.QueryRowContext(ctx, query,
		task.ID,
		task.ExternalTaskID,
		nullString(task.ContextID),
		task.ClientAgentID,
		task.RemoteAgentID,
		nullString(task.SkillID),
		task.State,
		task.CreatedAt,
		nullTime(task.StartedAt),
		nullTime(task.CompletedAt),
		nullInt(task.DurationMs),
		nullJSON(task.PolicyDecision),
		nullTime(task.PolicyEvaluatedAt),
		nullFloat(task.ClientTrustScoreSnapshot),
		nullFloat(task.RemoteTrustScoreSnapshot),
		task.MessageCount,
		nullString(task.ErrorCode),
		nullString(task.ErrorMessage),
	).Scan(&task.ID, &task.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create A2A task: %w", err)
	}

	return nil
}

func (r *A2ATaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.A2ATask, error) {
	query := `
		SELECT
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		FROM a2a_tasks
		WHERE id = $1
	`
	return r.scanTask(r.db.QueryRowContext(ctx, query, id))
}

func (r *A2ATaskRepository) GetByExternalID(ctx context.Context, externalID string) (*domain.A2ATask, error) {
	query := `
		SELECT
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		FROM a2a_tasks
		WHERE external_task_id = $1
	`
	return r.scanTask(r.db.QueryRowContext(ctx, query, externalID))
}

func (r *A2ATaskRepository) Update(ctx context.Context, task *domain.A2ATask) error {
	query := `
		UPDATE a2a_tasks SET
			state = $1,
			started_at = $2,
			completed_at = $3,
			duration_ms = $4,
			policy_decision = $5,
			policy_evaluated_at = $6,
			message_count = $7,
			error_code = $8,
			error_message = $9
		WHERE id = $10
	`

	_, err := r.db.ExecContext(ctx, query,
		task.State,
		nullTime(task.StartedAt),
		nullTime(task.CompletedAt),
		nullInt(task.DurationMs),
		nullJSON(task.PolicyDecision),
		nullTime(task.PolicyEvaluatedAt),
		task.MessageCount,
		nullString(task.ErrorCode),
		nullString(task.ErrorMessage),
		task.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update A2A task: %w", err)
	}

	return nil
}

// ListTasks returns one page of A2A tasks visible to callerOrgID.
//
// SECURITY: callerOrgID is REQUIRED and is not optional-with-a-nil-escape. This query was
// `FROM a2a_tasks WHERE 1=1` with no organization predicate anywhere in the handler,
// service or repository, and GET /api/v1/a2a/tasks is mounted behind authentication but no
// org scoping — so any authenticated user of any organization could page through every A2A
// task in the system, without even naming a target id.
//
// a2a_tasks carries no organization_id; it references two agents. A task is visible when
// EITHER end belongs to the caller's organization, which is the honest reading of a
// bilateral record: your agent took part in it, so you may see it. That deliberately does
// include tasks whose other end is a different organization's agent — that is what an A2A
// task IS — but it never discloses a task both of whose ends are foreign.
func (r *A2ATaskRepository) ListTasks(ctx context.Context, callerOrgID uuid.UUID, agentID *uuid.UUID, state string, limit, offset int) ([]*domain.A2ATask, int, error) {
	// Fail closed: a uuid.Nil caller org means the auth chain did not populate it and the
	// handler proceeded anyway. Matching Nil against real rows would disclose them.
	if callerOrgID == uuid.Nil {
		return nil, 0, fmt.Errorf("a2a task listing requires a caller organization")
	}

	baseQuery := `FROM a2a_tasks WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	baseQuery += fmt.Sprintf(`
		AND (
		  EXISTS (SELECT 1 FROM agents a WHERE a.id = a2a_tasks.client_agent_id AND a.organization_id = $%d)
		  OR EXISTS (SELECT 1 FROM agents a WHERE a.id = a2a_tasks.remote_agent_id AND a.organization_id = $%d)
		)`, argIdx, argIdx)
	args = append(args, callerOrgID)
	argIdx++

	if agentID != nil {
		baseQuery += fmt.Sprintf(" AND (client_agent_id = $%d OR remote_agent_id = $%d)", argIdx, argIdx)
		args = append(args, *agentID)
		argIdx++
	}
	if state != "" {
		baseQuery += fmt.Sprintf(" AND state = $%d", argIdx)
		args = append(args, state)
		argIdx++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	// Fetch page
	selectQuery := `
		SELECT
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		` + baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	tasks, err := r.queryTasks(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *A2ATaskRepository) ListByClientAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.A2ATask, error) {
	query := `
		SELECT
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		FROM a2a_tasks
		WHERE client_agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.queryTasks(ctx, query, agentID, limit, offset)
}

func (r *A2ATaskRepository) ListByRemoteAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.A2ATask, error) {
	query := `
		SELECT
			id, external_task_id, context_id, client_agent_id, remote_agent_id,
			skill_id, state, created_at, started_at, completed_at, duration_ms,
			policy_decision, policy_evaluated_at,
			client_trust_score_snapshot, remote_trust_score_snapshot,
			message_count, error_code, error_message
		FROM a2a_tasks
		WHERE remote_agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.queryTasks(ctx, query, agentID, limit, offset)
}

func (r *A2ATaskRepository) CountByState(ctx context.Context, agentID uuid.UUID, state domain.A2ATaskState) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM a2a_tasks
		WHERE (client_agent_id = $1 OR remote_agent_id = $1) AND state = $2
	`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentID, state).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}
	return count, nil
}

func (r *A2ATaskRepository) scanTask(row *sql.Row) (*domain.A2ATask, error) {
	task := &domain.A2ATask{}
	var contextID, skillID, errorCode, errorMessage sql.NullString
	var startedAt, completedAt, policyEvaluatedAt sql.NullTime
	var durationMs sql.NullInt32
	var clientTrust, remoteTrust sql.NullFloat64
	policyDecision := make([]byte, 0)

	err := row.Scan(
		&task.ID,
		&task.ExternalTaskID,
		&contextID,
		&task.ClientAgentID,
		&task.RemoteAgentID,
		&skillID,
		&task.State,
		&task.CreatedAt,
		&startedAt,
		&completedAt,
		&durationMs,
		&policyDecision,
		&policyEvaluatedAt,
		&clientTrust,
		&remoteTrust,
		&task.MessageCount,
		&errorCode,
		&errorMessage,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan task: %w", err)
	}

	task.ContextID = contextID.String
	task.SkillID = skillID.String
	task.ErrorCode = errorCode.String
	task.ErrorMessage = errorMessage.String
	task.PolicyDecision = policyDecision

	if startedAt.Valid {
		task.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if policyEvaluatedAt.Valid {
		task.PolicyEvaluatedAt = &policyEvaluatedAt.Time
	}
	if durationMs.Valid {
		task.DurationMs = int(durationMs.Int32)
	}
	if clientTrust.Valid {
		task.ClientTrustScoreSnapshot = &clientTrust.Float64
	}
	if remoteTrust.Valid {
		task.RemoteTrustScoreSnapshot = &remoteTrust.Float64
	}

	return task, nil
}

func (r *A2ATaskRepository) queryTasks(ctx context.Context, query string, args ...interface{}) ([]*domain.A2ATask, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*domain.A2ATask, 0)
	for rows.Next() {
		task := &domain.A2ATask{}
		var contextID, skillID, errorCode, errorMessage sql.NullString
		var startedAt, completedAt, policyEvaluatedAt sql.NullTime
		var durationMs sql.NullInt32
		var clientTrust, remoteTrust sql.NullFloat64
		policyDecision := make([]byte, 0)

		err := rows.Scan(
			&task.ID,
			&task.ExternalTaskID,
			&contextID,
			&task.ClientAgentID,
			&task.RemoteAgentID,
			&skillID,
			&task.State,
			&task.CreatedAt,
			&startedAt,
			&completedAt,
			&durationMs,
			&policyDecision,
			&policyEvaluatedAt,
			&clientTrust,
			&remoteTrust,
			&task.MessageCount,
			&errorCode,
			&errorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task.ContextID = contextID.String
		task.SkillID = skillID.String
		task.ErrorCode = errorCode.String
		task.ErrorMessage = errorMessage.String
		task.PolicyDecision = policyDecision

		if startedAt.Valid {
			task.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			task.CompletedAt = &completedAt.Time
		}
		if policyEvaluatedAt.Valid {
			task.PolicyEvaluatedAt = &policyEvaluatedAt.Time
		}
		if durationMs.Valid {
			task.DurationMs = int(durationMs.Int32)
		}
		if clientTrust.Valid {
			task.ClientTrustScoreSnapshot = &clientTrust.Float64
		}
		if remoteTrust.Valid {
			task.RemoteTrustScoreSnapshot = &remoteTrust.Float64
		}

		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ============================================================================
// A2A Peer Trust Repository
// ============================================================================

// A2APeerTrustRepository implements domain.A2APeerTrustRepository
type A2APeerTrustRepository struct {
	db *sql.DB
}

// NewA2APeerTrustRepository creates a new A2APeerTrustRepository
func NewA2APeerTrustRepository(db *sql.DB) *A2APeerTrustRepository {
	return &A2APeerTrustRepository{db: db}
}

func (r *A2APeerTrustRepository) Upsert(ctx context.Context, trust *domain.A2APeerTrust) error {
	query := `
		INSERT INTO a2a_peer_trust (
			id, agent_id, peer_agent_id,
			tasks_initiated, tasks_initiated_completed, tasks_initiated_failed,
			tasks_received, tasks_received_completed, tasks_received_failed,
			success_rate, avg_response_time_ms, p95_response_time_ms,
			peer_trust_score, trust_computed_at, trust_data_points,
			first_interaction_at, last_interaction_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (agent_id, peer_agent_id) DO UPDATE SET
			tasks_initiated = EXCLUDED.tasks_initiated,
			tasks_initiated_completed = EXCLUDED.tasks_initiated_completed,
			tasks_initiated_failed = EXCLUDED.tasks_initiated_failed,
			tasks_received = EXCLUDED.tasks_received,
			tasks_received_completed = EXCLUDED.tasks_received_completed,
			tasks_received_failed = EXCLUDED.tasks_received_failed,
			success_rate = EXCLUDED.success_rate,
			avg_response_time_ms = EXCLUDED.avg_response_time_ms,
			p95_response_time_ms = EXCLUDED.p95_response_time_ms,
			peer_trust_score = EXCLUDED.peer_trust_score,
			trust_computed_at = EXCLUDED.trust_computed_at,
			trust_data_points = EXCLUDED.trust_data_points,
			last_interaction_at = EXCLUDED.last_interaction_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	if trust.ID == uuid.Nil {
		trust.ID = uuid.New()
	}
	now := time.Now().UTC()

	err := r.db.QueryRowContext(ctx, query,
		trust.ID,
		trust.AgentID,
		trust.PeerAgentID,
		trust.TasksInitiated,
		trust.TasksInitiatedCompleted,
		trust.TasksInitiatedFailed,
		trust.TasksReceived,
		trust.TasksReceivedCompleted,
		trust.TasksReceivedFailed,
		nullFloat(trust.SuccessRate),
		nullIntPtr(trust.AvgResponseTimeMs),
		nullIntPtr(trust.P95ResponseTimeMs),
		nullFloat(trust.PeerTrustScore),
		nullTime(trust.TrustComputedAt),
		trust.TrustDataPoints,
		nullTime(trust.FirstInteractionAt),
		nullTime(trust.LastInteractionAt),
		now,
		now,
	).Scan(&trust.ID, &trust.CreatedAt, &trust.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert peer trust: %w", err)
	}

	return nil
}

func (r *A2APeerTrustRepository) GetByPeer(ctx context.Context, agentID, peerAgentID uuid.UUID) (*domain.A2APeerTrust, error) {
	query := `
		SELECT
			id, agent_id, peer_agent_id,
			tasks_initiated, tasks_initiated_completed, tasks_initiated_failed,
			tasks_received, tasks_received_completed, tasks_received_failed,
			success_rate, avg_response_time_ms, p95_response_time_ms,
			peer_trust_score, trust_computed_at, trust_data_points,
			first_interaction_at, last_interaction_at, created_at, updated_at
		FROM a2a_peer_trust
		WHERE agent_id = $1 AND peer_agent_id = $2
	`
	return r.scanPeerTrust(r.db.QueryRowContext(ctx, query, agentID, peerAgentID))
}

func (r *A2APeerTrustRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]*domain.A2APeerTrust, error) {
	query := `
		SELECT
			id, agent_id, peer_agent_id,
			tasks_initiated, tasks_initiated_completed, tasks_initiated_failed,
			tasks_received, tasks_received_completed, tasks_received_failed,
			success_rate, avg_response_time_ms, p95_response_time_ms,
			peer_trust_score, trust_computed_at, trust_data_points,
			first_interaction_at, last_interaction_at, created_at, updated_at
		FROM a2a_peer_trust
		WHERE agent_id = $1
		ORDER BY peer_trust_score DESC NULLS LAST
	`
	return r.queryPeerTrusts(ctx, query, agentID)
}

func (r *A2APeerTrustRepository) GetTopPeers(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.A2APeerTrust, error) {
	query := `
		SELECT
			id, agent_id, peer_agent_id,
			tasks_initiated, tasks_initiated_completed, tasks_initiated_failed,
			tasks_received, tasks_received_completed, tasks_received_failed,
			success_rate, avg_response_time_ms, p95_response_time_ms,
			peer_trust_score, trust_computed_at, trust_data_points,
			first_interaction_at, last_interaction_at, created_at, updated_at
		FROM a2a_peer_trust
		WHERE agent_id = $1 AND peer_trust_score IS NOT NULL
		ORDER BY peer_trust_score DESC
		LIMIT $2
	`
	return r.queryPeerTrusts(ctx, query, agentID, limit)
}

func (r *A2APeerTrustRepository) ComputeAveragePeerTrust(ctx context.Context, agentID uuid.UUID) (float64, error) {
	query := `
		SELECT COALESCE(AVG(peer_trust_score), 0)
		FROM a2a_peer_trust
		WHERE agent_id = $1 AND peer_trust_score IS NOT NULL
	`
	var avg float64
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("failed to compute average peer trust: %w", err)
	}
	return avg, nil
}

func (r *A2APeerTrustRepository) scanPeerTrust(row *sql.Row) (*domain.A2APeerTrust, error) {
	trust := &domain.A2APeerTrust{}
	var successRate, peerTrust sql.NullFloat64
	var avgResp, p95Resp sql.NullInt32
	var trustComputed, firstInteraction, lastInteraction sql.NullTime

	err := row.Scan(
		&trust.ID,
		&trust.AgentID,
		&trust.PeerAgentID,
		&trust.TasksInitiated,
		&trust.TasksInitiatedCompleted,
		&trust.TasksInitiatedFailed,
		&trust.TasksReceived,
		&trust.TasksReceivedCompleted,
		&trust.TasksReceivedFailed,
		&successRate,
		&avgResp,
		&p95Resp,
		&peerTrust,
		&trustComputed,
		&trust.TrustDataPoints,
		&firstInteraction,
		&lastInteraction,
		&trust.CreatedAt,
		&trust.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan peer trust: %w", err)
	}

	if successRate.Valid {
		trust.SuccessRate = &successRate.Float64
	}
	if peerTrust.Valid {
		trust.PeerTrustScore = &peerTrust.Float64
	}
	if avgResp.Valid {
		v := int(avgResp.Int32)
		trust.AvgResponseTimeMs = &v
	}
	if p95Resp.Valid {
		v := int(p95Resp.Int32)
		trust.P95ResponseTimeMs = &v
	}
	if trustComputed.Valid {
		trust.TrustComputedAt = &trustComputed.Time
	}
	if firstInteraction.Valid {
		trust.FirstInteractionAt = &firstInteraction.Time
	}
	if lastInteraction.Valid {
		trust.LastInteractionAt = &lastInteraction.Time
	}

	return trust, nil
}

func (r *A2APeerTrustRepository) queryPeerTrusts(ctx context.Context, query string, args ...interface{}) ([]*domain.A2APeerTrust, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query peer trusts: %w", err)
	}
	defer rows.Close()

	trusts := make([]*domain.A2APeerTrust, 0)
	for rows.Next() {
		trust := &domain.A2APeerTrust{}
		var successRate, peerTrust sql.NullFloat64
		var avgResp, p95Resp sql.NullInt32
		var trustComputed, firstInteraction, lastInteraction sql.NullTime

		err := rows.Scan(
			&trust.ID,
			&trust.AgentID,
			&trust.PeerAgentID,
			&trust.TasksInitiated,
			&trust.TasksInitiatedCompleted,
			&trust.TasksInitiatedFailed,
			&trust.TasksReceived,
			&trust.TasksReceivedCompleted,
			&trust.TasksReceivedFailed,
			&successRate,
			&avgResp,
			&p95Resp,
			&peerTrust,
			&trustComputed,
			&trust.TrustDataPoints,
			&firstInteraction,
			&lastInteraction,
			&trust.CreatedAt,
			&trust.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan peer trust: %w", err)
		}

		if successRate.Valid {
			trust.SuccessRate = &successRate.Float64
		}
		if peerTrust.Valid {
			trust.PeerTrustScore = &peerTrust.Float64
		}
		if avgResp.Valid {
			v := int(avgResp.Int32)
			trust.AvgResponseTimeMs = &v
		}
		if p95Resp.Valid {
			v := int(p95Resp.Int32)
			trust.P95ResponseTimeMs = &v
		}
		if trustComputed.Valid {
			trust.TrustComputedAt = &trustComputed.Time
		}
		if firstInteraction.Valid {
			trust.FirstInteractionAt = &firstInteraction.Time
		}
		if lastInteraction.Valid {
			trust.LastInteractionAt = &lastInteraction.Time
		}

		trusts = append(trusts, trust)
	}
	return trusts, nil
}

// ============================================================================
// A2A Consent Repository
// ============================================================================

// A2AConsentRepository implements domain.A2AConsentRepository
type A2AConsentRepository struct {
	db *sql.DB
}

// NewA2AConsentRepository creates a new A2AConsentRepository
func NewA2AConsentRepository(db *sql.DB) *A2AConsentRepository {
	return &A2AConsentRepository{db: db}
}

func (r *A2AConsentRepository) Create(ctx context.Context, consent *domain.A2AConsentRecord) error {
	query := `
		INSERT INTO a2a_consent_records (
			id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
			scope, purpose, data_types, granted_at, expires_at,
			revoked, consent_method, evidence, ip_address, user_agent,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, created_at, updated_at
	`

	if consent.ID == uuid.Nil {
		consent.ID = uuid.New()
	}
	now := time.Now().UTC()
	if consent.GrantedAt.IsZero() {
		consent.GrantedAt = now
	}

	scopeJSON, _ := json.Marshal(consent.Scope)
	dataTypesJSON, _ := json.Marshal(consent.DataTypes)

	var ipAddr sql.NullString
	if consent.IPAddress != nil {
		ipAddr = sql.NullString{String: consent.IPAddress.String(), Valid: true}
	}

	err := r.db.QueryRowContext(ctx, query,
		consent.ID,
		consent.UserID,
		nullUUID(consent.OrganizationID),
		consent.GrantorAgentID,
		consent.RecipientAgentID,
		scopeJSON,
		consent.Purpose,
		dataTypesJSON,
		consent.GrantedAt,
		nullTime(consent.ExpiresAt),
		consent.Revoked,
		consent.ConsentMethod,
		nullString(consent.Evidence),
		ipAddr,
		nullString(consent.UserAgent),
		now,
		now,
	).Scan(&consent.ID, &consent.CreatedAt, &consent.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create consent record: %w", err)
	}

	return nil
}

func (r *A2AConsentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
	query := `
		SELECT
			id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
			scope, purpose, data_types, granted_at, expires_at,
			revoked, revoked_at, revoked_reason, consent_method, evidence,
			ip_address, user_agent, created_at, updated_at
		FROM a2a_consent_records
		WHERE id = $1
	`
	return r.scanConsent(r.db.QueryRowContext(ctx, query, id))
}

func (r *A2AConsentRepository) CheckConsent(ctx context.Context, userID string, grantorID, recipientID uuid.UUID, scope string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM a2a_consent_records
			WHERE user_id = $1
				AND grantor_agent_id = $2
				AND recipient_agent_id = $3
				AND scope @> $4::jsonb
				AND revoked = FALSE
				AND (expires_at IS NULL OR expires_at > NOW())
		)
	`

	scopeJSON, _ := json.Marshal([]string{scope})
	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID, grantorID, recipientID, scopeJSON).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check consent: %w", err)
	}
	return exists, nil
}

func (r *A2AConsentRepository) ListByUser(ctx context.Context, userID string, orgID uuid.UUID, includeRevoked bool) ([]*domain.A2AConsentRecord, error) {
	// SECURITY (A3c #42): scope by organization_id to prevent cross-tenant
	// userId enumeration. A caller in org A must not be able to read a
	// userId's consents from org B. NULL organization_id rows are
	// deliberately excluded — including them would reintroduce a
	// cross-tenant leak (a NULL-org row would be visible to every
	// caller). The companion fix in A2AHandler.RecordConsent now stamps
	// caller orgID on every new write, so future rows are always
	// org-scoped. Existing NULL-org rows from the pre-fix write path
	// remain dark to this query; a backfill migration (infer org from
	// grantor_agent_id → agents.organization_id) is filed as audit-doc
	// follow-up. See tenant_scope.go:41-46.
	var query string
	if includeRevoked {
		query = `
			SELECT
				id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
				scope, purpose, data_types, granted_at, expires_at,
				revoked, revoked_at, revoked_reason, consent_method, evidence,
				ip_address, user_agent, created_at, updated_at
			FROM a2a_consent_records
			WHERE user_id = $1 AND organization_id = $2
			ORDER BY granted_at DESC
		`
	} else {
		query = `
			SELECT
				id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
				scope, purpose, data_types, granted_at, expires_at,
				revoked, revoked_at, revoked_reason, consent_method, evidence,
				ip_address, user_agent, created_at, updated_at
			FROM a2a_consent_records
			WHERE user_id = $1 AND organization_id = $2 AND revoked = FALSE
			ORDER BY granted_at DESC
		`
	}
	return r.queryConsents(ctx, query, userID, orgID)
}

func (r *A2AConsentRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.A2AConsentRecord, int, error) {
	countQuery := `SELECT COUNT(*) FROM a2a_consent_records`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count consents: %w", err)
	}

	query := `
		SELECT
			id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
			scope, purpose, data_types, granted_at, expires_at,
			revoked, revoked_at, revoked_reason, consent_method, evidence,
			ip_address, user_agent, created_at, updated_at
		FROM a2a_consent_records
		ORDER BY granted_at DESC
		LIMIT $1 OFFSET $2
	`
	consents, err := r.queryConsents(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return consents, total, nil
}

func (r *A2AConsentRepository) Revoke(ctx context.Context, id uuid.UUID, reason string) error {
	query := `
		UPDATE a2a_consent_records SET
			revoked = TRUE,
			revoked_at = $1,
			revoked_reason = $2,
			updated_at = $3
		WHERE id = $4 AND revoked = FALSE
	`

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, query, now, reason, now, id)
	if err != nil {
		return fmt.Errorf("failed to revoke consent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("consent not found or already revoked")
	}

	return nil
}

func (r *A2AConsentRepository) scanConsent(row *sql.Row) (*domain.A2AConsentRecord, error) {
	consent := &domain.A2AConsentRecord{}
	var orgID sql.NullString
	var scopeJSON, dataTypesJSON []byte
	var expiresAt, revokedAt sql.NullTime
	var revokedReason, evidence, ipAddress, userAgent sql.NullString

	err := row.Scan(
		&consent.ID,
		&consent.UserID,
		&orgID,
		&consent.GrantorAgentID,
		&consent.RecipientAgentID,
		&scopeJSON,
		&consent.Purpose,
		&dataTypesJSON,
		&consent.GrantedAt,
		&expiresAt,
		&consent.Revoked,
		&revokedAt,
		&revokedReason,
		&consent.ConsentMethod,
		&evidence,
		&ipAddress,
		&userAgent,
		&consent.CreatedAt,
		&consent.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan consent: %w", err)
	}

	if orgID.Valid {
		id, _ := uuid.Parse(orgID.String)
		consent.OrganizationID = &id
	}
	_ = json.Unmarshal(scopeJSON, &consent.Scope)
	_ = json.Unmarshal(dataTypesJSON, &consent.DataTypes)

	if expiresAt.Valid {
		consent.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		consent.RevokedAt = &revokedAt.Time
	}
	consent.RevokedReason = revokedReason.String
	consent.Evidence = evidence.String
	consent.UserAgent = userAgent.String
	if ipAddress.Valid {
		consent.IPAddress = parseIP(ipAddress.String)
	}

	return consent, nil
}

func (r *A2AConsentRepository) queryConsents(ctx context.Context, query string, args ...interface{}) ([]*domain.A2AConsentRecord, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query consents: %w", err)
	}
	defer rows.Close()

	consents := make([]*domain.A2AConsentRecord, 0)
	for rows.Next() {
		consent := &domain.A2AConsentRecord{}
		var orgID sql.NullString
		var scopeJSON, dataTypesJSON []byte
		var expiresAt, revokedAt sql.NullTime
		var revokedReason, evidence, ipAddress, userAgent sql.NullString

		err := rows.Scan(
			&consent.ID,
			&consent.UserID,
			&orgID,
			&consent.GrantorAgentID,
			&consent.RecipientAgentID,
			&scopeJSON,
			&consent.Purpose,
			&dataTypesJSON,
			&consent.GrantedAt,
			&expiresAt,
			&consent.Revoked,
			&revokedAt,
			&revokedReason,
			&consent.ConsentMethod,
			&evidence,
			&ipAddress,
			&userAgent,
			&consent.CreatedAt,
			&consent.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan consent: %w", err)
		}

		if orgID.Valid {
			id, _ := uuid.Parse(orgID.String)
			consent.OrganizationID = &id
		}
		_ = json.Unmarshal(scopeJSON, &consent.Scope)
		_ = json.Unmarshal(dataTypesJSON, &consent.DataTypes)

		if expiresAt.Valid {
			consent.ExpiresAt = &expiresAt.Time
		}
		if revokedAt.Valid {
			consent.RevokedAt = &revokedAt.Time
		}
		consent.RevokedReason = revokedReason.String
		consent.Evidence = evidence.String
		consent.UserAgent = userAgent.String
		if ipAddress.Valid {
			consent.IPAddress = parseIP(ipAddress.String)
		}

		consents = append(consents, consent)
	}
	return consents, nil
}

// ============================================================================
// A2A Request Nonce Repository
// ============================================================================

// A2ARequestNonceRepository implements domain.A2ARequestNonceRepository
type A2ARequestNonceRepository struct {
	db *sql.DB
}

// NewA2ARequestNonceRepository creates a new A2ARequestNonceRepository
func NewA2ARequestNonceRepository(db *sql.DB) *A2ARequestNonceRepository {
	return &A2ARequestNonceRepository{db: db}
}

func (r *A2ARequestNonceRepository) Create(ctx context.Context, nonce *domain.A2ARequestNonce) error {
	query := `
		INSERT INTO a2a_request_nonces (nonce, agent_id, used_at, expires_at, request_hash)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query,
		nonce.Nonce,
		nonce.AgentID,
		nonce.UsedAt,
		nonce.ExpiresAt,
		nullString(nonce.RequestHash),
	)

	if err != nil {
		return fmt.Errorf("failed to create nonce: %w", err)
	}

	return nil
}

func (r *A2ARequestNonceRepository) Exists(ctx context.Context, nonce string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM a2a_request_nonces WHERE nonce = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, nonce).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check nonce: %w", err)
	}
	return exists, nil
}

func (r *A2ARequestNonceRepository) DeleteExpired(ctx context.Context) (int, error) {
	query := `DELETE FROM a2a_request_nonces WHERE expires_at < NOW()`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired nonces: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ============================================================================
// Helper functions
// ============================================================================

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func nullInt(i int) sql.NullInt32 {
	if i == 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(i), Valid: true}
}

func nullIntPtr(i *int) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*i), Valid: true}
}

func nullFloat(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullJSON(j json.RawMessage) interface{} {
	if j == nil || len(j) == 0 {
		return nil
	}
	return j
}

func nullUUID(u *uuid.UUID) sql.NullString {
	if u == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: u.String(), Valid: true}
}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}

// ============================================================================
// A2A Trust Score Repository
// ============================================================================

// A2ATrustScoreRepository implements domain.A2ATrustScoreRepository
type A2ATrustScoreRepository struct {
	db *sql.DB
}

// NewA2ATrustScoreRepository creates a new A2ATrustScoreRepository
func NewA2ATrustScoreRepository(db *sql.DB) *A2ATrustScoreRepository {
	return &A2ATrustScoreRepository{db: db}
}

func (r *A2ATrustScoreRepository) Upsert(ctx context.Context, score *domain.A2ATrustScore) error {
	query := `
		INSERT INTO a2a_trust_scores (
			agent_id, total_tasks_as_client, total_tasks_as_remote,
			tasks_completed, tasks_failed, tasks_cancelled,
			avg_response_time_ms, p95_response_time_ms, avg_task_duration_ms,
			a2a_trust_score, peer_trust_average, unique_peers_count,
			positive_feedback_count, negative_feedback_count,
			computed_at, data_points, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (agent_id) DO UPDATE SET
			total_tasks_as_client = EXCLUDED.total_tasks_as_client,
			total_tasks_as_remote = EXCLUDED.total_tasks_as_remote,
			tasks_completed = EXCLUDED.tasks_completed,
			tasks_failed = EXCLUDED.tasks_failed,
			tasks_cancelled = EXCLUDED.tasks_cancelled,
			avg_response_time_ms = EXCLUDED.avg_response_time_ms,
			p95_response_time_ms = EXCLUDED.p95_response_time_ms,
			avg_task_duration_ms = EXCLUDED.avg_task_duration_ms,
			a2a_trust_score = EXCLUDED.a2a_trust_score,
			peer_trust_average = EXCLUDED.peer_trust_average,
			unique_peers_count = EXCLUDED.unique_peers_count,
			positive_feedback_count = EXCLUDED.positive_feedback_count,
			negative_feedback_count = EXCLUDED.negative_feedback_count,
			computed_at = EXCLUDED.computed_at,
			data_points = EXCLUDED.data_points,
			updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at
	`

	now := time.Now().UTC()

	err := r.db.QueryRowContext(ctx, query,
		score.AgentID,
		score.TotalTasksAsClient,
		score.TotalTasksAsRemote,
		score.TasksCompleted,
		score.TasksFailed,
		score.TasksCancelled,
		nullIntPtr(score.AvgResponseTimeMs),
		nullIntPtr(score.P95ResponseTimeMs),
		nullIntPtr(score.AvgTaskDurationMs),
		nullFloat(score.A2ATrustScore),
		nullFloat(score.PeerTrustAverage),
		score.UniquePeersCount,
		score.PositiveFeedbackCount,
		score.NegativeFeedbackCount,
		nullTime(score.ComputedAt),
		score.DataPoints,
		now,
		now,
	).Scan(&score.CreatedAt, &score.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert A2A trust score: %w", err)
	}

	return nil
}

func (r *A2ATrustScoreRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*domain.A2ATrustScore, error) {
	query := `
		SELECT
			agent_id, total_tasks_as_client, total_tasks_as_remote,
			tasks_completed, tasks_failed, tasks_cancelled,
			avg_response_time_ms, p95_response_time_ms, avg_task_duration_ms,
			a2a_trust_score, peer_trust_average, unique_peers_count,
			positive_feedback_count, negative_feedback_count,
			computed_at, data_points, created_at, updated_at
		FROM a2a_trust_scores
		WHERE agent_id = $1
	`

	score := &domain.A2ATrustScore{}
	var avgResp, p95Resp, avgDuration sql.NullInt32
	var a2aTrust, peerAvg sql.NullFloat64
	var computedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&score.AgentID,
		&score.TotalTasksAsClient,
		&score.TotalTasksAsRemote,
		&score.TasksCompleted,
		&score.TasksFailed,
		&score.TasksCancelled,
		&avgResp,
		&p95Resp,
		&avgDuration,
		&a2aTrust,
		&peerAvg,
		&score.UniquePeersCount,
		&score.PositiveFeedbackCount,
		&score.NegativeFeedbackCount,
		&computedAt,
		&score.DataPoints,
		&score.CreatedAt,
		&score.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get A2A trust score: %w", err)
	}

	if avgResp.Valid {
		v := int(avgResp.Int32)
		score.AvgResponseTimeMs = &v
	}
	if p95Resp.Valid {
		v := int(p95Resp.Int32)
		score.P95ResponseTimeMs = &v
	}
	if avgDuration.Valid {
		v := int(avgDuration.Int32)
		score.AvgTaskDurationMs = &v
	}
	if a2aTrust.Valid {
		score.A2ATrustScore = &a2aTrust.Float64
	}
	if peerAvg.Valid {
		score.PeerTrustAverage = &peerAvg.Float64
	}
	if computedAt.Valid {
		score.ComputedAt = &computedAt.Time
	}

	return score, nil
}

func (r *A2ATrustScoreRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.A2ATrustScore, int, error) {
	countQuery := `SELECT COUNT(*) FROM a2a_trust_scores`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count trust scores: %w", err)
	}

	query := `
		SELECT
			agent_id, total_tasks_as_client, total_tasks_as_remote,
			tasks_completed, tasks_failed, tasks_cancelled,
			avg_response_time_ms, p95_response_time_ms, avg_task_duration_ms,
			a2a_trust_score, peer_trust_average, unique_peers_count,
			positive_feedback_count, negative_feedback_count,
			computed_at, data_points, created_at, updated_at
		FROM a2a_trust_scores
		ORDER BY a2a_trust_score DESC NULLS LAST
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list trust scores: %w", err)
	}
	defer rows.Close()

	scores := make([]*domain.A2ATrustScore, 0)
	for rows.Next() {
		score := &domain.A2ATrustScore{}
		var avgResp, p95Resp, avgDuration sql.NullInt32
		var a2aTrust, peerAvg sql.NullFloat64
		var computedAt sql.NullTime

		err := rows.Scan(
			&score.AgentID,
			&score.TotalTasksAsClient,
			&score.TotalTasksAsRemote,
			&score.TasksCompleted,
			&score.TasksFailed,
			&score.TasksCancelled,
			&avgResp,
			&p95Resp,
			&avgDuration,
			&a2aTrust,
			&peerAvg,
			&score.UniquePeersCount,
			&score.PositiveFeedbackCount,
			&score.NegativeFeedbackCount,
			&computedAt,
			&score.DataPoints,
			&score.CreatedAt,
			&score.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan trust score: %w", err)
		}

		if avgResp.Valid {
			v := int(avgResp.Int32)
			score.AvgResponseTimeMs = &v
		}
		if p95Resp.Valid {
			v := int(p95Resp.Int32)
			score.P95ResponseTimeMs = &v
		}
		if avgDuration.Valid {
			v := int(avgDuration.Int32)
			score.AvgTaskDurationMs = &v
		}
		if a2aTrust.Valid {
			score.A2ATrustScore = &a2aTrust.Float64
		}
		if peerAvg.Valid {
			score.PeerTrustAverage = &peerAvg.Float64
		}
		if computedAt.Valid {
			score.ComputedAt = &computedAt.Time
		}

		scores = append(scores, score)
	}

	return scores, total, nil
}

func (r *A2ATrustScoreRepository) IncrementTaskStats(ctx context.Context, agentID uuid.UUID, asClient bool, completed bool) error {
	var query string
	if asClient {
		if completed {
			query = `
				INSERT INTO a2a_trust_scores (agent_id, total_tasks_as_client, tasks_completed, data_points, created_at, updated_at)
				VALUES ($1, 1, 1, 1, $2, $2)
				ON CONFLICT (agent_id) DO UPDATE SET
					total_tasks_as_client = a2a_trust_scores.total_tasks_as_client + 1,
					tasks_completed = a2a_trust_scores.tasks_completed + 1,
					data_points = a2a_trust_scores.data_points + 1,
					updated_at = $2
			`
		} else {
			query = `
				INSERT INTO a2a_trust_scores (agent_id, total_tasks_as_client, tasks_failed, data_points, created_at, updated_at)
				VALUES ($1, 1, 1, 1, $2, $2)
				ON CONFLICT (agent_id) DO UPDATE SET
					total_tasks_as_client = a2a_trust_scores.total_tasks_as_client + 1,
					tasks_failed = a2a_trust_scores.tasks_failed + 1,
					data_points = a2a_trust_scores.data_points + 1,
					updated_at = $2
			`
		}
	} else {
		if completed {
			query = `
				INSERT INTO a2a_trust_scores (agent_id, total_tasks_as_remote, tasks_completed, data_points, created_at, updated_at)
				VALUES ($1, 1, 1, 1, $2, $2)
				ON CONFLICT (agent_id) DO UPDATE SET
					total_tasks_as_remote = a2a_trust_scores.total_tasks_as_remote + 1,
					tasks_completed = a2a_trust_scores.tasks_completed + 1,
					data_points = a2a_trust_scores.data_points + 1,
					updated_at = $2
			`
		} else {
			query = `
				INSERT INTO a2a_trust_scores (agent_id, total_tasks_as_remote, tasks_failed, data_points, created_at, updated_at)
				VALUES ($1, 1, 1, 1, $2, $2)
				ON CONFLICT (agent_id) DO UPDATE SET
					total_tasks_as_remote = a2a_trust_scores.total_tasks_as_remote + 1,
					tasks_failed = a2a_trust_scores.tasks_failed + 1,
					data_points = a2a_trust_scores.data_points + 1,
					updated_at = $2
			`
		}
	}

	_, err := r.db.ExecContext(ctx, query, agentID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to increment task stats: %w", err)
	}
	return nil
}

func (r *A2ATrustScoreRepository) UpdatePerformanceMetrics(ctx context.Context, agentID uuid.UUID, responseTimeMs, durationMs int) error {
	query := `
		UPDATE a2a_trust_scores SET
			avg_response_time_ms = COALESCE(
				(COALESCE(avg_response_time_ms, 0) * data_points + $2) / (data_points + 1),
				$2
			),
			avg_task_duration_ms = COALESCE(
				(COALESCE(avg_task_duration_ms, 0) * data_points + $3) / (data_points + 1),
				$3
			),
			updated_at = $4
		WHERE agent_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, agentID, responseTimeMs, durationMs, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to update performance metrics: %w", err)
	}
	return nil
}

// ============================================================================
// A2A Policy Repository
// ============================================================================

// A2APolicyRepository implements domain.A2APolicyRepository
type A2APolicyRepository struct {
	db *sql.DB
}

// NewA2APolicyRepository creates a new A2APolicyRepository
func NewA2APolicyRepository(db *sql.DB) *A2APolicyRepository {
	return &A2APolicyRepository{db: db}
}

func (r *A2APolicyRepository) Create(ctx context.Context, policy *domain.A2APolicy) error {
	query := `
		INSERT INTO a2a_policies (
			id, organization_id, name, description, policy_yaml,
			applies_to_agents, applies_to_skills, is_active, priority,
			created_by, updated_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`

	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	now := time.Now().UTC()

	agentsJSON, _ := json.Marshal(policy.AppliesToAgents)
	skillsJSON, _ := json.Marshal(policy.AppliesToSkills)

	err := r.db.QueryRowContext(ctx, query,
		policy.ID,
		nullUUID(policy.OrganizationID),
		policy.Name,
		policy.Description,
		policy.PolicyYAML,
		agentsJSON,
		skillsJSON,
		policy.IsActive,
		policy.Priority,
		nullUUID(policy.CreatedBy),
		nullUUID(policy.UpdatedBy),
		now,
		now,
	).Scan(&policy.ID, &policy.CreatedAt, &policy.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create A2A policy: %w", err)
	}

	return nil
}

func (r *A2APolicyRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.A2APolicy, error) {
	query := `
		SELECT
			id, organization_id, name, description, policy_yaml,
			applies_to_agents, applies_to_skills, is_active, priority,
			created_by, updated_by, created_at, updated_at
		FROM a2a_policies
		WHERE id = $1
	`

	policy := &domain.A2APolicy{}
	var orgID, createdBy, updatedBy sql.NullString
	var agentsJSON, skillsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&policy.ID,
		&orgID,
		&policy.Name,
		&policy.Description,
		&policy.PolicyYAML,
		&agentsJSON,
		&skillsJSON,
		&policy.IsActive,
		&policy.Priority,
		&createdBy,
		&updatedBy,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get A2A policy: %w", err)
	}

	if orgID.Valid {
		id, _ := uuid.Parse(orgID.String)
		policy.OrganizationID = &id
	}
	if createdBy.Valid {
		id, _ := uuid.Parse(createdBy.String)
		policy.CreatedBy = &id
	}
	if updatedBy.Valid {
		id, _ := uuid.Parse(updatedBy.String)
		policy.UpdatedBy = &id
	}
	_ = json.Unmarshal(agentsJSON, &policy.AppliesToAgents)
	_ = json.Unmarshal(skillsJSON, &policy.AppliesToSkills)

	return policy, nil
}

func (r *A2APolicyRepository) GetByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.A2APolicy, error) {
	query := `
		SELECT
			id, organization_id, name, description, policy_yaml,
			applies_to_agents, applies_to_skills, is_active, priority,
			created_by, updated_by, created_at, updated_at
		FROM a2a_policies
		WHERE organization_id = $1
		ORDER BY priority, name
	`
	return r.queryPolicies(ctx, query, orgID)
}

func (r *A2APolicyRepository) GetActivePolicies(ctx context.Context, orgID uuid.UUID) ([]*domain.A2APolicy, error) {
	query := `
		SELECT
			id, organization_id, name, description, policy_yaml,
			applies_to_agents, applies_to_skills, is_active, priority,
			created_by, updated_by, created_at, updated_at
		FROM a2a_policies
		WHERE organization_id = $1 AND is_active = TRUE
		ORDER BY priority, name
	`
	return r.queryPolicies(ctx, query, orgID)
}

func (r *A2APolicyRepository) Update(ctx context.Context, policy *domain.A2APolicy) error {
	query := `
		UPDATE a2a_policies SET
			name = $1,
			description = $2,
			policy_yaml = $3,
			applies_to_agents = $4,
			applies_to_skills = $5,
			is_active = $6,
			priority = $7,
			updated_by = $8,
			updated_at = $9
		WHERE id = $10
		RETURNING updated_at
	`

	agentsJSON, _ := json.Marshal(policy.AppliesToAgents)
	skillsJSON, _ := json.Marshal(policy.AppliesToSkills)

	err := r.db.QueryRowContext(ctx, query,
		policy.Name,
		policy.Description,
		policy.PolicyYAML,
		agentsJSON,
		skillsJSON,
		policy.IsActive,
		policy.Priority,
		nullUUID(policy.UpdatedBy),
		time.Now().UTC(),
		policy.ID,
	).Scan(&policy.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update A2A policy: %w", err)
	}

	return nil
}

func (r *A2APolicyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM a2a_policies WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete A2A policy: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("A2A policy not found")
	}
	return nil
}

func (r *A2APolicyRepository) queryPolicies(ctx context.Context, query string, args ...interface{}) ([]*domain.A2APolicy, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies: %w", err)
	}
	defer rows.Close()

	policies := make([]*domain.A2APolicy, 0)
	for rows.Next() {
		policy := &domain.A2APolicy{}
		var orgID, createdBy, updatedBy sql.NullString
		var agentsJSON, skillsJSON []byte

		err := rows.Scan(
			&policy.ID,
			&orgID,
			&policy.Name,
			&policy.Description,
			&policy.PolicyYAML,
			&agentsJSON,
			&skillsJSON,
			&policy.IsActive,
			&policy.Priority,
			&createdBy,
			&updatedBy,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		if orgID.Valid {
			id, _ := uuid.Parse(orgID.String)
			policy.OrganizationID = &id
		}
		if createdBy.Valid {
			id, _ := uuid.Parse(createdBy.String)
			policy.CreatedBy = &id
		}
		if updatedBy.Valid {
			id, _ := uuid.Parse(updatedBy.String)
			policy.UpdatedBy = &id
		}
		_ = json.Unmarshal(agentsJSON, &policy.AppliesToAgents)
		_ = json.Unmarshal(skillsJSON, &policy.AppliesToSkills)

		policies = append(policies, policy)
	}
	return policies, nil
}

// ============================================================================
// A2A Agent Attestation Repository
// ============================================================================

// A2AAgentAttestationRepository handles agent attestation storage
type A2AAgentAttestationRepository struct {
	db *sql.DB
}

// NewA2AAgentAttestationRepository creates a new attestation repository
func NewA2AAgentAttestationRepository(db *sql.DB) *A2AAgentAttestationRepository {
	return &A2AAgentAttestationRepository{db: db}
}

func (r *A2AAgentAttestationRepository) Create(ctx context.Context, att *domain.A2AAgentAttestation) error {
	query := `
		INSERT INTO a2a_agent_attestations (
			id, attesting_agent_id, attested_agent_id, skill_id, task_id,
			task_completed, response_time_ms, quality_score,
			attestation_signature, challenge, attester_trust_score,
			verified_at, expires_at, is_valid, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	if att.ID == uuid.Nil {
		att.ID = uuid.New()
	}
	now := time.Now().UTC()
	att.CreatedAt = now
	att.VerifiedAt = now
	att.IsValid = true

	var qualityScorePtr, attesterTrustPtr *float64
	if att.QualityScore != 0 {
		qualityScorePtr = &att.QualityScore
	}
	if att.AttesterTrustScore != 0 {
		attesterTrustPtr = &att.AttesterTrustScore
	}

	_, err := r.db.ExecContext(ctx, query,
		att.ID,
		att.AttestingAgentID,
		att.AttestedAgentID,
		nullString(att.SkillID),
		nullUUID(att.TaskID),
		att.TaskCompleted,
		nullInt(att.ResponseTimeMs),
		nullFloat(qualityScorePtr),
		att.AttestationSignature,
		nullString(att.Challenge),
		nullFloat(attesterTrustPtr),
		att.VerifiedAt,
		att.ExpiresAt,
		att.IsValid,
		att.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create attestation: %w", err)
	}
	return nil
}

func (r *A2AAgentAttestationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.A2AAgentAttestation, error) {
	query := `
		SELECT id, attesting_agent_id, attested_agent_id, skill_id, task_id,
			task_completed, response_time_ms, quality_score,
			attestation_signature, challenge, attester_trust_score,
			verified_at, expires_at, is_valid, revoked_at, revoked_reason, created_at
		FROM a2a_agent_attestations
		WHERE id = $1
	`

	att := &domain.A2AAgentAttestation{}
	var skillID, challenge, revokedReason sql.NullString
	var taskID sql.NullString
	var responseTimeMs sql.NullInt64
	var qualityScore, attesterTrustScore sql.NullFloat64
	var revokedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&att.ID,
		&att.AttestingAgentID,
		&att.AttestedAgentID,
		&skillID,
		&taskID,
		&att.TaskCompleted,
		&responseTimeMs,
		&qualityScore,
		&att.AttestationSignature,
		&challenge,
		&attesterTrustScore,
		&att.VerifiedAt,
		&att.ExpiresAt,
		&att.IsValid,
		&revokedAt,
		&revokedReason,
		&att.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attestation: %w", err)
	}

	if skillID.Valid {
		att.SkillID = skillID.String
	}
	if taskID.Valid {
		id, _ := uuid.Parse(taskID.String)
		att.TaskID = &id
	}
	if responseTimeMs.Valid {
		att.ResponseTimeMs = int(responseTimeMs.Int64)
	}
	if qualityScore.Valid {
		att.QualityScore = qualityScore.Float64
	}
	if challenge.Valid {
		att.Challenge = challenge.String
	}
	if attesterTrustScore.Valid {
		att.AttesterTrustScore = attesterTrustScore.Float64
	}
	if revokedAt.Valid {
		att.RevokedAt = &revokedAt.Time
	}
	if revokedReason.Valid {
		att.RevokedReason = revokedReason.String
	}

	return att, nil
}

func (r *A2AAgentAttestationRepository) GetByAttestedAgent(ctx context.Context, agentID uuid.UUID, skillID string) ([]*domain.A2AAgentAttestation, error) {
	query := `
		SELECT id, attesting_agent_id, attested_agent_id, skill_id, task_id,
			task_completed, response_time_ms, quality_score,
			attestation_signature, challenge, attester_trust_score,
			verified_at, expires_at, is_valid, revoked_at, revoked_reason, created_at
		FROM a2a_agent_attestations
		WHERE attested_agent_id = $1 AND is_valid = TRUE AND expires_at > NOW()
	`
	args := []interface{}{agentID}
	if skillID != "" {
		query += " AND skill_id = $2"
		args = append(args, skillID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get attestations: %w", err)
	}
	defer rows.Close()

	attestations := make([]*domain.A2AAgentAttestation, 0)
	for rows.Next() {
		att := &domain.A2AAgentAttestation{}
		var sID, ch, rr sql.NullString
		var tID sql.NullString
		var rtMs sql.NullInt64
		var qs, ats sql.NullFloat64
		var ra sql.NullTime

		err := rows.Scan(
			&att.ID, &att.AttestingAgentID, &att.AttestedAgentID, &sID, &tID,
			&att.TaskCompleted, &rtMs, &qs, &att.AttestationSignature, &ch, &ats,
			&att.VerifiedAt, &att.ExpiresAt, &att.IsValid, &ra, &rr, &att.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attestation: %w", err)
		}
		if sID.Valid {
			att.SkillID = sID.String
		}
		if tID.Valid {
			id, _ := uuid.Parse(tID.String)
			att.TaskID = &id
		}
		if rtMs.Valid {
			att.ResponseTimeMs = int(rtMs.Int64)
		}
		if qs.Valid {
			att.QualityScore = qs.Float64
		}
		if ch.Valid {
			att.Challenge = ch.String
		}
		if ats.Valid {
			att.AttesterTrustScore = ats.Float64
		}
		if ra.Valid {
			att.RevokedAt = &ra.Time
		}
		if rr.Valid {
			att.RevokedReason = rr.String
		}
		attestations = append(attestations, att)
	}
	return attestations, nil
}

func (r *A2AAgentAttestationRepository) CountUniqueAttesters(ctx context.Context, agentID uuid.UUID, skillID string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT attesting_agent_id)
		FROM a2a_agent_attestations
		WHERE attested_agent_id = $1 AND is_valid = TRUE AND expires_at > NOW()
	`
	args := []interface{}{agentID}
	if skillID != "" {
		query += " AND skill_id = $2"
		args = append(args, skillID)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count attesters: %w", err)
	}
	return count, nil
}

func (r *A2AAgentAttestationRepository) CountUniqueOwners(ctx context.Context, agentID uuid.UUID, skillID string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT a.organization_id)
		FROM a2a_agent_attestations att
		JOIN agents a ON a.id = att.attesting_agent_id
		WHERE att.attested_agent_id = $1 AND att.is_valid = TRUE AND att.expires_at > NOW()
	`
	args := []interface{}{agentID}
	if skillID != "" {
		query += " AND att.skill_id = $2"
		args = append(args, skillID)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count owners: %w", err)
	}
	return count, nil
}

func (r *A2AAgentAttestationRepository) Revoke(ctx context.Context, id uuid.UUID, reason string) error {
	query := `UPDATE a2a_agent_attestations SET is_valid = FALSE, revoked_at = NOW(), revoked_reason = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, reason)
	return err
}

func (r *A2AAgentAttestationRepository) DeleteExpired(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM a2a_agent_attestations WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// ============================================================================
// A2A Revoked Agent Repository
// ============================================================================

// A2ARevokedAgentRepository handles revoked agent storage
type A2ARevokedAgentRepository struct {
	db *sql.DB
}

// NewA2ARevokedAgentRepository creates a new revoked agent repository
func NewA2ARevokedAgentRepository(db *sql.DB) *A2ARevokedAgentRepository {
	return &A2ARevokedAgentRepository{db: db}
}

func (r *A2ARevokedAgentRepository) Revoke(ctx context.Context, agentID uuid.UUID, reason string, revokedBy *uuid.UUID) error {
	query := `INSERT INTO a2a_revoked_agents (agent_id, reason, revoked_by) VALUES ($1, $2, $3) ON CONFLICT (agent_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, agentID, nullString(reason), nullUUID(revokedBy))
	return err
}

func (r *A2ARevokedAgentRepository) IsRevoked(ctx context.Context, agentID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM a2a_revoked_agents WHERE agent_id = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(&exists)
	return exists, err
}

func (r *A2ARevokedAgentRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID) (*domain.A2ARevokedAgent, error) {
	query := `SELECT agent_id, revoked_at, reason, revoked_by FROM a2a_revoked_agents WHERE agent_id = $1`
	ra := &domain.A2ARevokedAgent{}
	var reason sql.NullString
	var revokedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(&ra.AgentID, &ra.RevokedAt, &reason, &revokedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if reason.Valid {
		ra.Reason = reason.String
	}
	if revokedBy.Valid {
		id, _ := uuid.Parse(revokedBy.String)
		ra.RevokedBy = &id
	}
	return ra, nil
}

func (r *A2ARevokedAgentRepository) List(ctx context.Context, limit, offset int) ([]*domain.A2ARevokedAgent, error) {
	query := `SELECT agent_id, revoked_at, reason, revoked_by FROM a2a_revoked_agents ORDER BY revoked_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*domain.A2ARevokedAgent, 0)
	for rows.Next() {
		ra := &domain.A2ARevokedAgent{}
		var reason, revokedBy sql.NullString
		if err := rows.Scan(&ra.AgentID, &ra.RevokedAt, &reason, &revokedBy); err != nil {
			return nil, err
		}
		if reason.Valid {
			ra.Reason = reason.String
		}
		if revokedBy.Valid {
			id, _ := uuid.Parse(revokedBy.String)
			ra.RevokedBy = &id
		}
		list = append(list, ra)
	}
	return list, nil
}

func (r *A2ARevokedAgentRepository) Reinstate(ctx context.Context, agentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM a2a_revoked_agents WHERE agent_id = $1`, agentID)
	return err
}

// ============================================================================
// A2A Security Settings Repository
// ============================================================================

type A2ASecuritySettingsRepository struct {
	db *sql.DB
}

func NewA2ASecuritySettingsRepository(db *sql.DB) *A2ASecuritySettingsRepository {
	return &A2ASecuritySettingsRepository{db: db}
}

func (r *A2ASecuritySettingsRepository) Create(ctx context.Context, s *domain.A2ASecuritySettings) error {
	allowedJSON, _ := json.Marshal(s.AllowedAgentIDs)
	blockedJSON, _ := json.Marshal(s.BlockedAgentIDs)

	query := `INSERT INTO a2a_security_settings (
		id, organization_id, enforcement_mode, require_verified_skills,
		min_trust_score, min_attestation_count, require_request_signing,
		require_nonce_validation, nonce_validity_seconds, allowed_agent_ids,
		blocked_agent_ids, max_requests_per_minute, max_requests_per_hour,
		created_by, updated_by, created_at, updated_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

	_, err := r.db.ExecContext(ctx, query,
		s.ID, s.OrganizationID, s.EnforcementMode, s.RequireVerifiedSkills,
		s.MinTrustScore, s.MinAttestationCount, s.RequireRequestSigning,
		s.RequireNonceValidation, s.NonceValiditySeconds, allowedJSON,
		blockedJSON, s.MaxRequestsPerMinute, s.MaxRequestsPerHour,
		s.CreatedBy, s.UpdatedBy, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *A2ASecuritySettingsRepository) GetByOrganization(ctx context.Context, orgID uuid.UUID) (*domain.A2ASecuritySettings, error) {
	query := `SELECT id, organization_id, enforcement_mode, require_verified_skills,
		min_trust_score, min_attestation_count, require_request_signing,
		require_nonce_validation, nonce_validity_seconds, allowed_agent_ids,
		blocked_agent_ids, max_requests_per_minute, max_requests_per_hour,
		created_by, updated_by, created_at, updated_at
		FROM a2a_security_settings WHERE organization_id = $1`

	s := &domain.A2ASecuritySettings{}
	var allowedJSON, blockedJSON []byte
	var createdBy, updatedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, orgID).Scan(
		&s.ID, &s.OrganizationID, &s.EnforcementMode, &s.RequireVerifiedSkills,
		&s.MinTrustScore, &s.MinAttestationCount, &s.RequireRequestSigning,
		&s.RequireNonceValidation, &s.NonceValiditySeconds, &allowedJSON,
		&blockedJSON, &s.MaxRequestsPerMinute, &s.MaxRequestsPerHour,
		&createdBy, &updatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(allowedJSON, &s.AllowedAgentIDs)
	json.Unmarshal(blockedJSON, &s.BlockedAgentIDs)
	if createdBy.Valid {
		id, _ := uuid.Parse(createdBy.String)
		s.CreatedBy = &id
	}
	if updatedBy.Valid {
		id, _ := uuid.Parse(updatedBy.String)
		s.UpdatedBy = &id
	}
	return s, nil
}

func (r *A2ASecuritySettingsRepository) Update(ctx context.Context, s *domain.A2ASecuritySettings) error {
	allowedJSON, _ := json.Marshal(s.AllowedAgentIDs)
	blockedJSON, _ := json.Marshal(s.BlockedAgentIDs)

	query := `UPDATE a2a_security_settings SET
		enforcement_mode = $1, require_verified_skills = $2, min_trust_score = $3,
		min_attestation_count = $4, require_request_signing = $5,
		require_nonce_validation = $6, nonce_validity_seconds = $7,
		allowed_agent_ids = $8, blocked_agent_ids = $9,
		max_requests_per_minute = $10, max_requests_per_hour = $11,
		updated_by = $12, updated_at = NOW()
		WHERE organization_id = $13`

	_, err := r.db.ExecContext(ctx, query,
		s.EnforcementMode, s.RequireVerifiedSkills, s.MinTrustScore,
		s.MinAttestationCount, s.RequireRequestSigning,
		s.RequireNonceValidation, s.NonceValiditySeconds,
		allowedJSON, blockedJSON,
		s.MaxRequestsPerMinute, s.MaxRequestsPerHour,
		s.UpdatedBy, s.OrganizationID)
	return err
}

func (r *A2ASecuritySettingsRepository) Delete(ctx context.Context, orgID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM a2a_security_settings WHERE organization_id = $1`, orgID)
	return err
}

// ============================================================================
// A2A Security Violations Repository
// ============================================================================

type A2ASecurityViolationRepository struct {
	db *sql.DB
}

func NewA2ASecurityViolationRepository(db *sql.DB) *A2ASecurityViolationRepository {
	return &A2ASecurityViolationRepository{db: db}
}

func (r *A2ASecurityViolationRepository) Create(ctx context.Context, v *domain.A2ASecurityViolation) error {
	detailsJSON, _ := json.Marshal(v.ViolationDetails)

	query := `INSERT INTO a2a_security_violations (
		id, organization_id, requesting_agent_id, target_agent_id, skill_id,
		task_id, violation_type, violation_details, policy_id, action_taken,
		request_signature, request_nonce, request_timestamp, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := r.db.ExecContext(ctx, query,
		v.ID, v.OrganizationID, v.RequestingAgentID, v.TargetAgentID, v.SkillID,
		v.TaskID, v.ViolationType, detailsJSON, v.PolicyID, v.ActionTaken,
		v.RequestSignature, v.RequestNonce, v.RequestTimestamp, v.CreatedAt)
	return err
}

func (r *A2ASecurityViolationRepository) GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ASecurityViolation, error) {
	query := `SELECT id, organization_id, requesting_agent_id, target_agent_id, skill_id,
		task_id, violation_type, violation_details, policy_id, action_taken,
		request_signature, request_nonce, request_timestamp, created_at
		FROM a2a_security_violations WHERE organization_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	return r.queryViolations(ctx, query, orgID, limit, offset)
}

func (r *A2ASecurityViolationRepository) GetByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.A2ASecurityViolation, error) {
	query := `SELECT id, organization_id, requesting_agent_id, target_agent_id, skill_id,
		task_id, violation_type, violation_details, policy_id, action_taken,
		request_signature, request_nonce, request_timestamp, created_at
		FROM a2a_security_violations WHERE requesting_agent_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	return r.queryViolations(ctx, query, agentID, limit, offset)
}

func (r *A2ASecurityViolationRepository) GetByType(ctx context.Context, orgID uuid.UUID, violationType domain.A2AViolationType, limit, offset int) ([]*domain.A2ASecurityViolation, error) {
	query := `SELECT id, organization_id, requesting_agent_id, target_agent_id, skill_id,
		task_id, violation_type, violation_details, policy_id, action_taken,
		request_signature, request_nonce, request_timestamp, created_at
		FROM a2a_security_violations WHERE organization_id = $1 AND violation_type = $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`

	rows, err := r.db.QueryContext(ctx, query, orgID, violationType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanViolations(rows)
}

func (r *A2ASecurityViolationRepository) queryViolations(ctx context.Context, query string, args ...interface{}) ([]*domain.A2ASecurityViolation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanViolations(rows)
}

func (r *A2ASecurityViolationRepository) scanViolations(rows *sql.Rows) ([]*domain.A2ASecurityViolation, error) {
	list := make([]*domain.A2ASecurityViolation, 0)
	for rows.Next() {
		v := &domain.A2ASecurityViolation{}
		detailsJSON := make([]byte, 0)
		var reqAgentID, targetAgentID, taskID, policyID sql.NullString
		var reqSig, reqNonce sql.NullString
		var reqTimestamp sql.NullTime

		err := rows.Scan(&v.ID, &v.OrganizationID, &reqAgentID, &targetAgentID, &v.SkillID,
			&taskID, &v.ViolationType, &detailsJSON, &policyID, &v.ActionTaken,
			&reqSig, &reqNonce, &reqTimestamp, &v.CreatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(detailsJSON, &v.ViolationDetails)
		if reqAgentID.Valid {
			id, _ := uuid.Parse(reqAgentID.String)
			v.RequestingAgentID = &id
		}
		if targetAgentID.Valid {
			id, _ := uuid.Parse(targetAgentID.String)
			v.TargetAgentID = &id
		}
		if taskID.Valid {
			id, _ := uuid.Parse(taskID.String)
			v.TaskID = &id
		}
		if policyID.Valid {
			id, _ := uuid.Parse(policyID.String)
			v.PolicyID = &id
		}
		if reqSig.Valid {
			v.RequestSignature = reqSig.String
		}
		if reqNonce.Valid {
			v.RequestNonce = reqNonce.String
		}
		if reqTimestamp.Valid {
			v.RequestTimestamp = &reqTimestamp.Time
		}
		list = append(list, v)
	}
	return list, nil
}

func (r *A2ASecurityViolationRepository) CountByOrganization(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM a2a_security_violations WHERE organization_id = $1 AND created_at >= $2`,
		orgID, since).Scan(&count)
	return count, err
}

func (r *A2ASecurityViolationRepository) CountByType(ctx context.Context, orgID uuid.UUID, violationType domain.A2AViolationType, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM a2a_security_violations WHERE organization_id = $1 AND violation_type = $2 AND created_at >= $3`,
		orgID, violationType, since).Scan(&count)
	return count, err
}

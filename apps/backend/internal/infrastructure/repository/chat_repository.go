package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ChatConversationRepository implements domain.ChatConversationRepository
type ChatConversationRepository struct {
	db *sqlx.DB
}

// NewChatConversationRepository creates a new chat conversation repository
func NewChatConversationRepository(db *sqlx.DB) *ChatConversationRepository {
	return &ChatConversationRepository{db: db}
}

func (r *ChatConversationRepository) Create(ctx context.Context, conversation *domain.ChatConversation) error {
	// Convert metadata to JSON bytes
	metadataJSON, err := json.Marshal(conversation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO chat_conversations (
			id, organization_id, user_id, agent_id, title, status, 
			metadata, created_at, updated_at, last_message_at
		) VALUES (
			:id, :organization_id, :user_id, :agent_id, :title, :status,
			:metadata_json, :created_at, :updated_at, :last_message_at
		)`
	
	args := map[string]interface{}{
		"id":              conversation.ID,
		"organization_id": conversation.OrganizationID,
		"user_id":         conversation.UserID,
		"agent_id":        conversation.AgentID,
		"title":           conversation.Title,
		"status":          conversation.Status,
		"metadata_json":   metadataJSON,
		"created_at":      conversation.CreatedAt,
		"updated_at":      conversation.UpdatedAt,
		"last_message_at": conversation.LastMessageAt,
	}
	
	_, err = r.db.NamedExecContext(ctx, query, args)
	return err
}

func (r *ChatConversationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ChatConversation, error) {
	query := `
		SELECT id, organization_id, user_id, agent_id, title, status,
		       metadata, created_at, updated_at, last_message_at
		FROM chat_conversations 
		WHERE id = $1`
	
	var conversation domain.ChatConversation
	var metadataJSON []byte
	
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&conversation.ID,
		&conversation.OrganizationID,
		&conversation.UserID,
		&conversation.AgentID,
		&conversation.Title,
		&conversation.Status,
		&metadataJSON,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
		&conversation.LastMessageAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, err
	}
	
	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &conversation.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		conversation.Metadata = make(map[string]interface{})
	}
	
	return &conversation, nil
}

func (r *ChatConversationRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ChatConversation, error) {
	query := `
		SELECT 
			cc.id, cc.organization_id, cc.user_id, cc.agent_id, cc.title, cc.status,
			cc.metadata, cc.created_at, cc.updated_at, cc.last_message_at,
			a.name as agent_name, a.display_name as agent_display_name
		FROM chat_conversations cc
		LEFT JOIN agents a ON cc.agent_id = a.id
		WHERE cc.user_id = $1 AND cc.status = 'active'
		ORDER BY cc.last_message_at DESC NULLS LAST, cc.created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var conversations []*domain.ChatConversation
	for rows.Next() {
		var conversation domain.ChatConversation
		var metadataJSON []byte
		var agentName, agentDisplayName sql.NullString
		
		err := rows.Scan(
			&conversation.ID,
			&conversation.OrganizationID,
			&conversation.UserID,
			&conversation.AgentID,
			&conversation.Title,
			&conversation.Status,
			&metadataJSON,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.LastMessageAt,
			&agentName,
			&agentDisplayName,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &conversation.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			conversation.Metadata = make(map[string]interface{})
		}
		
		// Create Agent object if data exists
		if agentName.Valid || agentDisplayName.Valid {
			conversation.Agent = &domain.Agent{
				ID: conversation.AgentID,
			}
			if agentName.Valid {
				conversation.Agent.Name = agentName.String
			}
			if agentDisplayName.Valid {
				conversation.Agent.DisplayName = agentDisplayName.String
			}
		}
		
		conversations = append(conversations, &conversation)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return conversations, nil
}

func (r *ChatConversationRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.ChatConversation, error) {
	query := `
		SELECT id, organization_id, user_id, agent_id, title, status,
		       metadata, created_at, updated_at, last_message_at
		FROM chat_conversations 
		WHERE agent_id = $1 AND status = 'active'
		ORDER BY last_message_at DESC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var conversations []*domain.ChatConversation
	for rows.Next() {
		var conversation domain.ChatConversation
		var metadataJSON []byte
		
		err := rows.Scan(
			&conversation.ID,
			&conversation.OrganizationID,
			&conversation.UserID,
			&conversation.AgentID,
			&conversation.Title,
			&conversation.Status,
			&metadataJSON,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.LastMessageAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &conversation.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			conversation.Metadata = make(map[string]interface{})
		}
		
		conversations = append(conversations, &conversation)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return conversations, nil
}

func (r *ChatConversationRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.ChatConversation, error) {
	query := `
		SELECT id, organization_id, user_id, agent_id, title, status,
		       metadata, created_at, updated_at, last_message_at
		FROM chat_conversations 
		WHERE organization_id = $1 AND status = 'active'
		ORDER BY last_message_at DESC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var conversations []*domain.ChatConversation
	for rows.Next() {
		var conversation domain.ChatConversation
		var metadataJSON []byte
		
		err := rows.Scan(
			&conversation.ID,
			&conversation.OrganizationID,
			&conversation.UserID,
			&conversation.AgentID,
			&conversation.Title,
			&conversation.Status,
			&metadataJSON,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.LastMessageAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &conversation.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			conversation.Metadata = make(map[string]interface{})
		}
		
		conversations = append(conversations, &conversation)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return conversations, nil
}

func (r *ChatConversationRepository) Update(ctx context.Context, conversation *domain.ChatConversation) error {
	// Convert metadata to JSON bytes
	metadataJSON, err := json.Marshal(conversation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE chat_conversations 
		SET title = :title, status = :status, metadata = :metadata_json, 
		    updated_at = :updated_at, last_message_at = :last_message_at
		WHERE id = :id`
	
	args := map[string]interface{}{
		"id":              conversation.ID,
		"title":           conversation.Title,
		"status":          conversation.Status,
		"metadata_json":   metadataJSON,
		"updated_at":      conversation.UpdatedAt,
		"last_message_at": conversation.LastMessageAt,
	}
	
	_, err = r.db.NamedExecContext(ctx, query, args)
	return err
}

func (r *ChatConversationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Hard delete - this will cascade delete all related messages due to foreign key constraints
	query := `DELETE FROM chat_conversations WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ChatConversationRepository) UpdateLastMessageAt(ctx context.Context, conversationID uuid.UUID, timestamp time.Time) error {
	query := `UPDATE chat_conversations SET last_message_at = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, timestamp, conversationID)
	return err
}

func (r *ChatConversationRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM chat_conversations WHERE user_id = $1 AND status = 'active'`
	var count int
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}

func (r *ChatConversationRepository) CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM chat_conversations WHERE agent_id = $1 AND status = 'active'`
	var count int
	err := r.db.GetContext(ctx, &count, query, agentID)
	return count, err
}

// ChatMessageRepository implements domain.ChatMessageRepository
type ChatMessageRepository struct {
	db *sqlx.DB
}

// NewChatMessageRepository creates a new chat message repository
func NewChatMessageRepository(db *sqlx.DB) *ChatMessageRepository {
	return &ChatMessageRepository{db: db}
}

func (r *ChatMessageRepository) Create(ctx context.Context, message *domain.ChatMessage) error {
	// Convert metadata to JSON bytes
	metadataJSON, err := json.Marshal(message.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO chat_messages (
			id, conversation_id, user_id, agent_id, message_type, content, role,
			metadata, parent_message_id, is_edited, edited_at, created_at
		) VALUES (
			:id, :conversation_id, :user_id, :agent_id, :message_type, :content, :role,
			:metadata_json, :parent_message_id, :is_edited, :edited_at, :created_at
		)`
	
	args := map[string]interface{}{
		"id":                message.ID,
		"conversation_id":   message.ConversationID,
		"user_id":           message.UserID,
		"agent_id":          message.AgentID,
		"message_type":      message.MessageType,
		"content":           message.Content,
		"role":              message.Role,
		"metadata_json":     metadataJSON,
		"parent_message_id": message.ParentMessageID,
		"is_edited":         message.IsEdited,
		"edited_at":         message.EditedAt,
		"created_at":        message.CreatedAt,
	}
	
	_, err = r.db.NamedExecContext(ctx, query, args)
	return err
}

func (r *ChatMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ChatMessage, error) {
	query := `
		SELECT id, conversation_id, user_id, agent_id, message_type, content, role,
		       metadata, parent_message_id, is_edited, edited_at, created_at
		FROM chat_messages 
		WHERE id = $1`
	
	var message domain.ChatMessage
	err := r.db.GetContext(ctx, &message, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found")
		}
		return nil, err
	}
	
	return &message, nil
}

func (r *ChatMessageRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.ChatMessage, error) {
	query := `
		SELECT id, conversation_id, user_id, agent_id, message_type, content, role,
		       metadata, parent_message_id, is_edited, edited_at, created_at
		FROM chat_messages 
		WHERE conversation_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var messages []*domain.ChatMessage
	for rows.Next() {
		var message domain.ChatMessage
		var metadataJSON []byte
		
		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.UserID,
			&message.AgentID,
			&message.MessageType,
			&message.Content,
			&message.Role,
			&metadataJSON,
			&message.ParentMessageID,
			&message.IsEdited,
			&message.EditedAt,
			&message.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &message.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			message.Metadata = make(map[string]interface{})
		}
		
		messages = append(messages, &message)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return messages, nil
}

func (r *ChatMessageRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ChatMessage, error) {
	query := `
		SELECT id, conversation_id, user_id, agent_id, message_type, content, role,
		       metadata, parent_message_id, is_edited, edited_at, created_at
		FROM chat_messages 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	var messages []*domain.ChatMessage
	err := r.db.SelectContext(ctx, &messages, query, userID, limit, offset)
	return messages, err
}

func (r *ChatMessageRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.ChatMessage, error) {
	query := `
		SELECT id, conversation_id, user_id, agent_id, message_type, content, role,
		       metadata, parent_message_id, is_edited, edited_at, created_at
		FROM chat_messages 
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	var messages []*domain.ChatMessage
	err := r.db.SelectContext(ctx, &messages, query, agentID, limit, offset)
	return messages, err
}

func (r *ChatMessageRepository) Update(ctx context.Context, message *domain.ChatMessage) error {
	query := `
		UPDATE chat_messages 
		SET content = :content, metadata = :metadata, is_edited = :is_edited, edited_at = :edited_at
		WHERE id = :id`
	
	_, err := r.db.NamedExecContext(ctx, query, message)
	return err
}

func (r *ChatMessageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM chat_messages WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *ChatMessageRepository) CountByConversationID(ctx context.Context, conversationID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM chat_messages WHERE conversation_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, conversationID)
	return count, err
}

func (r *ChatMessageRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM chat_messages WHERE user_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}

func (r *ChatMessageRepository) CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM chat_messages WHERE agent_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, agentID)
	return count, err
}

func (r *ChatMessageRepository) CountUserMessagesToday(ctx context.Context, userID uuid.UUID, date time.Time) (int, error) {
	// Count only USER messages (role = 'user') sent today
	// We need to count from the start of the day to now
	startOfDay := date.Truncate(24 * time.Hour)
	endOfDay := startOfDay.Add(24 * time.Hour)
	
	query := `
		SELECT COUNT(*) 
		FROM chat_messages 
		WHERE user_id = $1 
		  AND role = 'user'
		  AND created_at >= $2 
		  AND created_at < $3`
	
	var count int
	err := r.db.GetContext(ctx, &count, query, userID, startOfDay, endOfDay)
	return count, err
}

// UserDailyLimitRepository implements domain.UserDailyLimitRepository
type UserDailyLimitRepository struct {
	db *sqlx.DB
}

// NewUserDailyLimitRepository creates a new user daily limit repository
func NewUserDailyLimitRepository(db *sqlx.DB) *UserDailyLimitRepository {
	return &UserDailyLimitRepository{db: db}
}

func (r *UserDailyLimitRepository) GetByUserIDAndDate(ctx context.Context, userID uuid.UUID, date time.Time) (*domain.UserDailyLimit, error) {
	query := `
		SELECT id, user_id, organization_id, date, message_count, daily_limit,
		       is_limit_exceeded, last_reset_at, created_at, updated_at
		FROM user_daily_limits 
		WHERE user_id = $1 AND date = $2`
	
	var limit domain.UserDailyLimit
	err := r.db.GetContext(ctx, &limit, query, userID, date)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("daily limit not found")
		}
		return nil, err
	}
	
	return &limit, nil
}

func (r *UserDailyLimitRepository) Create(ctx context.Context, limit *domain.UserDailyLimit) error {
	query := `
		INSERT INTO user_daily_limits (
			id, user_id, organization_id, date, message_count, daily_limit,
			is_limit_exceeded, last_reset_at, created_at, updated_at
		) VALUES (
			:id, :user_id, :organization_id, :date, :message_count, :daily_limit,
			:is_limit_exceeded, :last_reset_at, :created_at, :updated_at
		)`
	
	_, err := r.db.NamedExecContext(ctx, query, limit)
	return err
}

func (r *UserDailyLimitRepository) Update(ctx context.Context, limit *domain.UserDailyLimit) error {
	query := `
		UPDATE user_daily_limits 
		SET message_count = :message_count, daily_limit = :daily_limit,
		    is_limit_exceeded = :is_limit_exceeded, updated_at = :updated_at
		WHERE id = :id`
	
	_, err := r.db.NamedExecContext(ctx, query, limit)
	return err
}

func (r *UserDailyLimitRepository) IncrementMessageCount(ctx context.Context, userID uuid.UUID, date time.Time) error {
	query := `
		UPDATE user_daily_limits 
		SET message_count = message_count + 1, 
		    is_limit_exceeded = (message_count + 1) >= daily_limit,
		    updated_at = NOW()
		WHERE user_id = $1 AND date = $2`
	
	result, err := r.db.ExecContext(ctx, query, userID, date)
	if err != nil {
		return err
	}
	
	// Check if any row was updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	// If no rows were updated, it means the record doesn't exist
	if rowsAffected == 0 {
		return fmt.Errorf("no daily limit record found for user")
	}
	
	return nil
}

// UpsertDailyLimit creates or updates a daily limit record (UPSERT)
func (r *UserDailyLimitRepository) UpsertDailyLimit(ctx context.Context, limit *domain.UserDailyLimit) error {
	query := `
		INSERT INTO user_daily_limits (
			id, user_id, organization_id, date, message_count, daily_limit,
			is_limit_exceeded, last_reset_at, created_at, updated_at
		) VALUES (
			:id, :user_id, :organization_id, :date, :message_count, :daily_limit,
			:is_limit_exceeded, :last_reset_at, :created_at, :updated_at
		)
		ON CONFLICT (user_id, date) 
		DO UPDATE SET
			message_count = EXCLUDED.message_count,
			is_limit_exceeded = EXCLUDED.is_limit_exceeded,
			updated_at = EXCLUDED.updated_at`
	
	_, err := r.db.NamedExecContext(ctx, query, limit)
	return err
}

func (r *UserDailyLimitRepository) ResetDailyLimits(ctx context.Context, date time.Time) error {
	query := `UPDATE user_daily_limits SET message_count = 0, is_limit_exceeded = false, last_reset_at = NOW() WHERE date = $1`
	_, err := r.db.ExecContext(ctx, query, date)
	return err
}

func (r *UserDailyLimitRepository) GetExceededLimits(ctx context.Context, orgID uuid.UUID, date time.Time) ([]*domain.UserDailyLimit, error) {
	query := `
		SELECT id, user_id, organization_id, date, message_count, daily_limit,
		       is_limit_exceeded, last_reset_at, created_at, updated_at
		FROM user_daily_limits 
		WHERE organization_id = $1 AND date = $2 AND is_limit_exceeded = true`
	
	var limits []*domain.UserDailyLimit
	err := r.db.SelectContext(ctx, &limits, query, orgID, date)
	return limits, err
}

// AgentActivityLogRepository implements domain.AgentActivityLogRepository
type AgentActivityLogRepository struct {
	db *sqlx.DB
}

// NewAgentActivityLogRepository creates a new agent activity log repository
func NewAgentActivityLogRepository(db *sqlx.DB) *AgentActivityLogRepository {
	return &AgentActivityLogRepository{db: db}
}

func (r *AgentActivityLogRepository) Create(ctx context.Context, log *domain.AgentActivityLog) error {
	// Convert activity data to JSON bytes
	activityDataJSON, err := json.Marshal(log.ActivityData)
	if err != nil {
		return fmt.Errorf("failed to marshal activity data: %w", err)
	}

	query := `
		INSERT INTO agent_activity_logs (
			id, organization_id, agent_id, user_id, conversation_id, activity_type,
			activity_data, ip_address, user_agent, created_at
		) VALUES (
			:id, :organization_id, :agent_id, :user_id, :conversation_id, :activity_type,
			:activity_data_json, :ip_address, :user_agent, :created_at
		)`
	
	args := map[string]interface{}{
		"id":                log.ID,
		"organization_id":   log.OrganizationID,
		"agent_id":          log.AgentID,
		"user_id":           log.UserID,
		"conversation_id":   log.ConversationID,
		"activity_type":     log.ActivityType,
		"activity_data_json": activityDataJSON,
		"ip_address":        log.IPAddress,
		"user_agent":        log.UserAgent,
		"created_at":        log.CreatedAt,
	}
	
	_, err = r.db.NamedExecContext(ctx, query, args)
	return err
}

func (r *AgentActivityLogRepository) GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AgentActivityLog, error) {
	query := `
		SELECT id, organization_id, agent_id, user_id, conversation_id, activity_type,
		       activity_data, ip_address, user_agent, created_at
		FROM agent_activity_logs 
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []*domain.AgentActivityLog
	for rows.Next() {
		var log domain.AgentActivityLog
		var activityDataJSON []byte
		
		err := rows.Scan(
			&log.ID,
			&log.OrganizationID,
			&log.AgentID,
			&log.UserID,
			&log.ConversationID,
			&log.ActivityType,
			&activityDataJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal activity data
		if len(activityDataJSON) > 0 {
			if err := json.Unmarshal(activityDataJSON, &log.ActivityData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal activity data: %w", err)
			}
		} else {
			log.ActivityData = make(map[string]interface{})
		}
		
		logs = append(logs, &log)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return logs, nil
}

func (r *AgentActivityLogRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.AgentActivityLog, error) {
	query := `
		SELECT id, organization_id, agent_id, user_id, conversation_id, activity_type,
		       activity_data, ip_address, user_agent, created_at
		FROM agent_activity_logs 
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []*domain.AgentActivityLog
	for rows.Next() {
		var log domain.AgentActivityLog
		var activityDataJSON []byte
		
		err := rows.Scan(
			&log.ID,
			&log.OrganizationID,
			&log.AgentID,
			&log.UserID,
			&log.ConversationID,
			&log.ActivityType,
			&activityDataJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal activity data
		if len(activityDataJSON) > 0 {
			if err := json.Unmarshal(activityDataJSON, &log.ActivityData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal activity data: %w", err)
			}
		} else {
			log.ActivityData = make(map[string]interface{})
		}
		
		logs = append(logs, &log)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return logs, nil
}

func (r *AgentActivityLogRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.AgentActivityLog, error) {
	query := `
		SELECT id, organization_id, agent_id, user_id, conversation_id, activity_type,
		       activity_data, ip_address, user_agent, created_at
		FROM agent_activity_logs 
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []*domain.AgentActivityLog
	for rows.Next() {
		var log domain.AgentActivityLog
		var activityDataJSON []byte
		
		err := rows.Scan(
			&log.ID,
			&log.OrganizationID,
			&log.AgentID,
			&log.UserID,
			&log.ConversationID,
			&log.ActivityType,
			&activityDataJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal activity data
		if len(activityDataJSON) > 0 {
			if err := json.Unmarshal(activityDataJSON, &log.ActivityData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal activity data: %w", err)
			}
		} else {
			log.ActivityData = make(map[string]interface{})
		}
		
		logs = append(logs, &log)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return logs, nil
}

func (r *AgentActivityLogRepository) GetByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.AgentActivityLog, error) {
	query := `
		SELECT id, organization_id, agent_id, user_id, conversation_id, activity_type,
		       activity_data, ip_address, user_agent, created_at
		FROM agent_activity_logs 
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []*domain.AgentActivityLog
	for rows.Next() {
		var log domain.AgentActivityLog
		var activityDataJSON []byte
		
		err := rows.Scan(
			&log.ID,
			&log.OrganizationID,
			&log.AgentID,
			&log.UserID,
			&log.ConversationID,
			&log.ActivityType,
			&activityDataJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Unmarshal activity data
		if len(activityDataJSON) > 0 {
			if err := json.Unmarshal(activityDataJSON, &log.ActivityData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal activity data: %w", err)
			}
		} else {
			log.ActivityData = make(map[string]interface{})
		}
		
		logs = append(logs, &log)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return logs, nil
}

func (r *AgentActivityLogRepository) GetByActivityType(ctx context.Context, activityType string, limit, offset int) ([]*domain.AgentActivityLog, error) {
	query := `
		SELECT id, organization_id, agent_id, user_id, conversation_id, activity_type,
		       activity_data, ip_address, user_agent, created_at
		FROM agent_activity_logs 
		WHERE activity_type = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	
	var logs []*domain.AgentActivityLog
	err := r.db.SelectContext(ctx, &logs, query, activityType, limit, offset)
	return logs, err
}

func (r *AgentActivityLogRepository) CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM agent_activity_logs WHERE agent_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, agentID)
	return count, err
}

func (r *AgentActivityLogRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM agent_activity_logs WHERE user_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}

func (r *AgentActivityLogRepository) CountByOrganizationID(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM agent_activity_logs WHERE organization_id = $1`
	var count int
	err := r.db.GetContext(ctx, &count, query, orgID)
	return count, err
}

// ChatSystemConfigRepository implements domain.ChatSystemConfigRepository
type ChatSystemConfigRepository struct {
	db *sqlx.DB
}

// NewChatSystemConfigRepository creates a new chat system config repository
func NewChatSystemConfigRepository(db *sqlx.DB) *ChatSystemConfigRepository {
	return &ChatSystemConfigRepository{db: db}
}

func (r *ChatSystemConfigRepository) GetByOrganizationIDAndKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.ChatSystemConfig, error) {
	query := `
		SELECT id, organization_id, config_key, config_value, description, is_active, created_at, updated_at
		FROM chat_system_config 
		WHERE organization_id = $1 AND config_key = $2 AND is_active = true`
	
	var config domain.ChatSystemConfig
	err := r.db.GetContext(ctx, &config, query, orgID, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("config not found")
		}
		return nil, err
	}
	
	return &config, nil
}

func (r *ChatSystemConfigRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*domain.ChatSystemConfig, error) {
	query := `
		SELECT id, organization_id, config_key, config_value, description, is_active, created_at, updated_at
		FROM chat_system_config 
		WHERE organization_id = $1 AND is_active = true
		ORDER BY config_key`
	
	var configs []*domain.ChatSystemConfig
	err := r.db.SelectContext(ctx, &configs, query, orgID)
	return configs, err
}

func (r *ChatSystemConfigRepository) Create(ctx context.Context, config *domain.ChatSystemConfig) error {
	query := `
		INSERT INTO chat_system_config (
			id, organization_id, config_key, config_value, description, is_active, created_at, updated_at
		) VALUES (
			:id, :organization_id, :config_key, :config_value, :description, :is_active, :created_at, :updated_at
		)`
	
	_, err := r.db.NamedExecContext(ctx, query, config)
	return err
}

func (r *ChatSystemConfigRepository) Update(ctx context.Context, config *domain.ChatSystemConfig) error {
	query := `
		UPDATE chat_system_config 
		SET config_value = :config_value, description = :description, is_active = :is_active, updated_at = :updated_at
		WHERE id = :id`
	
	_, err := r.db.NamedExecContext(ctx, query, config)
	return err
}

func (r *ChatSystemConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE chat_system_config SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}


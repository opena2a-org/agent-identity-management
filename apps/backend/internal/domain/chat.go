package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ChatConversation represents a conversation between a user and an agent
type ChatConversation struct {
	ID             uuid.UUID              `json:"id" db:"id"`
	OrganizationID uuid.UUID              `json:"organization_id" db:"organization_id"`
	UserID         uuid.UUID              `json:"user_id" db:"user_id"`
	AgentID        uuid.UUID              `json:"agent_id" db:"agent_id"`
	Title          string                 `json:"title" db:"title"`
	Status         string                 `json:"status" db:"status"`
	Metadata       map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
	LastMessageAt  *time.Time             `json:"last_message_at" db:"last_message_at"`
	
	// Related entities
	User  *User  `json:"user,omitempty"`
	Agent *Agent `json:"agent,omitempty"`
}

// ChatMessage represents a message within a conversation
type ChatMessage struct {
	ID               uuid.UUID              `json:"id" db:"id"`
	ConversationID   uuid.UUID              `json:"conversation_id" db:"conversation_id"`
	UserID           uuid.UUID              `json:"user_id" db:"user_id"`
	AgentID          uuid.UUID              `json:"agent_id" db:"agent_id"`
	MessageType      string                 `json:"message_type" db:"message_type"`
	Content          string                 `json:"content" db:"content"`
	Role             string                 `json:"role" db:"role"`
	Metadata         map[string]interface{} `json:"metadata" db:"metadata"`
	ParentMessageID  *uuid.UUID             `json:"parent_message_id" db:"parent_message_id"`
	IsEdited         bool                   `json:"is_edited" db:"is_edited"`
	EditedAt         *time.Time             `json:"edited_at" db:"edited_at"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	
	// Related entities
	User         *User           `json:"user,omitempty"`
	Agent        *Agent          `json:"agent,omitempty"`
	ParentMessage *ChatMessage   `json:"parent_message,omitempty"`
}

// UserDailyLimit tracks daily message limits for users
type UserDailyLimit struct {
	ID               uuid.UUID `json:"id" db:"id"`
	UserID           uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID   uuid.UUID `json:"organization_id" db:"organization_id"`
	Date             time.Time `json:"date" db:"date"`
	MessageCount     int       `json:"message_count" db:"message_count"`
	DailyLimit       int       `json:"daily_limit" db:"daily_limit"`
	IsLimitExceeded  bool      `json:"is_limit_exceeded" db:"is_limit_exceeded"`
	LastResetAt      time.Time `json:"last_reset_at" db:"last_reset_at"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
	
	// Related entities
	User *User `json:"user,omitempty"`
}

// AgentActivityLog tracks all agent-related activities
type AgentActivityLog struct {
	ID             uuid.UUID              `json:"id" db:"id"`
	OrganizationID uuid.UUID              `json:"organization_id" db:"organization_id"`
	AgentID        uuid.UUID              `json:"agent_id" db:"agent_id"`
	UserID         *uuid.UUID             `json:"user_id" db:"user_id"`
	ConversationID *uuid.UUID             `json:"conversation_id" db:"conversation_id"`
	ActivityType   string                 `json:"activity_type" db:"activity_type"`
	ActivityData   map[string]interface{} `json:"activity_data" db:"activity_data"`
	IPAddress      string                 `json:"ip_address" db:"ip_address"`
	UserAgent      string                 `json:"user_agent" db:"user_agent"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	
	// Related entities
	User         *User              `json:"user,omitempty"`
	Agent        *Agent             `json:"agent,omitempty"`
	Conversation *ChatConversation  `json:"conversation,omitempty"`
}

// ChatSystemConfig stores configuration for chat system
type ChatSystemConfig struct {
	ID             uuid.UUID              `json:"id" db:"id"`
	OrganizationID uuid.UUID              `json:"organization_id" db:"organization_id"`
	ConfigKey      string                 `json:"config_key" db:"config_key"`
	ConfigValue    map[string]interface{} `json:"config_value" db:"config_value"`
	Description    string                 `json:"description" db:"description"`
	IsActive       bool                   `json:"is_active" db:"is_active"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}

// Chat conversation status constants
const (
	ConversationStatusActive   = "active"
	ConversationStatusArchived = "archived"
	ConversationStatusDeleted  = "deleted"
)

// Chat message type constants
const (
	MessageTypeText   = "text"
	MessageTypeImage  = "image"
	MessageTypeFile   = "file"
	MessageTypeSystem = "system"
)

// Chat message role constants
const (
	MessageRoleUser   = "user"
	MessageRoleAgent  = "agent"
	MessageRoleSystem = "system"
)

// Agent activity type constants
const (
	ActivityTypeMessageSent           = "message_sent"
	ActivityTypeMessageReceived       = "message_received"
	ActivityTypeConversationStarted   = "conversation_started"
	ActivityTypeConversationEnded     = "conversation_ended"
	ActivityTypeLimitExceeded         = "limit_exceeded"
	ActivityTypeAgentResponseGenerated = "agent_response_generated"
	ActivityTypeUserTyping            = "user_typing"
	ActivityTypeUserStoppedTyping     = "user_stopped_typing"
	ActivityTypeConversationArchived  = "conversation_archived"
	ActivityTypeConversationDeleted   = "conversation_deleted"
	ActivityTypeMessageEdited         = "message_edited"
	ActivityTypeFileUploaded          = "file_uploaded"
)

// Chat system configuration keys
const (
	ConfigKeyDailyMessageLimit      = "daily_message_limit"
	ConfigKeyChatEnabled            = "chat_enabled"
	ConfigKeyMaxConversationsPerUser = "max_conversations_per_user"
	ConfigKeyMessageRetentionDays   = "message_retention_days"
	ConfigKeyAutoArchiveDays        = "auto_archive_days"
)

// Chat repository interfaces
type ChatConversationRepository interface {
	Create(ctx context.Context, conversation *ChatConversation) error
	GetByID(ctx context.Context, id uuid.UUID) (*ChatConversation, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*ChatConversation, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*ChatConversation, error)
	GetByOrganizationID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*ChatConversation, error)
	Update(ctx context.Context, conversation *ChatConversation) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastMessageAt(ctx context.Context, conversationID uuid.UUID, timestamp time.Time) error
	CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error)
}

type ChatMessageRepository interface {
	Create(ctx context.Context, message *ChatMessage) error
	GetByID(ctx context.Context, id uuid.UUID) (*ChatMessage, error)
	GetByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*ChatMessage, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*ChatMessage, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*ChatMessage, error)
	Update(ctx context.Context, message *ChatMessage) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountByConversationID(ctx context.Context, conversationID uuid.UUID) (int, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error)
	CountUserMessagesToday(ctx context.Context, userID uuid.UUID, date time.Time) (int, error)
}

type UserDailyLimitRepository interface {
	GetByUserIDAndDate(ctx context.Context, userID uuid.UUID, date time.Time) (*UserDailyLimit, error)
	Create(ctx context.Context, limit *UserDailyLimit) error
	Update(ctx context.Context, limit *UserDailyLimit) error
	UpsertDailyLimit(ctx context.Context, limit *UserDailyLimit) error
	IncrementMessageCount(ctx context.Context, userID uuid.UUID, date time.Time) error
	ResetDailyLimits(ctx context.Context, date time.Time) error
	GetExceededLimits(ctx context.Context, orgID uuid.UUID, date time.Time) ([]*UserDailyLimit, error)
}

type AgentActivityLogRepository interface {
	Create(ctx context.Context, log *AgentActivityLog) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*AgentActivityLog, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*AgentActivityLog, error)
	GetByOrganizationID(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*AgentActivityLog, error)
	GetByConversationID(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*AgentActivityLog, error)
	GetByActivityType(ctx context.Context, activityType string, limit, offset int) ([]*AgentActivityLog, error)
	CountByAgentID(ctx context.Context, agentID uuid.UUID) (int, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	CountByOrganizationID(ctx context.Context, orgID uuid.UUID) (int, error)
}

type ChatSystemConfigRepository interface {
	GetByOrganizationIDAndKey(ctx context.Context, orgID uuid.UUID, key string) (*ChatSystemConfig, error)
	GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*ChatSystemConfig, error)
	Create(ctx context.Context, config *ChatSystemConfig) error
	Update(ctx context.Context, config *ChatSystemConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
}


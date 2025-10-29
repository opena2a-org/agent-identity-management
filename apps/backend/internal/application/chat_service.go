package application

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gage-technologies/mistral-go"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ChatService handles chat-related business logic
type ChatService struct {
	conversationRepo domain.ChatConversationRepository
	messageRepo      domain.ChatMessageRepository
	dailyLimitRepo   domain.UserDailyLimitRepository
	activityRepo     domain.AgentActivityLogRepository
	configRepo       domain.ChatSystemConfigRepository
	auditService     *AuditService
	mistralClient    *mistral.MistralClient
}

// NewChatService creates a new chat service
func NewChatService(
	conversationRepo domain.ChatConversationRepository,
	messageRepo domain.ChatMessageRepository,
	dailyLimitRepo domain.UserDailyLimitRepository,
	activityRepo domain.AgentActivityLogRepository,
	configRepo domain.ChatSystemConfigRepository,
	auditService *AuditService,
) *ChatService {
	// Initialize Mistral client
	mistralAPIKey := os.Getenv("MISTRAL_API_KEY")
	
	// Debug: Print all environment variables that contain "MISTRAL"
	fmt.Println("🔍 Debugging Mistral environment variables:")
	for _, env := range os.Environ() {
		if strings.Contains(strings.ToUpper(env), "MISTRAL") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				if len(value) > 8 {
					value = value[:8] + "..."
				}
				fmt.Printf("  %s=%s\n", key, value)
			}
		}
	}
	
	var mistralClient *mistral.MistralClient
	if mistralAPIKey != "" {
		mistralClient = mistral.NewMistralClientDefault(mistralAPIKey)
		keyPreview := mistralAPIKey
		if len(mistralAPIKey) > 8 {
			keyPreview = mistralAPIKey[:8]
		}
		fmt.Printf("✅ Mistral client initialized with API key: %s...\n", keyPreview)
	} else {
		fmt.Println("⚠️  MISTRAL_API_KEY not found, using simulated responses")
	}

	return &ChatService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		dailyLimitRepo:   dailyLimitRepo,
		activityRepo:     activityRepo,
		configRepo:       configRepo,
		auditService:     auditService,
		mistralClient:    mistralClient,
	}
}

// CreateConversationRequest represents a request to create a conversation
type CreateConversationRequest struct {
	UserID      uuid.UUID              `json:"user_id"`
	AgentID     uuid.UUID              `json:"agent_id"`
	Title       string                 `json:"title"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SendMessageRequest represents a request to send a message
type SendMessageRequest struct {
	ConversationID uuid.UUID              `json:"conversation_id"`
	UserID         uuid.UUID              `json:"user_id"`
	AgentID        uuid.UUID              `json:"agent_id"`
	Content        string                 `json:"content"`
	MessageType    string                 `json:"message_type"`
	Metadata       map[string]interface{} `json:"metadata"`
	ParentMessageID *uuid.UUID            `json:"parent_message_id"`
}

// ChatResponse represents a response from the chat system
type ChatResponse struct {
	Message       *domain.ChatMessage   `json:"message"`
	Conversation  *domain.ChatConversation `json:"conversation"`
	DailyLimit    *UserDailyLimitInfo   `json:"daily_limit"`
	AgentActivity *AgentActivityInfo    `json:"agent_activity"`
}

// UserDailyLimitInfo provides information about user's daily limits
type UserDailyLimitInfo struct {
	MessageCount     int  `json:"message_count"`
	DailyLimit       int  `json:"daily_limit"`
	IsLimitExceeded  bool `json:"is_limit_exceeded"`
	RemainingMessages int `json:"remaining_messages"`
}

// AgentActivityInfo provides information about agent activity
type AgentActivityInfo struct {
	ActivityType string                 `json:"activity_type"`
	ActivityData map[string]interface{} `json:"activity_data"`
	Timestamp    time.Time              `json:"timestamp"`
}

// CreateConversation creates a new chat conversation
func (s *ChatService) CreateConversation(ctx context.Context, req *CreateConversationRequest, orgID uuid.UUID) (*domain.ChatConversation, error) {
	// Check if chat is enabled for the organization
	enabled, err := s.isChatEnabled(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check chat status: %w", err)
	}
	if !enabled {
		return nil, fmt.Errorf("chat is disabled for this organization")
	}

	// Check user's conversation limit
	maxConversations, err := s.getMaxConversationsPerUser(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation limit: %w", err)
	}

	currentCount, err := s.conversationRepo.CountByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to count user conversations: %w", err)
	}

	if currentCount >= maxConversations {
		return nil, fmt.Errorf("user has reached maximum conversation limit of %d", maxConversations)
	}

	// Create conversation
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	conversation := &domain.ChatConversation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         req.UserID,
		AgentID:        req.AgentID,
		Title:          req.Title,
		Status:         domain.ConversationStatusActive,
		Metadata:       metadata,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.conversationRepo.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Log activity
	activity := &domain.AgentActivityLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AgentID:        req.AgentID,
		UserID:         &req.UserID,
		ConversationID: &conversation.ID,
		ActivityType:   domain.ActivityTypeConversationStarted,
		ActivityData: map[string]interface{}{
			"conversation_id": conversation.ID.String(),
			"title":          conversation.Title,
		},
		CreatedAt: time.Now(),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		// Log error but don't fail the conversation creation
		fmt.Printf("Failed to log conversation start activity: %v\n", err)
	}

	// Audit log
	auditLog := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         req.UserID,
		Action:         domain.AuditAction("conversation_created"),
		ResourceType:   "chat_conversation",
		ResourceID:     conversation.ID,
		Metadata:       map[string]interface{}{"agent_id": req.AgentID.String()},
		Timestamp:      time.Now(),
	}
	if err := s.auditService.Log(ctx, auditLog); err != nil {
		fmt.Printf("Failed to audit conversation creation: %v\n", err)
	}

	return conversation, nil
}

// SendMessage sends a message in a conversation
func (s *ChatService) SendMessage(ctx context.Context, req *SendMessageRequest, orgID uuid.UUID, ipAddress, userAgent string) (*ChatResponse, error) {
	// Check if chat is enabled for the organization
	enabled, err := s.isChatEnabled(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check chat status: %w", err)
	}
	if !enabled {
		return nil, fmt.Errorf("chat is disabled for this organization")
	}

	// Get conversation to retrieve agent_id
	conversation, err := s.conversationRepo.GetByID(ctx, req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// ⭐ Check daily message limit FIRST (without incrementing)
	dailyLimit, err := s.GetUserDailyLimit(ctx, req.UserID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check daily limit: %w", err)
	}

	// ⭐ If limit exceeded, return error immediately WITHOUT incrementing or calling Mistral
	if dailyLimit.IsLimitExceeded {
		// Log limit exceeded activity
		activity := &domain.AgentActivityLog{
			ID:             uuid.New(),
			OrganizationID: orgID,
			AgentID:        conversation.AgentID,
			UserID:         &req.UserID,
			ConversationID: &req.ConversationID,
			ActivityType:   domain.ActivityTypeLimitExceeded,
			ActivityData: map[string]interface{}{
				"message_count":    dailyLimit.MessageCount,
				"daily_limit":      dailyLimit.DailyLimit,
				"conversation_id":  req.ConversationID.String(),
			},
			IPAddress: ipAddress,
			UserAgent: userAgent,
			CreatedAt: time.Now(),
		}

		if err := s.activityRepo.Create(ctx, activity); err != nil {
			fmt.Printf("Failed to log limit exceeded activity: %v\n", err)
		}

		return nil, fmt.Errorf("daily message limit of %d exceeded", dailyLimit.DailyLimit)
	}

	// Create message
	metadata := req.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	message := &domain.ChatMessage{
		ID:               uuid.New(),
		ConversationID:   req.ConversationID,
		UserID:           req.UserID,
		AgentID:          conversation.AgentID,
		MessageType:      req.MessageType,
		Content:          req.Content,
		Role:             domain.MessageRoleUser,
		Metadata:         metadata,
		ParentMessageID:  req.ParentMessageID,
		IsEdited:         false,
		CreatedAt:        time.Now(),
	}

	if err := s.messageRepo.Create(ctx, message); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// ⭐ NOW increment the message count (after message created successfully)
	today := time.Now().Truncate(24 * time.Hour)
	
	// Try to increment first (if record exists)
	err = s.dailyLimitRepo.IncrementMessageCount(ctx, req.UserID, today)
	if err != nil {
		// Record doesn't exist - use UPSERT to create it
		fmt.Printf("📊 Daily limit record not found for user %s, creating new record...\n", req.UserID)
		
		defaultLimit, limitErr := s.getDailyMessageLimit(ctx, orgID)
		if limitErr != nil {
			fmt.Printf("⚠️  Failed to get default daily limit: %v, using fallback 5000\n", limitErr)
			defaultLimit = 5000 // Fallback to default
		}
		
		newLimit := &domain.UserDailyLimit{
			ID:              uuid.New(),
			UserID:          req.UserID,
			OrganizationID:  orgID,
			Date:            today,
			MessageCount:    1, // Start with 1 (this message)
			DailyLimit:      defaultLimit,
			IsLimitExceeded: false,
			LastResetAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		
		fmt.Printf("📝 Attempting to upsert daily limit: UserID=%s, Date=%s, Count=1, Limit=%d\n", 
			req.UserID, today.Format("2006-01-02"), defaultLimit)
		
		// Use UPSERT to avoid duplicates
		if upsertErr := s.dailyLimitRepo.UpsertDailyLimit(ctx, newLimit); upsertErr != nil {
			fmt.Printf("❌ CRITICAL: Failed to upsert daily limit record: %v\n", upsertErr)
			// Don't fail the message send, but log the error
		} else {
			fmt.Printf("✅ Successfully created daily limit record for user %s\n", req.UserID)
		}
	} else {
		fmt.Printf("✅ Successfully incremented message count for user %s\n", req.UserID)
	}

	// Update conversation's last message timestamp
	if err := s.conversationRepo.UpdateLastMessageAt(ctx, req.ConversationID, time.Now()); err != nil {
		fmt.Printf("Failed to update conversation last message time: %v\n", err)
	}

	// Conversation details are already available from the beginning of the method

	// Log message sent activity
	activity := &domain.AgentActivityLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AgentID:        conversation.AgentID,
		UserID:         &req.UserID,
		ConversationID: &req.ConversationID,
		ActivityType:   domain.ActivityTypeMessageSent,
		ActivityData: map[string]interface{}{
			"message_id":      message.ID.String(),
			"conversation_id": req.ConversationID.String(),
			"message_type":    req.MessageType,
			"content_length":  len(req.Content),
		},
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: time.Now(),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		fmt.Printf("Failed to log message sent activity: %v\n", err)
	}

	// Generate agent response (simulated for now)
	agentResponse, err := s.generateAgentResponse(ctx, message, conversation)
	if err != nil {
		fmt.Printf("Failed to generate agent response: %v\n", err)
		// Don't fail the message creation if agent response fails
	}

	// ⭐ Get updated daily limit info after increment
	updatedDailyLimit, err := s.GetUserDailyLimit(ctx, req.UserID, orgID)
	if err != nil {
		fmt.Printf("Failed to get updated daily limit: %v\n", err)
		// Use the old limit info if we can't get updated one
		updatedDailyLimit = dailyLimit
	}

	// Prepare response
	response := &ChatResponse{
		Message:      message,
		Conversation: conversation,
		DailyLimit: &UserDailyLimitInfo{
			MessageCount:      updatedDailyLimit.MessageCount,
			DailyLimit:        updatedDailyLimit.DailyLimit,
			IsLimitExceeded:   updatedDailyLimit.IsLimitExceeded,
			RemainingMessages: updatedDailyLimit.RemainingMessages,
		},
		AgentActivity: &AgentActivityInfo{
			ActivityType: domain.ActivityTypeMessageSent,
			ActivityData: activity.ActivityData,
			Timestamp:    activity.CreatedAt,
		},
	}

	// Add agent response if available
	if agentResponse != nil {
		response.Message = agentResponse
	}

	// Audit log
	auditLog := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         req.UserID,
		Action:         domain.AuditAction("message_sent"),
		ResourceType:   "chat_message",
		ResourceID:     message.ID,
		Metadata:       map[string]interface{}{"conversation_id": req.ConversationID.String()},
		Timestamp:      time.Now(),
	}
	if err := s.auditService.Log(ctx, auditLog); err != nil {
		fmt.Printf("Failed to audit message sending: %v\n", err)
	}

	return response, nil
}

// GetConversations retrieves conversations for a user
func (s *ChatService) GetConversations(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ChatConversation, error) {
	return s.conversationRepo.GetByUserID(ctx, userID, limit, offset)
}

// GetMessages retrieves messages for a conversation
func (s *ChatService) GetMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]*domain.ChatMessage, error) {
	return s.messageRepo.GetByConversationID(ctx, conversationID, limit, offset)
}

// DeleteConversation deletes a conversation and all related messages
func (s *ChatService) DeleteConversation(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID, orgID uuid.UUID) error {
	// Verify the conversation belongs to the user and organization
	conversation, err := s.conversationRepo.GetByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	if conversation.UserID != userID || conversation.OrganizationID != orgID {
		return fmt.Errorf("unauthorized: conversation does not belong to user")
	}

	// Delete the conversation (messages will be cascade deleted due to foreign key constraints)
	err = s.conversationRepo.Delete(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// GetAgentActivity retrieves activity logs for an agent
func (s *ChatService) GetAgentActivity(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AgentActivityLog, error) {
	return s.activityRepo.GetByAgentID(ctx, agentID, limit, offset)
}

// GetUserDailyLimit retrieves user's daily limit information
func (s *ChatService) GetUserDailyLimit(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (*UserDailyLimitInfo, error) {
	today := time.Now().Truncate(24 * time.Hour)
	limit, err := s.dailyLimitRepo.GetByUserIDAndDate(ctx, userID, today)
	if err != nil {
		// If no limit record exists for TODAY, count actual messages from chat_messages table
		actualMessageCount, countErr := s.messageRepo.CountUserMessagesToday(ctx, userID, today)
		if countErr != nil {
			fmt.Printf("Failed to count user messages: %v\n", countErr)
			actualMessageCount = 0
		}

		// Get default limit from config
		dailyLimit, err := s.getDailyMessageLimit(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily message limit: %w", err)
		}

		// Return actual message count (not just 0)
		isExceeded := actualMessageCount >= dailyLimit
		remaining := dailyLimit - actualMessageCount
		if remaining < 0 {
			remaining = 0
		}

		return &UserDailyLimitInfo{
			MessageCount:      actualMessageCount,
			DailyLimit:        dailyLimit,
			IsLimitExceeded:   isExceeded,
			RemainingMessages: remaining,
		}, nil
	}

	// ⭐ Double-check the date matches today (safety check)
	if limit.Date.Truncate(24 * time.Hour) != today {
		// Record is from a different day - count actual messages for today
		actualMessageCount, countErr := s.messageRepo.CountUserMessagesToday(ctx, userID, today)
		if countErr != nil {
			fmt.Printf("Failed to count user messages: %v\n", countErr)
			actualMessageCount = 0
		}

		// Get default limit from config
		dailyLimit, err := s.getDailyMessageLimit(ctx, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get daily message limit: %w", err)
		}

		// Return actual message count
		isExceeded := actualMessageCount >= dailyLimit
		remaining := dailyLimit - actualMessageCount
		if remaining < 0 {
			remaining = 0
		}

		return &UserDailyLimitInfo{
			MessageCount:      actualMessageCount,
			DailyLimit:        dailyLimit,
			IsLimitExceeded:   isExceeded,
			RemainingMessages: remaining,
		}, nil
	}

	// Return current day's limit info from database record
	return &UserDailyLimitInfo{
		MessageCount:     limit.MessageCount,
		DailyLimit:       limit.DailyLimit,
		IsLimitExceeded:  limit.IsLimitExceeded,
		RemainingMessages: limit.DailyLimit - limit.MessageCount,
	}, nil
}

// Helper methods

func (s *ChatService) isChatEnabled(ctx context.Context, orgID uuid.UUID) (bool, error) {
	config, err := s.configRepo.GetByOrganizationIDAndKey(ctx, orgID, domain.ConfigKeyChatEnabled)
	if err != nil {
		// If no config found, default to enabled
		return true, nil
	}

	enabled, ok := config.ConfigValue["enabled"].(bool)
	if !ok {
		// If config value is not boolean, default to enabled
		return true, nil
	}

	return enabled, nil
}

func (s *ChatService) getDailyMessageLimit(ctx context.Context, orgID uuid.UUID) (int, error) {
	config, err := s.configRepo.GetByOrganizationIDAndKey(ctx, orgID, domain.ConfigKeyDailyMessageLimit)
	if err != nil {
		// Default to 5000 if no config found
		return 5000, nil
	}

	limit, ok := config.ConfigValue["limit"].(float64)
	if !ok {
		// If config value is not a number, default to 5000
		return 5000, nil
	}

	return int(limit), nil
}

func (s *ChatService) getMaxConversationsPerUser(ctx context.Context, orgID uuid.UUID) (int, error) {
	config, err := s.configRepo.GetByOrganizationIDAndKey(ctx, orgID, domain.ConfigKeyMaxConversationsPerUser)
	if err != nil {
		// Default to 100 if no config found
		return 100, nil
	}

	limit, ok := config.ConfigValue["limit"].(float64)
	if !ok {
		// If config value is not a number, default to 100
		return 100, nil
	}

	return int(limit), nil
}

func (s *ChatService) generateAgentResponse(ctx context.Context, userMessage *domain.ChatMessage, conversation *domain.ChatConversation) (*domain.ChatMessage, error) {
	// Check if Mistral client is available
	if s.mistralClient == nil {
		fmt.Println("⚠️  Mistral client is nil, falling back to simulated response")
		// Fallback to simulated response if Mistral is not configured
		return s.generateSimulatedResponse(ctx, userMessage, conversation)
	}
	
	fmt.Println("✅ Using Mistral client for agent response")

	// Get conversation history for context
	messages, err := s.messageRepo.GetByConversationID(ctx, userMessage.ConversationID, 10, 0)
	if err != nil {
		fmt.Printf("Failed to get conversation history: %v\n", err)
		return s.generateSimulatedResponse(ctx, userMessage, conversation)
	}

	// Convert to Mistral chat messages format
	var mistralMessages []mistral.ChatMessage
	for _, msg := range messages {
		role := mistral.RoleUser
		if msg.Role == domain.MessageRoleAgent {
			role = mistral.RoleAssistant
		} else if msg.Role == domain.MessageRoleSystem {
			role = mistral.RoleSystem
		}
		
		mistralMessages = append(mistralMessages, mistral.ChatMessage{
			Content: msg.Content,
			Role:    role,
		})
	}

	// Add the current user message
	mistralMessages = append(mistralMessages, mistral.ChatMessage{
		Content: userMessage.Content,
		Role:    mistral.RoleUser,
	})

	// Call Mistral API
	fmt.Printf("🤖 Calling Mistral API with %d messages\n", len(mistralMessages))
	response, err := s.mistralClient.Chat("mistral-small", mistralMessages, &mistral.ChatRequestParams{
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        0.9, // Set TopP to a valid value (0, 1]
	})
	if err != nil {
		fmt.Printf("❌ Mistral API error: %v\n", err)
		return s.generateSimulatedResponse(ctx, userMessage, conversation)
	}
	
	fmt.Printf("✅ Mistral API response received: %s\n", response.Choices[0].Message.Content)

	// Create agent response
	agentResponse := &domain.ChatMessage{
		ID:             uuid.New(),
		ConversationID: userMessage.ConversationID,
		UserID:         userMessage.UserID,
		AgentID:        userMessage.AgentID,
		MessageType:    domain.MessageTypeText,
		Content:        response.Choices[0].Message.Content,
		Role:           domain.MessageRoleAgent,
		Metadata: map[string]interface{}{
			"response_type":     "mistral",
			"user_message_id":   userMessage.ID.String(),
			"model":            "mistral-small",
			"tokens_used":      response.Usage.TotalTokens,
		},
		CreatedAt: time.Now(),
	}

	// Save agent response
	if err := s.messageRepo.Create(ctx, agentResponse); err != nil {
		return nil, fmt.Errorf("failed to create agent response: %w", err)
	}

	// Log agent response activity
	activity := &domain.AgentActivityLog{
		ID:             uuid.New(),
		OrganizationID: conversation.OrganizationID,
		AgentID:        userMessage.AgentID,
		UserID:         &userMessage.UserID,
		ConversationID: &userMessage.ConversationID,
		ActivityType:   domain.ActivityTypeAgentResponseGenerated,
		ActivityData: map[string]interface{}{
			"message_id":        agentResponse.ID.String(),
			"user_message_id":   userMessage.ID.String(),
			"conversation_id":   userMessage.ConversationID.String(),
			"response_type":     "mistral",
			"model":            "mistral-small",
			"tokens_used":      response.Usage.TotalTokens,
		},
		CreatedAt: time.Now(),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		fmt.Printf("Failed to log agent response activity: %v\n", err)
	}

	return agentResponse, nil
}

// generateSimulatedResponse provides a fallback response when Mistral is not available
func (s *ChatService) generateSimulatedResponse(ctx context.Context, userMessage *domain.ChatMessage, conversation *domain.ChatConversation) (*domain.ChatMessage, error) {
	agentResponse := &domain.ChatMessage{
		ID:             uuid.New(),
		ConversationID: userMessage.ConversationID,
		UserID:         userMessage.UserID,
		AgentID:        userMessage.AgentID,
		MessageType:    domain.MessageTypeText,
		Content:        fmt.Sprintf("I received your message: \"%s\". This is a simulated agent response. Please configure MISTRAL_API_KEY environment variable for real AI responses.", userMessage.Content),
		Role:           domain.MessageRoleAgent,
		Metadata: map[string]interface{}{
			"response_type": "simulated",
			"user_message_id": userMessage.ID.String(),
		},
		CreatedAt: time.Now(),
	}

	// Save agent response
	if err := s.messageRepo.Create(ctx, agentResponse); err != nil {
		return nil, fmt.Errorf("failed to create agent response: %w", err)
	}

	// Log agent response activity
	activity := &domain.AgentActivityLog{
		ID:             uuid.New(),
		OrganizationID: conversation.OrganizationID,
		AgentID:        userMessage.AgentID,
		UserID:         &userMessage.UserID,
		ConversationID: &userMessage.ConversationID,
		ActivityType:   domain.ActivityTypeAgentResponseGenerated,
		ActivityData: map[string]interface{}{
			"message_id":        agentResponse.ID.String(),
			"user_message_id":   userMessage.ID.String(),
			"conversation_id":   userMessage.ConversationID.String(),
			"response_type":     "simulated",
		},
		CreatedAt: time.Now(),
	}

	if err := s.activityRepo.Create(ctx, activity); err != nil {
		fmt.Printf("Failed to log agent response activity: %v\n", err)
	}

	return agentResponse, nil
}

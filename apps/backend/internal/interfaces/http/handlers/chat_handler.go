package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
)

// ChatHandler handles chat-related HTTP requests
type ChatHandler struct {
	chatService *application.ChatService
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatService *application.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

// RegisterRoutes registers chat routes
func (h *ChatHandler) RegisterRoutes(app fiber.Router, authMiddleware fiber.Handler) {
	api := app.Group("/chat")
	api.Use(authMiddleware)

	// Conversation endpoints
	api.Post("/conversations", h.CreateConversation)
	api.Get("/conversations", h.GetConversations)
	api.Get("/conversations/:id", h.GetConversation)
	api.Put("/conversations/:id", h.UpdateConversation)
	api.Delete("/conversations/:id", h.DeleteConversation)

	// Message endpoints
	api.Post("/messages", h.SendMessage)
	api.Get("/conversations/:id/messages", h.GetMessages)
	api.Put("/messages/:id", h.UpdateMessage)
	api.Delete("/messages/:id", h.DeleteMessage)

	// Activity and limits endpoints
	api.Get("/activity/agent/:id", h.GetAgentActivity)
	api.Get("/activity/user/:id", h.GetUserActivity)
	api.Get("/limits/daily", h.GetDailyLimits)
	api.Get("/stats", h.GetChatStats)
}

// CreateConversation creates a new chat conversation
// @Summary Create conversation
// @Description Create a new chat conversation between user and agent
// @Tags chat
// @Accept json
// @Produce json
// @Param request body application.CreateConversationRequest true "Conversation details"
// @Success 201 {object} domain.ChatConversation
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations [post]
func (h *ChatHandler) CreateConversation(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req application.CreateConversationRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Set user ID from context
	req.UserID = userID

	conversation, err := h.chatService.CreateConversation(c.Context(), &req, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(conversation)
}

// SendMessage sends a message in a conversation
// @Summary Send message
// @Description Send a message in a conversation
// @Tags chat
// @Accept json
// @Produce json
// @Param request body application.SendMessageRequest true "Message details"
// @Success 201 {object} application.ChatResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/messages [post]
func (h *ChatHandler) SendMessage(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req application.SendMessageRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Set user ID from context
	req.UserID = userID

	// Get IP address and user agent
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	response, err := h.chatService.SendMessage(c.Context(), &req, orgID, ipAddress, userAgent)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// GetMessages retrieves messages for a conversation
// @Summary Get messages
// @Description Get paginated list of messages for a conversation
// @Tags chat
// @Produce json
// @Param id path string true "Conversation ID"
// @Param limit query int false "Number of messages to return" default(50)
// @Param offset query int false "Number of messages to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations/{id}/messages [get]
func (h *ChatHandler) GetMessages(c fiber.Ctx) error {
	conversationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	messages, err := h.chatService.GetMessages(c.Context(), conversationID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch messages",
		})
	}

	return c.JSON(fiber.Map{
		"messages": messages,
		"total":    len(messages),
		"limit":    limit,
		"offset":   offset,
	})
}

// GetConversations retrieves conversations for the authenticated user
// @Summary Get conversations
// @Description Get paginated list of conversations for the authenticated user
// @Tags chat
// @Produce json
// @Param limit query int false "Number of conversations to return" default(20)
// @Param offset query int false "Number of conversations to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations [get]
func (h *ChatHandler) GetConversations(c fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	conversations, err := h.chatService.GetConversations(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch conversations",
		})
	}

	return c.JSON(fiber.Map{
		"conversations": conversations,
		"total":         len(conversations),
		"limit":         limit,
		"offset":        offset,
	})
}

// GetConversation retrieves a specific conversation
// @Summary Get conversation
// @Description Get a specific conversation by ID
// @Tags chat
// @Produce json
// @Param id path string true "Conversation ID"
// @Success 200 {object} domain.ChatConversation
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations/{id} [get]
func (h *ChatHandler) GetConversation(c fiber.Ctx) error {
	conversationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID",
		})
	}

	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"id":    conversationID,
		"error": "Not implemented yet",
	})
}

// UpdateConversation updates a conversation
// @Summary Update conversation
// @Description Update conversation details
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Conversation ID"
// @Success 200 {object} domain.ChatConversation
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations/{id} [put]
func (h *ChatHandler) UpdateConversation(c fiber.Ctx) error {
	conversationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID",
		})
	}

	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"id":    conversationID,
		"error": "Not implemented yet",
	})
}

// DeleteConversation deletes a conversation
// @Summary Delete conversation
// @Description Delete a conversation and all related messages
// @Tags chat
// @Param id path string true "Conversation ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/conversations/{id} [delete]
func (h *ChatHandler) DeleteConversation(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	
	conversationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid conversation ID",
		})
	}

	err = h.chatService.DeleteConversation(c.Context(), conversationID, userID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete conversation",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateMessage updates a message
// @Summary Update message
// @Description Update message content
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "Message ID"
// @Success 200 {object} domain.ChatMessage
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/messages/{id} [put]
func (h *ChatHandler) UpdateMessage(c fiber.Ctx) error {
	messageID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid message ID",
		})
	}

	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"id":    messageID,
		"error": "Not implemented yet",
	})
}

// DeleteMessage deletes a message
// @Summary Delete message
// @Description Delete a message
// @Tags chat
// @Param id path string true "Message ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/messages/{id} [delete]
func (h *ChatHandler) DeleteMessage(c fiber.Ctx) error {
	messageID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid message ID",
		})
	}

	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.Status(fiber.StatusNoContent).JSON(fiber.Map{
		"id":    messageID,
		"error": "Not implemented yet",
	})
}

// GetAgentActivity retrieves activity logs for an agent
// @Summary Get agent activity
// @Description Get activity logs for a specific agent
// @Tags chat
// @Produce json
// @Param id path string true "Agent ID"
// @Param limit query int false "Number of activities to return" default(50)
// @Param offset query int false "Number of activities to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/activity/agent/{id} [get]
func (h *ChatHandler) GetAgentActivity(c fiber.Ctx) error {
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	activities, err := h.chatService.GetAgentActivity(c.Context(), agentID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agent activity",
		})
	}

	return c.JSON(fiber.Map{
		"activities": activities,
		"total":      len(activities),
		"limit":      limit,
		"offset":     offset,
	})
}

// GetUserActivity retrieves activity logs for a user
// @Summary Get user activity
// @Description Get activity logs for a specific user
// @Tags chat
// @Produce json
// @Param id path string true "User ID"
// @Param limit query int false "Number of activities to return" default(50)
// @Param offset query int false "Number of activities to skip" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/activity/user/{id} [get]
func (h *ChatHandler) GetUserActivity(c fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"user_id": userID,
		"error":   "Not implemented yet",
	})
}

// GetDailyLimits retrieves daily message limits for the authenticated user
// @Summary Get daily limits
// @Description Get daily message limits and usage for the authenticated user
// @Tags chat
// @Produce json
// @Success 200 {object} application.UserDailyLimitInfo
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/limits/daily [get]
func (h *ChatHandler) GetDailyLimits(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	limits, err := h.chatService.GetUserDailyLimit(c.Context(), userID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch daily limits",
		})
	}

	return c.JSON(limits)
}

// GetChatStats retrieves chat statistics
// @Summary Get chat stats
// @Description Get chat statistics for the organization
// @Tags chat
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/chat/stats [get]
func (h *ChatHandler) GetChatStats(c fiber.Ctx) error {
	// This would need to be implemented in the service layer
	// For now, return a placeholder response
	return c.JSON(fiber.Map{
		"error": "Not implemented yet",
	})
}

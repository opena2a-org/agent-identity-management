package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// TagHandler handles HTTP requests for tag management
type TagHandler struct {
	tagService *application.TagService
}

// NewTagHandler creates a new tag handler instance
func NewTagHandler(tagService *application.TagService) *TagHandler {
	return &TagHandler{
		tagService: tagService,
	}
}

// CreateTagRequest represents the request body for creating a tag
type CreateTagRequest struct {
	Key         string `json:"key" validate:"required,max=100"`
	Value       string `json:"value" validate:"required,max=255"`
	Category    string `json:"category" validate:"required"`
	Description string `json:"description"`
	Color       string `json:"color" validate:"omitempty,len=7"`
}

// AddTagsRequest represents the request body for adding tags to an asset
type AddTagsRequest struct {
	TagIDs []string `json:"tag_ids" validate:"required,min=1,max=3,dive,uuid"`
}

// CreateTag godoc
// @Summary Create a new tag
// @Description Create a new tag for the authenticated user's organization
// @Tags tags
// @Accept json
// @Produce json
// @Param tag body CreateTagRequest true "Tag details"
// @Success 201 {object} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags [post]
func (h *TagHandler) CreateTag(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found",
		})
	}

	// Parse request body
	var req CreateTagRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Create tag
	tag, err := h.tagService.CreateTag(c.Context(), application.CreateTagInput{
		OrganizationID: orgID,
		Key:            req.Key,
		Value:          req.Value,
		Category:       domain.TagCategory(req.Category),
		Description:    req.Description,
		Color:          req.Color,
		CreatedBy:      userID,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(tag)
}

// GetTags godoc
// @Summary Get all tags for organization
// @Description Retrieve all tags for the authenticated user's organization
// @Tags tags
// @Produce json
// @Param category query string false "Filter by category"
// @Success 200 {array} domain.Tag
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags [get]
func (h *TagHandler) GetTags(c fiber.Ctx) error {
	// Get authenticated user's organization
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found",
		})
	}

	// Check if filtering by category
	categoryParam := c.Query("category")
	var categoryFilter *domain.TagCategory
	if categoryParam != "" {
		cat := domain.TagCategory(categoryParam)
		categoryFilter = &cat
	}

	tags, err := h.tagService.GetTagsByOrganization(c.Context(), orgID, categoryFilter)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tags)
}

// UpdateTagRequest represents the request body for updating a tag
type UpdateTagRequest struct {
	Key         string `json:"key" validate:"omitempty,max=100"`
	Value       string `json:"value" validate:"omitempty,max=255"`
	Category    string `json:"category" validate:"omitempty"`
	Description string `json:"description"`
	Color       string `json:"color" validate:"omitempty,len=7"`
}

// UpdateTag godoc
// @Summary Update a tag
// @Description Update an existing tag
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "Tag ID"
// @Param tag body UpdateTagRequest true "Tag details to update"
// @Success 200 {object} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags/{id} [put]
func (h *TagHandler) UpdateTag(c fiber.Ctx) error {
	// Parse tag ID
	tagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid tag ID",
		})
	}

	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found",
		})
	}

	// Parse request body
	var req UpdateTagRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Update tag
	tag, err := h.tagService.UpdateTag(c.Context(), tagID, orgID, application.UpdateTagInput{
		Key:         req.Key,
		Value:       req.Value,
		Category:    req.Category,
		Description: req.Description,
		Color:       req.Color,
		UpdatedBy:   userID,
	})
	if err != nil {
		if contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
				Error: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tag)
}

// DeleteTag godoc
// @Summary Delete a tag
// @Description Delete a tag (only if not in use)
// @Tags tags
// @Param id path string true "Tag ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags/{id} [delete]
func (h *TagHandler) DeleteTag(c fiber.Ctx) error {
	// Parse tag ID
	tagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid tag ID",
		})
	}

	// Delete tag
	if err := h.tagService.DeleteTag(c.Context(), tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// AddTagsToAgent godoc
// @Summary Add tags to an agent
// @Description Add one or more tags to an agent (Community Edition: max 3 tags)
// @Tags agents, tags
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param tags body AddTagsRequest true "Tag IDs to add"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/agents/{id}/tags [post]
func (h *TagHandler) AddTagsToAgent(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Parse request body
	var req AddTagsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Convert string IDs to UUIDs
	tagIDs := make([]uuid.UUID, len(req.TagIDs))
	for i, idStr := range req.TagIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "Invalid tag ID format",
			})
		}
		tagIDs[i] = id
	}

	// Add tags to agent
	if err := h.tagService.AddTagsToAgent(c.Context(), agentID, tagIDs, userID); err != nil {
		// Check if it's a Community Edition limit error
		if contains(err.Error(), "Community Edition limited to 3 tags") {
			return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{
				Error: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveTagFromAgent godoc
// @Summary Remove a tag from an agent
// @Description Remove a specific tag from an agent
// @Tags agents, tags
// @Param id path string true "Agent ID"
// @Param tagId path string true "Tag ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/agents/{id}/tags/{tagId} [delete]
func (h *TagHandler) RemoveTagFromAgent(c fiber.Ctx) error {
	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Parse tag ID
	tagID, err := uuid.Parse(c.Params("tagId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid tag ID",
		})
	}

	// Remove tag from agent
	if err := h.tagService.RemoveTagFromAgent(c.Context(), agentID, tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetAgentTags godoc
// @Summary Get agent tags
// @Description Retrieve all tags for a specific agent
// @Tags agents, tags
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/agents/{id}/tags [get]
func (h *TagHandler) GetAgentTags(c fiber.Ctx) error {
	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Get agent tags
	tags, err := h.tagService.GetAgentTags(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tags)
}

// SuggestTagsForAgent godoc
// @Summary Suggest tags for an agent
// @Description Get tag suggestions based on agent capabilities
// @Tags agents, tags
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/agents/{id}/tags/suggestions [get]
func (h *TagHandler) SuggestTagsForAgent(c fiber.Ctx) error {
	// Parse agent ID
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid agent ID",
		})
	}

	// Get tag suggestions
	suggestions, err := h.tagService.SuggestTagsForAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(suggestions)
}

// AddTagsToMCPServer godoc
// @Summary Add tags to an MCP server
// @Description Add one or more tags to an MCP server (Community Edition: max 3 tags)
// @Tags mcp-servers, tags
// @Accept json
// @Produce json
// @Param id path string true "MCP Server ID"
// @Param tags body AddTagsRequest true "Tag IDs to add"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/mcp-servers/{id}/tags [post]
func (h *TagHandler) AddTagsToMCPServer(c fiber.Ctx) error {
	// Get authenticated user
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Unauthorized",
		})
	}

	// Parse MCP server ID
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid MCP server ID",
		})
	}

	// Parse request body
	var req AddTagsRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid request body",
		})
	}

	// Convert string IDs to UUIDs
	tagIDs := make([]uuid.UUID, len(req.TagIDs))
	for i, idStr := range req.TagIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "Invalid tag ID format",
			})
		}
		tagIDs[i] = id
	}

	// Add tags to MCP server
	if err := h.tagService.AddTagsToMCPServer(c.Context(), mcpServerID, tagIDs, userID); err != nil {
		// Check if it's a Community Edition limit error
		if contains(err.Error(), "Community Edition limited to 3 tags") {
			return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{
				Error: err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveTagFromMCPServer godoc
// @Summary Remove a tag from an MCP server
// @Description Remove a specific tag from an MCP server
// @Tags mcp-servers, tags
// @Param id path string true "MCP Server ID"
// @Param tagId path string true "Tag ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/mcp-servers/{id}/tags/{tagId} [delete]
func (h *TagHandler) RemoveTagFromMCPServer(c fiber.Ctx) error {
	// Parse MCP server ID
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid MCP server ID",
		})
	}

	// Parse tag ID
	tagID, err := uuid.Parse(c.Params("tagId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid tag ID",
		})
	}

	// Remove tag from MCP server
	if err := h.tagService.RemoveTagFromMCPServer(c.Context(), mcpServerID, tagID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetMCPServerTags godoc
// @Summary Get MCP server tags
// @Description Retrieve all tags for a specific MCP server
// @Tags mcp-servers, tags
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/mcp-servers/{id}/tags [get]
func (h *TagHandler) GetMCPServerTags(c fiber.Ctx) error {
	// Parse MCP server ID
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid MCP server ID",
		})
	}

	// Get MCP server tags
	tags, err := h.tagService.GetMCPServerTags(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tags)
}

// SuggestTagsForMCPServer godoc
// @Summary Suggest tags for an MCP server
// @Description Get tag suggestions based on MCP server capabilities
// @Tags mcp-servers, tags
// @Produce json
// @Param id path string true "MCP Server ID"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/mcp-servers/{id}/tags/suggestions [get]
func (h *TagHandler) SuggestTagsForMCPServer(c fiber.Ctx) error {
	// Parse MCP server ID
	mcpServerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Invalid MCP server ID",
		})
	}

	// Get tag suggestions
	suggestions, err := h.tagService.SuggestTagsForMCPServer(c.Context(), mcpServerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(suggestions)
}

// GetPopularTags returns the most popular tags by usage count
// @Summary Get popular tags
// @Description Get most popular tags ordered by usage count
// @Tags tags
// @Produce json
// @Param limit query int false "Maximum number of tags to return (default: 10, max: 50)"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags/popular [get]
func (h *TagHandler) GetPopularTags(c fiber.Ctx) error {
	// Get authenticated user's organization
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found",
		})
	}

	// Parse limit parameter with validation
	limit := 10 // default
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			if parsedLimit > 0 && parsedLimit <= 50 {
				limit = parsedLimit
			} else if parsedLimit > 50 {
				limit = 50
			}
		}
	}

	tags, err := h.tagService.GetPopularTags(c.Context(), orgID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tags)
}

// SearchTags searches for tags by query string
// @Summary Search tags
// @Description Search tags by name (case-insensitive)
// @Tags tags
// @Produce json
// @Param q query string true "Search query"
// @Param category query string false "Filter by category"
// @Success 200 {array} domain.Tag
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/tags/search [get]
func (h *TagHandler) SearchTags(c fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "query parameter 'q' is required",
		})
	}

	// Get authenticated user's organization
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error: "Organization ID not found",
		})
	}

	// Optional category filter
	categoryFilter := c.Query("category", "")

	tags, err := h.tagService.SearchTags(c.Context(), orgID, query, categoryFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: err.Error(),
		})
	}

	return c.JSON(tags)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

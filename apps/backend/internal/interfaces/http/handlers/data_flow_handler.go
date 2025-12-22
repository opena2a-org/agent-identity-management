package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// DataFlowHandler handles data flow visibility HTTP requests
type DataFlowHandler struct {
	service *application.DataFlowService
}

// NewDataFlowHandler creates a new DataFlowHandler
func NewDataFlowHandler(service *application.DataFlowService) *DataFlowHandler {
	return &DataFlowHandler{service: service}
}

// ============================================================================
// DATA FLOWS
// ============================================================================

// RecordDataFlowRequest is the API request to record a data flow
type RecordDataFlowRequest struct {
	AgentID             string                 `json:"agentId"`
	AgentName           string                 `json:"agentName"`
	SourceType          string                 `json:"sourceType"`
	SourceName          string                 `json:"sourceName"`
	SourceIdentifier    string                 `json:"sourceIdentifier,omitempty"`
	SourceDetails       map[string]interface{} `json:"sourceDetails,omitempty"`
	DatabaseType        string                 `json:"databaseType,omitempty"`
	SchemaName          string                 `json:"schemaName,omitempty"`
	TableName           string                 `json:"tableName,omitempty"`
	ColumnName          string                 `json:"columnName,omitempty"`
	DestinationType     string                 `json:"destinationType"`
	DestinationName     string                 `json:"destinationName"`
	DestinationEndpoint string                 `json:"destinationEndpoint,omitempty"`
	DestinationDetails  map[string]interface{} `json:"destinationDetails,omitempty"`
	DataSizeBytes       int64                  `json:"dataSizeBytes,omitempty"`
	RecordCount         int                    `json:"recordCount,omitempty"`
	FieldCount          int                    `json:"fieldCount,omitempty"`
	FieldsDetected      []string               `json:"fieldsDetected,omitempty"`
	RequestID           string                 `json:"requestId,omitempty"`
	SessionID           string                 `json:"sessionId,omitempty"`
	UserID              string                 `json:"userId,omitempty"`
	VerificationEventID string                 `json:"verificationEventId,omitempty"`
	Capability          string                 `json:"capability,omitempty"`
	FlowID              string                 `json:"flowId,omitempty"`
	ParentFlowID        string                 `json:"parentFlowId,omitempty"`
	HopNumber           int                    `json:"hopNumber,omitempty"`
	Classification      *ClassificationInput   `json:"classification,omitempty"` // Explicit Tier 1
}

// ClassificationInput is explicit classification from SDK (Tier 1)
type ClassificationInput struct {
	Category string `json:"category"` // PII, PHI, PCI
	Type     string `json:"type"`     // ssn, credit_card, etc.
}

// RecordDataFlow handles POST /api/v1/data-flows
// @Summary Record a data flow event
// @Description Records a data flow event from the SDK and evaluates policies
// @Tags Data Flow
// @Accept json
// @Produce json
// @Param request body RecordDataFlowRequest true "Data flow details"
// @Success 200 {object} application.RecordDataFlowResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows [post]
func (h *DataFlowHandler) RecordDataFlow(c fiber.Ctx) error {
	var req RecordDataFlowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.AgentID == "" || req.SourceType == "" || req.DestinationType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agentId, sourceType, and destinationType are required",
		})
	}

	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agentId format",
		})
	}

	// Build service request
	svcReq := &application.RecordDataFlowRequest{
		OrganizationID:      orgID,
		AgentID:             agentID,
		AgentName:           req.AgentName,
		SourceType:          domain.DataSourceType(req.SourceType),
		SourceName:          req.SourceName,
		SourceIdentifier:    req.SourceIdentifier,
		SourceDetails:       req.SourceDetails,
		DatabaseType:        req.DatabaseType,
		SchemaName:          req.SchemaName,
		TableName:           req.TableName,
		ColumnName:          req.ColumnName,
		DestinationType:     domain.DataFlowDestinationType(req.DestinationType),
		DestinationName:     req.DestinationName,
		DestinationEndpoint: req.DestinationEndpoint,
		DestinationDetails:  req.DestinationDetails,
		DataSizeBytes:       req.DataSizeBytes,
		RecordCount:         req.RecordCount,
		FieldCount:          req.FieldCount,
		FieldsDetected:      req.FieldsDetected,
		RequestID:           req.RequestID,
		SessionID:           req.SessionID,
		UserID:              req.UserID,
		Capability:          req.Capability,
		FlowID:              req.FlowID,
		ParentFlowID:        req.ParentFlowID,
		HopNumber:           req.HopNumber,
		ClientIP:            c.IP(),
	}

	// Handle optional verification event ID
	if req.VerificationEventID != "" {
		if veID, err := uuid.Parse(req.VerificationEventID); err == nil {
			svcReq.VerificationEventID = &veID
		}
	}

	// Handle explicit classification (Tier 1)
	if req.Classification != nil {
		svcReq.ExplicitClassification = &application.ExplicitClassification{
			Category:   req.Classification.Category,
			Type:       req.Classification.Type,
			Confidence: 1.0,
		}
	}

	// Record the flow
	resp, err := h.service.RecordDataFlow(c.Context(), svcReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(resp)
}

// CompleteDataFlowRequest is the request to complete a data flow
type CompleteDataFlowRequest struct {
	DurationMs int    `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

// CompleteDataFlow handles POST /api/v1/data-flows/:id/complete
// @Summary Complete a data flow
// @Description Marks a data flow as completed with timing information
// @Tags Data Flow
// @Accept json
// @Produce json
// @Param id path string true "Flow ID"
// @Param request body CompleteDataFlowRequest true "Completion details"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows/{id}/complete [post]
func (h *DataFlowHandler) CompleteDataFlow(c fiber.Ctx) error {
	flowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid flow ID",
		})
	}

	var req CompleteDataFlowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.service.CompleteDataFlow(c.Context(), flowID, req.DurationMs, req.Error); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// GetDataFlows handles GET /api/v1/data-flows
// @Summary List data flows
// @Description Retrieves data flows with pagination and filtering
// @Tags Data Flow
// @Accept json
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Param agentId query string false "Filter by agent ID"
// @Param category query string false "Filter by classification category"
// @Param type query string false "Filter by classification type"
// @Success 200 {object} DataFlowListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows [get]
func (h *DataFlowHandler) GetDataFlows(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	agentIDStr := c.Query("agentId")
	category := c.Query("category")
	dataType := c.Query("type")

	var flows []*domain.DataFlow
	var total int

	if agentIDStr != "" {
		agentID, parseErr := uuid.Parse(agentIDStr)
		if parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agentId format",
			})
		}
		flows, total, err = h.service.GetDataFlowsByAgent(c.Context(), agentID, limit, offset)
	} else if category != "" || dataType != "" {
		flows, total, err = h.service.GetDataFlowsByClassification(c.Context(), orgID, category, dataType, limit, offset)
	} else {
		flows, total, err = h.service.GetDataFlows(c.Context(), orgID, limit, offset)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"flows": flows,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}

// GetDataFlowSummary handles GET /api/v1/data-flows/summary
// @Summary Get data flow summary
// @Description Returns high-level statistics about data flows
// @Tags Data Flow
// @Accept json
// @Produce json
// @Success 200 {object} domain.DataFlowSummary
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows/summary [get]
func (h *DataFlowHandler) GetDataFlowSummary(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	summary, err := h.service.GetSummary(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(summary)
}

// GetRecentFlows handles GET /api/v1/data-flows/recent
// @Summary Get recent data flows
// @Description Returns data flows from the last N minutes
// @Tags Data Flow
// @Accept json
// @Produce json
// @Param minutes query int false "Minutes" default(15)
// @Success 200 {object} DataFlowListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows/recent [get]
func (h *DataFlowHandler) GetRecentFlows(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	minutes, _ := strconv.Atoi(c.Query("minutes", "15"))
	if minutes > 60 {
		minutes = 60 // Cap at 1 hour
	}

	flows, err := h.service.GetRecentFlows(c.Context(), orgID, minutes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"flows":   flows,
		"minutes": minutes,
	})
}

// GetFlowChain handles GET /api/v1/data-flows/chain/:flowId
// @Summary Get flow chain
// @Description Returns all flows in a chain by flow ID
// @Tags Data Flow
// @Accept json
// @Produce json
// @Param flowId path string true "Flow chain ID"
// @Success 200 {object} DataFlowListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flows/chain/{flowId} [get]
func (h *DataFlowHandler) GetFlowChain(c fiber.Ctx) error {
	flowID := c.Params("flowId")
	if flowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "flowId is required",
		})
	}

	flows, err := h.service.GetFlowChain(c.Context(), flowID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"flows":  flows,
		"flowId": flowID,
	})
}

// ============================================================================
// DATA SOURCES
// ============================================================================

// GetDataSources handles GET /api/v1/data-sources
// @Summary List data sources
// @Description Retrieves discovered data sources with their classifications
// @Tags Data Sources
// @Accept json
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} DataSourceListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-sources [get]
func (h *DataFlowHandler) GetDataSources(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	sources, total, err := h.service.GetDataSources(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"sources": sources,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// UpdateSourceClassificationRequest is the request to update a source's classification
type UpdateSourceClassificationRequest struct {
	ClassificationTypeID string `json:"classificationTypeId"`
}

// UpdateSourceClassification handles PUT /api/v1/data-sources/:id/classification
// @Summary Update source classification
// @Description Manually updates a data source's classification (Tier 1)
// @Tags Data Sources
// @Accept json
// @Produce json
// @Param id path string true "Source ID"
// @Param request body UpdateSourceClassificationRequest true "New classification"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-sources/{id}/classification [put]
func (h *DataFlowHandler) UpdateSourceClassification(c fiber.Ctx) error {
	sourceID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid source ID",
		})
	}

	var req UpdateSourceClassificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	typeID, err := uuid.Parse(req.ClassificationTypeID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid classificationTypeId",
		})
	}

	userID := getUserID(c)
	if err := h.service.UpdateSourceClassification(c.Context(), sourceID, typeID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// ============================================================================
// POLICIES
// ============================================================================

// GetPolicies handles GET /api/v1/data-flow-policies
// @Summary List data flow policies
// @Description Retrieves all data flow policies for the organization
// @Tags Data Flow Policies
// @Accept json
// @Produce json
// @Success 200 {object} PolicyListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-policies [get]
func (h *DataFlowHandler) GetPolicies(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	policies, err := h.service.GetPolicies(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"policies": policies,
	})
}

// DataFlowPolicyRequest is the request to create/update a data flow policy
type DataFlowPolicyRequest struct {
	Name                     string   `json:"name"`
	Description              string   `json:"description,omitempty"`
	Priority                 int      `json:"priority"`
	ClassificationCategories []string `json:"classificationCategories,omitempty"`
	ClassificationTypes      []string `json:"classificationTypes,omitempty"`
	MinClassificationTier    *int     `json:"minClassificationTier,omitempty"`
	SourceTypes              []string `json:"sourceTypes,omitempty"`
	DestinationTypes         []string `json:"destinationTypes,omitempty"`
	BlockedDestinations      []string `json:"blockedDestinations,omitempty"`
	AllowedDestinations      []string `json:"allowedDestinations,omitempty"`
	Action                   string   `json:"action"`
	AlertSeverity            string   `json:"alertSeverity,omitempty"`
	AlertMessage             string   `json:"alertMessage,omitempty"`
	IsEnabled                bool     `json:"isEnabled"`
}

// CreatePolicy handles POST /api/v1/data-flow-policies
// @Summary Create a data flow policy
// @Description Creates a new data flow policy
// @Tags Data Flow Policies
// @Accept json
// @Produce json
// @Param request body DataFlowPolicyRequest true "Policy details"
// @Success 201 {object} domain.DataFlowPolicy
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-policies [post]
func (h *DataFlowHandler) CreatePolicy(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	var req DataFlowPolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" || req.Action == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name and action are required",
		})
	}

	userID := getUserID(c)
	policy := &domain.DataFlowPolicy{
		OrganizationID:           orgID,
		Name:                     req.Name,
		Description:              req.Description,
		Priority:                 req.Priority,
		ClassificationCategories: req.ClassificationCategories,
		ClassificationTypes:      req.ClassificationTypes,
		MinClassificationTier:    req.MinClassificationTier,
		SourceTypes:              req.SourceTypes,
		DestinationTypes:         req.DestinationTypes,
		BlockedDestinations:      req.BlockedDestinations,
		AllowedDestinations:      req.AllowedDestinations,
		Action:                   domain.PolicyAction(req.Action),
		AlertSeverity:            req.AlertSeverity,
		AlertMessage:             req.AlertMessage,
		IsEnabled:                req.IsEnabled,
		CreatedBy:                &userID,
	}

	if err := h.service.CreatePolicy(c.Context(), policy); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(policy)
}

// UpdatePolicy handles PUT /api/v1/data-flow-policies/:id
// @Summary Update a data flow policy
// @Description Updates an existing data flow policy
// @Tags Data Flow Policies
// @Accept json
// @Produce json
// @Param id path string true "Policy ID"
// @Param request body DataFlowPolicyRequest true "Policy details"
// @Success 200 {object} domain.DataFlowPolicy
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-policies/{id} [put]
func (h *DataFlowHandler) UpdatePolicy(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	var req DataFlowPolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	policy := &domain.DataFlowPolicy{
		ID:                       policyID,
		OrganizationID:           orgID,
		Name:                     req.Name,
		Description:              req.Description,
		Priority:                 req.Priority,
		ClassificationCategories: req.ClassificationCategories,
		ClassificationTypes:      req.ClassificationTypes,
		MinClassificationTier:    req.MinClassificationTier,
		SourceTypes:              req.SourceTypes,
		DestinationTypes:         req.DestinationTypes,
		BlockedDestinations:      req.BlockedDestinations,
		AllowedDestinations:      req.AllowedDestinations,
		Action:                   domain.PolicyAction(req.Action),
		AlertSeverity:            req.AlertSeverity,
		AlertMessage:             req.AlertMessage,
		IsEnabled:                req.IsEnabled,
	}

	if err := h.service.UpdatePolicy(c.Context(), policy); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(policy)
}

// DeletePolicy handles DELETE /api/v1/data-flow-policies/:id
// @Summary Delete a data flow policy
// @Description Deletes a data flow policy
// @Tags Data Flow Policies
// @Accept json
// @Produce json
// @Param id path string true "Policy ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-policies/{id} [delete]
func (h *DataFlowHandler) DeletePolicy(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	if err := h.service.DeletePolicy(c.Context(), policyID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// ============================================================================
// VIOLATIONS
// ============================================================================

// GetViolations handles GET /api/v1/data-flow-violations
// @Summary List data flow violations
// @Description Retrieves data flow policy violations
// @Tags Data Flow Violations
// @Accept json
// @Produce json
// @Param status query string false "Filter by status" Enums(open, acknowledged, resolved, false_positive)
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} ViolationListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-violations [get]
func (h *DataFlowHandler) GetViolations(c fiber.Ctx) error {
	orgID, err := getOrganizationID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	status := domain.ViolationStatus(c.Query("status"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	violations, total, err := h.service.GetViolations(c.Context(), orgID, status, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"violations": violations,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// ResolveViolationRequest is the request to resolve a violation
type ResolveViolationRequest struct {
	Status string `json:"status"` // resolved, false_positive, acknowledged
	Notes  string `json:"notes,omitempty"`
}

// ResolveViolation handles POST /api/v1/data-flow-violations/:id/resolve
// @Summary Resolve a violation
// @Description Resolves a data flow violation
// @Tags Data Flow Violations
// @Accept json
// @Produce json
// @Param id path string true "Violation ID"
// @Param request body ResolveViolationRequest true "Resolution details"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/data-flow-violations/{id}/resolve [post]
func (h *DataFlowHandler) ResolveViolation(c fiber.Ctx) error {
	violationID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid violation ID",
		})
	}

	var req ResolveViolationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	userID := getUserID(c)
	status := domain.ViolationStatus(req.Status)

	if err := h.service.ResolveViolation(c.Context(), violationID, status, userID, req.Notes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// ============================================================================
// CLASSIFICATION REFERENCE DATA
// ============================================================================

// GetClassificationCategories handles GET /api/v1/classifications/categories
// @Summary List classification categories
// @Description Retrieves all classification categories (PII, PHI, PCI, etc.)
// @Tags Classifications
// @Accept json
// @Produce json
// @Success 200 {object} CategoryListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/classifications/categories [get]
func (h *DataFlowHandler) GetClassificationCategories(c fiber.Ctx) error {
	categories, err := h.service.GetClassificationCategories(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"categories": categories,
	})
}

// GetClassificationTypes handles GET /api/v1/classifications/types
// @Summary List classification types
// @Description Retrieves classification types, optionally filtered by category
// @Tags Classifications
// @Accept json
// @Produce json
// @Param categoryId query string false "Filter by category ID"
// @Success 200 {object} TypeListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/classifications/types [get]
func (h *DataFlowHandler) GetClassificationTypes(c fiber.Ctx) error {
	var categoryID *uuid.UUID
	if catIDStr := c.Query("categoryId"); catIDStr != "" {
		if id, err := uuid.Parse(catIDStr); err == nil {
			categoryID = &id
		}
	}

	types, err := h.service.GetClassificationTypes(c.Context(), categoryID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"types": types,
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func getUserID(c fiber.Ctx) uuid.UUID {
	if userID, ok := c.Locals("userID").(uuid.UUID); ok {
		return userID
	}
	return uuid.Nil
}

// ============================================================================
// ROUTE REGISTRATION
// ============================================================================

// RegisterRoutes registers data flow visibility routes
func (h *DataFlowHandler) RegisterRoutes(api fiber.Router) {
	// Data Flows
	flows := api.Group("/data-flows")
	flows.Post("/", h.RecordDataFlow)
	flows.Get("/", h.GetDataFlows)
	flows.Get("/summary", h.GetDataFlowSummary)
	flows.Get("/recent", h.GetRecentFlows)
	flows.Get("/chain/:flowId", h.GetFlowChain)
	flows.Post("/:id/complete", h.CompleteDataFlow)

	// Data Sources
	sources := api.Group("/data-sources")
	sources.Get("/", h.GetDataSources)
	sources.Put("/:id/classification", h.UpdateSourceClassification)

	// Policies
	policies := api.Group("/data-flow-policies")
	policies.Get("/", h.GetPolicies)
	policies.Post("/", h.CreatePolicy)
	policies.Put("/:id", h.UpdatePolicy)
	policies.Delete("/:id", h.DeletePolicy)

	// Violations
	violations := api.Group("/data-flow-violations")
	violations.Get("/", h.GetViolations)
	violations.Post("/:id/resolve", h.ResolveViolation)

	// Classifications (reference data)
	classifications := api.Group("/classifications")
	classifications.Get("/categories", h.GetClassificationCategories)
	classifications.Get("/types", h.GetClassificationTypes)
}

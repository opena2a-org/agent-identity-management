package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AlertHandler struct {
	DB interface{} // Will be properly typed later
}

func NewAlertHandler(db interface{}) *AlertHandler {
	return &AlertHandler{DB: db}
}

// Alert represents a security or operational alert
type Alert struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organizationId"`
	AgentID         *uuid.UUID `json:"agentId,omitempty"`
	AlertType       string     `json:"alertType"`
	Severity        string     `json:"severity"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	AcknowledgedBy  *uuid.UUID `json:"acknowledgedBy,omitempty"`
	AcknowledgedAt  *time.Time `json:"acknowledgedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// ListAlerts returns paginated list of alerts for the organization
// GET /api/v1/alerts?status=active&severity=high&limit=50&offset=0
func (h *AlertHandler) ListAlerts(c fiber.Ctx) error {
	// Get organization from auth context
	orgID, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Parse query parameters
	status := c.Query("status", "")       // active, acknowledged, dismissed, resolved
	severity := c.Query("severity", "")   // low, medium, high, critical
	alertType := c.Query("type", "")      // suspicious_activity, trust_drop, failed_verification, etc.

	// Parse limit and offset as integers
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	if limit > 100 {
		limit = 100 // Maximum 100 alerts per page
	}

	// Build query
	query := `
		SELECT id, organization_id, agent_id, alert_type, severity, title, description,
		       status, metadata, acknowledged_by, acknowledged_at, created_at, updated_at
		FROM alerts
		WHERE organization_id = $1
	`
	args := []interface{}{orgID}
	argCount := 1

	if status != "" {
		argCount++
		query += " AND status = $" + string(rune(argCount+'0'))
		args = append(args, status)
	}
	if severity != "" {
		argCount++
		query += " AND severity = $" + string(rune(argCount+'0'))
		args = append(args, severity)
	}
	if alertType != "" {
		argCount++
		query += " AND alert_type = $" + string(rune(argCount+'0'))
		args = append(args, alertType)
	}

	query += " ORDER BY created_at DESC LIMIT $" + string(rune(argCount+'1'+'0')) + " OFFSET $" + string(rune(argCount+'2'+'0'))
	args = append(args, limit, offset)

	// Execute query (mock for now - will use real DB connection)
	alerts := []Alert{
		{
			ID:             uuid.New(),
			OrganizationID: orgID,
			AlertType:      "suspicious_activity",
			Severity:       "high",
			Title:          "Suspicious API usage detected",
			Description:    "Agent made 100+ API calls in 1 minute",
			Status:         "active",
			CreatedAt:      time.Now().Add(-2 * time.Hour),
			UpdatedAt:      time.Now().Add(-2 * time.Hour),
		},
	}

	// Count total alerts for pagination
	_ = "SELECT COUNT(*) FROM alerts WHERE organization_id = $1" // countQuery - will be used when implementing real DB
	total := 42 // Mock total

	return c.JSON(fiber.Map{
		"alerts": alerts,
		"pagination": fiber.Map{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetAlert returns details for a specific alert
// GET /api/v1/alerts/:id
func (h *AlertHandler) GetAlert(c fiber.Ctx) error {
	// Get organization from auth context
	orgID, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Parse alert ID from path
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	// Query alert (will be used when implementing real DB)
	_ = `
		SELECT id, organization_id, agent_id, alert_type, severity, title, description,
		       status, metadata, acknowledged_by, acknowledged_at, created_at, updated_at
		FROM alerts
		WHERE id = $1 AND organization_id = $2
	`

	// Mock response
	alert := Alert{
		ID:             alertID,
		OrganizationID: orgID,
		AlertType:      "trust_drop",
		Severity:       "medium",
		Title:          "Agent trust score dropped below 50%",
		Description:    "Trust score decreased from 75% to 45% in 24 hours",
		Status:         "active",
		Metadata: map[string]interface{}{
			"previous_score": 0.75,
			"current_score":  0.45,
			"factors_changed": []string{"update_frequency", "repository_quality"},
		},
		CreatedAt: time.Now().Add(-6 * time.Hour),
		UpdatedAt: time.Now().Add(-6 * time.Hour),
	}

	return c.JSON(alert)
}

// AcknowledgeAlert marks an alert as acknowledged
// POST /api/v1/alerts/:id/acknowledge
func (h *AlertHandler) AcknowledgeAlert(c fiber.Ctx) error {
	// Get user and organization from auth context
	userID, err := uuid.Parse(c.Locals("user_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	orgID, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Parse alert ID from path
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	// Update alert status (will be used when implementing real DB)
	_ = `
		UPDATE alerts
		SET status = 'acknowledged',
		    acknowledged_by = $1,
		    acknowledged_at = $2,
		    updated_at = $2
		WHERE id = $3 AND organization_id = $4 AND status = 'active'
		RETURNING id, organization_id, agent_id, alert_type, severity, title, description,
		          status, metadata, acknowledged_by, acknowledged_at, created_at, updated_at
	`

	now := time.Now()

	// Mock response
	alert := Alert{
		ID:             alertID,
		OrganizationID: orgID,
		AlertType:      "trust_drop",
		Severity:       "medium",
		Title:          "Agent trust score dropped below 50%",
		Description:    "Trust score decreased from 75% to 45% in 24 hours",
		Status:         "acknowledged",
		AcknowledgedBy: &userID,
		AcknowledgedAt: &now,
		CreatedAt:      time.Now().Add(-6 * time.Hour),
		UpdatedAt:      now,
	}

	return c.JSON(alert)
}

// DismissAlert dismisses an alert (soft delete)
// DELETE /api/v1/alerts/:id
func (h *AlertHandler) DismissAlert(c fiber.Ctx) error {
	// Get organization from auth context
	_, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Parse alert ID from path
	alertID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid alert ID",
		})
	}

	// Update alert status to dismissed (will be used when implementing real DB)
	_ = `
		UPDATE alerts
		SET status = 'dismissed',
		    updated_at = $1
		WHERE id = $2 AND organization_id = $3
	`

	// Execute update (mock for now)
	now := time.Now()

	return c.JSON(fiber.Map{
		"message": "Alert dismissed successfully",
		"alert_id": alertID.String(),
		"dismissed_at": now,
	})
}

// GetAlertStats returns alert statistics for the organization
// GET /api/v1/alerts/stats
func (h *AlertHandler) GetAlertStats(c fiber.Ctx) error {
	// Get organization from auth context
	orgID, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Query alert statistics (will be used when implementing real DB)
	_ = `
		SELECT
		    status,
		    severity,
		    COUNT(*) as count
		FROM alerts
		WHERE organization_id = $1
		GROUP BY status, severity
	`

	// Mock statistics
	stats := fiber.Map{
		"organization_id": orgID.String(),
		"total_alerts": 156,
		"by_status": fiber.Map{
			"active":       42,
			"acknowledged": 78,
			"dismissed":    24,
			"resolved":     12,
		},
		"by_severity": fiber.Map{
			"critical": 8,
			"high":     28,
			"medium":   76,
			"low":      44,
		},
		"by_type": fiber.Map{
			"suspicious_activity":  18,
			"trust_drop":           34,
			"failed_verification":  12,
			"unusual_usage":        22,
			"security_audit_fail":  6,
			"other":                64,
		},
		"recent_24h": 12,
		"recent_7d":  48,
		"recent_30d": 156,
	}

	return c.JSON(stats)
}

// TestAlert generates a test alert (admin only)
// POST /api/v1/alerts/test
func (h *AlertHandler) TestAlert(c fiber.Ctx) error {
	// Get organization from auth context
	orgID, err := uuid.Parse(c.Locals("organization_id").(string))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid organization ID",
		})
	}

	// Parse request body (optional)
	type TestAlertRequest struct {
		AlertType   string                 `json:"alertType"`
		Severity    string                 `json:"severity"`
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	var req TestAlertRequest
	if err := c.Bind().Body(&req); err != nil {
		// Use defaults if parsing fails
		req.AlertType = "test_alert"
		req.Severity = "low"
		req.Title = "Test Alert"
		req.Description = "This is a test alert generated via API"
	}

	// Validate severity
	validSeverities := map[string]bool{
		"low": true, "medium": true, "high": true, "critical": true,
	}
	if !validSeverities[req.Severity] {
		req.Severity = "low"
	}

	// Create test alert
	now := time.Now()
	testAlert := Alert{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AlertType:      req.AlertType,
		Severity:       req.Severity,
		Title:          req.Title,
		Description:    req.Description,
		Status:         "active",
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Insert into database (will be used when implementing real DB)
	_ = `
		INSERT INTO alerts (id, organization_id, alert_type, severity, title, description, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Test alert created successfully",
		"alert":   testAlert,
	})
}

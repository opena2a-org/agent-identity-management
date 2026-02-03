package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type ComplianceHandler struct {
	// Concrete service pointers (used by existing code)
	complianceService *application.ComplianceService
	auditService      *application.AuditService

	// Interface fields for testability (used when set)
	complianceServicer ComplianceServicerExtended
	auditServicer      AuditServicer
}

func NewComplianceHandler(
	complianceService *application.ComplianceService,
	auditService *application.AuditService,
) *ComplianceHandler {
	return &ComplianceHandler{
		complianceService: complianceService,
		auditService:      auditService,
	}
}

// NewComplianceHandlerWithInterfaces creates a ComplianceHandler using interfaces for testability
func NewComplianceHandlerWithInterfaces(
	complianceService ComplianceServicerExtended,
	auditService AuditServicer,
) *ComplianceHandler {
	return &ComplianceHandler{
		complianceServicer: complianceService,
		auditServicer:      auditService,
	}
}

// Helper methods to use interfaces when available, otherwise use concrete types
func (h *ComplianceHandler) getComplianceService() ComplianceServicerExtended {
	if h.complianceServicer != nil {
		return h.complianceServicer
	}
	return h.complianceService
}

func (h *ComplianceHandler) getAuditService() AuditServicer {
	if h.auditServicer != nil {
		return h.auditServicer
	}
	return h.auditService
}

// GetComplianceStatus returns current compliance status
func (h *ComplianceHandler) GetComplianceStatus(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	compSvc := h.getComplianceService()
	status, err := compSvc.GetComplianceStatus(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance status",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_status",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(status)
}

// GetComplianceMetrics returns compliance metrics over time
func (h *ComplianceHandler) GetComplianceMetrics(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	// Parse time range
	var req struct {
		StartDate string `query:"start_date"`
		EndDate   string `query:"end_date"`
		Interval  string `query:"interval"` // "day", "week", "month"
	}

	if err := c.Bind().Query(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	// Default to last 30 days if not specified
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if req.StartDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.StartDate)
		if err == nil {
			startDate = parsed
		}
	}

	if req.EndDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.EndDate)
		if err == nil {
			endDate = parsed
		}
	}

	// Default interval
	if req.Interval == "" {
		req.Interval = "day"
	}

	// Get metrics
	compSvc := h.getComplianceService()
	metrics, err := compSvc.GetComplianceMetrics(
		c.Context(),
		orgID,
		startDate,
		endDate,
		req.Interval,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance metrics",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_metrics",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"startDate": startDate,
			"endDate":   endDate,
			"interval":   req.Interval,
		},
	)

	return c.JSON(fiber.Map{
		"metrics":    metrics,
		"startDate": startDate,
		"endDate":   endDate,
		"interval":   req.Interval,
	})
}

// GetAccessReview returns list of user access for review
func (h *ComplianceHandler) GetAccessReview(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	compSvc := h.getComplianceService()
	review, err := compSvc.GetAccessReview(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate access review",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"access_review",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(review)
}

// RunComplianceCheck runs compliance checks
func (h *ComplianceHandler) RunComplianceCheck(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		CheckType string `json:"checkType"` // "soc2", "iso27001", "hipaa", "gdpr", "all"
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Default to all checks
	if req.CheckType == "" {
		req.CheckType = "all"
	}

	// Run compliance checks
	compSvc := h.getComplianceService()
	results, err := compSvc.RunComplianceCheck(
		c.Context(),
		orgID,
		req.CheckType,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to run compliance check",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCheck,
		"compliance",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"checkType": req.CheckType,
		},
	)

	return c.JSON(results)
}

// ExportComplianceReport exports compliance report in specified format
// @Summary Export compliance report
// @Description Export comprehensive compliance report in CSV or JSON format
// @Tags compliance
// @Produce text/csv,application/json
// @Param format query string false "Export format (csv or json)" default(csv)
// @Param framework query string false "Compliance framework (soc2 or hipaa)" default(soc2)
// @Param start_date query string false "Start date for report (RFC3339)"
// @Param end_date query string false "End date for report (RFC3339)"
// @Success 200 {file} file
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/compliance/export [get]
func (h *ComplianceHandler) ExportComplianceReport(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	format := c.Query("format", "csv")
	if format != "csv" && format != "json" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid format. Supported formats: csv, json",
		})
	}

	framework := c.Query("framework", "aim")
	if framework != "aim" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Only 'aim' is supported",
		})
	}

	// Parse date range (optional)
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30) // Default to last 30 days

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		parsed, err := time.Parse(time.RFC3339, startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		parsed, err := time.Parse(time.RFC3339, endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionExport,
		"compliance_report",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"format":    format,
			"framework": framework,
			"startDate": startDate,
			"endDate":   endDate,
		},
	)

	compSvc := h.getComplianceService()
	if format == "json" {
		// Full JSON export with all compliance data
		report, err := compSvc.ExportComplianceReportFull(
			c.Context(),
			orgID,
			framework,
			startDate,
			endDate,
			userID,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate compliance report",
			})
		}

		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", "attachment; filename=compliance-report.json")
		return c.JSON(report)
	}

	// CSV format
	csvData, err := compSvc.ExportToCSV(c.Context(), orgID, framework)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate CSV report",
		})
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=compliance-report.csv")
	return c.SendString(csvData)
}

// GetComplianceTrending returns compliance score history
// @Summary Get compliance score trending
// @Description Get historical compliance scores over time for a framework
// @Tags compliance
// @Produce json
// @Param framework query string false "Compliance framework (soc2 or hipaa)" default(soc2)
// @Param start_date query string false "Start date (RFC3339)"
// @Param end_date query string false "End date (RFC3339)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/trending [get]
func (h *ComplianceHandler) GetComplianceTrending(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	framework := c.Query("framework", "aim")
	if framework != "aim" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Only 'aim' is supported",
		})
	}

	// Parse date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30) // Default to last 30 days

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		parsed, err := time.Parse(time.RFC3339, startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		parsed, err := time.Parse(time.RFC3339, endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	compSvc := h.getComplianceService()
	trending, err := compSvc.GetComplianceTrending(
		c.Context(),
		orgID,
		framework,
		startDate,
		endDate,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance trending",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_trending",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": framework,
			"startDate": startDate,
			"endDate":   endDate,
		},
	)

	return c.JSON(trending)
}

// RecordComplianceSnapshot records a point-in-time compliance score
// @Summary Record compliance snapshot
// @Description Record a snapshot of current compliance scores for trending
// @Tags compliance
// @Accept json
// @Produce json
// @Param body body object true "Snapshot request"
// @Success 200 {object} domain.ComplianceSnapshot
// @Router /api/v1/compliance/snapshot [post]
func (h *ComplianceHandler) RecordComplianceSnapshot(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		Framework string `json:"framework"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Framework == "" {
		req.Framework = "aim"
	}

	if req.Framework != "aim" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Only 'aim' is supported",
		})
	}

	compSvc := h.getComplianceService()
	snapshot, err := compSvc.RecordComplianceSnapshot(
		c.Context(),
		orgID,
		domain.ComplianceFramework(req.Framework),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to record compliance snapshot",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"compliance_snapshot",
		snapshot.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": req.Framework,
			"score":     snapshot.Score,
		},
	)

	return c.JSON(snapshot)
}

// ListEvidence returns compliance evidence for the organization
// @Summary List compliance evidence
// @Description Get a list of collected compliance evidence
// @Tags compliance
// @Produce json
// @Param framework query string false "Filter by framework (soc2 or hipaa)"
// @Param limit query int false "Limit results" default(50)
// @Param offset query int false "Offset results" default(0)
// @Success 200 {array} domain.ComplianceEvidence
// @Router /api/v1/compliance/evidence [get]
func (h *ComplianceHandler) ListEvidence(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	framework := c.Query("framework", "")

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePagination(c)

	compSvc := h.getComplianceService()
	evidence, err := compSvc.ListEvidence(
		c.Context(),
		orgID,
		framework,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance evidence",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_evidence",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": framework,
			"limit":     p.Limit,
			"offset":    p.Offset,
		},
	)

	return c.JSON(fiber.Map{
		"evidence": evidence,
		"count":    len(evidence),
	})
}

// CollectEvidence collects evidence for a specific compliance check
// @Summary Collect compliance evidence
// @Description Automatically collect evidence for a compliance check
// @Tags compliance
// @Accept json
// @Produce json
// @Param body body object true "Evidence collection request"
// @Success 200 {object} domain.ComplianceEvidence
// @Router /api/v1/compliance/evidence/collect [post]
func (h *ComplianceHandler) CollectEvidence(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		Framework string `json:"framework"`
		CheckName string `json:"checkName"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Framework == "" || req.CheckName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Framework and checkName are required",
		})
	}

	if req.Framework != "aim" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Only 'aim' is supported",
		})
	}

	compSvc := h.getComplianceService()
	evidence, err := compSvc.CollectEvidence(
		c.Context(),
		orgID,
		domain.ComplianceFramework(req.Framework),
		req.CheckName,
		userID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to collect evidence: " + err.Error(),
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"compliance_evidence",
		evidence.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": req.Framework,
			"checkName": req.CheckName,
		},
	)

	return c.JSON(evidence)
}

// GetEvidenceForCheck returns evidence for a specific compliance check
// @Summary Get evidence for check
// @Description Get collected evidence for a specific compliance check
// @Tags compliance
// @Produce json
// @Param checkName path string true "Check name"
// @Success 200 {array} domain.ComplianceEvidence
// @Router /api/v1/compliance/evidence/check/{checkName} [get]
func (h *ComplianceHandler) GetEvidenceForCheck(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	checkName := c.Params("checkName")
	if checkName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Check name is required",
		})
	}

	compSvc := h.getComplianceService()
	evidence, err := compSvc.GetEvidenceForCheck(
		c.Context(),
		orgID,
		checkName,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch evidence",
		})
	}

	// Log audit
	auditSvc := h.getAuditService()
	auditSvc.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_evidence",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"checkName": checkName,
		},
	)

	return c.JSON(fiber.Map{
		"checkName": checkName,
		"evidence":  evidence,
		"count":     len(evidence),
	})
}

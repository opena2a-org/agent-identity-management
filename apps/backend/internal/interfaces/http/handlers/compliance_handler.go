package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type ComplianceHandler struct {
	complianceService *application.ComplianceService
	auditService      *application.AuditService
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

// GenerateComplianceReport generates a compliance report
func (h *ComplianceHandler) GenerateComplianceReport(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		ReportType string `json:"report_type"` // "soc2", "iso27001", "hipaa", "gdpr", "full"
		StartDate  string `json:"start_date"`
		EndDate    string `json:"end_date"`
		Format     string `json:"format"` // "json", "pdf", "csv"
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate report type
	validTypes := map[string]bool{
		"soc2":     true,
		"iso27001": true,
		"hipaa":    true,
		"gdpr":     true,
		"full":     true,
	}
	if !validTypes[req.ReportType] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid report type. Must be: soc2, iso27001, hipaa, gdpr, or full",
		})
	}

	// Parse dates
	startDate, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid start_date format. Use RFC3339",
		})
	}

	endDate, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid end_date format. Use RFC3339",
		})
	}

	// Validate date range
	if endDate.Before(startDate) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "end_date must be after start_date",
		})
	}

	// Set default format
	if req.Format == "" {
		req.Format = "json"
	}

	// Generate report
	report, err := h.complianceService.GenerateComplianceReport(
		c.Context(),
		orgID,
		req.ReportType,
		startDate,
		endDate,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate compliance report",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionGenerate,
		"compliance_report",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"report_type": req.ReportType,
			"start_date":  req.StartDate,
			"end_date":    req.EndDate,
			"format":      req.Format,
		},
	)

	// Return based on format
	switch req.Format {
	case "json":
		return c.JSON(report)
	case "pdf":
		// TODO: Implement PDF generation
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "PDF format not yet implemented",
		})
	case "csv":
		// TODO: Implement CSV export
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": "CSV format not yet implemented",
		})
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid format. Must be: json, pdf, or csv",
		})
	}
}

// GetComplianceStatus returns current compliance status
func (h *ComplianceHandler) GetComplianceStatus(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	status, err := h.complianceService.GetComplianceStatus(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance status",
		})
	}

	// Log audit
	h.auditService.LogAction(
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
	metrics, err := h.complianceService.GetComplianceMetrics(
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
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_metrics",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"start_date": startDate,
			"end_date":   endDate,
			"interval":   req.Interval,
		},
	)

	return c.JSON(fiber.Map{
		"metrics":    metrics,
		"start_date": startDate,
		"end_date":   endDate,
		"interval":   req.Interval,
	})
}

// ExportAuditLog exports audit log for compliance
func (h *ComplianceHandler) ExportAuditLog(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		StartDate string `query:"start_date"`
		EndDate   string `query:"end_date"`
		Format    string `query:"format"` // "json", "csv"
	}

	if err := c.Bind().Query(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	// Parse dates with defaults (last 90 days if not provided)
	var startDate, endDate time.Time
	if req.StartDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.StartDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid start_date format. Use RFC3339",
			})
		}
		startDate = parsed
	} else {
		startDate = time.Now().AddDate(0, 0, -90) // Default: last 90 days
	}

	if req.EndDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.EndDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid end_date format. Use RFC3339",
			})
		}
		endDate = parsed
	} else {
		endDate = time.Now() // Default: now
	}

	// Default format
	if req.Format == "" {
		req.Format = "csv"
	}

	// Export audit log
	export, err := h.complianceService.ExportAuditLog(
		c.Context(),
		orgID,
		startDate,
		endDate,
		req.Format,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to export audit log",
		})
	}

	// Log this export action
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionExport,
		"audit_log",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
			"format":     req.Format,
		},
	)

	// Set appropriate headers based on format
	switch req.Format {
	case "json":
		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", "attachment; filename=audit-log.json")
		return c.SendString(export)
	case "csv":
		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=audit-log.csv")
		return c.SendString(export)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid format. Must be: json or csv",
		})
	}
}

// GetAccessReview returns list of user access for review
func (h *ComplianceHandler) GetAccessReview(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	review, err := h.complianceService.GetAccessReview(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate access review",
		})
	}

	// Log audit
	h.auditService.LogAction(
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

// GetDataRetention returns data retention policy and status
func (h *ComplianceHandler) GetDataRetention(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	retention, err := h.complianceService.GetDataRetentionStatus(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch data retention status",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"data_retention",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(retention)
}

// RunComplianceCheck runs compliance checks
func (h *ComplianceHandler) RunComplianceCheck(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		CheckType string `json:"check_type"` // "soc2", "iso27001", "hipaa", "gdpr", "all"
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
	results, err := h.complianceService.RunComplianceCheck(
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
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCheck,
		"compliance",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"check_type": req.CheckType,
		},
	)

	return c.JSON(results)
}

// GetComplianceFrameworks lists all supported compliance frameworks
// @Summary List compliance frameworks
// @Description Get all supported compliance frameworks (SOC2, HIPAA, GDPR, ISO27001)
// @Tags compliance
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/frameworks [get]
func (h *ComplianceHandler) GetComplianceFrameworks(c fiber.Ctx) error {
	frameworks := []map[string]interface{}{
		{
			"id":          "soc2",
			"name":        "SOC 2",
			"description": "Service Organization Control 2 - Trust Services Criteria",
			"categories":  []string{"Security", "Availability", "Processing Integrity", "Confidentiality", "Privacy"},
		},
		{
			"id":          "hipaa",
			"name":        "HIPAA",
			"description": "Health Insurance Portability and Accountability Act",
			"categories":  []string{"Administrative Safeguards", "Physical Safeguards", "Technical Safeguards"},
		},
		{
			"id":          "gdpr",
			"name":        "GDPR",
			"description": "General Data Protection Regulation",
			"categories":  []string{"Lawfulness", "Data Minimization", "Accuracy", "Storage Limitation", "Security"},
		},
		{
			"id":          "iso27001",
			"name":        "ISO 27001",
			"description": "Information Security Management System",
			"categories":  []string{"Access Control", "Cryptography", "Physical Security", "Operations Security"},
		},
	}

	return c.JSON(fiber.Map{
		"frameworks": frameworks,
		"total":      len(frameworks),
	})
}

// GetComplianceReportByFramework generates a compliance report for a specific framework
// @Summary Get compliance report for framework
// @Description Generate a compliance report for a specific framework
// @Tags compliance
// @Produce json
// @Param framework path string true "Framework ID (soc2, hipaa, gdpr, iso27001)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/compliance/reports/{framework} [get]
func (h *ComplianceHandler) GetComplianceReportByFramework(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	framework := c.Params("framework")

	// Validate framework
	validFrameworks := map[string]bool{
		"soc2":     true,
		"hipaa":    true,
		"gdpr":     true,
		"iso27001": true,
	}

	if !validFrameworks[framework] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Must be: soc2, hipaa, gdpr, or iso27001",
		})
	}

	// Generate report for the specific framework
	report, err := h.complianceService.GenerateComplianceReport(
		c.Context(),
		orgID,
		framework,
		time.Now().AddDate(0, -1, 0), // Last month
		time.Now(),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate compliance report",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionGenerate,
		"compliance_report",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": framework,
		},
	)

	return c.JSON(report)
}

// RunComplianceScanByFramework runs a compliance scan for a specific framework
// @Summary Run compliance scan for framework
// @Description Run a compliance scan for a specific framework
// @Tags compliance
// @Produce json
// @Param framework path string true "Framework ID (soc2, hipaa, gdpr, iso27001)"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/compliance/scan/{framework} [post]
func (h *ComplianceHandler) RunComplianceScanByFramework(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	framework := c.Params("framework")

	// Validate framework
	validFrameworks := map[string]bool{
		"soc2":     true,
		"hipaa":    true,
		"gdpr":     true,
		"iso27001": true,
	}

	if !validFrameworks[framework] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid framework. Must be: soc2, hipaa, gdpr, or iso27001",
		})
	}

	// Run compliance check for the specific framework
	results, err := h.complianceService.RunComplianceCheck(
		c.Context(),
		orgID,
		framework,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to run compliance scan",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCheck,
		"compliance_scan",
		orgID, // Use orgID for collection operations
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"framework": framework,
		},
	)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message":   "Compliance scan initiated",
		"framework": framework,
		"results":   results,
	})
}

// GetComplianceViolations lists all compliance violations
// @Summary List compliance violations
// @Description Get all compliance violations for the organization
// @Tags compliance
// @Produce json
// @Param framework query string false "Filter by framework"
// @Param severity query string false "Filter by severity (low, medium, high, critical)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/violations [get]
func (h *ComplianceHandler) GetComplianceViolations(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	frameworkFilter := c.Query("framework", "")
	severityFilter := c.Query("severity", "")

	violations, err := h.complianceService.GetComplianceViolations(
		c.Context(),
		orgID,
		frameworkFilter,
		severityFilter,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch compliance violations",
		})
	}

	return c.JSON(fiber.Map{
		"violations": violations,
		"total":      len(violations),
		"filters": map[string]string{
			"framework": frameworkFilter,
			"severity":  severityFilter,
		},
	})
}

// RemediateViolation marks a compliance violation as remediated
// @Summary Remediate compliance violation
// @Description Mark a compliance violation as remediated
// @Tags compliance
// @Accept json
// @Produce json
// @Param violation_id path string true "Violation ID"
// @Param request body map[string]string true "Remediation details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/compliance/remediate/{violation_id} [post]
func (h *ComplianceHandler) RemediateViolation(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	violationID, err := uuid.Parse(c.Params("violation_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid violation ID",
		})
	}

	var req struct {
		RemediationNotes string `json:"remediation_notes"`
		RemediationDate  string `json:"remediation_date"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse remediation date if provided
	var remediationDate time.Time
	if req.RemediationDate != "" {
		remediationDate, err = time.Parse(time.RFC3339, req.RemediationDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid remediation_date format. Use RFC3339",
			})
		}
	} else {
		remediationDate = time.Now()
	}

	// Remediate violation
	if err := h.complianceService.RemediateViolation(
		c.Context(),
		violationID,
		userID,
		req.RemediationNotes,
		remediationDate,
	); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"compliance_violation",
		violationID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":            "remediate",
			"remediation_notes": req.RemediationNotes,
		},
	)

	return c.JSON(fiber.Map{
		"message":      "Violation remediated successfully",
		"violation_id": violationID,
		"remediated_at": remediationDate,
	})
}

// ListComplianceReports returns a list of compliance reports
// @Summary List compliance reports
// @Description Get all compliance reports for the organization
// @Tags compliance
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/reports [get]
func (h *ComplianceHandler) ListComplianceReports(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	reports, err := h.complianceService.ListComplianceReports(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve compliance reports",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"compliance_reports",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(fiber.Map{
		"reports": reports,
		"total":   len(reports),
	})
}

// ListAccessReviews returns a list of access review reports
// @Summary List access reviews
// @Description Get all access review reports with optional status filtering
// @Tags compliance
// @Produce json
// @Param status query string false "Filter by status (pending, approved, rejected)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/access-reviews [get]
func (h *ComplianceHandler) ListAccessReviews(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	statusFilter := c.Query("status", "")

	reviews, err := h.complianceService.ListAccessReviews(c.Context(), orgID, statusFilter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve access reviews",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"access_reviews",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"status_filter": statusFilter,
		},
	)

	return c.JSON(fiber.Map{
		"reviews": reviews,
		"total":   len(reviews),
		"filter": map[string]string{
			"status": statusFilter,
		},
	})
}

// GetDataRetentionPolicies returns data retention policies
// @Summary Get data retention policies
// @Description Get data retention policies for the organization
// @Tags compliance
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/compliance/data-retention [get]
func (h *ComplianceHandler) GetDataRetentionPolicies(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	policies, err := h.complianceService.GetDataRetentionPolicies(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve data retention policies",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"data_retention_policies",
		orgID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(policies)
}

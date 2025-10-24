package middleware

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AnalyticsTracking middleware tracks API calls for real-time analytics
func AnalyticsTracking(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Record start time
		start := time.Now()

		// Get request details before processing
		method := c.Method()
		endpoint := c.Path()
		requestSize := len(c.Body())

		// Process the request
		err := c.Next()

		// Calculate response time
		duration := time.Since(start)
		durationMs := int(duration.Milliseconds())

		// Get response details
		statusCode := c.Response().StatusCode()
		responseSize := len(c.Response().Body())

		// Get organization and agent IDs from context (if authenticated)
		var orgID, agentID, userID *uuid.UUID

		if orgIDValue := c.Locals("organization_id"); orgIDValue != nil {
			if id, ok := orgIDValue.(uuid.UUID); ok {
				orgID = &id
			}
		}

		if agentIDValue := c.Locals("agent_id"); agentIDValue != nil {
			if id, ok := agentIDValue.(uuid.UUID); ok {
				agentID = &id
			}
		}

		if userIDValue := c.Locals("user_id"); userIDValue != nil {
			if id, ok := userIDValue.(uuid.UUID); ok {
				userID = &id
			}
		}

		// Get user agent and IP
		userAgent := c.Get("User-Agent")
		ipAddress := c.IP()

		// Get error message if request failed
		var errorMessage *string
		if statusCode >= 400 {
			errMsg := string(c.Response().Body())
			if errMsg != "" && len(errMsg) < 1000 { // Limit error message size
				errorMessage = &errMsg
			}
		}

		// Log API call asynchronously to avoid blocking
		go logAPICall(db, APICallLog{
			OrganizationID:    orgID,
			AgentID:           agentID,
			UserID:            userID,
			Method:            method,
			Endpoint:          endpoint,
			StatusCode:        statusCode,
			DurationMs:        durationMs,
			RequestSizeBytes:  requestSize,
			ResponseSizeBytes: responseSize,
			UserAgent:         userAgent,
			IPAddress:         ipAddress,
			ErrorMessage:      errorMessage,
		})

		return err
	}
}

// APICallLog represents an API call record
type APICallLog struct {
	OrganizationID    *uuid.UUID
	AgentID           *uuid.UUID
	UserID            *uuid.UUID
	Method            string
	Endpoint          string
	StatusCode        int
	DurationMs        int
	RequestSizeBytes  int
	ResponseSizeBytes int
	UserAgent         string
	IPAddress         string
	ErrorMessage      *string
}

// logAPICall inserts API call record into database
func logAPICall(db *sql.DB, log APICallLog) {
	// Skip logging for health check endpoints to reduce noise
	if log.Endpoint == "/health" || log.Endpoint == "/api/v1/status" {
		return
	}

	// Skip logging if no organization ID (public endpoints)
	if log.OrganizationID == nil {
		return
	}

	query := `
		INSERT INTO api_calls (
			organization_id,
			agent_id,
			user_id,
			method,
			endpoint,
			status_code,
			duration_ms,
			request_size_bytes,
			response_size_bytes,
			user_agent,
			ip_address,
			error_message,
			called_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
	`

	_, err := db.Exec(
		query,
		log.OrganizationID,
		log.AgentID,
		log.UserID,
		log.Method,
		log.Endpoint,
		log.StatusCode,
		log.DurationMs,
		log.RequestSizeBytes,
		log.ResponseSizeBytes,
		log.UserAgent,
		log.IPAddress,
		log.ErrorMessage,
	)

	if err != nil {
		// Log error but don't fail the request
		// In production, you might want to use a proper logging framework
		// log.Printf("Failed to log API call: %v", err)
	}
}

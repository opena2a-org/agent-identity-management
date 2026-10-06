package middleware

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AnalyticsTracking middleware tracks API calls for real-time analytics
func AnalyticsTracking(db *sql.DB) fiber.Handler {
	if db != nil {
		startInsertFailureReporting.Do(func() {
			go apiCallInsertFailures.run(time.NewTicker(insertFailureReportPeriod).C)
		})
	}
	return func(c fiber.Ctx) error {
		// Record start time
		start := time.Now()

		// Get request details before processing. Fiber's request strings are views
		// onto pooled buffers that are valid only until this handler returns
		// (Immutable is off), and the record below is written from a goroutine
		// after that: copy every string it keeps, or a later request that reuses
		// the context rewrites the endpoint and user agent this row stores.
		method := strings.Clone(c.Method())
		endpoint := RedactBootstrapTokens(strings.Clone(c.Path()))
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
		userAgent := strings.Clone(c.Get("User-Agent"))
		ipAddress := strings.Clone(c.IP())

		// Get error message if request failed
		var errorMessage *string
		if statusCode >= 400 {
			errMsg := string(c.Response().Body())
			if errMsg != "" && len(errMsg) < 1000 { // Limit error message size
				errorMessage = &errMsg
			}
		}

		// Log API call asynchronously to avoid blocking
		go recordAPICall(db, APICallLog{
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

// recordAPICall is what the middleware's goroutine calls; tests replace it to
// observe the record the goroutine receives.
var recordAPICall = logAPICall

// logAPICall inserts API call record into database
func logAPICall(db *sql.DB, log APICallLog) {
	// No database, nothing to record. Analytics is observability, never a request gate, so
	// this returns quietly rather than failing the caller's request.
	//
	// This guard used to be provided by accident: the org-less early return below shielded
	// every nil-db path, because a nil db only ever arrived in tests that also sent no
	// organization. Recording org-less requests removed that accident and turned it into a
	// nil-pointer panic in the request goroutine — caught by
	// TestAnalyticsTracking_CapturesRequestDetails. Make the guard explicit rather than
	// depend on another condition happening to cover it.
	if db == nil {
		return
	}

	// Skip logging for health check endpoints to reduce noise
	if log.Endpoint == "/health" || log.Endpoint == "/api/v1/status" {
		return
	}

	// An org-less request IS recorded, with organization_id NULL.
	//
	// This used to return early, and `api_calls.organization_id` was NOT NULL, so between
	// them no request to an unauthenticated route was recorded — or could be. That is not
	// an absence of evidence, it is the absence of a place to put evidence, and it is the
	// difference between "we looked and found nothing" and "no record could exist".
	//
	// It cost us a real answer: the 2026 verification-event exposure ran on an
	// unauthenticated route, so the question "was it ever exploited" is permanently
	// unanswerable, while the same table held 69,029 rows for other endpoints across the
	// same window. Migration 107 makes the column nullable. Every unauthenticated route in
	// this service was in that position until it landed.
	//
	// Do not reinstate this early return. If org-less rows become a volume problem, sample
	// or retain them differently — but a route that cannot be seen is a route that cannot
	// be investigated.

	class := insertAPICall(db,
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
	if class != "" {
		// The request has already been answered; the loss is counted, never the row.
		apiCallInsertFailures.record(class)
	}
}

const apiCallInsertQuery = `
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

// insertAPICall writes one api_calls row and returns the class of the failure, or ""
// when the row was written.
//
// The connection is taken from the pool before the statement runs, so that a deadline
// spent waiting for a connection (pool) is told apart from one spent on the statement
// (timeout). A connection the driver reports broken is replaced once before the row is
// counted as lost.
func insertAPICall(db *sql.DB, args ...any) insertFailureClass {
	ctx, cancel := context.WithTimeout(context.Background(), apiCallInsertTimeout)
	defer cancel()

	for attempt := 0; ; attempt++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return insertFailurePool
			}
			return classifyInsertFailure(err)
		}
		_, err = conn.ExecContext(ctx, apiCallInsertQuery, args...)
		_ = conn.Close()
		switch {
		case err == nil:
			return ""
		case ctx.Err() != nil:
			return insertFailureTimeout
		case errors.Is(err, driver.ErrBadConn) && attempt == 0:
			continue
		default:
			return classifyInsertFailure(err)
		}
	}
}

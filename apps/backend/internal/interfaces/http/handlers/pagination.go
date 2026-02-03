package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// PaginationParams holds validated pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// DefaultPagination returns default pagination values
func DefaultPagination() PaginationParams {
	return PaginationParams{Limit: 50, Offset: 0}
}

// MaxOffset is the maximum allowed offset to prevent DB performance issues
// 10000 = 100 pages of 100 items, sufficient for most UI pagination
const MaxOffset = 10000

// ParsePagination extracts and validates pagination parameters from query string
// SECURITY: Prevents DoS attacks via unbounded limit/offset values
// - limit: clamped to 1-100 (default 50)
// - offset: clamped to 0-10000 (default 0)
func ParsePagination(c fiber.Ctx) PaginationParams {
	return ParsePaginationWithDefaults(c, 50, 100)
}

// ParsePaginationWithDefaults allows custom default and max limit
func ParsePaginationWithDefaults(c fiber.Ctx, defaultLimit, maxLimit int) PaginationParams {
	params := PaginationParams{
		Limit:  defaultLimit,
		Offset: 0,
	}

	// Parse limit with validation
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit >= 1 && limit <= maxLimit {
				params.Limit = limit
			} else if limit > maxLimit {
				params.Limit = maxLimit // Cap at maximum
			}
			// If limit < 1, keep default
		}
	}

	// Parse offset with validation
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			if offset > MaxOffset {
				params.Offset = MaxOffset // Cap at maximum to prevent DB performance issues
			} else {
				params.Offset = offset
			}
		}
		// If negative or error, keep default (0)
	}

	return params
}

// ParseDays extracts and validates a days parameter
// SECURITY: Prevents DoS via unbounded date range queries
// - Clamped to 1-365 (default provided by caller)
func ParseDays(c fiber.Ctx, paramName string, defaultDays, maxDays int) int {
	if daysStr := c.Query(paramName); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil {
			if days >= 1 && days <= maxDays {
				return days
			} else if days > maxDays {
				return maxDays // Cap at maximum
			}
		}
	}
	return defaultDays
}

// ParseWeeks extracts and validates a weeks parameter
// SECURITY: Prevents DoS via unbounded date range queries
// - Clamped to 1-52 (default provided by caller)
func ParseWeeks(c fiber.Ctx, paramName string, defaultWeeks, maxWeeks int) int {
	if weeksStr := c.Query(paramName); weeksStr != "" {
		if weeks, err := strconv.Atoi(weeksStr); err == nil {
			if weeks >= 1 && weeks <= maxWeeks {
				return weeks
			} else if weeks > maxWeeks {
				return maxWeeks
			}
		}
	}
	return defaultWeeks
}

// ParseMonths extracts and validates a months parameter
// SECURITY: Prevents DoS via unbounded date range queries
// - Clamped to 1-24 (default provided by caller)
func ParseMonths(c fiber.Ctx, paramName string, defaultMonths, maxMonths int) int {
	if monthsStr := c.Query(paramName); monthsStr != "" {
		if months, err := strconv.Atoi(monthsStr); err == nil {
			if months >= 1 && months <= maxMonths {
				return months
			} else if months > maxMonths {
				return maxMonths
			}
		}
	}
	return defaultMonths
}

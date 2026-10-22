package handlers

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /api/v1/agents/:id/alerts, GET /api/v1/agents/:id/violations,
// GET /api/v1/admin/alerts and GET /api/v1/a2a/security/violations read a
// limit below 1 as the page size they read when the request names no limit.
// The routes run over the database of list_routes_negative_offset_test.go,
// whose page query answers only the limit and offset it expects, so a limit
// below 1 that reaches the query fails the request.

// With no limit, or a limit below 1, the page query is sent the route's
// default page size, and the first page is answered 200.
func TestListRoutes_LimitBelowOneReadsAsDefaultPageSize(t *testing.T) {
	for _, route := range listOffsetRoutes {
		for _, query := range []string{
			"offset=0", "limit=0&offset=0", "limit=0", "limit=-1&offset=0", "limit=-100&offset=0",
		} {
			t.Run(route.name+" "+query, func(t *testing.T) {
				status, body, recordID := listOffsetGet(t, route, query, route.defaultLimit, 0)
				require.Equal(t, fiber.StatusOK, status, string(body))
				ids, echoed := route.page(t, body, route.defaultLimit)
				assert.Equal(t, []string{recordID}, ids, "the first page is served")
				if echoed != nil {
					assert.Equal(t, 0, *echoed)
				}
			})
		}
	}
}

// A limit of 1 or more reaches the page query as sent.
func TestListRoutes_PositiveLimitIsSentAsIs(t *testing.T) {
	for _, route := range listOffsetRoutes {
		for _, limit := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s limit=%d", route.name, limit), func(t *testing.T) {
				status, body, recordID := listOffsetGet(t, route, fmt.Sprintf("limit=%d&offset=0", limit), limit, 0)
				require.Equal(t, fiber.StatusOK, status, string(body))
				ids, _ := route.page(t, body, limit)
				assert.Equal(t, []string{recordID}, ids)
			})
		}
	}
}

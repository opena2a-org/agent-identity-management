package handlers

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /api/v1/agents/:id/alerts, GET /api/v1/agents/:id/violations,
// GET /api/v1/admin/alerts and GET /api/v1/a2a/security/violations read limit
// and offset through ParsePaginationWithDefaults: a limit above the route's
// largest page size is read as that size, and an offset above MaxOffset as
// MaxOffset. The routes run over the database of
// list_routes_negative_offset_test.go, whose page query answers only the limit
// and offset it expects, so a limit or offset that reaches the query unclamped
// fails the request.

// listRouteMaxLimit is the largest page size each route serves.
var listRouteMaxLimit = map[string]int{
	"GET /api/v1/agents/:id/alerts":       100,
	"GET /api/v1/agents/:id/violations":   100,
	"GET /api/v1/admin/alerts":            1000,
	"GET /api/v1/a2a/security/violations": 100,
}

// A limit above the route's largest page size reaches the page query as that
// size, and an offset above MaxOffset reaches it as MaxOffset.
func TestListRoutes_LimitAndOffsetAboveTheMaximumAreClamped(t *testing.T) {
	for _, route := range listOffsetRoutes {
		maxLimit, ok := listRouteMaxLimit[route.name]
		require.True(t, ok, "largest page size of %s", route.name)
		for _, query := range []string{
			"limit=1000000&offset=1000000000",
			fmt.Sprintf("limit=%d&offset=%d", maxLimit+1, MaxOffset+1),
			fmt.Sprintf("limit=%d&offset=%d", maxLimit, MaxOffset),
		} {
			t.Run(route.name+" "+query, func(t *testing.T) {
				status, body, recordID := listOffsetGet(t, route, query, maxLimit, MaxOffset)
				require.Equal(t, fiber.StatusOK, status, string(body))
				ids, echoed := route.page(t, body, maxLimit)
				assert.Equal(t, []string{recordID}, ids)
				if echoed != nil {
					assert.Equal(t, MaxOffset, *echoed, "the body echoes the offset read")
				}
			})
		}
	}
}

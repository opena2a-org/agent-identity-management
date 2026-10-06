package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// The chain-head route reads only the caller's organization: with no
// organization in the request it answers 401 and reads no chain (the handler
// has no database to read one from).
func TestAuditChainHeadNeedsTheCallersOrganization(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v1/admin/audit-logs/chain/head", NewAuditChainHandler(nil).GetChainHead)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/audit-logs/chain/head", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

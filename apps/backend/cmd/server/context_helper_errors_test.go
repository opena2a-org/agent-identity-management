package main

// A principal that passed authentication but carries no organization or user
// in context gets the context helper's 401 through the server's error
// handler, never a 500. Every handler call site returns the helper's error
// unchanged (the census in the handlers package holds that), so this is the
// status each of those routes answers with.

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
)

func TestContextHelperErrorsAnswer401ThroughServerErrorHandler(t *testing.T) {
	requireOrg := func(c fiber.Ctx) error {
		_, err := handlers.RequireOrganizationID(c)
		return err
	}
	requireUser := func(c fiber.Ctx) error {
		_, err := handlers.RequireUserID(c)
		return err
	}
	requireBoth := func(c fiber.Ctx) error {
		_, _, err := handlers.RequireOrgAndUserID(c)
		return err
	}

	cases := []struct {
		name        string
		orgID       bool
		userID      bool
		call        func(fiber.Ctx) error
		wantMessage string
	}{
		{name: "organization required, none in context", userID: true, call: requireOrg, wantMessage: "Organization ID not found in context"},
		{name: "user required, none in context", orgID: true, call: requireUser, wantMessage: "User ID not found in context"},
		{name: "both required, no organization", userID: true, call: requireBoth, wantMessage: "Organization ID not found in context"},
		{name: "both required, no user", orgID: true, call: requireBoth, wantMessage: "User ID not found in context"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
			app.Get("/probe", func(c fiber.Ctx) error {
				if tc.orgID {
					c.Locals("organization_id", uuid.New())
				}
				if tc.userID {
					c.Locals("user_id", uuid.New())
				}
				if err := tc.call(c); err != nil {
					return err
				}
				return c.SendStatus(fiber.StatusOK)
			})

			resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
			var body struct {
				Error   bool   `json:"error"`
				Message string `json:"message"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.True(t, body.Error)
			assert.Equal(t, tc.wantMessage, body.Message)
		})
	}
}

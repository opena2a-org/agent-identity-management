package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bindingTestRoute = "POST /agents/:id/heartbeat"

// bindingTestApp mounts AgentPathBinding the way production does: on the
// route, after a stand-in for the group's authenticator that sets the
// principal locals from the test's arguments.
func bindingTestApp(principal any, authMethod string, resolve AgentNameResolver) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if principal != nil {
			c.Locals("agent_id", principal)
			c.Locals("auth_method", authMethod)
		}
		return c.Next()
	})
	app.Post("/agents/:id/heartbeat", AgentPathBinding(bindingTestRoute, "id", resolve), func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusTeapot)
	})
	return app
}

type bindingResponse struct {
	status      int
	contentType string
	body        []byte
}

func callBinding(t *testing.T, app *fiber.App, path string) bindingResponse {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return bindingResponse{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type"), body: body}
}

func TestAgentPathBinding_OwnIDPasses(t *testing.T) {
	self := uuid.New()
	got := callBinding(t, bindingTestApp(self, "ed25519", nil), "/agents/"+self.String()+"/heartbeat")
	assert.Equal(t, http.StatusTeapot, got.status, "the agent's own id must reach the handler")
}

// A sibling agent's existing id and an id that names nothing must be
// indistinguishable: the refusal comes before any lookup.
func TestAgentPathBinding_ForeignAndUnknownIDsGetIdenticalRefusals(t *testing.T) {
	self, sibling, unknown := uuid.New(), uuid.New(), uuid.New()
	app := bindingTestApp(self, "ed25519", nil)

	siblingResp := callBinding(t, app, "/agents/"+sibling.String()+"/heartbeat")
	unknownResp := callBinding(t, app, "/agents/"+unknown.String()+"/heartbeat")
	notAUUID := callBinding(t, app, "/agents/not-a-uuid/heartbeat")

	assert.Equal(t, http.StatusForbidden, siblingResp.status)
	assert.Contains(t, string(siblingResp.body), `"reasonCode":"`+AgentPathMismatchReasonCode+`"`)
	assert.Equal(t, siblingResp, unknownResp, "an existing sibling id and an unknown id must get byte-identical responses")
	assert.Equal(t, siblingResp, notAUUID, "a parameter that is not a UUID gets the same refusal")
}

// Every authenticator that sets agent_id makes the caller an agent principal.
func TestAgentPathBinding_EveryAgentAuthenticatorIsBound(t *testing.T) {
	for _, method := range []string{"ed25519", "mldsa", "hybrid", "api_key", "atc", "service"} {
		t.Run(method, func(t *testing.T) {
			self, sibling := uuid.New(), uuid.New()
			app := bindingTestApp(self, method, nil)
			assert.Equal(t, http.StatusTeapot, callBinding(t, app, "/agents/"+self.String()+"/heartbeat").status)
			assert.Equal(t, http.StatusForbidden, callBinding(t, app, "/agents/"+sibling.String()+"/heartbeat").status)
		})
	}
}

// A user JWT sets no agent_id; the binding leaves that caller to the handler's
// own organization and role checks.
func TestAgentPathBinding_UserCallerUnchanged(t *testing.T) {
	got := callBinding(t, bindingTestApp(nil, "", nil), "/agents/"+uuid.New().String()+"/heartbeat")
	assert.Equal(t, http.StatusTeapot, got.status)
}

// Mounted with Use(), Fiber leaves Params empty. That must be a 500 naming the
// route for every caller, never a pass.
func TestAgentPathBinding_UseMountIsAServerFaultNamingTheRoute(t *testing.T) {
	self := uuid.New()
	for name, principal := range map[string]any{"agent principal": self, "user caller": nil} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				if principal != nil {
					c.Locals("agent_id", principal)
				}
				return c.Next()
			})
			group := app.Group("/agents")
			group.Use(AgentPathBinding(bindingTestRoute, "id", nil))
			group.Post("/:id/heartbeat", func(c fiber.Ctx) error { return c.SendStatus(http.StatusTeapot) })

			got := callBinding(t, app, "/agents/"+self.String()+"/heartbeat")
			assert.Equal(t, http.StatusInternalServerError, got.status)
			assert.Contains(t, string(got.body), bindingTestRoute)
		})
	}
}

func TestAgentPathBinding_UnreadablePrincipalIsAServerFault(t *testing.T) {
	for name, principal := range map[string]any{"string": uuid.New().String(), "nil uuid": uuid.Nil} {
		t.Run(name, func(t *testing.T) {
			got := callBinding(t, bindingTestApp(principal, "ed25519", nil), "/agents/"+uuid.New().String()+"/heartbeat")
			assert.Equal(t, http.StatusInternalServerError, got.status)
		})
	}
}

// countingResolver records every agent id the binding asks about.
type countingResolver struct {
	names map[uuid.UUID]string
	asked []uuid.UUID
	err   error
}

func (r *countingResolver) resolve(_ context.Context, id uuid.UUID) (string, error) {
	r.asked = append(r.asked, id)
	if r.err != nil {
		return "", r.err
	}
	return r.names[id], nil
}

func TestAgentPathBinding_NameResolverReadsOnlyThePrincipal(t *testing.T) {
	self, sibling := uuid.New(), uuid.New()
	resolver := &countingResolver{names: map[uuid.UUID]string{self: "self-agent", sibling: "sibling-agent"}}
	app := bindingTestApp(self, "ed25519", resolver.resolve)

	assert.Equal(t, http.StatusTeapot, callBinding(t, app, "/agents/self-agent/heartbeat").status,
		"the agent's own name must reach the handler")

	siblingName := callBinding(t, app, "/agents/sibling-agent/heartbeat")
	unknownName := callBinding(t, app, "/agents/no-such-agent/heartbeat")
	assert.Equal(t, http.StatusForbidden, siblingName.status)
	assert.Equal(t, siblingName, unknownName, "a sibling's name and an unknown name must get byte-identical responses")

	for _, asked := range resolver.asked {
		assert.Equal(t, self, asked, "the resolver may be asked about the caller only, never the agent the path names")
	}
	assert.Len(t, resolver.asked, 3)

	resolver.asked = nil
	assert.Equal(t, http.StatusTeapot, callBinding(t, app, "/agents/"+self.String()+"/heartbeat").status)
	assert.Equal(t, http.StatusForbidden, callBinding(t, app, "/agents/"+sibling.String()+"/heartbeat").status)
	assert.Empty(t, resolver.asked, "a UUID parameter is compared directly; no row is read")
}

func TestAgentPathBinding_NameResolverFailureIsAServerFault(t *testing.T) {
	resolver := &countingResolver{err: errors.New("database unavailable")}
	got := callBinding(t, bindingTestApp(uuid.New(), "ed25519", resolver.resolve), "/agents/some-name/heartbeat")
	assert.Equal(t, http.StatusInternalServerError, got.status)
}

func TestAgentPathBinding_RequiresRouteAndParam(t *testing.T) {
	assert.Panics(t, func() { AgentPathBinding("", "id", nil) })
	assert.Panics(t, func() { AgentPathBinding(bindingTestRoute, "", nil) })
}

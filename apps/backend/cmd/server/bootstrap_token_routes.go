package main

import "github.com/gofiber/fiber/v3"

// Onboarding bootstrap token routes, relative to /api/v1.
const (
	bootstrapTokenMintPath     = "/onboarding/bootstrap-tokens"
	bootstrapTokenRevokePath   = "/onboarding/bootstrap-tokens/revoke"
	bootstrapTokenExchangePath = "/onboarding/bootstrap-tokens/exchange"
)

// bootstrapTokenRouteDeps is the middleware and handlers the bootstrap token
// routes mount. Production passes the real ones; a test passes stand-ins and
// still gets the real paths and the real middleware order.
type bootstrapTokenRouteDeps struct {
	Authenticate  fiber.Handler // dashboard session (JWT)
	RequireMember fiber.Handler // member, manager or admin; viewers cannot mint
	MintLimit     fiber.Handler
	RevokeLimit   fiber.Handler
	ExchangeLimit fiber.Handler

	Mint     fiber.Handler
	Revoke   fiber.Handler
	Exchange fiber.Handler
}

// registerBootstrapTokenRoutes mounts the three routes with middleware
// attached per route.
//
// Mint and revoke run behind the dashboard session; the limiter follows
// authentication so it keys on the user rather than the address. Exchange is
// called by an SDK that holds only the token, so its sole middleware is the
// limiter, keyed on the client address, and the handler authenticates the
// token itself.
func registerBootstrapTokenRoutes(r fiber.Router, d bootstrapTokenRouteDeps) {
	r.Post(bootstrapTokenExchangePath, d.ExchangeLimit, d.Exchange)
	r.Post(bootstrapTokenRevokePath, d.Authenticate, d.RevokeLimit, d.RequireMember, d.Revoke)
	r.Post(bootstrapTokenMintPath, d.Authenticate, d.MintLimit, d.RequireMember, d.Mint)
}

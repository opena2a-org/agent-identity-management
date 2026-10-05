package middleware

import "github.com/gofiber/fiber/v3"

// DeferBearerRequests serves next only for a request without an Authorization header.
// PQCAgentMiddleware passes a request that carries one through unauthenticated, so on a
// route of an agent-signed group such a request has no organization in its context. It
// continues to the next route that matches its path instead, where the JWT group that
// registers the same path authenticates it.
func DeferBearerRequests(next fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get(fiber.HeaderAuthorization) != "" {
			return c.Next()
		}
		return next(c)
	}
}

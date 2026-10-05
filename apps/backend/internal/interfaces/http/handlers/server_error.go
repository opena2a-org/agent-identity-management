package handlers

import (
	"log"

	"github.com/gofiber/fiber/v3"
)

// ServerErrorMessage is the whole message of every 5xx answer a handler
// writes for an unexpected failure. The web client shows the same line for
// any 5xx it receives.
const ServerErrorMessage = "An internal error occurred. Please try again later."

// respondServerError answers status (a 5xx) with {"error": ServerErrorMessage}
// and logs err. The error stays on the server: a repository error names
// tables, constraints and query fragments, and none of that is the client's
// to read.
func respondServerError(c fiber.Ctx, status int, err error) error {
	log.Printf("server error [%d] %s %s: %v", status, c.Method(), c.Path(), err)
	return c.Status(status).JSON(fiber.Map{"error": ServerErrorMessage})
}

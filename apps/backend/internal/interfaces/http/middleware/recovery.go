package middleware

import (
	"fmt"
	"log"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			log.Printf("\n========== PANIC RECOVERED ==========\n")
			log.Printf("Error: %v\n", e)
			log.Printf("Path: %s\n", c.Path())
			log.Printf("Method: %s\n", c.Method())
			log.Printf("\nStack Trace:\n%s\n", debug.Stack())
			log.Printf("=====================================\n\n")

			// Also log to a more structured format
			c.Locals("panic_error", fmt.Sprintf("%v", e))
		},
	})
}

package trace

import (
	"github.com/gofiber/fiber/v3"
)

// traceparentHeader is the W3C Trace Context request header.
const traceparentHeader = "traceparent"

// Middleware gives each request its own trace: it mints a trace identifier
// and sets the request's context to one that carries it, so every record the
// request writes without a parent shares that trace.
//
// It reads the request's trace context itself, and reads only the trace-id
// field of one traceparent header. A request with no traceparent, more than
// one, or one ParseTraceparent refuses carries no request trace. It never
// applies a registered propagator to the request, and it sets no response
// header.
func Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		var traceparent string
		if values := c.RequestCtx().Request.Header.PeekAll(traceparentHeader); len(values) == 1 {
			traceparent = string(values[0])
		}
		tc, err := ForRequest(traceparent)
		if err != nil {
			return err
		}
		c.SetContext(With(c.Context(), tc))
		return c.Next()
	}
}

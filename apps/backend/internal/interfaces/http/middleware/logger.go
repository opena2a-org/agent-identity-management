package middleware

import (
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// bootstrapTokenPattern matches a bootstrap token wherever it appears: the
// prefix followed by its base64url secret.
var bootstrapTokenPattern = regexp.MustCompile(regexp.QuoteMeta(domain.BootstrapTokenPrefix) + `[A-Za-z0-9_-]*`)

// RedactBootstrapTokens replaces every bootstrap token in s with the prefix
// and a redaction marker. The exchange endpoint accepts the token only in a
// header or the JSON body, neither of which is logged; this covers a client
// that puts one in the URL path anyway.
func RedactBootstrapTokens(s string) string {
	if !strings.Contains(s, domain.BootstrapTokenPrefix) {
		return s
	}
	return bootstrapTokenPattern.ReplaceAllString(s, domain.BootstrapTokenPrefix+"[REDACTED]")
}

// LoggerMiddleware configures request logging
func LoggerMiddleware() fiber.Handler {
	return newLoggerMiddleware(os.Stdout)
}

// newLoggerMiddleware logs to out. The line carries the path only: no query
// string, no headers and no body, and the path is passed through
// RedactBootstrapTokens.
func newLoggerMiddleware(out io.Writer) fiber.Handler {
	return logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${redactedPath}\n",
		TimeFormat: time.RFC3339,
		TimeZone:   "UTC",
		Stream:     out,
		CustomTags: map[string]logger.LogFunc{
			"redactedPath": func(output logger.Buffer, c fiber.Ctx, _ *logger.Data, _ string) (int, error) {
				return output.WriteString(RedactBootstrapTokens(c.Path()))
			},
		},
	})
}

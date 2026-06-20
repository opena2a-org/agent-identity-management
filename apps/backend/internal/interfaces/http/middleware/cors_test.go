package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVercelPreviewOriginAllowed(t *testing.T) {
	allowed := []string{
		"https://aim-cloud-git-feat-x-opena2a.vercel.app",
		"https://aim-frontend-git-feat-x-opena2a.vercel.app",
		"https://aim-cloud-abc123-opena2a.vercel.app",
	}
	for _, o := range allowed {
		assert.True(t, vercelPreviewOriginAllowed(o), "should allow AIM preview: %s", o)
	}

	denied := []string{
		"https://evil.vercel.app",
		"https://aim-cloud-git-x-evil.vercel.app",     // wrong suffix
		"http://aim-cloud-git-x-opena2a.vercel.app",   // not https
		"https://aim-cloud-git-x-opena2a.vercel.app.evil.com", // suffix smuggling
		"https://notaim-cloud-opena2a.vercel.app",
		"https://aim-backend-git-x-opena2a.vercel.app", // not cloud/frontend
		"",
	}
	for _, o := range denied {
		assert.False(t, vercelPreviewOriginAllowed(o), "should deny: %s", o)
	}
}

func TestValidateAndSanitizeCORSOrigins_RejectsWildcard(t *testing.T) {
	origins := []string{"*"}
	result := ValidateAndSanitizeCORSOrigins(origins)

	// Should reject wildcard and default to localhost
	assert.Equal(t, []string{"http://localhost:3000"}, result)
}

func TestValidateAndSanitizeCORSOrigins_RejectsEmpty(t *testing.T) {
	origins := []string{"", "  ", ""}
	result := ValidateAndSanitizeCORSOrigins(origins)

	// Should reject empty origins and default to localhost
	assert.Equal(t, []string{"http://localhost:3000"}, result)
}

func TestValidateAndSanitizeCORSOrigins_RejectsInvalidFormat(t *testing.T) {
	origins := []string{
		"example.com",          // Missing protocol
		"ftp://example.com",    // Wrong protocol
		"javascript:void(0)",   // Invalid scheme
		"data:text/html,hello", // Data URL
	}
	result := ValidateAndSanitizeCORSOrigins(origins)

	// Should reject all invalid formats and default to localhost
	assert.Equal(t, []string{"http://localhost:3000"}, result)
}

func TestValidateAndSanitizeCORSOrigins_RejectsWildcardSubdomain(t *testing.T) {
	origins := []string{
		"https://*.example.com",
		"http://*.localhost",
		"https://api.*.example.com",
	}
	result := ValidateAndSanitizeCORSOrigins(origins)

	// Should reject wildcard subdomains and default to localhost
	assert.Equal(t, []string{"http://localhost:3000"}, result)
}

func TestValidateAndSanitizeCORSOrigins_AcceptsValidOrigins(t *testing.T) {
	origins := []string{
		"http://localhost:3000",
		"https://example.com",
		"https://api.example.com",
		"http://192.168.1.1:8080",
	}
	result := ValidateAndSanitizeCORSOrigins(origins)

	assert.Len(t, result, 4)
	assert.Contains(t, result, "http://localhost:3000")
	assert.Contains(t, result, "https://example.com")
	assert.Contains(t, result, "https://api.example.com")
	assert.Contains(t, result, "http://192.168.1.1:8080")
}

func TestValidateAndSanitizeCORSOrigins_MixedOrigins(t *testing.T) {
	origins := []string{
		"https://valid.example.com",
		"*",                         // Should be rejected
		"https://another-valid.com",
		"*.bad.com",                 // Should be rejected
		"",                          // Should be rejected
		"http://localhost:3000",
	}
	result := ValidateAndSanitizeCORSOrigins(origins)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "https://valid.example.com")
	assert.Contains(t, result, "https://another-valid.com")
	assert.Contains(t, result, "http://localhost:3000")
}

func TestValidateAndSanitizeCORSOrigins_TrimsWhitespace(t *testing.T) {
	origins := []string{
		"  https://example.com  ",
		"\thttps://another.com\n",
	}
	result := ValidateAndSanitizeCORSOrigins(origins)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "https://example.com")
	assert.Contains(t, result, "https://another.com")
}

func TestCORSMiddleware_AllowsValidOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(CORSMiddleware([]string{"https://example.com"}))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return the origin header
	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", resp.Header.Get("Access-Control-Allow-Credentials"))
}

func TestCORSMiddleware_AllowsMethods(t *testing.T) {
	app := fiber.New()
	app.Use(CORSMiddleware([]string{"https://example.com"}))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	allowedMethods := resp.Header.Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowedMethods, "GET")
	assert.Contains(t, allowedMethods, "POST")
	assert.Contains(t, allowedMethods, "PUT")
	assert.Contains(t, allowedMethods, "PATCH")
	assert.Contains(t, allowedMethods, "DELETE")
}

func TestCORSMiddleware_AllowsCustomHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(CORSMiddleware([]string{"https://example.com"}))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Agent-ID")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	allowedHeaders := resp.Header.Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowedHeaders, "Authorization")
	assert.Contains(t, allowedHeaders, "X-Agent-ID")
	assert.Contains(t, allowedHeaders, "X-Signature")
	assert.Contains(t, allowedHeaders, "X-API-Key")
}

func TestCORSMiddleware_ExposesHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(CORSMiddleware([]string{"https://example.com"}))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	exposedHeaders := resp.Header.Get("Access-Control-Expose-Headers")
	assert.Contains(t, exposedHeaders, "Content-Disposition")
	assert.Contains(t, exposedHeaders, "Content-Length")
}

func TestCORSMiddleware_DefaultsToLocalhostForInvalidConfig(t *testing.T) {
	app := fiber.New()
	// Pass only invalid origins
	app.Use(CORSMiddleware([]string{"*", "invalid-origin"}))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should allow localhost as it's the default fallback
	assert.Equal(t, "http://localhost:3000", resp.Header.Get("Access-Control-Allow-Origin"))
}

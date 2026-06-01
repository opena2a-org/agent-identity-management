package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function for SDK tests that need org/user context
func withSDKContext(handler func(c fiber.Ctx) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		c.Locals("email", "test@example.com")
		c.Locals("role", "member")
		return handler(c)
	}
}

// ===========================
// NewSDKHandler Tests
// ===========================

func TestNewSDKHandler_NilDeps(t *testing.T) {
	handler := NewSDKHandler(nil, nil)
	assert.NotNil(t, handler)
}

// ===========================
// SDKHandler.DownloadSDK Tests
// ===========================

func TestSDKHandler_DownloadSDK_InvalidSDKType(t *testing.T) {
	handler := &SDKHandler{}
	app := fiber.New()
	app.Get("/sdk/download", withSDKContext(handler.DownloadSDK))

	req := httptest.NewRequest("GET", "/sdk/download?sdk=ruby", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestSDKHandler_DownloadSDK_NoUserContext(t *testing.T) {
	handler := &SDKHandler{}
	app := fiber.New()
	// Only set organization_id, not user_id
	app.Get("/sdk/download", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		return handler.DownloadSDK(c)
	})

	req := httptest.NewRequest("GET", "/sdk/download", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestSDKHandler_DownloadSDK_NoOrgContext(t *testing.T) {
	handler := &SDKHandler{}
	app := fiber.New()
	// Only set user_id, not organization_id
	app.Get("/sdk/download", func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return handler.DownloadSDK(c)
	})

	req := httptest.NewRequest("GET", "/sdk/download", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// ===========================
// SDKHandler helper function tests
// ===========================

func TestSDKHandler_parseDeviceName(t *testing.T) {
	handler := &SDKHandler{}

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "Empty user agent",
			userAgent: "",
			want:      "Unknown Device",
		},
		{
			name:      "Chrome on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "Chrome on macOS",
		},
		{
			name:      "Firefox on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			want:      "Firefox on Windows 10/11",
		},
		{
			name:      "Safari on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
			want:      "Safari on macOS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.parseDeviceName(tt.userAgent)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSDKHandler_generateDeviceFingerprint(t *testing.T) {
	handler := &SDKHandler{}

	// Same inputs should produce same fingerprint
	fp1 := handler.generateDeviceFingerprint("Mozilla/5.0", "192.168.1.1")
	fp2 := handler.generateDeviceFingerprint("Mozilla/5.0", "192.168.1.1")
	assert.Equal(t, fp1, fp2)

	// Different inputs should produce different fingerprint
	fp3 := handler.generateDeviceFingerprint("Mozilla/5.0", "192.168.1.2")
	assert.NotEqual(t, fp1, fp3)
}

func TestSDKHandler_generateSetupInstructions(t *testing.T) {
	handler := &SDKHandler{}

	// Test Python instructions
	pythonInstructions := handler.generateSetupInstructions("python", "aim-sdk-test")
	assert.Contains(t, pythonInstructions, "python")

	// Test Java instructions
	javaInstructions := handler.generateSetupInstructions("java", "aim-sdk-test")
	assert.Contains(t, javaInstructions, "java")

	// Test Go instructions
	goInstructions := handler.generateSetupInstructions("go", "aim-sdk-test")
	assert.Contains(t, goInstructions, "go")

	// Test JavaScript instructions
	jsInstructions := handler.generateSetupInstructions("javascript", "aim-sdk-test")
	assert.Contains(t, jsInstructions, "javascript")

	// Test unknown SDK (default case)
	unknownInstructions := handler.generateSetupInstructions("unknown", "aim-sdk-test")
	assert.Contains(t, unknownInstructions, "SDK documentation")
}

func TestGetSDKDisplayName(t *testing.T) {
	tests := []struct {
		sdkType  string
		expected string
	}{
		{"python", "Python"},
		{"java", "Java"},
		{"go", "Go"},
		{"javascript", "JavaScript"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.sdkType, func(t *testing.T) {
			result := getSDKDisplayName(tt.sdkType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsUA(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "Chrome", "Chrome", true},
		{"contains at start", "Chrome Browser", "Chrome", true},
		{"contains in middle", "Mozilla Chrome Browser", "Chrome", true},
		{"contains at end", "Mozilla Chrome", "Chrome", true},
		{"not contains", "Firefox", "Chrome", false},
		{"empty string", "", "Chrome", false},
		{"empty substr", "Chrome", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsUA(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringContainsUA(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "Chrome", "Chrome", true},
		{"not contains", "Firefox", "Chrome", false},
		{"longer substr", "Ch", "Chrome", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringContainsUA(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSDKHandler_parseDeviceName_MoreBrowsers(t *testing.T) {
	handler := &SDKHandler{}

	tests := []struct {
		name      string
		userAgent string
		contains  string
	}{
		{
			name:      "Edge on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			contains:  "Edge",
		},
		{
			name:      "Opera on Windows detected as Chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
			contains:  "Chrome", // Opera contains "Chrome" and is detected as Chrome
		},
		{
			name:      "Safari on iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
			contains:  "Safari",
		},
		{
			name:      "Chrome iOS on iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
			contains:  "Safari", // CriOS (Chrome iOS) doesn't contain "Chrome", detected as Safari
		},
		{
			name:      "Chrome on Android",
			userAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
			contains:  "Chrome",
		},
		{
			name:      "Chrome on Linux",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			contains:  "Linux",
		},
		{
			name:      "Windows 8.1",
			userAgent: "Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36",
			contains:  "Windows 8.1",
		},
		{
			name:      "Windows 8",
			userAgent: "Mozilla/5.0 (Windows NT 6.2; Win64; x64) AppleWebKit/537.36",
			contains:  "Windows 8",
		},
		{
			name:      "Windows 7",
			userAgent: "Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36",
			contains:  "Windows 7",
		},
		{
			name:      "Generic Windows",
			userAgent: "Mozilla/5.0 (Windows NT 5.1; Win64; x64) AppleWebKit/537.36",
			contains:  "Windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.parseDeviceName(tt.userAgent)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestSDKCredentials_Struct(t *testing.T) {
	creds := SDKCredentials{
		SchemaVersion: "1.0",
		Type:          "sdk_oauth",
		AIMUrl:        "http://localhost:8080",
		RefreshToken:  "test-token",
		SDKTokenID:    "token-123",
		UserID:        uuid.New().String(),
		Email:         "test@example.com",
	}

	assert.Equal(t, "1.0", creds.SchemaVersion)
	assert.Equal(t, "sdk_oauth", creds.Type)
	assert.Equal(t, "http://localhost:8080", creds.AIMUrl)
	assert.Equal(t, "test-token", creds.RefreshToken)
	assert.Equal(t, "token-123", creds.SDKTokenID)
	assert.Equal(t, "test@example.com", creds.Email)
}

// ===========================
// SDK download bug regressions (http-scheme + userEmail)
// ===========================

// TestNormalizeAIMURL locks in that the URL embedded in a downloaded SDK is
// always https for a public host (so the refresh-token POST is never 301'd and
// stripped behind a TLS-terminating ingress), while loopback hosts keep http for
// local development.
func TestNormalizeAIMURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"public http coerced to https", "http://api.aim.opena2a.org", "https://api.aim.opena2a.org"},
		{"public http with path coerced", "http://api.aim.opena2a.org/", "https://api.aim.opena2a.org"},
		{"https left untouched", "https://api.aim.opena2a.org", "https://api.aim.opena2a.org"},
		{"misconfigured AIM_PUBLIC_URL http coerced", "http://aim.opena2a.org", "https://aim.opena2a.org"},
		// Uppercase scheme: url.Parse lowercases the scheme, so this still coerces.
		{"uppercase scheme coerced", "HTTP://api.aim.opena2a.org", "https://api.aim.opena2a.org"},
		// Loopback hosts keep http for local development.
		{"localhost http preserved", "http://localhost:8080", "http://localhost:8080"},
		{"127.0.0.1 http preserved", "http://127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"0.0.0.0 http preserved", "http://0.0.0.0:8080", "http://0.0.0.0:8080"},
		{".localhost http preserved", "http://aim.localhost", "http://aim.localhost"},
		{"ipv6 loopback http preserved", "http://[::1]:8080", "http://[::1]:8080"},
		// Loopback-looking-but-public hosts must NOT be treated as loopback.
		{"localhost.evil.com is public", "http://localhost.evil.com", "https://localhost.evil.com"},
		{"notlocalhost is public", "http://notlocalhost", "https://notlocalhost"},
		{"127.0.0.1.evil.com is public", "http://127.0.0.1.evil.com", "https://127.0.0.1.evil.com"},
		// Userinfo in the authority is preserved; scheme still coerced.
		{"userinfo preserved on coercion", "http://user@evil.com", "https://user@evil.com"},
		// DELIBERATE trade-off (security-first): a non-loopback private/LAN host
		// served over plain http is coerced to https. A self-hosted http-only LAN
		// deployment must front AIM with TLS. Documented in normalizeAIMURL.
		{"rfc1918 lan http coerced", "http://10.0.0.5:8080", "https://10.0.0.5:8080"},
		{"trailing slash trimmed", "https://api.aim.opena2a.org/", "https://api.aim.opena2a.org"},
		{"empty stays empty", "", ""},
		{"scheme-less value returned as-is", "api.aim.opena2a.org", "api.aim.opena2a.org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeAIMURL(tt.in))
		})
	}
}

// TestSDKCredentials_JSONUsesUserEmail locks in that the credentials file
// embedded in the download carries the email under "userEmail" (read by the
// Python SDK, aim_sdk/cli.py — emitting only "email" produced `User: Unknown`)
// AND under "email" (read by the Java SDK CredentialManager). Both keys must be
// present and equal so neither SDK regresses.
func TestSDKCredentials_JSONUsesUserEmail(t *testing.T) {
	creds := SDKCredentials{
		SchemaVersion: "1.0",
		Type:          "sdk_oauth",
		AIMUrl:        "https://api.aim.opena2a.org",
		RefreshToken:  "test-token",
		SDKTokenID:    "token-123",
		UserID:        uuid.New().String(),
		UserEmail:     "test@example.com",
		Email:         "test@example.com",
	}

	data, err := json.Marshal(creds)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	gotUserEmail, ok := raw["userEmail"]
	require.True(t, ok, "credentials JSON must carry userEmail (read by the Python SDK)")
	assert.Equal(t, "test@example.com", gotUserEmail)

	gotEmail, ok := raw["email"]
	require.True(t, ok, "credentials JSON must still carry email (read by the Java SDK)")
	assert.Equal(t, "test@example.com", gotEmail)
}

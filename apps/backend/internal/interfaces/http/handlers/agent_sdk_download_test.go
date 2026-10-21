package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// GET /api/v1/agents/:id/sdk builds a Python package around an agent's keys. The dashboard's
// agent download names the language in ?language=, and the package carries the version that
// sdk/python/VERSION states, the version GET /sdk/download names its archive after (#792).

// agentSDKDownload answers GET /agents/:id/sdk<query> for a member of the agent's organization,
// and returns the status, the headers and the body.
func agentSDKDownload(t *testing.T, query string) (int, http.Header, []byte) {
	t.Helper()
	return agentSDKDownloadAt(t, "", query)
}

// agentSDKDownloadAt is agentSDKDownload for a request whose Host header is host; an empty
// host leaves the header httptest gives the request.
func agentSDKDownloadAt(t *testing.T, host, query string) (int, http.Header, []byte) {
	t.Helper()
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	agents := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "download-agent", Status: domain.AgentStatusVerified}, nil
		},
		GetAgentCredentialsFunc: func(ctx context.Context, id uuid.UUID) (string, string, error) {
			return "not-a-real-public-key", "not-a-real-private-key", nil
		},
	}
	handler := NewAgentHandlerWithInterfaces(
		agents,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, orgID, userID)
	app.Get("/agents/:id/sdk", handler.DownloadSDK)

	req := httptest.NewRequest("GET", "/agents/"+agentID.String()+"/sdk"+query, nil)
	if host != "" {
		req.Host = host
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header, body
}

// sdkTreeWithPythonVersion returns an SDK base directory whose python/VERSION holds content.
func sdkTreeWithPythonVersion(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "python"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "python", "VERSION"), []byte(content), 0o644))
	return dir
}

// zipTexts returns each file of the zip in body by name.
func zipTexts(t *testing.T, body []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err, "the body is a zip")
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		files[f.Name] = string(data)
	}
	return files
}

func TestAgentSDKDownload_AnswersForTheLanguageInTheQuery(t *testing.T) {
	t.Setenv("SDK_BASE_DIR", sdkTreeWithPythonVersion(t, "7.8.9\n"))

	cases := []struct {
		query  string
		status int
	}{
		// ?language= is what the dashboard sends.
		{"?language=python", fiber.StatusOK},
		{"?language=java", fiber.StatusBadRequest},
		{"?language=rust", fiber.StatusBadRequest},
		{"?language=nodejs", fiber.StatusNotImplemented},
		{"?language=go", fiber.StatusNotImplemented},
		// ?lang= is read when ?language= is not given.
		{"?lang=python", fiber.StatusOK},
		{"?lang=java", fiber.StatusBadRequest},
		{"?language=java&lang=python", fiber.StatusBadRequest},
		{"?language=python&lang=java", fiber.StatusOK},
		// Neither: the Python package.
		{"", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			status, header, body := agentSDKDownload(t, tc.query)
			require.Equal(t, tc.status, status, string(body))
			switch tc.status {
			case fiber.StatusOK:
				assert.Equal(t, "application/zip", header.Get("Content-Type"))
				assert.Contains(t, zipTexts(t, body), "aim_sdk/config.py")
			case fiber.StatusBadRequest:
				assert.JSONEq(t, `{"error":"Invalid language. Supported: python, nodejs, go"}`, string(body))
				assert.Empty(t, header.Get("Content-Disposition"), "no package is sent for a language the route does not know")
			}
		})
	}
}

func TestAgentSDKDownload_CarriesThePythonSDKVersion(t *testing.T) {
	t.Setenv("SDK_BASE_DIR", sdkTreeWithPythonVersion(t, "7.8.9\n"))

	status, header, body := agentSDKDownload(t, "?language=python")
	require.Equal(t, fiber.StatusOK, status, string(body))
	assert.Equal(t, "attachment; filename=aim-sdk-python-v7.8.9.zip", header.Get("Content-Disposition"))

	files := zipTexts(t, body)
	assert.Contains(t, files["aim_sdk/__init__.py"], `__version__ = "7.8.9"`)
	assert.Contains(t, files["aim_sdk/client.py"], `'User-Agent': 'AIM-Python-SDK/7.8.9'`)
	assert.Contains(t, files["aim_sdk/config.py"], `SDK_VERSION = "7.8.9"`)
	assert.Contains(t, files["setup.py"], `version="7.8.9"`)
	assert.Contains(t, files["README.md"], "**SDK Version**: 7.8.9")
	for name, text := range files {
		assert.NotContains(t, text, "1.6.0", "%s names the version the route was stamped with before", name)
	}
}

// The repository's own tree: the per-agent package names the version GET /sdk/download names.
func TestAgentSDKDownload_NamesTheVersionTheSDKDownloadNames(t *testing.T) {
	sdkStatus, sdkHeader, sdkBody := fetchSDKDownload(t, "python")
	require.Equal(t, fiber.StatusOK, sdkStatus, string(sdkBody))

	t.Setenv("SDK_BASE_DIR", sdkSourceTree(t))
	version, err := os.ReadFile(filepath.Join(sdkSourceTree(t), "python", "VERSION"))
	require.NoError(t, err)

	status, header, body := agentSDKDownload(t, "?language=python")
	require.Equal(t, fiber.StatusOK, status, string(body))
	assert.Equal(t, sdkHeader.Get("Content-Disposition"), header.Get("Content-Disposition"))
	assert.Equal(t, "attachment; filename=aim-sdk-python-v"+string(bytes.TrimSpace(version))+".zip", header.Get("Content-Disposition"))
}

// The package's config.py and README name the server at the host and port the request reached
// it on: a package downloaded from 127.0.0.1:18080 names http://127.0.0.1:18080, where it named
// http://127.0.0.1 (#792).
func TestAgentSDKDownload_NamesThePortTheRequestReached(t *testing.T) {
	t.Setenv("SDK_BASE_DIR", sdkTreeWithPythonVersion(t, "7.8.9\n"))

	cases := []struct {
		host   string
		aimURL string
	}{
		{"127.0.0.1:18080", "http://127.0.0.1:18080"},
		{"localhost:8080", "http://localhost:8080"},
		{"[::1]:8080", "http://[::1]:8080"},
		// A public host gets https and keeps its port.
		{"aim.example.com:8443", "https://aim.example.com:8443"},
		// No port in the request: none in the URL.
		{"aim.example.com", "https://aim.example.com"},
		{"127.0.0.1", "http://127.0.0.1"},
		// The scheme's own port is left out before a public host gets https.
		{"aim.example.com:80", "https://aim.example.com"},
		{"localhost:80", "http://localhost"},
		// A port that is not a number from 1 to 65535 is left out.
		{"aim.example.com:0", "https://aim.example.com"},
		{"aim.example.com:65536", "https://aim.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			status, _, body := agentSDKDownloadAt(t, tc.host, "?language=python")
			require.Equal(t, fiber.StatusOK, status, string(body))

			files := zipTexts(t, body)
			assert.Contains(t, files["aim_sdk/config.py"], "AIM_URL = \""+tc.aimURL+"\"\n")
			assert.Contains(t, files["README.md"], "- **AIM Server**: "+tc.aimURL+"\n")
		})
	}
}

// A Host header whose port is empty or holds something other than digits is left without its
// port. app.Test refuses to send aim.example.com:abc, so this sets the header on the request
// context.
func TestGetAIMBaseURL_LeavesOutAPortThatIsNotANumber(t *testing.T) {
	app := fiber.New()
	for _, host := range []string{"aim.example.com:abc", "aim.example.com:+80", "aim.example.com:"} {
		t.Run(host, func(t *testing.T) {
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetHost(host)
			c := app.AcquireCtx(fctx)
			defer app.ReleaseCtx(c)
			require.Equal(t, host, c.Host(), "the request carries the header as written")
			assert.Equal(t, "https://aim.example.com", getAIMBaseURL(c))
		})
	}
}

// Without a version to name, the route sends no package rather than one stamped with a number
// that is not the SDK's.
func TestAgentSDKDownload_RefusesWithoutTheSDKVersion(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"no VERSION file": func(t *testing.T) string { return t.TempDir() },
		"an empty VERSION file": func(t *testing.T) string {
			return sdkTreeWithPythonVersion(t, "\n")
		},
		"a VERSION file with a quote and a line break": func(t *testing.T) string {
			return sdkTreeWithPythonVersion(t, "7.8.9\"\nimport os\n")
		},
		"a VERSION file with a space": func(t *testing.T) string {
			return sdkTreeWithPythonVersion(t, "7.8.9 beta\n")
		},
	}
	for name, sdkDir := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SDK_BASE_DIR", sdkDir(t))
			status, header, body := agentSDKDownload(t, "?language=python")
			assert.Equal(t, fiber.StatusInternalServerError, status, string(body))
			assert.JSONEq(t, `{"error":"Failed to generate SDK"}`, string(body))
			assert.Empty(t, header.Get("Content-Disposition"))
		})
	}
}

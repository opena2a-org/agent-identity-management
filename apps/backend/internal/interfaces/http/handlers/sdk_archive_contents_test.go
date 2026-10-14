package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// GET /sdk/download?sdk=<type> builds its zip from the repository's sdk/ tree, the tree
// the server image copies (COPY sdk ./sdk). These tests read that tree, not a fixture.

// sdkSourceTree is the repository's sdk/ directory.
func sdkSourceTree(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "..", "sdk"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "python", "VERSION"))
	require.NoError(t, err, "the repository sdk/ tree, which the server image copies")
	return dir
}

// fetchSDKArchive answers GET /sdk/download?sdk=<sdk> for a signed-in user, with the archive
// built from the repository's sdk/ tree, and returns the status and body.
func fetchSDKArchive(t *testing.T, sdk string) (int, []byte) {
	t.Helper()
	t.Setenv("SDK_BASE_DIR", sdkSourceTree(t))
	t.Setenv("AIM_PUBLIC_URL", "https://api.aim.example.org")
	f := newMintFixture(t, &rotationStore{}, false)
	h := NewSDKHandler(f.svc, &rotationSDKRepo{}, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Get("/sdk/download", f.acting(f.family, h.DownloadSDK))
	resp, err := app.Test(httptest.NewRequest("GET", "/sdk/download?sdk="+sdk, nil), fiber.TestConfig{Timeout: 30 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// The download is for using an SDK, so it leaves out the SDK's own tests and the tooling used
// to build and check it, while the sources, packaging and examples still ship. Each excluded
// path exists in the repository's tree, so an archive that copied the tree whole fails here.
func TestSDKArchive_LeavesOutTheSDKsTestsAndTooling(t *testing.T) {
	sdkDir := sdkSourceTree(t)
	cases := []struct {
		sdk      string
		excluded []string // present in the tree, absent from the zip; a trailing slash is a directory
		kept     []string
	}{
		{
			sdk:      "python",
			excluded: []string{"tests/", "scripts/", "tools/", "tests/conftest.py", "pytest.ini", "requirements-dev.txt", "test_sdk_comprehensive.py"},
			kept:     []string{"aim_sdk/__init__.py", "setup.py", "requirements.txt", "README.md", "examples/example.py"},
		},
		{
			sdk:      "java",
			excluded: []string{"src/test/"},
			kept:     []string{"pom.xml", "README.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.sdk, func(t *testing.T) {
			for _, p := range tc.excluded {
				_, err := os.Stat(filepath.Join(sdkDir, tc.sdk, filepath.FromSlash(strings.TrimSuffix(p, "/"))))
				require.NoError(t, err, "sdk/%s/%s is in the repository tree", tc.sdk, p)
			}

			status, body := fetchSDKArchive(t, tc.sdk)
			require.Equal(t, fiber.StatusOK, status, string(body))
			zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
			require.NoError(t, err, "the body is a zip")

			prefix := "aim-sdk-" + tc.sdk + "/"
			names := map[string]bool{}
			for _, e := range zr.File {
				rel := strings.TrimPrefix(e.Name, prefix)
				names[rel] = true
				for _, p := range tc.excluded {
					if rel == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(rel, p)) {
						t.Errorf("%s ships in the download; %s is the SDK's own test or tooling path", e.Name, p)
					}
				}
				base := rel[strings.LastIndex(rel, "/")+1:]
				if base == "conftest.py" || (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) || strings.Contains(rel, "/tests/") {
					t.Errorf("%s ships in the download; it is a test file", e.Name)
				}
			}
			for _, p := range tc.kept {
				assert.True(t, names[p], "%s%s is part of the SDK and ships", prefix, p)
			}
		})
	}
}

// The Python quick start points at the examples the download carries, not at test files it
// leaves out.
func TestSDKArchive_PythonQuickStartPointsAtShippedExamples(t *testing.T) {
	status, body := fetchSDKArchive(t, "python")
	require.Equal(t, fiber.StatusOK, status, string(body))
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	rc, err := zr.Open("aim-sdk-python/QUICKSTART.md")
	require.NoError(t, err)
	quickstart, err := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.NoError(t, err)
	assert.NotContains(t, string(quickstart), "test files")
	assert.Contains(t, string(quickstart), "`examples/`")
}

// An SDK the endpoint does not serve is refused with the list of SDKs it does serve, and the
// answer promises no release date.
func TestSDKArchive_UnservedSDKIsRefusedWithoutASchedule(t *testing.T) {
	for _, sdk := range []string{"go", "javascript"} {
		t.Run(sdk, func(t *testing.T) {
			status, body := fetchSDKArchive(t, sdk)
			require.Equal(t, fiber.StatusBadRequest, status, string(body))
			var got struct {
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, fmt.Sprintf("Invalid SDK type '%s'. Available SDKs: 'python', 'java'.", sdk), got.Error)
		})
	}
}

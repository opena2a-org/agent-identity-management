package domain

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionRequestConstants(t *testing.T) {
	// K >= W: a purged nonce can be admitted again only if the database clock
	// steps back by more than K, which also refuses honest requests.
	assert.GreaterOrEqual(t, ActionRequestNonceRetentionSeconds, ActionRequestWindowSeconds)
	assert.Equal(t, 30, ActionRequestWindowSeconds)
	assert.Equal(t, 31, ActionRequestNonceRetentionSeconds)
	// The admission statement times out before K elapses.
	assert.Less(t, ActionRequestAdmissionTimeout, time.Duration(ActionRequestNonceRetentionSeconds)*time.Second)
	assert.Positive(t, ActionRequestAdmissionTimeout)

	assert.Equal(t, 46, len(ActionRequestPayloadType))
	assert.Equal(t, 87382, ActionRequestMaxSignedBytesChars)
	assert.Equal(t, 88406, ActionRequestMaxRawBody)
	assert.Equal(t, 32, ActionRequestMaxDepth)
	assert.LessOrEqual(t, ActionRequestMaxRawBody, 4*1024*1024, "below the server's default body limit")
}

// backendGoSources returns the backend's non-test Go files and their text.
func backendGoSources(t *testing.T) map[string]string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	sources := map[string]string{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		sources[rel] = string(b)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, sources)
	return sources
}

// The format's public name and its payload type each have one declaration;
// every message interpolates them.
func TestActionRequestNamesAreDeclaredOnce(t *testing.T) {
	names := 0
	types := 0
	for path, src := range backendGoSources(t) {
		if n := strings.Count(src, "action-request-v1"); n > 0 {
			names += n
			assert.Equal(t, filepath.Join("internal", "domain", "action_request.go"), path)
		}
		if n := strings.Count(src, "application/vnd.opena2a.action-request.v1+json"); n > 0 {
			types += n
			assert.Equal(t, filepath.Join("internal", "domain", "action_request.go"), path)
		}
		// The S1 request-signing middleware names no part of this format.
		if strings.Contains(path, filepath.Join("http", "middleware")) {
			assert.NotContains(t, src, "action-request-v", path)
		}
	}
	assert.Equal(t, 1, names)
	assert.Equal(t, 1, types)
}

// The window, the retention, the purge schedule and the admission timeout
// are code constants: nothing on their path reads configuration or the
// environment, and no flag turns the purge off.
func TestActionRequestConstantsAreNotConfigurable(t *testing.T) {
	sources := backendGoSources(t)
	for _, path := range []string{
		filepath.Join("internal", "domain", "action_request.go"),
		filepath.Join("internal", "application", "action_request_nonce_service.go"),
		filepath.Join("internal", "infrastructure", "repository", "agent_request_nonce_repository.go"),
		filepath.Join("internal", "interfaces", "http", "handlers", "action_request_statement.go"),
	} {
		src, ok := sources[path]
		require.True(t, ok, path)
		for _, read := range []string{"os.Getenv", "os.LookupEnv", "os.Environ", "viper.", "config."} {
			assert.NotContains(t, src, read, path)
		}
	}
	for path, src := range sources {
		for _, name := range []string{"ActionRequestWindowSeconds", "ActionRequestNonceRetentionSeconds",
			"ActionRequestNoncePurgeInterval", "ActionRequestAdmissionTimeout"} {
			if !strings.Contains(src, name) {
				continue
			}
			for _, line := range strings.Split(src, "\n") {
				if strings.Contains(line, name) {
					assert.NotContains(t, line, "Getenv", "%s: %s", path, line)
					assert.NotContains(t, line, "cfg.", "%s: %s", path, line)
				}
			}
		}
	}
}

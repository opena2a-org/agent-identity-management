package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// The introduction of section 5 of docs/specs/atc-verification-v1.md said
// that, with REGISTRY_URL set, no request the server answers reads the list it
// holds, while GET /metrics and GET /health/ready both answer from that list
// (#786). The test below holds the introduction to the server: both routes
// answer from the list held and send the authority no request, and the
// introduction names both and says no request the server answers fetches the
// list.

// atcSpecSection5Intro returns the introduction of section 5 of the ATC
// verification specification, from its heading to the heading of section
// 5.1, with each run of white space replaced by one space.
func atcSpecSection5Intro(t *testing.T) string {
	t.Helper()
	text := aim03ReadRepoFile(t, "docs/specs/atc-verification-v1.md")
	start := strings.Index(text, "\n## 5. ")
	require.GreaterOrEqual(t, start, 0, "the ATC verification specification has no section 5")
	intro := text[start+1:]
	end := strings.Index(intro, "\n### 5.1 ")
	require.GreaterOrEqual(t, end, 0, "section 5 of the ATC verification specification has no section 5.1")
	return strings.Join(strings.Fields(intro[:end]), " ")
}

func TestATCSpecSection5_NamesTheRequestsThatReadTheListHeld(t *testing.T) {
	requirePublicRepositoryFiles(t)
	a := newRevocationListAuthority(t, "did:web:agents.example:revoked-agent")
	captureLockedLog(t)

	refresher, err := startRevocationList(config.RegistryConfig{URL: a.srv.URL}, a.srv.Client())
	require.NoError(t, err)
	require.NotNil(t, refresher)
	t.Cleanup(refresher.Stop)
	deadline := time.Now().Add(10 * time.Second)
	for ok, _ := refresher.State(); !ok; ok, _ = refresher.State() {
		if time.Now().After(deadline) {
			t.Fatal("no list was held 10 s after the start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	before := a.requested()

	assert.Contains(t, scrapeMetrics(t), "aim_atc_revocation_list_entries 1\n",
		"GET /metrics gives the entries of the list held")

	resp, err := revocationReadyApp(aim07CheckOK, revocationReadyCheck(refresher)).
		Test(httptest.NewRequest(fiber.MethodGet, "/health/ready", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"status": "ok", "required": false}, revocationStateEntry(t, body),
		"GET /health/ready gives the state of the list held")

	assert.Equal(t, before, a.requested(), "GET /metrics and GET /health/ready send the authority no request")

	intro := atcSpecSection5Intro(t)
	for _, want := range []string{
		"no request it answers fetches the list",
		"`GET /metrics` and `GET /health/ready` read the list it holds",
		"`aim_atc_revocation_list_age_seconds` and `aim_atc_revocation_list_entries`",
		"`dependencies.revocationState`",
		"Its ATC verifier does not read it: it reads step 8 from its own store (Section 9.4).",
	} {
		assert.Contains(t, intro, want, "the introduction of section 5 does not say it")
	}
	assert.NotContains(t, intro, "no request it answers reads the list it holds",
		"GET /metrics and GET /health/ready are requests the server answers, and both read the list held")
}

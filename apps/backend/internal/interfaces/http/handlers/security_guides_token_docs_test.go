package handlers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The security guide and the token rotation guide tell an SDK user where the
// SDK keeps its credentials, which dashboard page lists SDK tokens, how long an
// access token lives and what the server does when a rotated-out refresh token
// is presented again. Each of those is pinned here to what the refresh route,
// the dashboard and the Python SDK do, so a change to any of them fails next to
// the page that would then be wrong.
//
// The guides named ~/.aim/credentials.json for agent credentials, a
// /dashboard/settings/sdk-tokens page, a checklist item to confirm an
// .encrypted credentials file exists, 1-hour access tokens and a security alert
// on a reused refresh token (#678).

const (
	securityGuidePath      = "../../docs/guides/SECURITY.md"
	tokenRotationGuidePath = "../../docs/security/token-rotation.md"
)

// guideSection returns page from the line that is heading to the next heading
// of the same or a higher level. A line in a fenced code block is not a
// heading: a shell comment starts with # too.
func guideSection(t *testing.T, page, heading string) string {
	t.Helper()
	lines := strings.Split(page, "\n")
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	start := -1
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if start == -1 {
			if line == heading {
				start = i
			}
			continue
		}
		hashes := len(line) - len(strings.TrimLeft(line, "#"))
		if hashes > 0 && hashes <= level && strings.HasPrefix(line[hashes:], " ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	require.NotEqual(t, -1, start, "the guide has no %q heading", heading)
	return strings.Join(lines[start:], "\n")
}

// jsonBlockWith returns the ```json block in text that carries needle, decoded.
func jsonBlockWith(t *testing.T, text, needle string) map[string]interface{} {
	t.Helper()
	for _, block := range regexp.MustCompile("(?s)```json\n(.*?)```").FindAllStringSubmatch(text, -1) {
		if !strings.Contains(block[1], needle) {
			continue
		}
		var example map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(block[1]), &example), "the documented %s example must be valid JSON", needle)
		return example
	}
	t.Fatalf("no JSON example carries %q", needle)
	return nil
}

// snakeTokenKey matches a JSON key the refresh route neither reads nor writes.
var snakeTokenKey = regexp.MustCompile(`"(access|refresh)_token"\s*:`)

func TestSecurityGuidesQuoteTheRefreshTheServerAnswers(t *testing.T) {
	// Unset, so the lifetime is the server default the guides quote.
	t.Setenv("JWT_ACCESS_TTL", "")

	f := newSDKFamilyFixture(t, &rotationStore{})
	body, status := postRefreshRaw(t, f.refresh, f.downloadSDK(t))
	require.Equal(t, fiber.StatusOK, status, body)
	var issued map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &issued))

	ttl := f.svc.AccessTTLSeconds()
	require.Equal(t, float64(ttl), issued["expiresIn"], "the response reports the configured lifetime")
	require.Zero(t, ttl%3600, "the guides quote the default lifetime in whole hours")
	hours := ttl / 3600

	requestField, ok := reflect.TypeOf(RefreshTokenRequest{}).FieldByName("RefreshToken")
	require.True(t, ok)
	field := strings.Split(requestField.Tag.Get("json"), ",")[0]

	security := readBackendFile(t, securityGuidePath)
	rotation := readBackendFile(t, tokenRotationGuidePath)

	t.Run("the security guide's refresh example is a response the route gives", func(t *testing.T) {
		step := guideSection(t, security, "### 4. **Token Rotation on Refresh**")
		textHas(t, step, fmt.Sprintf(`{"%s": `, field), "the documented request carries the field the handler binds")
		example := jsonBlockWith(t, step, "accessToken")
		assert.Equal(t, sortedKeys(issued), sortedKeys(example),
			"the documented response carries exactly the fields the handler returns")
		for _, key := range []string{"tokenType", "expiresIn", "rotated"} {
			assert.Equal(t, issued[key], example[key], "the documented %s", key)
		}
	})

	t.Run("no guide uses a token field the route does not read or write", func(t *testing.T) {
		assert.Empty(t, snakeTokenKey.FindAllString(security, -1), "the security guide")
		assert.Empty(t, snakeTokenKey.FindAllString(rotation, -1), "the token rotation guide")
	})

	t.Run("the token rotation guide quotes the access token lifetime", func(t *testing.T) {
		faq := guideSection(t, rotation, "### Q: How often do tokens rotate?")
		textHas(t, faq, fmt.Sprintf("**%d hours**", hours), "the FAQ quotes the default access token lifetime")
		textHas(t, faq, "JWT_ACCESS_TTL", "the FAQ names the setting that changes it")
		for _, page := range []string{security, rotation} {
			textLacks(t, page, "1 hour", "a guide quotes another access token lifetime")
			textLacks(t, page, "once per hour", "a guide quotes another rotation interval")
			textLacks(t, page, "86400", "a guide quotes a 24-hour expiresIn")
		}
	})
}

func TestTokenRotationGuideDescribesWhatAReuseDoes(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	out, status := postRefresh(t, f.refresh, s1)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, out.Rotated)

	// The guide's theft scenario: the rotated-out token is presented again.
	_, status = postRefreshRaw(t, f.refresh, s1)
	require.Equal(t, fiber.StatusUnauthorized, status, "the rotated-out token is refused")
	_, status = postRefreshRaw(t, f.refresh, out.RefreshToken)
	require.Equal(t, fiber.StatusUnauthorized, status, "the reuse ended the family: the agent's own token is refused too")
	reuse := f.reuseRows()
	require.Len(t, reuse, 1, "the reuse writes one audit record")
	require.Equal(t, true, reuse[0].Metadata["familyRevoked"])
	require.Len(t, f.audit.rows, 2, "the later refusal of the family's live token writes its own record")
	require.Equal(t, domain.AuditActionRefreshSessionRevoked, f.audit.rows[1].Action)

	// The refresh route holds no alert service, so nothing raises an alert for
	// a reuse. If it gets one, rewrite the guide's paragraphs and this test
	// together.
	handler := reflect.TypeOf(AuthRefreshHandler{})
	for i := 0; i < handler.NumField(); i++ {
		assert.NotContains(t, strings.ToLower(handler.Field(i).Type.String()), "alert",
			"the refresh route raises no alert (field %s)", handler.Field(i).Name)
	}

	rotation := readBackendFile(t, tokenRotationGuidePath)
	textLacks(t, strings.ToLower(rotation), "alert", "the guide must not promise an alert the server does not raise")

	theft := guideSection(t, rotation, "### Theft Scenario (Token Rotation Protects You)")
	textHas(t, theft, string(domain.AuditActionRefreshTokenReuse), "the theft scenario names the audit record a reuse writes")
	textHas(t, theft, "token family", "the theft scenario says the reuse ends the token family")
	textHas(t, theft, "refresh_token_v2 included", "the theft scenario says the agent's own token is refused too")
	textHas(t, theft, "aim-sdk login", "the theft scenario says how the agent gets new credentials")

	monitoring := guideSection(t, rotation, "### Monitoring Token Rotation")
	textHas(t, monitoring, string(domain.AuditActionRefreshTokenReuse), "the monitoring section names the reuse record")
	textHas(t, monitoring, string(domain.AuditActionRefreshSessionRevoked), "the monitoring section names the family member record")
	textHas(t, monitoring, "/dashboard/admin/audit-logs", "the monitoring section names the audit log page")
	textLacks(t, monitoring, "Token Activity", "the dashboard has no Token Activity page")
}

func TestSecurityGuidesNameTheSDKCredentialFilesAndTheirPage(t *testing.T) {
	// Where the Python SDK keeps what it writes.
	sdk := readBackendFile(t, "../../sdk/python/aim_sdk/credentials.py")
	textHas(t, sdk, `SDK_CREDENTIALS_FILE = AIM_DIR / "sdk_credentials.json"`, "the SDK's own credentials file")
	textHas(t, sdk, `AGENTS_DIR = AIM_DIR / "agents"`, "the directory of agent credentials")
	textHas(t, sdk, `return AGENTS_DIR / f"{safe_name}.json"`, "one file per agent")

	// The dashboard page and section that list SDK tokens.
	textHas(t, readBackendFile(t, "../web/lib/hub-tabs.ts"),
		`{ name: "Credentials", href: "/dashboard/credentials"`, "the Developers hub links the Credentials page")
	textHas(t, readBackendFile(t, "../web/app/dashboard/credentials/page.tsx"),
		`<section id="sdk-tokens"`, "the Credentials page has an SDK tokens section")

	// A credentials.json that is not sdk_credentials.json: the legacy file the
	// SDK migrates away from.
	legacyFile := regexp.MustCompile(`(^|[^_A-Za-z])credentials\.json`)

	security := readBackendFile(t, securityGuidePath)
	rotation := readBackendFile(t, tokenRotationGuidePath)
	for name, page := range map[string]string{"security guide": security, "token rotation guide": rotation} {
		assert.Empty(t, legacyFile.FindAllString(page, -1), "the %s names a credentials file the SDK does not write", name)
		textLacks(t, page, "/dashboard/settings", "the "+name+" names a page the dashboard does not serve")
		textLacks(t, page, "Settings → SDK Tokens", "the "+name+" names a page the dashboard does not serve")
	}

	textHas(t, security, "~/.aim/sdk_credentials.json", "the security guide names the SDK credentials file")
	textHas(t, security, "~/.aim/agents/<agent-name>.json", "the security guide names the agent credentials files")
	textHas(t, security, "/dashboard/credentials#sdk-tokens", "the security guide links the SDK tokens section")

	checklist := guideSection(t, security, "### For Developers")
	textLacks(t, checklist, "keyring", "the SDK keeps no encryption key in the keyring")
	textLacks(t, checklist, ".encrypted file exists", "the SDK writes no .encrypted file")
	textHas(t, checklist, "~/.aim/agents/<agent-name>.json", "the checklist names the agent credentials files")
}

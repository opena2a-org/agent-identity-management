package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
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

// sdkRotationLine returns a pattern for the line the Python SDK prints after a
// refresh that rotated its token: the literal oauth.py prints, then the new
// token's id.
func sdkRotationLine(t *testing.T) *regexp.Regexp {
	t.Helper()
	src := readBackendFile(t, "../../sdk/python/aim_sdk/oauth.py")
	printed := regexp.MustCompile(`print\(\s*("Token rotated successfully[^"\n]*")\s*f"\(new id: \{[^\n]*\}\)"\s*\)`).FindStringSubmatch(src)
	require.Len(t, printed, 2, "oauth.py must print the rotation line as one literal followed by the new id")
	// The literal's escapes (—) read the same in Go.
	text, err := strconv.Unquote(printed[1])
	require.NoError(t, err)
	return regexp.MustCompile(`^` + regexp.QuoteMeta(text) + `\(new id: \S+\)$`)
}

// The token rotation guide's FAQ quoted a line the Python SDK does not print
// and said it follows every successful refresh (#697). The line is pinned here
// to the SDK's print statement, and what the FAQ says it depends on to the
// answers of the refresh route with and without a revocation store.
func TestTokenRotationGuideQuotesTheRotationLineTheSDKPrints(t *testing.T) {
	rotation := readBackendFile(t, tokenRotationGuidePath)
	faq := guideSection(t, rotation, "### Q: How do I know if my token has been rotated?")

	t.Run("the quoted line is the one the Python SDK prints", func(t *testing.T) {
		blocks := regexp.MustCompile("(?s)```\n(.*?)\n```").FindAllStringSubmatch(faq, -1)
		require.Len(t, blocks, 1, "the FAQ quotes the line in one fenced block")
		assert.Regexp(t, sdkRotationLine(t), blocks[0][1], "the FAQ quotes the line in the format oauth.py prints")
		textLacks(t, rotation, "new credentials saved", "the guide quotes a line no SDK in this repository prints")
	})

	t.Run("a sign-in from aim-sdk login rotates only with a revocation store", func(t *testing.T) {
		// The device grant hands aim-sdk login a login-issued pair, the kind
		// rotationApp presents.
		textHas(t, readBackendFile(t, "internal/application/device_auth_service.go"),
			"s.jwtService.GenerateTokenPair(", "the device grant issues a login token pair")

		for _, tc := range []struct {
			name     string
			store    *rotationStore
			failOpen bool
			rotated  bool
		}{
			{name: "a revocation store", store: &rotationStore{}, rotated: true},
			{name: "no revocation store"},
			{name: "a store that refuses the write", store: &rotationStore{setErr: errors.New("write refused")}},
			{name: "a store that refuses the write, fail-open", store: &rotationStore{setErr: errors.New("write refused")}, failOpen: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				app, _, presented := rotationApp(t, tc.store, tc.failOpen)
				out, status := postRefresh(t, app, presented)
				require.Equal(t, fiber.StatusOK, status)
				assert.Equal(t, tc.rotated, out.Rotated, "the rotated field of the answer")
				assert.Equal(t, tc.rotated, out.RefreshToken != presented,
					"a new refresh token is answered only when the presented one was retired")
			})
		}

		textHas(t, faq, "A sign-in from `aim-sdk login` is rotated only by a server with Redis", "the FAQ says which server rotates a sign-in")
		textHas(t, faq, "A server without Redis, or one whose Redis refused the write, answers with the\n  refresh token it was given",
			"the FAQ says what the server answers when it cannot retire the token")
		textHas(t, faq, `"rotated": true`, "the FAQ names the field of the answer")
		textHas(t, faq, `"rotated": false`, "the FAQ names the field of the answer")
		textLacks(t, faq, "Token refresh succeeds", "the FAQ lists the line as following every successful refresh")
	})

	t.Run("a server without Redis logs the line the FAQ quotes", func(t *testing.T) {
		// One line for an empty REDIS_HOST, another for a Redis that could not
		// be reached: the FAQ quotes both, as the server writes them.
		server := readBackendFile(t, "cmd/server/main.go")
		for _, logged := range []string{
			"Token revocation disabled (no Redis configured)",
			"Token revocation disabled (Redis unavailable)",
		} {
			textHas(t, faq, "`"+logged+"`", "the FAQ quotes the startup line")
			textHas(t, server, `"ℹ️  `+logged+`: refresh tokens are not rotated`, "the server writes that line at startup")
		}
	})

	t.Run("an SDK download token rotates without a revocation store", func(t *testing.T) {
		app, _, presented := sdkRotationApp(t, &rotationSDKRepo{})
		out, status := postRefresh(t, app, presented)
		require.Equal(t, fiber.StatusOK, status)
		assert.True(t, out.Rotated, "the sdk_tokens row is retired with no revocation store")
		assert.NotEqual(t, presented, out.RefreshToken)
		textHas(t, faq, "A token from an SDK download is rotated in the database and does not need\n  Redis",
			"the FAQ says an SDK download token needs no Redis")
	})
}

// oneLine joins text's lines, so a phrase is found across a line break.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// The token rotation guide said in its opening and in its FAQ that every
// refresh rotates the refresh token, and its configuration block named a file
// the repository does not have (#701). A sign-in from aim-sdk login rotates
// only on a server with Redis, which the FAQ test above holds to the refresh
// route; every section that says how often a token rotates says so too.
func TestTokenRotationGuideSaysWhichRefreshesRotate(t *testing.T) {
	rotation := readBackendFile(t, tokenRotationGuidePath)
	for _, claim := range []string{"Every time you use a refresh token", "every time they're used", "cannot be disabled"} {
		textLacks(t, rotation, claim, "the guide says every refresh rotates")
	}

	for heading, phrases := range map[string][]string{
		"## What is Token Rotation?": {
			"A token from an SDK download is rotated on every refresh",
			"A sign-in from `aim-sdk login` is rotated only by a server with Redis",
		},
		"### Q: How often do tokens rotate?": {
			"a token from an SDK download on every server, and a sign-in from `aim-sdk login` on a server with Redis",
		},
		"### Q: Can I disable token rotation?": {
			"A sign-in from `aim-sdk login` rotates only on a server with Redis",
		},
		"### Configuration": {
			"There is no configuration file for token rotation",
			"`REDIS_HOST`", "`AUTH_REVOCATION_FAIL_OPEN`", "`JWT_ACCESS_TTL`",
		},
	} {
		section := oneLine(guideSection(t, rotation, heading))
		for _, phrase := range phrases {
			textHas(t, section, phrase, heading)
		}
	}
	textHas(t, readBackendFile(t, "internal/config/config.go"), `"REDIS_HOST"`, "the server reads REDIS_HOST")
	textHas(t, readBackendFile(t, "cmd/server/main.go"), `"AUTH_REVOCATION_FAIL_OPEN"`, "the server reads AUTH_REVOCATION_FAIL_OPEN")

	// Every repository path the guide names is in the repository.
	for _, path := range regexp.MustCompile("apps/backend/[A-Za-z0-9_./-]+[A-Za-z0-9_]").FindAllString(rotation, -1) {
		_, err := os.Stat(backendFile(t, strings.TrimPrefix(path, "apps/backend/")))
		assert.NoError(t, err, "the guide names %s, which the repository does not have", path)
	}
}

// The troubleshooting guide told the reader to look for the backend log lines
// "Token refresh successful" and "Token rotated: old_token → new_token", which
// nothing in the backend writes (#701). What it quotes now is what the backend
// writes: the access log line of the refresh route and a refresh warning.
func TestTroubleshootingGuideQuotesRefreshLinesTheBackendWrites(t *testing.T) {
	guide := readBackendFile(t, "../../docs/troubleshooting/authentication.md")
	section := guideSection(t, guide, "### Monitor Token Refresh Attempts")
	for _, line := range []string{"Token refresh successful", "Token rotated:", "Token refresh failed"} {
		textLacks(t, section, line, "the guide quotes a line the backend does not write")
	}

	// The access log line: the logger writes the method and path last, and the
	// refresh route is POST /api/v1/auth/refresh.
	textHas(t, section, "`POST /api/v1/auth/refresh`", "the guide quotes the refresh route's access log line")
	textHas(t, readBackendFile(t, "internal/interfaces/http/middleware/logger.go"),
		`${status} - ${latency} ${method} ${redactedPath}\n"`, "the access log line ends with the method and path")
	main := readBackendFile(t, "cmd/server/main.go")
	textHas(t, main, `v1 := app.Group("/api/v1")`, "the v1 routes are under /api/v1")
	textHas(t, main, `auth := v1.Group("/auth")`, "the auth routes are under /api/v1/auth")
	textHas(t, main, `auth.Post("/refresh", h.AuthRefresh.RefreshToken)`, "the refresh route")

	// The warnings are written with the log package, to stderr, which a
	// pipe from docker logs reads only with 2>&1.
	textHas(t, section, `docker logs -f aim-backend 2>&1 | grep -E "/api/v1/auth/refresh|Refresh:"`, "the guide's command reads stderr")
	handler := readBackendFile(t, "internal/interfaces/http/handlers/auth_refresh_handler.go")
	quoted := regexp.MustCompile("`(Refresh: [^`]+)`").FindAllStringSubmatch(oneLine(section), -1)
	require.NotEmpty(t, quoted, "the guide quotes a refresh warning")
	for _, q := range quoted {
		format := strings.ReplaceAll(q[1], "<id>", "%s")
		textHas(t, handler, `log.Printf("⚠️  `+format, "the refresh handler writes the warning the guide quotes")
	}
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

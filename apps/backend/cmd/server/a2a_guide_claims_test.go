package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

var (
	// guideMethodCall captures the method a line of Python calls on an object:
	// "agent.attest_mcp(" gives attest_mcp.
	guideMethodCall = regexp.MustCompile(`\.([A-Za-z_]\w*)\(`)
	// pythonDef captures the name of a Python function or method definition.
	pythonDef = regexp.MustCompile(`(?m)^\s*def ([A-Za-z_]\w*)\(`)
	// attestPayloadKey captures a key of the attestation dict attest_mcp signs.
	attestPayloadKey = regexp.MustCompile(`(?m)^\s*"(\w+)":`)
	// attenuationDiagramLine captures the label and the figure of one line of
	// the guide's attenuation diagram: "subdelegation-1  effectiveTrust 0.8".
	attenuationDiagramLine = regexp.MustCompile(`(?m)^\s*(root|subdelegation-\d+)\s+effectiveTrust (\d+(?:\.\d+)?)`)
	// serverDiagramRoot and serverDiagramLine capture the root delegator's trust
	// score and each delegation's figure in the guide's diagram of a server
	// chain verification: "delegation 1   effectiveTrust 0.72".
	serverDiagramRoot = regexp.MustCompile(`(?m)^agent A\s+trustScore (\d+(?:\.\d+)?)$`)
	serverDiagramLine = regexp.MustCompile(`(?m)^\s+delegation (\d+)\s+effectiveTrust (\d+(?:\.\d+)?)`)

	tsRootTrustDefault   = regexp.MustCompile(`let effectiveTrust = options\?\.rootTrustScore \?\? (\d+(?:\.\d+)?);`)
	tsAttenuationDefault = regexp.MustCompile(`const attenuation = options\?\.trustAttenuation \?\? (\d+(?:\.\d+)?);`)
	tsMinTrustDefault    = regexp.MustCompile(`const minTrust = options\?\.minDelegatedTrust \?\? (\d+(?:\.\d+)?);`)
	// tsChainTrustRead matches a read of either attenuation field from anything
	// but the verifier's options: a delegation's own fields are unsigned.
	tsChainTrustRead     = regexp.MustCompile(`[^?]\.(?:trustAttenuation|minDelegatedTrust)\b`)
	sqlAttenuationColumn = regexp.MustCompile(`trust_attenuation_factor DOUBLE PRECISION NOT NULL DEFAULT (\d+(?:\.\d+)?);`)
	sqlMinTrustColumn    = regexp.MustCompile(`min_delegated_trust_score DOUBLE PRECISION NOT NULL DEFAULT (\d+(?:\.\d+)?);`)
)

// guideSubsection returns the text from the third-level heading `heading` to
// the next heading of the same or a higher level.
func guideSubsection(t *testing.T, guide, heading string) string {
	t.Helper()
	start := strings.Index(guide, "\n"+heading+"\n")
	require.NotEqual(t, -1, start, "the guide has no %q heading", heading)
	rest := guide[start+1+len(heading):]
	end := len(rest)
	for _, next := range []string{"\n### ", "\n## "} {
		if i := strings.Index(rest, next); i != -1 && i < end {
			end = i
		}
	}
	return heading + rest[:end]
}

// pythonMethodBody returns a method of a Python class from its def line to the
// next def at the same indentation.
func pythonMethodBody(t *testing.T, src, name string) string {
	t.Helper()
	marker := "\n    def " + name + "("
	start := strings.Index(src, marker)
	require.NotEqual(t, -1, start, "no method %q", name)
	rest := src[start+1:]
	if end := strings.Index(rest[len(marker):], "\n    def "); end != -1 {
		return rest[:len(marker)+end]
	}
	return rest
}

// submatch returns the first capture of re in src, failing the test if there is none.
func submatch(t *testing.T, re *regexp.Regexp, src, what string) string {
	t.Helper()
	m := re.FindStringSubmatch(src)
	require.NotNil(t, m, "%s: %s not found", what, re)
	return m[1]
}

// repoFilesContaining returns the files under dir, relative to the repository
// root, whose name ends in ext and whose text matches re.
func repoFilesContaining(t *testing.T, dir, ext string, re *regexp.Regexp) []string {
	t.Helper()
	root := aim03RepoRoot(t)
	var hits []string
	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ext) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if re.Match(b) {
			rel, _ := filepath.Rel(root, path)
			hits = append(hits, rel)
		}
		return nil
	})
	require.NoError(t, err)
	return hits
}

// backendDelegationTrustReaders returns one line per place the backend's Go
// code, tests aside, uses a delegation's attenuation factor or trust floor:
// a selector on the domain fields, or a string that names their columns. The
// field declarations themselves are not uses.
func backendDelegationTrustReaders(t *testing.T) []string {
	t.Helper()
	root := aim03RepoRoot(t)
	backend := filepath.Join(root, "apps", "backend")
	fields := map[string]bool{"TrustAttenuationFactor": true, "MinDelegatedTrustScore": true}
	var uses []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if fields[x.Sel.Name] {
					uses = append(uses, fmt.Sprintf("%s: .%s", fset.Position(x.Pos()), x.Sel.Name))
				}
			case *ast.KeyValueExpr:
				if k, ok := x.Key.(*ast.Ident); ok && fields[k.Name] {
					uses = append(uses, fmt.Sprintf("%s: %s:", fset.Position(x.Pos()), k.Name))
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING && (strings.Contains(x.Value, "trust_attenuation_factor") ||
					strings.Contains(x.Value, "min_delegated_trust_score")) {
					uses = append(uses, fmt.Sprintf("%s: %s", fset.Position(x.Pos()), x.Value))
				}
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	sort.Strings(uses)
	return uses
}

// TestA2AGuideAttestationIsAnExplicitCall pins what docs/guides/A2A.md says
// about attesting an MCP server to the code that does it. The guide said the
// SDK attests automatically the first time an agent uses an MCP server, and
// showed a perform_action decorator doing it; perform_action never attests.
// Its manual example called AIMClient.from_sdk_token() and
// client.create_attestation(), neither of which the Python SDK defines. The
// guide now names the call, attest_mcp(), the routes it signs against, the
// fields it signs, and the one SDK method that attests on its own.
func TestA2AGuideAttestationIsAnExplicitCall(t *testing.T) {
	requirePublicRepositoryFiles(t)

	guide := aim03ReadRepoFile(t, "docs/guides/A2A.md")
	section := guideSubsection(t, guide, "### Attesting an MCP Server")

	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	srcContains(t, mainSrc, `mcpServersAgentAuth.Get("/:id/challenge", middleware.AuthMiddleware(jwtService), h.MCPAttestation.GetAttestationChallenge)`,
		"the server issues the attestation challenge")
	srcContains(t, mainSrc, `mcpServersAgentAuth.Post("/:id/attest", middleware.AuthMiddleware(jwtService), h.MCPAttestation.AttestMCP)`,
		"the server takes an agent's signed attestation")
	srcContains(t, mainSrc, `mcpServers.Post("/:id/manual-attest", middleware.MemberMiddleware(), h.MCPAttestation.ManualAttestMCP)`,
		"a member attests without an agent key")
	srcContains(t, section, "`GET /api/v1/mcp-servers/{id}/challenge`", "the guide names the challenge route")
	srcContains(t, section, "`POST /api/v1/mcp-servers/{id}/attest`", "the guide names the attest route")
	srcContains(t, section, "`POST /api/v1/mcp-servers/{id}/manual-attest`", "the guide names the manual route")

	client := aim03ReadRepoFile(t, "sdk/python/aim_sdk/client.py")
	attest := pythonMethodBody(t, client, "attest_mcp")
	srcContains(t, attest, `/api/v1/mcp-servers/{mcp_server_id}/challenge`, "attest_mcp requests a challenge")
	srcContains(t, attest, `/api/v1/mcp-servers/{mcp_server_id}/attest`, "attest_mcp submits to the attest route")
	srcContains(t, attest, "if not self.signing_key:\n            raise ConfigurationError(",
		"attest_mcp refuses an agent with no signing key")
	srcContains(t, section, "An agent registered in API-key mode has no signing key and cannot attest",
		"the guide says an API-key agent cannot attest")

	payloadStart := strings.Index(attest, "attestation_payload = {")
	require.NotEqual(t, -1, payloadStart, "attest_mcp builds attestation_payload")
	payload := attest[payloadStart : payloadStart+strings.Index(attest[payloadStart:], "\n            }")]
	var keys []string
	for _, m := range attestPayloadKey.FindAllStringSubmatch(payload, -1) {
		keys = append(keys, m[1])
	}
	assert.ElementsMatch(t, []string{"agentId", "mcpUrl", "mcpName", "capabilitiesFound", "connectionSuccessful",
		"healthCheckPassed", "connectionLatencyMs", "timestamp", "sdkVersion", "challenge"}, keys,
		"the guide lists what the signed attestation carries; update that sentence with this list")
	srcContains(t, section, "The signed attestation carries the agent ID, the server's URL and name, the tools the agent found, "+
		"whether the connection and the health check succeeded, the connection latency, a timestamp, the SDK version and the challenge.",
		"the guide lists what the signed attestation carries")

	srcNotContains(t, pythonMethodBody(t, client, "perform_action"), "attest",
		"perform_action creates no attestation")
	srcContains(t, section, "`@agent.perform_action` does not attest", "the guide says perform_action does not attest")
	srcNotContains(t, guide, "### Auto-Attestation", "the guide must not promise attestation on first use")
	srcNotContains(t, guide, "automatically creates an attestation", "the guide must not promise attestation on first use")

	useTool := pythonMethodBody(t, client, "use_mcp_tool")
	srcContains(t, useTool, "auto_attest: bool = True,", "use_mcp_tool attests unless told not to")
	srcContains(t, section, "Pass `auto_attest=False` to turn this off.", "the guide names the switch")
	cache := aim03ReadRepoFile(t, "sdk/python/aim_sdk/attestation_cache.py")
	// should_attest answers shouldAttest true for exactly these reasons; the
	// guide names each one.
	var reasons []string
	for _, m := range regexp.MustCompile(`"shouldAttest": True,\s*"reason": "(\w+)"`).FindAllStringSubmatch(cache, -1) {
		reasons = append(reasons, m[1])
	}
	assert.ElementsMatch(t, []string{"first_use", "new_tool", "stale", "invalid_timestamp", "capability_drift"}, reasons,
		"the guide lists when use_mcp_tool attests; update that sentence with this list")
	ttl := submatch(t, regexp.MustCompile(`DEFAULT_TTL_HOURS = (\d+)`), cache, "the attestation cache's lifetime")
	srcContains(t, section, "it attests on the agent's first use of the server, on the first use of a tool it has neither "+
		"used nor attested, when its last attestation is more than "+ttl+" hours old or its recorded time cannot be read, "+
		"and when discovery finds that the server's tools have changed.",
		"the guide lists when use_mcp_tool attests")

	java := aim03ReadRepoFile(t, "sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java")
	srcContains(t, java, "public Map<String, Object> attestMcp(", "the Java SDK has attestMcp")
	srcContains(t, java, `"/challenge?agent_id="`, "attestMcp requests a challenge")
	srcContains(t, java, `"/attest"`, "attestMcp submits to the attest route")
	srcContains(t, section, "`AIMClient.attestMcp()`", "the guide names the Java call")

	srcContains(t, aim03ReadRepoFile(t, "apps/web/lib/api.ts"), "/manual-attest`",
		"the dashboard's client posts to the manual route")
	srcContains(t, aim03ReadRepoFile(t, "apps/web/components/mcp/manual-attestation-form.tsx"), "api.manualAttestMCP(",
		"the dashboard's attestation form calls it")
	srcContains(t, aim03ReadRepoFile(t, "apps/web/app/dashboard/mcp/[id]/page.tsx"), "<ManualAttestationForm",
		"the MCP server's page shows the form")

	// Every method the section's Python examples call is one the SDK defines.
	defined := map[string]bool{}
	for _, rel := range repoFilesContaining(t, "sdk/python/aim_sdk", ".py", pythonDef) {
		for _, m := range pythonDef.FindAllStringSubmatch(aim03ReadRepoFile(t, rel), -1) {
			defined[m[1]] = true
		}
	}
	var calls int
	for _, block := range fencedBlock.FindAllStringSubmatch(section, -1) {
		if block[1] != "python" {
			continue
		}
		srcNotContains(t, block[2], "perform_action", "an attestation example must not show perform_action")
		for _, m := range guideMethodCall.FindAllStringSubmatch(block[2], -1) {
			calls++
			assert.True(t, defined[m[1]], "the guide calls %s(), which the Python SDK does not define", m[1])
		}
	}
	assert.Equal(t, 2, calls, "the section's Python examples call attest_mcp() and use_mcp_tool()")
}

// TestA2AGuideDelegationAttenuationNamesWhereItApplies pins what
// docs/guides/A2A.md says about delegation trust attenuation to the code that
// applies it. The server applies each stored delegation's
// trustAttenuationFactor and minDelegatedTrustScore when it verifies a chain
// (POST /api/v1/a2a/delegations/verify); the fields are read by that
// verification and by the repository, service and handler that store, default
// and audit them, and by no other server code. The TypeScript SDK's
// verifyDelegationChain() attenuates a chain of SDK delegations under other
// field names, and its first delegation keeps the root trust. It takes the
// factor and the floor from the verifier's options and never from a
// delegation, whose fields of those names are outside its signature, and it
// refuses a chain that carries one of them when the verifier left the
// matching option unset. A server change that reads the fields anywhere else
// fails here until the guide says what it does.
func TestA2AGuideDelegationAttenuationNamesWhereItApplies(t *testing.T) {
	requirePublicRepositoryFiles(t)

	guide := aim03ReadRepoFile(t, "docs/guides/A2A.md")
	section := guideSubsection(t, guide, "### Delegation Trust Attenuation")

	repoRoot := aim03RepoRoot(t)
	readerFiles := map[string]bool{}
	for _, use := range backendDelegationTrustReaders(t) {
		rel, err := filepath.Rel(repoRoot, use[:strings.Index(use, ".go:")+len(".go")])
		require.NoError(t, err)
		readerFiles[filepath.ToSlash(rel)] = true
	}
	assert.Equal(t, map[string]bool{
		"apps/backend/internal/domain/a2a_delegation_chain.go":                         true,
		"apps/backend/internal/application/a2a_delegation_service.go":                  true,
		"apps/backend/internal/infrastructure/repository/a2a_delegation_repository.go": true,
		"apps/backend/internal/interfaces/http/handlers/a2a_delegation_handler.go":     true,
	}, readerFiles, "the backend reads a delegation's attenuation fields somewhere new; say what it does in the guide's Delegation Trust Attenuation section")
	srcContains(t, section, "The AIM server applies trust attenuation when it verifies a chain of the delegations it stores",
		"the guide says the server attenuates")
	srcContains(t, section, "a chain verification reads both from every delegation in the chain", "the guide says the stored values are read")
	srcContains(t, section, "`POST /api/v1/a2a/delegations/verify`", "the guide names the verification route")
	srcContains(t, section, "`POST /api/v1/a2a/delegations`:", "the guide names the grant route")
	srcContains(t, section, "A manager or admin of the organization grants a delegation", "the grant route is manager-gated")
	srcContains(t, section, "Every delegation, the first one included, multiplies it by its own trust attenuation factor",
		"the guide says every server delegation attenuates")
	srcContains(t, section, "so a later delegation cannot lower a floor an earlier one set", "the guide says the floor carries forward")
	srcContains(t, section, "the result is rounded to six decimal places", "the guide says the trust is rounded")
	srcContains(t, section, fmt.Sprintf("`trustAttenuationFactor` (default %s) and `minDelegatedTrustScore` (default %s) are numbers from 0 to 1",
		strconv.FormatFloat(domain.DefaultA2ADelegationTrustAttenuationFactor, 'f', -1, 64),
		strconv.FormatFloat(domain.DefaultA2ADelegationMinTrustScore, 'f', -1, 64)),
		"the guide gives the server's defaults")
	srcContains(t, section, fmt.Sprintf("`maxChainDepth` (default %d) is from 1 to %d",
		domain.DefaultA2ADelegationMaxChainDepth, domain.MaxA2ADelegationChainLength), "the guide gives the depth's default and range")
	srcContains(t, section, fmt.Sprintf("from 1 to %d of them with none repeated", domain.MaxA2ADelegationChainLength),
		"the guide gives the chain's length limit")
	srcContains(t, section, fmt.Sprintf("names at most %d skills", application.MaxA2ADelegationSkills), "the guide gives the skill limit")
	srcNotContains(t, section, "Combined with per-capability trust thresholds",
		"the server does not weigh delegated trust in capability checks")
	srcContains(t, section, "no other server check reads a chain's effective trust", "the guide says the verdict is not used elsewhere")

	// The server diagram's figures are the ones VerifyA2ADelegationChain
	// reports for that root trust score and delegations at the defaults.
	serverRoot, err := strconv.ParseFloat(submatch(t, serverDiagramRoot, section, "the server diagram's root trust score"), 64)
	require.NoError(t, err)
	serverLines := serverDiagramLine.FindAllStringSubmatch(section, -1)
	require.GreaterOrEqual(t, len(serverLines), 3, "the guide shows the server's trust at three or more delegations")
	chain := make([]*domain.A2ADelegation, len(serverLines))
	agents := map[uuid.UUID]*domain.Agent{}
	agentIDs := make([]uuid.UUID, len(serverLines)+1)
	for i := range agentIDs {
		agentIDs[i] = uuid.New()
		agents[agentIDs[i]] = &domain.Agent{ID: agentIDs[i], Status: domain.AgentStatusVerified}
	}
	agents[agentIDs[0]].TrustScore = serverRoot
	for i := range chain {
		chain[i] = &domain.A2ADelegation{
			DelegatorAgentID: agentIDs[i], DelegateAgentID: agentIDs[i+1],
			MaxChainDepth:          domain.MaxA2ADelegationChainLength,
			TrustAttenuationFactor: domain.DefaultA2ADelegationTrustAttenuationFactor,
			MinDelegatedTrustScore: domain.DefaultA2ADelegationMinTrustScore,
			CanDelegateFurther:     true, IsActive: true,
		}
	}
	verdict := domain.VerifyA2ADelegationChain(chain, agents, "", time.Now())
	for i, m := range serverLines {
		assert.Equal(t, strconv.Itoa(i+1), m[1], "server diagram line %d", i)
		assert.Equal(t, strconv.FormatFloat(verdict.Hops[i].EffectiveTrust, 'f', -1, 64), m[2],
			"server diagram line %d; VerifyA2ADelegationChain reports %v", i, verdict.Hops[i].EffectiveTrust)
	}

	migration := aim03ReadRepoFile(t, "apps/backend/migrations/077_add_trust_attenuation_to_a2a_delegation.sql")
	columns := regexp.MustCompile(`(?i)\b(trust_attenuation_factor|min_delegated_trust_score)\b`)
	assert.Equal(t, []string{"apps/backend/migrations/077_add_trust_attenuation_to_a2a_delegation.sql"},
		repoFilesContaining(t, "apps/backend/migrations", ".sql", columns), "one migration defines the columns")
	srcContains(t, section, fmt.Sprintf("a trust attenuation factor (default %s) and a minimum delegated trust score (default %s)",
		submatch(t, sqlAttenuationColumn, migration, "the attenuation column's default"),
		submatch(t, sqlMinTrustColumn, migration, "the floor column's default")),
		"the guide gives the server's stored defaults")

	attenuation := regexp.MustCompile(`(?i)attenuation`)
	assert.Empty(t, repoFilesContaining(t, "sdk/python/aim_sdk", ".py", attenuation), "the Python SDK has no attenuation")
	assert.Empty(t, repoFilesContaining(t, "sdk/java/src/main", ".java", attenuation), "the Java SDK has no attenuation")
	srcContains(t, section, "Neither the Python SDK nor the Java SDK applies attenuation", "the guide says so")

	ts := aim03ReadRepoFile(t, "sdk/typescript/src/crypto/delegation.ts")
	srcContains(t, aim03ReadRepoFile(t, "sdk/typescript/src/index.ts"), "verifyDelegationChain,",
		"the TypeScript SDK exports verifyDelegationChain")
	srcContains(t, ts, "trustAttenuation?: number;", "a delegation and the verifier's options carry trustAttenuation")
	srcContains(t, ts, "minDelegatedTrust?: number;", "a delegation and the verifier's options carry minDelegatedTrust")
	verifyStart := strings.Index(ts, "export async function verifyDelegationChain(")
	require.NotEqual(t, -1, verifyStart, "the SDK has verifyDelegationChain")
	verify := ts[verifyStart:]
	verifyEnd := strings.Index(verify, "\nexport ")
	require.NotEqual(t, -1, verifyEnd, "verifyDelegationChain is followed by another export")
	verify = verify[:verifyEnd]
	assert.Empty(t, tsChainTrustRead.FindAllString(verify, -1),
		"verifyDelegationChain takes the factor and the floor from its options, never from a delegation's unsigned fields")
	srcContains(t, section, "`verifyDelegationChain()` does not read them", "the guide says the delegation's own fields are not read")
	// The one function that looks at a delegation's own fields tests only
	// whether they are there, to refuse the chain, and never takes their values.
	carriedStart := strings.Index(ts, "function carriedTrustFieldsWithoutOption(")
	require.NotEqual(t, -1, carriedStart, "the SDK has carriedTrustFieldsWithoutOption")
	carried := ts[carriedStart:]
	carriedEnd := strings.Index(carried, "\n}\n")
	require.NotEqual(t, -1, carriedEnd, "carriedTrustFieldsWithoutOption ends")
	carried = carried[:carriedEnd]
	assert.Len(t, tsChainTrustRead.FindAllString(carried, -1), 2,
		"carriedTrustFieldsWithoutOption reads each delegation field once, to test whether it is set")
	srcContains(t, carried, "chain.some((d) => isSet(d.trustAttenuation))", "the factor on a delegation is only tested for presence")
	srcContains(t, carried, "chain.some((d) => isSet(d.minDelegatedTrust))", "the floor on a delegation is only tested for presence")
	srcContains(t, ts, "pass ${fields} in the options to verify this chain", "a chain carrying a field whose option is unset is refused")
	srcContains(t, section, "A chain in which any delegation carries `trustAttenuation` fails unless the verifier passes `trustAttenuation` in the options",
		"the guide says a chain carrying a field whose option is unset fails")
	srcContains(t, ts, "refusedChain('rootTrustScore must be a number from 0 to 1')", "the root trust is held to 0 to 1")
	srcContains(t, section, "and so must `rootTrustScore`", "the guide says the root trust is held to 0 to 1")
	srcContains(t, section, "takes two optional settings from its caller, the verifier, in its options", "the guide says the verifier sets both")
	srcContains(t, ts, "effectiveTrust = effectiveTrust * attenuation;", "each later delegation multiplies the trust")
	srcContains(t, ts, "if (effectiveTrust < minTrust) {", "a delegation below the floor fails")
	srcContains(t, ts, "const label = i === 0 ? 'root' : `subdelegation-${i}`;", "the delegations' labels")
	rootTrust := submatch(t, tsRootTrustDefault, ts, "rootTrustScore's default")
	factor := submatch(t, tsAttenuationDefault, ts, "trustAttenuation's default")
	floor := submatch(t, tsMinTrustDefault, ts, "minDelegatedTrust's default")
	srcContains(t, section, "`trustAttenuation` (default "+factor+")", "the guide gives trustAttenuation's default")
	srcContains(t, section, "`minDelegatedTrust` (default "+floor+")", "the guide gives minDelegatedTrust's default")
	srcContains(t, section, "`rootTrustScore` (default "+rootTrust+")", "the guide gives rootTrustScore's default")

	srcContains(t, ts, "const EXCLUDED_FROM_SIGNING = new Set(['signature', 'publicKey', 'trustAttenuation', 'minDelegatedTrust']);",
		"the attenuation fields are left out of the signed payload")
	srcContains(t, section, "the SDK leaves both out of the signed payload", "the guide says the fields are unsigned")

	createStart := strings.Index(ts, "export async function createDelegation(")
	require.NotEqual(t, -1, createStart, "the SDK has createDelegation")
	create := ts[createStart:]
	createEnd := strings.Index(create, "\nexport ")
	require.NotEqual(t, -1, createEnd, "createDelegation is followed by another export")
	create = create[:createEnd]
	srcNotContains(t, create, "trustAttenuation", "createDelegation does not set trustAttenuation")
	srcNotContains(t, create, "minDelegatedTrust", "createDelegation does not set minDelegatedTrust")
	srcContains(t, section, "`createDelegation()` sets neither field", "the guide says createDelegation sets neither")

	// The diagram's figures are the ones verifyDelegationChain reports with the
	// defaults: the root keeps the root trust, and each later delegation
	// multiplies the previous figure by the factor.
	root, err := strconv.ParseFloat(rootTrust, 64)
	require.NoError(t, err)
	f, err := strconv.ParseFloat(factor, 64)
	require.NoError(t, err)
	lines := attenuationDiagramLine.FindAllStringSubmatch(section, -1)
	require.GreaterOrEqual(t, len(lines), 3, "the guide shows the trust at three or more delegations")
	for i, m := range lines {
		label := "root"
		if i > 0 {
			label = fmt.Sprintf("subdelegation-%d", i)
		}
		assert.Equal(t, label, m[1], "diagram line %d", i)
		got, err := strconv.ParseFloat(m[2], 64)
		require.NoError(t, err)
		want := root * math.Pow(f, float64(i))
		assert.InDelta(t, want, got, 0.0005, "diagram line %d gives %s; verifyDelegationChain reports %.4f", i, m[2], want)
	}
}

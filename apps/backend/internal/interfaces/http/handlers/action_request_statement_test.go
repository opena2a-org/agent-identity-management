package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// statementSigner builds signed action-request statements for tests.
type statementSigner struct {
	agentID uuid.UUID
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
}

func newStatementSigner(t *testing.T) *statementSigner {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &statementSigner{agentID: uuid.New(), pub: pub, priv: priv}
}

// registeredKey is the key as AIM stores it on the agent row.
func (s *statementSigner) registeredKey() *string {
	k := base64.StdEncoding.EncodeToString(s.pub)
	return &k
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func newTestNonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, domain.ActionRequestNonceBytes)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b64url(b)
}

// paeFrame is the DSSE v1 pre-authentication encoding.
func paeFrame(payloadType string, payload []byte) []byte {
	return append([]byte(fmt.Sprintf("DSSEv1 %d %s %d ", len(payloadType), payloadType, len(payload))), payload...)
}

// payload is a statement payload with every required member; extra is
// appended verbatim before the closing brace.
func (s *statementSigner) payload(t *testing.T, extra string) string {
	t.Helper()
	return fmt.Sprintf(`{"action_type":"db:read","agent_id":"%s","resource":"orders","timestamp":"%s","nonce":"%s"%s}`,
		s.agentID, time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"), newTestNonce(t), extra)
}

// body signs the frame of payload and returns the request body.
func (s *statementSigner) body(frame []byte) []byte {
	return s.bodyWith(frame, ed25519.Sign(s.priv, frame), s.pub)
}

func (s *statementSigner) bodyWith(frame, sig, pub []byte) []byte {
	b, _ := json.Marshal(map[string]string{
		"signedBytes": b64url(frame),
		"signature":   b64url(sig),
		"publicKey":   b64url(pub),
	})
	return b
}

func (s *statementSigner) statement(payload string) []byte {
	return s.body(paeFrame(domain.ActionRequestPayloadType, []byte(payload)))
}

// parseStatementBody runs dispatch and step 1 on a raw body.
func parseStatementBody(t *testing.T, body []byte) (*actionRequestStatement, *actionRequestRefusal) {
	t.Helper()
	members, err := readJSONObjectMembers(body)
	require.NoError(t, err, "test body must be one JSON object")
	require.True(t, members.has("signedBytes"), "test body must name signedBytes")
	return parseActionRequestStatement(body, members)
}

func requireRefusal(t *testing.T, refusal *actionRequestRefusal, status int, code, message, limit string) {
	t.Helper()
	require.NotNil(t, refusal, "expected a refusal")
	assert.Equal(t, status, refusal.status)
	assert.Equal(t, code, refusal.reasonCode)
	assert.Equal(t, message, refusal.message)
	assert.Equal(t, limit, refusal.limit)
}

func invalidLine(t *testing.T, refusal *actionRequestRefusal, message string) {
	t.Helper()
	requireRefusal(t, refusal, 400, "invalidRequest", message, "")
}

func TestActionRequestStatement_ValidStatementParses(t *testing.T) {
	s := newStatementSigner(t)
	stmt, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"context":{"k":"v"},"risk_level":"low"`)))
	require.Nil(t, refusal)
	assert.Equal(t, s.agentID, stmt.agentID)
	assert.Equal(t, "db:read", stmt.actionType)
	assert.Equal(t, "orders", stmt.resource)
	assert.Equal(t, "low", stmt.riskLevel)
	assert.Len(t, stmt.nonce, 16)
	assert.Equal(t, map[string]interface{}{"k": "v"}, stmt.context)
}

// Context values are recorded with the digits and characters they were signed
// with: no number passes through a float.
func TestActionRequestStatement_ContextNumbersAndTextAreExactlyAsSigned(t *testing.T) {
	s := newStatementSigner(t)
	stmt, refusal := parseStatementBody(t, s.statement(s.payload(t,
		`,"context":{"big":9007199254740993,"one":1.0,"word":"café"}`)))
	require.Nil(t, refusal)
	assert.Equal(t, json.Number("9007199254740993"), stmt.context["big"])
	assert.Equal(t, json.Number("1.0"), stmt.context["one"])
	assert.Equal(t, "café", stmt.context["word"])

	recorded, err := json.Marshal(stmt.context)
	require.NoError(t, err)
	assert.Equal(t, `{"big":9007199254740993,"one":1.0,"word":"café"}`, string(recorded))
}

func TestActionRequestStatement_ResourceMayBeNull(t *testing.T) {
	s := newStatementSigner(t)
	payload := strings.Replace(s.payload(t, ""), `"resource":"orders"`, `"resource":null`, 1)
	stmt, refusal := parseStatementBody(t, s.statement(payload))
	require.Nil(t, refusal)
	assert.Equal(t, "", stmt.resource)
}

// One cell per step-1 line, in check order.
func TestActionRequestStatement_BodyLines(t *testing.T) {
	s := newStatementSigner(t)
	frame := paeFrame(domain.ActionRequestPayloadType, []byte(s.payload(t, "")))
	sig := b64url(ed25519.Sign(s.priv, frame))

	t.Run("body repeats a member name", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":"%s","signature":"%s","publicKey":"%s"}`,
			b64url(frame), sig, sig, b64url(s.pub))
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "the body repeats a member name")
	})
	t.Run("extra member", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":"%s","publicKey":"%s","nonce":"x"}`,
			b64url(frame), sig, b64url(s.pub))
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "the body must have exactly the members signedBytes, signature and publicKey")
	})
	t.Run("missing member", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":"%s"}`, b64url(frame), sig)
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "the body must have exactly the members signedBytes, signature and publicKey")
	})
	t.Run("member that is not a string", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":12,"publicKey":"%s"}`, b64url(frame), b64url(s.pub))
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "signature is not unpadded base64url")
	})
	for _, tc := range []struct{ member, value string }{
		{"signedBytes", b64url(frame) + "="},
		{"signedBytes", strings.Replace(b64url(frame), "R", "+", 1)},
		{"signature", sig[:40] + "\n" + sig[40:]},
		{"signature", sig[:len(sig)-1] + "B"},   // 86 chars: the last carries 4 trailing bits
		{"publicKey", b64url(s.pub)[:42] + "B"}, // 43 chars: the last carries 2 trailing bits
	} {
		t.Run(tc.member+" not unpadded base64url", func(t *testing.T) {
			values := map[string]string{"signedBytes": b64url(frame), "signature": sig, "publicKey": b64url(s.pub)}
			values[tc.member] = tc.value
			body, _ := json.Marshal(values)
			_, refusal := parseStatementBody(t, body)
			invalidLine(t, refusal, tc.member+" is not unpadded base64url")
		})
	}
}

func TestActionRequestStatement_FrameLine(t *testing.T) {
	s := newStatementSigner(t)
	payload := []byte(s.payload(t, ""))
	typ := domain.ActionRequestPayloadType
	line := "signedBytes is not a DSSE v1 frame of type application/vnd.opena2a.action-request.v1+json"

	frames := map[string][]byte{
		"S1 canonical string":     []byte("POST\n/api/v1/sdk-api/verifications\n1759600000\n" + string(payload)),
		"revocation type":         paeFrame("application/vnd.opena2a.revocation.v1+json", payload),
		"delegation hop type":     paeFrame("application/vnd.opena2a.delegation.v1+json", payload),
		"type with a suffix":      paeFrame(typ+"x", payload),
		"type length leading 0":   []byte(fmt.Sprintf("DSSEv1 046 %s %d %s", typ, len(payload), payload)),
		"payload length signed":   []byte(fmt.Sprintf("DSSEv1 46 %s +%d %s", typ, len(payload), payload)),
		"payload length leading0": []byte(fmt.Sprintf("DSSEv1 46 %s 0%d %s", typ, len(payload), payload)),
		"length with six digits":  []byte(fmt.Sprintf("DSSEv1 46 %s 100000 %s", typ, payload)),
		"trailing byte":           append(paeFrame(typ, payload), ' '),
		"short payload":           paeFrame(typ, payload)[:len(paeFrame(typ, payload))-1],
		"lowercase prefix":        []byte(strings.Replace(string(paeFrame(typ, payload)), "DSSEv1", "dssev1", 1)),
	}
	for name, frame := range frames {
		t.Run(name, func(t *testing.T) {
			_, refusal := parseStatementBody(t, s.body(frame))
			invalidLine(t, refusal, line)
		})
	}
}

func TestActionRequestStatement_PayloadLines(t *testing.T) {
	s := newStatementSigner(t)
	notOneObject := "the payload is not one JSON object in valid UTF-8 with no trailing bytes"
	valid := s.payload(t, "")

	for name, payload := range map[string]string{
		"byte before {":       " " + valid,
		"byte after }":        valid + " ",
		"two objects":         valid + "{}",
		"invalid UTF-8":       strings.Replace(valid, "orders", "ord\xffers", 1),
		"array":               "[" + valid + "]",
		"trailing comma":      strings.TrimSuffix(valid, "}") + ",}",
		"unterminated string": `{"action_type":"db:read}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, refusal := parseStatementBody(t, s.statement(payload))
			invalidLine(t, refusal, notOneObject)
		})
	}

	t.Run("member repeated at the top level", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"risk_level":"low","risk_level":"high"`)))
		invalidLine(t, refusal, "the payload repeats a member name")
	})
	t.Run("member repeated inside context", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"context":{"a":{"b":[{"c":1,"c":2}]}}`)))
		invalidLine(t, refusal, "the payload repeats a member name")
	})
	t.Run("member repeated through an escape", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"context":{"a":1,"a":2}`)))
		invalidLine(t, refusal, "the payload repeats a member name")
	})
	t.Run("member not defined", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"riskLevel":"low"`)))
		invalidLine(t, refusal, "the payload member riskLevel is not defined by action-request-v1")
	})
	t.Run("missing action_type", func(t *testing.T) {
		payload := strings.Replace(s.payload(t, ""), `"action_type":"db:read",`, "", 1)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload is missing action_type")
	})
	t.Run("missing resource", func(t *testing.T) {
		payload := strings.Replace(s.payload(t, ""), `"resource":"orders",`, "", 1)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload is missing resource")
	})
	t.Run("missing nonce", func(t *testing.T) {
		payload := fmt.Sprintf(`{"action_type":"db:read","agent_id":"%s","resource":null,"timestamp":"2026-10-05T12:00:00Z"}`, s.agentID)
		_, refusal := parseStatementBody(t, s.statement(payload))
		requireRefusal(t, refusal, 400, "nonceRequired", "the signed statement has no nonce", "")
	})
}

// nest wraps an object in depth-1 levels of context nesting, so the payload
// has the given depth counting the payload object as level 1.
func nest(depth int) string {
	inner := `1`
	for i := 0; i < depth-1; i++ {
		inner = `{"n":` + inner + `}`
	}
	return inner
}

func TestActionRequestStatement_DepthCeiling(t *testing.T) {
	s := newStatementSigner(t)

	// depth 32: the payload object, then "context" holding 31 levels.
	_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"context":`+nest(32))))
	require.Nil(t, refusal, "depth 32 is accepted")

	_, refusal = parseStatementBody(t, s.statement(s.payload(t, `,"context":`+nest(33))))
	requireRefusal(t, refusal, 422, "limitExceeded", "the payload nests deeper than 32 levels", "statementDepth")

	// Arrays count as levels too, and a nesting far past any recursive
	// walker's limit is refused, not crashed on.
	deep := strings.Repeat("[", 20000) + strings.Repeat("]", 20000)
	_, refusal = parseStatementBody(t, s.statement(s.payload(t, `,"context":{"a":`+deep+`}`)))
	requireRefusal(t, refusal, 422, "limitExceeded", "the payload nests deeper than 32 levels", "statementDepth")
}

// frameOfSize returns a frame of exactly n bytes whose payload is valid.
func (s *statementSigner) frameOfSize(t *testing.T, n int) []byte {
	t.Helper()
	base := s.payload(t, `,"context":{"pad":""}`)
	frame := paeFrame(domain.ActionRequestPayloadType, []byte(base))
	padding := n - len(frame)
	require.Positive(t, padding)
	// Growing the payload can add a digit to its length field.
	for {
		payload := strings.Replace(base, `"pad":""`, `"pad":"`+strings.Repeat("x", padding)+`"`, 1)
		frame = paeFrame(domain.ActionRequestPayloadType, []byte(payload))
		if len(frame) == n {
			return frame
		}
		padding -= len(frame) - n
	}
}

func TestActionRequestStatement_SizeCeiling(t *testing.T) {
	s := newStatementSigner(t)
	assert.Equal(t, 87382, domain.ActionRequestMaxSignedBytesChars)
	assert.Equal(t, 88406, domain.ActionRequestMaxRawBody)
	sizeLine := "signedBytes is longer than 87382 base64url characters (65536 bytes decoded)"

	atCeiling := s.body(s.frameOfSize(t, 65536))
	assert.Len(t, atCeiling, 87559, "the compact body of a 65,536-byte statement")
	_, refusal := parseStatementBody(t, atCeiling)
	require.Nil(t, refusal, "65,536 bytes are accepted")

	_, refusal = parseStatementBody(t, s.body(s.frameOfSize(t, 65537)))
	requireRefusal(t, refusal, 422, "limitExceeded", sizeLine, "statementBytes")

	// Over the ceiling and not even base64url: the size check runs first,
	// before any decoding.
	notDecodable := fmt.Sprintf(`{"signedBytes":"%s","signature":"x","publicKey":"y"}`, strings.Repeat("*", 87383))
	_, refusal = parseStatementBody(t, []byte(notDecodable))
	requireRefusal(t, refusal, 422, "limitExceeded", sizeLine, "statementBytes")

	// A raw body over its ceiling is refused before its members are read.
	padded := append([]byte(`{"signedBytes":"`+b64url(s.frameOfSize(t, 1000))+`","signature":"x","publicKey":"y"`),
		[]byte(strings.Repeat(" ", domain.ActionRequestMaxRawBody))...)
	padded = append(padded, '}')
	_, refusal = parseStatementBody(t, padded)
	requireRefusal(t, refusal, 422, "limitExceeded", sizeLine, "statementBytes")
}

func TestActionRequestStatement_MemberForms(t *testing.T) {
	s := newStatementSigner(t)
	form := func(member string) string {
		return member + " is not in the form action-request-v1 requires"
	}
	withMember := func(member, value string) string {
		p := s.payload(t, "")
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(p), &m))
		if _, ok := m[member]; ok {
			old, _ := json.Marshal(map[string]json.RawMessage{member: m[member]})
			repl := fmt.Sprintf(`{"%s":%s}`, member, value)
			return strings.Replace(p, strings.Trim(string(old), "{}"), strings.Trim(repl, "{}"), 1)
		}
		return strings.TrimSuffix(p, "}") + fmt.Sprintf(`,"%s":%s}`, member, value)
	}

	for _, tc := range []struct{ name, member, value string }{
		{"empty action_type", "action_type", `""`},
		{"numeric action_type", "action_type", `7`},
		{"agent_id without hyphens", "agent_id", `"` + strings.ReplaceAll(s.agentID.String(), "-", "") + `"`},
		{"agent_id urn", "agent_id", `"urn:uuid:` + s.agentID.String() + `"`},
		{"resource number", "resource", `3`},
		{"context array", "context", `[]`},
		{"context null", "context", `null`},
		{"timestamp now", "timestamp", `"now"`},
		{"timestamp infinity", "timestamp", `"infinity"`},
		{"timestamp epoch", "timestamp", `"epoch"`},
		{"timestamp with offset", "timestamp", `"2026-10-05T12:00:00+00:00"`},
		{"timestamp without zone", "timestamp", `"2026-10-05T12:00:00"`},
		{"timestamp month 13", "timestamp", `"2026-13-05T12:00:00Z"`},
		{"timestamp unix", "timestamp", `1759600000`},
		{"nonce 15 bytes", "nonce", `"` + b64url(make([]byte, 15)) + `"`},
		{"nonce 17 bytes", "nonce", `"` + b64url(make([]byte, 17)) + `"`},
		{"nonce padded", "nonce", `"` + base64.URLEncoding.EncodeToString(make([]byte, 16)) + `"`},
		{"nonce trailing bits", "nonce", `"AAAAAAAAAAAAAAAAAAAAAB"`},
		{"nonce malformed", "nonce", `"not a nonce at all!!!!"`},
		{"risk_level number", "risk_level", `1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, refusal := parseStatementBody(t, s.statement(withMember(tc.member, tc.value)))
			invalidLine(t, refusal, form(tc.member))
		})
	}

	digest := b64url(make([]byte, 32))
	for _, tc := range []struct{ name, value, line string }{
		{"null chain", `null`, "delegation_digests must be omitted or be a non-empty array of hop digests"},
		{"empty chain", `[]`, "delegation_digests must be omitted or be a non-empty array of hop digests"},
		{"string chain", `"` + digest + `"`, "delegation_digests must be omitted or be a non-empty array of hop digests"},
		{"short digest", `["` + digest + `","abc"]`, "delegation_digests[1]: not a 43-character unpadded base64url SHA-256 digest"},
		{"padded digest", `["` + digest + `="]`, "delegation_digests[0]: not a 43-character unpadded base64url SHA-256 digest"},
		{"number digest", `[1]`, "delegation_digests[0]: not a 43-character unpadded base64url SHA-256 digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, refusal := parseStatementBody(t, s.statement(withMember("delegation_digests", tc.value)))
			invalidLine(t, refusal, tc.line)
		})
	}

	stmt, refusal := parseStatementBody(t, s.statement(withMember("delegation_digests", `["`+digest+`","`+digest+`"]`)))
	require.Nil(t, refusal)
	assert.Equal(t, []string{digest, digest}, stmt.delegationDigests)
}

// With two failures planted, the earlier check names the refusal.
func TestActionRequestStatement_EarlierCheckWins(t *testing.T) {
	s := newStatementSigner(t)
	frame := paeFrame(domain.ActionRequestPayloadType, []byte(s.payload(t, "")))
	sig := b64url(ed25519.Sign(s.priv, frame))

	t.Run("body repeat before member set", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signedBytes":"%s","signature":"%s","extra":1}`, b64url(frame), b64url(frame), sig)
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "the body repeats a member name")
	})
	t.Run("member set before size", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":"%s"}`, strings.Repeat("A", 87383), sig)
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "the body must have exactly the members signedBytes, signature and publicKey")
	})
	t.Run("base64url before frame", func(t *testing.T) {
		body := fmt.Sprintf(`{"signedBytes":"%s","signature":"%s","publicKey":"%s="}`,
			b64url([]byte("not a frame")), sig, b64url(s.pub))
		_, refusal := parseStatementBody(t, []byte(body))
		invalidLine(t, refusal, "publicKey is not unpadded base64url")
	})
	t.Run("not one object before depth", func(t *testing.T) {
		payload := s.payload(t, `,"context":`+nest(40)) + " "
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload is not one JSON object in valid UTF-8 with no trailing bytes")
	})
	t.Run("depth before repeat", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"context":`+nest(40)+`,"context":{}`)))
		requireRefusal(t, refusal, 422, "limitExceeded", "the payload nests deeper than 32 levels", "statementDepth")
	})
	t.Run("repeat before undefined member", func(t *testing.T) {
		_, refusal := parseStatementBody(t, s.statement(s.payload(t, `,"extra":1,"risk_level":"a","risk_level":"b"`)))
		invalidLine(t, refusal, "the payload repeats a member name")
	})
	t.Run("undefined member before missing member", func(t *testing.T) {
		payload := strings.Replace(s.payload(t, `,"extra":1`), `"action_type":"db:read",`, "", 1)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload member extra is not defined by action-request-v1")
	})
	t.Run("missing member before form", func(t *testing.T) {
		payload := strings.Replace(s.payload(t, ""), `"action_type":"db:read",`, "", 1)
		payload = strings.Replace(payload, `"resource":"orders"`, `"resource":5`, 1)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload is missing action_type")
	})
	t.Run("missing member before missing nonce", func(t *testing.T) {
		payload := fmt.Sprintf(`{"agent_id":"%s","resource":null,"timestamp":"2026-10-05T12:00:00Z"}`, s.agentID)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "the payload is missing action_type")
	})
	t.Run("forms in member order", func(t *testing.T) {
		payload := strings.Replace(s.payload(t, ""), `"timestamp":"`, `"timestamp":"x`, 1)
		payload = strings.Replace(payload, `"agent_id":"`, `"agent_id":"x`, 1)
		_, refusal := parseStatementBody(t, s.statement(payload))
		invalidLine(t, refusal, "agent_id is not in the form action-request-v1 requires")
	})
}

// No refusal line on this path names the request-signing format of the S1
// headers, and every line is ASCII.
func TestActionRequestStatement_RefusalLinesNameOnlyThisFormat(t *testing.T) {
	lines := []string{
		legacyFreshMemberLine,
		statementBytesRefusal().message,
		agentKeyNotRecognized.message,
		statementSignatureInvalid.message,
		statementHybridRequired.message,
		statementBehindClock.message,
		statementAheadOfClock.message,
		statementNonceReused.message,
		statementFreshnessUnavailable.message,
		statementFormRefusal("nonce").message,
	}
	for _, line := range lines {
		assert.NotContains(t, line, "agent-request-v", line)
		for _, r := range line {
			assert.Less(t, r, rune(128), "non-ASCII in %q", line)
		}
	}
	assert.Equal(t, "Agent is in hybrid mode and action-request-v1 statements cannot be hybrid-signed",
		statementHybridRequired.message)
	assert.Equal(t, "request timestamp is more than 30 seconds behind the AIM database clock", statementBehindClock.message)
	assert.Equal(t, "request timestamp is more than 30 seconds ahead of the AIM database clock", statementAheadOfClock.message)
}

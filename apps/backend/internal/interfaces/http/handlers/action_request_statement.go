package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/agentauth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Signed action-request statements on POST /api/v1/verifications and
// POST /api/v1/sdk-api/verifications. A body with the top-level member
// signedBytes is a statement; any other body is the original request form,
// handled exactly as before.
//
// A statement is checked in this order, and a refusal names the first check
// that fails:
//
//  1. Shape: body, decoding, the DSSE frame and the payload. Reads no stored
//     state.
//  2. Lookup of the payload's agent_id, reading only its registered key, and
//     key match on decoded bytes. An unknown agent, an agent with no key, a
//     stored key that does not decode to 32 bytes and a different key get
//     one status and one byte-identical body.
//  3. Ed25519 signature over signedBytes as received, never rebuilt; then
//     the hybrid-mode gate. Nothing is written at steps 1-3.
//  4. and 5. Window and nonce admission, one statement on the database clock.
//     Its row is the request's first write.
//  6. Status gate. The nonce stays spent.
//  7. Everything else: the same capability decision and records as the
//     original form, from the signed payload only.

// Refusal reason codes of the statement path.
const (
	actionRequestInvalidRequestCode       = "invalidRequest"
	actionRequestLimitExceededCode        = "limitExceeded"
	actionRequestNonceRequiredCode        = "nonceRequired"
	actionRequestKeyNotRecognizedCode     = agentauth.KeyNotRecognizedReason
	actionRequestSignatureInvalidCode     = "signatureInvalid"
	actionRequestHybridRequiredCode       = "hybridSignatureRequired"
	actionRequestOutsideWindowCode        = "timestampOutsideWindow"
	actionRequestNonceReusedCode          = "nonceReused"
	actionRequestFreshnessUnavailableCode = "freshnessCheckUnavailable"
	actionRequestStatusDeniedCode         = "agentStatusDenied"

	actionRequestStatementBytesLimit = "statementBytes"
	actionRequestStatementDepthLimit = "statementDepth"
)

// legacyFreshMemberLine refuses an original-form body that carries a member
// only a signed statement defines. Unsigned, such a member would be a claim
// nothing checks.
const legacyFreshMemberLine = "nonce and delegationDigests are accepted only inside a signed statement (signedBytes)"

// actionRequestRefusal is one refusal: status, reasonCode, the error line
// and, for limitExceeded, which limit.
type actionRequestRefusal struct {
	status     int
	reasonCode string
	message    string
	limit      string
}

func (r *actionRequestRefusal) send(c fiber.Ctx) error {
	body := fiber.Map{"error": r.message, "reasonCode": r.reasonCode}
	if r.limit != "" {
		body["limit"] = r.limit
	}
	return c.Status(r.status).JSON(body)
}

func invalidStatement(format string, args ...any) *actionRequestRefusal {
	return &actionRequestRefusal{
		status:     fiber.StatusBadRequest,
		reasonCode: actionRequestInvalidRequestCode,
		message:    fmt.Sprintf(format, args...),
	}
}

// statementBytesRefusal is the size ceiling, checked before decoding.
func statementBytesRefusal() *actionRequestRefusal {
	return &actionRequestRefusal{
		status:     fiber.StatusUnprocessableEntity,
		reasonCode: actionRequestLimitExceededCode,
		message: fmt.Sprintf("signedBytes is longer than %d base64url characters (%d bytes decoded)",
			domain.ActionRequestMaxSignedBytesChars, domain.ActionRequestMaxSignedBytes),
		limit: actionRequestStatementBytesLimit,
	}
}

// The refusals after step 1. Each body is built from constants, so two
// requests refused for the same reason get byte-identical bodies.
var (
	agentKeyNotRecognized = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestKeyNotRecognizedCode,
		message:    agentauth.KeyNotRecognizedMessage,
	}
	statementSignatureInvalid = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestSignatureInvalidCode,
		message:    "the signature does not verify over signedBytes with this agent's registered key",
	}
	statementHybridRequired = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestHybridRequiredCode,
		message: "Agent is in hybrid mode and " + domain.ActionRequestSchemeID +
			" statements cannot be hybrid-signed",
	}
	statementBehindClock = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestOutsideWindowCode,
		message: fmt.Sprintf("request timestamp is more than %d seconds behind the AIM database clock",
			domain.ActionRequestWindowSeconds),
	}
	statementAheadOfClock = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestOutsideWindowCode,
		message: fmt.Sprintf("request timestamp is more than %d seconds ahead of the AIM database clock",
			domain.ActionRequestWindowSeconds),
	}
	statementNonceReused = &actionRequestRefusal{
		status:     fiber.StatusUnauthorized,
		reasonCode: actionRequestNonceReusedCode,
		message:    "this nonce was already used by this agent",
	}
	statementFreshnessUnavailable = &actionRequestRefusal{
		status:     fiber.StatusServiceUnavailable,
		reasonCode: actionRequestFreshnessUnavailableCode,
		message:    "cannot check this request's time and nonce against the AIM database; nothing was recorded",
	}
)

// actionRequestStatement is a statement that passed step 1.
type actionRequestStatement struct {
	signedBytes   []byte
	signature     []byte
	publicKey     []byte
	signatureText string
	publicKeyText string

	agentID           uuid.UUID
	actionType        string
	resource          string // "" when the payload's resource is null
	context           map[string]interface{}
	timestamp         time.Time
	nonce             []byte
	delegationDigests []string
	riskLevel         string
}

// jsonMembers is a JSON object's top-level members, read without decoding
// their values.
type jsonMembers struct {
	names  []string // in body order, each once
	values map[string]json.RawMessage
	dup    string // the first name that repeats, "" if none
}

func (m *jsonMembers) has(name string) bool {
	_, ok := m.values[name]
	return ok
}

var errNotOneJSONObject = errors.New("not one JSON object")

// readJSONObjectMembers reads the top-level members of a body that is
// exactly one JSON object. It is the dispatch read: it decides which form a
// body is.
func readJSONObjectMembers(data []byte) (*jsonMembers, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errNotOneJSONObject
	}
	m := &jsonMembers{values: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := tok.(string)
		if !ok {
			return nil, errNotOneJSONObject
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, seen := m.values[name]; seen {
			if m.dup == "" {
				m.dup = name
			}
			continue
		}
		m.names = append(m.names, name)
		m.values[name] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errNotOneJSONObject
	}
	return m, nil
}

// jsonStringValue returns raw as a string when it is a JSON string.
func jsonStringValue(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err != nil {
		return "", false
	}
	return s, true
}

// decodeUnpaddedBase64URL decodes strictly: no padding, no line breaks (which
// the standard decoder skips) and no non-zero trailing bits.
func decodeUnpaddedBase64URL(s string) ([]byte, bool) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, false
	}
	return b, true
}

var statementBodyMembers = []string{"signedBytes", "signature", "publicKey"}

// statementLengthDigits is the most digits a DSSE length field may have: the
// digits of the size ceiling.
var statementLengthDigits = len(strconv.Itoa(domain.ActionRequestMaxSignedBytes))

// cutCanonicalLength reads an ASCII decimal length and the space after it:
// digits only, no sign, no leading zero, at most statementLengthDigits.
func cutCanonicalLength(b []byte) (int, []byte, bool) {
	end := bytes.IndexByte(b, ' ')
	if end <= 0 || end > statementLengthDigits {
		return 0, nil, false
	}
	digits := b[:end]
	if digits[0] == '0' && len(digits) > 1 {
		return 0, nil, false
	}
	n := 0
	for _, d := range digits {
		if d < '0' || d > '9' {
			return 0, nil, false
		}
		n = n*10 + int(d-'0')
	}
	return n, b[end+1:], true
}

// parseStatementFrame splits "DSSEv1 <len> <type> <len> <payload>" and
// returns the payload. The type must equal the payload type byte for byte,
// and the payload must be exactly its stated length and end the input.
func parseStatementFrame(frame []byte) ([]byte, bool) {
	rest, ok := bytes.CutPrefix(frame, []byte("DSSEv1 "))
	if !ok {
		return nil, false
	}
	typeLen, rest, ok := cutCanonicalLength(rest)
	if !ok || typeLen != len(domain.ActionRequestPayloadType) || len(rest) < typeLen+1 {
		return nil, false
	}
	if string(rest[:typeLen]) != domain.ActionRequestPayloadType || rest[typeLen] != ' ' {
		return nil, false
	}
	payloadLen, payload, ok := cutCanonicalLength(rest[typeLen+1:])
	if !ok || payloadLen != len(payload) {
		return nil, false
	}
	return payload, true
}

// payloadScan is what one pass over the payload's tokens found.
type payloadScan struct {
	topNames []string // top-level member names, in order
	maxDepth int
	dup      bool // a member name repeats in some object, at any depth
}

// scanStatementPayload walks the payload token by token. It holds an explicit
// stack, so nesting costs no call depth whatever the input. ok is false
// unless the payload is exactly one JSON object in valid UTF-8, starting with
// its first byte and ending with its last.
func scanStatementPayload(payload []byte) (payloadScan, bool) {
	var scan payloadScan
	if len(payload) < 2 || payload[0] != '{' || payload[len(payload)-1] != '}' || !utf8.Valid(payload) {
		return scan, false
	}

	type container struct {
		object    bool
		expectKey bool
		names     map[string]struct{}
	}
	stack := make([]*container, 0)

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if err != nil {
			return scan, false
		}

		var parent *container
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		if parent != nil && parent.object && parent.expectKey {
			if d, ok := tok.(json.Delim); ok && d == '}' {
				stack = stack[:len(stack)-1]
			} else {
				name, ok := tok.(string)
				if !ok {
					return scan, false
				}
				if _, seen := parent.names[name]; seen {
					scan.dup = true
				}
				parent.names[name] = struct{}{}
				if len(stack) == 1 {
					scan.topNames = append(scan.topNames, name)
				}
				parent.expectKey = false
				continue
			}
		} else if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
		} else {
			if parent != nil && parent.object {
				parent.expectKey = true
			}
			if d, ok := tok.(json.Delim); ok {
				stack = append(stack, &container{
					object:    d == '{',
					expectKey: d == '{',
					names:     map[string]struct{}{},
				})
				if len(stack) > scan.maxDepth {
					scan.maxDepth = len(stack)
				}
			} else if parent == nil {
				return scan, false
			}
		}

		if len(stack) == 0 {
			return scan, dec.InputOffset() == int64(len(payload))
		}
	}
}

// statementMemberOrder is every payload member, in the order the form checks
// run.
var statementMemberOrder = []string{
	"action_type", "agent_id", "resource", "context", "timestamp", "nonce",
	"delegation_digests", "risk_level",
}

var statementRequiredMembers = []string{"action_type", "agent_id", "resource", "timestamp", "nonce"}

// statementTimestampForm is RFC 3339 in UTC with a literal Z and optional
// fractional seconds. PostgreSQL's special inputs such as now, infinity and
// epoch never reach the database.
var statementTimestampForm = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?Z$`)

// delegationDigestChars is an unpadded base64url SHA-256 digest.
const delegationDigestChars = 43

func statementFormRefusal(member string) *actionRequestRefusal {
	return invalidStatement("%s is not in the form %s requires", member, domain.ActionRequestSchemeID)
}

// parseActionRequestStatement runs step 1 on a body whose members were
// read by readJSONObjectMembers and that names signedBytes.
func parseActionRequestStatement(body []byte, members *jsonMembers) (*actionRequestStatement, *actionRequestRefusal) {
	if len(body) > domain.ActionRequestMaxRawBody {
		return nil, statementBytesRefusal()
	}
	if members.dup != "" {
		return nil, invalidStatement("the body repeats a member name")
	}
	if len(members.names) != len(statementBodyMembers) {
		return nil, invalidStatement("the body must have exactly the members signedBytes, signature and publicKey")
	}
	for _, name := range statementBodyMembers {
		if !members.has(name) {
			return nil, invalidStatement("the body must have exactly the members signedBytes, signature and publicKey")
		}
	}

	texts := map[string]string{}
	for _, name := range statementBodyMembers {
		if s, ok := jsonStringValue(members.values[name]); ok {
			texts[name] = s
		}
	}
	if s, ok := texts["signedBytes"]; ok && len(s) > domain.ActionRequestMaxSignedBytesChars {
		return nil, statementBytesRefusal()
	}
	decoded := map[string][]byte{}
	for _, name := range statementBodyMembers {
		s, ok := texts[name]
		if !ok {
			return nil, invalidStatement("%s is not unpadded base64url", name)
		}
		b, ok := decodeUnpaddedBase64URL(s)
		if !ok {
			return nil, invalidStatement("%s is not unpadded base64url", name)
		}
		decoded[name] = b
	}

	payload, ok := parseStatementFrame(decoded["signedBytes"])
	if !ok {
		return nil, invalidStatement("signedBytes is not a DSSE v1 frame of type %s", domain.ActionRequestPayloadType)
	}

	scan, ok := scanStatementPayload(payload)
	if !ok {
		return nil, invalidStatement("the payload is not one JSON object in valid UTF-8 with no trailing bytes")
	}
	if scan.maxDepth > domain.ActionRequestMaxDepth {
		return nil, &actionRequestRefusal{
			status:     fiber.StatusUnprocessableEntity,
			reasonCode: actionRequestLimitExceededCode,
			message:    fmt.Sprintf("the payload nests deeper than %d levels", domain.ActionRequestMaxDepth),
			limit:      actionRequestStatementDepthLimit,
		}
	}
	if scan.dup {
		return nil, invalidStatement("the payload repeats a member name")
	}

	defined := map[string]bool{}
	for _, name := range statementMemberOrder {
		defined[name] = true
	}
	for _, name := range scan.topNames {
		if !defined[name] {
			return nil, invalidStatement("the payload member %s is not defined by %s", name, domain.ActionRequestSchemeID)
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, invalidStatement("the payload is not one JSON object in valid UTF-8 with no trailing bytes")
	}
	for _, name := range statementRequiredMembers {
		if _, ok := fields[name]; ok {
			continue
		}
		if name == "nonce" {
			return nil, &actionRequestRefusal{
				status:     fiber.StatusBadRequest,
				reasonCode: actionRequestNonceRequiredCode,
				message:    "the signed statement has no nonce",
			}
		}
		return nil, invalidStatement("the payload is missing %s", name)
	}

	stmt := &actionRequestStatement{
		signedBytes:   decoded["signedBytes"],
		signature:     decoded["signature"],
		publicKey:     decoded["publicKey"],
		signatureText: texts["signature"],
		publicKeyText: texts["publicKey"],
	}
	for _, name := range statementMemberOrder {
		raw, present := fields[name]
		if !present {
			continue
		}
		if refusal := stmt.setMember(name, raw); refusal != nil {
			return nil, refusal
		}
	}
	return stmt, nil
}

// setMember checks one payload member's form and stores its value.
func (s *actionRequestStatement) setMember(name string, raw json.RawMessage) *actionRequestRefusal {
	switch name {
	case "action_type":
		v, ok := jsonStringValue(raw)
		if !ok || v == "" {
			return statementFormRefusal(name)
		}
		s.actionType = v
	case "agent_id":
		v, ok := jsonStringValue(raw)
		if !ok || len(v) != 36 {
			return statementFormRefusal(name)
		}
		id, err := uuid.Parse(v)
		if err != nil {
			return statementFormRefusal(name)
		}
		s.agentID = id
	case "resource":
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			s.resource = ""
			return nil
		}
		v, ok := jsonStringValue(raw)
		if !ok {
			return statementFormRefusal(name)
		}
		s.resource = v
	case "context":
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return statementFormRefusal(name)
		}
		// Numbers stay json.Number, so a value is recorded with the digits
		// it was signed with and never passes through a float.
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		var ctx map[string]interface{}
		if err := dec.Decode(&ctx); err != nil {
			return statementFormRefusal(name)
		}
		s.context = ctx
	case "timestamp":
		v, ok := jsonStringValue(raw)
		if !ok || !statementTimestampForm.MatchString(v) {
			return statementFormRefusal(name)
		}
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return statementFormRefusal(name)
		}
		s.timestamp = t
	case "nonce":
		v, ok := jsonStringValue(raw)
		if !ok {
			return statementFormRefusal(name)
		}
		b, ok := decodeUnpaddedBase64URL(v)
		if !ok || len(b) != domain.ActionRequestNonceBytes {
			return statementFormRefusal(name)
		}
		s.nonce = b
	case "delegation_digests":
		elems := make([]json.RawMessage, 0)
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal(trimmed, &elems) != nil || len(elems) == 0 {
			return invalidStatement("delegation_digests must be omitted or be a non-empty array of hop digests")
		}
		digests := make([]string, 0, len(elems))
		for k, elem := range elems {
			v, ok := jsonStringValue(elem)
			b, decodedOK := decodeUnpaddedBase64URL(v)
			if !ok || len(v) != delegationDigestChars || !decodedOK || len(b) != 32 {
				return invalidStatement("delegation_digests[%d]: not a 43-character unpadded base64url SHA-256 digest", k)
			}
			digests = append(digests, v)
		}
		s.delegationDigests = digests
	case "risk_level":
		v, ok := jsonStringValue(raw)
		if !ok {
			return statementFormRefusal(name)
		}
		s.riskLevel = v
	}
	return nil
}

// freshnessErrorClass names why admission could not run, for the log line.
// The line carries no request value.
func freshnessErrorClass(err error) string {
	switch {
	case errors.Is(err, domain.ErrActionRequestAdmissionTimedOut):
		return "statement_timeout"
	case errors.Is(err, domain.ErrActionRequestNoncePurgeNotRunning):
		return "purge_not_running"
	default:
		return "other"
	}
}

// createVerificationFromStatement handles a body that names signedBytes.
func (h *VerificationHandler) createVerificationFromStatement(c fiber.Ctx, members *jsonMembers) error {
	stmt, refusal := parseActionRequestStatement(c.Body(), members)
	if refusal != nil {
		return refusal.send(c)
	}

	// Only the agent's registered key is read before the signature verifies.
	// The presented key is compared on decoded bytes, so it is passed in the
	// encoding the key set reads.
	keys := agentauth.KeySet(c.Context(), h.getAgentService(), stmt.agentID)
	verified, err := keys.VerifyEd25519(base64.StdEncoding.EncodeToString(stmt.publicKey), stmt.signedBytes, stmt.signature)
	if err != nil {
		if errors.Is(err, agentauth.ErrKeyNotRecognized) {
			return agentKeyNotRecognized.send(c)
		}
		return statementSignatureInvalid.send(c)
	}
	if agentauth.RequiresHybrid(verified) {
		return statementHybridRequired.send(c)
	}
	agent := agentauth.LoadVerifiedAgent(verified)

	if h.actionRequestNonces == nil {
		log.Printf("action-request statement refused: freshness check unavailable (error_class=no_store)")
		return statementFreshnessUnavailable.send(c)
	}
	admission, err := h.actionRequestNonces.Admit(c.Context(), stmt.agentID, agent.OrganizationID, stmt.nonce, stmt.timestamp)
	if err != nil {
		log.Printf("action-request statement refused: freshness check unavailable (error_class=%s)", freshnessErrorClass(err))
		return statementFreshnessUnavailable.send(c)
	}
	switch admission {
	case domain.ActionRequestAdmitted:
	case domain.ActionRequestBehindClock:
		return statementBehindClock.send(c)
	case domain.ActionRequestAheadOfClock:
		return statementAheadOfClock.send(c)
	case domain.ActionRequestNonceReused:
		return statementNonceReused.send(c)
	default:
		log.Printf("action-request statement refused: freshness check unavailable (error_class=other)")
		return statementFreshnessUnavailable.send(c)
	}

	if !domain.AgentStatusPermitsAuth(agent.Status) {
		return (&actionRequestRefusal{
			status:     fiber.StatusUnauthorized,
			reasonCode: actionRequestStatusDeniedCode,
			message:    domain.AgentStatusDeniedMessage(agent.Status),
		}).send(c)
	}

	extra := map[string]interface{}{"signedStatement": domain.ActionRequestSchemeID}
	if len(stmt.delegationDigests) > 0 {
		extra["delegationDigests"] = stmt.delegationDigests
	}
	return h.decideAndRecord(c, agent, stmt.agentID, verificationInput{
		capability: stmt.actionType,
		resource:   stmt.resource,
		context:    stmt.context,
		riskLevel:  stmt.riskLevel,
		signature:  stmt.signatureText,
		publicKey:  stmt.publicKeyText,
		metadata:   extra,
	})
}

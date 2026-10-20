package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// AgentRequestNonceAdmitter admits the nonce of a signed agent request: the
// window check and the insert of (agentID, nonce), in one call.
// application.ActionRequestNonceService implements it.
type AgentRequestNonceAdmitter interface {
	Admit(ctx context.Context, agentID, organizationID uuid.UUID, nonce []byte, signedAt time.Time) (domain.ActionRequestAdmission, error)
}

// AgentRequestV2Middleware authenticates a request that carries a signed agent
// request (domain.AgentRequestHeader; the format is described beside
// domain.AgentRequestSchemeID). A request without that header goes to the next
// handler untouched. A request with it is answered here: it is authenticated,
// or it is refused, and a refused request never reaches the next handler.
//
// audience is the deployment's public origin; the statement's audience must
// equal it byte for byte. It is compared as given: nothing here lowercases it,
// trims it or reads it from the request.
//
// The checks run in this order, and a refusal names the first that fails:
//
//  1. No audience: every signed agent request is refused, 503
//     audienceNotConfigured.
//  2. A header another authenticator reads (agentRequestOtherPrincipalHeaders):
//     400 conflictingAuthentication.
//  3. Shape: the statement header, its payload and the signature header. Reads
//     no stored state.
//  4. The statement names this request: its audience, method, target (path and
//     query as sent) and body digest.
//  5. Lookup of the payload's agent_id, reading only its registered keys, and
//     the signatures over the DSSE pre-authentication encoding of the payload
//     as received; then the hybrid-mode gate. An agent in hybrid mode needs
//     one Ed25519 and one ML-DSA signature, any other agent exactly one.
//  6. Window and nonce admission: the one call to admitter.Admit, whose nonce
//     row is the only row this middleware causes to be written. Steps 1-5
//     write nothing.
//  7. Status. The nonce stays spent.
//
// An authenticated request gets the locals agent_id, organization_id,
// authenticated_via, auth_method and auth_algorithm, the keys
// PQCAgentMiddleware sets, and agent_request_scheme, which names this format.
// It is counted by metrics.RecordAgentRequestAuthenticated under
// metrics.AgentRequestSchemeV2. A refused request is counted by
// metrics.RecordAgentRequestV2Refusal under its refusal's reason.
func AgentRequestV2Middleware(agents agentauth.Source, admitter AgentRequestNonceAdmitter, audience string) fiber.Handler {
	return func(c fiber.Ctx) error {
		statements := headerValues(c, domain.AgentRequestHeader)
		if len(statements) == 0 {
			return c.Next()
		}
		if audience == "" {
			return agentRequestAudienceNotConfigured.send(c)
		}
		for _, name := range agentRequestOtherPrincipalHeaders {
			if len(c.Request().Header.PeekAll(name)) > 0 {
				return agentRequestConflictingAuthentication(name).send(c)
			}
		}
		if len(statements) != 1 {
			return agentRequestStatementInvalid("%s must be sent once", domain.AgentRequestHeader).send(c)
		}

		stmt, refusal := parseAgentRequestStatement(statements[0])
		if refusal != nil {
			return refusal.send(c)
		}
		signatures, refusal := parseAgentRequestSignatures(headerValues(c, domain.AgentRequestSignatureHeader))
		if refusal != nil {
			return refusal.send(c)
		}
		if refusal := stmt.mismatch(c, audience); refusal != nil {
			return refusal.send(c)
		}

		// Only the agent's registered keys are read before the signatures verify, and
		// the request presents no key: each signature is verified against the
		// registered key of the algorithm it names, so naming an algorithm the agent
		// has no key for is the one unrecognised-key refusal. The signed bytes are the
		// payload as received, never a copy rebuilt from its members.
		message := agentRequestPAE(stmt.payload)
		keys := agentauth.KeySet(c.Context(), agents, stmt.agentID)

		var verified agentauth.VerifiedAgent
		var err error
		var authMethod string
		var algorithm pqc.Algorithm
		switch {
		case len(signatures) == 2:
			ed, ml := signatures[0], signatures[1]
			if ml.alg == pqc.AlgorithmEd25519 {
				ed, ml = ml, ed
			}
			authMethod, algorithm = "hybrid", agentRequestHybridAlgorithms[ml.alg]
			verified, err = keys.VerifyHybrid(algorithm, "", "", message, ed.sig, ml.sig)
		case signatures[0].alg == pqc.AlgorithmEd25519:
			authMethod, algorithm = "ed25519", pqc.AlgorithmEd25519
			verified, err = keys.VerifyEd25519("", message, signatures[0].sig)
		default:
			authMethod, algorithm = "mldsa", signatures[0].alg
			verified, err = keys.VerifyMLDSA(algorithm, "", message, signatures[0].sig)
		}
		if err != nil {
			return agentRequestVerifyRefusal(err).send(c)
		}

		// The hybrid flag is an attribute: read only once a signature verified under
		// the agent's key, and before the nonce is admitted, so a request signed the
		// wrong way spends no nonce.
		hybrid := agentauth.RequiresHybrid(verified)
		if hybrid && len(signatures) != 2 {
			return agentRequestHybridRequired.send(c)
		}
		if !hybrid && len(signatures) != 1 {
			return agentRequestSingleSignatureRequired.send(c)
		}
		agent := agentauth.LoadVerifiedAgent(verified)

		if admitter == nil {
			log.Printf("%s refused: freshness check unavailable (error_class=no_store)", domain.AgentRequestSchemeID)
			return agentRequestFreshnessUnavailable.send(c)
		}
		admission, err := admitter.Admit(c.Context(), stmt.agentID, agent.OrganizationID, stmt.nonce, stmt.timestamp)
		if err != nil {
			log.Printf("%s refused: freshness check unavailable (error_class=%s)",
				domain.AgentRequestSchemeID, agentRequestFreshnessErrorClass(err))
			return agentRequestFreshnessUnavailable.send(c)
		}
		switch admission {
		case domain.ActionRequestAdmitted:
		case domain.ActionRequestBehindClock:
			return agentRequestBehindClock.send(c)
		case domain.ActionRequestAheadOfClock:
			return agentRequestAheadOfClock.send(c)
		case domain.ActionRequestNonceReused:
			return agentRequestNonceReused.send(c)
		default:
			log.Printf("%s refused: freshness check unavailable (error_class=other)", domain.AgentRequestSchemeID)
			return agentRequestFreshnessUnavailable.send(c)
		}

		if !agentStatusPermitsAuth(agent.Status) {
			return (&agentRequestRefusal{
				status: fiber.StatusUnauthorized,
				reason: metrics.AgentRequestV2ReasonAgentStatusDenied,
				body:   agentauth.StatusDeniedBody(agent.Status),
			}).send(c)
		}

		c.Locals("agent_id", stmt.agentID)
		c.Locals("organization_id", agent.OrganizationID)
		c.Locals("authenticated_via", authMethod)
		c.Locals("auth_method", authMethod)
		c.Locals("auth_algorithm", string(algorithm))
		c.Locals("agent_request_scheme", domain.AgentRequestSchemeID)
		metrics.RecordAgentRequestAuthenticated(metrics.AgentRequestSchemeV2, c.Get("User-Agent"))
		return c.Next()
	}
}

// agentRequestOtherPrincipalHeaders are the headers through which the other
// authenticators of an agent route name a principal: a bearer token
// (Authorization), an API key (X-API-Key), the headers PQCAgentMiddleware and
// Ed25519AgentMiddleware read, and the X-AIM- headers of the SDK verification
// read. A signed agent request that carries any of them is refused, so a
// request names its principal one way.
var agentRequestOtherPrincipalHeaders = []string{
	"Authorization",
	"X-API-Key",
	"X-Agent-ID",
	"X-Algorithm",
	"X-Timestamp",
	"X-Signature",
	"X-Signature-Ed25519",
	"X-Signature-MLDSA",
	"X-Public-Key",
	"X-PQC-Public-Key",
	"X-AIM-Agent-ID",
	"X-AIM-Timestamp",
	"X-AIM-Signature",
}

// headerValues returns every value of the named request header, copied:
// fasthttp reuses the slice PeekAll returns on the next Peek.
func headerValues(c fiber.Ctx, name string) []string {
	raw := c.Request().Header.PeekAll(name)
	values := make([]string, len(raw))
	for i, v := range raw {
		values[i] = string(v)
	}
	return values
}

// Refusal reason codes of the signed agent request. The key, signature,
// hybrid and status refusals use agentauth's codes and bodies, which the other
// agent authenticators send too.
const (
	agentRequestInvalidRequestCode        = "invalidRequest"
	agentRequestNonceRequiredCode         = "nonceRequired"
	agentRequestConflictingAuthCode       = "conflictingAuthentication"
	agentRequestAudienceNotConfiguredCode = "audienceNotConfigured"
	agentRequestAudienceMismatchCode      = "audienceMismatch"
	agentRequestRequestMismatchCode       = "requestMismatch"
	agentRequestSingleSignatureCode       = "singleSignatureRequired"
	agentRequestOutsideWindowCode         = "timestampOutsideWindow"
	agentRequestNonceReusedCode           = "nonceReused"
	agentRequestFreshnessUnavailableCode  = "freshnessCheckUnavailable"
)

// agentRequestRefusal is one refusal: the status, the body, and the reason it
// is counted under. No body carries a value taken from the request.
type agentRequestRefusal struct {
	status int
	reason metrics.AgentRequestV2RefusalReason
	body   map[string]string
}

// send is the one place the middleware answers a statement it refuses. It
// counts the refusal by reason and calling SDK, then writes the response.
func (r *agentRequestRefusal) send(c fiber.Ctx) error {
	metrics.RecordAgentRequestV2Refusal(r.reason, c.Get("User-Agent"))
	return c.Status(r.status).JSON(r.body)
}

func agentRequestRefused(status int, reason metrics.AgentRequestV2RefusalReason, reasonCode, message string) *agentRequestRefusal {
	return &agentRequestRefusal{
		status: status,
		reason: reason,
		body:   map[string]string{"error": message, "reasonCode": reasonCode},
	}
}

// agentRequestStatementInvalid refuses a statement header, or the payload it
// carries, outside the format.
func agentRequestStatementInvalid(format string, args ...any) *agentRequestRefusal {
	return agentRequestRefused(fiber.StatusBadRequest, metrics.AgentRequestV2ReasonStatementMalformed,
		agentRequestInvalidRequestCode, fmt.Sprintf(format, args...))
}

// agentRequestSignatureHeaderInvalid refuses a signature header outside the
// format.
func agentRequestSignatureHeaderInvalid(format string, args ...any) *agentRequestRefusal {
	return agentRequestRefused(fiber.StatusBadRequest, metrics.AgentRequestV2ReasonSignatureHeaderMalformed,
		agentRequestInvalidRequestCode, fmt.Sprintf(format, args...))
}

func agentRequestConflictingAuthentication(header string) *agentRequestRefusal {
	return agentRequestRefused(fiber.StatusBadRequest, metrics.AgentRequestV2ReasonConflictingAuthentication,
		agentRequestConflictingAuthCode,
		fmt.Sprintf("%s cannot be combined with %s: a request is authenticated one way", domain.AgentRequestHeader, header))
}

func agentRequestFormRefusal(member string) *agentRequestRefusal {
	return agentRequestStatementInvalid("%s is not in the form %s requires", member, domain.AgentRequestSchemeID)
}

// agentRequestMismatch refuses a statement that does not name the request it
// arrived on. The caller gets one reasonCode whatever part differs; reason
// counts the part.
func agentRequestMismatch(reason metrics.AgentRequestV2RefusalReason, message string) *agentRequestRefusal {
	return agentRequestRefused(fiber.StatusUnauthorized, reason, agentRequestRequestMismatchCode, message)
}

var (
	agentRequestAudienceNotConfigured = agentRequestRefused(fiber.StatusServiceUnavailable,
		metrics.AgentRequestV2ReasonAudienceNotConfigured, agentRequestAudienceNotConfiguredCode,
		"this server has no configured public origin and accepts no "+domain.AgentRequestSchemeID+" statement")
	agentRequestAudienceMismatch = agentRequestRefused(fiber.StatusUnauthorized,
		metrics.AgentRequestV2ReasonAudienceMismatch, agentRequestAudienceMismatchCode,
		"the statement's audience is not this server's public origin")
	agentRequestNonceRequired = agentRequestRefused(fiber.StatusBadRequest,
		metrics.AgentRequestV2ReasonNonceMissing, agentRequestNonceRequiredCode,
		"the signed statement has no nonce")
	agentRequestHybridRequired = &agentRequestRefusal{
		status: fiber.StatusUnauthorized,
		reason: metrics.AgentRequestV2ReasonHybridRequired,
		body:   agentauth.HybridRequiredBody("Agent is in hybrid mode: sign the statement with both Ed25519 and ML-DSA"),
	}
	agentRequestSingleSignatureRequired = agentRequestRefused(fiber.StatusUnauthorized,
		metrics.AgentRequestV2ReasonSingleSignatureRequired, agentRequestSingleSignatureCode,
		"Agent is not in hybrid mode: sign the statement with one of its registered keys")
	agentRequestBehindClock = agentRequestRefused(fiber.StatusUnauthorized,
		metrics.AgentRequestV2ReasonSkewPast, agentRequestOutsideWindowCode,
		"request timestamp is behind the AIM database clock by more than the admission window")
	agentRequestAheadOfClock = agentRequestRefused(fiber.StatusUnauthorized,
		metrics.AgentRequestV2ReasonSkewFuture, agentRequestOutsideWindowCode,
		"request timestamp is ahead of the AIM database clock by more than the admission window")
	agentRequestNonceReused = agentRequestRefused(fiber.StatusUnauthorized,
		metrics.AgentRequestV2ReasonNonceReused, agentRequestNonceReusedCode,
		"this nonce was already used by this agent")
	agentRequestFreshnessUnavailable = agentRequestRefused(fiber.StatusServiceUnavailable,
		metrics.AgentRequestV2ReasonFreshnessCheckUnavailable, agentRequestFreshnessUnavailableCode,
		"cannot check this request's time and nonce against the AIM database; nothing was recorded")
)

// agentRequestVerifyRefusal is the refusal for err from an agentauth Verify
// method. An unknown agent, an agent with no registered key for a named
// algorithm and a registered key that does not decode all get agentauth's one
// body, and are counted apart.
func agentRequestVerifyRefusal(err error) *agentRequestRefusal {
	switch {
	case errors.Is(err, agentauth.ErrKeyNotRecognized):
		cause, _ := agentauth.KeyNotRecognizedCause(err)
		switch cause {
		case agentauth.KeyCauseAgentUnknown:
			return agentRequestKeyNotRecognized(metrics.AgentRequestV2ReasonAgentLookupFailed)
		case agentauth.KeyCauseRegisteredKeyMalformed:
			return agentRequestKeyNotRecognized(metrics.AgentRequestV2ReasonRegisteredKeyMalformed)
		default:
			// The cause left that a statement can meet: the agent has no registered
			// key for a named algorithm. agentauth's fourth cause, a presented key
			// that is not the registered one, needs a presented key, and a statement
			// presents none.
			return agentRequestKeyNotRecognized(metrics.AgentRequestV2ReasonNoRegisteredKey)
		}
	case errors.Is(err, agentauth.ErrMLDSASignatureInvalid):
		return &agentRequestRefusal{
			status: fiber.StatusUnauthorized,
			reason: metrics.AgentRequestV2ReasonSignatureInvalidMLDSA,
			body: agentauth.SignatureInvalidBody(
				"the ML-DSA signature does not verify over the statement with this agent's registered key"),
		}
	default:
		return &agentRequestRefusal{
			status: fiber.StatusUnauthorized,
			reason: metrics.AgentRequestV2ReasonSignatureInvalidEd25519,
			body: agentauth.SignatureInvalidBody(
				"the Ed25519 signature does not verify over the statement with this agent's registered key"),
		}
	}
}

// agentRequestKeyNotRecognized is agentauth's one unrecognised-key refusal,
// counted under reason.
func agentRequestKeyNotRecognized(reason metrics.AgentRequestV2RefusalReason) *agentRequestRefusal {
	return &agentRequestRefusal{status: fiber.StatusUnauthorized, reason: reason, body: agentauth.KeyNotRecognizedBody()}
}

// agentRequestFreshnessErrorClass names why admission could not run, for the
// log line. The line carries no request value.
func agentRequestFreshnessErrorClass(err error) string {
	switch {
	case errors.Is(err, domain.ErrActionRequestAdmissionTimedOut):
		return "statement_timeout"
	case errors.Is(err, domain.ErrActionRequestNoncePurgeNotRunning):
		return "purge_not_running"
	default:
		return "other"
	}
}

// agentRequestPAE is the DSSE v1 pre-authentication encoding of a statement's
// payload, the bytes its signatures cover:
//
//	"DSSEv1" SP LEN(type) SP type SP LEN(payload) SP payload
//
// with type domain.AgentRequestPayloadType and each LEN a byte length in ASCII
// decimal.
func agentRequestPAE(payload []byte) []byte {
	out := make([]byte, 0, len("DSSEv1 ")+len(domain.AgentRequestPayloadType)+len(payload)+16)
	out = append(out, "DSSEv1 "...)
	out = strconv.AppendInt(out, int64(len(domain.AgentRequestPayloadType)), 10)
	out = append(out, ' ')
	out = append(out, domain.AgentRequestPayloadType...)
	out = append(out, ' ')
	out = strconv.AppendInt(out, int64(len(payload)), 10)
	out = append(out, ' ')
	return append(out, payload...)
}

// agentRequestStatement is a statement whose shape passed: its payload as
// received and the members read from it.
type agentRequestStatement struct {
	payload []byte

	method     string
	target     string
	audience   string
	bodySHA256 []byte // nil when the payload has no body_sha256
	agentID    uuid.UUID
	timestamp  time.Time
	nonce      []byte
}

// agentRequestMembers is every payload member, in the order the form checks
// run. body_sha256 is the one optional member: present exactly when the
// request has a body.
var agentRequestMembers = []string{"method", "target", "audience", "body_sha256", "agent_id", "timestamp", "nonce"}

var agentRequestRequiredMembers = []string{"method", "target", "audience", "agent_id", "timestamp", "nonce"}

// agentRequestTimestampForm is RFC 3339 in UTC with a literal Z and optional
// fractional seconds.
var agentRequestTimestampForm = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?Z$`)

// parseAgentRequestStatement decodes the statement header and reads its
// payload strictly: one JSON object in valid UTF-8 that starts at the first
// byte and ends at the last, no member name twice, no member the format does
// not define, and every member a JSON string in its form.
func parseAgentRequestStatement(encoded string) (*agentRequestStatement, *agentRequestRefusal) {
	payload, ok := decodeAgentRequestBase64URL(encoded)
	if !ok {
		return nil, agentRequestStatementInvalid("%s is not unpadded base64url", domain.AgentRequestHeader)
	}
	names, values, dup, ok := readAgentRequestPayload(payload)
	if !ok {
		return nil, agentRequestStatementInvalid("the payload is not one JSON object in valid UTF-8 with no trailing bytes")
	}
	if dup {
		return nil, agentRequestStatementInvalid("the payload repeats a member name")
	}
	defined := map[string]bool{}
	for _, name := range agentRequestMembers {
		defined[name] = true
	}
	for _, name := range names {
		if !defined[name] {
			return nil, agentRequestStatementInvalid("the payload has a member %s does not define", domain.AgentRequestSchemeID)
		}
	}
	for _, name := range agentRequestRequiredMembers {
		if _, ok := values[name]; ok {
			continue
		}
		if name == "nonce" {
			return nil, agentRequestNonceRequired
		}
		return nil, agentRequestStatementInvalid("the payload is missing %s", name)
	}

	stmt := &agentRequestStatement{payload: payload}
	for _, name := range agentRequestMembers {
		raw, present := values[name]
		if !present {
			continue
		}
		if refusal := stmt.setMember(name, raw); refusal != nil {
			return nil, refusal
		}
	}
	return stmt, nil
}

// readAgentRequestPayload reads the top-level members of a payload that is
// exactly one JSON object in valid UTF-8, from its first byte to its last.
// ok is false for anything else; dup reports a member name that repeats.
func readAgentRequestPayload(payload []byte) (names []string, values map[string]json.RawMessage, dup, ok bool) {
	if len(payload) < 2 || payload[0] != '{' || payload[len(payload)-1] != '}' || !utf8.Valid(payload) {
		return nil, nil, false, false
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, false, false
	}
	values = map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, false, false
		}
		name, isName := tok.(string)
		if !isName {
			return nil, nil, false, false
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, false, false
		}
		if _, seen := values[name]; seen {
			dup = true
			continue
		}
		names = append(names, name)
		values[name] = raw
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('}') {
		return nil, nil, false, false
	}
	if _, err := dec.Token(); err != io.EOF || dec.InputOffset() != int64(len(payload)) {
		return nil, nil, false, false
	}
	return names, values, dup, true
}

// setMember checks one payload member's form and stores its value.
func (s *agentRequestStatement) setMember(name string, raw json.RawMessage) *agentRequestRefusal {
	v, ok := agentRequestString(raw)
	if !ok {
		return agentRequestFormRefusal(name)
	}
	switch name {
	case "method":
		if v == "" {
			return agentRequestFormRefusal(name)
		}
		s.method = v
	case "target":
		if !strings.HasPrefix(v, "/") {
			return agentRequestFormRefusal(name)
		}
		s.target = v
	case "audience":
		if v == "" {
			return agentRequestFormRefusal(name)
		}
		s.audience = v
	case "body_sha256":
		b, ok := decodeAgentRequestBase64URL(v)
		if !ok || len(b) != sha256.Size {
			return agentRequestFormRefusal(name)
		}
		s.bodySHA256 = b
	case "agent_id":
		if len(v) != 36 {
			return agentRequestFormRefusal(name)
		}
		id, err := uuid.Parse(v)
		if err != nil {
			return agentRequestFormRefusal(name)
		}
		s.agentID = id
	case "timestamp":
		if !agentRequestTimestampForm.MatchString(v) {
			return agentRequestFormRefusal(name)
		}
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return agentRequestFormRefusal(name)
		}
		s.timestamp = t
	case "nonce":
		b, ok := decodeAgentRequestBase64URL(v)
		if !ok || len(b) != domain.ActionRequestNonceBytes {
			return agentRequestFormRefusal(name)
		}
		s.nonce = b
	}
	return nil
}

// mismatch compares the statement with the request it arrived on: the
// configured audience, the method, the target as sent, and the body as
// received, whose digest the statement carries exactly when the body is not
// empty.
func (s *agentRequestStatement) mismatch(c fiber.Ctx, audience string) *agentRequestRefusal {
	if s.audience != audience {
		return agentRequestAudienceMismatch
	}
	if s.method != c.Method() {
		return agentRequestMismatch(metrics.AgentRequestV2ReasonMethodMismatch,
			"the statement's method is not this request's method")
	}
	if s.target != c.OriginalURL() {
		return agentRequestMismatch(metrics.AgentRequestV2ReasonTargetMismatch,
			"the statement's target is not this request's path and query")
	}
	body := c.BodyRaw()
	switch {
	case len(body) > 0 && s.bodySHA256 == nil:
		return agentRequestMismatch(metrics.AgentRequestV2ReasonBodyMismatch,
			"this request has a body and the statement has no body_sha256")
	case len(body) == 0 && s.bodySHA256 != nil:
		return agentRequestMismatch(metrics.AgentRequestV2ReasonBodyMismatch,
			"the statement has body_sha256 and this request has no body")
	case len(body) > 0:
		sum := sha256.Sum256(body)
		if !bytes.Equal(sum[:], s.bodySHA256) {
			return agentRequestMismatch(metrics.AgentRequestV2ReasonBodyMismatch,
				"body_sha256 is not the SHA-256 of this request's body")
		}
	}
	return nil
}

// agentRequestSignature is one signature of the signature header, decoded.
type agentRequestSignature struct {
	alg pqc.Algorithm
	sig []byte
}

// agentRequestSignatureAlgorithms are the algorithms a signature may name: the
// single-key values PQCAgentMiddleware accepts in X-Algorithm.
var agentRequestSignatureAlgorithms = map[string]pqc.Algorithm{
	string(pqc.AlgorithmEd25519): pqc.AlgorithmEd25519,
	string(pqc.AlgorithmMLDSA44): pqc.AlgorithmMLDSA44,
	string(pqc.AlgorithmMLDSA65): pqc.AlgorithmMLDSA65,
	string(pqc.AlgorithmMLDSA87): pqc.AlgorithmMLDSA87,
}

// agentRequestHybridAlgorithms is the hybrid algorithm of each ML-DSA level.
var agentRequestHybridAlgorithms = map[pqc.Algorithm]pqc.Algorithm{
	pqc.AlgorithmMLDSA44: pqc.AlgorithmHybridEd25519MLDSA44,
	pqc.AlgorithmMLDSA65: pqc.AlgorithmHybridEd25519MLDSA65,
	pqc.AlgorithmMLDSA87: pqc.AlgorithmHybridEd25519MLDSA87,
}

// parseAgentRequestSignatures reads the signature header's values. The header
// is a list: two header lines and one line holding both values separated by
// a comma are the same two values. There are one or two values, and two are
// one Ed25519 and one ML-DSA signature.
func parseAgentRequestSignatures(lines []string) ([]agentRequestSignature, *agentRequestRefusal) {
	var values []string
	for _, line := range lines {
		for _, v := range strings.Split(line, ",") {
			values = append(values, strings.Trim(v, " \t"))
		}
	}
	switch {
	case len(values) == 0:
		return nil, agentRequestSignatureHeaderInvalid("%s is required", domain.AgentRequestSignatureHeader)
	case len(values) > 2:
		return nil, agentRequestSignatureHeaderInvalid("%s carries more than two signatures", domain.AgentRequestSignatureHeader)
	}
	signatures := make([]agentRequestSignature, 0, len(values))
	for _, v := range values {
		name, encoded, found := strings.Cut(v, ":")
		alg, known := agentRequestSignatureAlgorithms[name]
		sig, decoded := decodeAgentRequestBase64URL(encoded)
		if !found || !known || !decoded || len(sig) == 0 {
			return nil, agentRequestSignatureHeaderInvalid("each %s value is an algorithm, a colon and an unpadded base64url "+
				"signature; the algorithm is Ed25519, ML-DSA-44, ML-DSA-65 or ML-DSA-87", domain.AgentRequestSignatureHeader)
		}
		signatures = append(signatures, agentRequestSignature{alg: alg, sig: sig})
	}
	if len(signatures) == 2 && (signatures[0].alg == pqc.AlgorithmEd25519) == (signatures[1].alg == pqc.AlgorithmEd25519) {
		return nil, agentRequestSignatureHeaderInvalid("two signatures are one Ed25519 and one ML-DSA signature")
	}
	return signatures, nil
}

// agentRequestString returns raw as a string when it is a JSON string.
func agentRequestString(raw json.RawMessage) (string, bool) {
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

// decodeAgentRequestBase64URL decodes strictly: no padding, no line breaks
// (which the decoder would skip) and no non-zero trailing bits.
func decodeAgentRequestBase64URL(s string) ([]byte, bool) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, false
	}
	return b, true
}

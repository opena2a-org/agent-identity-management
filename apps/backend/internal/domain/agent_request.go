package domain

// A signed agent request authenticates one HTTP request as an agent. The
// request carries a statement about itself, a JSON payload in the
// AgentRequestHeader header, and the agent's signatures over the DSSE v1
// pre-authentication encoding of AgentRequestPayloadType and that payload in
// the AgentRequestSignatureHeader header. The payload is verified as received,
// never as a copy rebuilt from its members, and must name the request's
// method, path and query, body and audience.
//
// The statement's nonce is admitted by a call with the signature of
// ActionRequestNonceRepository.Admit, which application.ActionRequestNonceService
// implements over agent_request_nonces. This format declares no window or
// retention of its own.
const (
	// AgentRequestSchemeID is the public name of the format. It is the one
	// declaration of that name in the backend's Go source.
	AgentRequestSchemeID = "agent-request-v2"

	// AgentRequestPayloadType is the DSSE payload type. It leads the signed
	// bytes of every statement, after "DSSEv1" and its length.
	AgentRequestPayloadType = "application/vnd.opena2a.agent-request.v2+json"

	// AgentRequestHeader carries the statement's payload as unpadded
	// base64url. A request carries it once.
	AgentRequestHeader = "X-Agent-Request"

	// AgentRequestSignatureHeader carries the signatures, each an algorithm, a
	// colon and the unpadded base64url signature ("Ed25519:...",
	// "ML-DSA-65:..."): one, or for an agent in hybrid mode one Ed25519 and one
	// ML-DSA signature, sent as two header lines or as one comma-separated line.
	AgentRequestSignatureHeader = "X-Agent-Request-Signature"
)

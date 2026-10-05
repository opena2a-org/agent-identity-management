package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// A signed action-request statement is the second body form of
// POST /api/v1/verifications and of its SDK mount. The body carries the
// signed bytes themselves (`signedBytes`, a DSSE v1 pre-authentication
// encoding of ActionRequestPayloadType and a JSON payload), so the server
// verifies exactly what the client signed instead of rebuilding it. The
// payload names a nonce and a timestamp: a statement is accepted once, and
// only while its timestamp is within ActionRequestWindowSeconds of the
// database clock.
//
// Every value below is a code constant. None is read from configuration or the
// environment: widening the window or shortening the retention would reopen
// replay, so a change is a code change with its tests.
const (
	// ActionRequestSchemeID is the public name of the statement format. It is
	// the one declaration of that name in the backend's Go source.
	ActionRequestSchemeID = "action-request-v1"

	// ActionRequestPayloadType is the DSSE payload type. It leads every
	// statement's signed bytes, so no byte string the same agent key signs in
	// another format can be presented as a statement.
	ActionRequestPayloadType = "application/vnd.opena2a.action-request.v1+json"

	// ActionRequestWindowSeconds bounds the signed timestamp T: a statement is
	// accepted only when now-30s <= T <= now+30s, inclusive, where now is the
	// database clock read by the admission statement. It is the whole skew
	// allowance; there is no separate skew term.
	ActionRequestWindowSeconds = 30

	// ActionRequestNonceRetentionSeconds is K: an admitted nonce is kept until
	// at least expires_at + K, where expires_at is T + the window. K is never
	// below the window, so a nonce can be admitted twice only if the database
	// clock steps back by more than K, and such a step also refuses honest
	// requests as ahead of the clock.
	ActionRequestNonceRetentionSeconds = 31

	// ActionRequestAdmissionTimeout bounds the admission statement
	// (SET LOCAL statement_timeout). It must stay below K.
	ActionRequestAdmissionTimeout = 5 * time.Second

	// ActionRequestNoncePurgeInterval is how often the server deletes admitted
	// nonces past expires_at + K. The purge also runs once at start.
	ActionRequestNoncePurgeInterval = 60 * time.Second

	// ActionRequestNonceBytes is the decoded nonce length: 16 bytes, sent as
	// 22 characters of unpadded base64url.
	ActionRequestNonceBytes = 16

	// ActionRequestMaxSignedBytes is the size ceiling on decoded signedBytes.
	ActionRequestMaxSignedBytes = 65536

	// ActionRequestMaxSignedBytesChars is the same ceiling on the encoded
	// form, ceil(4 * ActionRequestMaxSignedBytes / 3) unpadded base64url
	// characters. It is checked before decoding.
	ActionRequestMaxSignedBytesChars = (4*ActionRequestMaxSignedBytes + 2) / 3

	// ActionRequestRawBodyAllowance covers the request body's fixed overhead
	// around signedBytes: 177 bytes in compact form (a 48-byte three-member
	// skeleton, an 86-character signature and a 43-character public key),
	// plus whitespace.
	ActionRequestRawBodyAllowance = 1024

	// ActionRequestMaxRawBody is the raw body ceiling, checked before the
	// body is parsed.
	ActionRequestMaxRawBody = ActionRequestMaxSignedBytesChars + ActionRequestRawBodyAllowance

	// ActionRequestMaxDepth is the nesting ceiling on the payload: JSON object
	// and array levels, the payload object counted as level 1.
	ActionRequestMaxDepth = 32
)

// ActionRequestAdmission is the outcome of the admission statement.
type ActionRequestAdmission int

const (
	// ActionRequestAdmitted: the timestamp is inside the window and the nonce
	// row was written.
	ActionRequestAdmitted ActionRequestAdmission = iota + 1
	// ActionRequestBehindClock: T is more than the window before the reading.
	ActionRequestBehindClock
	// ActionRequestAheadOfClock: T is more than the window after the reading.
	ActionRequestAheadOfClock
	// ActionRequestNonceReused: the agent already used this nonce.
	ActionRequestNonceReused
)

var (
	// ErrActionRequestNoncePurgeNotRunning refuses admission while this
	// process has not completed a nonce purge recently: no nonce is stored on
	// a deployment that is not deleting them.
	ErrActionRequestNoncePurgeNotRunning = errors.New("action-request nonce purge is not running")

	// ErrActionRequestAdmissionTimedOut is the admission statement hitting
	// ActionRequestAdmissionTimeout.
	ErrActionRequestAdmissionTimedOut = errors.New("action-request admission statement timed out")
)

// ActionRequestNonceRepository is the admission store, agent_request_nonces.
// Only admission and the purge use it; a row is never returned or copied.
type ActionRequestNonceRepository interface {
	// Admit checks signedAt against the database clock and inserts
	// (agentID, nonce) in one statement, inserting nothing when the
	// timestamp is outside the window or the pair already exists.
	Admit(ctx context.Context, agentID, organizationID uuid.UUID, nonce []byte, signedAt time.Time) (ActionRequestAdmission, error)
	// ListOrganizationIDs returns the ids of every organization the purge
	// visits.
	ListOrganizationIDs(ctx context.Context) ([]uuid.UUID, error)
	// PurgeOrganization deletes one organization's nonces past
	// expires_at + K on the database clock and returns the count.
	PurgeOrganization(ctx context.Context, organizationID uuid.UUID) (int64, error)
}

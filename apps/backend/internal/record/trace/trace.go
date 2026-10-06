// Package trace gives audit records their correlation identifier and parent
// reference.
//
// Every record carries, in its tenant part, a trace_id and a parent_id, and
// in its retained part opena2a.trace_origin, which says where the trace_id
// came from:
//
//   - parent: the record has a parent, and its trace_id is the parent's.
//     parent_id is the parent's event_id.
//   - server: the record is the first of its trace. Its trace_id is the one
//     the server minted for the request or job run that wrote it, and
//     parent_id is null.
//
// A trace_id is 16 bytes from the CSPRNG, never all zeros, in 32 lowercase
// hexadecimal characters. A caller's traceparent header is never the source
// of a trace_id: AIM cannot show that a caller's value is random, and a
// caller-chosen trace_id would let one caller place records inside another
// run's trace. The trace-id field of a valid incoming traceparent is kept
// instead as opena2a.request_trace_id in the personal part, so that a caller
// can match its own trace to the records. Nothing else of the request's trace
// context is read: not its parent-id or flags, and never tracestate or
// baggage.
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
)

// Origins of a record's trace_id, the values of opena2a.trace_origin.
const (
	OriginParent = "parent"
	OriginServer = "server"
)

// Member names.
const (
	memberTraceID        = "trace_id"
	memberParentID       = "parent_id"
	memberTraceOrigin    = "trace_origin"
	memberRequestTraceID = "request_trace_id"
	extensionMember      = "opena2a"
)

// idSize is the length in bytes of a trace identifier.
const idSize = 16

// ErrInvalidTrace is wrapped by every error that refuses a record's trace
// members.
var ErrInvalidTrace = errors.New("trace: invalid trace")

// ErrNoTrace is returned by Stamp for a record that has no parent when its
// context carries no trace.
var ErrNoTrace = errors.New("trace: the context carries no trace")

// Context is the trace of one request or one job run.
type Context struct {
	// TraceID is minted by the server for the request or job run.
	TraceID string
	// RequestTraceID is the trace-id field of the request's traceparent
	// header when that header is valid, and empty otherwise. It is chosen by
	// the caller and is never a record's trace_id.
	RequestTraceID string
}

type contextKey struct{}

// With returns a context that carries tc.
func With(ctx context.Context, tc Context) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

// From returns the trace ctx carries.
func From(ctx context.Context) (Context, bool) {
	tc, ok := ctx.Value(contextKey{}).(Context)
	return tc, ok && tc.TraceID != ""
}

// Begin returns a context that carries a newly minted trace, for a job run or
// any other unit of work that is not a request. It replaces any trace ctx
// already carries.
func Begin(ctx context.Context) (context.Context, error) {
	id, err := Mint()
	if err != nil {
		return nil, err
	}
	return With(ctx, Context{TraceID: id}), nil
}

// ForRequest mints the trace of one request. traceparent is the request's
// traceparent header, or empty when it has none or more than one.
func ForRequest(traceparent string) (Context, error) {
	id, err := Mint()
	if err != nil {
		return Context{}, err
	}
	requestTraceID, _ := ParseTraceparent(traceparent)
	return Context{TraceID: id, RequestTraceID: requestTraceID}, nil
}

// Mint returns a new trace identifier from the CSPRNG.
func Mint() (string, error) { return mint(rand.Reader) }

func mint(r io.Reader) (string, error) {
	b := make([]byte, idSize)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", fmt.Errorf("trace: mint: %w", err)
	}
	for _, c := range b {
		if c != 0 {
			return hex.EncodeToString(b), nil
		}
	}
	return "", errors.New("trace: mint: the random source returned all zeros")
}

// ValidID reports whether s is a trace identifier: 32 lowercase hexadecimal
// characters, not all zeros.
func ValidID(s string) bool { return isLowerHex(s, 2*idSize) }

// ParseTraceparent returns the trace-id field of a traceparent header of
// version 00 whose trace-id and parent-id are not all zeros. Any other value
// is refused, and nothing of it is returned.
func ParseTraceparent(header string) (string, bool) {
	// version "-" trace-id "-" parent-id "-" flags: 2+1+32+1+16+1+2.
	if len(header) != 55 || header[2] != '-' || header[35] != '-' || header[52] != '-' {
		return "", false
	}
	version, traceID, parentID, flags := header[0:2], header[3:35], header[36:52], header[53:55]
	if version != "00" || !allLowerHex(flags) || !isLowerHex(traceID, 32) || !isLowerHex(parentID, 16) {
		return "", false
	}
	return traceID, true
}

// isLowerHex reports whether s is n lowercase hexadecimal characters, not all
// zeros.
func isLowerHex(s string, n int) bool {
	if len(s) != n || !allLowerHex(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return true
		}
	}
	return false
}

func allLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Parent is the record a new record follows in its trace.
type Parent struct {
	EventID string
	TraceID string
}

// Stamp sets a draft's trace members. A record with a parent takes the
// parent's trace; a record without one takes the trace ctx carries, and Stamp
// returns ErrNoTrace when ctx carries none. When the request that ctx carries
// sent a valid traceparent, its trace-id is set as opena2a.request_trace_id
// in the personal part.
func Stamp(ctx context.Context, d *record.Draft, parent *Parent) error {
	tc, traced := From(ctx)
	var traceID, origin string
	var parentID any
	switch {
	case parent != nil:
		if !ValidID(parent.TraceID) {
			return fmt.Errorf("%w: the parent's trace_id is not 32 lowercase hex characters, not all zeros", ErrInvalidTrace)
		}
		if !canonicalUUID(parent.EventID) || parent.EventID == d.EventID {
			return fmt.Errorf("%w: the parent's event_id is not another record's lowercase hyphenated UUID", ErrInvalidTrace)
		}
		traceID, parentID, origin = parent.TraceID, parent.EventID, OriginParent
	case traced:
		traceID, parentID, origin = tc.TraceID, nil, OriginServer
	default:
		return ErrNoTrace
	}

	d.Tenant = withMember(d.Tenant, memberTraceID, traceID)
	d.Tenant[memberParentID] = parentID
	d.Retained = withExtension(d.Retained, memberTraceOrigin, origin)
	if traced && ValidID(tc.RequestTraceID) {
		d.Personal = withExtension(d.Personal, memberRequestTraceID, tc.RequestTraceID)
	}
	return nil
}

// Link is a record's place in its trace, as its members state it.
type Link struct {
	TraceID string
	// ParentID is empty for the first record of a trace.
	ParentID string
	Origin   string
}

// Check reads a draft's trace members and refuses a draft whose members do
// not give it a place in a trace: trace_id must be a trace identifier in the
// tenant part; parent_id must be in the tenant part, null or another
// record's event_id; opena2a.trace_origin must be in the retained part, parent
// exactly when parent_id is set and server otherwise; and
// opena2a.request_trace_id, when present, must be a trace identifier in the
// personal part and nowhere else.
func Check(d record.Draft) (Link, error) {
	var l Link
	raw, ok := d.Tenant[memberTraceID]
	if !ok {
		return Link{}, fmt.Errorf("%w: the record has no trace_id in its tenant part", ErrInvalidTrace)
	}
	if l.TraceID, ok = raw.(string); !ok || !ValidID(l.TraceID) {
		return Link{}, fmt.Errorf("%w: trace_id is not 32 lowercase hex characters, not all zeros", ErrInvalidTrace)
	}
	raw, ok = d.Tenant[memberParentID]
	if !ok {
		return Link{}, fmt.Errorf("%w: the record has no parent_id in its tenant part", ErrInvalidTrace)
	}
	if raw != nil {
		if l.ParentID, ok = raw.(string); !ok || !canonicalUUID(l.ParentID) {
			return Link{}, fmt.Errorf("%w: parent_id is neither null nor a lowercase hyphenated UUID", ErrInvalidTrace)
		}
		if l.ParentID == d.EventID {
			return Link{}, fmt.Errorf("%w: the record names itself as its parent", ErrInvalidTrace)
		}
	}
	l.Origin, _ = extension(d.Retained)[memberTraceOrigin].(string)
	switch {
	case l.ParentID != "" && l.Origin != OriginParent:
		return Link{}, fmt.Errorf("%w: a record with a parent has opena2a.trace_origin %q", ErrInvalidTrace, OriginParent)
	case l.ParentID == "" && l.Origin != OriginServer:
		return Link{}, fmt.Errorf("%w: the first record of a trace has opena2a.trace_origin %q", ErrInvalidTrace, OriginServer)
	}
	for _, part := range []map[string]any{d.Retained, d.Tenant} {
		if _, set := extension(part)[memberRequestTraceID]; set {
			return Link{}, fmt.Errorf("%w: opena2a.request_trace_id sits only in the personal part", ErrInvalidTrace)
		}
	}
	if raw, set := extension(d.Personal)[memberRequestTraceID]; set {
		if s, ok := raw.(string); !ok || !ValidID(s) {
			return Link{}, fmt.Errorf("%w: opena2a.request_trace_id is not 32 lowercase hex characters, not all zeros", ErrInvalidTrace)
		}
	}
	return l, nil
}

// TraceIDOfPart returns the trace_id of a record's stored tenant part.
func TraceIDOfPart(tenantPart []byte) (string, error) {
	var part struct {
		TraceID *string `json:"trace_id"`
	}
	if err := json.Unmarshal(tenantPart, &part); err != nil {
		return "", fmt.Errorf("%w: the tenant part: %v", ErrInvalidTrace, err)
	}
	if part.TraceID == nil || !ValidID(*part.TraceID) {
		return "", fmt.Errorf("%w: the tenant part carries no trace_id", ErrInvalidTrace)
	}
	return *part.TraceID, nil
}

// withMember returns m, or a new map when m is nil, with name set to v.
func withMember(m map[string]any, name string, v any) map[string]any {
	if m == nil {
		m = map[string]any{}
	}
	m[name] = v
	return m
}

// withExtension returns m with name set to v under "opena2a". The "opena2a"
// object is copied, so a map the caller shares with another draft is not
// changed.
func withExtension(m map[string]any, name string, v any) map[string]any {
	ext := map[string]any{}
	for k, val := range extension(m) {
		ext[k] = val
	}
	ext[name] = v
	return withMember(m, extensionMember, ext)
}

func extension(m map[string]any) map[string]any {
	ext, _ := m[extensionMember].(map[string]any)
	return ext
}

func canonicalUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s
}

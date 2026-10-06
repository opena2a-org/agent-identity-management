package store

import (
	"context"
	"fmt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
)

// requestTraceQuery reads the records of one organization's chain whose
// personal part carries the request trace. It reads the value from the
// personal part itself: no column or index holds a copy, so a record whose
// personal part is erased is no longer found by it.
const requestTraceQuery = `
SELECT r.payload_type, r.payload, r.key_id, r.signature, r.record_hash,
       r.tenant_part, r.tenant_salt, r.personal_part, r.personal_salt
  FROM record_chains c
  JOIN audit_records r ON r.chain_id = c.id
 WHERE c.organization_id = $1
   AND r.personal_part IS NOT NULL
   AND convert_from(r.personal_part, 'UTF8')::jsonb #>> '{opena2a,request_trace_id}' = $2
 ORDER BY r.seq`

// RecordsByRequestTrace returns, in sequence order, the records of the
// organization's chain whose personal part carries opena2a.request_trace_id
// equal to requestTraceID: the records written in requests that sent a
// traceparent with that trace-id. It is how a caller matches its own trace to
// the records.
//
// It reads only the organization's own chain. A value sent in requests of
// another organization returns nothing here, and an organization with no
// chain has no records. requestTraceID is 32 lowercase hexadecimal
// characters, not all zeros.
func RecordsByRequestTrace(ctx context.Context, q Querier, organizationID, requestTraceID string) ([]record.Record, error) {
	if !canonicalUUID(organizationID) {
		return nil, fmt.Errorf("%w: the organization id is not a lowercase hyphenated UUID", ErrInvalidWrite)
	}
	if !trace.ValidID(requestTraceID) {
		return nil, fmt.Errorf("%w: the request trace id is not 32 lowercase hex characters, not all zeros", ErrInvalidWrite)
	}
	rows, err := q.QueryContext(ctx, requestTraceQuery, organizationID, requestTraceID)
	if err != nil {
		return nil, fmt.Errorf("store: read records by request trace: %w", err)
	}
	out, err := scanRecords(rows)
	if err != nil {
		return nil, fmt.Errorf("store: read records by request trace: %w", err)
	}
	return out, nil
}

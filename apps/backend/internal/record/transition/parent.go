package transition

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// requestNamespace is the namespace of the event ids RequestEventID derives.
var requestNamespace = uuid.MustParse("e03a23b5-1ae7-412e-a4f9-c33a98959b6a")

// RequestEventID is the event_id of a capability request's
// capability_requested record: a name-based UUID (version 5) of the request's
// id. The decision on the request finds that record by it, with no reference
// stored beside the request.
func RequestEventID(requestID uuid.UUID) string {
	return uuid.NewSHA1(requestNamespace, requestID[:]).String()
}

// Parent is a record a change follows: its event id, which the change's
// record names as parent_id, and its trace, which the change's record joins.
type Parent struct {
	EventID string
	TraceID string
}

const requestRecordQuery = `
SELECT a.record_type, a.payload, a.tenant_part
  FROM audit_records a
  JOIN record_chains c ON c.id = a.chain_id
 WHERE c.organization_id = $1 AND a.event_id = $2`

// RequestParent returns the capability_requested record of an agent's
// capability request. It returns false when the organization's chain holds
// no record at the request's event id, as for a request filed while no
// recorder was set, and when that record's tenant part has been erased. A
// record at that event id that is not the agent's capability_requested
// record is an error.
func (r *Recorder) RequestParent(ctx context.Context, organizationID, agentID, requestID uuid.UUID) (Parent, bool, error) {
	eventID := RequestEventID(requestID)
	var (
		recordType      string
		payload, tenant []byte
	)
	err := r.db.QueryRowContext(ctx, requestRecordQuery, organizationID, eventID).Scan(&recordType, &payload, &tenant)
	if errors.Is(err, sql.ErrNoRows) {
		return Parent{}, false, nil
	}
	if err != nil {
		return Parent{}, false, fmt.Errorf("transition: read the request's record: %w", err)
	}
	var h head
	if err := json.Unmarshal(payload, &h); err != nil {
		return Parent{}, false, fmt.Errorf("transition: the request's record: %w", err)
	}
	if recordType != RecordType || h.Type != RecordType || h.Trigger.Type != string(TriggerCapabilityRequested) {
		return Parent{}, false, fmt.Errorf("transition: the record at request %s's event id is not a %s record", requestID, TriggerCapabilityRequested)
	}
	if tenant == nil {
		return Parent{}, false, nil
	}
	var t struct {
		TraceID string `json:"trace_id"`
		Opena2a struct {
			SubjectAgentID string `json:"subject_agent_id"`
		} `json:"opena2a"`
	}
	if err := json.Unmarshal(tenant, &t); err != nil {
		return Parent{}, false, fmt.Errorf("transition: the tenant part of the request's record: %w", err)
	}
	if t.Opena2a.SubjectAgentID != agentID.String() || !validTraceID(t.TraceID) {
		return Parent{}, false, fmt.Errorf("transition: the record of request %s does not name its agent and a trace", requestID)
	}
	return Parent{EventID: eventID, TraceID: t.TraceID}, true, nil
}

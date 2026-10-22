package record

import (
	"encoding/json"
	"fmt"
)

// memberCorrectedEventID is the member, under "opena2a" in a correction's
// tenant part, that names the corrected record by its event_id. A reference
// from one record to another is a tenant value, so it sits in the tenant part
// and is erased with it.
const memberCorrectedEventID = "corrected_event_id"

// Correction is the input of a correction record.
type Correction struct {
	// CorrectedEventID is the event_id of the record this record corrects, in
	// lowercase hyphenated form. The corrected record sits before the
	// correction in the same chain. Verify checks that; a builder that holds
	// only the chain's head cannot.
	CorrectedEventID string
	// Draft carries the record's own members, placed by the caller as for
	// NewRecord. Its Type is TypeCorrection or empty. The tenant part holds
	// the reference, so the draft takes a tenant salt even when it gives no
	// tenant member of its own.
	Draft Draft
}

// NewCorrection builds the correction record that follows prev: sequence
// prev.Seq plus one, committing to prev.Hash. The record names the record it
// corrects as opena2a.corrected_event_id in its tenant part. Building it
// leaves the corrected record as it is: the correction is a record of its
// own.
func NewCorrection(prev Head, c Correction) (Body, error) {
	if err := checkHead(prev); err != nil {
		return Body{}, err
	}
	d := c.Draft
	if d.Type == "" {
		d.Type = TypeCorrection
	}
	if d.Type != TypeCorrection {
		return Body{}, fmt.Errorf("%w: a correction has type %s, not %q", ErrInvalidRecord, TypeCorrection, d.Type)
	}
	if !isCanonicalUUID(c.CorrectedEventID) {
		return Body{}, fmt.Errorf("%w: the corrected event id is not a lowercase hyphenated UUID", ErrInvalidRecord)
	}
	if c.CorrectedEventID == d.EventID {
		return Body{}, fmt.Errorf("%w: a record does not correct itself", ErrInvalidRecord)
	}
	own := owned{tenant: map[string]any{memberCorrectedEventID: c.CorrectedEventID}}
	return seal(prev.ChainID, prev.Seq+1, prev.Hash, d, own)
}

// checkCorrection returns why a correction does not name a record before it in
// its chain, or "". earlier holds the event ids of the records before it. The
// tenant part has already matched its commitment. A correction always commits
// to a tenant part, since that part holds its reference; once the part is
// erased, the reference is gone and nothing is left to check. The reason
// quotes nothing from the tenant part, whose values are erasable.
func checkCorrection(h header, tenantPart []byte, earlier map[string]bool) string {
	if _, committed := h.commitments["tenant"]; !committed {
		return "the correction commits to no tenant part, so it never named the record it corrects"
	}
	if len(tenantPart) == 0 {
		return ""
	}
	var part map[string]any
	if err := json.Unmarshal(tenantPart, &part); err != nil {
		return "the tenant part is not a JSON object"
	}
	ext, _ := part[extensionMember].(map[string]any)
	raw, present := ext[memberCorrectedEventID]
	if !present {
		return "the tenant part does not name the corrected record in opena2a.corrected_event_id"
	}
	id, ok := raw.(string)
	if !ok || !isCanonicalUUID(id) {
		return "opena2a.corrected_event_id is not a lowercase hyphenated UUID"
	}
	if !earlier[id] {
		return "opena2a.corrected_event_id is the event id of no record before the correction in its chain"
	}
	return ""
}

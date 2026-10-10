package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
)

// A debt is a pending copy of the record a reduction could not append: one
// record_debts row. The settler builds the late record from the row alone,
// so every column becomes exactly one member of it, in the part the member
// takes, and a reduction's draft can only carry members a column holds.

// The parts of a record.
const (
	partRetained = "retained"
	partTenant   = "tenant"
	partPersonal = "personal"
)

// columnKind is what a debt column holds and how it is checked.
type columnKind int

const (
	kindDraft       columnKind = iota // set from the draft's event id, type or timestamp, or from the write's organization
	kindVocabulary                    // a closed-vocabulary value: lowercase letters, digits and underscores
	kindStateSpace                    // one of stateSpaces
	kindActorClass                    // operatorActorClass
	kindBuildCommit                   // 40 lowercase hex characters, or "unstamped"
	kindUUID                          // a UUID in lowercase hyphenated form
	kindText                          // any string
	kindJSON                          // any value of the canonical data model
)

// debtColumn is one column of record_debts and the member of the late record
// it becomes. A member under "opena2a" is named "opena2a.<name>".
type debtColumn struct {
	name   string
	member string
	// part is the part the member sits in. byResource columns sit in the
	// tenant part, or in the personal part when the record's resource type
	// names a person.
	part       string
	byResource bool
	kind       columnKind
}

// debtColumns are the columns of record_debts, in table order. A test reads
// the table's columns from the database and compares them with this list.
var debtColumns = []debtColumn{
	{name: "id", member: "event_id", part: partRetained, kind: kindDraft},
	{name: "organization_id", member: "opena2a.organization_id", part: partTenant, kind: kindDraft},
	{name: "record_type", member: "type", part: partRetained, kind: kindDraft},
	{name: "occurred_at", member: "opena2a.occurred_at", part: partRetained, kind: kindDraft},
	{name: "state_space", member: "opena2a.state_space", part: partRetained, kind: kindStateSpace},
	{name: "trigger_type", member: "opena2a.trigger_type", part: partRetained, kind: kindVocabulary},
	{name: "admin_action", member: "opena2a.admin_action", part: partRetained, kind: kindVocabulary},
	{name: "resource_type", member: "opena2a.resource_type", part: partRetained, kind: kindVocabulary},
	{name: "actor_class", member: "actor", part: partRetained, kind: kindActorClass},
	{name: "operator_subcommand", member: "opena2a.operator_subcommand", part: partRetained, kind: kindVocabulary},
	{name: "operator_reason_code", member: "opena2a.operator_reason_code", part: partRetained, kind: kindVocabulary},
	{name: "build_commit", member: "opena2a.build_commit", part: partRetained, kind: kindBuildCommit},
	{name: "subject_agent_id", member: "opena2a.subject_agent_id", part: partTenant, kind: kindUUID},
	{name: "verification_ref", member: "opena2a.verification_ref", part: partTenant, kind: kindText},
	{name: "trace_id", member: "trace_id", part: partTenant, kind: kindText},
	{name: "parent_id", member: "parent_id", part: partTenant, kind: kindText},
	{name: "resource_id", member: "opena2a.resource_id", part: partTenant, byResource: true, kind: kindText},
	{name: "previous_state", member: "opena2a.previous_state", part: partTenant, byResource: true, kind: kindJSON},
	{name: "new_state", member: "opena2a.new_state", part: partTenant, byResource: true, kind: kindJSON},
	{name: "actor", member: "actor", part: partPersonal, kind: kindText},
	{name: "request_trace_id", member: "opena2a.request_trace_id", part: partPersonal, kind: kindText},
	{name: "reason", member: "opena2a.reason", part: partPersonal, kind: kindText},
}

// Members the late record carries that no column holds: the record
// package's own, and the two that mark the record late.
const (
	memberLate       = "late"
	memberOccurredAt = "occurred_at"
)

// The trace members a column does not hold as written. A traced record's
// parent_id is null on the first record of its trace, which the parent_id
// column holds as NULL, and its opena2a.trace_origin follows from parent_id
// (trace.Check refuses any other), so the late record rebuilds both.
const (
	memberTraceID     = "trace_id"
	memberParentID    = "parent_id"
	memberTraceOrigin = "opena2a.trace_origin"
)

// operatorActorClass is the one actor value that names no person, agent or
// key: an operator's act. It is retained; any other actor is personal.
const operatorActorClass = "operator_command"

// operatorColumns are retained with actor_class, and only with it.
var operatorColumns = []string{"operator_subcommand", "operator_reason_code", "build_commit"}

var stateSpaces = map[string]bool{"agent": true, "verification": true, "appraisal": true}

// personResourceTypes name a person: a record about one carries its
// resource id and states in the personal part.
var personResourceTypes = map[string]bool{"user": true, "user_rejection": true, "membership": true, "session": true}

var (
	vocabularyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	recordTypePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	buildCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// debt is one record_debts row. values holds every column after the first
// four that is not NULL: a string, or for a kindJSON column any value of the
// canonical data model, JSON null included.
type debt struct {
	id             string
	organizationID string
	recordType     string
	occurredAt     time.Time
	values         map[string]any
}

// debtFromDraft is the debt a reduction's draft leaves if its record cannot
// be appended. It refuses a draft that carries a member no column holds, or
// holds in another part, so that the late record built from the row is the
// record the draft would have been, marked late.
func debtFromDraft(organizationID string, d record.Draft) (debt, error) {
	if !canonicalUUID(d.EventID) {
		return debt{}, errors.New("store: a debt's event id is not a lowercase hyphenated UUID")
	}
	if !recordTypePattern.MatchString(d.Type) || d.Type == record.TypeChainGenesis {
		return debt{}, fmt.Errorf("store: a debt cannot hold record type %q", d.Type)
	}
	if d.Timestamp.IsZero() {
		return debt{}, errors.New("store: a debt's timestamp is not set")
	}
	out := debt{
		id:             d.EventID,
		organizationID: organizationID,
		recordType:     d.Type,
		occurredAt:     d.Timestamp.UTC().Truncate(time.Microsecond),
		values:         map[string]any{},
	}

	resourceType := ""
	if ext, ok := d.Retained["opena2a"].(map[string]any); ok {
		if v, ok := ext["resource_type"].(string); ok {
			resourceType = v
		}
	}
	parts := []struct {
		name    string
		members map[string]any
	}{{partRetained, d.Retained}, {partTenant, d.Tenant}, {partPersonal, d.Personal}}
	for _, p := range parts {
		for name, value := range p.members {
			if name == memberParentID && p.name == partTenant && value == nil {
				continue
			}
			if name != "opena2a" {
				if err := out.take(p.name, name, value, resourceType); err != nil {
					return debt{}, err
				}
				continue
			}
			ext, ok := value.(map[string]any)
			if !ok {
				return debt{}, fmt.Errorf("store: opena2a in the %s part is not an object", p.name)
			}
			for inner, v := range ext {
				member := "opena2a." + inner
				if member == "opena2a.organization_id" && p.name == partTenant {
					if v != organizationID {
						return debt{}, errors.New("store: the draft's opena2a.organization_id is not the write's organization")
					}
					continue
				}
				if member == memberTraceOrigin && p.name == partRetained {
					if v != traceOrigin(d.Tenant[memberParentID]) {
						return debt{}, errors.New("store: the draft's opena2a.trace_origin does not follow from its parent_id")
					}
					continue
				}
				if err := out.take(p.name, member, v, resourceType); err != nil {
					return debt{}, err
				}
			}
		}
	}

	if _, traced := out.values[memberTraceID]; traced {
		if _, set := d.Tenant[memberParentID]; !set {
			return debt{}, errors.New("store: a traced draft has no parent_id")
		}
		ext, _ := d.Retained["opena2a"].(map[string]any)
		if _, set := ext["trace_origin"]; !set {
			return debt{}, errors.New("store: a traced draft has no opena2a.trace_origin")
		}
	}

	_, operator := out.values["actor_class"]
	for _, name := range operatorColumns {
		if _, set := out.values[name]; set != operator {
			return debt{}, fmt.Errorf("store: %s is carried exactly when the actor is %s", name, operatorActorClass)
		}
	}
	return out, nil
}

// take places one member of the draft in its column.
func (d *debt) take(part, member string, value any, resourceType string) error {
	for _, c := range debtColumns {
		if c.kind == kindDraft || c.member != member || c.partFor(resourceType) != part {
			continue
		}
		if err := checkColumnValue(c, value); err != nil {
			return err
		}
		d.values[c.name] = value
		return nil
	}
	return fmt.Errorf("store: no debt column holds %s in the %s part", member, part)
}

// partFor is the part the column's member sits in on a record of the given
// resource type.
func (c debtColumn) partFor(resourceType string) string {
	if c.byResource && personResourceTypes[resourceType] {
		return partPersonal
	}
	return c.part
}

func checkColumnValue(c debtColumn, value any) error {
	s, isString := value.(string)
	switch c.kind {
	case kindJSON:
		if err := checkNoNUL(value); err != nil {
			return fmt.Errorf("store: %s: %w", c.member, err)
		}
		return nil
	case kindDraft:
		return fmt.Errorf("store: %s is set from the draft", c.member)
	}
	if !isString {
		return fmt.Errorf("store: %s holds a %T, not a string", c.member, value)
	}
	var ok bool
	switch c.kind {
	case kindVocabulary:
		ok = vocabularyPattern.MatchString(s)
	case kindStateSpace:
		ok = stateSpaces[s]
	case kindActorClass:
		ok = s == operatorActorClass
	case kindBuildCommit:
		ok = buildCommitPattern.MatchString(s) || s == "unstamped"
	case kindUUID:
		ok = canonicalUUID(s)
	case kindText:
		ok = !strings.ContainsRune(s, 0) && !(c.name == "actor" && s == operatorActorClass)
	}
	if !ok {
		return fmt.Errorf("store: %s holds a value its debt column does not admit", c.member)
	}
	return nil
}

// checkNoNUL refuses a string holding U+0000, which no database column holds.
func checkNoNUL(v any) error {
	switch x := v.(type) {
	case string:
		if strings.ContainsRune(x, 0) {
			return errors.New("a string holds U+0000")
		}
	case []any:
		for _, item := range x {
			if err := checkNoNUL(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for k, item := range x {
			if strings.ContainsRune(k, 0) {
				return errors.New("a member name holds U+0000")
			}
			if err := checkNoNUL(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// lateDraft is the late record the debt stands for, built from the row
// alone. Its timestamp is now; it carries the reduction's time as
// opena2a.occurred_at and opena2a.late true, both retained.
func (d debt) lateDraft(now time.Time) (record.Draft, error) {
	occurred, err := record.FormatTimestamp(d.occurredAt)
	if err != nil {
		return record.Draft{}, err
	}
	resourceType, _ := d.values["resource_type"].(string)
	parts := map[string]map[string]any{
		partRetained: {"opena2a": map[string]any{memberLate: true, memberOccurredAt: occurred}},
		partTenant:   {"opena2a": map[string]any{"organization_id": d.organizationID}},
		partPersonal: {},
	}
	for _, c := range debtColumns {
		value, set := d.values[c.name]
		if !set {
			continue
		}
		part := parts[c.partFor(resourceType)]
		if name, ok := strings.CutPrefix(c.member, "opena2a."); ok {
			ext, _ := part["opena2a"].(map[string]any)
			if ext == nil {
				ext = map[string]any{}
				part["opena2a"] = ext
			}
			ext[name] = value
			continue
		}
		part[c.member] = value
	}
	if _, traced := d.values[memberTraceID]; traced {
		parent, set := d.values[memberParentID]
		if !set {
			parent = nil
			parts[partTenant][memberParentID] = nil
		}
		parts[partRetained]["opena2a"].(map[string]any)["trace_origin"] = traceOrigin(parent)
	}
	draft := record.Draft{
		EventID:   d.id,
		Type:      d.recordType,
		Timestamp: now.UTC().Truncate(time.Microsecond),
		Retained:  parts[partRetained],
		Tenant:    parts[partTenant],
	}
	if len(parts[partPersonal]) > 0 {
		draft.Personal = parts[partPersonal]
	}
	return draft, nil
}

// traceOrigin is the opena2a.trace_origin of a record whose parent_id is
// parent: the parent's trace when it has one, the server's otherwise.
func traceOrigin(parent any) string {
	if parent == nil {
		return trace.OriginServer
	}
	return trace.OriginParent
}

// args are the insert's arguments, in debtColumns order.
func (d debt) args() ([]any, error) {
	out := make([]any, 0, len(debtColumns))
	for _, c := range debtColumns {
		switch c.name {
		case "id":
			out = append(out, d.id)
		case "organization_id":
			out = append(out, d.organizationID)
		case "record_type":
			out = append(out, d.recordType)
		case "occurred_at":
			out = append(out, d.occurredAt)
		default:
			value, set := d.values[c.name]
			switch {
			case !set:
				out = append(out, nil)
			case c.kind == kindJSON:
				b, err := json.Marshal(value)
				if err != nil {
					return nil, fmt.Errorf("store: %s: %w", c.member, err)
				}
				out = append(out, string(b))
			default:
				out = append(out, value)
			}
		}
	}
	return out, nil
}

// scanDebt reads one row selected as debtColumnList.
func scanDebt(row interface{ Scan(...any) error }) (debt, error) {
	var d debt
	optional := make([]sql.NullString, len(debtColumns)-4)
	dest := []any{&d.id, &d.organizationID, &d.recordType, &d.occurredAt}
	for i := range optional {
		dest = append(dest, &optional[i])
	}
	if err := row.Scan(dest...); err != nil {
		return debt{}, err
	}
	d.occurredAt = d.occurredAt.UTC()
	d.values = map[string]any{}
	for i, c := range debtColumns[4:] {
		if !optional[i].Valid {
			continue
		}
		if c.kind != kindJSON {
			d.values[c.name] = optional[i].String
			continue
		}
		v, err := decodeCanonicalJSON(optional[i].String)
		if err != nil {
			return debt{}, fmt.Errorf("store: debt %s: %s: %w", d.id, c.name, err)
		}
		d.values[c.name] = v
	}
	return d, nil
}

// decodeCanonicalJSON decodes one JSON value into the canonical data model:
// every number must be an integer, and becomes an int64.
func decodeCanonicalJSON(s string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON value")
	}
	return integers(v)
}

func integers(v any) (any, error) {
	switch x := v.(type) {
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return nil, fmt.Errorf("%s is not an integer", x)
		}
		return n, nil
	case []any:
		for i := range x {
			item, err := integers(x[i])
			if err != nil {
				return nil, err
			}
			x[i] = item
		}
	case map[string]any:
		for k := range x {
			item, err := integers(x[k])
			if err != nil {
				return nil, err
			}
			x[k] = item
		}
	}
	return v, nil
}

// debtColumnList is the column list of every statement that writes or reads
// a whole debt row, in debtColumns order.
var debtColumnList = func() string {
	names := make([]string, len(debtColumns))
	for i, c := range debtColumns {
		names[i] = c.name
	}
	return strings.Join(names, ", ")
}()

// The statements on record_debts. Each names one organization.
var (
	//nolint:gosec // G202: the column list and the placeholders come from the static debtColumns table; no value is concatenated
	insertDebtQuery = `INSERT INTO record_debts (` + debtColumnList + `)
VALUES (` + placeholders(len(debtColumns)) + `)`
	// takeDebtQuery deletes one open debt of the organization and returns it,
	// in the transaction that appends its late record. A debt another
	// settler is taking is skipped, never waited for.
	takeDebtQuery = `DELETE FROM record_debts
 WHERE organization_id = $1
   AND id = (SELECT id FROM record_debts
              WHERE organization_id = $1 AND id = $2
              FOR UPDATE SKIP LOCKED)
RETURNING ` + debtColumnList
	openDebtIDsQuery = `SELECT id FROM record_debts
 WHERE organization_id = $1
 ORDER BY occurred_at, id
 LIMIT $2`
	openDebtsQuery = `SELECT count(*), min(occurred_at) FROM record_debts
 WHERE organization_id = $1`
)

func placeholders(n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(out, ", ")
}

// line is the debt's console line, written once the row is committed. It
// names the debt, its organization, its state space, trigger type and
// resource type, when the reduction happened, and the UUID of its subject,
// and nothing a caller supplied as free text.
func (d debt) line() string {
	value := func(column string) string {
		if s, ok := d.values[column].(string); ok {
			return s
		}
		return "none"
	}
	subject := "none"
	if s, ok := d.values["subject_agent_id"].(string); ok {
		subject = s
	} else if s, ok := d.values["resource_id"].(string); ok && canonicalUUID(s) {
		subject = s
	}
	occurred, err := record.FormatTimestamp(d.occurredAt)
	if err != nil {
		occurred = "none"
	}
	return fmt.Sprintf("%s debt_id=%s organization_id=%s state_space=%s trigger_type=%s resource_type=%s occurred_at=%s subject=%s",
		EventRecordDebtWritten, d.id, d.organizationID, value("state_space"), value("trigger_type"),
		value("resource_type"), occurred, subject)
}

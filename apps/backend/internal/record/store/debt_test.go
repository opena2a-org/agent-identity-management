package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/require"
)

const (
	debtOrg     = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	debtAgent   = "6b0c8f7e-1d2a-4c3b-9e8f-7a6b5c4d3e2f"
	debtUser    = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	debtEventID = "a1a1a1a1-0000-4000-8000-000000000001"
)

var debtTime = time.Date(2026, time.October, 6, 12, 30, 45, 123456000, time.UTC)

// agentDebtDraft is a reduction on an agent that carries a member for every
// column but the operator columns.
func agentDebtDraft() record.Draft {
	return record.Draft{
		EventID:   debtEventID,
		Type:      "opena2a.authorization_transition",
		Timestamp: debtTime,
		Retained: map[string]any{"opena2a": map[string]any{
			"state_space":   "agent",
			"trigger_type":  "operator_suspended",
			"admin_action":  "agent_suspended",
			"resource_type": "agent",
		}},
		Tenant: map[string]any{
			"trace_id":  "trace-1",
			"parent_id": "b2b2b2b2-0000-4000-8000-000000000002",
			"opena2a": map[string]any{
				"organization_id":  debtOrg,
				"subject_agent_id": debtAgent,
				"verification_ref": "c3c3c3c3-0000-4000-8000-000000000003",
				"resource_id":      debtAgent,
				"previous_state":   map[string]any{"status": "active", "scopes": []any{"read", "write"}, "keys": int64(2)},
				"new_state":        map[string]any{"status": "suspended", "scopes": []any{}, "keys": int64(0)},
			},
		},
		Personal: map[string]any{
			"actor": debtUser,
			"opena2a": map[string]any{
				"request_trace_id": "req-1",
				"reason":           "left the team",
			},
		},
	}
}

// operatorDebtDraft is an operator's act on a user: the actor is the
// operator class, and the resource id and states are personal.
func operatorDebtDraft() record.Draft {
	return record.Draft{
		EventID:   "d4d4d4d4-0000-4000-8000-000000000004",
		Type:      "opena2a.administrative",
		Timestamp: debtTime,
		Retained: map[string]any{
			"actor": "operator_command",
			"opena2a": map[string]any{
				"admin_action":         "user_deactivated",
				"resource_type":        "user",
				"operator_subcommand":  "user_deactivate",
				"operator_reason_code": "account_compromised",
				"build_commit":         "unstamped",
			},
		},
		Personal: map[string]any{"opena2a": map[string]any{
			"resource_id":    debtUser,
			"previous_state": "active",
			"new_state":      "deactivated",
		}},
	}
}

// A debt holds every member of the draft, and the late record built from it
// alone is the draft with its timestamp moved to the settle time and the
// reduction's time kept as opena2a.occurred_at, marked late.
func TestRecordDebtLateDraftIsTheDraftMarkedLate(t *testing.T) {
	settledAt := debtTime.Add(90 * time.Second)
	for name, draft := range map[string]record.Draft{"agent": agentDebtDraft(), "operator": operatorDebtDraft()} {
		t.Run(name, func(t *testing.T) {
			d, err := debtFromDraft(debtOrg, draft)
			require.NoError(t, err)

			// The row survives the database's text forms.
			args, err := d.args()
			require.NoError(t, err)
			require.Len(t, args, len(debtColumns))
			for i, c := range debtColumns {
				if c.kind == kindJSON && args[i] != nil {
					v, err := decodeCanonicalJSON(args[i].(string))
					require.NoError(t, err)
					require.Equal(t, d.values[c.name], v, c.name)
				}
			}

			late, err := d.lateDraft(settledAt)
			require.NoError(t, err)
			want := cloneDraft(draft)
			want.Timestamp = settledAt
			ext := want.Retained["opena2a"].(map[string]any)
			ext["late"] = true
			ext["occurred_at"] = "2026-10-06T12:30:45.123456Z"
			if want.Tenant == nil {
				want.Tenant = map[string]any{}
			}
			tenantExt, _ := want.Tenant["opena2a"].(map[string]any)
			if tenantExt == nil {
				tenantExt = map[string]any{}
				want.Tenant["opena2a"] = tenantExt
			}
			tenantExt["organization_id"] = debtOrg
			require.Equal(t, want, late)

			_, err = record.NewRecord(placeholderHead, withSalts(t, late))
			require.NoError(t, err, "the late draft is a record")
		})
	}
}

func withSalts(t *testing.T, d record.Draft) record.Draft {
	t.Helper()
	require.NoError(t, addSalts(&d))
	return d
}

// Every column of the table maps to one member, in one part, and no two
// columns that can be set together map to the same member.
func TestRecordDebtColumnsMapToDistinctMembers(t *testing.T) {
	seen := map[string]string{}
	for _, c := range debtColumns {
		require.Contains(t, []string{partRetained, partTenant, partPersonal}, c.part, c.name)
		if other, dup := seen[c.member]; dup {
			pair := map[string]bool{other: true, c.name: true}
			require.True(t, pair["actor"] && pair["actor_class"], "%s and %s map to %s", other, c.name, c.member)
			continue
		}
		seen[c.member] = c.name
	}
}

func TestRecordDebtRefusesWhatItCannotHold(t *testing.T) {
	cases := map[string]func(d *record.Draft){
		"a member no column holds": func(d *record.Draft) {
			d.Tenant["opena2a"].(map[string]any)["metadata"] = "x"
		},
		"a top-level member no column holds": func(d *record.Draft) {
			d.Personal["email"] = "someone@example.com"
		},
		"a tenant member in the personal part": func(d *record.Draft) {
			delete(d.Tenant, "trace_id")
			d.Personal["trace_id"] = "trace-1"
		},
		"a resource id in the personal part of a non-person resource": func(d *record.Draft) {
			delete(d.Tenant["opena2a"].(map[string]any), "resource_id")
			d.Personal["opena2a"].(map[string]any)["resource_id"] = debtAgent
		},
		"a person's resource id in the tenant part": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["resource_type"] = "user"
		},
		"a personal actor naming the operator class": func(d *record.Draft) {
			d.Personal["actor"] = "operator_command"
		},
		"a retained actor naming a person": func(d *record.Draft) {
			delete(d.Personal, "actor")
			d.Retained["actor"] = debtUser
		},
		"operator columns without the operator class": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["build_commit"] = "unstamped"
		},
		"another organization": func(d *record.Draft) {
			d.Tenant["opena2a"].(map[string]any)["organization_id"] = debtAgent
		},
		"an organization id outside the tenant part": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["organization_id"] = debtOrg
		},
		"a member the late record sets": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["late"] = true
		},
		"a value outside the vocabulary": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["trigger_type"] = "Operator suspended!"
		},
		"a state space outside the set": func(d *record.Draft) {
			d.Retained["opena2a"].(map[string]any)["state_space"] = "user"
		},
		"a subject that is not a UUID": func(d *record.Draft) {
			d.Tenant["opena2a"].(map[string]any)["subject_agent_id"] = "agent-7"
		},
		"a string holding U+0000": func(d *record.Draft) {
			d.Personal["opena2a"].(map[string]any)["reason"] = "a\x00b"
		},
		"a state holding U+0000": func(d *record.Draft) {
			d.Tenant["opena2a"].(map[string]any)["new_state"] = map[string]any{"status": "a\x00b"}
		},
		"a non-string text value": func(d *record.Draft) {
			d.Tenant["trace_id"] = int64(7)
		},
		"the genesis type": func(d *record.Draft) {
			d.Type = record.TypeChainGenesis
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d := cloneDraft(agentDebtDraft())
			d.Tenant["opena2a"] = cloneMembers(d.Tenant["opena2a"].(map[string]any))
			d.Personal["opena2a"] = cloneMembers(d.Personal["opena2a"].(map[string]any))
			mutate(&d)
			_, err := debtFromDraft(debtOrg, d)
			require.Error(t, err)
		})
	}

	op := operatorDebtDraft()
	delete(op.Retained["opena2a"].(map[string]any), "operator_reason_code")
	_, err := debtFromDraft(debtOrg, op)
	require.Error(t, err, "an operator's act carries all three operator columns")
}

// A reduction whose draft a debt cannot hold is refused before the write
// takes a connection, with reason canonical, so no reduction can commit
// without the means to record it later.
func TestRecordReductionThatNoDebtCanHoldIsRefusedBeforeTheLock(t *testing.T) {
	w, reg, logs := offlineWriter(t)
	d := agentDebtDraft()
	d.Personal["email"] = "someone@example.com"
	_, err := w.Write(context.Background(), Write{Class: ClassReduction, OrganizationID: debtOrg, Draft: d})
	var we *WriteError
	require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
	require.Equal(t, ReasonCanonical, we.Reason)
	require.Equal(t, 1.0, failureTotal(t, reg))
	require.Equal(t, "SECURITY record_write_failed class=reduction reason=canonical debt=none\n", logs.String())

	// The same draft is no debt's business when the write is an expansion:
	// it fails later, at the unreachable database, not as canonical.
	_, err = w.Write(context.Background(), Write{Class: ClassExpansion, OrganizationID: debtOrg, Draft: d})
	require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
	require.NotEqual(t, ReasonCanonical, we.Reason)
}

func TestRecordDebtLineNamesNoFreeText(t *testing.T) {
	d, err := debtFromDraft(debtOrg, agentDebtDraft())
	require.NoError(t, err)
	require.Equal(t, "record_debt_written debt_id="+debtEventID+" organization_id="+debtOrg+
		" state_space=agent trigger_type=operator_suspended resource_type=agent"+
		" occurred_at=2026-10-06T12:30:45.123456Z subject="+debtAgent, d.line())
	for _, free := range []string{"left the team", "req-1", "trace-1", debtUser} {
		require.NotContains(t, d.line(), free)
	}

	op, err := debtFromDraft(debtOrg, operatorDebtDraft())
	require.NoError(t, err)
	require.Contains(t, op.line(), " state_space=none trigger_type=none resource_type=user ")
	require.Contains(t, op.line(), " subject="+debtUser)
}

// Every statement on record_debts names one organization, and the
// settler's organization list reads no debt.
func TestRecordDebtStatementsNameOneOrganization(t *testing.T) {
	for _, q := range []string{takeDebtQuery, openDebtIDsQuery, openDebtsQuery} {
		require.Contains(t, q, "WHERE organization_id = $1", q)
	}
	require.Contains(t, insertDebtQuery, "(id, organization_id, ")
	require.NotContains(t, settlerOrganizationsQuery, "record_debts")
}

func TestRecordPathStateFollowsTheWindow(t *testing.T) {
	var h pathHealth
	start := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	require.Equal(t, PathStatus{State: PathUnknown}, h.status(start))

	h.succeeded(start)
	require.Equal(t, PathStatus{State: PathOK}, h.status(start.Add(time.Second)))

	h.failed(start.Add(10*time.Second), ClassObservation, ReasonDatabase)
	require.Equal(t, PathOK, h.status(start.Add(11*time.Second)).State, "an observation's failure leaves the path ok")

	h.failed(start.Add(20*time.Second), ClassReduction, ReasonChainHead)
	h.succeeded(start.Add(30 * time.Second))
	require.Equal(t, PathStatus{State: PathUnavailable, Reason: "record write failed (class reduction, reason chain_head)"},
		h.status(start.Add(31*time.Second)), "a failure in the window outweighs a later success")

	require.Equal(t, PathOK, h.status(start.Add(20*time.Second+RecordPathWindow)).State,
		"the failure leaves the window; the later success is still in it")
	require.Equal(t, PathUnknown, h.status(start.Add(30*time.Second+RecordPathWindow)).State)
}

func TestRecordPathReadsUnavailableAfterAFailedWrite(t *testing.T) {
	w, _, _ := offlineWriter(t)
	require.Equal(t, PathUnknown, w.RecordPath(time.Now()).State)
	d := agentDebtDraft()
	d.Personal["email"] = "someone@example.com"
	_, err := w.Write(context.Background(), Write{Class: ClassReduction, OrganizationID: debtOrg, Draft: d})
	require.Error(t, err)
	status := w.RecordPath(time.Now())
	require.Equal(t, PathUnavailable, status.State)
	require.Equal(t, "record write failed (class reduction, reason canonical)", status.Reason)
}

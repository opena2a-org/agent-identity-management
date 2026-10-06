package transition

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// ErrAgentNotFound is returned when the agent is not in the organization.
var ErrAgentNotFound = errors.New("transition: the agent is not in the organization")

// Key roles.
const (
	KeyRoleCurrent  = "current"
	KeyRolePrevious = "previous"
)

// Key custody of a current key: the service generated the key and holds its
// private half, or the agent holds it.
const (
	KeyCustodyServer   = "server"
	KeyCustodyExternal = "external"
)

// State is an agent's authorization state.
type State struct {
	// Scope is the capability types in force: GrantedScope, or empty while
	// the agent is suspended or revoked. Sorted, without duplicates.
	Scope []string
	// GrantedScope is the capability types granted and not revoked. Sorted,
	// without duplicates.
	GrantedScope []string
	Status       string
	Keys         []Key
	// TalksTo is the entries of the agent's talks_to list as exact strings,
	// sorted by byte order, without duplicates.
	TalksTo []string
}

// Key is one key the agent row holds.
type Key struct {
	Alg string `json:"alg"`
	// ID recovers the public key: did:key for an Ed25519 key, otherwise
	// multibase base64url ("u" followed by the key bytes).
	ID   string `json:"id"`
	Role string `json:"role"`
	// Custody is set on a current key only.
	Custody string `json:"custody,omitempty"`
	// GraceUntil is the rotation grace deadline of a previous key, or nil
	// when the row holds none. Every previous key carries it, as null when
	// nil; no current key does.
	GraceUntil *string `json:"grace_until,omitempty"`
}

// Equal reports whether two states have the same members.
func Equal(a, b State) bool {
	if a.Status != b.Status || !equalStrings(a.Scope, b.Scope) ||
		!equalStrings(a.GrantedScope, b.GrantedScope) || !equalStrings(a.TalksTo, b.TalksTo) ||
		len(a.Keys) != len(b.Keys) {
		return false
	}
	for i := range a.Keys {
		x, y := a.Keys[i], b.Keys[i]
		if x.Alg != y.Alg || x.ID != y.ID || x.Role != y.Role || x.Custody != y.Custody {
			return false
		}
		if (x.GraceUntil == nil) != (y.GraceUntil == nil) ||
			x.GraceUntil != nil && *x.GraceUntil != *y.GraceUntil {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// member is the state as a record member.
func (s State) member() map[string]any {
	keys := make([]any, 0, len(s.Keys))
	for _, k := range s.Keys {
		m := map[string]any{"alg": k.Alg, "id": k.ID, "role": k.Role}
		if k.Custody != "" {
			m["custody"] = k.Custody
		}
		if k.Role == KeyRolePrevious {
			if k.GraceUntil == nil {
				m["grace_until"] = nil
			} else {
				m["grace_until"] = *k.GraceUntil
			}
		}
		keys = append(keys, m)
	}
	return map[string]any{
		"scope": stringList(s.Scope),
		"opena2a": map[string]any{
			"granted_scope": stringList(s.GrantedScope),
			"status":        s.Status,
			"keys":          keys,
			"talks_to":      stringList(s.TalksTo),
		},
	}
}

func stringList(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

// stateMember is a state member as a record holds it.
type stateMember struct {
	Scope   []string `json:"scope"`
	Opena2a struct {
		GrantedScope []string `json:"granted_scope"`
		Status       string   `json:"status"`
		Keys         []Key    `json:"keys"`
		TalksTo      []string `json:"talks_to"`
	} `json:"opena2a"`
}

func (m stateMember) state() State {
	return State{
		Scope:        nonNil(m.Scope),
		GrantedScope: nonNil(m.Opena2a.GrantedScope),
		Status:       m.Opena2a.Status,
		Keys:         m.Opena2a.Keys,
		TalksTo:      nonNil(m.Opena2a.TalksTo),
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

const (
	agentStateQuery = `
SELECT status, public_key, key_algorithm,
       encrypted_private_key IS NOT NULL AND encrypted_private_key <> '',
       previous_public_key, key_rotation_grace_until,
       pqc_public_key, pqc_key_algorithm, previous_pqc_public_key, talks_to
  FROM agents
 WHERE id = $1 AND organization_id = $2`
	grantedScopeQuery = `
SELECT DISTINCT capability_type FROM agent_capabilities
 WHERE agent_id = $1 AND revoked_at IS NULL`
)

// CurrentState reads an agent's state from the state tables.
func CurrentState(ctx context.Context, q store.Querier, organizationID, agentID uuid.UUID) (State, error) {
	return readState(ctx, q, organizationID, agentID, false)
}

// readState reads an agent's state. With lock it takes the agent's row lock
// first, so q must be a transaction.
func readState(ctx context.Context, q store.Querier, organizationID, agentID uuid.UUID, lock bool) (State, error) {
	query := agentStateQuery
	if lock {
		query += "\n   FOR UPDATE"
	}
	var (
		status                         string
		publicKey, keyAlg, previousKey sql.NullString
		pqcKey, pqcAlg, previousPQCKey sql.NullString
		serverCustody                  bool
		graceUntil                     sql.NullTime
		talksTo                        []byte
	)
	err := q.QueryRowContext(ctx, query, agentID, organizationID).Scan(
		&status, &publicKey, &keyAlg, &serverCustody, &previousKey, &graceUntil,
		&pqcKey, &pqcAlg, &previousPQCKey, &talksTo)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, ErrAgentNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("transition: read agent state: %w", err)
	}

	rows, err := q.QueryContext(ctx, grantedScopeQuery, agentID)
	if err != nil {
		return State{}, fmt.Errorf("transition: read granted scope: %w", err)
	}
	defer rows.Close()
	granted := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return State{}, fmt.Errorf("transition: read granted scope: %w", err)
		}
		granted = append(granted, c)
	}
	if err := rows.Err(); err != nil {
		return State{}, fmt.Errorf("transition: read granted scope: %w", err)
	}
	sort.Strings(granted)

	entries, err := domain.DecodeTalksTo(talksTo)
	if err != nil {
		return State{}, fmt.Errorf("transition: read talks_to: %w", err)
	}

	s := State{Scope: granted, GrantedScope: granted, Status: status, Keys: []Key{}, TalksTo: sortedSet(entries)}
	if status == "suspended" || status == "revoked" {
		s.Scope = []string{}
	}

	alg := ed25519Alg
	if keyAlg.Valid && keyAlg.String != "" {
		alg = keyAlg.String
	}
	if present(publicKey) {
		custody := KeyCustodyExternal
		if serverCustody {
			custody = KeyCustodyServer
		}
		s.Keys = append(s.Keys, Key{Alg: alg, ID: keyIdentifier(alg, publicKey.String), Role: KeyRoleCurrent, Custody: custody})
	}
	if present(previousKey) {
		k := Key{Alg: alg, ID: keyIdentifier(alg, previousKey.String), Role: KeyRolePrevious}
		if graceUntil.Valid {
			ts, err := record.FormatTimestamp(graceUntil.Time)
			if err != nil {
				return State{}, fmt.Errorf("transition: grace deadline: %w", err)
			}
			k.GraceUntil = &ts
		}
		s.Keys = append(s.Keys, k)
	}
	if present(pqcKey) {
		s.Keys = append(s.Keys, Key{Alg: pqcAlg.String, ID: keyIdentifier(pqcAlg.String, pqcKey.String),
			Role: KeyRoleCurrent, Custody: KeyCustodyExternal})
	}
	if present(previousPQCKey) {
		s.Keys = append(s.Keys, Key{Alg: pqcAlg.String, ID: keyIdentifier(pqcAlg.String, previousPQCKey.String),
			Role: KeyRolePrevious})
	}
	return s, nil
}

func present(s sql.NullString) bool { return s.Valid && s.String != "" }

// sortedSet returns the distinct members of s sorted by byte order, never
// nil.
func sortedSet(s []string) []string {
	out := make([]string, 0, len(s))
	for v := range stringSet(s) {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

const ed25519Alg = "Ed25519"

// ed25519Multicodec is the multicodec prefix of an Ed25519 public key.
var ed25519Multicodec = []byte{0xed, 0x01}

// keyIdentifier returns an identifier from which a stored public key is
// recovered. A stored key is standard base64 of the key bytes; a value that
// does not decode is identified by its own bytes.
func keyIdentifier(alg, stored string) string {
	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		raw = []byte(stored)
	} else if strings.EqualFold(alg, ed25519Alg) && len(raw) == 32 {
		return "did:key:z" + base58btc(append(append([]byte(nil), ed25519Multicodec...), raw...))
	}
	return "u" + base64.RawURLEncoding.EncodeToString(raw)
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58btc encodes b in the Bitcoin base58 alphabet, one leading '1' per
// leading zero byte.
func base58btc(b []byte) string {
	n := new(big.Int).SetBytes(b)
	base, mod := big.NewInt(58), new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

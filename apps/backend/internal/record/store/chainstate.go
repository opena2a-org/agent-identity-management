package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
)

// Querier runs statements. *sql.DB, *sql.Conn and *sql.Tx satisfy it.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ChainStatus is one organization's chain state as ReadChainState reads it.
type ChainStatus struct {
	State record.ChainState `json:"chainState"`
	// ChainID is nil when no chain started.
	ChainID *string `json:"chainId"`
	// Head is the head stored for the chain, nil when no chain started. When
	// the chain is not extendable it is the stored head that cannot be linked
	// to.
	Head *HeadView `json:"head"`
	// Reason names why a started chain is not extendable, and is empty
	// otherwise.
	Reason NotExtendableReason `json:"reason,omitempty"`

	keyID string
}

// ChainReport is a chain state as every surface that reports one answers
// it: the members of the ChainStatus that ReadChainState read, and
// latestCheckpoint. The body of GET /api/v1/admin/audit-logs/chain/head and
// the output of aim-breakglass chain status --json are both this value.
type ChainReport struct {
	ChainStatus
	// LatestCheckpoint is always nil: no chain checkpoint is written yet.
	LatestCheckpoint *struct{} `json:"latestCheckpoint"`
}

// HeadView is a stored chain head.
type HeadView struct {
	Seq  int64  `json:"seq"`
	Hash string `json:"hash"`
}

// NotExtendableReason names why a started chain cannot be extended. The set
// is closed.
type NotExtendableReason string

const (
	// NotExtendableNoRecords: the chain row exists and the chain holds no
	// record, its genesis included.
	NotExtendableNoRecords NotExtendableReason = "no_records"
	// NotExtendableHeadMismatch: the stored head does not name the chain's
	// newest record by sequence number and hash.
	NotExtendableHeadMismatch NotExtendableReason = "head_mismatch"
	// NotExtendableRecordModified: the newest record's stored bytes are not a
	// record payload whose hash is the stored hash.
	NotExtendableRecordModified NotExtendableReason = "record_modified"
)

// chainStateQuery is the one statement that reads a chain's state: the chain
// row of the organization and the chain's newest record, in one snapshot.
const chainStateQuery = `
SELECT c.id, c.key_id, c.head_seq, c.head_hash,
       r.seq, r.record_hash, r.payload_type, r.payload
  FROM record_chains c
  LEFT JOIN LATERAL (
        SELECT seq, record_hash, payload_type, payload
          FROM audit_records
         WHERE chain_id = c.id
         ORDER BY seq DESC
         LIMIT 1) r ON TRUE
 WHERE c.organization_id = $1`

// ReadChainState reads the state of an organization's chain. It is the one
// reading of chain state: the writer calls it under the append lock and
// refuses to extend any chain it does not report extendable, and every
// surface that reports a chain's state calls it.
//
// It checks that the head can be linked to, not that the chain verifies:
// walking the chain from its genesis is the verifier's work.
func ReadChainState(ctx context.Context, q Querier, organizationID string) (ChainStatus, error) {
	if !canonicalUUID(organizationID) {
		return ChainStatus{}, fmt.Errorf("%w: the organization id is not a lowercase hyphenated UUID", ErrInvalidWrite)
	}
	var (
		chainID, keyID, headHash string
		headSeq                  int64
		seq                      sql.NullInt64
		hash, payloadType        sql.NullString
		payload                  []byte
	)
	err := q.QueryRowContext(ctx, chainStateQuery, organizationID).
		Scan(&chainID, &keyID, &headSeq, &headHash, &seq, &hash, &payloadType, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return ChainStatus{State: record.ChainNotStarted}, nil
	}
	if err != nil {
		return ChainStatus{}, fmt.Errorf("store: read chain state: %w", err)
	}
	status := ChainStatus{
		State:   record.ChainExtendable,
		ChainID: &chainID,
		Head:    &HeadView{Seq: headSeq, Hash: headHash},
		keyID:   keyID,
	}
	switch {
	case !seq.Valid:
		status.State, status.Reason = record.ChainNotExtendable, NotExtendableNoRecords
	case seq.Int64 != headSeq || hash.String != headHash:
		status.State, status.Reason = record.ChainNotExtendable, NotExtendableHeadMismatch
	case payloadType.String != record.PayloadTypeRecordV1 || hashHex(payload) != hash.String:
		status.State, status.Reason = record.ChainNotExtendable, NotExtendableRecordModified
	}
	return status, nil
}

// ReadChain returns a chain's records in sequence order, in the form Verify
// takes them.
func ReadChain(ctx context.Context, q Querier, chainID string) ([]record.Record, error) {
	if !canonicalUUID(chainID) {
		return nil, fmt.Errorf("%w: the chain id is not a lowercase hyphenated UUID", ErrInvalidWrite)
	}
	rows, err := q.QueryContext(ctx, `
SELECT payload_type, payload, key_id, signature, record_hash,
       tenant_part, tenant_salt, personal_part, personal_salt
  FROM audit_records
 WHERE chain_id = $1
 ORDER BY seq`, chainID)
	if err != nil {
		return nil, fmt.Errorf("store: read chain: %w", err)
	}
	out, err := scanRecords(rows)
	if err != nil {
		return nil, fmt.Errorf("store: read chain: %w", err)
	}
	return out, nil
}

// scanRecords reads rows of payload_type, payload, key_id, signature,
// record_hash and the two parts with their salts as records, and closes
// rows.
func scanRecords(rows *sql.Rows) ([]record.Record, error) {
	defer rows.Close()
	var out []record.Record
	for rows.Next() {
		var (
			payloadType, keyID, recordHash string
			payload, signature             []byte
			rec                            record.Record
		)
		if err := rows.Scan(&payloadType, &payload, &keyID, &signature, &recordHash,
			&rec.TenantPart, &rec.TenantSalt, &rec.PersonalPart, &rec.PersonalSalt); err != nil {
			return nil, err
		}
		rec.Envelope = record.Envelope{
			Payload:     base64.StdEncoding.EncodeToString(payload),
			PayloadType: payloadType,
			Signatures:  []record.Signature{{KeyID: keyID, Sig: base64.StdEncoding.EncodeToString(signature)}},
		}
		rec.RecordHash = recordHash
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// canonicalUUID reports whether s is a UUID in lowercase hyphenated form.
func canonicalUUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s
}

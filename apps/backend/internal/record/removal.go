package record

import (
	"errors"
	"fmt"
	"sort"
)

// RemovedChain is what a tombstone states about a chain removed whole: the
// chain's final head, and the sequence number of the checkpoint the removal
// verified the chain against before it removed the chain.
type RemovedChain struct {
	// Final names the removed chain, its final sequence number and its final
	// record's hash.
	Final Head
	// CheckpointSeq is the sequence number of the checkpoint the chain was
	// verified against before its removal: from 0 to Final.Seq.
	CheckpointSeq int64
}

// ChainOutcome is VerifyChainSet's finding about one chain. The set is
// closed.
type ChainOutcome string

const (
	// OutcomeVerified: the chain is present, its records verify, and every
	// held checkpoint that names it agrees with them.
	OutcomeVerified ChainOutcome = "verified"
	// OutcomeFailed: the chain is present, and its records do not verify, or
	// a held checkpoint that names it does not agree with them.
	OutcomeFailed ChainOutcome = "failed"
	// OutcomeRemoved: the chain is absent, and exactly one tombstone names
	// it. That tombstone names a checkpoint sequence from 0 to its final
	// sequence, and its final sequence is at or above every held checkpoint
	// of the chain, with the hash each held checkpoint at that sequence
	// names.
	OutcomeRemoved ChainOutcome = "removed"
	// OutcomeRemovedWithoutRecord: the chain is absent, and no tombstone
	// accounts for its removal: none names it, more than one does, or the one
	// that does names a checkpoint sequence outside 0 to its final sequence,
	// sits below a held checkpoint of the chain, or names another final
	// record than a held checkpoint at its final sequence.
	OutcomeRemovedWithoutRecord ChainOutcome = "removed_without_record"
)

// ChainFinding is VerifyChainSet's finding about one chain.
type ChainFinding struct {
	ChainID string
	Outcome ChainOutcome
	// Checkpoints is the number of held checkpoints that name the chain. A
	// chain verified against none shows nothing about records removed from
	// its end.
	Checkpoints int
	// Failure is set when Outcome is OutcomeFailed: the walk's failure, or
	// else the failure of the first held checkpoint of the chain, in
	// sequence order, that does not agree with the walk.
	Failure *Failure
	// Detail says why the removal is not accounted for when Outcome is
	// OutcomeRemovedWithoutRecord, and is empty otherwise.
	Detail string
}

// UnreadCheckpoint is a held checkpoint VerifyChainSet could not read.
type UnreadCheckpoint struct {
	// Index is the checkpoint's position among the held checkpoints.
	Index int
	// Reason is why it did not open as a checkpoint.
	Reason Reason
}

// ChainSetReport is what VerifyChainSet finds.
type ChainSetReport struct {
	// Chains holds one finding per chain, sorted by chain id: each chain
	// present, and each absent chain that a held checkpoint or a tombstone
	// names.
	Chains []ChainFinding
	// Unread holds each held checkpoint that did not open as a checkpoint, in
	// the order given. It names no chain, so it accounts for none.
	Unread []UnreadCheckpoint
}

// VerifyChainSet reports on the chains of one deployment together: the
// chains the deployment holds, and the chains that checkpoints held outside
// it or tombstones name. A chain removed whole, its records and its chain
// row together, leaves no record to walk; a checkpoint of it held outside
// the deployment still names it.
//
// present holds the walk of every chain the deployment holds, as Verify
// returns it, by the id of the chain whose records were walked: a chain
// absent from present is absent from the deployment. held are checkpoints
// held outside the deployment. Each is read once it opens as a checkpoint
// under keys, the deployment's record keys the verifier holds, or under the
// keys a walk in present saw; keys may be nil. One that opens under neither
// is reported in Unread, never dropped. removed are the tombstones of chains
// removed whole.
//
// A chain in present is OutcomeVerified when its walk verified at least one
// record and reached the end of the sequence, the records walked are that
// chain's, and every held checkpoint that names it passes
// Result.VerifyCheckpoint. Otherwise it is OutcomeFailed with the first
// failure. A tombstone that names a chain in present changes nothing in its
// finding.
//
// A chain absent from present is OutcomeRemoved when exactly one tombstone
// names it, that tombstone's checkpoint sequence is from 0 to its final
// sequence, and its final sequence is at or above the sequence of every
// held checkpoint of the chain, with the same hash as each held checkpoint
// at that sequence. Any other absent chain is OutcomeRemovedWithoutRecord:
// a chain a held checkpoint names that is neither present and verifying nor
// accounted for by exactly one tombstone is reported, never left out.
//
// It returns an error and no report when a chain id in present is not a
// lowercase hyphenated UUID, and when a tombstone's final head has a chain
// id that is not, a negative sequence number, or a hash that is not 64
// lowercase hex characters.
func VerifyChainSet(present map[string]Result, held []Envelope, keys SignatureVerifier, removed []RemovedChain) (ChainSetReport, error) {
	ids := make([]string, 0, len(present))
	for id := range present {
		if !isCanonicalUUID(id) {
			return ChainSetReport{}, errors.New("record: a chain id of the set is not a lowercase hyphenated UUID")
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	tombstones := map[string][]RemovedChain{}
	for _, t := range removed {
		if err := checkHead(t.Final); err != nil {
			return ChainSetReport{}, fmt.Errorf("record: a tombstone's final head: %w", err)
		}
		tombstones[t.Final.ChainID] = append(tombstones[t.Final.ChainID], t)
	}

	// A held checkpoint opens under the verifier's keys, or under the keys a
	// walk saw, such as the next key of a rotation.
	open := keySet{keys}
	for _, id := range ids {
		open = append(open, present[id].ring)
	}
	var report ChainSetReport
	heldOf := map[string][]heldCheckpoint{}
	for i, env := range held {
		cp, err := OpenCheckpoint(env, open)
		if err != nil {
			reason, _ := ReasonOf(err)
			report.Unread = append(report.Unread, UnreadCheckpoint{Index: i, Reason: reason})
			continue
		}
		heldOf[cp.Head.ChainID] = append(heldOf[cp.Head.ChainID], heldCheckpoint{env: env, cp: cp})
	}

	named := map[string]bool{}
	for _, id := range ids {
		named[id] = true
	}
	for id := range heldOf {
		named[id] = true
		sort.SliceStable(heldOf[id], func(a, b int) bool { return heldOf[id][a].cp.Head.Seq < heldOf[id][b].cp.Head.Seq })
	}
	for id := range tombstones {
		named[id] = true
	}
	all := make([]string, 0, len(named))
	for id := range named {
		all = append(all, id)
	}
	sort.Strings(all)

	for _, id := range all {
		if res, ok := present[id]; ok {
			report.Chains = append(report.Chains, presentFinding(id, res, heldOf[id]))
			continue
		}
		report.Chains = append(report.Chains, absentFinding(id, heldOf[id], tombstones[id]))
	}
	return report, nil
}

// heldCheckpoint is a held checkpoint that opened, with its envelope.
type heldCheckpoint struct {
	env Envelope
	cp  Checkpoint
}

// keySet is the SignatureVerifier of the keys its members hold, asked in
// order. A key identifier names one key, so the first member that holds the
// key decides.
type keySet []SignatureVerifier

func (s keySet) VerifySignature(keyID string, message, signature []byte) error {
	for _, v := range s {
		if v == nil {
			continue
		}
		if err := v.VerifySignature(keyID, message, signature); !errors.Is(err, ErrUnknownKey) {
			return err
		}
	}
	return ErrUnknownKey
}

// presentFinding judges a chain the deployment holds by its walk and the
// checkpoints held of it, in sequence order.
func presentFinding(id string, res Result, held []heldCheckpoint) ChainFinding {
	f := ChainFinding{ChainID: id, Outcome: OutcomeFailed, Checkpoints: len(held)}
	switch {
	case res.Head != nil && res.Head.ChainID != id:
		f.Failure = &Failure{Position: 0, Kind: FailureBrokenLink,
			Detail: fmt.Sprintf("the records verify as chain %s, not as chain %s", res.Head.ChainID, id)}
	case res.Failure != nil:
		failure := *res.Failure
		f.Failure = &failure
	case res.Verified == 0:
		f.Failure = &Failure{Position: 0, Kind: FailureGenesis, Detail: "the chain holds no record, its genesis included"}
	default:
		for _, h := range held {
			if _, failure := res.VerifyCheckpoint(h.env); failure != nil {
				f.Failure = failure
				return f
			}
		}
		f.Outcome = OutcomeVerified
	}
	return f
}

// absentFinding decides whether exactly one tombstone accounts for the
// removal of a chain the deployment does not hold, against the checkpoints
// held of it, in sequence order.
func absentFinding(id string, held []heldCheckpoint, tombstones []RemovedChain) ChainFinding {
	f := ChainFinding{ChainID: id, Outcome: OutcomeRemovedWithoutRecord, Checkpoints: len(held)}
	switch len(tombstones) {
	case 0:
		f.Detail = "the chain is absent and no tombstone names it"
		return f
	case 1:
	default:
		f.Detail = fmt.Sprintf("the chain is absent and %d tombstones name it; exactly one accounts for a removal", len(tombstones))
		return f
	}
	t := tombstones[0]
	if t.CheckpointSeq < 0 || t.CheckpointSeq > t.Final.Seq {
		f.Detail = fmt.Sprintf("the tombstone names checkpoint sequence %d, outside 0 to its final sequence %d", t.CheckpointSeq, t.Final.Seq)
		return f
	}
	for _, h := range held {
		seq := h.cp.Head.Seq
		if seq > t.Final.Seq {
			f.Detail = fmt.Sprintf("the tombstone's final sequence %d is below the held checkpoint at sequence %d", t.Final.Seq, seq)
			return f
		}
		if seq == t.Final.Seq && h.cp.Head.Hash != t.Final.Hash {
			f.Detail = fmt.Sprintf("the tombstone names another final record at sequence %d than the held checkpoint does", seq)
			return f
		}
	}
	f.Outcome = OutcomeRemoved
	return f
}

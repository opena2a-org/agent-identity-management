package record

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The chains of the deployment the chain-set tests share. setChainA is the
// fixture chain's id; the other two hold the same records under another id.
const (
	setChainA = fixtureChainID
	setChainB = "1b1b1b1b-0000-4000-8000-00000000000b"
	setChainC = "2c2c2c2c-0000-4000-8000-00000000000c"
)

// chainSet is a deployment of three chains signed by one record key: each
// chain's records and walk, and the checkpoints held outside the deployment,
// two of each chain in chain order, at its genesis and at its head.
type chainSet struct {
	kp      *memoryKeyProvider
	records map[string][]Record
	walks   map[string]Result
	held    []Envelope
}

func newChainSet(t *testing.T) chainSet {
	t.Helper()
	s := chainSet{kp: newMemoryKeyProvider(t), records: map[string][]Record{}, walks: map[string]Result{}}
	for _, id := range []string{setChainA, setChainB, setChainC} {
		records := signedChainNamed(t, id, s.kp)
		walk := mustVerify(t, records, s.kp.key(t))
		if !walk.OK() {
			t.Fatalf("chain %s does not verify", id)
		}
		s.records[id] = records
		s.walks[id] = walk
		genesis := Head{ChainID: id, Seq: 0, Hash: records[0].RecordHash}
		s.held = append(s.held,
			checkpointOf(t, genesis, fixtureStart.Add(time.Second), s.kp),
			checkpointOf(t, *walk.Head, fixtureCheckpointAt, s.kp))
	}
	return s
}

// signedChainNamed builds the fixture chain under chain id id and signs it
// with kp.
func signedChainNamed(t *testing.T, id string, kp *memoryKeyProvider) []Record {
	t.Helper()
	b := built(t)
	g := fixtureGenesis(kp.key(t), fixturePreGenesis())
	g.ChainID = id
	bodies := []Body{b(NewGenesis(g))}
	for _, d := range fixtureDrafts() {
		bodies = append(bodies, b(NewRecord(bodies[len(bodies)-1].Head(), d)))
	}
	records := make([]Record, len(bodies))
	for i, body := range bodies {
		records[i] = mustSign(t, body, kp)
	}
	return records
}

// without returns the walks of the deployment's chains other than ids: what
// the deployment holds once those chains are deleted.
func (s chainSet) without(ids ...string) map[string]Result {
	out := map[string]Result{}
	for id, walk := range s.walks {
		out[id] = walk
	}
	for _, id := range ids {
		delete(out, id)
	}
	return out
}

// tombstoneOf states the removal of chain id at its head, verified against
// the checkpoint of its head.
func (s chainSet) tombstoneOf(id string) RemovedChain {
	head := *s.walks[id].Head
	return RemovedChain{Final: head, CheckpointSeq: head.Seq}
}

func mustVerifyChainSet(t *testing.T, present map[string]Result, held []Envelope, keys SignatureVerifier, removed []RemovedChain) ChainSetReport {
	t.Helper()
	report, err := VerifyChainSet(present, held, keys, removed)
	if err != nil {
		t.Fatalf("VerifyChainSet: %v", err)
	}
	return report
}

// findingOf returns the report's finding about chain id.
func findingOf(t *testing.T, report ChainSetReport, id string) ChainFinding {
	t.Helper()
	for _, f := range report.Chains {
		if f.ChainID == id {
			return f
		}
	}
	t.Fatalf("the report has no finding about chain %s", id)
	return ChainFinding{}
}

// wantOutcome asserts a finding's outcome and that its detail carries the
// given text.
func wantOutcome(t *testing.T, f ChainFinding, outcome ChainOutcome, detail string) {
	t.Helper()
	if f.Outcome != outcome {
		t.Fatalf("chain %s: outcome %s (detail %q, failure %v); want %s", f.ChainID, f.Outcome, f.Detail, f.Failure, outcome)
	}
	if !strings.Contains(f.Detail, detail) {
		t.Fatalf("chain %s: detail %q does not say %q", f.ChainID, f.Detail, detail)
	}
}

// wantPresentFailure asserts that a present chain's finding is failed with
// the given failure.
func wantPresentFailure(t *testing.T, f ChainFinding, position int, kind FailureKind, detail string) {
	t.Helper()
	if f.Outcome != OutcomeFailed {
		t.Fatalf("chain %s: outcome %s; want %s", f.ChainID, f.Outcome, OutcomeFailed)
	}
	wantCheckpointFailure(t, f.Failure, position, kind, detail)
}

func TestVerifyChainSet_ChainsPresentThatAgreeWithTheirHeldCheckpointsVerify(t *testing.T) {
	s := newChainSet(t)
	report := mustVerifyChainSet(t, s.walks, s.held, s.kp.key(t), nil)
	if len(report.Unread) != 0 {
		t.Fatalf("held checkpoints left unread: %+v", report.Unread)
	}
	var ids []string
	for _, f := range report.Chains {
		ids = append(ids, f.ChainID)
		if f.Outcome != OutcomeVerified || f.Failure != nil || f.Detail != "" || f.Checkpoints != 2 {
			t.Fatalf("chain %s: %+v; want verified against 2 held checkpoints", f.ChainID, f)
		}
	}
	if want := []string{setChainA, setChainB, setChainC}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("findings about %v; want one per chain in chain id order, %v", ids, want)
	}
}

// Golden case: a chain deleted whole, its records and its chain row
// together, leaves nothing in the deployment to verify. The checkpoints held
// of it still name it, so it is reported.
func TestVerifyChainSet_ReportsADeletionWithNoTombstone(t *testing.T) {
	s := newChainSet(t)
	report := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), nil)
	b := findingOf(t, report, setChainB)
	wantOutcome(t, b, OutcomeRemovedWithoutRecord, "no tombstone names it")
	if b.Checkpoints != 2 || b.Failure != nil {
		t.Fatalf("chain B: %+v; want the 2 held checkpoints that name it counted and no failure", b)
	}
	for _, id := range []string{setChainA, setChainC} {
		wantOutcome(t, findingOf(t, report, id), OutcomeVerified, "")
	}
}

// Golden case: a tombstone whose final record sits below a checkpoint held
// of the chain does not account for the chain's removal: records after it
// existed when the checkpoint was taken.
func TestVerifyChainSet_ReportsATombstoneBelowAHeldCheckpoint(t *testing.T) {
	s := newChainSet(t)
	early := RemovedChain{
		Final:         Head{ChainID: setChainB, Seq: 1, Hash: s.records[setChainB][1].RecordHash},
		CheckpointSeq: 0,
	}
	report := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), []RemovedChain{early})
	wantOutcome(t, findingOf(t, report, setChainB), OutcomeRemovedWithoutRecord,
		"final sequence 1 is below the held checkpoint at sequence 3")
}

// Golden case: a chain removed with a tombstone at its head is reported as
// removed, and the findings about the other chains are what they were before
// the removal.
func TestVerifyChainSet_AProperRemovalLeavesEveryOtherChainsFindingUnchanged(t *testing.T) {
	s := newChainSet(t)
	before := mustVerifyChainSet(t, s.walks, s.held, s.kp.key(t), nil)
	after := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), []RemovedChain{s.tombstoneOf(setChainB)})

	b := findingOf(t, after, setChainB)
	wantOutcome(t, b, OutcomeRemoved, "")
	if b.Detail != "" || b.Failure != nil || b.Checkpoints != 2 {
		t.Fatalf("chain B: %+v; want removed, checked against its 2 held checkpoints", b)
	}
	for _, id := range []string{setChainA, setChainC} {
		if got, want := findingOf(t, after, id), findingOf(t, before, id); !reflect.DeepEqual(got, want) {
			t.Fatalf("chain %s: %+v after the removal; want %+v, as before it", id, got, want)
		}
	}
	if len(after.Chains) != 3 || len(after.Unread) != 0 {
		t.Fatalf("report %+v; want three findings and no unread checkpoint", after)
	}
}

func TestVerifyChainSet_ReportsATombstoneThatNamesAnotherRecordAtAHeldCheckpoint(t *testing.T) {
	s := newChainSet(t)
	other := s.tombstoneOf(setChainB)
	other.Final.Hash = s.records[setChainB][2].RecordHash
	report := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), []RemovedChain{other})
	wantOutcome(t, findingOf(t, report, setChainB), OutcomeRemovedWithoutRecord,
		"another final record at sequence 3 than the held checkpoint")
}

func TestVerifyChainSet_TwoTombstonesOfOneChainAccountForNoRemoval(t *testing.T) {
	s := newChainSet(t)
	tombstone := s.tombstoneOf(setChainB)
	report := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), []RemovedChain{tombstone, tombstone})
	wantOutcome(t, findingOf(t, report, setChainB), OutcomeRemovedWithoutRecord, "2 tombstones name it")
}

// A removal verifies the chain against a checkpoint of it before it removes
// the chain, so a tombstone that names a checkpoint above its own final
// record, or a negative sequence, accounts for no removal.
func TestVerifyChainSet_ReportsATombstoneVerifiedAgainstACheckpointOutsideItsChain(t *testing.T) {
	s := newChainSet(t)
	for _, seq := range []int64{4, -1} {
		tombstone := s.tombstoneOf(setChainB)
		tombstone.CheckpointSeq = seq
		report := mustVerifyChainSet(t, s.without(setChainB), s.held, s.kp.key(t), []RemovedChain{tombstone})
		wantOutcome(t, findingOf(t, report, setChainB), OutcomeRemovedWithoutRecord, "names checkpoint sequence")
	}
}

// A tombstone accounts for a chain no held checkpoint names. A chain that
// the deployment no longer holds and that nothing names is unknown to the
// verifier: a checkpoint held from its genesis on is what names it.
func TestVerifyChainSet_AChainNamedOnlyByItsTombstoneIsRemoved(t *testing.T) {
	s := newChainSet(t)
	heldOfA := s.held[:2]
	report := mustVerifyChainSet(t, s.without(setChainB, setChainC), heldOfA, s.kp.key(t), []RemovedChain{s.tombstoneOf(setChainB)})
	b := findingOf(t, report, setChainB)
	wantOutcome(t, b, OutcomeRemoved, "")
	if b.Checkpoints != 0 {
		t.Fatalf("chain B: %d held checkpoints counted; want 0", b.Checkpoints)
	}
	if len(report.Chains) != 2 {
		t.Fatalf("findings %+v; want chains A and B alone", report.Chains)
	}
}

// A present chain is judged by its records and the checkpoints held of it.
func TestVerifyChainSet_ReportsAPresentChainThatFails(t *testing.T) {
	s := newChainSet(t)
	key := s.kp.key(t)

	t.Run("records removed from its end", func(t *testing.T) {
		present := s.without()
		present[setChainB] = mustVerify(t, s.records[setChainB][:2], key)
		report := mustVerifyChainSet(t, present, s.held, key, nil)
		wantPresentFailure(t, findingOf(t, report, setChainB), 3, FailureTruncated, "records that verified end at sequence 1")
	})

	t.Run("a record whose signature does not verify", func(t *testing.T) {
		tampered := cloneRecords(s.records[setChainB])
		flipSignatureByte(t, &tampered[2].Envelope.Signatures[0], 7)
		present := s.without()
		present[setChainB] = mustVerify(t, tampered, key)
		report := mustVerifyChainSet(t, present, s.held, key, nil)
		// The walk's own failure, not the held checkpoint it then leaves
		// unreached.
		wantPresentFailure(t, findingOf(t, report, setChainB), 2, FailureBadSignature, "")
	})

	t.Run("no record, its genesis included", func(t *testing.T) {
		present := s.without()
		present[setChainB] = mustVerify(t, nil, key)
		report := mustVerifyChainSet(t, present, s.held, key, nil)
		wantPresentFailure(t, findingOf(t, report, setChainB), 0, FailureGenesis, "holds no record")
	})

	t.Run("another chain's records under its id", func(t *testing.T) {
		present := s.without()
		present[setChainB] = s.walks[setChainA]
		report := mustVerifyChainSet(t, present, nil, key, nil)
		wantPresentFailure(t, findingOf(t, report, setChainB), 0, FailureBrokenLink, "records verify as chain "+setChainA)
	})
}

// A tombstone states a removal. A chain the deployment still holds is judged
// by its records and held checkpoints, and a tombstone that names it changes
// nothing in the report.
func TestVerifyChainSet_ATombstoneOfAPresentChainChangesNoFinding(t *testing.T) {
	s := newChainSet(t)
	without := mustVerifyChainSet(t, s.walks, s.held, s.kp.key(t), nil)
	with := mustVerifyChainSet(t, s.walks, s.held, s.kp.key(t), []RemovedChain{s.tombstoneOf(setChainB)})
	if !reflect.DeepEqual(with, without) {
		t.Fatalf("report %+v with the tombstone; want %+v", with, without)
	}
}

// A held checkpoint that does not open names no chain. It is reported, never
// dropped, and changes no finding.
func TestVerifyChainSet_ReportsAHeldCheckpointItCannotRead(t *testing.T) {
	s := newChainSet(t)
	stranger := newMemoryKeyProvider(t)
	held := append(append([]Envelope(nil), s.held...),
		checkpointOf(t, *s.walks[setChainB].Head, fixtureCheckpointAt, stranger),
		s.records[setChainA][0].Envelope)
	report := mustVerifyChainSet(t, s.walks, held, s.kp.key(t), nil)
	want := []UnreadCheckpoint{
		{Index: len(held) - 2, Reason: ReasonUnknownKey},
		{Index: len(held) - 1, Reason: ReasonPayloadTypeMismatch},
	}
	if !reflect.DeepEqual(report.Unread, want) {
		t.Fatalf("unread %+v; want %+v", report.Unread, want)
	}
	if b := findingOf(t, report, setChainB); b.Outcome != OutcomeVerified || b.Checkpoints != 2 {
		t.Fatalf("chain B: %+v; want verified against its 2 held checkpoints", b)
	}
}

// After a key rotation the next key signs a chain's checkpoints. A verifier
// that holds only the chain's first key, or no key, reads them under the keys
// the walk saw; with no walk to see the key, the checkpoint is reported
// unread.
func TestVerifyChainSet_ReadsAHeldCheckpointUnderTheKeysTheWalkSaw(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)
	walk := mustVerify(t, records, first)
	held := []Envelope{checkpointOf(t, *walk.Head, fixtureCheckpointAt, keys[2])}

	for name, verifierKeys := range map[string]SignatureVerifier{"the first key": first, "no key": nil} {
		t.Run(name, func(t *testing.T) {
			report := mustVerifyChainSet(t, map[string]Result{fixtureChainID: walk}, held, verifierKeys, nil)
			if len(report.Unread) != 0 {
				t.Fatalf("unread %+v; want the checkpoint read under the walk's keys", report.Unread)
			}
			if f := findingOf(t, report, fixtureChainID); f.Outcome != OutcomeVerified || f.Checkpoints != 1 {
				t.Fatalf("finding %+v; want verified against 1 held checkpoint", f)
			}
		})
	}

	report := mustVerifyChainSet(t, map[string]Result{}, held, first, nil)
	if want := []UnreadCheckpoint{{Index: 0, Reason: ReasonUnknownKey}}; !reflect.DeepEqual(report.Unread, want) {
		t.Fatalf("unread %+v; want %+v", report.Unread, want)
	}
	if len(report.Chains) != 0 {
		t.Fatalf("findings %+v; want none: the unread checkpoint names no chain", report.Chains)
	}
}

func TestVerifyChainSet_RefusesInvalidInput(t *testing.T) {
	s := newChainSet(t)
	key := s.kp.key(t)

	upper := map[string]Result{strings.ToUpper(setChainB): s.walks[setChainB]}
	if _, err := VerifyChainSet(upper, nil, key, nil); err == nil {
		t.Fatalf("a chain id in upper case was accepted")
	}

	cases := map[string]func(*RemovedChain){
		"a chain id that is not a UUID form": func(r *RemovedChain) { r.Final.ChainID = "not-a-chain" },
		"a negative final sequence":          func(r *RemovedChain) { r.Final.Seq = -1 },
		"a hash that is not lowercase hex":   func(r *RemovedChain) { r.Final.Hash = strings.ToUpper(r.Final.Hash) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			tombstone := s.tombstoneOf(setChainB)
			change(&tombstone)
			if _, err := VerifyChainSet(s.without(setChainB), s.held, key, []RemovedChain{tombstone}); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("got %v; want ErrInvalidRecord", err)
			}
		})
	}
}

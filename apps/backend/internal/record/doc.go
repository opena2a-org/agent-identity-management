// Package record holds the byte-level rules of an audit record: its canonical
// form, its place in a hash chain, the signed envelope around it, and a
// verifier that walks a sequence of records.
//
// A record is three JSON objects with disjoint member sets: the retained part,
// the tenant part and the personal part. The bytes the chain and the
// signatures cover are the RFC 8785 (JCS) serialization of the retained part,
// over a restricted data model: no floating point number, integers within 53
// bits, timestamps in RFC 3339 UTC truncated to microseconds. Values that can
// be erased later (the tenant and personal parts) never enter those bytes; the
// retained part carries a salted SHA-256 commitment to each instead.
//
// Each record names its chain, its sequence number (0 at genesis, then plus
// one) and the SHA-256 of its predecessor's canonical bytes (null at genesis),
// so the newest record's hash commits to the whole history.
//
// A record and a chain checkpoint are each carried in a DSSE v1 envelope
// (https://github.com/secure-systems-lab/dsse) signed with the deployment's
// record key. The record key signs a closed set of payload classes, each with
// exactly one payload type, and the bytes it signs begin with that payload
// type, so a signature made for one class never verifies as another. The key
// is reached only through KeyProvider, whose one signing operation takes a
// class from the closed set and a Payload that only this package's serializer
// builds, so no caller can have the record key sign a type or bytes of its own
// choosing. Open compares an envelope's payload type with the one class the
// caller expects before it reads anything else in the envelope.
//
// A chain's genesis names its first key. A key rotation record, signed by the
// key in force, names the next key and the reason (scheduled, custody change
// or exposure, with the time an exposure is believed to have begun), all in
// its retained part. Verify accepts a later key only through such a record at
// its position in the chain, and fails a record after the rotation that the
// retired key signs, whatever that record's timestamp says.
//
// A correction is a new record that names, by event_id, an earlier record of
// its chain that it corrects; the corrected record is left as it is. The
// reference sits in the correction's tenant part, so erasing that part removes
// it and leaves the chain verifying. Verify fails a correction that commits to
// no tenant part, or whose tenant part, while it exists, names no record
// before it in the chain.
//
// Verify walks records from the genesis and reports the first position at
// which the chain or a signature fails. It cannot see records removed from the
// end of a sequence: that needs a checkpoint, a signed statement of a chain's
// head at one moment, held outside the sequence. A checkpoint is built by
// this package's serializer alone, signed under the checkpoint class by the
// key in force after its head, and read back by a verifier that accepts it
// under that key alone. Result.VerifyCheckpoint compares a held checkpoint
// with the walk and reports records removed from the end, and a record at
// the checkpoint's sequence other than the one the checkpoint names. An
// empty sequence is reported as no chain started, never as verified.
//
// A chain removed whole, its records and its chain row together, leaves no
// sequence to walk. VerifyChainSet reports on a deployment's chains
// together: the walks of the chains the deployment holds, the checkpoints
// held outside it, and tombstones, each of which states that a chain was
// removed whole at a final head. A chain a held checkpoint names that is
// neither present and verifying nor accounted for by exactly one tombstone,
// at or above every held checkpoint of the chain, is reported as removed
// without record.
//
// The package is pure: it reads and writes no database and serves no route.
// Its subpackage store keeps chains in PostgreSQL: it starts a chain, appends
// to it under the chain's append lock and reads a chain's state, which is one
// of the three ChainState values. Its writer appends a key rotation, and the
// statement that appends it moves the chain to the next key; it appends a
// correction only of a record of the organization's chain. A reduction whose
// record cannot be appended commits with a debt row instead, and the store's
// settler appends the late record built from that row. At process start the
// store reads whether a foreign key still removes audit rows by cascade:
// while one does and no chain has started, the deployment is in the pre-chain
// state, no chain starts and a write of an ordinary record commits with no
// record. Not built here: the store writes no checkpoint yet, the placement
// table that decides which member sits in which part, and the record that
// states a chain's removal.
package record

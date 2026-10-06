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
// Verify walks records from the genesis and reports the first position at
// which the chain or a signature fails. It cannot see records removed from the
// end of a sequence: that needs a head held outside the sequence, which this
// package does not build yet. An empty sequence is reported as no chain
// started, never as verified.
//
// The package is pure: it reads and writes no database and serves no route.
// Its subpackage store keeps chains in PostgreSQL: it starts a chain, appends
// to it under the chain's append lock and reads a chain's state, which is one
// of the three ChainState values. A reduction whose record cannot be appended
// commits with a debt row instead, and the store's settler appends the late
// record built from that row. Not built here: key rotation and correction
// records, checkpoints, and the placement table that decides which member
// sits in which part.
package record

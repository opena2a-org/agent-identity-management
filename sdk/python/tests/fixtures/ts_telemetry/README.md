# TS-reference telemetry fixtures (AIM-01.AC1)

Cross-SDK schema fixtures for the ARP telemetry seam. Each file pins one
representative input shape and the JSONL record the **TypeScript reference**
emits for it, so the Python seam (`aim_sdk.telemetry`) can be asserted
field-for-field against the same local log format.

Reference implementation (same repo, no external fetch):

- `sdk/typescript/src/telemetry/correlated-record.ts`
  — `buildCorrelatedRecord()` (record shape, builder-owned `observed` /
  `attribution` flags) and `toSharedIndicator()` (the shared-indicator
  reduction).
- `sdk/typescript/src/telemetry/local-writer.ts`
  — `writeCorrelatedRecord()` (`correlated-events.jsonl`, one compact
  `JSON.stringify` line per record).

## How each fixture was derived

The `expectedRecord` / `expectedIndicator` blocks are the emitted shapes of the
TS functions above for the given `input`: every field the TS interface declares,
with TS-optional fields **omitted** exactly where `JSON.stringify` drops an
`undefined` (that omission is itself part of the schema and is asserted).

Fields whose value is generated per-call cannot be pinned in a fixture and are
listed in `volatileFields`; the test asserts their *format* instead
(`recordId`, `createdAt`, and the indicator's `triggeredAt`).

## Key order

JSON object key order is not part of the schema: the TS builder emits nested
enforcement/intent/detection keys in *caller* order (object spread), so no fixed
byte order exists to match. `test_arp_telemetry_seam_parity.py` therefore
compares **canonicalized** JSON (`sort_keys=True`, compact separators) — the
field names, the field values, and the set of present/omitted keys all have to
match byte-for-byte — and separately asserts the emitted line is compact, as
`JSON.stringify` produces.

## Cases

| File | Shape | `assembly.completeness` |
| --- | --- | --- |
| `full-join.json` | enforcement + intent + detection, every optional populated | `full` |
| `deny-detection-only.json` | enforcement + detection, optionals omitted | `partial-missing-intent` |
| `allow-intent-only.json` | enforcement + intent, allow verdict | `partial-missing-detection` |
| `enforcement-only.json` | enforcement alone (the anchor fact) | `partial-missing-both` |

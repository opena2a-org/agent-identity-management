# ARP behavioral cases shared with the TypeScript SDK

`behavioral-cases.json` holds the behavioral suite for the Python engine port
(`aim_sdk.arp`). Each case has a `kind` (the component it exercises), an
`input`, and the `expected` output that the **TypeScript reference**
(`sdk/typescript/src/arp`) gives for that input.

Two suites run the file:

- `sdk/typescript/tests/arp/python-port-parity.test.ts`, through
  `sdk/typescript/tests/arp/python-parity/run-case.ts`, checks that the TS SDK
  still gives every `expected` output;
- `sdk/python/tests/test_arp_engine_parity.py` checks that the Python port
  gives the same.

A behavior change in either SDK fails one of the two. Make the change in both
SDKs, then update `expected` to the TS output. The TS runner returns that
output (`runCase(kind, input)`), so a case's new `expected` is what it returns.

Outputs leave out what differs on every run (event ids and timestamps) and are
compared as JSON, so a field the TS code leaves `undefined` is absent and the
Python port must leave it out too rather than write `null`.

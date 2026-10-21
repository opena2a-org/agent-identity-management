# AIM server features

What the AIM server adds over the SDKs' local checks: the authorization engine, the trust score, attestation of MCP servers, privileged access management, security event forwarding to Splunk and Microsoft Sentinel, agent credentials read from a CyberArk vault, and the web dashboard. Available in the self-hosted and AIM Cloud deployment modes, except SIEM forwarding and the CyberArk secrets backend, which run in self-hosted deployments only.

## 5-step Fine-Grained Authorization

`POST /api/v1/agents/:id/authorize` and the SDK verification route `POST /api/v1/sdk-api/verifications` run an agent's request for a capability through the checks below, in order, and stop at the first one that denies. When the agent has no FGA policy for that capability, the request is decided under a default policy: no attribute, action or object restriction, no context rules, no chain rules, and the risk level detected from the capability (or the `riskLevel` an SDK verification names), so steps 2, 3 and 4 run and allow, and step 5 runs for HIGH and MEDIUM risk. The `fga.authorize` span says which applied in `fga.policy`: `stored` or `default`.

| Step | Check | Latency budget |
|---|---|---|
| 1 | Capability | <10ms |
| 2 | Attribute | <10ms |
| 3 | Context | <10ms |
| 4 | Chain | <10ms |
| 5 | Intent (NanoMind) | not on the decision path |

Step 5 runs only when the policy's risk level is HIGH or MEDIUM; it is skipped for LOW. It hands the request to a NanoMind daemon (`FGA_DAEMON_URL`, default `http://127.0.0.1:47200`) without holding the decision, records the daemon's classification, and never denies. Neither compose file in this repository starts that daemon.

The SDKs do not call `POST /api/v1/agents/:id/authorize`. `@perform_action` (Python) and `performAction()` (Java) post each call to `/api/v1/sdk-api/verifications`, where step 1 is the server's capability check: the capabilities granted to the agent, the minimum trust score of each capability's definition, the organization's enforcement mode and its security policies. Steps 2 to 5 then run on the agent's FGA policy for the capability, or on the default policy. A call's `context` may name what it touches under the members the authorize route takes in its body: `attributes` (a list of field names), `action` and `dataClass`; the attribute check reads them, and the tool call and attestation records keep them. A denial by the attribute, context or chain check is enforced as the organization's mode enforces every other denial on that route: in strict mode the call is denied with the check's reason, and in monitoring mode it runs, with the denial recorded in the audit entry and the verification event under `fgaDeniedBy` and `fgaDeniedReason`. Both records carry the decision's `fgaOutcome` and `fgaStepsTriggered`. The route records an `fga.authorize` span for each decision with one child span per step that ran.

## Trust-gated capabilities

The 9-factor trust score is calculated when an agent registers. It is recalculated when a manager verifies, suspends or reactivates the agent, grants or revokes one of its capabilities, or calls `POST /api/v1/agents/:id/trust-score/recalculate`. The trust re-evaluation job also recalculates the score of every agent that is not revoked, every hour by default (`TRUST_REEVALUATION_INTERVAL`, at least five minutes), and records a score only when it moved since the last one on record.

A verification request (`POST /api/v1/verifications`) does not recalculate the score. From an SDK the same request goes to `POST /api/v1/sdk-api/verifications`; the two routes share a handler. A request for a capability the agent was granted leaves the score unchanged. When a verified agent that holds at least one granted capability, and is not marked compromised, requests a capability outside its grants, the score drops by a fixed 0.10 if the request is blocked and 0.05 if it is let through, never below 0.

Each capability definition carries a minimum trust score: 0%, 30%, 50% and 70% for low-, medium-, high- and critical-risk capabilities. In strict enforcement mode a verification request for a granted capability is denied when the agent's trust score is below that minimum; in monitoring mode it is allowed. No route changes a minimum.

On a strict-mode organization, a score below the threshold of an enabled `trust_score_low` security policy can block a verification request for a granted capability; the policy's enforcement action decides whether it blocks or only raises an alert.

## MCP attestation

Multi-agent consensus. 3+ attesters across 2+ owners, each connecting and reporting the same tool set, equals verified. A server verified this way returns to pending when its attesters stop agreeing.

## Privileged Access Management

Three tiers: STANDARD, PRIVILEGED, SUPER_PRIVILEGED. The routes under `/api/v1/pam` keep the approvals, break-glass declarations and certification campaigns of the caller's organization; a record of another organization is answered `404`.

- `GET /api/v1/pam/tiers` lists the tier registry. A capability the registry does not hold is STANDARD.
- A member, manager or admin requests approval for a SUPER_PRIVILEGED capability of one of the organization's agents with `POST /api/v1/pam/approvals`. The request waits five minutes for an admin other than the requester to answer `APPROVED` or `DENIED` with `POST /api/v1/pam/approvals/:id/decision`; `GET /api/v1/pam/approvals/:id` reads its status.
- An admin declares a break-glass emergency for agents of the organization with `POST /api/v1/pam/emergencies`, for at most 60 minutes. An admin other than the declarer extends it once, by 15 minutes, with `POST /api/v1/pam/emergencies/:id/extend`, and an admin ends it with `POST /api/v1/pam/emergencies/:id/revoke`.
- An admin creates a certification campaign with `POST /api/v1/pam/campaigns`. While it is open, a manager or admin records a decision on an agent's capability with `POST /api/v1/pam/campaigns/:id/decisions`.

Requesting and deciding an approval, and declaring, extending and revoking an emergency, write an audit log entry. The capability verification and authorization routes do not read approvals, emergency declarations or certification decisions.

## CyberArk integration

A self-hosted server reads agent credentials from a CyberArk vault through the Central Credential Provider (CCP). With `CYBERARK_CCP_URL`, `CYBERARK_CCP_APP_ID` and `CYBERARK_CCP_SAFE` set, a secrets namespace created with `"backendType": "cyberark_ccp"` is held by the account named `aim-secrets-<namespace id>` in that safe: `POST /api/v1/secrets/resolve` reads the account through CCP for the agent that owns the namespace and answers with its content encrypted to the agent's key. The credential is stored and rotated in CyberArk; a store or a rotation through AIM is answered `409`. See [CyberArk Central Credential Provider](DEPLOYMENT.md#cyberark-central-credential-provider).

Sending session events to Privileged Session Manager (PSM) is under evaluation: no CyberArk interface that accepts them has been identified.

## SIEM adapters

Splunk HEC. Set `SIEM_SPLUNK_HEC_URL` and `SIEM_SPLUNK_HEC_TOKEN` and every audit log entry the server writes is forwarded: buffered batch delivery, retry, severity filtering. Microsoft Sentinel: set the six required `SIEM_SENTINEL_*` variables and the same events reach a Log Analytics table through the Azure Monitor Logs Ingestion API, with the client credentials of a Microsoft Entra application. The server does not use the HTTP Data Collector API, for which Microsoft ended support on 14 September 2026. See [SIEM Forwarding](DEPLOYMENT.md#siem-forwarding).

## Web dashboard

Available in Self-hosted and AIM Cloud modes.

![Fleet overview](images/dashboard-security.png)
*Fleet overview: agents monitored, actions blocked, and risk by category.*

![Agent registry](images/agents.png)
*Agent registry with trust scores and verification status per agent.*

![Per-agent trust score breakdown](images/agent-trust-score.png)
*Per-agent 9-factor trust score breakdown with weighted signal contributions.*

![MCP supply chain](images/supply-chain.png)
*MCP server dependencies with multi-agent attestation status.*

## Trust scoring

The local and server trust scores measure different things.

**Local (8 factors)** answers "is the agent's security posture set up correctly?" Computed from local files. Source: [`packages/aim-core/src/trust.ts`](https://github.com/opena2a-org/opena2a/blob/main/packages/aim-core/src/trust.ts).

| Factor | Weight | Signal |
|---|---|---|
| Identity | 20% | `identity.json` exists |
| Capabilities | 15% | `policy.yaml` exists |
| Audit log | 10% | `audit.jsonl` exists |
| Secrets managed | 15% | Secretless integration active |
| Config signed | 10% | ConfigGuard signatures present |
| Skills verified | 10% | HMA verification on skills |
| Network controlled | 10% | Egress policy detected |
| Heartbeat monitored | 10% | ARP runtime heartbeat present |

**Server (9 factors)** answers "is the agent behaving in a way that should still be trusted right now?" Recalculated on the events listed under [Trust-gated capabilities](#trust-gated-capabilities), not on each verification request. Source: [`apps/backend/internal/domain/trust_score.go`](../apps/backend/internal/domain/trust_score.go).

| Factor | Weight | Source |
|---|---|---|
| Verification status | 25% | Externally attested |
| Uptime | 15% | Observed |
| Action success rate | 15% | Observed |
| Security alerts | 15% | Observed |
| Compliance | 10% | Externally attested |
| Execution isolation | 10% | Externally attested |
| Agent age | 5% | Server-recorded |
| Drift detection | 3% | Observed |
| User feedback | 2% | Human input |

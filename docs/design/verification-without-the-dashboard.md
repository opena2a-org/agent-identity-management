# Verifying an agent without a trip to the dashboard: Design Note

**Status:** Draft for the owner's decision. Nothing in this note is implemented.
**Date:** 2026-11-25
**Audience:** the maintainers, and a security reviewer.

---

## 1. The problem

An organisation in strict mode does not verify an agent when it is registered. The agent is stored as `pending`, and stays so until an administrator or manager opens the agent's page in the dashboard and clicks "Verify agent". The developer registered the agent in code, from a terminal. Making them (or someone else) sign in to a web page to click one button for each agent is a poor experience. The owner's position is that an agent should be verified without that trip where that is safe, and that a hardware key may be what makes it safe.

This note looks at four ways to remove or shorten the trip and to raise assurance:

1. A short-lived verification grant given at device sign-in (section 4).
2. Verification from the terminal, signed with a WebAuthn security key or passkey (section 5).
3. Hardware-bound agent keys, with platform attestation, as a trust factor (section 6).
4. An organisation policy that verifies members' agents at registration up to a capability class (section 7).

Monitoring mode is in scope only where an option raises assurance there. The backend already means to verify at registration in monitoring mode; a defect there (section 2.2) is handled by another change and is not addressed here.

For each option the note gives: what the developer and the administrator do, what is stored and what is signed by which key, what three attackers gain compared with today, what it costs to build, and what can be undone. Section 8 compares them, section 9 recommends, section 10 lists the owner's open questions.

The three attackers are:

- **(a)** someone holding a stolen SDK refresh token;
- **(b)** someone at a developer's unlocked laptop;
- **(c)** someone who has a CI secret.

Every statement about the present system cites the file and line it was checked against. Where a statement could not be checked against this repository (mostly what hardware platforms can attest), the note says so.

## 2. How verification works today

### 2.1 Registration in strict mode

- Both SDKs register through `POST /api/v1/agents`. A member's user token or an organisation API key is accepted there (`apps/backend/cmd/server/main.go:1993`, `MemberOrAPIKeyMiddleware`).
- The SDK makes the agent's Ed25519 key pair on the client and sends only the public key:
  - TypeScript: `sdk/typescript/src/client/AIMClient.ts:431-444`.
  - Python: `sdk/python/aim_sdk/client.py:5678-5686`.
- When a public key is sent, the server stores it, records the algorithm as `"Ed25519"`, and stores no private key (`apps/backend/internal/application/agent_service.go:392-398`).
- Every agent starts as `pending` (`agent_service.go:461`). Verification at registration is skipped when the organisation is in strict mode (`agent_service.go:667`; `inStrictMode` at `agent_service.go:641-647`).
- The capabilities the agent declares are granted at registration in both modes, as its baseline (`agent_service.go:566-573`, and in one transaction at `agent_service.go:682-690`). A pending agent in strict mode therefore already holds its capabilities. What stops it from using them is its status: `VerifyCapability` refuses an unverified agent unless the organisation is in monitoring mode (`agent_service.go:1484-1492`).
- The agent row records the user who registered it (`created_by`, `agent_service.go:462-464`), and the SDK token or API key that was used (`agent_service.go:465-466`). The SDK token id comes from a client-supplied `X-SDK-Token` header (`apps/backend/internal/interfaces/http/middleware/sdk_token_tracking.go:29-34`). It is a label, not an authenticated binding.

### 2.2 Verification at registration in monitoring mode

- `shouldAutoVerifyAgent` requires a public key and a server-held encrypted private key, a trust score of at least 0.3, and a name, display name and description (`agent_service.go:825-846`).
- Because it requires `EncryptedPrivateKey != nil` (`agent_service.go:827`), an agent that registered its own public key (every SDK agent, section 2.1) is never verified at registration even in monitoring mode. That is the defect another change handles.

### 2.3 The dashboard click

- The route is `POST /api/v1/agents/:id/verify` behind `ManagerMiddleware` (`main.go:2002`). That middleware admits only the `admin` and `manager` roles (`apps/backend/internal/interfaces/http/middleware/admin.go:31-43`).
- The four roles are `admin`, `manager`, `member` and `viewer` (`apps/backend/internal/domain/user.go:14-17`).
- The handler checks that the agent belongs to the caller's organisation (`apps/backend/internal/interfaces/http/handlers/agent_handler.go:662-668`). It then calls `AgentService.VerifyAgent` with the user named as the actor (`agent_handler.go:671`) and writes an audit log row (`agent_handler.go:679-692`).
- `VerifyAgent` accepts only `pending` to `verified` (`agent_service.go:1158-1186`; transition table at `apps/backend/internal/domain/agent_status_transition.go:25`). It sets `verified_at`, records an `agent_verified` transition (`agent_service.go:1174`), recalculates the trust score and sends the `agent.verified` webhook.
- The dashboard shows the button to admins and managers only (`apps/web/app/dashboard/agents/[id]/page.tsx:322`, `:612`). It calls `POST /api/v1/agents/${id}/verify` (`apps/web/lib/api.ts:1375-1376`).
- There is no `verified_by` column on `agents`. The only migration that names `verified_by` is `apps/backend/migrations/108_add_isolation_attestation_verification.sql`, for isolation attestations. Who verified an agent is recorded only in the audit log and in the transition record.

### 2.4 What is signed today, and by which key

- **The click itself is signed by nothing the administrator holds.** It is authorised by a bearer access token: an HS256 JWT signed with the server's shared `JWT_SECRET` (`apps/backend/internal/infrastructure/auth/jwt.go:570`, validation accepts HMAC only at `jwt.go:652`).
- **The record of the change** is a transition record. Records are hash-chained and carried in DSSE envelopes signed with the deployment's record key (`apps/backend/internal/record/doc.go:17-27`). `agent_verified` is in the "expansion" class (`apps/backend/internal/record/transition/transition.go:137`). The record proves the server wrote it. It does not prove that the named user intended it.

### 2.5 Which credentials can call the verify route today

This matters for every attacker comparison below.

- **Device sign-in.** `aim-sdk login` uses the device grant (Python SDK only; `sdk/python/aim_sdk/cli.py:740`). On approval the poll mints the same user token pair a dashboard login gets, carrying the user's role (`apps/backend/internal/application/device_auth_service.go:205-210`).
  - The pair is stored in plaintext JSON in `~/.aim/sdk_credentials.json` with mode 0600 (`sdk/python/aim_sdk/cli.py:870-879`; `sdk/python/aim_sdk/credentials.py:48`, `:395`).
  - The session ends at most `JWT_SESSION_MAX_AGE` (default 8h) after sign-in (`jwt.go:412`).
  - The refresh token is rotated on use only by a server with Redis (`docs/security/token-rotation.md:16-17`).
- **SDK download.** A dashboard SDK download embeds a 90-day refresh token (`jwt.go:452-459`) in a family that may live up to `SDK_TOKEN_FAMILY_MAX_AGE`, default 8760h (`jwt.go:417`).
  - It cannot be used directly (`apps/backend/internal/interfaces/http/middleware/auth.go:80-84`).
  - Exchanged at `/auth/refresh`, it yields an access token with the user's current role (`apps/backend/internal/interfaces/http/handlers/auth_refresh_handler.go:465`).
- **The verify route has no live sign-in check.** `refuseUnlessLiveSignIn` (`apps/backend/internal/interfaces/http/handlers/credential_mint_family.go:36-46`) is called only by the SDK download, device approval, SDK recovery and API key creation routes. I found no call in `agent_handler.go`.
- **Consequence.** An admin's or manager's stored SDK refresh token, of either kind, can already verify any pending agent of the organisation without a browser. The dashboard click is a habit, not a control.
- **API keys and agent credentials cannot verify.** They set no role (`middleware/api_key.go`, `middleware/pqc_agent_auth.go`, from the search report), so `ManagerMiddleware` answers 401.

### 2.6 What can be undone today

- `suspend` moves `pending` or `verified` to `suspended`. `reactivate` moves `suspended` to `verified`. Nothing leaves `revoked` (`agent_status_transition.go:17-27`).
- There is no act that returns a verified agent to `pending`.
- Suspend, reactivate and revoke are manager-gated routes (`main.go:2004-2006`).
- A suspended agent that is reactivated goes straight back to `verified`. So suspending an agent that was verified wrongly and later reactivating it skips any fresh review.
- Replacing a verified agent's public key (`PUT /agents/:id/keys`, `MemberMiddleware`, `main.go:2008`) records `key_updated`. I found no status change in `UpdateAgentPublicKey` (`agent_service.go:2919` onwards). So verification today is bound to the agent's id, not to its key.

### 2.7 Pieces that exist and that the options can reuse

- **A `scope` field on device codes.** `device_codes.scope` and `DeviceCode.Scope` exist (`apps/backend/internal/domain/device_code.go:35`). The code-request handler accepts a `scope` (`apps/backend/internal/interfaces/http/handlers/device_auth_handler.go:36`), and `/verify` echoes it (`device_auth_handler.go:262`). Nothing else reads it, the Python SDK never sends one (`cli.py:593`), and the device page does not show it.
- **Device approval checks no role.** `/oauth/device/approve` is behind `AuthMiddleware` only (`main.go:1905-1907`), and `ApproveDeviceCode` checks no role (`device_auth_service.go:240-256`). Any signed-in user, a viewer included, can approve a device sign-in.
- **Capability risk levels.** `capability_definitions` carries `risk_level` (`low`, `medium`, `high`, `critical`) for the same `namespace:action` strings agents declare (`apps/backend/internal/domain/capability.go:83`; seeds at `apps/backend/migrations/043_create_capability_definitions_table.sql:40-67`). Each level has a default minimum trust score (`capability.go:94-106`) and execution mode (`capability.go:110-121`).
- **A privileged-capability registry.** `privileged_capability_registry` has tiers STANDARD, PRIVILEGED and SUPER_PRIVILEGED (`apps/backend/migrations/081_privileged_capability_registry.sql`). It does not match agent capabilities, for two reasons:
  - Its rows use dotted names (`db.read`, `infra.modify`), while agent capabilities have the form `namespace:action` (`capability.go:134`).
  - The table has no organisation column, and a capability it does not hold is STANDARD (`apps/backend/internal/application/pam_service.go:210-226`, `:250-259`).
- **An owner-signature gate with a passkey placeholder.** `AuthorizePrivilegedAction` checks an owner's signature over a statement that names one action (`apps/backend/internal/domain/privileged_authority.go:160-167`). It defines a `passkey` owner-key kind, but "assertion verification is not implemented here, so a passkey signature is refused" (`privileged_authority.go:50-54`). I found no caller of `AuthorizePrivilegedAction` outside tests.
- **No WebAuthn, FIDO2, TOTP, TPM, secure enclave or keychain support.** None in the backend, the web app or either SDK (from the search report, which covered `go.mod`, `package.json` and the SDK manifests). Multi-factor sign-in is listed as future work (`ROADMAP.md:47`).
- **The isolation-attestation trust factor.** Factor 9 is an SDK self-report, weighted 0.10 (`apps/backend/internal/application/trust_calculator.go:169`). An unverified report is capped (`trust_calculator.go:756-758`). Nothing writes `verified = TRUE` today, and TEE attestation is named as a future evidence class (`apps/backend/migrations/108_add_isolation_attestation_verification.sql:21-28`).

## 3. Principles every option keeps

- **A verification names one agent id and one public key.** Option 2 signs that pair. The other options record it. Section 10 asks whether a key replacement should return a verified agent to `pending`.
- **Every automatic verification is an `agent_verified` transition** whose record names the actor and the reason (grant id, policy version, or assertion id), so it can be found and undone in bulk.
- **Unknown means highest.** An unknown capability, an unreadable organisation setting, or an unparseable attestation counts as the most restrictive case. Today some reads fail open: an undefined capability requires no minimum trust score (`agent_service.go:2045-2062`), and an unknown PAM capability is STANDARD (`pam_service.go:220-221`). New code must not copy that.
- **Nothing here changes monitoring mode's existing behaviour** except where an option adds assurance.

## 4. Option 1: a verification grant at device sign-in

### 4.1 Steps

Developer, once per working session:

1. Runs `aim-sdk login --verify-agents 30m --up-to medium`. The flags are new. The SDK sends the request in the existing `scope` field, for example `agents:verify ttl=1800 class=medium`.
2. The browser opens `/device?user_code=...` as it does today.
3. The page shows the code, and a new box: "Agents this command line registers in the next 30 minutes, with capabilities up to medium risk, will be verified by you." The box is unticked by default, and its limits can be narrowed but not widened.
4. The developer approves. This is the one trip, the same one sign-in already makes.
5. Agents registered with this session's tokens inside the window, whose declared capabilities are all at or below the class, are stored as `verified`. Others stay `pending` as today.

Administrator:

- Nothing per agent.
- They choose, once, which roles may give a grant, and the largest window and class (section 10). The default proposed here is: admins and managers only, at most 60 minutes, at most `medium`.
- They can list and revoke live grants on a new dashboard page.

### 4.2 Stored, and signed by which key

- A new table `verification_grants`, one row per grant:
  - `id`, `organization_id`, `granted_by` (user id);
  - `device_code_id`, and `session_family` (the `sid` of the pair the device poll mints);
  - `max_risk_level`, `max_agents`, `expires_at`;
  - `created_at`, `revoked_at`, `used_count`.
- A new nullable `agents.verification_grant_id`, so every agent a grant verified can be found.
- **Binding.** The grant is bound to the session family taken from the validated token (`middleware/auth.go:175` sets `sid`). It is never bound to the client-supplied `X-SDK-Token` header (section 2.1). A refresh keeps the family, so the grant follows rotation. Logout or a reuse revocation ends the family and therefore the grant.
- **Signing.**
  - Nothing new is signed by a user-held key. The grant is created by an HS256-authorised request from a live dashboard session, as the click is today.
  - Each verification it causes is an `agent_verified` transition, signed in its DSSE envelope by the deployment record key, with the granting user as actor and the grant id in the record.
  - The `scope` string is never trusted. Whoever started the device flow chose it, so the server computes the grant from what the approving user ticked on the page.

### 4.3 What each attacker gains

**(a) Stolen SDK refresh token.**
- Today: if it belongs to an admin or manager, it can already verify any pending agent (section 2.5). If it belongs to a member, it can register agents that stay pending.
- With a grant: for an admin, nothing new. For a member allowed to give grants, a token stolen inside the window registers agents that come out verified, up to the class and the count. After the window it is as today.
- A token from an SDK download (a different family) never carries a grant.

**(b) Unlocked laptop.**
- Today: the same as (a) with the token in `~/.aim/sdk_credentials.json`, plus whatever browser session is open.
- With a grant: the same as (a) inside the window. The attacker can also start a new `aim-sdk login` with a grant. That needs the browser session to approve it, and on an unlocked laptop they may have that.

**(c) Compromised CI secret.**
- Today: CI usually holds an API key, which can register agents but not verify them (section 2.5). If it holds a copied `sdk_credentials.json` of an admin, see (a).
- With a grant: no change for API keys or SDK-download tokens, because grants bind only to a device sign-in family. A device sign-in copied into CI is capped at 8h by the session maximum age (`jwt.go:412`), and its grant at the grant's window.

**New risk: device-code phishing.**
- An attacker starts a device flow and sends a developer the link. Today a careless approval gives the attacker a session.
- With grants, that session could also verify agents. The box is unticked by default to reduce this. The page also shows that the code was requested from a different IP or user agent than the approving browser (both are stored, `device_code.go:42-43`).

### 4.4 Cost

Backend:
- A migration adding the `verification_grants` table and `agents.verification_grant_id`.
- `DeviceAuthService.ApproveDeviceCode` and the approve handler take the grant's limits (`device_auth_service.go:240`, `device_auth_handler.go:153`).
- `AgentService.CreateAgent` and `registerAgent` look up a live grant for the request's `sid` (`agent_service.go:359`, `:659`). The decision moves into one function shared with option 4.
- New routes: `GET /users/me/verification-grants`, `DELETE /users/me/verification-grants/:id`, and an admin list.
- An organisation setting for who may grant and the limits.

Web:
- The device page gets the grant box (`apps/web/app/device/page.tsx`).
- A grants list.

Python SDK:
- New flags on `login` (`cli.py:1648`).

TypeScript SDK:
- No device login exists (search report), so TypeScript users would get the grant only through the Python CLI's `sdk_credentials.json`, or after a TypeScript login is built.

### 4.5 Undoing it

- Revoking a grant (or logging out) stops further verifications at once.
- Agents it already verified stay verified. They can be found by `verification_grant_id` and suspended in bulk. There is no "back to pending" act (section 2.6), so the bulk action is suspend.

## 5. Option 2: verification from the terminal with a hardware-backed key

### 5.1 Steps

Administrator, once:

1. In the dashboard, with a live sign-in, registers a security key or passkey on their account (a WebAuthn `create` ceremony).

Administrator, per agent:

1. Runs `aim-sdk verify <agent-name-or-id>` while signed in with `aim-sdk login` as an admin or manager.
2. The CLI fetches a challenge from the server and shows the agent's name, id, the fingerprint of its public key, its declared capabilities and who registered it.
3. The administrator touches the key (and enters its PIN if the organisation requires user verification).
4. The CLI sends the assertion. The server checks it (section 5.3) and moves the agent to `verified`. No web page is opened.

`--all-pending --registered-by me` could sign one challenge per agent, one touch each. One touch for a list would be a later choice (section 10).

Developer: nothing. They tell the administrator the agent's name, or the administrator sees it in `aim-sdk verify --list-pending`.

### 5.2 Stored, and signed by which key

- A new table `webauthn_credentials`: `id`, `user_id`, `credential_id`, the COSE public key, `sign_count`, `aaguid`, the attestation format, `transports`, `user_verified_at_registration`, `created_at`, `last_used_at`, `revoked_at`.
- A new table `verification_challenges`: `id`, `organization_id`, `agent_id`, `agent_public_key_sha256`, `requested_by`, `challenge` (32 random bytes), `expires_at` (at most 2 minutes), `used_at`.
- **What is signed.** The authenticator signs `authenticatorData || SHA-256(clientDataJSON)` with the private key held on the security key, or by the platform for a passkey. `clientDataJSON` carries the server's challenge.
  - The challenge is not a bare nonce. It is the SHA-256 of a canonical statement: `aim-agent-verify-v1`, organisation id, agent id, the SHA-256 of the agent's public key, the key algorithm, the server's random nonce, issued-at and expires-at.
  - It is built in the same line-per-field way as `PrivilegedActionStatement.SigningBytes` (`privileged_authority.go:125-152`), with its own version prefix so it cannot be confused with a privileged-action statement.
- **What is recorded.** The assertion, the statement and the credential id go into the `agent_verified` transition record. A reviewer can then check the administrator's signature independently of the server's record key.

### 5.3 What the server must check

1. **The caller.** The request carries a live access token of a user who is `admin` or `manager` in the agent's organisation now. The role is read at the time of the request, as the refresh route already does (`auth_refresh_handler.go:465`).
2. **The credential.** The credential id is in `webauthn_credentials`, belongs to that same user, and is not revoked.
3. **The challenge.**
   - It was issued by this server to this user for this agent.
   - It has not expired and has not been used; mark it used in the same transaction as the status change.
   - The statement it hashes names the agent's id and the public key the agent holds at the moment of the status change. If the key was replaced after the challenge was issued, refuse.
4. **The WebAuthn assertion** (the WebAuthn Level 2 verification steps, done with a maintained library rather than by hand):
   - `type` is `webauthn.get`.
   - The challenge matches.
   - The origin is one the server is configured to accept for the CLI (see 5.5).
   - The `rpIdHash` is SHA-256 of the configured RP id.
   - The user-present flag is set, and the user-verified flag is set when the organisation requires it.
   - The signature verifies under the stored COSE key.
   - The sign count is greater than the stored one, unless both are zero; synced passkeys report zero.
5. **The agent.** The agent is still `pending`. The status write is conditional on that, as `SetAgentStatusTx` already is (search report: `changeAgentStatus` at `agent_service.go:2842-2853` writes only if the status is unchanged).

### 5.4 How an organisation requires it

- A new organisation setting `verification_requires_security_key` (boolean), set through the admin enforcement-settings route that already exists (`main.go:2125-2126`, `AdminMiddleware`).
- When it is on, `POST /agents/:id/verify` without a valid assertion is refused, so the dashboard button must also run a WebAuthn `get` ceremony in the browser. When it is off, the plain call still works.
- An optional `attestation_policy`: accept any authenticator, or only listed AAGUIDs, for example only hardware security keys and not synced passkeys. This is checked when a credential is registered.
- The setting should also apply to `reactivate`, which writes `verified` too (`agent_status_transition.go:27`). Otherwise suspend-and-reactivate goes around it.

### 5.5 What the terminal can and cannot reach (not checked in this repository)

- **Roaming FIDO2 security keys** (USB or NFC) can be used from a terminal through CTAP2 libraries such as `python-fido2` or `libfido2`. The client builds `clientDataJSON` itself, so the server must accept a configured CLI origin. That origin proves nothing about the caller; the binding comes from the challenge and the credential.
- **Platform passkeys** (Touch ID, Windows Hello, Android) are reached through operating system APIs.
  - Windows exposes `webauthn.dll` to native programs.
  - On macOS, a program must be a signed app with an associated-domains entitlement to use platform passkeys. A plain Python command cannot.
  - So on macOS "a passkey from the terminal" means either a roaming security key, or a small signed helper app.
- **Synced passkeys** are not bound to one device and usually give no attestation. An organisation that wants "a hardware key" needs the AAGUID allowlist above.

### 5.6 What each attacker gains

**(a) Stolen SDK refresh token.**
- Today: an admin's or manager's token verifies agents.
- With the setting on: it does not, because it has no security key. This is the main gain of this option. It closes the hole described in section 2.5.

**(b) Unlocked laptop.**
- If the security key is plugged in, and the organisation does not require user verification, the attacker can touch it. Requiring user verification (a PIN or biometric on the key) stops that.
- A platform passkey on that laptop is protected by the OS unlock prompt, which the attacker may not pass.

**(c) Compromised CI secret.**
- Nothing new. CI has no security key, so with the setting on CI can never verify, even holding an admin's token.

### 5.7 Cost

- **Backend.** A new dependency (for example `github.com/go-webauthn/webauthn`); `scripts/crypto-audit.sh` will need to know about it. Two migrations.
- **Routes.**
  - `POST /users/me/webauthn/credentials/begin|finish`, requiring a live sign-in (`refuseUnlessLiveSignIn`).
  - `GET /users/me/webauthn/credentials` and `DELETE /users/me/webauthn/credentials/:id`.
  - `POST /agents/:id/verify/challenge`, and `POST /agents/:id/verify` accepting an assertion.
  - The organisation setting.
- **Shared code.** The same verifier can end the refusal of `OwnerKeyPasskey` in `privileged_authority.go:50-54`.
- **Web.** A security-keys page in settings (a browser WebAuthn library such as `@simplewebauthn/browser`). The verify button runs the ceremony when the organisation requires it.
- **Python SDK.** `aim-sdk verify` and `aim-sdk verify --list-pending`, with an optional `fido2` extra. Today the CLI has `login`, `init`, `logout`, `status`, `version`, `help` and `demo` (`cli.py:1648-1752`, from the search report).
- **TypeScript SDK.** Nothing at first.

### 5.8 Undoing it

- An agent verified this way is undone like any other: suspend or revoke.
- A lost or stolen security key is revoked from the dashboard. Its past verifications stay valid but can be listed by credential id from the transition records, and suspended if needed.
- Turning the organisation setting off restores today's behaviour.

## 6. Option 3: hardware-bound agent keys as a trust factor

### 6.1 Steps

Developer:

1. Registers as today. Where the platform offers it, the SDK generates the agent key inside the hardware (a TPM, Secure Enclave, Android StrongBox or TEE, or a cloud vTPM).
2. The SDK sends the public key with the platform's attestation evidence.
3. Nothing else changes for the developer.

Administrator:

- Chooses whether attestation is only shown (a badge and a trust factor), or also feeds policy (section 7).
- Maintains the list of trusted roots, or accepts the shipped list.

### 6.2 What each platform can attest, and what it cannot prove

**Not checked in this repository.** No hardware support exists here (section 2.7). The statements in this section are from the platforms' public documentation as I understand it, and need a spike to confirm before any build.

**TPM 2.0** (Windows, Linux with `/dev/tpmrm0`, cloud vTPMs such as Google Shielded VM, Azure Trusted Launch and AWS NitroTPM).
- Can attest: a key was created inside a TPM and is not exportable (`TPM2_Certify` by an attestation key). That attestation key belongs to a TPM whose endorsement key certificate chains to the manufacturer or cloud provider. The attestation key is bound to the endorsement key through credential activation, which needs a round trip with the server.
- Keys are RSA or ECDSA P-256; Ed25519 is generally not available.
- On Linux the process needs access to the TPM device.

**Apple Secure Enclave** (macOS and iOS).
- Keys are P-256 only and not exportable.
- App Attest gives an Apple-signed attestation for an app on iOS-family devices. I am not aware of a general Apple attestation that a command-line process on macOS can obtain for a Secure Enclave key.
- On macOS, then, the SDK can probably make a hardware key but not prove it to the server.

**Android Key Attestation.**
- A certificate chain to Google's root that states the key's security level (TEE or StrongBox), the app's package and signature, and the boot state.
- P-256 is supported widely.

**AWS Nitro Enclaves.**
- An attestation document signed by the Nitro PKI with measurement registers (PCRs).
- This attests the enclave image, which is closer to "what code holds the key" than a TPM key certificate is.

What no platform attestation proves:

- **Which program uses the key.** Any process allowed to use the key can make the device sign. Malware on the host has a signing oracle while it runs.
- **That the host is uncompromised**, unless measured boot (PCR quotes) is also checked against known-good values. That is a much larger project.
- **Who owns the device, or that the agent is the one the developer meant to register.**
- **Freshness of the device's state after registration**, unless attestation is repeated.

Attestation also reveals a stable device identity (the endorsement key certificate), which is a privacy cost, and vendor roots and revocation lists must be kept current.

### 6.3 Stored, and signed by which key

Stored:
- New columns or a table `agent_key_attestations`: `agent_id`, `public_key_sha256`, `format` (`tpm2`, `android-key`, `apple-app-attest`, `nitro`), the raw evidence, the verified chain's root, the result, `verified_at`, `expires_at`.
- Bound to the key, not to the agent: replacing the key through `PUT /agents/:id/keys` must drop the factor.

Signed:
- **Agent requests** are signed by the hardware key: ECDSA P-256 for most platforms.
- **The attestation** is signed by the platform's attestation key, chaining to a vendor root.
- **The server's acceptance** is recorded in a transition record signed by the record key.

**Cost hidden here.** Agent keys are Ed25519 everywhere today. Registration hard-sets `"Ed25519"` (`agent_service.go:396`). The agent-signature middleware accepts Ed25519, ML-DSA and the hybrid (`apps/backend/internal/interfaces/http/middleware/pqc_agent_auth.go:22-24`). Supporting hardware keys means adding P-256 to every place that verifies an agent signature. It also conflicts with an organisation's `RequirePQC` setting (`apps/backend/internal/domain/organization.go:32`), because no consumer hardware holds ML-DSA keys yet.

### 6.4 How it enters the trust score

- **Today the score is a weighted mean of nine factors** (`trust_calculator.go:160-170`):
  - verification 0.25; uptime, success rate and security alerts 0.15 each;
  - compliance and execution isolation 0.10 each;
  - age 0.05, drift 0.03, user feedback 0.02.
- **Factors without data are dropped, and the result is capped at the neutral-imputed sum** (`trust_calculator.go:198-231`).
- **The verification factor depends on status.** It falls back to 1.0 for verified and 0.3 for pending (`trust_calculator.go:369-379`), so verification itself raises the score.
- **Proposal.** Add a tenth factor, `key_custody`, at a small weight (0.05, taken from the isolation factor's 0.10 or by rescaling all weights):
  - 1.0 for a verified attestation from a listed root;
  - 0.5 for a hardware key with no attestation (the macOS case);
  - 0.3 for a software key.
- Alternatively, a verified attestation could be allowed to set `isolation_attestations.verified`, which migration 108 reserves for "TEE attestation" (`migrations/108_add_isolation_attestation_verification.sql:26-28`). That only fits if the evidence is about the running environment (Nitro, measured boot). A TPM key certificate is not about the environment, so the separate factor is the honest choice.

### 6.5 May a policy verify automatically above a score that includes it?

**Recommended: no, not on the score alone.** Three reasons:
- **At registration the score has almost no evidence.** Uptime, success rate, alerts, drift and feedback have no history yet. The verification factor is the pending fallback. The score of a new agent is mostly placeholders.
- **The status feeds back into the score.** The verification factor reads the agent's own status, so a rule "verify above score S" partly reads its own output.
- **Weights move.** Factor weights have changed before (`trust_calculator.go:158-159`), and a threshold tuned to one set of weights silently changes meaning with the next.

**What to do instead.** Let policy (section 7) name attestation as a separate condition: "verified hardware key attestation from an allowed root" is a yes/no predicate next to the capability class, not a contribution to a number. The score keeps its current role: a minimum per capability at use time (`agent_service.go:1905-1915`), and automatic suspension when it falls (`apps/backend/internal/application/security_policy_service.go:427-467`).

### 6.6 What each attacker gains

**(a) Stolen SDK refresh token.**
- No change for agent verification: the token is the user's, not the agent's.
- If a policy requires attestation, the thief can still register agents, but only from a machine with a genuine attested chip. That keeps out a token replayed from an attacker's laptop without a TPM. It does not keep out a cloud VM with a vTPM, which anyone can rent.

**(b) Unlocked laptop.**
- The attacker can make the laptop's TPM sign for its agents while they are there, but cannot copy the agent's private key away.
- Today the agent key is plaintext JSON in `~/.aim/agents/<name>.json`, mode 0600 (`sdk/python/aim_sdk/credentials.py:774-799`, from the search report), and can simply be copied. This is the real gain of the option: agent key theft, not verification.

**(c) Compromised CI secret.**
- CI runners on cloud VMs may have a vTPM, so an attested key proves "a genuine cloud vTPM", which the attacker's own runner also has. It does not separate a legitimate pipeline from a hijacked one.

### 6.7 Cost

This is the most expensive option by far.

- **SDKs.** Per-platform key providers: Python through `tpm2-pytss` or a helper, TypeScript through native addons, and a signed helper on macOS. Each also needs the P-256 signing path.
- **Backend.**
  - An attestation verifier per format, and maintained vendor root lists.
  - P-256 agent-signature support in `pqc_agent_auth.go` and every verifier of agent signatures (A2A, MCP attestation, OAuth jwt-bearer).
  - A migration.
  - A new trust factor in `trust_calculator.go`.
- **Web.** A badge and a filter.

### 6.8 Undoing it

- The factor can be turned off by setting its weight to zero. Scores recalculate.
- A root found to be bad (a vendor flaw) is removed from the allowed list, and every attestation that chains to it is re-checked and drops to unattested.
- Agents that a policy verified because of it can be found from their transition records and suspended.

## 7. Option 4: a policy that verifies members' agents up to a capability class

### 7.1 Steps

Administrator, once:

1. In the security-policies page, where the enforcement mode is set today (`apps/web/app/dashboard/admin/security-policies/page.tsx:324`), sets: "Agents registered by `member` (or higher) are verified at registration when every capability they declare is at most `medium` risk." Optionally they add "and the agent's key has a verified hardware attestation" (option 3).

Developer:

- Nothing. They register as today.
- An agent within the class comes back `verified`. One above it comes back `pending`, with a reason naming the capability that was above the class.

Administrator, afterwards:

- Verifies only the agents above the class, by the dashboard or by option 2.

### 7.2 How capability classes are defined from what exists

**Use `capability_definitions.risk_level`, and not the PAM registry.**
- The risk levels already exist for the same `namespace:action` strings agents declare (`capability.go:83`, `:134`).
- They already drive the per-capability minimum trust score and execution mode (`capability.go:94-121`).
- They can be overridden per organisation, because definitions carry an `organization_id` and are looked up with it (`capability.go:88`; `agent_service.go:2054`).
- The PAM registry names capabilities differently, has no organisation column, and treats unknown capabilities as STANDARD (section 2.7), so it would need reconciling first.

**Rules for the class of a registration:**

1. Its class is the highest `risk_level` among its declared capabilities. A registration that declares no capability is `low`.
2. A declared capability that does not parse as `namespace:action`, or has no definition, is `critical`. This is the reverse of `capabilityMinTrustScore`, which treats it as requiring nothing (`agent_service.go:2045-2062`). Registration does not validate capability names today: the validation pattern is only reported to clients (`apps/backend/internal/application/capability_service.go:669`).
3. A wildcard (`file:*`) is the highest level of any definition in its namespace. A bare `*` is `critical`. The matcher treats a trailing `*` as a prefix match (`agent_service.go:2066-2086`, from the search report).
4. The policy names the largest class it verifies. The proposed default is "off". The largest allowed value is `high`: `critical` always waits for an administrator.

**Seeded examples.** The seeds in migration 043 would make:
- `file:read`, `db:read`, `api:call` and `network:connect` `low`;
- `file:write`, `db:write` and `data:export` `medium`;
- `file:delete` and `network:listen` `high`;
- `system:exec`, `db:admin`, `file:execute` and `user:impersonate` `critical`.

### 7.3 Stored, and signed by which key

- **New organisation settings.** `auto_verify_max_risk_level` (null when off), `auto_verify_min_role` (`member`, `manager` or `admin`), and `auto_verify_requires_attestation` (boolean, option 3). They are typed columns rather than entries in the free-form `Settings` map (`organization.go:29`). They are changed through the admin enforcement-settings route (`main.go:2125-2126`, `AdminMiddleware`), which already writes an audit row (search report: `admin_handler.go:1804-1816`).
- **The decision is made inside the registration transaction.** `registerAgent` already decides the status before the row is stored (`agent_service.go:659-675`), so the policy is one more input there. The record names the policy's settings and the computed class.
- **Signing.** Nothing new is signed by a user-held key. The administrator's choice of policy is the authorisation, recorded in the audit log. Each verification is recorded in the transition chain under the record key, with the actor being the registering user and the reason being the policy.
- **Re-registration.** A re-registration that adds capabilities above the class still files capability requests for an administrator, as strict mode does today (`agent_service.go:991-1021`, from the search report). The agent keeps its status, but the new capability waits.
- **Changes to risk levels.** A definition whose `risk_level` is lowered by an organisation admin changes what the policy admits. That change is already an admin act. It should also be audited, and the note proposes recording the risk level used in each verification record.

### 7.4 What each attacker gains

**(a) Stolen SDK refresh token.**
- Today: a member's token registers pending agents.
- With the policy on: it registers verified agents up to the class, for as long as the token lives. That is 8h for a device sign-in (`jwt.go:412`), and up to the SDK-download family's maximum for a downloaded token (`jwt.go:417`). This is wider than option 1, because there is no window tied to a moment of intent.
- For admin tokens, nothing changes (section 2.5).

**(b) Unlocked laptop.**
- The same as (a).

**(c) Compromised CI secret.**
- Organisation API keys can register agents (`main.go:1993`). The policy should not apply to API-key registrations, because a key has no role and no person behind it. With that rule, a CI API key gains nothing.
- If the owner wants CI agents verified, a separate per-key setting is safer than letting the role rule cover keys (section 10).

### 7.5 Cost

The smallest option.
- **Backend.** One migration on `organizations`. The admin service and handler extended (`admin_service.go:277`; `admin_handler.go:1749`). One decision function in `agent_service.go`, shared with option 1. A risk-level lookup that fails closed. The registration response gains a reason when the agent stays pending.
- **Web.** Controls on the security-policies page.
- **SDKs.** Print the reason when an agent comes back pending. No new commands.

### 7.6 Undoing it

- Turning the policy off stops further automatic verifications.
- Agents verified by it stay verified. They can be listed from their transition records (reason: policy) and suspended in bulk.
- A risk level that was set too low can be raised, but that does not unverify agents verified under it.

## 8. Comparison

"Trips" counts how often someone leaves the terminal for a web page, for the first agent and for the tenth agent of a developer in strict mode. It assumes the developer already ran `aim-sdk login` once; that sign-in is counted as one trip where it is the trip the option uses.

| | Today (dashboard click) | 1. Grant at sign-in | 2. Hardware key from terminal | 3. Attested agent keys | 4. Policy by class |
|---|---|---|---|---|---|
| Trips, first agent | 1 sign-in + 1 admin page visit | 1 (the sign-in itself) | 1 sign-in + 1 one-time key registration; then 0 | Same as today unless combined with 4 | 1 sign-in; 0 for verification if within class |
| Trips, tenth agent | 1 admin page visit (or a batch) | 0 inside the window; 1 new sign-in after it | 0 (one touch each) | Same as today unless combined with 4 | 0 within class; 1 admin action above it |
| Who acts per agent | Admin or manager | Nobody (the grant) | Admin or manager, with a touch | Nobody | Nobody within class |
| (a) stolen SDK refresh token | Admin token verifies anything; member token registers pending only | Member token in window: verified agents up to class and count | Admin token can no longer verify without the key (setting on): **better than today** | No change to verification; agent keys of other machines unaffected | Member token: verified agents up to class for the token's life |
| (b) unlocked laptop | Same as (a), plus the browser session | Same as (a); can also approve a new grant from the open session | Can verify only by touching a present key; blocked if PIN or biometric required | Agent keys cannot be copied off the machine: **better than today** | Same as (a) |
| (c) compromised CI secret | API key: pending only; copied admin token: verifies anything | No change for API keys or SDK-download tokens | Cannot verify (no key): **better than today** | Proves "a genuine vTPM", which an attacker's runner also has | No change if the policy excludes API keys |
| What signs the approval | Nothing user-held; HS256 token; record key | Nothing user-held; record key | The admin's WebAuthn credential, plus the record key | The platform's attestation key (for the key, not the approval) | Nothing user-held; record key |
| Build cost | None | Medium | Medium to large | Very large | Small |
| Undo | Suspend or revoke | Revoke grant; bulk suspend by grant id | Revoke credential; suspend | Remove root; re-check; suspend | Turn off; bulk suspend by reason |

## 9. Recommendation

Build option 4 first, with classes from `capability_definitions.risk_level`, unknown capabilities counted as critical, and API-key registrations excluded. It removes the trip for most agents at the smallest cost, and its decision function is the one option 1 needs. Build option 2 next, and offer its organisation setting to strict organisations: it is the only option that is stronger than today against a stolen token, because today an admin's stored refresh token can already verify without any page (section 2.5). Leave option 1 until option 4 shows whether a policy is enough, and treat option 3 as a separate project about agent key theft that may later feed policy as a yes/no condition, never as auto-verification above a trust score.

## 10. Open questions for the owner

1. Should a policy (option 4) or a grant (option 1) be allowed at all in strict mode? Or should strict mean "a person, or a person's hardware key, approves every agent", with option 4 offered only as a new mode between monitoring and strict?
2. Which roles may give a grant, or be covered by the policy: members, or only managers and admins? Today device approval checks no role at all (section 2.7). Should that be fixed separately, so viewers cannot approve a device sign-in?
3. What is the largest class a policy may cover: `medium` or `high`? Is `critical` always excluded, as proposed?
4. Should option 2's setting also close the gap of section 2.5 for the dashboard button, by requiring a security-key ceremony there too? Or should the verify route at least require a live sign-in, as credential routes do? This could be done before any option, as a small separate change.
5. Should replacing a verified agent's public key return it to `pending` (section 2.6)? Option 2 binds approvals to the key. Without this, a member can swap the key of an agent an administrator verified.
6. Should `reactivate` be subject to the same requirements as `verify`, since both write `verified`?
7. Are synced passkeys acceptable for option 2, or only device-bound security keys (an AAGUID allowlist)?
8. Should one touch be able to approve a batch of agents, with the challenge naming all of them? Or one touch per agent?
9. Should CI agents ever be verified automatically? If so, through a per-API-key setting rather than through the role rule?
10. Which platforms matter for option 3 (Linux servers with TPM, cloud vTPMs, macOS laptops, Nitro)? Is the conflict with `RequirePQC` acceptable for organisations that set it?
11. Should the TypeScript SDK gain a device login, so that options 1 and 2 are not Python-only?
12. Should the PAM capability registry be reconciled with `capability_definitions`, so the organisation has one notion of capability class rather than two?

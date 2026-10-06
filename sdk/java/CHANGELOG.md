# Changelog

All notable changes to the AIM Java SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `registerPQCKey(..., true)` (both overloads) and `setHybridMode(true)` now throw `ConfigurationException`
  before sending anything. They sent `"enableHybrid": true` and `"enable": true`, which mark the agent as
  requiring both an Ed25519 and an ML-DSA signature, and no AIM SDK signs requests in that form yet. Register
  the key with `registerPQCKey(pqcPublicKey, algorithm, false)`; `setHybridMode(false)` still turns hybrid
  mode off.
- A JSON string is now refused where the SDK reads a trust or confidence number, whatever its length and
  including a numeric one such as `"0.75"`, which was converted to a number before. This covers
  `A2ATrustScore` (`score`, `a2aTrustScore`, `peerTrustAverage`, `confidence`, `factors`), `A2APeerTrust`
  (`peerTrustScore`, `successRate`), `A2ASecurityCheckResult.requesterTrustScore`,
  `A2ASecuritySettings.minTrustScore` and `A2AConsensusResult.confidenceScore`, and every other `Double`,
  `double`, `Float`, `float`, `BigDecimal` or `Number` member of a class the SDK maps from JSON, its local
  cache files included. The MCP server trust and confidence scores that `MCPIntegration` reads from the
  JSON tree, rather than mapping them, are unchanged. The method throws its usual exception (`A2AException` from `A2AClient`) with a `MismatchedInputException` as
  its cause, whose message names the field and the string's length and never the string:
  `Field "/score": a JSON string of 4 characters was refused where a number is bound; a JSON number is expected`.
  A JSON number reads as before. AIM servers send these fields as JSON numbers; a proxy, mock or other server
  that sends them as strings needs to send numbers.

### Deprecated

- `AIMClient.createHybridRequestHeaders` and `PQCOperations.createHybridRequestHeaders`. AIM's request
  verification does not accept these headers: it verifies a signature over the method, path, timestamp and
  body joined by newlines, and these headers sign `timestamp:METHOD:path:sha256(body)`. Their javadoc said
  they were compatible with the AIM backend; it now says they are not accepted.

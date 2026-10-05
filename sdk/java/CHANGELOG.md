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

### Deprecated

- `AIMClient.createHybridRequestHeaders` and `PQCOperations.createHybridRequestHeaders`. AIM's request
  verification does not accept these headers: it verifies a signature over the method, path, timestamp and
  body joined by newlines, and these headers sign `timestamp:METHOD:path:sha256(body)`. Their javadoc said
  they were compatible with the AIM backend; it now says they are not accepted.

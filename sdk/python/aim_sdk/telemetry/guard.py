"""
NanoMind-Guard socket client -- stage 2 of the runtime-protection port (#441).

JSON-lines over AF_UNIX to the NanoMind-Guard daemon, mirroring the TypeScript
client (``sdk/typescript/src/arp/guard/nanomind-guard-client.ts``). The daemon
owns the wire protocol: one request, one response line, one connection::

    send: {"op": "classify", "text": "..."}\\n
    recv: {"ok": true, ...}\\n

Contract
--------
* **None on socket-absent.** A missing, unreachable, symlinked or silent socket
  returns None. None means "analyst unavailable", never a benign verdict.
* **Signed fields or None.** The only fields returned are the ones covered by
  the daemon's hybrid Ed25519+ML-DSA-44 signature (``SIGNED_FIELDS``). A reply
  with no signature, a different algorithm, a signature that does not verify, a
  content hash for different text, or a stale timestamp returns None. Unsigned
  diagnostics the daemon adds to a reply (gate labels, NLM analysis) are
  discarded. Without a configured guard public key nothing can be verified, so
  every call returns None. When ``liboqs`` is not installed the ML-DSA half
  cannot be checked and every call returns None.
* **No direct HuggingFace fallback.** The daemon is what puts the
  input-classifier gate in front of the model; loading the model any other way
  bypasses the gate. This module opens AF_UNIX sockets only.

The signed payload is the reply's ``SIGNED_FIELDS`` (the TypeScript
``NanoMindGuardResult`` without its ``signature`` block), serialized as JSON
with sorted keys and no whitespace -- byte-identical to
``canonicalizeGuardResultPayload`` in the TypeScript verifier.

Module-level imports are stdlib only; the Ed25519 (PyNaCl) and ML-DSA
(``aim_sdk.crypto.pqc``) verifiers are imported when a signature is checked.

This is a telemetry producer: nothing on the verify/deny path imports it.
"""
from __future__ import annotations

import base64
import binascii
import hashlib
import json
import math
import os
import re
import socket
import stat
import time
from dataclasses import dataclass
from typing import Any, Callable, Dict, Mapping, Optional

#: Socket path used when neither the caller nor ``NANOMIND_GUARD_SOCK`` sets one.
FALLBACK_SOCK_PATH = "/tmp/nanomind-guard.sock"

#: The only signature algorithm a guard result may carry.
GUARD_SIGNATURE_ALGORITHM = "Ed25519+ML-DSA-44"

#: The fields the daemon signs; these, and only these, are returned.
SIGNED_FIELDS = ("classification", "confidence", "modelVersion", "contentHash", "timestamp")

#: Per-request timeout. The daemon's NLM p50 is ~6 s; 30 s covers the tail.
DEFAULT_REQUEST_TIMEOUT_S = 30.0

#: Default freshness window for a signed result, and tolerated clock skew.
DEFAULT_MAX_AGE_MS = 5 * 60 * 1000
DEFAULT_FUTURE_SKEW_MS = 30 * 1000

#: The daemon refuses requests over 1 MiB; fail fast client-side.
MAX_REQUEST_BYTES = 1024 * 1024
#: Hard cap on a single response line.
MAX_RESPONSE_BYTES = 256 * 1024

_SHORT_FIELD_CAP = 256
_CONTENT_HASH_RE = re.compile(r"^[0-9a-fA-F]{64}$")

# FIPS 204 / RFC 8032 sizes for the Ed25519+ML-DSA-44 lane.
_ED25519_PUBLIC_KEY_BYTES = 32
_ED25519_SIGNATURE_BYTES = 64
_MLDSA44_PUBLIC_KEY_BYTES = 1312
_MLDSA44_SIGNATURE_BYTES = 2420


def resolve_socket_path(override: Optional[str] = None) -> str:
    """Explicit override > ``NANOMIND_GUARD_SOCK`` > ``FALLBACK_SOCK_PATH``."""
    if override:
        return override
    return os.environ.get("NANOMIND_GUARD_SOCK") or FALLBACK_SOCK_PATH


@dataclass(frozen=True)
class GuardPublicKey:
    """The daemon's hybrid Ed25519+ML-DSA-44 public key."""

    ed25519_public_key: bytes
    mldsa_public_key: bytes

    @classmethod
    def from_encoded(cls, encoded: Mapping[str, Any]) -> "GuardPublicKey":
        """Build from the TS ``EncodedHybridPublicKey`` shape (base64 fields).

        Raises ValueError on a wrong algorithm, bad base64 or wrong key size.
        """
        if encoded.get("algorithm") != GUARD_SIGNATURE_ALGORITHM:
            raise ValueError(f"guard public key algorithm must be {GUARD_SIGNATURE_ALGORITHM}")
        if encoded.get("mldsaVariant", "ML-DSA-44") != "ML-DSA-44":
            raise ValueError("guard public key mldsaVariant must be ML-DSA-44")
        ed = _b64(encoded.get("ed25519PublicKey"))
        ml = _b64(encoded.get("mldsaPublicKey"))
        if ed is None or len(ed) != _ED25519_PUBLIC_KEY_BYTES:
            raise ValueError("ed25519PublicKey must be 32 bytes of base64")
        if ml is None or len(ml) != _MLDSA44_PUBLIC_KEY_BYTES:
            raise ValueError("mldsaPublicKey must be 1312 bytes of base64")
        return cls(ed25519_public_key=ed, mldsa_public_key=ml)


@dataclass(frozen=True)
class GuardVerdict:
    """A classification whose every field was covered by a verified signature."""

    classification: str
    confidence: float
    model_version: str
    content_hash: str
    timestamp: int


#: ``(message, signature_block, public_key) -> bool``
Verifier = Callable[[bytes, Mapping[str, Any], GuardPublicKey], bool]


# --------------------------------------------------------------------------- #
# Canonical signed payload
# --------------------------------------------------------------------------- #
def _js_number(value: Any) -> str:
    """Render a number the way JavaScript's ``JSON.stringify`` does."""
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise TypeError("not a number")
    if isinstance(value, int):
        return str(value)
    if not math.isfinite(value):
        return "null"
    if value.is_integer() and abs(value) < 1e21:
        return str(int(value))
    text = repr(value)
    if "e" in text:
        mantissa, exponent = text.split("e")
        sign = "-" if exponent.startswith("-") else "+"
        text = f"{mantissa}e{sign}{int(exponent.lstrip('+-'))}"
    return text


def _stable_stringify(value: Any) -> str:
    if value is None or isinstance(value, (bool, str)):
        return json.dumps(value, ensure_ascii=False)
    if isinstance(value, (int, float)):
        return _js_number(value)
    if isinstance(value, (list, tuple)):
        return "[" + ",".join(_stable_stringify(v) for v in value) + "]"
    if isinstance(value, Mapping):
        return "{" + ",".join(
            f"{json.dumps(str(k), ensure_ascii=False)}:{_stable_stringify(value[k])}"
            for k in sorted(value)
        ) + "}"
    raise TypeError(f"cannot canonicalize {type(value).__name__}")


def canonicalize_guard_result_payload(payload: Mapping[str, Any]) -> bytes:
    """Signed-payload bytes: the result minus ``signature``, sorted keys, no spaces."""
    stripped = {k: v for k, v in payload.items() if k != "signature"}
    return _stable_stringify(stripped).encode("utf-8")


# --------------------------------------------------------------------------- #
# Signature verification
# --------------------------------------------------------------------------- #
def _b64(value: Any) -> Optional[bytes]:
    if not isinstance(value, str) or not value:
        return None
    try:
        return base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        return None


def _verify_ed25519(public_key: bytes, message: bytes, signature: bytes) -> bool:
    try:
        from nacl.exceptions import BadSignatureError
        from nacl.signing import VerifyKey
    except ImportError:
        return False
    try:
        VerifyKey(public_key).verify(message, signature)
        return True
    except BadSignatureError:
        return False
    except Exception:
        return False


def _verify_mldsa44(public_key: bytes, message: bytes, signature: bytes) -> bool:
    try:
        from aim_sdk.crypto import pqc
    except ImportError:
        return False
    try:
        return pqc.verify_mldsa(public_key, message, signature, pqc.Algorithm.MLDSA_44) is True
    except Exception:
        # PQCNotAvailableError (no liboqs) included: unverifiable is unsigned.
        return False


def verify_guard_signature(
    message: bytes, signature: Mapping[str, Any], public_key: GuardPublicKey
) -> bool:
    """Hybrid check: both the Ed25519 and the ML-DSA-44 halves must verify.

    Returns False -- never raises -- on a wrong algorithm, malformed base64,
    wrong sizes, a missing verifier library, or either half failing.
    """
    if signature.get("alg") != GUARD_SIGNATURE_ALGORITHM:
        return False
    ed_sig = _b64(signature.get("ed25519Sig"))
    ml_sig = _b64(signature.get("mldsaSig"))
    if ed_sig is None or len(ed_sig) != _ED25519_SIGNATURE_BYTES:
        return False
    if ml_sig is None or len(ml_sig) != _MLDSA44_SIGNATURE_BYTES:
        return False
    if len(public_key.ed25519_public_key) != _ED25519_PUBLIC_KEY_BYTES:
        return False
    if len(public_key.mldsa_public_key) != _MLDSA44_PUBLIC_KEY_BYTES:
        return False
    # Evaluate both halves before combining.
    ed_ok = _verify_ed25519(public_key.ed25519_public_key, message, ed_sig)
    ml_ok = _verify_mldsa44(public_key.mldsa_public_key, message, ml_sig)
    return ed_ok and ml_ok


# --------------------------------------------------------------------------- #
# Client
# --------------------------------------------------------------------------- #
class NanoMindGuardClient:
    """Classify text through the NanoMind-Guard daemon's AF_UNIX socket.

    ``classify`` returns a :class:`GuardVerdict` built only from signed fields,
    or None. It never raises for an unavailable daemon or a bad reply.
    """

    def __init__(
        self,
        public_key: Optional[GuardPublicKey] = None,
        *,
        socket_path: Optional[str] = None,
        timeout: float = DEFAULT_REQUEST_TIMEOUT_S,
        max_age_ms: int = DEFAULT_MAX_AGE_MS,
        future_skew_ms: int = DEFAULT_FUTURE_SKEW_MS,
        now: Optional[Callable[[], float]] = None,
        verifier: Optional[Verifier] = None,
    ) -> None:
        self.public_key = public_key
        self.socket_path = socket_path
        self.timeout = timeout
        self.max_age_ms = max_age_ms
        self.future_skew_ms = future_skew_ms
        self._now = now or (lambda: time.time() * 1000.0)
        self._verifier = verifier or verify_guard_signature

    def classify(self, text: str) -> Optional[GuardVerdict]:
        if not isinstance(text, str) or not text.strip():
            return None
        if self.public_key is None:
            # Nothing can be verified, so nothing may be returned.
            return None
        reply = self._request({"op": "classify", "text": text})
        if reply is None:
            return None
        return self._signed_verdict(reply, text)

    # -- transport --------------------------------------------------------- #
    def _request(self, payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        af_unix = getattr(socket, "AF_UNIX", None)
        if af_unix is None:
            return None
        path = resolve_socket_path(self.socket_path)
        try:
            st = os.lstat(path)
        except OSError:
            return None  # socket absent: unavailable, not benign
        if stat.S_ISLNK(st.st_mode) or not stat.S_ISSOCK(st.st_mode):
            return None  # refuse a redirected or non-socket path

        line = (json.dumps(payload, ensure_ascii=False) + "\n").encode("utf-8")
        if len(line) > MAX_REQUEST_BYTES:
            return None

        deadline = time.monotonic() + self.timeout
        try:
            sock = socket.socket(af_unix, socket.SOCK_STREAM)
        except OSError:
            return None
        try:
            sock.settimeout(self.timeout)
            sock.connect(path)
            sock.sendall(line)
            buf = bytearray()
            while b"\n" not in buf:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    return None
                sock.settimeout(remaining)
                chunk = sock.recv(65536)
                if not chunk:
                    return None  # peer closed without a complete line
                buf.extend(chunk)
                if len(buf) > MAX_RESPONSE_BYTES:
                    return None
        except (OSError, ValueError):
            return None
        finally:
            try:
                sock.close()
            except OSError:
                pass

        raw = bytes(buf[: buf.index(b"\n")])
        if len(raw) > MAX_RESPONSE_BYTES:
            return None
        try:
            parsed = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, ValueError):
            return None
        return parsed if isinstance(parsed, dict) else None

    # -- verification ------------------------------------------------------ #
    def _signed_verdict(self, reply: Mapping[str, Any], text: str) -> Optional[GuardVerdict]:
        if reply.get("ok") is not True:
            return None
        signature = reply.get("signature")
        if not isinstance(signature, Mapping):
            return None  # unsigned reply
        if signature.get("alg") != GUARD_SIGNATURE_ALGORITHM:
            return None
        if any(field not in reply for field in SIGNED_FIELDS):
            return None

        classification = reply["classification"]
        confidence = reply["confidence"]
        model_version = reply["modelVersion"]
        content_hash = reply["contentHash"]
        timestamp = reply["timestamp"]
        if not isinstance(classification, str) or not 0 < len(classification) <= _SHORT_FIELD_CAP:
            return None
        if isinstance(confidence, bool) or not isinstance(confidence, (int, float)):
            return None
        if not math.isfinite(confidence) or not 0 <= confidence <= 1:
            return None
        if not isinstance(model_version, str) or not 0 < len(model_version) <= _SHORT_FIELD_CAP:
            return None
        if not isinstance(content_hash, str) or not _CONTENT_HASH_RE.match(content_hash):
            return None
        if isinstance(timestamp, bool) or not isinstance(timestamp, (int, float)):
            return None
        if not math.isfinite(timestamp):
            return None

        signed = {field: reply[field] for field in SIGNED_FIELDS}
        try:
            message = canonicalize_guard_result_payload(signed)
            if not self._verifier(message, signature, self.public_key):
                return None
        except Exception:
            return None

        # The signature binds the result to the text that was sent.
        expected = hashlib.sha256(text.encode("utf-8")).hexdigest()
        if content_hash.lower() != expected:
            return None

        age = self._now() - timestamp
        if age > self.max_age_ms or -age > self.future_skew_ms:
            return None

        return GuardVerdict(
            classification=classification,
            confidence=float(confidence),
            model_version=model_version,
            content_hash=content_hash,
            timestamp=int(timestamp),
        )

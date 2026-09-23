"""
Offline dependency shims for the test suite.

Some CI/verification containers run this suite with no package index, so the
SDK's third-party dependencies (`requests`, `PyNaCl`, `cryptography`, `PyJWT`)
may be absent. When — and only when — a dependency is missing, this conftest
installs a minimal functional stand-in into ``sys.modules`` before test
collection imports ``aim_sdk``. When the real package is installed, nothing
here runs and the suite behaves exactly as before.

The `requests` stand-in is a real HTTP client over ``http.client`` with the
same exception taxonomy, so tests against loopback fakes and closed loopback
ports exercise genuine socket behaviour (a closed port raises
``requests.exceptions.ConnectionError``, a silent server ``Timeout``).

The `nacl`/`cryptography`/`jwt` stand-ins implement NO real cryptography —
deterministic digests stand in for signatures, which is sufficient because the
offline suite only ever talks to its own fakes, which do not verify
signatures. They exist to satisfy imports and produce byte-shaped values.
"""

import base64
import hashlib
import importlib.machinery
import importlib.util
import json as _json
import socket
import sys
import types
import urllib.parse
from pathlib import Path


def _module(name):
    mod = types.ModuleType(name)
    # A real ModuleSpec so importlib.util.find_spec(name) keeps working after
    # the shim is installed instead of raising ValueError.
    mod.__spec__ = importlib.machinery.ModuleSpec(name, loader=None)
    sys.modules[name] = mod
    return mod


# ---------------------------------------------------------------------------
# requests
# ---------------------------------------------------------------------------

def _install_requests_shim():
    requests = _module("requests")
    exceptions = _module("requests.exceptions")
    requests.exceptions = exceptions

    class RequestException(IOError):
        def __init__(self, *args, response=None, request=None):
            super().__init__(*args)
            self.response = response
            self.request = request

    class ConnectionError(RequestException):  # noqa: A001 - mirrors requests' name
        pass

    class Timeout(RequestException):
        pass

    class ConnectTimeout(ConnectionError, Timeout):
        pass

    class ReadTimeout(Timeout):
        pass

    class HTTPError(RequestException):
        pass

    class JSONDecodeError(ValueError, RequestException):
        pass

    class _Headers(dict):
        """Case-insensitive header lookup, enough for .get()/[] access."""

        def __init__(self, items=()):
            super().__init__()
            for k, v in dict(items).items():
                self[k] = v

        def __setitem__(self, key, value):
            super().__setitem__(str(key).title(), value)

        def __getitem__(self, key):
            return super().__getitem__(str(key).title())

        def __contains__(self, key):
            return super().__contains__(str(key).title())

        def get(self, key, default=None):
            return super().get(str(key).title(), default)

    class Response:
        def __init__(self, status_code, headers, content, url):
            self.status_code = status_code
            self.headers = _Headers(headers)
            self.content = content
            self.url = url
            self.reason = ""

        @property
        def text(self):
            return self.content.decode("utf-8", errors="replace")

        def json(self, **kwargs):
            try:
                return _json.loads(self.text)
            except ValueError as exc:
                raise JSONDecodeError(str(exc)) from None

        @property
        def ok(self):
            return self.status_code < 400

        def raise_for_status(self):
            if self.status_code >= 400:
                raise HTTPError(
                    f"{self.status_code} Error for url: {self.url}", response=self
                )

        def close(self):
            pass

    def _issue(method, url, params=None, data=None, json=None, headers=None,
               timeout=None, **_ignored):
        import http.client

        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme not in ("http", "https"):
            raise RequestException(f"Unsupported or missing URL scheme: {url!r}")
        host = parsed.hostname or ""
        port = parsed.port or (443 if parsed.scheme == "https" else 80)

        path = parsed.path or "/"
        query = parsed.query
        if params:
            extra = urllib.parse.urlencode(params)
            query = f"{query}&{extra}" if query else extra
        if query:
            path = f"{path}?{query}"

        send_headers = _Headers(headers or {})
        body = None
        if json is not None:
            body = _json.dumps(json).encode("utf-8")
            send_headers.setdefault("Content-Type", "application/json")
        elif data is not None:
            body = data.encode("utf-8") if isinstance(data, str) else data

        if parsed.scheme == "https":
            conn = http.client.HTTPSConnection(host, port, timeout=timeout)
        else:
            conn = http.client.HTTPConnection(host, port, timeout=timeout)
        try:
            try:
                conn.request(method.upper(), path, body=body, headers=dict(send_headers))
                raw = conn.getresponse()
                content = raw.read()
                return Response(raw.status, dict(raw.getheaders()), content, url)
            except socket.timeout as exc:
                raise Timeout(f"timed out: {exc}") from None
            except (ConnectionRefusedError, ConnectionResetError, BrokenPipeError,
                    socket.gaierror, OSError) as exc:
                raise ConnectionError(
                    f"Connection error for url {url}: {exc}"
                ) from None
        finally:
            conn.close()

    class Session:
        def __init__(self):
            self.headers = _Headers()

        def request(self, method, url=None, **kwargs):
            merged = _Headers(self.headers)
            for k, v in _Headers(kwargs.pop("headers", None) or {}).items():
                merged[k] = v
            return _issue(method, url, headers=merged, **kwargs)

        def get(self, url, **kwargs):
            return self.request("GET", url, **kwargs)

        def post(self, url, **kwargs):
            return self.request("POST", url, **kwargs)

        def put(self, url, **kwargs):
            return self.request("PUT", url, **kwargs)

        def delete(self, url, **kwargs):
            return self.request("DELETE", url, **kwargs)

        def close(self):
            pass

        def __enter__(self):
            return self

        def __exit__(self, *exc):
            self.close()

    def request(method, url, **kwargs):
        return _issue(method, url, **kwargs)

    def get(url, **kwargs):
        return _issue("GET", url, **kwargs)

    def post(url, **kwargs):
        return _issue("POST", url, **kwargs)

    def put(url, **kwargs):
        return _issue("PUT", url, **kwargs)

    def delete(url, **kwargs):
        return _issue("DELETE", url, **kwargs)

    for cls in (RequestException, ConnectionError, Timeout, ConnectTimeout,
                ReadTimeout, HTTPError, JSONDecodeError):
        setattr(exceptions, cls.__name__, cls)
        setattr(requests, cls.__name__, cls)
    requests.Response = Response
    requests.Session = Session
    requests.request = request
    requests.get = get
    requests.post = post
    requests.put = put
    requests.delete = delete
    requests.__version__ = "0.0.0-offline-shim"


# ---------------------------------------------------------------------------
# nacl (PyNaCl)
# ---------------------------------------------------------------------------

def _install_nacl_shim():
    nacl = _module("nacl")
    signing = _module("nacl.signing")
    encoding = _module("nacl.encoding")
    bindings = _module("nacl.bindings")
    nacl_exceptions = _module("nacl.exceptions")
    nacl.signing = signing
    nacl.encoding = encoding
    nacl.bindings = bindings
    nacl.exceptions = nacl_exceptions

    class RawEncoder:
        @staticmethod
        def encode(data):
            return data

        @staticmethod
        def decode(data):
            return data

    class Base64Encoder:
        @staticmethod
        def encode(data):
            return base64.b64encode(data)

        @staticmethod
        def decode(data):
            return base64.b64decode(data)

    class BadSignatureError(Exception):
        pass

    class _SignedMessage(bytes):
        signature = b""
        message = b""

    class VerifyKey:
        def __init__(self, key, encoder=RawEncoder):
            self._key = encoder.decode(key)

        def encode(self, encoder=RawEncoder):
            return encoder.encode(self._key)

        def __bytes__(self):
            return self._key

        def verify(self, smessage, signature=None, encoder=RawEncoder):
            # NOT cryptographic verification; accepts anything. Offline fakes
            # never depend on rejecting a signature.
            return encoder.decode(smessage) if signature is None else smessage

    class SigningKey:
        def __init__(self, seed, encoder=RawEncoder):
            seed = encoder.decode(seed)
            if len(seed) != 32:
                raise ValueError("SigningKey seed must be 32 bytes")
            self._seed = seed
            self.verify_key = VerifyKey(hashlib.sha256(b"vk" + seed).digest())

        @classmethod
        def generate(cls):
            import secrets as _secrets
            return cls(_secrets.token_bytes(32))

        def sign(self, message, encoder=RawEncoder):
            sig = hashlib.sha512(self._seed + bytes(message)).digest()  # 64 bytes
            signed = _SignedMessage(encoder.encode(sig + bytes(message)))
            signed.signature = sig
            signed.message = bytes(message)
            return signed

        def encode(self, encoder=RawEncoder):
            return encoder.encode(self._seed)

        def __bytes__(self):
            return self._seed

    signing.SigningKey = SigningKey
    signing.VerifyKey = VerifyKey
    encoding.RawEncoder = RawEncoder
    encoding.Base64Encoder = Base64Encoder
    encoding.HexEncoder = RawEncoder
    nacl_exceptions.BadSignatureError = BadSignatureError
    bindings.crypto_sign_ed25519_pk_to_curve25519 = lambda pk: pk


# ---------------------------------------------------------------------------
# cryptography (only the names aim_sdk.secrets imports at module level)
# ---------------------------------------------------------------------------

def _install_cryptography_shim():
    crypto = _module("cryptography")
    hazmat = _module("cryptography.hazmat")
    primitives = _module("cryptography.hazmat.primitives")
    asymmetric = _module("cryptography.hazmat.primitives.asymmetric")
    x25519 = _module("cryptography.hazmat.primitives.asymmetric.x25519")
    ciphers = _module("cryptography.hazmat.primitives.ciphers")
    aead = _module("cryptography.hazmat.primitives.ciphers.aead")
    crypto.hazmat = hazmat
    hazmat.primitives = primitives
    primitives.asymmetric = asymmetric
    asymmetric.x25519 = x25519
    primitives.ciphers = ciphers
    ciphers.aead = aead

    class _Unusable:
        def __init__(self, *a, **k):
            raise NotImplementedError(
                "cryptography is not installed; this offline shim satisfies "
                "imports only"
            )

        @classmethod
        def generate(cls, *a, **k):
            raise NotImplementedError("cryptography is not installed")

        from_private_bytes = from_public_bytes = generate

    x25519.X25519PrivateKey = type("X25519PrivateKey", (_Unusable,), {})
    x25519.X25519PublicKey = type("X25519PublicKey", (_Unusable,), {})
    aead.ChaCha20Poly1305 = type("ChaCha20Poly1305", (_Unusable,), {})

    fernet = _module("cryptography.fernet")
    crypto.fernet = fernet
    fernet.Fernet = type("Fernet", (_Unusable,), {})


# ---------------------------------------------------------------------------
# jwt (PyJWT; aim_sdk.oauth uses jwt.decode + jwt.exceptions.DecodeError)
# ---------------------------------------------------------------------------

def _install_jwt_shim():
    jwt = _module("jwt")
    jwt_exceptions = _module("jwt.exceptions")
    jwt.exceptions = jwt_exceptions

    class DecodeError(Exception):
        pass

    class ExpiredSignatureError(DecodeError):
        pass

    def decode(token, key=None, algorithms=None, options=None, **kwargs):
        # Parse the payload without verification (options passed by the SDK
        # disable verification anyway; offline fakes sign nothing).
        try:
            payload_b64 = token.split(".")[1]
            payload_b64 += "=" * (-len(payload_b64) % 4)
            return _json.loads(base64.urlsafe_b64decode(payload_b64))
        except Exception as exc:
            raise DecodeError(str(exc)) from None

    jwt.decode = decode
    jwt.DecodeError = DecodeError
    jwt.ExpiredSignatureError = ExpiredSignatureError
    jwt_exceptions.DecodeError = DecodeError
    jwt_exceptions.ExpiredSignatureError = ExpiredSignatureError


_MISSING = {
    name
    for name in ("requests", "nacl", "cryptography", "jwt", "responses")
    if importlib.util.find_spec(name) is None
}

for _name, _installer in (
    ("requests", _install_requests_shim),
    ("nacl", _install_nacl_shim),
    ("cryptography", _install_cryptography_shim),
    ("jwt", _install_jwt_shim),
):
    if _name in _MISSING:
        _installer()


# ---------------------------------------------------------------------------
# Collection: skip test modules whose real dependency is absent
# ---------------------------------------------------------------------------
# The shims satisfy what aim_sdk itself imports. A few test modules import
# more than the shims provide (the `responses` mocking library, or deep
# cryptography submodules like ...asymmetric.ed25519). When the real package
# is installed these run exactly as before; when it is absent they cannot
# even be collected, so they are excluded rather than erroring the run.

collect_ignore = []
_UNSHIMMED_MARKERS = []
if "responses" in _MISSING:
    _UNSHIMMED_MARKERS.append("import responses")
if "cryptography" in _MISSING:
    _UNSHIMMED_MARKERS.append("cryptography.hazmat.primitives.asymmetric.ed25519")
    # Real key-exchange/AEAD tests: the shim satisfies imports only.
    _UNSHIMMED_MARKERS.append("cryptography.hazmat.primitives.asymmetric.x25519")

if _UNSHIMMED_MARKERS:
    for _test_file in Path(__file__).parent.glob("test_*.py"):
        _source = _test_file.read_text(encoding="utf-8", errors="replace")
        if any(marker in _source for marker in _UNSHIMMED_MARKERS):
            collect_ignore.append(_test_file.name)

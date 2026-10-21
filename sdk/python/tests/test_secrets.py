"""
Tests for AIM Secrets Client — Phase 2 (Python SDK).

Covers:
- SecretsClient initialization and wiring
- Resolve flow: Ed25519 signature, X25519 ECDH, ChaCha20-Poly1305 decryption
- Namespace CRUD via _make_request
- Credential store/rotate
- Audit log retrieval
- Error handling
"""

import base64
import enum
import hashlib
import json
import os
from unittest.mock import patch
from urllib.parse import parse_qsl, urlsplit

import pytest
from nacl.signing import SigningKey
from nacl.bindings import crypto_sign_ed25519_pk_to_curve25519
from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey, X25519PublicKey
from cryptography.hazmat.primitives.ciphers.aead import ChaCha20Poly1305

from aim_sdk.client import AIMClient
from aim_sdk.secrets import SecretsClient
from aim_sdk.exceptions import SecretsError


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_test_client():
    """Create an AIMClient with real Ed25519 keys for testing."""
    signing_key = SigningKey.generate()
    pub_b64 = base64.b64encode(bytes(signing_key.verify_key)).decode("utf-8")
    # Go-format 64-byte private key (seed + public)
    priv_b64 = base64.b64encode(
        bytes(signing_key._signing_key)
    ).decode("utf-8")

    client = AIMClient(
        agent_id="test-agent-id",
        public_key=pub_b64,
        private_key=priv_b64,
        aim_url="http://localhost:8080",
    )
    return client, signing_key


def encrypt_for_agent(plaintext: bytes, agent_ed25519_pub_bytes: bytes):
    """
    Mirror the server's encryptForAgent() — used to build mock responses.

    Returns (encrypted_blob_b64, ephemeral_pub_b64).
    """
    # Convert agent Ed25519 pub -> X25519 pub
    agent_x25519_pub_bytes = crypto_sign_ed25519_pk_to_curve25519(agent_ed25519_pub_bytes)

    # Generate ephemeral X25519 key pair
    ephemeral_priv = X25519PrivateKey.generate()
    ephemeral_pub = ephemeral_priv.public_key()

    # ECDH
    agent_x25519_pub = X25519PublicKey.from_public_bytes(agent_x25519_pub_bytes)
    shared_secret = ephemeral_priv.exchange(agent_x25519_pub)

    # Derive key
    enc_key = hashlib.sha256(shared_secret).digest()

    # ChaCha20-Poly1305 encrypt
    aead = ChaCha20Poly1305(enc_key)
    nonce = os.urandom(12)
    ciphertext = aead.encrypt(nonce, plaintext, None)

    # Format: nonce || ciphertext (matches Go server's aead.Seal(nonce, nonce, ...))
    encrypted_blob = nonce + ciphertext

    return (
        base64.b64encode(encrypted_blob).decode("utf-8"),
        base64.b64encode(
            ephemeral_pub.public_bytes_raw()
        ).decode("utf-8"),
    )


# ---------------------------------------------------------------------------
# Tests: Initialization
# ---------------------------------------------------------------------------

class TestSecretsClientInit:
    def test_lazy_init_via_property(self):
        client, _ = make_test_client()
        assert client._secrets is None
        secrets = client.secrets
        assert isinstance(secrets, SecretsClient)
        # Same instance on second access
        assert client.secrets is secrets

    def test_secrets_client_has_parent_ref(self):
        client, _ = make_test_client()
        assert client.secrets._client is client


# ---------------------------------------------------------------------------
# Tests: Resolve (end-to-end decryption)
# ---------------------------------------------------------------------------

class TestResolve:
    def test_resolve_decrypts_credential(self):
        """Full round-trip: encrypt like server, decrypt in SDK."""
        client, signing_key = make_test_client()
        agent_pub_bytes = bytes(signing_key.verify_key)
        credential = b"ghp_test_github_token_12345"

        encrypted_blob_b64, ephemeral_pub_b64 = encrypt_for_agent(credential, agent_pub_bytes)

        mock_response = {
            "encryptedBlob": encrypted_blob_b64,
            "ephemeralPubKey": ephemeral_pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
            "namespace": "github-creds",
        }

        with patch.object(client, "_make_request", return_value=mock_response):
            result = client.secrets.resolve("github-creds", "read")

        assert result["credential"] == credential
        assert result["encryptionAlg"] == "X25519-ChaCha20-Poly1305"
        assert result["namespace"] == "github-creds"

    def test_resolve_different_operations(self):
        """Resolve works for different operation types."""
        client, signing_key = make_test_client()
        agent_pub_bytes = bytes(signing_key.verify_key)
        credential = b"write-token"

        blob_b64, pub_b64 = encrypt_for_agent(credential, agent_pub_bytes)
        mock_response = {
            "encryptedBlob": blob_b64,
            "ephemeralPubKey": pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client, "_make_request", return_value=mock_response) as mock_req:
            result = client.secrets.resolve("aws-creds", "write")

        assert result["credential"] == credential
        # Verify the request was signed with correct namespace/operation
        call_args = mock_req.call_args
        payload = call_args[1]["data"] if "data" in call_args[1] else call_args[0][2]
        assert payload["namespace"] == "aws-creds"
        assert payload["operation"] == "write"

    def test_resolve_requires_signing_key(self):
        """Resolve fails if client has no Ed25519 key (API key only mode)."""
        client = AIMClient(
            agent_id="test",
            aim_url="http://localhost:8080",
            api_key="aim_live_test123",
        )
        with pytest.raises(SecretsError, match="signing key required"):
            client.secrets.resolve("github-creds")

    def test_resolve_incomplete_response(self):
        """Resolve fails on missing encryptedBlob."""
        client, _ = make_test_client()
        mock_response = {"encryptionAlg": "X25519-ChaCha20-Poly1305"}

        with patch.object(client, "_make_request", return_value=mock_response):
            with pytest.raises(SecretsError, match="incomplete resolution response"):
                client.secrets.resolve("github-creds")

    def test_resolve_tampered_ciphertext(self):
        """Tampered ciphertext fails authentication."""
        client, signing_key = make_test_client()
        agent_pub_bytes = bytes(signing_key.verify_key)

        blob_b64, pub_b64 = encrypt_for_agent(b"secret", agent_pub_bytes)
        # Tamper: flip a bit in the ciphertext
        blob = bytearray(base64.b64decode(blob_b64))
        blob[15] ^= 0xFF  # Flip a byte in the ciphertext portion
        tampered_b64 = base64.b64encode(bytes(blob)).decode("utf-8")

        mock_response = {
            "encryptedBlob": tampered_b64,
            "ephemeralPubKey": pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client, "_make_request", return_value=mock_response):
            with pytest.raises(SecretsError, match="Failed to decrypt"):
                client.secrets.resolve("test-ns")

    def test_resolve_wrong_key_fails(self):
        """Decryption with wrong agent key fails."""
        client, _ = make_test_client()

        # Encrypt for a DIFFERENT agent's key
        other_key = SigningKey.generate()
        other_pub = bytes(other_key.verify_key)
        blob_b64, pub_b64 = encrypt_for_agent(b"secret", other_pub)

        mock_response = {
            "encryptedBlob": blob_b64,
            "ephemeralPubKey": pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client, "_make_request", return_value=mock_response):
            with pytest.raises(SecretsError, match="Failed to decrypt"):
                client.secrets.resolve("test-ns")

    def test_resolve_blob_too_short(self):
        """Blob shorter than nonce+tag raises error."""
        client, _ = make_test_client()
        short_blob = base64.b64encode(b"short").decode("utf-8")

        mock_response = {
            "encryptedBlob": short_blob,
            "ephemeralPubKey": base64.b64encode(os.urandom(32)).decode("utf-8"),
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client, "_make_request", return_value=mock_response):
            with pytest.raises(SecretsError, match="too short"):
                client.secrets.resolve("test-ns")

    def test_resolve_sends_correct_endpoint(self):
        """Verify resolve calls POST /api/v1/secrets/resolve."""
        client, signing_key = make_test_client()
        agent_pub_bytes = bytes(signing_key.verify_key)
        blob_b64, pub_b64 = encrypt_for_agent(b"cred", agent_pub_bytes)

        mock_response = {
            "encryptedBlob": blob_b64,
            "ephemeralPubKey": pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client, "_make_request", return_value=mock_response) as mock_req:
            client.secrets.resolve("my-ns", "read")

        mock_req.assert_called_once()
        args = mock_req.call_args
        assert args[0][0] == "POST"
        assert args[0][1] == "/api/v1/secrets/resolve"


# ---------------------------------------------------------------------------
# Tests: Resolve refuses a field the signed message cannot carry
# ---------------------------------------------------------------------------

UNSIGNABLE_VALUES = [
    pytest.param("a\nb", "a line feed", id="LF"),
    pytest.param("a\rb", "a carriage return", id="CR"),
    pytest.param("a|b", "'|'", id="pipe"),
]


class TestResolveRefusesUnsignableFields:
    """resolve() signs "namespace|operation|nonce": neither field may hold LF, CR or "|"."""

    @pytest.mark.parametrize("field", ["namespace", "operation"])
    @pytest.mark.parametrize("value, named", UNSIGNABLE_VALUES)
    def test_refuses_before_signing_and_sends_nothing(self, field, value, named):
        client, _ = make_test_client()
        fields = {"namespace": "my-ns", "operation": "read", field: value}

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(fields["namespace"], fields["operation"])

        assert f"{field} contains {named}" in str(excinfo.value)
        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    def test_fields_without_those_characters_are_signed_and_sent_once(self):
        """Control: the same call with plain fields signs one message and sends one request."""
        client, signing_key = make_test_client()
        blob_b64, pub_b64 = encrypt_for_agent(b"cred", bytes(signing_key.verify_key))
        mock_response = {
            "encryptedBlob": blob_b64,
            "ephemeralPubKey": pub_b64,
            "encryptionAlg": "X25519-ChaCha20-Poly1305",
        }

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request", return_value=mock_response) as mock_req:
            result = client.secrets.resolve("my-ns", "read")

        assert result["credential"] == b"cred"
        mock_req.assert_called_once()
        payload = mock_req.call_args[1]["data"]
        signed_message = f"my-ns|read|{payload['nonce']}".encode("utf-8")
        mock_sign.assert_called_once_with(signed_message)
        signing_key.verify_key.verify(signed_message, base64.b64decode(payload["signature"]))


# ---------------------------------------------------------------------------
# Tests: Resolve never signs a message that starts as another signed format
# ---------------------------------------------------------------------------

# Namespaces that start as the DSSE v1 pre-authentication encoding of a
# statement: "DSSEv1", a space, the payload type's length, the type, a space.
DSSE_PREFIXED_NAMESPACES = [
    pytest.param("DSSEv1 45 application/vnd.opena2a.agent-request.v2+json 2 {}", id="agent-request-v2"),
    pytest.param("DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}", id="action-request-v1"),
    pytest.param(
        "DSSEv1 53 application/vnd.opena2a.delegation-revocation.v1+json 2 {}", id="delegation-revocation-v1"
    ),
    pytest.param("DSSEv1 ", id="prefix-alone"),
]

# Namespaces that hold "DSSEv1" somewhere other than as the first seven
# characters "DSSEv1 ", so the signed message does not start with it.
NAMESPACES_NOT_DSSE_PREFIXED = [
    pytest.param("DSSEv1", id="no-space"),
    pytest.param("dssev1 creds", id="lower-case"),
    pytest.param(" DSSEv1 creds", id="leading-space"),
    pytest.param("creds DSSEv1 x", id="not-at-start"),
]

# The text of an agent-request-v1 signature starts with the method in upper
# case and a line feed.
METHOD_LINE_NAMESPACES = [
    pytest.param(f"{method}\n/api/v1/agents", id=method) for method in ("GET", "POST", "PUT", "PATCH", "DELETE")
]


def resolvable_response(signing_key):
    blob_b64, pub_b64 = encrypt_for_agent(b"cred", bytes(signing_key.verify_key))
    return {
        "encryptedBlob": blob_b64,
        "ephemeralPubKey": pub_b64,
        "encryptionAlg": "X25519-ChaCha20-Poly1305",
    }


class TestResolveRefusesAnotherFormatsStart:
    """resolve() signs "namespace|operation|nonce", which starts with the caller's namespace."""

    @pytest.mark.parametrize("namespace", DSSE_PREFIXED_NAMESPACES)
    def test_refuses_a_namespace_beginning_with_the_dsse_prefix(self, namespace):
        client, _ = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(namespace, "read")

        assert str(excinfo.value) == (
            "Cannot resolve secret: namespace begins with 'DSSEv1 ', the first bytes of a DSSE "
            "statement an agent key signs. A namespace cannot begin with 'DSSEv1 '."
        )
        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    @pytest.mark.parametrize("namespace", METHOD_LINE_NAMESPACES)
    def test_refuses_a_namespace_beginning_with_a_method_and_a_line_feed(self, namespace):
        client, _ = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError, match="namespace contains a line feed"):
                client.secrets.resolve(namespace, "read")

        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    @pytest.mark.parametrize("namespace", NAMESPACES_NOT_DSSE_PREFIXED)
    def test_signs_and_sends_a_namespace_not_beginning_with_the_prefix(self, namespace):
        """Control: only the first seven characters "DSSEv1 " are refused."""
        client, signing_key = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request", return_value=resolvable_response(signing_key)) as mock_req:
            result = client.secrets.resolve(namespace, "read")

        assert result["credential"] == b"cred"
        mock_req.assert_called_once()
        payload = mock_req.call_args[1]["data"]
        signed_message = f"{namespace}|read|{payload['nonce']}".encode("utf-8")
        mock_sign.assert_called_once_with(signed_message)
        signing_key.verify_key.verify(signed_message, base64.b64decode(payload["signature"]))


# ---------------------------------------------------------------------------
# Tests: Resolve checks, signs and sends the text a field holds
# ---------------------------------------------------------------------------

class RendersOtherText(str):
    """A str whose str() and format() give other text than it holds.

    A member of an enum that mixes in str renders as "<Class>.<member>" through
    both on Python 3.11 to 3.14.
    """

    def __str__(self):
        return "Other.TEXT"


class FormatsHeldText(str):
    """A str whose str() gives other text than it holds, while format() gives the held text.

    A member of an enum that mixes in str renders this way on Python 3.9 and 3.10.
    """

    def __str__(self):
        return "Other.TEXT"

    def __format__(self, format_spec):
        return str.__format__(str.__str__(self), format_spec)


class Namespace(str, enum.Enum):
    CREDS = "creds"
    PIPED = "a|b"
    LINE_FEED = "a\nb"
    DSSE = "DSSEv1 x"


class Operation(str, enum.Enum):
    READ = "read"
    PIPED = "a|b"


# Each holds the text "creds" (namespace) or "read" (operation).
FIELDS_HOLDING_OTHER_TEXT_THAN_THEY_RENDER = [
    pytest.param(RendersOtherText("creds"), "read", id="namespace-renders-other-text"),
    pytest.param(FormatsHeldText("creds"), "read", id="namespace-formats-held-text"),
    pytest.param(Namespace.CREDS, "read", id="namespace-enum-member"),
    pytest.param("creds", RendersOtherText("read"), id="operation-renders-other-text"),
    pytest.param("creds", FormatsHeldText("read"), id="operation-formats-held-text"),
    pytest.param("creds", Operation.READ, id="operation-enum-member"),
]

STR_SUBCLASSES = [
    pytest.param(RendersOtherText, id="renders-other-text"),
    pytest.param(FormatsHeldText, id="formats-held-text"),
]

DSSE_REFUSAL = (
    "Cannot resolve secret: namespace begins with 'DSSEv1 ', the first bytes of a DSSE "
    "statement an agent key signs. A namespace cannot begin with 'DSSEv1 '."
)


class TestResolveSignsAndSendsTheTextAFieldHolds:
    """resolve() checks, signs and sends one text per field: the text the request body carries."""

    @pytest.mark.parametrize("namespace, operation", FIELDS_HOLDING_OTHER_TEXT_THAN_THEY_RENDER)
    def test_signs_and_sends_the_text_a_str_subclass_holds(self, namespace, operation):
        client, signing_key = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request", return_value=resolvable_response(signing_key)) as mock_req:
            result = client.secrets.resolve(namespace, operation)

        assert result["credential"] == b"cred"
        mock_req.assert_called_once()
        payload = mock_req.call_args[1]["data"]
        # The body as the server reads it; it rebuilds namespace|operation|nonce from it.
        body = json.loads(json.dumps(payload, sort_keys=True))
        assert (body["namespace"], body["operation"]) == ("creds", "read")
        signed_message = f"creds|read|{body['nonce']}".encode("utf-8")
        mock_sign.assert_called_once_with(signed_message)
        signing_key.verify_key.verify(signed_message, base64.b64decode(body["signature"]))

    @pytest.mark.parametrize("subclass", STR_SUBCLASSES)
    @pytest.mark.parametrize("field", ["namespace", "operation"])
    @pytest.mark.parametrize("value, named", UNSIGNABLE_VALUES)
    def test_refuses_a_character_the_held_text_carries(self, value, named, field, subclass):
        """str() of each value gives "Other.TEXT", which holds no refused character."""
        client, _ = make_test_client()
        fields = {"namespace": "my-ns", "operation": "read", field: subclass(value)}

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(fields["namespace"], fields["operation"])

        assert f"{field} contains {named}" in str(excinfo.value)
        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    @pytest.mark.parametrize("subclass", STR_SUBCLASSES)
    def test_refuses_a_namespace_whose_held_text_begins_with_the_dsse_prefix(self, subclass):
        client, _ = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(subclass("DSSEv1 x"), "read")

        assert str(excinfo.value) == DSSE_REFUSAL
        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    @pytest.mark.parametrize(
        "namespace, operation, refusal",
        [
            pytest.param(Namespace.PIPED, "read", "namespace contains '|'", id="namespace-pipe"),
            pytest.param(Namespace.LINE_FEED, "read", "namespace contains a line feed", id="namespace-line-feed"),
            pytest.param(Namespace.DSSE, "read", DSSE_REFUSAL, id="namespace-dsse-prefix"),
            pytest.param("creds", Operation.PIPED, "operation contains '|'", id="operation-pipe"),
        ],
    )
    def test_refuses_an_enum_member_by_its_value(self, namespace, operation, refusal):
        client, _ = make_test_client()

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(namespace, operation)

        assert refusal in str(excinfo.value)
        mock_sign.assert_not_called()
        mock_req.assert_not_called()

    @pytest.mark.parametrize("field", ["namespace", "operation"])
    @pytest.mark.parametrize(
        "value, type_name",
        [
            pytest.param(None, "NoneType", id="None"),
            pytest.param(5, "int", id="int"),
            pytest.param(b"creds", "bytes", id="bytes"),
        ],
    )
    def test_refuses_a_field_that_is_not_a_str(self, value, type_name, field):
        client, _ = make_test_client()
        fields = {"namespace": "my-ns", "operation": "read", field: value}

        with patch.object(client.signing_key, "sign", wraps=client.signing_key.sign) as mock_sign, \
                patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.resolve(fields["namespace"], fields["operation"])

        assert str(excinfo.value) == f"Cannot resolve secret: {field} must be a str, not {type_name}."
        mock_sign.assert_not_called()
        mock_req.assert_not_called()


# ---------------------------------------------------------------------------
# Tests: Namespace CRUD
# ---------------------------------------------------------------------------

class TestNamespaceCRUD:
    def test_create_namespace(self):
        client, _ = make_test_client()
        mock_resp = {"id": "ns-123", "namespace": "github", "status": "active"}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.create_namespace(
                agent_id="agent-1",
                namespace="github",
                operations=["read", "write"],
                url_patterns=["https://api.github.com/*"],
            )

        assert result["id"] == "ns-123"
        mock_req.assert_called_once_with(
            "POST",
            "/api/v1/secrets/namespaces",
            data={
                "agentId": "agent-1",
                "namespace": "github",
                "operations": ["read", "write"],
                "urlPatterns": ["https://api.github.com/*"],
            },
        )

    def test_list_namespaces(self):
        client, _ = make_test_client()
        mock_resp = {"namespaces": [{"id": "ns-1"}, {"id": "ns-2"}]}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.list_namespaces("agent-1")

        assert len(result) == 2
        mock_req.assert_called_once_with("GET", "/api/v1/secrets/namespaces?agentId=agent-1")

    def test_list_namespaces_empty(self):
        client, _ = make_test_client()
        mock_resp = {"namespaces": None}

        with patch.object(client, "_make_request", return_value=mock_resp):
            result = client.secrets.list_namespaces("agent-1")

        assert result == []

    def test_get_namespace(self):
        client, _ = make_test_client()
        mock_resp = {"id": "ns-123", "namespace": "github", "status": "active"}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.get_namespace("ns-123")

        assert result["namespace"] == "github"
        mock_req.assert_called_once_with("GET", "/api/v1/secrets/namespaces/ns-123")

    def test_delete_namespace(self):
        client, _ = make_test_client()
        mock_resp = {"message": "Namespace deleted"}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.delete_namespace("ns-123")

        assert result["message"] == "Namespace deleted"
        mock_req.assert_called_once_with("DELETE", "/api/v1/secrets/namespaces/ns-123")


# ---------------------------------------------------------------------------
# Tests: create_namespace refuses the namespaces resolve() refuses to sign
# ---------------------------------------------------------------------------

UNRESOLVABLE_NAMESPACES = [
    pytest.param(
        "team\nprod",
        "Cannot create namespace: namespace contains a line feed. resolve() refuses to sign a namespace "
        "that contains a line feed, a carriage return or '|'.",
        id="LF",
    ),
    pytest.param(
        "team\rprod",
        "Cannot create namespace: namespace contains a carriage return. resolve() refuses to sign a namespace "
        "that contains a line feed, a carriage return or '|'.",
        id="CR",
    ),
    pytest.param(
        "team|prod",
        "Cannot create namespace: namespace contains '|'. resolve() refuses to sign a namespace "
        "that contains a line feed, a carriage return or '|'.",
        id="pipe",
    ),
    pytest.param(
        "DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}",
        "Cannot create namespace: namespace begins with 'DSSEv1 ', the first bytes of a DSSE statement "
        "an agent key signs. resolve() refuses to sign a namespace that begins with 'DSSEv1 '.",
        id="dsse-prefix",
    ),
]


class TestCreateNamespaceRefusesWhatResolveRefuses:
    """A namespace created under a name resolve() refuses to sign could not be resolved."""

    @pytest.mark.parametrize("namespace, refusal", UNRESOLVABLE_NAMESPACES)
    def test_refuses_before_sending(self, namespace, refusal):
        client, _ = make_test_client()

        with patch.object(client, "_make_request") as mock_req:
            with pytest.raises(SecretsError) as excinfo:
                client.secrets.create_namespace("agent-1", namespace, operations=["read"])

        assert str(excinfo.value) == refusal
        mock_req.assert_not_called()

    @pytest.mark.parametrize("namespace", [pytest.param("team-prod", id="plain"), *NAMESPACES_NOT_DSSE_PREFIXED])
    def test_sends_a_namespace_resolve_signs_once(self, namespace):
        """Control: only those three characters, and "DSSEv1 " at the start, are refused."""
        client, _ = make_test_client()

        with patch.object(client, "_make_request", return_value={"id": "ns-123"}) as mock_req:
            client.secrets.create_namespace("agent-1", namespace, operations=["read"])

        mock_req.assert_called_once_with(
            "POST",
            "/api/v1/secrets/namespaces",
            data={"agentId": "agent-1", "namespace": namespace, "operations": ["read"], "urlPatterns": []},
        )


# ---------------------------------------------------------------------------
# Tests: Credential Store/Rotate
# ---------------------------------------------------------------------------

class TestCredentialStorage:
    def test_store_credential(self):
        client, _ = make_test_client()
        mock_resp = {"message": "Credential stored"}
        blob = b"encrypted-data"

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.store_credential("ns-123", blob)

        assert result["message"] == "Credential stored"
        # The server reads the algorithm as encryptionAlg.
        mock_req.assert_called_once_with(
            "POST",
            "/api/v1/secrets/namespaces/ns-123/credentials",
            data={
                "encryptedBlob": base64.b64encode(blob).decode("utf-8"),
                "encryptionAlg": "X25519-ChaCha20-Poly1305",
            },
        )

    def test_store_credential_with_an_ephemeral_key(self):
        client, _ = make_test_client()
        blob = b"encrypted-data"
        ephemeral = bytes(range(32))

        with patch.object(client, "_make_request", return_value={}) as mock_req:
            with pytest.warns(DeprecationWarning, match="ephemeral_public_key"):
                client.secrets.store_credential("ns-123", blob, encryption_alg="AES-256-GCM", ephemeral_public_key=ephemeral)

        # The store route reads encryptedBlob and encryptionAlg only, so the
        # deprecated ephemeral key is not sent.
        mock_req.assert_called_once_with(
            "POST",
            "/api/v1/secrets/namespaces/ns-123/credentials",
            data={
                "encryptedBlob": base64.b64encode(blob).decode("utf-8"),
                "encryptionAlg": "AES-256-GCM",
            },
        )

    def test_rotate_credential(self):
        client, _ = make_test_client()
        mock_resp = {"message": "Credential rotated"}
        blob = b"new-encrypted-data"
        ephemeral = bytes(range(32, 64))

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            with pytest.warns(DeprecationWarning, match="ephemeral_public_key"):
                result = client.secrets.rotate_credential("ns-123", blob, ephemeral_public_key=ephemeral)

        assert result["message"] == "Credential rotated"
        # The server reads the algorithm as encryptionAlg, and the rotate route
        # reads no ephemeral key.
        mock_req.assert_called_once_with(
            "POST",
            "/api/v1/secrets/namespaces/ns-123/rotate",
            data={
                "encryptedBlob": base64.b64encode(blob).decode("utf-8"),
                "encryptionAlg": "X25519-ChaCha20-Poly1305",
            },
        )


# ---------------------------------------------------------------------------
# Tests: Audit Log
# ---------------------------------------------------------------------------

class TestAuditLog:
    def test_get_audit_log(self):
        client, _ = make_test_client()
        mock_resp = {"entries": [{"operation": "resolve", "result": "granted"}]}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.get_audit_log("agent-1")

        assert len(result) == 1
        assert result[0]["result"] == "granted"
        mock_req.assert_called_once_with("GET", "/api/v1/secrets/audit?agentId=agent-1&limit=50&offset=0")

    def test_get_audit_log_empty(self):
        client, _ = make_test_client()
        mock_resp = {"entries": None}

        with patch.object(client, "_make_request", return_value=mock_resp):
            result = client.secrets.get_audit_log("agent-1")

        assert result == []

    def test_get_audit_log_with_params(self):
        client, _ = make_test_client()
        mock_resp = {"entries": []}

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            client.secrets.get_audit_log("agent-1", since="2026-04-12T00:00:00Z", limit=10, offset=5)

        mock_req.assert_called_once_with(
            "GET",
            "/api/v1/secrets/audit?agentId=agent-1&limit=10&offset=5&since=2026-04-12T00%3A00%3A00Z",
        )

    # The server decodes the query as a form is decoded, so a "+" sent as is
    # reads as a space: the audit handler reads since=2026-10-16T10:00:00+02:00
    # as "2026-10-16T10:00:00 02:00", which its RFC 3339 parse refuses, and
    # answers 400. Decoding the path the same way gives back the since passed.
    @pytest.mark.parametrize(
        "since",
        [
            "2026-10-16T10:00:00+02:00",
            "2026-10-16T10:00:00.250+05:30",
            "2026-10-16T10:00:00-03:00",
            "2026-10-16T10:00:00Z",
        ],
    )
    def test_get_audit_log_sends_since_as_passed(self, since):
        client, _ = make_test_client()

        with patch.object(client, "_make_request", return_value={"entries": []}) as mock_req:
            client.secrets.get_audit_log("agent-1", since=since)

        method, path = mock_req.call_args.args
        parts = urlsplit(path)
        assert method == "GET"
        assert parts.path == "/api/v1/secrets/audit"
        assert parse_qsl(parts.query, strict_parsing=True) == [
            ("agentId", "agent-1"),
            ("limit", "50"),
            ("offset", "0"),
            ("since", since),
        ]

    def test_get_audit_log_sends_a_positive_offset_in_the_form_the_handler_reads(self):
        # The backend's TestSecretsHandler_GetAuditLog_PassesTheFiltersItReadsToTheRepository
        # sends since=2026-10-16T10%3A00%3A00%2B02%3A00 to the audit handler,
        # which reads it as 08:00 UTC.
        client, _ = make_test_client()

        with patch.object(client, "_make_request", return_value={"entries": []}) as mock_req:
            client.secrets.get_audit_log("agent-1", since="2026-10-16T10:00:00+02:00")

        mock_req.assert_called_once_with(
            "GET",
            "/api/v1/secrets/audit?agentId=agent-1&limit=50&offset=0&since=2026-10-16T10%3A00%3A00%2B02%3A00",
        )


# ---------------------------------------------------------------------------
# Tests: Query values
# ---------------------------------------------------------------------------

class TestQueryValues:
    """A character that is syntax in a query string is data in a value: it
    adds no parameter, ends no query and changes no other value."""

    AGENT_IDS = ["a1&limit=1#x y", "a+b", "a%2Bb"]

    @pytest.mark.parametrize("agent_id", AGENT_IDS)
    def test_get_audit_log_writes_the_agent_id_as_one_value(self, agent_id):
        client, _ = make_test_client()

        with patch.object(client, "_make_request", return_value={"entries": []}) as mock_req:
            client.secrets.get_audit_log(agent_id, limit=7, offset=3)

        parts = urlsplit(mock_req.call_args.args[1])
        assert parts.path == "/api/v1/secrets/audit"
        assert parse_qsl(parts.query, strict_parsing=True) == [
            ("agentId", agent_id),
            ("limit", "7"),
            ("offset", "3"),
        ]

    @pytest.mark.parametrize("agent_id", AGENT_IDS)
    def test_list_namespaces_writes_the_agent_id_as_one_value(self, agent_id):
        client, _ = make_test_client()

        with patch.object(client, "_make_request", return_value={"namespaces": []}) as mock_req:
            client.secrets.list_namespaces(agent_id)

        parts = urlsplit(mock_req.call_args.args[1])
        assert parts.path == "/api/v1/secrets/namespaces"
        assert parse_qsl(parts.query, strict_parsing=True) == [("agentId", agent_id)]

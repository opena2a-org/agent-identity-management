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
import hashlib
import os
import time
from unittest.mock import MagicMock, patch

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
        call_data = mock_req.call_args[1]["data"]
        assert call_data["encryptedBlob"] == base64.b64encode(blob).decode("utf-8")
        assert call_data["encryptionAlgorithm"] == "X25519-ChaCha20-Poly1305"

    def test_rotate_credential(self):
        client, _ = make_test_client()
        mock_resp = {"message": "Credential rotated"}
        blob = b"new-encrypted-data"

        with patch.object(client, "_make_request", return_value=mock_resp) as mock_req:
            result = client.secrets.rotate_credential("ns-123", blob)

        assert result["message"] == "Credential rotated"
        mock_req.assert_called_once()
        assert "/rotate" in mock_req.call_args[0][1]


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

        url = mock_req.call_args[0][1]
        assert "limit=10" in url
        assert "offset=5" in url
        assert "since=2026-04-12T00:00:00Z" in url

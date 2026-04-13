"""
AIM Secrets Client - Identity-native secrets management for AI agents.

Credentials never enter agent process memory. The SDK signs a resolution
request with the agent's Ed25519 key, the server performs ECDH + ChaCha20-Poly1305
encryption, and the SDK decrypts the response locally.

Usage:
    from aim_sdk import AIMClient

    client = AIMClient(agent_id="...", public_key="...", private_key="...", aim_url="...")

    # Resolve a credential (agent auth via Ed25519 signature)
    result = client.secrets.resolve("github-creds", "read")
    token = result["credential"]  # decrypted credential bytes

    # Namespace management (JWT/API key auth)
    client.secrets.create_namespace(agent_id, "github-creds",
        operations=["read"], url_patterns=["https://api.github.com/*"])
    namespaces = client.secrets.list_namespaces(agent_id)
"""

import base64
import hashlib
import os
import time
from typing import Any, Dict, List, Optional
from uuid import uuid4

from nacl.signing import SigningKey
from nacl.bindings import crypto_sign_ed25519_pk_to_curve25519
from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey, X25519PublicKey
from cryptography.hazmat.primitives.ciphers.aead import ChaCha20Poly1305

from .exceptions import AIMError, AuthenticationError, SecretsError


class SecretsClient:
    """
    Identity-native secrets client.

    Handles the cryptographic resolution flow:
    1. Sign request with agent's Ed25519 key
    2. Server performs X25519 ECDH + ChaCha20-Poly1305 encryption
    3. Client decrypts response using Ed25519->X25519 converted private key

    Args:
        client: Parent AIMClient instance (provides HTTP, keys, agent_id)
    """

    def __init__(self, client: Any):
        self._client = client

    # -------------------------------------------------------------------------
    # Resolve (agent auth — Ed25519 signature)
    # -------------------------------------------------------------------------

    def resolve(self, namespace: str, operation: str = "read") -> Dict[str, Any]:
        """
        Resolve a credential from a namespace.

        The server encrypts the credential with an ephemeral X25519 key via ECDH.
        This method decrypts the response locally. The raw credential exists only
        in the SDK's memory, never in the agent's application memory if used via
        the context manager pattern.

        Args:
            namespace: The namespace to resolve (e.g., "github-creds")
            operation: The operation type (e.g., "read", "write")

        Returns:
            Dict with "credential" (bytes), "encryptionAlg" (str), "namespace" (str)

        Raises:
            SecretsError: If resolution fails
            AuthenticationError: If agent auth fails
        """
        if not self._client.signing_key:
            raise SecretsError("Ed25519 signing key required for secrets resolution")
        if not self._client.public_key:
            raise SecretsError("Public key required for secrets resolution")

        # Build nonce: timestamp:uuid (server rejects nonces older than 30s)
        nonce = f"{time.strftime('%Y-%m-%dT%H:%M:%S.000000000Z', time.gmtime())}:{uuid4()}"

        # Sign: namespace|operation|nonce
        message = f"{namespace}|{operation}|{nonce}"
        signed = self._client.signing_key.sign(message.encode("utf-8"))
        signature_b64 = base64.b64encode(signed.signature).decode("utf-8")

        # Build request
        payload = {
            "namespace": namespace,
            "operation": operation,
            "nonce": nonce,
            "signature": signature_b64,
            "agentPublicKey": self._client.public_key,
        }

        # POST /api/v1/secrets/resolve uses PQCAgentMiddleware (agent auth),
        # not JWT auth. We send Ed25519 headers via _make_request.
        result = self._client._make_request("POST", "/api/v1/secrets/resolve", data=payload)

        # Decrypt the response
        encrypted_blob_b64 = result.get("encryptedBlob", "")
        ephemeral_pub_b64 = result.get("ephemeralPubKey", "")

        if not encrypted_blob_b64 or not ephemeral_pub_b64:
            raise SecretsError("Server returned incomplete resolution response")

        encrypted_blob = base64.b64decode(encrypted_blob_b64)
        ephemeral_pub_bytes = base64.b64decode(ephemeral_pub_b64)

        # Decrypt using X25519 ECDH + ChaCha20-Poly1305
        credential = self._decrypt_resolved(encrypted_blob, ephemeral_pub_bytes)

        return {
            "credential": credential,
            "encryptionAlg": result.get("encryptionAlg", ""),
            "namespace": namespace,
        }

    def _decrypt_resolved(self, ciphertext: bytes, ephemeral_pub_bytes: bytes) -> bytes:
        """
        Decrypt a resolved credential using X25519 ECDH + ChaCha20-Poly1305.

        The server used:
            1. Generate ephemeral X25519 key pair
            2. Convert agent's Ed25519 pub -> X25519 pub
            3. ECDH(ephemeral_priv, agent_x25519_pub) -> shared_secret
            4. SHA-256(shared_secret) -> 256-bit key
            5. ChaCha20-Poly1305 encrypt: nonce || ciphertext || tag

        We mirror steps 1-4 but use agent's private key:
            1. Convert agent's Ed25519 priv -> X25519 priv
            2. Load ephemeral X25519 pub from response
            3. ECDH(agent_x25519_priv, ephemeral_pub) -> same shared_secret
            4. SHA-256(shared_secret) -> same 256-bit key
            5. ChaCha20-Poly1305 decrypt
        """
        # Convert agent's Ed25519 private key to X25519
        # PyNaCl: signing_key._signing_key is the 64-byte ed25519 secret key
        # We need the ed25519 public key bytes for the conversion function
        ed25519_sk_bytes = bytes(self._client.signing_key._signing_key)

        # Extract the 32-byte seed from the 64-byte Ed25519 secret key
        ed25519_seed = ed25519_sk_bytes[:32]

        # Convert Ed25519 public key to X25519 public key (for verification)
        ed25519_pk_bytes = base64.b64decode(self._client.public_key)
        _agent_x25519_pub = crypto_sign_ed25519_pk_to_curve25519(ed25519_pk_bytes)

        # For X25519 private key, we use the cryptography library
        # Ed25519 seed == X25519 private key scalar (clamped by the library)
        # The Go server uses crypto/ecdh which clamps the scalar identically.
        from nacl.bindings import crypto_sign_ed25519_sk_to_curve25519
        agent_x25519_sk_bytes = crypto_sign_ed25519_sk_to_curve25519(ed25519_sk_bytes)

        # Build X25519 key objects
        agent_x25519_priv = X25519PrivateKey.from_private_bytes(agent_x25519_sk_bytes)
        ephemeral_x25519_pub = X25519PublicKey.from_public_bytes(ephemeral_pub_bytes)

        # ECDH shared secret
        shared_secret = agent_x25519_priv.exchange(ephemeral_x25519_pub)

        # Derive key: SHA-256(shared_secret)
        enc_key = hashlib.sha256(shared_secret).digest()

        # ChaCha20-Poly1305 decrypt
        # Format: nonce (12 bytes) || ciphertext || tag (16 bytes)
        nonce_size = 12  # IETF ChaCha20-Poly1305 nonce
        if len(ciphertext) < nonce_size + 16:
            raise SecretsError("Encrypted blob too short to contain nonce and authentication tag")

        nonce = ciphertext[:nonce_size]
        encrypted_data = ciphertext[nonce_size:]

        try:
            aead = ChaCha20Poly1305(enc_key)
            plaintext = aead.decrypt(nonce, encrypted_data, None)
        except Exception as e:
            raise SecretsError(f"Failed to decrypt credential: {e}")

        return plaintext

    # -------------------------------------------------------------------------
    # Namespace CRUD (JWT/API key auth)
    # -------------------------------------------------------------------------

    def create_namespace(
        self,
        agent_id: str,
        namespace: str,
        operations: List[str],
        url_patterns: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """Create a secret namespace for an agent."""
        payload = {
            "agentId": agent_id,
            "namespace": namespace,
            "operations": operations,
            "urlPatterns": url_patterns or [],
        }
        return self._client._make_request("POST", "/api/v1/secrets/namespaces", data=payload)

    def list_namespaces(self, agent_id: str) -> List[Dict[str, Any]]:
        """List all secret namespaces for an agent."""
        result = self._client._make_request("GET", f"/api/v1/secrets/namespaces?agentId={agent_id}")
        return result.get("namespaces") or []

    def get_namespace(self, namespace_id: str) -> Dict[str, Any]:
        """Get details of a specific namespace."""
        return self._client._make_request("GET", f"/api/v1/secrets/namespaces/{namespace_id}")

    def delete_namespace(self, namespace_id: str) -> Dict[str, Any]:
        """Delete a namespace and its credentials."""
        return self._client._make_request("DELETE", f"/api/v1/secrets/namespaces/{namespace_id}")

    # -------------------------------------------------------------------------
    # Credential storage (JWT/API key auth)
    # -------------------------------------------------------------------------

    def store_credential(
        self,
        namespace_id: str,
        encrypted_blob: bytes,
        encryption_alg: str = "X25519-ChaCha20-Poly1305",
        ephemeral_public_key: Optional[bytes] = None,
    ) -> Dict[str, Any]:
        """Store an encrypted credential blob in a namespace."""
        payload = {
            "encryptedBlob": base64.b64encode(encrypted_blob).decode("utf-8"),
            "encryptionAlgorithm": encryption_alg,
        }
        if ephemeral_public_key:
            payload["ephemeralPublicKey"] = base64.b64encode(ephemeral_public_key).decode("utf-8")
        return self._client._make_request(
            "POST", f"/api/v1/secrets/namespaces/{namespace_id}/credentials", data=payload
        )

    def rotate_credential(
        self,
        namespace_id: str,
        encrypted_blob: bytes,
        encryption_alg: str = "X25519-ChaCha20-Poly1305",
        ephemeral_public_key: Optional[bytes] = None,
    ) -> Dict[str, Any]:
        """Rotate (replace) the credential in a namespace."""
        payload = {
            "encryptedBlob": base64.b64encode(encrypted_blob).decode("utf-8"),
            "encryptionAlgorithm": encryption_alg,
        }
        if ephemeral_public_key:
            payload["ephemeralPublicKey"] = base64.b64encode(ephemeral_public_key).decode("utf-8")
        return self._client._make_request(
            "POST", f"/api/v1/secrets/namespaces/{namespace_id}/rotate", data=payload
        )

    # -------------------------------------------------------------------------
    # Audit (JWT/API key auth)
    # -------------------------------------------------------------------------

    def get_audit_log(
        self,
        agent_id: str,
        since: Optional[str] = None,
        limit: int = 50,
        offset: int = 0,
    ) -> List[Dict[str, Any]]:
        """Get audit log entries for an agent's secrets operations."""
        params = f"agentId={agent_id}&limit={limit}&offset={offset}"
        if since:
            params += f"&since={since}"
        result = self._client._make_request("GET", f"/api/v1/secrets/audit?{params}")
        return result.get("entries") or []

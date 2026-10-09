package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// JWKSHandler serves the server's published signing keys.
type JWKSHandler struct {
	keys crypto.JWKSet
}

// NewJWKSHandler creates a handler that serves the public keys of ring. The set is built
// once from the in-memory key ring, so serving it reads no storage.
func NewJWKSHandler(ring *crypto.SigningKeyRing) *JWKSHandler {
	return &JWKSHandler{keys: ring.JWKS()}
}

// GetJWKS serves GET /.well-known/jwks.json: a JSON Web Key Set (RFC 7517) of the
// card-attestation and ATC-issuer public keys, each with its kid, purpose and status.
// It carries no private key material and needs no authentication.
func (h *JWKSHandler) GetJWKS(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.JSON(h.keys)
}

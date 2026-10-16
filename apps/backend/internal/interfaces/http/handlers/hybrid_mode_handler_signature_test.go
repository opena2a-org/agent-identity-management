package handlers

import (
	"crypto/ed25519"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An agent in hybrid mode authenticates only with an Ed25519 and an ML-DSA
// signature together. The token endpoint, legacy verification creation and the
// SDK verification read verify an Ed25519 signature themselves, outside the
// signature middlewares, so each refuses an agent in hybrid mode once that
// signature verifies, with the reasonCode the middlewares and the action-request
// statement use. The flag is read after the signature verifies and before the
// agent's status: a signature that does not verify gets the signature refusal,
// and a suspended hybrid-mode agent is told it needs both signatures.

// Written out as literals so this file states the contract independently of the
// declarations it checks.
const (
	handlerHybridRequiredBody = `{"error":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`
	tokenHybridRequiredBody   = `{"error":"invalid_client","error_description":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`
)

// hybridAgents is a verified and a suspended agent holding pub, both in hybrid mode.
func hybridAgents(pub ed25519.PublicKey) (verified, suspended *domain.Agent) {
	verified = refusalAgent(domain.AgentStatusVerified, pub)
	verified.HybridModeEnabled = true
	suspended = refusalAgent(domain.AgentStatusSuspended, pub)
	suspended.HybridModeEnabled = true
	return verified, suspended
}

func TestOAuthToken_RefusesAnAgentInHybridModeSigningWithEd25519Alone(t *testing.T) {
	pub, priv := mustKeypair(t)
	hybrid, suspended := hybridAgents(pub)
	app := tokenApp(t, hybrid, suspended)

	for name, id := range map[string]uuid.UUID{"verified": hybrid.ID, "suspended": suspended.ID} {
		code, body := postAssertion(t, app, id, assertion(t, priv, tokenClaims(id), nil))
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, tokenHybridRequiredBody, body, name)
	}

	code, body := postAssertion(t, app, hybrid.ID, assertion(t, priv, tokenClaims(hybrid.ID), flipByte))
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, tokenNotRecognizedBody, body, "a signature that does not verify gets the signature refusal")

	// The control: the same agent with hybrid mode off obtains a token.
	hybrid.HybridModeEnabled = false
	code, body = postAssertion(t, app, hybrid.ID, assertion(t, priv, tokenClaims(hybrid.ID), nil))
	require.Equal(t, fiber.StatusOK, code, body)
}

func TestCreateVerification_RefusesAnAgentInHybridModeSigningWithEd25519Alone(t *testing.T) {
	pub, priv := mustKeypair(t)
	hybrid, suspended := hybridAgents(pub)
	var reached bool
	app := createVerificationApp(&reached, hybrid, suspended)

	for name, id := range map[string]uuid.UUID{"verified": hybrid.ID, "suspended": suspended.ID} {
		code, body := createVerificationCase{agentID: id, signer: priv, presented: pub}.do(t, app)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, handlerHybridRequiredBody, body, name)
	}
	require.False(t, reached, "a hybrid-mode agent does not reach the capability check")

	code, body := createVerificationCase{agentID: hybrid.ID, signer: priv, presented: pub, flip: true}.do(t, app)
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, `{"error":"Signature verification failed: signature verification failed"}`, body,
		"a signature that does not verify gets the signature refusal")

	hybrid.HybridModeEnabled = false
	createVerificationCase{agentID: hybrid.ID, signer: priv, presented: pub}.do(t, app)
	require.True(t, reached, "the control must get past authentication")
}

func TestGetVerificationSDK_RefusesAnAgentInHybridModeSigningWithEd25519Alone(t *testing.T) {
	pub, priv := mustKeypair(t)
	hybrid, suspended := hybridAgents(pub)
	app, vid := sdkReadApp(hybrid, suspended)

	for name, id := range map[string]uuid.UUID{"verified": hybrid.ID, "suspended": suspended.ID} {
		code, body := sdkRead(t, app, vid, id, priv, false)
		require.Equal(t, fiber.StatusUnauthorized, code, "%s: %s", name, body)
		require.Equal(t, handlerHybridRequiredBody, body, name)
	}

	code, body := sdkRead(t, app, vid, hybrid.ID, priv, true)
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, `{"error":"signature verification failed"}`, body,
		"a signature that does not verify gets the signature refusal")

	hybrid.HybridModeEnabled = false
	code, body = sdkRead(t, app, vid, hybrid.ID, priv, false)
	require.Equal(t, fiber.StatusOK, code, body)
}

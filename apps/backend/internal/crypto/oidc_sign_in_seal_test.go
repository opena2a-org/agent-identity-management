package crypto

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCClientSecretOpensForTheOrganizationItWasSealedFor(t *testing.T) {
	kv, _ := newTestVault(t)
	org := uuid.New()

	sealed, err := kv.SealOIDCClientSecret(org, "not-a-real-client-secret")
	require.NoError(t, err)
	assert.NotContains(t, sealed, "not-a-real-client-secret")

	secret, err := kv.OpenOIDCClientSecret(org, sealed)
	require.NoError(t, err)
	assert.Equal(t, "not-a-real-client-secret", secret)

	_, err = kv.OpenOIDCClientSecret(uuid.New(), sealed)
	assert.Error(t, err, "a client secret sealed for one organization does not open for another")
}

// A ciphertext opens only for the purpose it was sealed for: a sign-in flow is
// not a client secret, neither is an agent private key, and the reverse.
func TestOIDCSealsDoNotOpenForAnotherPurpose(t *testing.T) {
	kv, _ := newTestVault(t)
	org := uuid.New()

	flow, err := kv.SealOIDCSignInFlow(org, []byte(`{"state":"s"}`))
	require.NoError(t, err)
	_, err = kv.OpenOIDCClientSecret(org, flow)
	assert.Error(t, err, "a sign-in flow does not open as a client secret")
	_, err = kv.DecryptPrivateKey(org, flow)
	assert.Error(t, err, "a sign-in flow does not open as an agent private key")

	secret, err := kv.SealOIDCClientSecret(org, "not-a-real-client-secret")
	require.NoError(t, err)
	_, err = kv.OpenOIDCSignInFlow(org, secret)
	assert.Error(t, err, "a client secret does not open as a sign-in flow")

	privateKey, err := kv.EncryptPrivateKey(org, base64.StdEncoding.EncodeToString([]byte("not-a-real-private-key")))
	require.NoError(t, err)
	_, err = kv.OpenOIDCSignInFlow(org, privateKey)
	assert.Error(t, err, "an agent private key does not open as a sign-in flow")
}

func TestOIDCSignInFlowRefusesAChangedCiphertext(t *testing.T) {
	kv, _ := newTestVault(t)
	org := uuid.New()

	sealed, err := kv.SealOIDCSignInFlow(org, []byte(`{"state":"s"}`))
	require.NoError(t, err)
	opened, err := kv.OpenOIDCSignInFlow(org, sealed)
	require.NoError(t, err)
	assert.Equal(t, `{"state":"s"}`, string(opened))

	raw, err := base64.StdEncoding.DecodeString(sealed)
	require.NoError(t, err)
	raw[len(raw)-1] ^= 0x01
	_, err = kv.OpenOIDCSignInFlow(org, base64.StdEncoding.EncodeToString(raw))
	assert.Error(t, err, "a flow changed in one byte does not open")

	_, err = kv.OpenOIDCSignInFlow(uuid.New(), sealed)
	assert.Error(t, err, "a flow sealed for one organization does not open for another")
}

func TestOIDCSealsRequireAnOrganization(t *testing.T) {
	kv, _ := newTestVault(t)

	_, err := kv.SealOIDCClientSecret(uuid.Nil, "not-a-real-client-secret")
	assert.Error(t, err)
	_, err = kv.OpenOIDCClientSecret(uuid.Nil, "x")
	assert.Error(t, err)
	_, err = kv.SealOIDCSignInFlow(uuid.Nil, []byte("{}"))
	assert.Error(t, err)
	_, err = kv.OpenOIDCSignInFlow(uuid.Nil, "x")
	assert.Error(t, err)
}

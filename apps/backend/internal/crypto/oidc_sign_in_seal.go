package crypto

import (
	"errors"

	"github.com/google/uuid"
)

// Purposes of the OIDC sign-in ciphertexts. Each is part of the additional data
// the ciphertext is sealed with, beside the organization's id, so a ciphertext
// opens only for the purpose and the organization it was sealed for, and never
// as an agent private key.
const (
	oidcClientSecretPurpose = "aim/oidc-client-secret"
	oidcSignInFlowPurpose   = "aim/oidc-sign-in-flow"
)

var errOIDCSealOrganization = errors.New("organization ID is required to seal or open OIDC sign-in data")

// SealOIDCClientSecret encrypts an organization's OIDC client secret for
// storage. The ciphertext is bound to organizationID: it opens only through
// OpenOIDCClientSecret with the same organization.
func (kv *KeyVault) SealOIDCClientSecret(organizationID uuid.UUID, secret string) (string, error) {
	if organizationID == uuid.Nil {
		return "", errOIDCSealOrganization
	}
	return kv.sealV3(oidcClientSecretPurpose, organizationID, []byte(secret))
}

// OpenOIDCClientSecret decrypts a client secret SealOIDCClientSecret sealed for
// organizationID.
func (kv *KeyVault) OpenOIDCClientSecret(organizationID uuid.UUID, sealed string) (string, error) {
	if organizationID == uuid.Nil {
		return "", errOIDCSealOrganization
	}
	plaintext, err := kv.open(oidcClientSecretPurpose, organizationID, sealed)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// SealOIDCSignInFlow encrypts the state of an OIDC sign-in in progress, which
// the client holds between the start of the sign-in and its callback. The
// ciphertext is bound to organizationID.
func (kv *KeyVault) SealOIDCSignInFlow(organizationID uuid.UUID, flow []byte) (string, error) {
	if organizationID == uuid.Nil {
		return "", errOIDCSealOrganization
	}
	return kv.sealV3(oidcSignInFlowPurpose, organizationID, flow)
}

// OpenOIDCSignInFlow decrypts a sign-in state SealOIDCSignInFlow sealed for
// organizationID. A ciphertext changed in any byte, or sealed for another
// organization or purpose, is an error.
func (kv *KeyVault) OpenOIDCSignInFlow(organizationID uuid.UUID, sealed string) ([]byte, error) {
	if organizationID == uuid.Nil {
		return nil, errOIDCSealOrganization
	}
	return kv.open(oidcSignInFlowPurpose, organizationID, sealed)
}

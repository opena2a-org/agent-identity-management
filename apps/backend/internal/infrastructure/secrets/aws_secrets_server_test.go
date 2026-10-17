package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/awssmtest"
)

// This test runs the backend through a real Secrets Manager client of the AWS
// SDK against awssmtest, an in-memory server of the Secrets Manager HTTP API
// on a loopback port. It needs no AWS account and reaches no AWS endpoint.

func assertAWSRoundTrip(t *testing.T, b *AWSSecretsBackend, nsID uuid.UUID, blob []byte, alg string, wantVersion int) {
	t.Helper()
	got, err := b.Retrieve(nsID)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !bytes.Equal(got.EncryptedBlob, blob) {
		t.Fatalf("Retrieve blob = %x, want %x", got.EncryptedBlob, blob)
	}
	if got.EncryptionAlg != alg {
		t.Errorf("Retrieve encryptionAlg = %q, want %q", got.EncryptionAlg, alg)
	}
	if got.Version != wantVersion {
		t.Errorf("Retrieve version = %d, want %d", got.Version, wantVersion)
	}
	if got.NamespaceID != nsID {
		t.Errorf("Retrieve namespaceID = %s, want %s", got.NamespaceID, nsID)
	}
}

func TestAWSSecretsBackend_StubServerStoreRetrieveRotateDelete(t *testing.T) {
	srv := awssmtest.NewServer(t)
	b := NewAWSSecretsBackendFromClient(awssmtest.NewClient(srv), "", "")
	nsID := uuid.New()
	name := "aim-secrets/" + nsID.String()

	first := []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff, 'a', 'w', 's'}
	if err := b.Store(nsID, first, "AES-256-GCM"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// The server holds the decoded SecretBinary: the backend's JSON payload.
	held, ok := srv.SecretBinary(name)
	if !ok {
		t.Fatalf("no secret named %s after Store", name)
	}
	var payload struct {
		Blob []byte `json:"blob"`
	}
	if err := json.Unmarshal(held, &payload); err != nil || !bytes.Equal(payload.Blob, first) {
		t.Fatalf("SecretBinary held by the server = %q (err %v), want the payload of the stored blob", held, err)
	}
	assertAWSRoundTrip(t, b, nsID, first, "AES-256-GCM", 1)

	second := []byte{0xff, 0x00, 0xc3, 0x28, 'r', 'o', 't'}
	if err := b.Rotate(nsID, second, "X25519-ChaCha20-Poly1305"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	assertAWSRoundTrip(t, b, nsID, second, "X25519-ChaCha20-Poly1305", 2)

	if err := b.Delete(nsID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := b.Retrieve(nsID)
	if err == nil {
		t.Fatalf("Retrieve after Delete returned %+v, want an error", got)
	}
	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		t.Fatalf("Retrieve after Delete: %v, want ResourceNotFoundException", err)
	}

	var ops []string
	for _, r := range srv.Requests() {
		ops = append(ops, r.Operation)
		if r.Operation != "DeleteSecret" {
			continue
		}
		var del struct {
			SecretID                   string `json:"SecretId"`
			ForceDeleteWithoutRecovery *bool  `json:"ForceDeleteWithoutRecovery"`
		}
		if err := json.Unmarshal(r.Body, &del); err != nil {
			t.Fatal(err)
		}
		if del.SecretID != name || del.ForceDeleteWithoutRecovery == nil || !*del.ForceDeleteWithoutRecovery {
			t.Fatalf("DeleteSecret body = %s, want SecretId %s with ForceDeleteWithoutRecovery true", r.Body, name)
		}
	}
	want := []string{"CreateSecret", "GetSecretValue", "GetSecretValue", "PutSecretValue", "GetSecretValue", "DeleteSecret", "GetSecretValue"}
	if !slices.Equal(ops, want) {
		t.Fatalf("operations = %v, want %v", ops, want)
	}
}

func TestAWSSecretsBackend_StubServerStoreOfAnExistingNamespaceFails(t *testing.T) {
	srv := awssmtest.NewServer(t)
	b := NewAWSSecretsBackendFromClient(awssmtest.NewClient(srv), "", "")
	nsID := uuid.New()
	if err := b.Store(nsID, []byte("one"), "AES-256-GCM"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := b.Store(nsID, []byte("two"), "AES-256-GCM"); err == nil {
		t.Fatal("a second Store of the same namespace succeeded; CreateSecret of an existing name must fail")
	}
	assertAWSRoundTrip(t, b, nsID, []byte("one"), "AES-256-GCM", 1)
}

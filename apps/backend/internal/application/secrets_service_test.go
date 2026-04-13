package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

// --- Mock implementations ---

type mockNamespaceRepo struct {
	namespaces map[string]*secrets.SecretNamespace // key: agentID|namespace
}

func newMockNamespaceRepo() *mockNamespaceRepo {
	return &mockNamespaceRepo{namespaces: make(map[string]*secrets.SecretNamespace)}
}

func (m *mockNamespaceRepo) Create(ns *secrets.SecretNamespace) error {
	if ns.ID == uuid.Nil {
		ns.ID = uuid.New()
	}
	ns.CreatedAt = time.Now()
	ns.UpdatedAt = time.Now()
	m.namespaces[ns.AgentID.String()+"|"+ns.Namespace] = ns
	return nil
}

func (m *mockNamespaceRepo) GetByID(id uuid.UUID) (*secrets.SecretNamespace, error) {
	for _, ns := range m.namespaces {
		if ns.ID == id {
			return ns, nil
		}
	}
	return nil, nil
}

func (m *mockNamespaceRepo) GetByAgentAndName(agentID uuid.UUID, namespace string) (*secrets.SecretNamespace, error) {
	key := agentID.String() + "|" + namespace
	return m.namespaces[key], nil
}

func (m *mockNamespaceRepo) ListByAgent(agentID uuid.UUID) ([]*secrets.SecretNamespace, error) {
	var result []*secrets.SecretNamespace
	for _, ns := range m.namespaces {
		if ns.AgentID == agentID {
			result = append(result, ns)
		}
	}
	return result, nil
}

func (m *mockNamespaceRepo) UpdateStatus(id uuid.UUID, status secrets.NamespaceStatus) error {
	for _, ns := range m.namespaces {
		if ns.ID == id {
			ns.Status = status
			return nil
		}
	}
	return nil
}

func (m *mockNamespaceRepo) Delete(id uuid.UUID) error {
	for k, ns := range m.namespaces {
		if ns.ID == id {
			delete(m.namespaces, k)
			return nil
		}
	}
	return nil
}

type mockAuditRepo struct {
	entries []*secrets.SecretAuditEntry
}

func (m *mockAuditRepo) Create(entry *secrets.SecretAuditEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditRepo) ListByNamespace(_ uuid.UUID, _, _ int) ([]*secrets.SecretAuditEntry, error) {
	return m.entries, nil
}

func (m *mockAuditRepo) ListByAgent(_ uuid.UUID, _ *time.Time, _, _ int) ([]*secrets.SecretAuditEntry, error) {
	return m.entries, nil
}

type mockAgentRepo struct {
	agents map[uuid.UUID]*domain.Agent
}

func newMockAgentRepo() *mockAgentRepo {
	return &mockAgentRepo{agents: make(map[uuid.UUID]*domain.Agent)}
}

func (m *mockAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, nil
	}
	return agent, nil
}

// Implement remaining interface methods as no-ops
func (m *mockAgentRepo) Create(_ *domain.Agent) error                              { return nil }
func (m *mockAgentRepo) GetByName(_ uuid.UUID, _ string) (*domain.Agent, error)    { return nil, nil }
func (m *mockAgentRepo) GetByOrganization(_ uuid.UUID) ([]*domain.Agent, error)    { return nil, nil }
func (m *mockAgentRepo) Update(_ *domain.Agent) error                              { return nil }
func (m *mockAgentRepo) Delete(_ uuid.UUID) error                                  { return nil }
func (m *mockAgentRepo) List(_, _ int) ([]*domain.Agent, error)                    { return nil, nil }
func (m *mockAgentRepo) UpdateTrustScore(_ uuid.UUID, _ float64) error             { return nil }
func (m *mockAgentRepo) IncrementViolationCount(_ uuid.UUID) error                 { return nil }
func (m *mockAgentRepo) MarkAsCompromised(_ uuid.UUID) error                       { return nil }
func (m *mockAgentRepo) UpdateLastActive(_ context.Context, _ uuid.UUID) error       { return nil }
func (m *mockAgentRepo) GetStaleAgents(_ context.Context, _ time.Time) ([]*domain.Agent, error) { return nil, nil }
func (m *mockAgentRepo) GetByIDs(_ context.Context, _ []uuid.UUID) ([]*domain.Agent, error) { return nil, nil }

type mockCapabilityRepo struct {
	capabilities map[uuid.UUID][]*domain.AgentCapability
}

func newMockCapabilityRepo() *mockCapabilityRepo {
	return &mockCapabilityRepo{capabilities: make(map[uuid.UUID][]*domain.AgentCapability)}
}

func (m *mockCapabilityRepo) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	return m.capabilities[agentID], nil
}

// No-op implementations for remaining interface methods
func (m *mockCapabilityRepo) CreateCapability(_ *domain.AgentCapability) error                     { return nil }
func (m *mockCapabilityRepo) GetCapabilityByID(_ uuid.UUID) (*domain.AgentCapability, error)       { return nil, nil }
func (m *mockCapabilityRepo) GetCapabilitiesByAgentID(_ uuid.UUID) ([]*domain.AgentCapability, error) { return nil, nil }
func (m *mockCapabilityRepo) RevokeCapability(_ uuid.UUID, _ time.Time) error                      { return nil }
func (m *mockCapabilityRepo) DeleteCapability(_ uuid.UUID) error                                   { return nil }
func (m *mockCapabilityRepo) CreateViolation(_ *domain.CapabilityViolation) error                  { return nil }
func (m *mockCapabilityRepo) GetViolationByID(_ uuid.UUID) (*domain.CapabilityViolation, error)    { return nil, nil }
func (m *mockCapabilityRepo) GetViolationsByAgentID(_ uuid.UUID, _, _ int) ([]*domain.CapabilityViolation, int, error) { return nil, 0, nil }
func (m *mockCapabilityRepo) GetRecentViolations(_ uuid.UUID, _ int) ([]*domain.CapabilityViolation, error) { return nil, nil }
func (m *mockCapabilityRepo) GetViolationsByOrganization(_ uuid.UUID, _, _ int) ([]*domain.CapabilityViolation, int, error) { return nil, 0, nil }
func (m *mockCapabilityRepo) ListCapabilityDefinitions(_ *uuid.UUID) ([]*domain.CapabilityDefinition, error) { return nil, nil }
func (m *mockCapabilityRepo) GetCapabilityDefinition(_, _ string, _ *uuid.UUID) (*domain.CapabilityDefinition, error) { return nil, nil }
func (m *mockCapabilityRepo) CreateCapabilityDefinition(_ *domain.CapabilityDefinition) error                         { return nil }
func (m *mockCapabilityRepo) UpdateCapabilityDefinition(_ *domain.CapabilityDefinition) error                         { return nil }

type mockATCVerifier struct {
	shouldFail   bool
	shouldRevoke bool
}

func (m *mockATCVerifier) Verify(rawToken string) (*atcdomain.ATCClaims, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("ATC verification failed")
	}
	return &atcdomain.ATCClaims{
		AgentID:      uuid.New(),
		Issuer:       "aim-test",
		Capabilities: []string{"secrets:resolve"},
		ATCID:        "test-atc-id",
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}, nil
}

func (m *mockATCVerifier) IsRevoked(atcID string) (bool, error) {
	return m.shouldRevoke, nil
}

type mockBackend struct {
	creds map[uuid.UUID]*secrets.SecretCredential
}

func newMockBackend() *mockBackend {
	return &mockBackend{creds: make(map[uuid.UUID]*secrets.SecretCredential)}
}

func (m *mockBackend) Store(nsID uuid.UUID, blob []byte, alg string) error {
	m.creds[nsID] = &secrets.SecretCredential{
		ID:            uuid.New(),
		NamespaceID:   nsID,
		EncryptedBlob: blob,
		EncryptionAlg: alg,
		Version:       1,
	}
	return nil
}

func (m *mockBackend) Retrieve(nsID uuid.UUID) (*secrets.SecretCredential, error) {
	cred, ok := m.creds[nsID]
	if !ok {
		return nil, fmt.Errorf("no credential found")
	}
	return cred, nil
}

func (m *mockBackend) Rotate(nsID uuid.UUID, blob []byte, alg string) error {
	return m.Store(nsID, blob, alg)
}

func (m *mockBackend) Delete(nsID uuid.UUID) error {
	delete(m.creds, nsID)
	return nil
}

func (m *mockBackend) Type() secrets.BackendType {
	return secrets.BackendTypeAIMNative
}

// --- Test helpers ---

func setupTestService(t *testing.T) (*SecretsService, *mockNamespaceRepo, *mockAuditRepo, *mockAgentRepo, *mockCapabilityRepo, *mockATCVerifier, *mockBackend) {
	t.Helper()
	nsRepo := newMockNamespaceRepo()
	auditRepo := &mockAuditRepo{}
	agentRepo := newMockAgentRepo()
	capRepo := newMockCapabilityRepo()
	atcVerifier := &mockATCVerifier{}
	backend := newMockBackend()

	svc := NewSecretsService(
		nsRepo,
		auditRepo,
		agentRepo,
		capRepo,
		atcVerifier,
		map[secrets.BackendType]secrets.SecretsBackend{
			secrets.BackendTypeAIMNative: backend,
		},
	)

	return svc, nsRepo, auditRepo, agentRepo, capRepo, atcVerifier, backend
}

func createTestAgent(t *testing.T, agentRepo *mockAgentRepo) (uuid.UUID, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agentID := uuid.New()
	pubKeyB64 := base64.StdEncoding.EncodeToString(pub)
	agentRepo.agents[agentID] = &domain.Agent{
		ID:        agentID,
		PublicKey: &pubKeyB64,
	}
	return agentID, pub, priv
}

func signRequest(priv ed25519.PrivateKey, namespace, operation, nonce string) []byte {
	message := []byte(namespace + "|" + operation + "|" + nonce)
	return ed25519.Sign(priv, message)
}

// --- Tests ---

func TestResolve_Success(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, capRepo, _, backend := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	// Grant capability
	capRepo.capabilities[agentID] = []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: CapabilitySecretsResolve},
	}

	// Create namespace
	ns := &secrets.SecretNamespace{
		AgentID:     agentID,
		Namespace:   "github",
		BackendType: secrets.BackendTypeAIMNative,
		Operations:  []string{"read"},
		Status:      secrets.NamespaceStatusActive,
		CreatedBy:   uuid.New(),
	}
	nsRepo.Create(ns)

	// Store credential
	backend.Store(ns.ID, []byte("secret-github-token"), "AES-256-GCM")

	nonce := time.Now().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	sig := signRequest(priv, "github", "read", nonce)

	result, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace:      "github",
		Operation:      "read",
		Nonce:          nonce,
		Signature:      sig,
		AgentPublicKey: pub,
	})

	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if len(result.EncryptedBlob) == 0 {
		t.Error("EncryptedBlob should not be empty")
	}
	if len(result.EphemeralPubKey) == 0 {
		t.Error("EphemeralPubKey should not be empty")
	}

	// Verify audit entry was created
	if len(auditRepo.entries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditRepo.entries))
	}
	if auditRepo.entries[0].Result != secrets.AuditResultGranted {
		t.Errorf("Audit result = %s, want granted", auditRepo.entries[0].Result)
	}
}

func TestResolve_Step1_InvalidSignature(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, capRepo, _, _ := setupTestService(t)
	agentID, _, _ := createTestAgent(t, agentRepo)

	capRepo.capabilities[agentID] = []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: CapabilitySecretsResolve},
	}

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	nonce := time.Now().Format(time.RFC3339Nano) + ":" + uuid.New().String()

	_, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace:      "github",
		Operation:      "read",
		Nonce:          nonce,
		Signature:      []byte("bad-signature-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"),
		AgentPublicKey: make([]byte, 32),
	})

	if err == nil {
		t.Fatal("Resolve should fail with invalid signature")
	}

	// Verify denied audit entry
	if len(auditRepo.entries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditRepo.entries))
	}
	if auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Errorf("Audit result = %s, want denied", auditRepo.entries[0].Result)
	}
}

func TestResolve_Step1_ReplayedNonce(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, capRepo, _, _ := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	capRepo.capabilities[agentID] = []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: CapabilitySecretsResolve},
	}

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	// Use a nonce with old timestamp (>30s)
	oldTime := time.Now().Add(-60 * time.Second)
	nonce := oldTime.Format(time.RFC3339Nano) + ":" + uuid.New().String()
	sig := signRequest(priv, "github", "read", nonce)

	_, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace: "github", Operation: "read", Nonce: nonce,
		Signature: sig, AgentPublicKey: pub,
	})

	if err == nil {
		t.Fatal("Resolve should reject replayed nonce")
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Error("Expected denied audit entry for replayed nonce")
	}
}

func TestResolve_Step3_RevokedNamespace(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, capRepo, _, _ := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	capRepo.capabilities[agentID] = []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: CapabilitySecretsResolve},
	}

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusRevoked, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	nonce := time.Now().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	sig := signRequest(priv, "github", "read", nonce)

	_, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace: "github", Operation: "read", Nonce: nonce,
		Signature: sig, AgentPublicKey: pub,
	})

	if err == nil {
		t.Fatal("Resolve should reject revoked namespace")
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Error("Expected denied audit entry for revoked namespace")
	}
}

func TestResolve_Step3_UnauthorizedOperation(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, capRepo, _, _ := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	capRepo.capabilities[agentID] = []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: CapabilitySecretsResolve},
	}

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	nonce := time.Now().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	sig := signRequest(priv, "github", "write", nonce) // "write" not in allowed ops

	_, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace: "github", Operation: "write", Nonce: nonce,
		Signature: sig, AgentPublicKey: pub,
	})

	if err == nil {
		t.Fatal("Resolve should reject unauthorized operation")
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Error("Expected denied audit entry for unauthorized operation")
	}
}

func TestResolve_Step4_MissingCapability(t *testing.T) {
	svc, nsRepo, auditRepo, agentRepo, _, _, _ := setupTestService(t)
	agentID, pub, priv := createTestAgent(t, agentRepo)

	// No capabilities granted

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Operations: []string{"read"}, Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	nonce := time.Now().Format(time.RFC3339Nano) + ":" + uuid.New().String()
	sig := signRequest(priv, "github", "read", nonce)

	_, err := svc.Resolve(agentID, &secrets.ResolutionRequest{
		Namespace: "github", Operation: "read", Nonce: nonce,
		Signature: sig, AgentPublicKey: pub,
	})

	if err == nil {
		t.Fatal("Resolve should reject missing capability")
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Error("Expected denied audit entry for missing capability")
	}
}

func TestResolve_Step8_AuditAlwaysWritten(t *testing.T) {
	svc, _, auditRepo, agentRepo, _, _, _ := setupTestService(t)

	// Agent doesn't exist — should fail at step 1 but still write audit
	_, _ = svc.Resolve(uuid.New(), &secrets.ResolutionRequest{
		Namespace: "github", Operation: "read", Nonce: "test",
		Signature: []byte("x"), AgentPublicKey: []byte("x"),
	})

	// Even with agent not found, a denied audit entry should exist
	if len(auditRepo.entries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditRepo.entries))
	}
	if auditRepo.entries[0].Result != secrets.AuditResultDenied {
		t.Errorf("Audit result = %s, want denied", auditRepo.entries[0].Result)
	}

	_ = agentRepo // suppress unused
}

func TestCreateAndListNamespaces(t *testing.T) {
	svc, _, _, _, _, _, _ := setupTestService(t)
	agentID := uuid.New()

	err := svc.CreateNamespace(&secrets.SecretNamespace{
		AgentID:     agentID,
		Namespace:   "github",
		Operations:  []string{"read", "write"},
		URLPatterns: []string{"https://api.github.com/*"},
		CreatedBy:   uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}

	namespaces, err := svc.ListNamespaces(agentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(namespaces) != 1 {
		t.Fatalf("Expected 1 namespace, got %d", len(namespaces))
	}
	if namespaces[0].Status != secrets.NamespaceStatusActive {
		t.Errorf("Status = %s, want active", namespaces[0].Status)
	}
	if namespaces[0].BackendType != secrets.BackendTypeAIMNative {
		t.Errorf("BackendType = %s, want aim_native", namespaces[0].BackendType)
	}
}

func TestDeleteNamespace(t *testing.T) {
	svc, nsRepo, _, _, _, _, _ := setupTestService(t)
	agentID := uuid.New()

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	err := svc.DeleteNamespace(ns.ID)
	if err != nil {
		t.Fatal(err)
	}

	namespaces, _ := svc.ListNamespaces(agentID)
	if len(namespaces) != 0 {
		t.Errorf("Expected 0 namespaces after delete, got %d", len(namespaces))
	}
}

func TestStoreAndRotateCredential(t *testing.T) {
	svc, nsRepo, _, _, _, _, backend := setupTestService(t)
	agentID := uuid.New()

	ns := &secrets.SecretNamespace{
		AgentID: agentID, Namespace: "github", BackendType: secrets.BackendTypeAIMNative,
		Status: secrets.NamespaceStatusActive, CreatedBy: uuid.New(),
	}
	nsRepo.Create(ns)

	// Store
	err := svc.StoreCredential(ns.ID, []byte("cred-v1"), "AES-256-GCM")
	if err != nil {
		t.Fatal(err)
	}

	cred, _ := backend.Retrieve(ns.ID)
	if string(cred.EncryptedBlob) != "cred-v1" {
		t.Errorf("Stored cred = %s, want cred-v1", cred.EncryptedBlob)
	}

	// Rotate
	err = svc.RotateCredential(ns.ID, []byte("cred-v2"), "AES-256-GCM")
	if err != nil {
		t.Fatal(err)
	}

	cred, _ = backend.Retrieve(ns.ID)
	if string(cred.EncryptedBlob) != "cred-v2" {
		t.Errorf("Rotated cred = %s, want cred-v2", cred.EncryptedBlob)
	}
}

func TestZeroize(t *testing.T) {
	data := []byte("sensitive-data-here")
	zeroize(data)
	for i, b := range data {
		if b != 0 {
			t.Fatalf("Byte %d not zeroed: %d", i, b)
		}
	}
}

func TestEd25519PublicToX25519(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	x25519Pub, err := ed25519PublicToX25519(pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(x25519Pub) != 32 {
		t.Fatalf("X25519 key length = %d, want 32", len(x25519Pub))
	}

	// Should fail with wrong length
	_, err = ed25519PublicToX25519([]byte("short"))
	if err == nil {
		t.Fatal("Should fail with short key")
	}
}

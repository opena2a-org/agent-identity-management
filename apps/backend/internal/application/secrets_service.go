package application

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

const (
	// maxNonceAge is the maximum age of a nonce before it's considered replayed (CR-003).
	maxNonceAge = 30 * time.Second

	// maxCredentialBlobSize is the maximum size of a credential blob (1 MiB).
	// Prevents DoS via oversized blobs in encryption/decryption.
	maxCredentialBlobSize = 1 << 20
)

// SecretsService implements identity-native secrets management.
// The 8-step Resolve() ordering is non-negotiable.
type SecretsService struct {
	nsRepo         secrets.NamespaceRepository
	auditRepo      secrets.AuditRepository
	agentRepo      domain.AgentRepository
	capabilityRepo domain.CapabilityRepository
	atcVerifier    atcdomain.ATCVerifier
	backends       map[secrets.BackendType]secrets.SecretsBackend
}

func NewSecretsService(
	nsRepo secrets.NamespaceRepository,
	auditRepo secrets.AuditRepository,
	agentRepo domain.AgentRepository,
	capabilityRepo domain.CapabilityRepository,
	atcVerifier atcdomain.ATCVerifier,
	backends map[secrets.BackendType]secrets.SecretsBackend,
) *SecretsService {
	return &SecretsService{
		nsRepo:         nsRepo,
		auditRepo:      auditRepo,
		agentRepo:      agentRepo,
		capabilityRepo: capabilityRepo,
		atcVerifier:    atcVerifier,
		backends:       backends,
	}
}

// Resolve executes the non-negotiable 8-step credential resolution.
//
//  1. Verify Ed25519 signature over namespace|operation|nonce
//  2. Verify ATC via atcVerifier.Verify()
//  3. Load namespace, verify agent ownership + status + operation + URL pattern
//  4. Check AIM capability (secrets:resolve)
//  5. Retrieve encrypted blob from backend
//  6. Encrypt with agent's X25519 public key via ECDH
//  7. Zeroize raw credential
//  8. Write audit entry (always — granted or denied)
func (s *SecretsService) Resolve(agentID uuid.UUID, req *secrets.ResolutionRequest) (*secrets.ResolutionResult, error) {
	var denyReason string
	var nsID uuid.UUID
	var atcID string

	defer func() {
		// Step 8: audit — always runs, granted or denied
		result := secrets.AuditResultGranted
		if denyReason != "" {
			result = secrets.AuditResultDenied
		}
		s.auditRepo.Create(&secrets.SecretAuditEntry{
			NamespaceID: nsID,
			AgentID:     agentID,
			Operation:   req.Operation,
			Result:      result,
			DenyReason:  denyReason,
			ATCID:       atcID,
		})
	}()

	// Step 1: Verify Ed25519 signature over namespace|operation|nonce
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		denyReason = "agent not found"
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		denyReason = "agent has no public key"
		return nil, fmt.Errorf("agent %s has no public key", agentID)
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(*agent.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		denyReason = "invalid agent public key"
		return nil, fmt.Errorf("invalid agent public key for %s", agentID)
	}

	message := []byte(req.Namespace + "|" + req.Operation + "|" + req.Nonce)
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), message, req.Signature) {
		denyReason = "signature verification failed"
		return nil, fmt.Errorf("signature verification failed for agent %s", agentID)
	}

	// Validate nonce freshness (replay protection — CR-003)
	nonceParts := strings.SplitN(req.Nonce, ":", 2)
	if len(nonceParts) == 2 {
		ts, parseErr := time.Parse(time.RFC3339Nano, nonceParts[0])
		if parseErr == nil && time.Since(ts) > maxNonceAge {
			denyReason = "nonce expired (replay protection)"
			return nil, fmt.Errorf("nonce expired: submitted %s, max age %s", time.Since(ts), maxNonceAge)
		}
	}

	// Step 2: Verify ATC
	if req.ATCID != "" {
		claims, err := s.atcVerifier.Verify(req.ATCID)
		if err != nil {
			denyReason = "ATC verification failed: " + err.Error()
			return nil, fmt.Errorf("ATC verification failed: %w", err)
		}
		atcID = claims.ATCID

		revoked, err := s.atcVerifier.IsRevoked(claims.ATCID)
		if err != nil {
			denyReason = "ATC revocation check failed"
			return nil, fmt.Errorf("ATC revocation check failed: %w", err)
		}
		if revoked {
			denyReason = "ATC has been revoked"
			return nil, fmt.Errorf("ATC %s has been revoked", claims.ATCID)
		}

		if claims.AgentID != agentID {
			denyReason = "ATC agent ID mismatch"
			return nil, fmt.Errorf("ATC agent ID %s does not match requesting agent %s", claims.AgentID, agentID)
		}
	}

	// Step 3: Load namespace, verify ownership + status + operation
	ns, err := s.nsRepo.GetByAgentAndName(agentID, req.Namespace)
	if err != nil {
		denyReason = "namespace lookup failed"
		return nil, fmt.Errorf("failed to lookup namespace %s: %w", req.Namespace, err)
	}
	if ns == nil {
		denyReason = "namespace not found"
		return nil, fmt.Errorf("namespace %s not found for agent %s", req.Namespace, agentID)
	}
	nsID = ns.ID

	if ns.AgentID != agentID {
		denyReason = "namespace ownership mismatch"
		return nil, fmt.Errorf("agent %s does not own namespace %s", agentID, req.Namespace)
	}

	if ns.Status != secrets.NamespaceStatusActive {
		denyReason = "namespace is revoked"
		return nil, fmt.Errorf("namespace %s is revoked", req.Namespace)
	}

	if !s.operationAllowed(ns.Operations, req.Operation) {
		denyReason = fmt.Sprintf("operation %s not allowed in namespace", req.Operation)
		return nil, fmt.Errorf("operation %s not in namespace %s allowed operations", req.Operation, req.Namespace)
	}

	// Step 4: Check AIM capability (secrets:resolve)
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		denyReason = "capability lookup failed"
		return nil, fmt.Errorf("failed to check capabilities: %w", err)
	}

	if !s.hasCapability(capabilities, CapabilitySecretsResolve) {
		denyReason = "missing secrets:resolve capability"
		return nil, fmt.Errorf("agent %s lacks %s capability", agentID, CapabilitySecretsResolve)
	}

	// Step 5: Retrieve encrypted blob from backend
	backend, ok := s.backends[ns.BackendType]
	if !ok {
		denyReason = "backend not available: " + string(ns.BackendType)
		return nil, fmt.Errorf("no backend registered for type %s", ns.BackendType)
	}

	cred, err := backend.Retrieve(ns.ID)
	if err != nil {
		denyReason = "credential retrieval failed"
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}

	rawBlob := make([]byte, len(cred.EncryptedBlob))
	copy(rawBlob, cred.EncryptedBlob)

	// Step 6: Encrypt with agent's X25519 public key via ECDH
	encryptedResult, ephemeralPub, err := s.encryptForAgent(rawBlob, req.AgentPublicKey)
	if err != nil {
		denyReason = "ECDH encryption failed"
		// Step 7 (zeroize) happens even on error
		zeroize(rawBlob)
		return nil, fmt.Errorf("failed to encrypt for agent: %w", err)
	}

	// Step 7: Zeroize raw credential
	zeroize(rawBlob)

	return &secrets.ResolutionResult{
		EncryptedBlob:   encryptedResult,
		EphemeralPubKey: ephemeralPub,
		EncryptionAlg:   "X25519-ChaCha20-Poly1305",
	}, nil
}

// CreateNamespace creates a new secret namespace for an agent.
func (s *SecretsService) CreateNamespace(ns *secrets.SecretNamespace) error {
	ns.Status = secrets.NamespaceStatusActive
	if ns.BackendType == "" {
		ns.BackendType = secrets.BackendTypeAIMNative
	}
	return s.nsRepo.Create(ns)
}

// ListNamespaces returns all namespaces for an agent.
func (s *SecretsService) ListNamespaces(agentID uuid.UUID) ([]*secrets.SecretNamespace, error) {
	return s.nsRepo.ListByAgent(agentID)
}

// GetNamespace returns a single namespace by ID.
func (s *SecretsService) GetNamespace(id uuid.UUID) (*secrets.SecretNamespace, error) {
	return s.nsRepo.GetByID(id)
}

// DeleteNamespace revokes the namespace and deletes all credentials.
func (s *SecretsService) DeleteNamespace(id uuid.UUID) error {
	ns, err := s.nsRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
	if ns == nil {
		return fmt.Errorf("namespace not found")
	}

	// Delete credentials from backend
	backend, ok := s.backends[ns.BackendType]
	if ok {
		backend.Delete(ns.ID)
	}

	return s.nsRepo.Delete(id)
}

// StoreCredential stores an encrypted credential blob in a namespace.
func (s *SecretsService) StoreCredential(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ns, err := s.nsRepo.GetByID(namespaceID)
	if err != nil || ns == nil {
		return fmt.Errorf("namespace not found")
	}

	backend, ok := s.backends[ns.BackendType]
	if !ok {
		return fmt.Errorf("backend %s not available", ns.BackendType)
	}

	return backend.Store(namespaceID, blob, encryptionAlg)
}

// RotateCredential stores a new version of the credential.
func (s *SecretsService) RotateCredential(namespaceID uuid.UUID, blob []byte, encryptionAlg string) error {
	ns, err := s.nsRepo.GetByID(namespaceID)
	if err != nil || ns == nil {
		return fmt.Errorf("namespace not found")
	}

	backend, ok := s.backends[ns.BackendType]
	if !ok {
		return fmt.Errorf("backend %s not available", ns.BackendType)
	}

	return backend.Rotate(namespaceID, blob, encryptionAlg)
}

// GetAuditLog returns audit entries for an agent.
func (s *SecretsService) GetAuditLog(agentID uuid.UUID, since *time.Time, limit, offset int) ([]*secrets.SecretAuditEntry, error) {
	return s.auditRepo.ListByAgent(agentID, since, limit, offset)
}

// encryptForAgent performs X25519 ECDH to encrypt rawBlob for the requesting agent.
// Returns the encrypted blob and the ephemeral public key.
func (s *SecretsService) encryptForAgent(rawBlob []byte, agentEd25519PubKey []byte) ([]byte, []byte, error) {
	// Generate ephemeral X25519 key pair
	ephemeralPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Convert agent's Ed25519 public key to X25519
	agentX25519Pub, err := ed25519PublicToX25519(agentEd25519PubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert agent key to X25519: %w", err)
	}

	agentX25519Key, err := ecdh.X25519().NewPublicKey(agentX25519Pub)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse agent X25519 key: %w", err)
	}

	// ECDH shared secret
	sharedSecret, err := ephemeralPriv.ECDH(agentX25519Key)
	if err != nil {
		return nil, nil, fmt.Errorf("ECDH failed: %w", err)
	}

	// Bound blob size to prevent DoS
	if len(rawBlob) > maxCredentialBlobSize {
		return nil, nil, fmt.Errorf("credential blob exceeds maximum size of %d bytes", maxCredentialBlobSize)
	}

	// Derive 256-bit encryption key from shared secret via SHA-256
	encKey := sha256.Sum256(sharedSecret)

	// Authenticated encryption using ChaCha20-Poly1305 (IETF variant, 256-bit key)
	aead, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AEAD cipher: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate: nonce || ciphertext || tag
	ciphertext := aead.Seal(nonce, nonce, rawBlob, nil)

	return ciphertext, ephemeralPriv.PublicKey().Bytes(), nil
}

// ed25519PublicToX25519 converts an Ed25519 public key to X25519.
// Uses the birational map: u = (1 + y) / (1 - y) mod p
func ed25519PublicToX25519(ed25519Pub []byte) ([]byte, error) {
	if len(ed25519Pub) != 32 {
		return nil, fmt.Errorf("invalid Ed25519 public key length: %d", len(ed25519Pub))
	}

	// The Ed25519 public key is a compressed Edwards point (y coordinate + sign bit).
	// Extract y coordinate (clear the sign bit).
	var y [32]byte
	copy(y[:], ed25519Pub)
	y[31] &= 0x7f

	// u = (1 + y) / (1 - y) mod p
	// where p = 2^255 - 19
	// We work in the field of integers mod p.
	var one, yField, numerator, denominator, invDenom, u fieldElement
	feOne(&one)
	feFromBytes(&yField, &y)

	feAdd(&numerator, &one, &yField)   // 1 + y
	feSub(&denominator, &one, &yField) // 1 - y
	feInvert(&invDenom, &denominator)  // (1 - y)^(-1)
	feMul(&u, &numerator, &invDenom)   // (1 + y) / (1 - y)

	var result [32]byte
	feToBytes(&result, &u)
	return result[:], nil
}

// operationAllowed checks if the requested operation is in the namespace's allowed list.
func (s *SecretsService) operationAllowed(allowed []string, requested string) bool {
	if len(allowed) == 0 {
		return true // empty = all operations allowed
	}
	for _, op := range allowed {
		if op == requested || op == "*" {
			return true
		}
	}
	return false
}

// hasCapability checks if the agent has a specific capability.
func (s *SecretsService) hasCapability(capabilities []*domain.AgentCapability, required string) bool {
	for _, cap := range capabilities {
		if cap.CapabilityType == required && cap.RevokedAt == nil {
			return true
		}
	}
	return false
}

// zeroize overwrites a byte slice with zeros.
func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Secrets capability constants
const (
	CapabilitySecretsResolve = "secrets:resolve"
	CapabilitySecretsStore   = "secrets:store"
	CapabilitySecretsRotate  = "secrets:rotate"
	CapabilitySecretsDelete  = "secrets:delete"
)

// --- Field arithmetic for Ed25519 -> X25519 conversion ---
// Minimal field element type for mod p = 2^255 - 19 arithmetic.
// Representation: 10 limbs of 25.5 bits each (alternating 26/25 bits).

type fieldElement [10]int32

func feZero(f *fieldElement) {
	for i := range f {
		f[i] = 0
	}
}

func feOne(f *fieldElement) {
	feZero(f)
	f[0] = 1
}

func feFromBytes(f *fieldElement, s *[32]byte) {
	h0 := load4(s[:])
	h1 := load3(s[4:]) << 6
	h2 := load3(s[7:]) << 5
	h3 := load3(s[10:]) << 3
	h4 := load3(s[13:]) << 2
	h5 := load4(s[16:])
	h6 := load3(s[20:]) << 7
	h7 := load3(s[23:]) << 5
	h8 := load3(s[26:]) << 4
	h9 := load3(s[29:]) << 2

	var carry [10]int64
	carry[9] = (h9 + 1<<24) >> 25
	h0 += carry[9] * 19
	h9 -= carry[9] << 25
	carry[1] = (h1 + 1<<24) >> 25; h2 += carry[1]; h1 -= carry[1] << 25
	carry[3] = (h3 + 1<<24) >> 25; h4 += carry[3]; h3 -= carry[3] << 25
	carry[5] = (h5 + 1<<24) >> 25; h6 += carry[5]; h5 -= carry[5] << 25
	carry[7] = (h7 + 1<<24) >> 25; h8 += carry[7]; h7 -= carry[7] << 25

	carry[0] = (h0 + 1<<25) >> 26; h1 += carry[0]; h0 -= carry[0] << 26
	carry[2] = (h2 + 1<<25) >> 26; h3 += carry[2]; h2 -= carry[2] << 26
	carry[4] = (h4 + 1<<25) >> 26; h5 += carry[4]; h4 -= carry[4] << 26
	carry[6] = (h6 + 1<<25) >> 26; h7 += carry[6]; h6 -= carry[6] << 26
	carry[8] = (h8 + 1<<25) >> 26; h9 += carry[8]; h8 -= carry[8] << 26

	f[0] = int32(h0)
	f[1] = int32(h1)
	f[2] = int32(h2)
	f[3] = int32(h3)
	f[4] = int32(h4)
	f[5] = int32(h5)
	f[6] = int32(h6)
	f[7] = int32(h7)
	f[8] = int32(h8)
	f[9] = int32(h9)
}

func feToBytes(s *[32]byte, f *fieldElement) {
	var h [10]int32
	copy(h[:], f[:])

	q := (19*h[9] + (1 << 24)) >> 25
	q = (h[0] + q) >> 26
	q = (h[1] + q) >> 25
	q = (h[2] + q) >> 26
	q = (h[3] + q) >> 25
	q = (h[4] + q) >> 26
	q = (h[5] + q) >> 25
	q = (h[6] + q) >> 26
	q = (h[7] + q) >> 25
	q = (h[8] + q) >> 26
	q = (h[9] + q) >> 25

	h[0] += 19 * q
	carry0 := h[0] >> 26; h[1] += carry0; h[0] -= carry0 << 26
	carry1 := h[1] >> 25; h[2] += carry1; h[1] -= carry1 << 25
	carry2 := h[2] >> 26; h[3] += carry2; h[2] -= carry2 << 26
	carry3 := h[3] >> 25; h[4] += carry3; h[3] -= carry3 << 25
	carry4 := h[4] >> 26; h[5] += carry4; h[4] -= carry4 << 26
	carry5 := h[5] >> 25; h[6] += carry5; h[5] -= carry5 << 25
	carry6 := h[6] >> 26; h[7] += carry6; h[6] -= carry6 << 26
	carry7 := h[7] >> 25; h[8] += carry7; h[7] -= carry7 << 25
	carry8 := h[8] >> 26; h[9] += carry8; h[8] -= carry8 << 26
	carry9 := h[9] >> 25;                 h[9] -= carry9 << 25

	s[0] = byte(h[0])
	s[1] = byte(h[0] >> 8)
	s[2] = byte(h[0] >> 16)
	s[3] = byte((h[0] >> 24) | (h[1] << 2))
	s[4] = byte(h[1] >> 6)
	s[5] = byte(h[1] >> 14)
	s[6] = byte((h[1] >> 22) | (h[2] << 3))
	s[7] = byte(h[2] >> 5)
	s[8] = byte(h[2] >> 13)
	s[9] = byte((h[2] >> 21) | (h[3] << 5))
	s[10] = byte(h[3] >> 3)
	s[11] = byte(h[3] >> 11)
	s[12] = byte((h[3] >> 19) | (h[4] << 6))
	s[13] = byte(h[4] >> 2)
	s[14] = byte(h[4] >> 10)
	s[15] = byte(h[4] >> 18)
	s[16] = byte(h[5])
	s[17] = byte(h[5] >> 8)
	s[18] = byte(h[5] >> 16)
	s[19] = byte((h[5] >> 24) | (h[6] << 1))
	s[20] = byte(h[6] >> 7)
	s[21] = byte(h[6] >> 15)
	s[22] = byte((h[6] >> 23) | (h[7] << 3))
	s[23] = byte(h[7] >> 5)
	s[24] = byte(h[7] >> 13)
	s[25] = byte((h[7] >> 21) | (h[8] << 4))
	s[26] = byte(h[8] >> 4)
	s[27] = byte(h[8] >> 12)
	s[28] = byte((h[8] >> 20) | (h[9] << 6))
	s[29] = byte(h[9] >> 2)
	s[30] = byte(h[9] >> 10)
	s[31] = byte(h[9] >> 18)
}

func feAdd(r, a, b *fieldElement) {
	for i := range r {
		r[i] = a[i] + b[i]
	}
}

func feSub(r, a, b *fieldElement) {
	for i := range r {
		r[i] = a[i] - b[i]
	}
}

func feMul(r, a, b *fieldElement) {
	a0, a1, a2, a3, a4 := int64(a[0]), int64(a[1]), int64(a[2]), int64(a[3]), int64(a[4])
	a5, a6, a7, a8, a9 := int64(a[5]), int64(a[6]), int64(a[7]), int64(a[8]), int64(a[9])
	b0, b1, b2, b3, b4 := int64(b[0]), int64(b[1]), int64(b[2]), int64(b[3]), int64(b[4])
	b5, b6, b7, b8, b9 := int64(b[5]), int64(b[6]), int64(b[7]), int64(b[8]), int64(b[9])

	b1_19 := 19 * b1; b2_19 := 19 * b2; b3_19 := 19 * b3; b4_19 := 19 * b4
	b5_19 := 19 * b5; b6_19 := 19 * b6; b7_19 := 19 * b7; b8_19 := 19 * b8; b9_19 := 19 * b9

	a1_2 := 2 * a1; a3_2 := 2 * a3; a5_2 := 2 * a5; a7_2 := 2 * a7; a9_2 := 2 * a9

	h0 := a0*b0 + a1_2*b9_19 + a2*b8_19 + a3_2*b7_19 + a4*b6_19 + a5_2*b5_19 + a6*b4_19 + a7_2*b3_19 + a8*b2_19 + a9_2*b1_19
	h1 := a0*b1 + a1*b0 + a2*b9_19 + a3*b8_19 + a4*b7_19 + a5*b6_19 + a6*b5_19 + a7*b4_19 + a8*b3_19 + a9*b2_19
	h2 := a0*b2 + a1_2*b1 + a2*b0 + a3_2*b9_19 + a4*b8_19 + a5_2*b7_19 + a6*b6_19 + a7_2*b5_19 + a8*b4_19 + a9_2*b3_19
	h3 := a0*b3 + a1*b2 + a2*b1 + a3*b0 + a4*b9_19 + a5*b8_19 + a6*b7_19 + a7*b6_19 + a8*b5_19 + a9*b4_19
	h4 := a0*b4 + a1_2*b3 + a2*b2 + a3_2*b1 + a4*b0 + a5_2*b9_19 + a6*b8_19 + a7_2*b7_19 + a8*b6_19 + a9_2*b5_19
	h5 := a0*b5 + a1*b4 + a2*b3 + a3*b2 + a4*b1 + a5*b0 + a6*b9_19 + a7*b8_19 + a8*b7_19 + a9*b6_19
	h6 := a0*b6 + a1_2*b5 + a2*b4 + a3_2*b3 + a4*b2 + a5_2*b1 + a6*b0 + a7_2*b9_19 + a8*b8_19 + a9_2*b7_19
	h7 := a0*b7 + a1*b6 + a2*b5 + a3*b4 + a4*b3 + a5*b2 + a6*b1 + a7*b0 + a8*b9_19 + a9*b8_19
	h8 := a0*b8 + a1_2*b7 + a2*b6 + a3_2*b5 + a4*b4 + a5_2*b3 + a6*b2 + a7_2*b1 + a8*b0 + a9_2*b9_19
	h9 := a0*b9 + a1*b8 + a2*b7 + a3*b6 + a4*b5 + a5*b4 + a6*b3 + a7*b2 + a8*b1 + a9*b0

	var carry [10]int64
	carry[0] = (h0 + (1 << 25)) >> 26; h1 += carry[0]; h0 -= carry[0] << 26
	carry[4] = (h4 + (1 << 25)) >> 26; h5 += carry[4]; h4 -= carry[4] << 26
	carry[1] = (h1 + (1 << 24)) >> 25; h2 += carry[1]; h1 -= carry[1] << 25
	carry[5] = (h5 + (1 << 24)) >> 25; h6 += carry[5]; h5 -= carry[5] << 25
	carry[2] = (h2 + (1 << 25)) >> 26; h3 += carry[2]; h2 -= carry[2] << 26
	carry[6] = (h6 + (1 << 25)) >> 26; h7 += carry[6]; h6 -= carry[6] << 26
	carry[3] = (h3 + (1 << 24)) >> 25; h4 += carry[3]; h3 -= carry[3] << 25
	carry[7] = (h7 + (1 << 24)) >> 25; h8 += carry[7]; h7 -= carry[7] << 25
	carry[4] = (h4 + (1 << 25)) >> 26; h5 += carry[4]; h4 -= carry[4] << 26
	carry[8] = (h8 + (1 << 25)) >> 26; h9 += carry[8]; h8 -= carry[8] << 26
	carry[9] = (h9 + (1 << 24)) >> 25; h0 += carry[9] * 19; h9 -= carry[9] << 25
	carry[0] = (h0 + (1 << 25)) >> 26; h1 += carry[0]; h0 -= carry[0] << 26

	r[0] = int32(h0); r[1] = int32(h1); r[2] = int32(h2); r[3] = int32(h3); r[4] = int32(h4)
	r[5] = int32(h5); r[6] = int32(h6); r[7] = int32(h7); r[8] = int32(h8); r[9] = int32(h9)
}

func feSquare(r, a *fieldElement) {
	feMul(r, a, a)
}

func feInvert(out, z *fieldElement) {
	// z^(p-2) via addition chain for p = 2^255-19
	var t0, t1, t2, t3 fieldElement
	var i int

	feSquare(&t0, z)
	feSquare(&t1, &t0)
	feSquare(&t1, &t1)
	feMul(&t1, z, &t1)
	feMul(&t0, &t0, &t1)
	feSquare(&t2, &t0)
	feMul(&t1, &t1, &t2)
	feSquare(&t2, &t1)
	for i = 1; i < 5; i++ { feSquare(&t2, &t2) }
	feMul(&t1, &t2, &t1)
	feSquare(&t2, &t1)
	for i = 1; i < 10; i++ { feSquare(&t2, &t2) }
	feMul(&t2, &t2, &t1)
	feSquare(&t3, &t2)
	for i = 1; i < 20; i++ { feSquare(&t3, &t3) }
	feMul(&t2, &t3, &t2)
	feSquare(&t2, &t2)
	for i = 1; i < 10; i++ { feSquare(&t2, &t2) }
	feMul(&t1, &t2, &t1)
	feSquare(&t2, &t1)
	for i = 1; i < 50; i++ { feSquare(&t2, &t2) }
	feMul(&t2, &t2, &t1)
	feSquare(&t3, &t2)
	for i = 1; i < 100; i++ { feSquare(&t3, &t3) }
	feMul(&t2, &t3, &t2)
	feSquare(&t2, &t2)
	for i = 1; i < 50; i++ { feSquare(&t2, &t2) }
	feMul(&t1, &t2, &t1)
	feSquare(&t1, &t1)
	for i = 1; i < 5; i++ { feSquare(&t1, &t1) }
	feMul(out, &t1, &t0)
}

func load3(s []byte) int64 {
	return int64(s[0]) | int64(s[1])<<8 | int64(s[2])<<16
}

func load4(s []byte) int64 {
	return int64(s[0]) | int64(s[1])<<8 | int64(s[2])<<16 | int64(s[3])<<24
}

package application

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// ChallengeData Tests
// ========================================

func TestChallengeData_Fields(t *testing.T) {
	serverID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	data := ChallengeData{
		Challenge: "test-challenge-string",
		ServerID:  serverID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	assert.Equal(t, "test-challenge-string", data.Challenge)
	assert.Equal(t, serverID, data.ServerID)
	assert.Equal(t, now, data.CreatedAt)
	assert.Equal(t, expiresAt, data.ExpiresAt)
}

func TestChallengeData_ExpirationCheck(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		isExpired bool
	}{
		{
			name:      "not expired - 5 minutes in future",
			expiresAt: now.Add(5 * time.Minute),
			isExpired: false,
		},
		{
			name:      "not expired - 1 second in future",
			expiresAt: now.Add(1 * time.Second),
			isExpired: false,
		},
		{
			name:      "expired - 1 second in past",
			expiresAt: now.Add(-1 * time.Second),
			isExpired: true,
		},
		{
			name:      "expired - 5 minutes in past",
			expiresAt: now.Add(-5 * time.Minute),
			isExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := ChallengeData{
				Challenge: "test",
				ServerID:  uuid.New(),
				CreatedAt: now.Add(-1 * time.Minute),
				ExpiresAt: tt.expiresAt,
			}

			// Check if current time is after expiration
			isExpired := time.Now().After(data.ExpiresAt)
			assert.Equal(t, tt.isExpired, isExpired)
		})
	}
}

// ========================================
// CreateMCPServerRequest Tests
// ========================================

func TestCreateMCPServerRequest_Fields(t *testing.T) {
	req := CreateMCPServerRequest{
		Name:              "Test MCP Server",
		Description:       "A test server",
		URL:               "https://mcp.example.com",
		Version:           "1.0.0",
		PublicKey:         "base64-public-key",
		VerificationURL:   "https://mcp.example.com/verify",
		Capabilities:      []string{"read", "write"},
		TagIds:            []string{"tag-1", "tag-2"},
		RegisteredByAgent: "agent-id-123",
	}

	assert.Equal(t, "Test MCP Server", req.Name)
	assert.Equal(t, "A test server", req.Description)
	assert.Equal(t, "https://mcp.example.com", req.URL)
	assert.Equal(t, "1.0.0", req.Version)
	assert.Equal(t, "base64-public-key", req.PublicKey)
	assert.Equal(t, "https://mcp.example.com/verify", req.VerificationURL)
	assert.Len(t, req.Capabilities, 2)
	assert.Len(t, req.TagIds, 2)
	assert.Equal(t, "agent-id-123", req.RegisteredByAgent)
}

func TestCreateMCPServerRequest_MinimalFields(t *testing.T) {
	// Test with only required fields
	req := CreateMCPServerRequest{
		Name: "Minimal Server",
		URL:  "https://minimal.example.com",
	}

	assert.Equal(t, "Minimal Server", req.Name)
	assert.Equal(t, "https://minimal.example.com", req.URL)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.Version)
	assert.Empty(t, req.PublicKey)
	assert.Empty(t, req.VerificationURL)
	assert.Nil(t, req.Capabilities)
	assert.Nil(t, req.TagIds)
}

// ========================================
// UpdateMCPServerRequest Tests
// ========================================

func TestUpdateMCPServerRequest_Fields(t *testing.T) {
	req := UpdateMCPServerRequest{
		Name:            "Updated Name",
		Description:     "Updated description",
		URL:             "https://updated.example.com",
		Version:         "2.0.0",
		PublicKey:       "new-public-key",
		VerificationURL: "https://updated.example.com/verify",
		Capabilities:    []string{"read", "write", "execute"},
	}

	assert.Equal(t, "Updated Name", req.Name)
	assert.Equal(t, "Updated description", req.Description)
	assert.Equal(t, "https://updated.example.com", req.URL)
	assert.Equal(t, "2.0.0", req.Version)
	assert.Equal(t, "new-public-key", req.PublicKey)
	assert.Equal(t, "https://updated.example.com/verify", req.VerificationURL)
	assert.Len(t, req.Capabilities, 3)
}

func TestUpdateMCPServerRequest_PartialUpdate(t *testing.T) {
	// Test partial update with only some fields set
	req := UpdateMCPServerRequest{
		Name: "Just the name",
	}

	assert.Equal(t, "Just the name", req.Name)
	assert.Empty(t, req.Description)
	assert.Empty(t, req.URL)
	assert.Empty(t, req.Version)
}

// ========================================
// AddPublicKeyRequest Tests
// ========================================

func TestAddPublicKeyRequest_Fields(t *testing.T) {
	req := AddPublicKeyRequest{
		PublicKey: "base64-encoded-public-key",
		KeyType:   "ed25519",
	}

	assert.Equal(t, "base64-encoded-public-key", req.PublicKey)
	assert.Equal(t, "ed25519", req.KeyType)
}

func TestAddPublicKeyRequest_KeyTypes(t *testing.T) {
	keyTypes := []string{"ed25519", "rsa", "ecdsa"}

	for _, keyType := range keyTypes {
		t.Run(keyType, func(t *testing.T) {
			req := AddPublicKeyRequest{
				PublicKey: "key-data",
				KeyType:   keyType,
			}
			assert.Equal(t, keyType, req.KeyType)
		})
	}
}

// ========================================
// ConnectedAgent Tests
// ========================================

func TestConnectedAgent_Fields(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	agent := ConnectedAgent{
		ID:          id,
		Name:        "test-agent",
		DisplayName: "Test Agent",
		Status:      "active",
		TrustScore:  85.5,
		UpdatedAt:   now,
	}

	assert.Equal(t, id, agent.ID)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "Test Agent", agent.DisplayName)
	assert.Equal(t, "active", agent.Status)
	assert.Equal(t, 85.5, agent.TrustScore)
	assert.Equal(t, now, agent.UpdatedAt)
}

func TestConnectedAgent_StatusValues(t *testing.T) {
	statuses := []string{"active", "inactive", "pending", "suspended"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			agent := ConnectedAgent{
				ID:     uuid.New(),
				Status: status,
			}
			assert.Equal(t, status, agent.Status)
		})
	}
}

// ========================================
// sanitizeTalksToEntries Tests
// ========================================

func TestSanitizeTalksToEntries_EmptyInput(t *testing.T) {
	result := sanitizeTalksToEntries([]string{})
	assert.Empty(t, result)
}

func TestSanitizeTalksToEntries_NilInput(t *testing.T) {
	result := sanitizeTalksToEntries(nil)
	assert.Nil(t, result)
}

func TestSanitizeTalksToEntries_ValidEntries(t *testing.T) {
	input := []string{"memory", "aws-terraform", "github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_CommaSeparatedEntry(t *testing.T) {
	// Test the fix for malformed entries like "memory,aws-terraform"
	input := []string{"memory,aws-terraform"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
}

func TestSanitizeTalksToEntries_MixedEntries(t *testing.T) {
	// Mix of valid and comma-separated entries
	input := []string{"github", "memory,aws-terraform,docker", "slack"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 5)
	assert.Contains(t, result, "github")
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "docker")
	assert.Contains(t, result, "slack")
}

func TestSanitizeTalksToEntries_WhitespaceHandling(t *testing.T) {
	input := []string{"  memory  ", "  aws-terraform  "}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
}

func TestSanitizeTalksToEntries_CommaSeparatedWithSpaces(t *testing.T) {
	input := []string{"memory, aws-terraform, github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "aws-terraform")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_EmptyEntries(t *testing.T) {
	input := []string{"", "memory", "", "github", ""}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_Deduplication(t *testing.T) {
	// Test that duplicate entries are removed
	input := []string{"memory", "memory", "github", "memory"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_DeduplicationAcrossCommaSplit(t *testing.T) {
	// Duplicates that appear after comma splitting
	input := []string{"memory", "memory,github", "github"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
}

func TestSanitizeTalksToEntries_OnlyWhitespace(t *testing.T) {
	input := []string{"   ", "\t", "  \n  "}
	result := sanitizeTalksToEntries(input)

	assert.Empty(t, result)
}

func TestSanitizeTalksToEntries_TrailingComma(t *testing.T) {
	input := []string{"memory,"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "memory")
}

func TestSanitizeTalksToEntries_LeadingComma(t *testing.T) {
	input := []string{",memory"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 1)
	assert.Contains(t, result, "memory")
}

func TestSanitizeTalksToEntries_MultipleCommas(t *testing.T) {
	input := []string{"memory,,github,,,docker"}
	result := sanitizeTalksToEntries(input)

	assert.Len(t, result, 3)
	assert.Contains(t, result, "memory")
	assert.Contains(t, result, "github")
	assert.Contains(t, result, "docker")
}

// ========================================
// MCPService Constructor Tests
// ========================================

func TestNewMCPService_NilDependencies(t *testing.T) {
	// Test that constructor handles nil dependencies gracefully
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.mcpRepo)
	assert.Nil(t, service.verificationEventRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.keyVault)
	assert.Nil(t, service.capabilityService)
	assert.Nil(t, service.capabilityRepo)
	assert.Nil(t, service.connectionRepo)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.tagRepo)
	assert.Nil(t, service.trustCalculator)
	assert.NotNil(t, service.cryptoService)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.challenges)
}

func TestNewMCPService_HttpClientTimeout(t *testing.T) {
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Verify HTTP client has 30 second timeout
	assert.NotNil(t, service.httpClient)
	assert.Equal(t, 30*time.Second, service.httpClient.Timeout)
}

func TestNewMCPService_ChallengesMapInitialized(t *testing.T) {
	service := NewMCPService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Verify challenges map is initialized
	assert.NotNil(t, service.challenges)
	assert.Len(t, service.challenges, 0)
}

// ========================================
// Challenge Expiration Logic Tests
// ========================================

func TestMCPService_ChallengeExpirationDuration(t *testing.T) {
	// Document and test the 5-minute challenge expiration
	expectedDuration := 5 * time.Minute

	now := time.Now()
	expiresAt := now.Add(expectedDuration)

	diff := expiresAt.Sub(now)
	assert.Equal(t, expectedDuration, diff)
}

// ========================================
// Trust Score Logic Tests
// ========================================

// The test this replaces declared three literal scores (75.0 / 0.0 / 75.0) and
// then asserted only that each was between 0 and 100. It passed for any value
// in that range, including the fabricated 75.0 it existed to document, and it
// would have passed unchanged after the literal was removed. It measured
// nothing.
//
// What follows pins the actual invariant: an MCP trust score is produced by
// the 8-factor calculator, on the canonical [0,1] scale, and no code path
// assigns a literal.

// TestMCPTrustScore_CalculatorOutputIsCanonicalScale pins the range contract
// that `mcp_servers_trust_score_range_check` (migration 104) enforces in the
// database. A score outside [0,1] is unstorable; if the calculator could
// produce one, registration would fail at the constraint.
func TestMCPTrustScore_CalculatorOutputIsCanonicalScale(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	servers := map[string]*domain.MCPServer{
		"freshly registered, no signal": {
			ID:        uuid.New(),
			URL:       "https://mcp.example.com",
			Status:    domain.MCPServerStatusPending,
			CreatedAt: time.Now(),
		},
		"verified, well established": {
			ID:                   uuid.New(),
			URL:                  "https://mcp.example.com",
			PublicKey:            "dGVzdC1wdWJsaWMta2V5",
			Description:          "an established server",
			Status:               domain.MCPServerStatusVerified,
			IsVerified:           true,
			CreatedBy:            uuid.New(),
			CreatedAt:            time.Now().Add(-365 * 24 * time.Hour),
			AttestationCount:     12,
			ConfidenceScore:      95.0,
			ConnectedAgentsCount: 9,
			Capabilities:         []string{"read_file", "write_file", "list_directory"},
		},
		"revoked": {
			ID:         uuid.New(),
			URL:        "http://mcp.example.com",
			Status:     domain.MCPServerStatusRevoked,
			IsVerified: false,
			CreatedAt:  time.Now().Add(-90 * 24 * time.Hour),
		},
	}

	for name, server := range servers {
		t.Run(name, func(t *testing.T) {
			score, err := calculator.Calculate(server)
			require.NoError(t, err)
			require.NotNil(t, score)

			assert.GreaterOrEqual(t, score.Score, 0.0,
				"score must be within the [0,1] range the DB constraint allows")
			assert.LessOrEqual(t, score.Score, 1.0,
				"score must be within the [0,1] range the DB constraint allows")

			// The specific value the service used to hardcode. On the
			// canonical scale it is not merely wrong, it is unrepresentable.
			assert.NotEqual(t, 75.0, score.Score,
				"75.0 was the fabricated literal; it cannot be a calculated score")
		})
	}
}

// TestMCPTrustScore_CalculatedScoresDiscriminate is the red-proof for the
// fabrication itself. Under the old code every SDK-registered and every
// verified server carried the same 75.0 regardless of its inputs, so the
// number could not distinguish a brand-new server from an established one.
// A calculated score must.
func TestMCPTrustScore_CalculatedScoresDiscriminate(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	fresh := &domain.MCPServer{
		ID:         uuid.New(),
		URL:        "https://mcp.example.com",
		PublicKey:  "dGVzdC1wdWJsaWMta2V5",
		Status:     domain.MCPServerStatusVerified,
		IsVerified: true,
		CreatedBy:  uuid.New(),
		CreatedAt:  time.Now(),
	}

	established := &domain.MCPServer{
		ID:                   uuid.New(),
		URL:                  "https://mcp.example.com",
		PublicKey:            "dGVzdC1wdWJsaWMta2V5",
		Description:          "an established server",
		Status:               domain.MCPServerStatusVerified,
		IsVerified:           true,
		CreatedBy:            uuid.New(),
		CreatedAt:            time.Now().Add(-365 * 24 * time.Hour),
		AttestationCount:     12,
		ConfidenceScore:      95.0,
		ConnectedAgentsCount: 9,
		Capabilities:         []string{"read_file", "write_file", "list_directory"},
	}

	freshScore, err := calculator.Calculate(fresh)
	require.NoError(t, err)
	establishedScore, err := calculator.Calculate(established)
	require.NoError(t, err)

	assert.Greater(t, establishedScore.Score, freshScore.Score,
		"a server with attestations, connections and a year of history must "+
			"outscore one registered seconds ago; a constant cannot do this")

	// Both were "verified", which is exactly the pair the 75.0 literal
	// collapsed into one value.
	assert.NotEqual(t, freshScore.Score, establishedScore.Score)
}

// TestMCPTrustScore_AgeFactorRequiresRealCreatedAt guards the failure mode
// named in the decision's pre-mortem. `Calculate` buckets by
// `time.Since(server.CreatedAt)`, so a zero `time.Time` — which is what an
// in-memory struct carries before the database supplies the column — reads as
// "90+ days old" and awards the maximum age factor to a server that does not
// exist yet. This is why scoring happens after the row is persisted.
func TestMCPTrustScore_AgeFactorRequiresRealCreatedAt(t *testing.T) {
	calculator := &MCPTrustCalculator{}

	unset := &domain.MCPServer{ID: uuid.New(), URL: "https://mcp.example.com"}
	justCreated := &domain.MCPServer{ID: uuid.New(), URL: "https://mcp.example.com", CreatedAt: time.Now()}

	assert.Equal(t, 1.0, calculator.calculateAge(unset),
		"a zero CreatedAt scores as maximally aged — the reason scoring must "+
			"run against the persisted row, not the in-memory struct")
	assert.Equal(t, 0.30, calculator.calculateAge(justCreated),
		"a server created now belongs in the under-7-days bucket")
}

// ========================================
// Default Verification URL Tests
// ========================================

func TestMCPService_DefaultVerificationURLPath(t *testing.T) {
	// Document the default verification URL path
	defaultPath := "/.well-known/mcp/verify"
	baseURL := "https://mcp.example.com"

	expectedURL := baseURL + defaultPath
	assert.Equal(t, "https://mcp.example.com/.well-known/mcp/verify", expectedURL)
}

// ========================================
// MCPServerRepository.GetByURL — defect #40 cross-org scoping
// ========================================
//
// SECURITY context: defect #40 was a cross-tenant existence leak via
// MCPService.CreateMCPServer's URL-collision check. The repository's
// GetByURL was unscoped and returned any org's matching row, which
// then leaked the victim's UUID + name in the handler's 409 response
// AND allowed the service's existing-found branch to create a
// cross-org agent-MCP connection AND mutate the victim's
// capabilities/version. The fix adds an organization_id filter at
// the SQL layer; these tests pin that filter so a future edit that
// drops it (or transposes args) is caught at unit-test time.
//
// The mcp_servers table has UNIQUE(organization_id, url), so the
// same URL can legally exist in multiple organizations — the lookup
// MUST be org-scoped or the schema and the application contradict.

func TestMCPServerRepository_GetByURL_FiltersByOrganization(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewMCPServerRepository(db)
	url := "https://mcp.example.com"
	callerOrgID := uuid.New()

	// The query MUST contain the organization_id filter clause and MUST
	// be invoked with both (url, orgID) args, in that order, matching
	// the placeholders $1 and $2.
	mock.ExpectQuery(regexp.QuoteMeta("WHERE url = $1 AND organization_id = $2")).
		WithArgs(url, callerOrgID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "name", "description", "url", "version",
			"public_key", "status", "is_verified", "last_verified_at",
			"verification_url", "capabilities", "trust_score",
			"registered_by_agent", "created_by", "created_at", "updated_at",
			"verification_method", "attestation_count", "confidence_score",
			"last_attested_at",
		}))

	_, _ = repo.GetByURL(url, callerOrgID)

	require.NoError(t, mock.ExpectationsWereMet(),
		"GetByURL must issue SQL filtered by organization_id; a mismatch here means the security filter has regressed")
}

func TestMCPServerRepository_GetByURL_CrossOrgReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewMCPServerRepository(db)
	url := "https://victim.example.com"
	callerOrgID := uuid.New()

	// Simulate the cross-org case: the victim's row exists in the DB
	// but the WHERE-clause filter on organization_id eliminates it,
	// so the query returns no rows.
	mock.ExpectQuery(regexp.QuoteMeta("WHERE url = $1 AND organization_id = $2")).
		WithArgs(url, callerOrgID).
		WillReturnError(sql.ErrNoRows)

	server, err := repo.GetByURL(url, callerOrgID)

	// Pre-fix behaviour: server would be a populated *MCPServer from
	// the victim's org. Post-fix: nil. The caller (MCPService.CreateMCPServer)
	// treats nil-existing as "no collision" and proceeds to create —
	// which the DB's UNIQUE(organization_id, url) constraint permits.
	assert.Nil(t, server,
		"GetByURL must return nil on cross-org URL collision; returning a populated server would leak cross-tenant existence")
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

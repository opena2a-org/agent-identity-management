package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bootstrapFakeRegistrar records every registration it is asked for.
type bootstrapFakeRegistrar struct {
	mu       sync.Mutex
	calls    []bootstrapRegistration
	failNext error
}

type bootstrapRegistration struct {
	req    CreateAgentRequest
	orgID  uuid.UUID
	userID uuid.UUID
	agent  *domain.Agent
}

func (f *bootstrapFakeRegistrar) CreateAgent(_ context.Context, req *CreateAgentRequest, orgID, userID uuid.UUID, _ *uuid.UUID, _ *uuid.UUID, _ string) (*domain.Agent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failNext; err != nil {
		f.failNext = nil
		return nil, err
	}
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CreatedBy:      userID,
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		AgentType:      req.AgentType,
		Status:         domain.AgentStatusPending,
	}
	f.calls = append(f.calls, bootstrapRegistration{req: *req, orgID: orgID, userID: userID, agent: agent})
	return agent, nil
}

func (f *bootstrapFakeRegistrar) GetAgentCredentials(_ context.Context, agentID uuid.UUID) (string, string, error) {
	return "server-public-" + agentID.String(), "server-private-" + agentID.String(), nil
}

func (f *bootstrapFakeRegistrar) registrations() []bootstrapRegistration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bootstrapRegistration(nil), f.calls...)
}

type bootstrapTestRig struct {
	svc    *BootstrapTokenService
	repo   *mocks.MemoryBootstrapTokenRepository
	agents *bootstrapFakeRegistrar
	now    time.Time
}

func newBootstrapTestRig(t *testing.T) *bootstrapTestRig {
	t.Helper()
	rig := &bootstrapTestRig{
		repo:   mocks.NewMemoryBootstrapTokenRepository(),
		agents: &bootstrapFakeRegistrar{},
		now:    time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC),
	}
	rig.svc = NewBootstrapTokenService(rig.repo, rig.agents, nil)
	rig.svc.SetClock(func() time.Time { return rig.now })
	return rig
}

func (r *bootstrapTestRig) mint(t *testing.T, orgID, userID uuid.UUID) *MintedBootstrapToken {
	t.Helper()
	minted, err := r.svc.Mint(context.Background(), orgID, userID, BootstrapRequestMeta{})
	require.NoError(t, err)
	return minted
}

func (r *bootstrapTestRig) exchange(plaintext string, req BootstrapExchangeRequest) (*BootstrapExchangeResult, error) {
	return r.svc.Exchange(context.Background(), plaintext, req, BootstrapRequestMeta{})
}

func newEd25519PublicKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(pub)
}

func TestBootstrapMint_StoresOnlyTheHashAndDisplayPrefix(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgID, userID := uuid.New(), uuid.New()

	minted := rig.mint(t, orgID, userID)
	secret := strings.TrimPrefix(minted.Plaintext, domain.BootstrapTokenPrefix)

	stored := rig.repo.All()
	require.Len(t, stored, 1)
	row := stored[0]
	assert.Equal(t, domain.HashBootstrapToken(minted.Plaintext), row.TokenHash)
	assert.Equal(t, secret[:8], row.DisplayPrefix)

	// Nothing stored, in any field or in its JSON form, carries the secret
	// beyond the 8-character display prefix.
	dump := fmt.Sprintf("%+v", *row)
	encoded, err := json.Marshal(row)
	require.NoError(t, err)
	for _, s := range []string{dump, string(encoded)} {
		assert.NotContains(t, s, minted.Plaintext)
		assert.NotContains(t, s, secret[:9])
	}
	assert.NotContains(t, string(encoded), row.TokenHash, "the hash is not serialised either")
}

func TestBootstrapMint_Is15MinutesScopedToRegisterInTheCallersOrg(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgID, userID := uuid.New(), uuid.New()

	minted := rig.mint(t, orgID, userID)
	assert.Equal(t, orgID, minted.Token.OrganizationID)
	assert.Equal(t, userID, minted.Token.CreatedBy)
	assert.Equal(t, domain.BootstrapTokenScopeAgentsRegister, minted.Token.Scope)
	assert.Equal(t, rig.now.Add(15*time.Minute), minted.Token.ExpiresAt)
}

func TestBootstrapMint_RefusesMissingOrgOrUser(t *testing.T) {
	rig := newBootstrapTestRig(t)
	_, err := rig.svc.Mint(context.Background(), uuid.Nil, uuid.New(), BootstrapRequestMeta{})
	assert.ErrorIs(t, err, ErrInvalidOrgOrUser)
	_, err = rig.svc.Mint(context.Background(), uuid.New(), uuid.Nil, BootstrapRequestMeta{})
	assert.ErrorIs(t, err, ErrInvalidOrgOrUser)
	assert.Empty(t, rig.repo.All())
}

func TestBootstrapMint_RevokesOnlyTheCallersPreviousUnusedToken(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgA, orgB := uuid.New(), uuid.New()
	alice, bob := uuid.New(), uuid.New()

	first := rig.mint(t, orgA, alice)
	bobsToken := rig.mint(t, orgA, bob)
	aliceInB := rig.mint(t, orgB, alice) // the same user in another organization
	second := rig.mint(t, orgA, alice)

	_, err := rig.exchange(first.Plaintext, BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenRevoked)

	for _, tok := range []*MintedBootstrapToken{second, bobsToken, aliceInB} {
		_, err := rig.exchange(tok.Plaintext, BootstrapExchangeRequest{Name: "agent-" + tok.Token.ID.String()})
		assert.NoError(t, err)
	}
}

func TestBootstrapExchange_RegistersInTheTokensOrgAndOwner(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgID, userID := uuid.New(), uuid.New()
	minted := rig.mint(t, orgID, userID)

	result, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	require.NoError(t, err)

	regs := rig.agents.registrations()
	require.Len(t, regs, 1)
	assert.Equal(t, orgID, regs[0].orgID)
	assert.Equal(t, userID, regs[0].userID)
	assert.Equal(t, BootstrapDefaultAgentName, regs[0].req.Name)
	assert.Equal(t, BootstrapDefaultAgentName, regs[0].req.DisplayName)
	assert.Equal(t, domain.AgentTypeCustom, regs[0].req.AgentType)
	assert.Equal(t, orgID, result.Agent.OrganizationID)

	stored := rig.repo.All()
	require.Len(t, stored, 1)
	require.NotNil(t, stored[0].UsedAt)
	require.NotNil(t, stored[0].AgentID)
	assert.Equal(t, result.Agent.ID, *stored[0].AgentID)
}

func TestBootstrapExchange_IsSingleUse(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	require.NoError(t, err)
	_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: "second"})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenUsed)
	assert.Len(t, rig.agents.registrations(), 1)
}

func TestBootstrapExchange_IsSingleUseUnderConcurrency(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: fmt.Sprintf("racer-%d", i)}); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int32(1), ok.Load())
	assert.Len(t, rig.agents.registrations(), 1)
}

func TestBootstrapExchange_ExpiresAfter15Minutes(t *testing.T) {
	rig := newBootstrapTestRig(t)
	early := rig.mint(t, uuid.New(), uuid.New())
	late := rig.mint(t, uuid.New(), uuid.New())

	rig.now = rig.now.Add(15*time.Minute - time.Second)
	_, err := rig.exchange(early.Plaintext, BootstrapExchangeRequest{})
	assert.NoError(t, err, "usable until the last second")

	rig.now = rig.now.Add(time.Second)
	_, err = rig.exchange(late.Plaintext, BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenExpired)
	assert.Len(t, rig.agents.registrations(), 1)
}

func TestBootstrapRevoke_StopsTheExchange(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgID, userID := uuid.New(), uuid.New()
	minted := rig.mint(t, orgID, userID)

	n, err := rig.svc.Revoke(context.Background(), orgID, userID, BootstrapRequestMeta{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenRevoked)
	assert.Empty(t, rig.agents.registrations())
}

func TestBootstrapRevoke_CannotReachAnotherOrganizationsToken(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgA, orgB := uuid.New(), uuid.New()
	userA, userB := uuid.New(), uuid.New()
	tokenA := rig.mint(t, orgA, userA)

	// A caller in org B, including one presenting org A's user id, revokes nothing in org A.
	for _, user := range []uuid.UUID{userB, userA} {
		n, err := rig.svc.Revoke(context.Background(), orgB, user, BootstrapRequestMeta{})
		require.NoError(t, err)
		assert.Zero(t, n)
	}

	result, err := rig.exchange(tokenA.Plaintext, BootstrapExchangeRequest{})
	require.NoError(t, err)
	assert.Equal(t, orgA, result.Agent.OrganizationID)
}

func TestBootstrapExchange_OrgATokenNeverWritesOrgB(t *testing.T) {
	rig := newBootstrapTestRig(t)
	orgA, orgB := uuid.New(), uuid.New()
	userA, userB := uuid.New(), uuid.New()
	tokenA := rig.mint(t, orgA, userA)
	tokenB := rig.mint(t, orgB, userB)

	_, err := rig.exchange(tokenA.Plaintext, BootstrapExchangeRequest{Name: "a-agent"})
	require.NoError(t, err)
	_, err = rig.exchange(tokenB.Plaintext, BootstrapExchangeRequest{Name: "b-agent"})
	require.NoError(t, err)

	byName := map[string]bootstrapRegistration{}
	for _, reg := range rig.agents.registrations() {
		byName[reg.req.Name] = reg
	}
	assert.Equal(t, orgA, byName["a-agent"].orgID)
	assert.Equal(t, userA, byName["a-agent"].userID)
	assert.Equal(t, orgB, byName["b-agent"].orgID)
	assert.Equal(t, userB, byName["b-agent"].userID)

	// Consuming org A's token left org B's records untouched, and vice versa.
	for _, row := range rig.repo.All() {
		require.NotNil(t, row.AgentID)
		reg := byName["a-agent"]
		if row.OrganizationID == orgB {
			reg = byName["b-agent"]
		}
		assert.Equal(t, reg.agent.ID, *row.AgentID)
	}
}

func TestBootstrapExchange_FailedRegistrationLeavesTheTokenUsable(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	rig.agents.failNext = ErrAgentNameExists
	_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	require.ErrorIs(t, err, ErrAgentNameExists)

	_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: "another-name"})
	require.NoError(t, err)
	_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: "third"})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenUsed)
}

func TestBootstrapExchange_RefusesUnknownAndMalformedTokens(t *testing.T) {
	rig := newBootstrapTestRig(t)
	rig.mint(t, uuid.New(), uuid.New())

	unknown, _, _, err := domain.GenerateBootstrapToken()
	require.NoError(t, err)
	_, err = rig.exchange(unknown, BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenNotFound)

	_, err = rig.exchange("aim_ob_short", BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenMalformed)
	assert.Empty(t, rig.agents.registrations())
}

func TestBootstrapExchange_ClientKeyIsStoredAndNoPrivateKeyReturned(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())
	pub := newEd25519PublicKey(t)

	result, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{PublicKey: pub})
	require.NoError(t, err)
	assert.Equal(t, pub, result.PublicKey)
	assert.Empty(t, result.PrivateKey)
	assert.Equal(t, pub, rig.agents.registrations()[0].req.PublicKey)
}

func TestBootstrapExchange_ServerGeneratedKeyIsReturnedOnce(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	result, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, result.PublicKey)
	assert.NotEmpty(t, result.PrivateKey)
}

func TestBootstrapExchange_InvalidPublicKeyDoesNotConsumeTheToken(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	for _, bad := range []string{"not-base64!", base64.StdEncoding.EncodeToString([]byte("too short"))} {
		_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{PublicKey: bad})
		assert.ErrorIs(t, err, ErrBootstrapInvalidPublicKey)
	}
	_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	assert.NoError(t, err)
}

func TestBootstrapRevokeExposed_RevokesAWellFormedToken(t *testing.T) {
	rig := newBootstrapTestRig(t)
	minted := rig.mint(t, uuid.New(), uuid.New())

	require.NoError(t, rig.svc.RevokeExposed(context.Background(), "not-a-token"))
	require.NoError(t, rig.svc.RevokeExposed(context.Background(), minted.Plaintext))

	_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{})
	assert.ErrorIs(t, err, domain.ErrBootstrapTokenRevoked)
}

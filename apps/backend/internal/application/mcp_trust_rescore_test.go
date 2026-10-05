package application

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeMarkerStore struct {
	set      map[string]string
	readErr  error
	writeErr error
	reads    int
}

func newFakeMarkerStore() *fakeMarkerStore {
	return &fakeMarkerStore{set: map[string]string{}}
}

func (f *fakeMarkerStore) IsMarkerSet(_ context.Context, key string) (bool, error) {
	f.reads++
	if f.readErr != nil {
		return false, f.readErr
	}
	_, ok := f.set[key]
	return ok, nil
}

func (f *fakeMarkerStore) SetMarker(_ context.Context, key, description string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.set[key] = description
	return nil
}

// rescoreScoreRepo completes the shared mock, which predates GetHistoryAuditTrail.
type rescoreScoreRepo struct {
	SharedMockMCPTrustScoreRepository
}

func (r *rescoreScoreRepo) GetHistoryAuditTrail(uuid.UUID, int) ([]*domain.MCPTrustScoreHistoryEntry, error) {
	return nil, nil
}

type fakeIDLister struct {
	ids   []uuid.UUID
	err   error
	calls int
}

func (f *fakeIDLister) ListAllIDs(_ context.Context) ([]uuid.UUID, error) {
	f.calls++
	return f.ids, f.err
}

// placeholderServers returns the two shapes a server could have before the
// calculator was wired in, after migration 104 divided them by 100: an
// SDK-registered or verified server holding the rescaled 75.0 literal, and a
// manually registered one holding the 0.0 column default.
func placeholderServers() []*domain.MCPServer {
	created := time.Now().Add(-40 * 24 * time.Hour)
	return []*domain.MCPServer{
		{
			ID: uuid.New(), Name: "sdk-registered", Status: domain.MCPServerStatusVerified,
			IsVerified: true, PublicKey: "pk", URL: "https://sdk.example.com",
			CreatedBy: uuid.New(), Description: "registered by the SDK",
			TrustScore: 0.75, CreatedAt: created,
		},
		{
			ID: uuid.New(), Name: "manual", Status: domain.MCPServerStatusPending,
			URL: "http://manual.example.com", CreatedBy: uuid.New(),
			TrustScore: 0.0, CreatedAt: created,
		},
	}
}

func idsOf(servers []*domain.MCPServer) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(servers))
	for _, s := range servers {
		ids = append(ids, s.ID)
	}
	return ids
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func TestRescoreMCPServersOnce_ScoresEveryServerThroughTheCalculator(t *testing.T) {
	servers := placeholderServers()
	serverRepo := new(SharedMockMCPServerRepository)
	scoreRepo := new(rescoreScoreRepo)
	for _, s := range servers {
		serverRepo.On("GetByID", s.ID).Return(s, nil).Once()
	}
	stored := map[uuid.UUID]float64{}
	scoreRepo.On("Create", mock.AnythingOfType("*domain.MCPTrustScore")).
		Run(func(args mock.Arguments) {
			score := args.Get(0).(*domain.MCPTrustScore)
			stored[score.MCPServerID] = score.Score
		}).Return(nil)

	calculator := NewMCPTrustCalculatorWithRepo(serverRepo, nil, nil, nil, scoreRepo)
	markers := newFakeMarkerStore()

	result, err := RescoreMCPServersOnce(context.Background(), markers, &fakeIDLister{ids: idsOf(servers)}, calculator)
	require.NoError(t, err)

	assert.False(t, result.AlreadyDone)
	assert.Equal(t, len(servers), result.Scored)
	assert.Empty(t, result.Failed)
	require.Len(t, stored, len(servers), "every server gets a calculated score row")
	for _, s := range servers {
		expected, err := calculator.Calculate(s)
		require.NoError(t, err)
		assert.InDelta(t, expected.Score, stored[s.ID], 1e-9, "server %s is scored by the calculator", s.Name)
	}
	// The SDK-registered placeholder is replaced, not kept: the calculator
	// does not return 0.75 for this server.
	assert.NotEqual(t, 0.75, stored[servers[0].ID])
	assert.Contains(t, markers.set, MCPTrustRescoreMarkerKey)
	serverRepo.AssertExpectations(t)
}

func TestRescoreMCPServersOnce_FailedServerIsLeftUnchangedAndLogged(t *testing.T) {
	servers := placeholderServers()
	missing := uuid.New()
	ids := []uuid.UUID{servers[0].ID, missing, servers[1].ID}

	serverRepo := new(SharedMockMCPServerRepository)
	scoreRepo := new(rescoreScoreRepo)
	serverRepo.On("GetByID", servers[0].ID).Return(servers[0], nil)
	serverRepo.On("GetByID", missing).Return(nil, errors.New("mcp server not found"))
	serverRepo.On("GetByID", servers[1].ID).Return(servers[1], nil)
	scoreRepo.On("Create", mock.AnythingOfType("*domain.MCPTrustScore")).Return(nil)

	calculator := NewMCPTrustCalculatorWithRepo(serverRepo, nil, nil, nil, scoreRepo)
	markers := newFakeMarkerStore()
	logs := captureLog(t)

	result, err := RescoreMCPServersOnce(context.Background(), markers, &fakeIDLister{ids: ids}, calculator)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Scored, "the servers after the failure are still scored")
	assert.Equal(t, []uuid.UUID{missing}, result.Failed)
	// Inserting a score row is the only way the calculator changes a server's
	// trust score, so no row for the failed server means it is unchanged.
	for _, call := range scoreRepo.Calls {
		assert.NotEqual(t, missing, call.Arguments.Get(0).(*domain.MCPTrustScore).MCPServerID)
	}
	scoreRepo.AssertNumberOfCalls(t, "Create", 2)
	assert.Contains(t, logs.String(), missing.String())
	assert.Contains(t, logs.String(), "left unchanged")
	assert.Contains(t, markers.set, MCPTrustRescoreMarkerKey, "a per-server failure does not make the pass run again")
}

func TestRescoreMCPServersOnce_StoreFailureCountsAsFailed(t *testing.T) {
	servers := placeholderServers()[:1]
	serverRepo := new(SharedMockMCPServerRepository)
	scoreRepo := new(rescoreScoreRepo)
	serverRepo.On("GetByID", servers[0].ID).Return(servers[0], nil)
	scoreRepo.On("Create", mock.AnythingOfType("*domain.MCPTrustScore")).Return(errors.New("check constraint"))

	calculator := NewMCPTrustCalculatorWithRepo(serverRepo, nil, nil, nil, scoreRepo)
	captureLog(t)

	result, err := RescoreMCPServersOnce(context.Background(), newFakeMarkerStore(), &fakeIDLister{ids: idsOf(servers)}, calculator)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Scored)
	assert.Equal(t, []uuid.UUID{servers[0].ID}, result.Failed)
}

func TestRescoreMCPServersOnce_RunsOnlyOnce(t *testing.T) {
	servers := placeholderServers()
	serverRepo := new(SharedMockMCPServerRepository)
	scoreRepo := new(rescoreScoreRepo)
	for _, s := range servers {
		serverRepo.On("GetByID", s.ID).Return(s, nil)
	}
	scoreRepo.On("Create", mock.AnythingOfType("*domain.MCPTrustScore")).Return(nil)

	calculator := NewMCPTrustCalculatorWithRepo(serverRepo, nil, nil, nil, scoreRepo)
	markers := newFakeMarkerStore()
	lister := &fakeIDLister{ids: idsOf(servers)}

	_, err := RescoreMCPServersOnce(context.Background(), markers, lister, calculator)
	require.NoError(t, err)
	second, err := RescoreMCPServersOnce(context.Background(), markers, lister, calculator)
	require.NoError(t, err)

	assert.True(t, second.AlreadyDone)
	assert.Equal(t, 1, lister.calls, "the second start does not list servers")
	scoreRepo.AssertNumberOfCalls(t, "Create", len(servers))
}

func TestRescoreMCPServersOnce_IncompleteRunLeavesMarkerUnset(t *testing.T) {
	servers := placeholderServers()

	t.Run("marker read fails", func(t *testing.T) {
		markers := newFakeMarkerStore()
		markers.readErr = errors.New("connection refused")
		lister := &fakeIDLister{ids: idsOf(servers)}

		_, err := RescoreMCPServersOnce(context.Background(), markers, lister, NewMCPTrustCalculator(nil, nil, nil, nil))
		require.Error(t, err)
		assert.Equal(t, 0, lister.calls)
		assert.Empty(t, markers.set)
	})

	t.Run("listing fails", func(t *testing.T) {
		markers := newFakeMarkerStore()
		lister := &fakeIDLister{err: errors.New("connection reset")}

		_, err := RescoreMCPServersOnce(context.Background(), markers, lister, NewMCPTrustCalculator(nil, nil, nil, nil))
		require.Error(t, err)
		assert.Empty(t, markers.set)
	})

	t.Run("no calculator", func(t *testing.T) {
		markers := newFakeMarkerStore()
		lister := &fakeIDLister{ids: idsOf(servers)}

		_, err := RescoreMCPServersOnce(context.Background(), markers, lister, nil)
		require.Error(t, err)
		assert.Equal(t, 0, lister.calls)
		assert.Empty(t, markers.set)
	})

	t.Run("context cancelled", func(t *testing.T) {
		serverRepo := new(SharedMockMCPServerRepository)
		scoreRepo := new(rescoreScoreRepo)
		markers := newFakeMarkerStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		result, err := RescoreMCPServersOnce(ctx, markers, &fakeIDLister{ids: idsOf(servers)}, NewMCPTrustCalculatorWithRepo(serverRepo, nil, nil, nil, scoreRepo))
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, result.Scored)
		assert.Empty(t, markers.set)
		scoreRepo.AssertNotCalled(t, "Create", mock.Anything)
	})
}

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type fakeActionRequestNonceRepo struct {
	admits    int
	orgs      []uuid.UUID
	listErr   error
	purged    []uuid.UUID
	purgeErr  map[uuid.UUID]error
	perOrg    int64
	admission domain.ActionRequestAdmission
}

func (f *fakeActionRequestNonceRepo) Admit(context.Context, uuid.UUID, uuid.UUID, []byte, time.Time) (domain.ActionRequestAdmission, error) {
	f.admits++
	return f.admission, nil
}

func (f *fakeActionRequestNonceRepo) ListOrganizationIDs(context.Context) ([]uuid.UUID, error) {
	return f.orgs, f.listErr
}

func (f *fakeActionRequestNonceRepo) PurgeOrganization(_ context.Context, orgID uuid.UUID) (int64, error) {
	if err := f.purgeErr[orgID]; err != nil {
		return 0, err
	}
	f.purged = append(f.purged, orgID)
	return f.perOrg, nil
}

// testClock is a settable process clock for the purge-liveness check.
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func newTestNonceService(repo *fakeActionRequestNonceRepo) (*ActionRequestNonceService, *testClock) {
	clock := &testClock{t: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	svc := NewActionRequestNonceService(repo)
	svc.now = clock.now
	return svc, clock
}

func admitOnce(svc *ActionRequestNonceService) (domain.ActionRequestAdmission, error) {
	return svc.Admit(context.Background(), uuid.New(), uuid.New(), make([]byte, 16), time.Now())
}

// No nonce is stored on a deployment that is not deleting them.
func TestActionRequestNonceService_RefusesUntilAPurgeCompletes(t *testing.T) {
	repo := &fakeActionRequestNonceRepo{admission: domain.ActionRequestAdmitted}
	svc, _ := newTestNonceService(repo)

	_, err := admitOnce(svc)
	assert.ErrorIs(t, err, domain.ErrActionRequestNoncePurgeNotRunning)
	assert.Zero(t, repo.admits, "nothing is inserted")

	_, err = svc.Purge(context.Background())
	require.NoError(t, err)
	got, err := admitOnce(svc)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionRequestAdmitted, got)
	assert.Equal(t, 1, repo.admits)
}

func TestActionRequestNonceService_RefusesWhenThePurgeStops(t *testing.T) {
	repo := &fakeActionRequestNonceRepo{admission: domain.ActionRequestAdmitted}
	svc, clock := newTestNonceService(repo)
	_, err := svc.Purge(context.Background())
	require.NoError(t, err)

	clock.t = clock.t.Add(2 * domain.ActionRequestNoncePurgeInterval)
	_, err = admitOnce(svc)
	require.NoError(t, err, "within two purge intervals")

	clock.t = clock.t.Add(time.Second)
	_, err = admitOnce(svc)
	assert.ErrorIs(t, err, domain.ErrActionRequestNoncePurgeNotRunning)
	assert.Equal(t, 1, repo.admits)
}

func TestActionRequestNonceService_PurgeVisitsEveryOrganization(t *testing.T) {
	orgs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	repo := &fakeActionRequestNonceRepo{orgs: orgs, perOrg: 2}
	svc, _ := newTestNonceService(repo)

	result, err := svc.Purge(context.Background())
	require.NoError(t, err)
	assert.Equal(t, orgs, repo.purged)
	assert.Equal(t, int64(6), result.Deleted)
	assert.Equal(t, 3, result.Organizations)
}

// A purge that fails part-way does not count as running.
func TestActionRequestNonceService_FailedPurgeDoesNotCount(t *testing.T) {
	orgs := []uuid.UUID{uuid.New(), uuid.New()}
	repo := &fakeActionRequestNonceRepo{orgs: orgs, perOrg: 1,
		purgeErr: map[uuid.UUID]error{orgs[1]: errors.New("connection reset")}}
	svc, _ := newTestNonceService(repo)

	result, err := svc.Purge(context.Background())
	assert.Error(t, err)
	assert.Equal(t, int64(1), result.Deleted)
	_, err = admitOnce(svc)
	assert.ErrorIs(t, err, domain.ErrActionRequestNoncePurgeNotRunning)

	repo.purgeErr = nil
	repo.listErr = errors.New("organizations unavailable")
	_, err = svc.Purge(context.Background())
	assert.Error(t, err)
	_, err = admitOnce(svc)
	assert.ErrorIs(t, err, domain.ErrActionRequestNoncePurgeNotRunning)
}

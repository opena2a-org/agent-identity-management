package application

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// One approval mints one pair. Before the code moved to consumed, every poll on
// an approved code minted a new, independent token family for as long as the
// code lived, so revoking the session a user could see left the others valid.

func storedStatus(t *testing.T, svc *DeviceAuthService, deviceCode string) domain.DeviceCodeStatus {
	t.Helper()
	dc, err := svc.deviceCodeRepo.GetByDeviceCode(context.Background(), deviceCode)
	require.NoError(t, err)
	return dc.Status
}

// A poll after the one that received the pair mints nothing and is told the
// grant was already used.
func TestDeviceAuthService_PollToken_SecondPollOnAnApprovedCode_MintsNothing(t *testing.T) {
	svc, _, deviceCode := approvedDeviceGrant(t)
	ctx := context.Background()

	first, err := svc.PollToken(ctx, deviceCode)
	require.NoError(t, err)
	require.NotEmpty(t, first.RefreshToken)
	assert.Equal(t, domain.DeviceCodeStatusConsumed, storedStatus(t, svc, deviceCode))

	second, err := svc.PollToken(ctx, deviceCode)
	assert.ErrorIs(t, err, ErrDeviceCodeUsed)
	assert.True(t, second == nil, "no second pair may be minted")
}

// Polls racing on one approved code yield exactly one pair; every other poll
// is told the grant was already used.
func TestDeviceAuthService_PollToken_ConcurrentPolls_YieldExactlyOnePair(t *testing.T) {
	svc, _, deviceCode := approvedDeviceGrant(t)
	const polls = 16

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		mu    sync.Mutex
		pairs int
		used  int
		other []error
	)
	for i := 0; i < polls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tok, err := svc.PollToken(context.Background(), deviceCode)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && tok != nil:
				pairs++
			case errors.Is(err, ErrDeviceCodeUsed):
				used++
			default:
				other = append(other, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, 1, pairs, "one approval mints one pair")
	assert.Equal(t, polls-1, used)
	assert.Empty(t, other)
}

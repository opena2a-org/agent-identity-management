package handlers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// An SDK-download token is retired by its sdk_tokens row, not by a denylist
// mark, so a reuse the row detects has no mark to classify from. The row the
// retiring rotation created (the successor) stores the address and user agent
// of the client that rotated; a confirmed row reuse compares them with the
// presenter under the same three-outcome rule as a login-family reuse:
// sameClient, differentClient, or unknown when there is no successor row or
// an address has no class. Enforcement is the same either way; the login
// family's mark-based classification is unchanged.

const (
	sdkCMAddr      = "203.0.113.7"
	sdkCMOtherAddr = "198.51.100.9"
	sdkCMUA        = "aim-sdk-python/1.0"
	sdkCMOtherUA   = "aim-sdk-python/2.0"
)

// sdkCMFixture is the SDK family fixture with the test's peer (0.0.0.0)
// trusted as a proxy so X-Real-IP names the client.
func sdkCMFixture(t *testing.T, store *rotationStore) *sdkFamilyFixture {
	t.Helper()
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	return newSDKFamilyFixture(t, store)
}

// refreshFrom presents token on the refresh route from addr with ua.
func (f *sdkFamilyFixture) refreshFrom(t *testing.T, token, addr, ua string) (*RefreshTokenResponse, string, int) {
	t.Helper()
	body, status := cmPost(t, f.refresh, "/auth/refresh", fmt.Sprintf(`{"refreshToken":%q}`, token), addr, ua)
	var out RefreshTokenResponse
	_ = json.Unmarshal([]byte(body), &out)
	return &out, body, status
}

// rotateFrom downloads an SDK and rotates its token once from addr with ua.
// It returns the retired token and the live one.
func (f *sdkFamilyFixture) rotateFrom(t *testing.T, addr, ua string) (retired, live string) {
	t.Helper()
	retired = f.downloadSDK(t)
	out, body, status := f.refreshFrom(t, retired, addr, ua)
	require.Equal(t, fiber.StatusOK, status, body)
	require.True(t, out.Rotated)
	return retired, out.RefreshToken
}

// assertSDKRowReuse pins the unchanged enforcement of a row-detected reuse
// (the same 401, no tokens, every row of the family revoked) and the one
// reuse row carrying the classification.
func assertSDKRowReuse(t *testing.T, f *sdkFamilyFixture, body string, status int, want string) {
	t.Helper()
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken")
	assert.Equal(t, 0, f.repo.activeRows(), "every row of the family is revoked")
	rows := f.reuseRows()
	require.Len(t, rows, 1, "the reuse is recorded once")
	assert.Equal(t, true, rows[0].Metadata["familyRevoked"])
	assert.Equal(t, want, rows[0].Metadata["clientMatch"])
}

// SC1: a retired SDK token presented again by the client that rotated it
// (same address, same user agent) is sameClient, with or without a
// revocation store: the verdict comes from the successor row.
func TestRefreshToken_SDKRowReuseFromTheRotatingClientIsSameClient(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := sdkCMFixture(t, store)
			s1, _ := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
			_, body, status := f.refreshFrom(t, s1, sdkCMAddr, sdkCMUA)
			assertSDKRowReuse(t, f, body, status, auth.ClientMatchSame)
		})
	}
}

// SC2: a replay from another address is differentClient.
func TestRefreshToken_SDKRowReuseFromAnotherAddressIsDifferentClient(t *testing.T) {
	f := sdkCMFixture(t, nil)
	s1, _ := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	_, body, status := f.refreshFrom(t, s1, sdkCMOtherAddr, sdkCMUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchDifferent)
}

// SC3: a replay from the same address with another user agent is
// differentClient.
func TestRefreshToken_SDKRowReuseWithAnotherUserAgentIsDifferentClient(t *testing.T) {
	f := sdkCMFixture(t, nil)
	s1, _ := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	_, body, status := f.refreshFrom(t, s1, sdkCMAddr, sdkCMOtherUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchDifferent)
}

// SC4: IPv6 addresses compare by their /64, as the login family's do.
func TestRefreshToken_SDKRowReuseIPv6ComparesByTheSlash64(t *testing.T) {
	f := sdkCMFixture(t, nil)
	s1, _ := f.rotateFrom(t, "2001:db8:1:2::1", sdkCMUA)
	_, body, status := f.refreshFrom(t, s1, "2001:db8:1:2:aaaa:bbbb:cccc:dddd", sdkCMUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchSame)

	g := sdkCMFixture(t, nil)
	t1, _ := g.rotateFrom(t, "2001:db8:1:2::1", sdkCMUA)
	_, body, status = g.refreshFrom(t, t1, "2001:db8:1:3::1", sdkCMUA)
	assertSDKRowReuse(t, g, body, status, auth.ClientMatchDifferent)
}

// SC5: the loser of a concurrent presentation from the same client (two SDK
// processes holding one token) is a reuse classified sameClient: the winner's
// row stores the client the loser presents from.
func TestRefreshToken_SDKRaceLoserFromTheSameClientIsSameClient(t *testing.T) {
	f := sdkCMFixture(t, &rotationStore{})
	s1 := f.downloadSDK(t)
	var winnerStatus int
	f.repo.beforeRetire = func() { _, _, winnerStatus = f.refreshFrom(t, s1, sdkCMAddr, sdkCMUA) }

	_, body, status := f.refreshFrom(t, s1, sdkCMAddr, sdkCMUA)
	require.Equal(t, fiber.StatusOK, winnerStatus, "the first retirement wins")
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchSame)
}

// SC6: a retired row with no successor row to read (the row the rotation
// created is gone) is unknown, never a verdict; the reuse still ends the
// family.
func TestRefreshToken_SDKRowReuseWithoutASuccessorRowIsUnknown(t *testing.T) {
	f := sdkCMFixture(t, nil)
	s1, s2 := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	delete(f.repo.rows, sha256Hex(s2))

	_, body, status := f.refreshFrom(t, s1, sdkCMAddr, sdkCMUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchUnknown)
}

// SC7: a presenter whose forwarded address does not parse has no class:
// unknown, and the record carries the connecting address, as a login
// family's record does.
func TestRefreshToken_SDKRowReuseFromAnUnparsableAddressIsUnknown(t *testing.T) {
	f := sdkCMFixture(t, nil)
	logs := captureLog(t)
	s1, _ := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	_, body, status := f.refreshFrom(t, s1, "not-an-address clientMatch=sameClient", sdkCMUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchUnknown)
	assert.Contains(t, logs.String(), "clientMatch=unknown ip=0.0.0.0")
}

// SC8: the successor row stores the rotating client as the route saw it: the
// trusted-proxy client address and the user agent, which is what a later
// reuse is compared with. A forwarded address that does not parse is not
// stored; the connecting address is.
func TestRefreshToken_SDKRotationRowStoresTheRotatingClient(t *testing.T) {
	f := sdkCMFixture(t, nil)
	_, s2 := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	row := f.repo.row(s2)
	require.NotNil(t, row)
	require.NotNil(t, row.IPAddress)
	require.NotNil(t, row.UserAgent)
	assert.Equal(t, sdkCMAddr, *row.IPAddress)
	assert.Equal(t, sdkCMUA, *row.UserAgent)

	g := sdkCMFixture(t, nil)
	_, t2 := g.rotateFrom(t, "not-an-address", sdkCMUA)
	require.NotNil(t, g.repo.row(t2).IPAddress)
	assert.Equal(t, "0.0.0.0", *g.repo.row(t2).IPAddress)
}

// SC9: the record carries the classification and identifiers only; the
// successor row's address and user agent are compared, not written.
func TestRefreshToken_SDKRowReuseRecordCarriesTheClassification(t *testing.T) {
	f := sdkCMFixture(t, nil)
	logs := captureLog(t)
	s1, _ := f.rotateFrom(t, sdkCMAddr, sdkCMUA)
	_, body, status := f.refreshFrom(t, s1, sdkCMOtherAddr, sdkCMOtherUA)
	assertSDKRowReuse(t, f, body, status, auth.ClientMatchDifferent)
	line := logs.String()
	assert.Contains(t, line, "SECURITY refresh_token_reuse")
	assert.Contains(t, line, "clientMatch=differentClient")
	assert.Contains(t, line, "ip="+sdkCMOtherAddr)
	assert.NotContains(t, line, sdkCMAddr, "the stored client is compared, not logged")
	assert.NotContains(t, line, sdkCMUA)
	rows := f.reuseRows()
	require.Len(t, rows, 1)
	assert.Equal(t, sdkCMOtherAddr, rows[0].IPAddress)
	for key := range rows[0].Metadata {
		assert.Contains(t, []string{"familyId", "jti", "familyRevoked", "clientMatch"}, key)
	}
}

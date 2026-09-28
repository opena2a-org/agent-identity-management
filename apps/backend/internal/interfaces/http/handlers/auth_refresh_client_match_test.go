package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
)

// A refresh-token reuse records whether the client that presented the token
// again is the one that retired it (unit 9933, PR 1). Rotation and logout
// store a keyed, token-bound client mark as the value of the jti's denylist
// key; a confirmed reuse reads it back and records clientMatch as sameClient,
// differentClient or unknown. A legitimate race (two tabs, two SDK processes)
// is sameClient; a replay from elsewhere is differentClient; a legacy value,
// a store that cannot read, or an address that does not parse is unknown.
// Enforcement is the same in all three: the family is revoked and the same
// 401 is answered. The client is varied through the trusted-proxy address,
// the only address the route records (app.Test connects from 0.0.0.0).

const (
	cmAddrA   = "203.0.113.7"
	cmAddrB   = "198.51.100.9"
	cmUA      = "client-match-cell/1.0"
	cmUAOther = "client-match-cell/2.0"
)

var cmMarkRE = regexp.MustCompile(`^v1:[0-9a-f]{32}$`)

// valueStore keeps each key's JSON-encoded value, as the Redis cache does, so
// a cell can read what rotation and logout wrote. blindReads makes the next
// N existence checks report "absent", which is what a concurrent
// presentation sees before the other one's write lands.
type valueStore struct {
	data       map[string]string
	blindReads int
	nxLost     int // set-if-absent writes that found the key present
}

func (s *valueStore) Exists(_ context.Context, key string) (bool, error) {
	if s.blindReads > 0 {
		s.blindReads--
		return false, nil
	}
	_, ok := s.data[key]
	return ok, nil
}

func (s *valueStore) put(key string, value interface{}) {
	if s.data == nil {
		s.data = map[string]string{}
	}
	raw, _ := json.Marshal(value)
	s.data[key] = string(raw)
}

func (s *valueStore) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	s.put(key, value)
	return nil
}

func (s *valueStore) SetWithNX(_ context.Context, key string, value interface{}, _ time.Duration) (bool, error) {
	if _, ok := s.data[key]; ok {
		s.nxLost++
		return false, nil
	}
	s.put(key, value)
	return true, nil
}

func (s *valueStore) Get(_ context.Context, key string, dest interface{}) error {
	raw, ok := s.data[key]
	if !ok {
		return fmt.Errorf("cache miss: %s", key)
	}
	return json.Unmarshal([]byte(raw), dest)
}

// value returns the decoded string stored under key, or "" when absent.
func (s *valueStore) value(key string) string {
	var v string
	if raw, ok := s.data[key]; ok {
		_ = json.Unmarshal([]byte(raw), &v)
	}
	return v
}

// plainValueStore is a valueStore with no set-if-absent and no read, so the
// rotation takes the plain-store write and a reuse cannot be classified.
type plainValueStore struct{ vs *valueStore }

func (s plainValueStore) Exists(ctx context.Context, key string) (bool, error) {
	return s.vs.Exists(ctx, key)
}
func (s plainValueStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return s.vs.Set(ctx, key, value, ttl)
}

// readerPlainStore has the plain write and a read, but no set-if-absent.
type readerPlainStore struct{ plainValueStore }

func (s readerPlainStore) Get(ctx context.Context, key string, dest interface{}) error {
	return s.vs.Get(ctx, key, dest)
}

type cmFixture struct {
	app    *fiber.App
	svc    *auth.JWTService
	audit  *familyAuditRepo
	p1     string
	userID uuid.UUID
	orgID  uuid.UUID
}

// cmApp serves the refresh route and the logout route over one JWT service
// and one revocation store, with the test's peer (0.0.0.0) trusted as a proxy
// so X-Real-IP names the client.
func cmApp(t *testing.T, store auth.RevocationStore) *cmFixture {
	t.Helper()
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "match@example.com"), nil
	}}
	audit := &familyAuditRepo{}
	app, svc := newRefreshTestAppAudit(t, users, nil, audit)
	svc.SetRevoker(auth.NewTokenRevoker(store, false))
	lh := &AuthHandler{jwtService: svc}
	app.Post("/auth/logout", lh.Logout)
	_, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "match@example.com", "admin")
	require.NoError(t, err)
	return &cmFixture{app: app, svc: svc, audit: audit, p1: refresh, userID: userID, orgID: orgID}
}

func cmPost(t *testing.T, app *fiber.App, path, body, addr, ua string) (string, int) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", addr)
	req.Header.Set("User-Agent", ua)
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	return string(raw), resp.StatusCode
}

func cmRefresh(t *testing.T, f *cmFixture, token, addr, ua string) (*RefreshTokenResponse, string, int) {
	t.Helper()
	body, status := cmPost(t, f.app, "/auth/refresh", fmt.Sprintf(`{"refreshToken":%q}`, token), addr, ua)
	var out RefreshTokenResponse
	_ = json.Unmarshal([]byte(body), &out)
	return &out, body, status
}

// reuseRows returns the refresh_token_reuse audit rows written so far.
func reuseRows(f *cmFixture) []*domain.AuditLog {
	var out []*domain.AuditLog
	for _, r := range f.audit.rows {
		if r.Action == domain.AuditActionRefreshTokenReuse {
			out = append(out, r)
		}
	}
	return out
}

// assertRefusedAsReuse pins the unchanged enforcement: the same 401, no
// tokens, the family revoked, one reuse row carrying the classification.
func assertRefusedAsReuse(t *testing.T, f *cmFixture, body string, status int, jti, want string) {
	t.Helper()
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken")
	revoked, known := f.svc.CheckFamilyRevoked(context.Background(), jti)
	assert.True(t, revoked && known, "the family is revoked")
	rows := reuseRows(f)
	require.Len(t, rows, 1)
	assert.Equal(t, jti, rows[0].Metadata["jti"])
	assert.Equal(t, want, rows[0].Metadata["clientMatch"])
}

// M1: the loser of a concurrent presentation from the same client (two tabs)
// is a reuse classified sameClient.
func TestCA9933_RaceLoserFromTheSameClientIsSameClient(t *testing.T) {
	store := &valueStore{blindReads: 4}
	f := cmApp(t, store)
	jti := jtiOfToken(t, f.svc, f.p1)
	_, _, s1 := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, s1)
	_, body, s2 := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	assertRefusedAsReuse(t, f, body, s2, jti, "sameClient")
	assert.Zero(t, store.blindReads, "both presentations read past the denylist")
	assert.Equal(t, 1, store.nxLost, "the second presentation lost the retirement write")
}

// M2: a later replay of a rotated-out token by the client that rotated it.
func TestCA9933_ReplayFromTheRotatingClientIsSameClient(t *testing.T) {
	f := cmApp(t, &valueStore{})
	jti := jtiOfToken(t, f.svc, f.p1)
	_, _, s1 := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, s1)
	_, body, s2 := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	assertRefusedAsReuse(t, f, body, s2, jti, "sameClient")
}

// M3: the same token replayed from another address is differentClient.
func TestCA9933_ReplayFromAnotherAddressIsDifferentClient(t *testing.T) {
	f := cmApp(t, &valueStore{})
	jti := jtiOfToken(t, f.svc, f.p1)
	cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	_, body, status := cmRefresh(t, f, f.p1, cmAddrB, cmUA)
	assertRefusedAsReuse(t, f, body, status, jti, "differentClient")
}

// M4: the same address with another user agent is differentClient.
func TestCA9933_ReplayWithAnotherUserAgentIsDifferentClient(t *testing.T) {
	f := cmApp(t, &valueStore{})
	jti := jtiOfToken(t, f.svc, f.p1)
	cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	_, body, status := cmRefresh(t, f, f.p1, cmAddrA, cmUAOther)
	assertRefusedAsReuse(t, f, body, status, jti, "differentClient")
}

// M5: IPv6 compares by /64 (temporary addresses rotate inside it); another
// /64 is differentClient.
func TestCA9933_IPv6ComparesByTheSlash64(t *testing.T) {
	for _, c := range []struct{ replay, want string }{
		{"2001:db8:1:2::beef", "sameClient"},
		{"2001:db8:1:3::1", "differentClient"},
	} {
		t.Run(c.want, func(t *testing.T) {
			f := cmApp(t, &valueStore{})
			jti := jtiOfToken(t, f.svc, f.p1)
			cmRefresh(t, f, f.p1, "2001:db8:1:2::1", cmUA)
			_, body, status := cmRefresh(t, f, f.p1, c.replay, cmUA)
			assertRefusedAsReuse(t, f, body, status, jti, c.want)
		})
	}
}

// M6: a denylist value written before this change ("1") is unknown, never
// either verdict.
func TestCA9933_LegacyValueIsUnknown(t *testing.T) {
	store := &valueStore{}
	f := cmApp(t, store)
	jti := jtiOfToken(t, f.svc, f.p1)
	store.put("revoked:jti:"+jti, "1")
	_, body, status := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	assertRefusedAsReuse(t, f, body, status, jti, "unknown")
}

// M7: a store that cannot read values back classifies nothing: unknown. Its
// plain write still stores the mark (the plain-store path).
func TestCA9933_StoreWithoutAReadIsUnknown(t *testing.T) {
	vs := &valueStore{}
	f := cmApp(t, plainValueStore{vs: vs})
	jti := jtiOfToken(t, f.svc, f.p1)
	_, _, s1 := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, s1)
	assert.Regexp(t, cmMarkRE, vs.value("revoked:jti:"+jti), "the plain-store write carries the mark")
	_, body, status := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	assertRefusedAsReuse(t, f, body, status, jti, "unknown")
}

// M8: the plain-store path with a read classifies like the set-if-absent path.
func TestCA9933_PlainStoreWithAReadClassifies(t *testing.T) {
	vs := &valueStore{}
	f := cmApp(t, readerPlainStore{plainValueStore{vs: vs}})
	jti := jtiOfToken(t, f.svc, f.p1)
	cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	_, body, status := cmRefresh(t, f, f.p1, cmAddrB, cmUA)
	assertRefusedAsReuse(t, f, body, status, jti, "differentClient")
}

// M9: an address that does not parse writes the legacy value at rotation and
// is unknown at reuse; it never becomes sameClient by matching itself.
func TestCA9933_UnparsableAddressIsUnknown(t *testing.T) {
	store := &valueStore{}
	f := cmApp(t, store)
	jti := jtiOfToken(t, f.svc, f.p1)
	cmRefresh(t, f, f.p1, "not-an-address", cmUA)
	assert.Equal(t, "1", store.value("revoked:jti:"+jti), "no class: the legacy value")
	_, body, status := cmRefresh(t, f, f.p1, "not-an-address", cmUA)
	assertRefusedAsReuse(t, f, body, status, jti, "unknown")
}

// M10: the stored value is a keyed, token-bound mark: v1 plus 32 hex, holding
// neither the address nor the user agent, and different for another token
// from the same client.
func TestCA9933_TheStoredValueIsATokenBoundMark(t *testing.T) {
	store := &valueStore{}
	f := cmApp(t, store)
	jti1 := jtiOfToken(t, f.svc, f.p1)
	out, _, status := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, status)
	jti2 := jtiOfToken(t, f.svc, out.RefreshToken)
	_, _, status = cmRefresh(t, f, out.RefreshToken, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, status)

	m1, m2 := store.value("revoked:jti:"+jti1), store.value("revoked:jti:"+jti2)
	for _, m := range []string{m1, m2} {
		assert.Regexp(t, cmMarkRE, m)
		assert.NotContains(t, m, cmAddrA)
		assert.NotContains(t, m, cmUA)
	}
	assert.NotEqual(t, m1, m2, "marks are bound to the jti, so they do not link tokens")
}

// M11: logout stores the mark for the refresh token, so a replay of a
// logged-out token by the client that logged out is sameClient and by another
// is differentClient. The access token's revocation keeps the plain value.
func TestCA9933_LogoutStoresTheMark(t *testing.T) {
	for _, c := range []struct{ addr, want string }{{cmAddrA, "sameClient"}, {cmAddrB, "differentClient"}} {
		t.Run(c.want, func(t *testing.T) {
			store := &valueStore{}
			f := cmApp(t, store)
			access, refresh, err := f.svc.GenerateTokenPair(f.userID.String(), f.orgID.String(), "match@example.com", "admin")
			require.NoError(t, err)
			jti := jtiOfToken(t, f.svc, refresh)
			req := httptest.NewRequest("POST", "/auth/logout", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, refresh)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+access)
			req.Header.Set("X-Real-IP", cmAddrA)
			req.Header.Set("User-Agent", cmUA)
			resp, err := f.app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			assert.Regexp(t, cmMarkRE, store.value("revoked:jti:"+jti))
			assert.Equal(t, "1", store.value("revoked:jti:"+jtiOfToken(t, f.svc, access)), "the access token keeps the plain value")

			_, body, status := cmRefresh(t, f, refresh, c.addr, cmUA)
			assertRefusedAsReuse(t, f, body, status, jti, c.want)
		})
	}
}

// M12: the SECURITY line carries the classification and the recorded address
// is the trusted-proxy client address; neither record carries the mark.
func TestCA9933_RecordsCarryTheClassificationNotTheMark(t *testing.T) {
	store := &valueStore{}
	f := cmApp(t, store)
	jti := jtiOfToken(t, f.svc, f.p1)
	cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	mark := store.value("revoked:jti:" + jti)
	require.Regexp(t, cmMarkRE, mark)

	logs := captureLog(t)
	cmRefresh(t, f, f.p1, cmAddrB, cmUA)
	line := logs.String()
	assert.Contains(t, line, "SECURITY refresh_token_reuse")
	assert.Contains(t, line, "clientMatch=differentClient")
	assert.Contains(t, line, "ip="+cmAddrB)
	assert.NotContains(t, line, mark[3:], "the mark never reaches the log")

	rows := reuseRows(f)
	require.Len(t, rows, 1)
	assert.Equal(t, cmAddrB, rows[0].IPAddress, "the row records the trusted-proxy client address")
	raw, _ := json.Marshal(rows[0].Metadata)
	assert.NotContains(t, string(raw), mark[3:], "the mark never reaches the audit row")
}

// M13: the classification is only on reuse records. A member of a family the
// reuse revoked is refused as refresh_session_revoked, with no clientMatch.
func TestCA9933_SessionRevokedRecordsCarryNoClassification(t *testing.T) {
	f := cmApp(t, &valueStore{})
	out, _, status := cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	require.Equal(t, fiber.StatusOK, status)
	cmRefresh(t, f, f.p1, cmAddrB, cmUA) // the reuse revokes the family
	_, _, status = cmRefresh(t, f, out.RefreshToken, cmAddrA, cmUA)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	var session *domain.AuditLog
	for _, r := range f.audit.rows {
		if r.Action == domain.AuditActionRefreshSessionRevoked {
			session = r
		}
	}
	require.NotNil(t, session)
	_, has := session.Metadata["clientMatch"]
	assert.False(t, has)
}

// M14 (pin): the Redis cache can read a denylist value back, so a stack with
// Redis classifies reuses without configuration.
func TestCA9933_RedisCacheIsAReadableStore(t *testing.T) {
	var _ auth.RevocationStoreReader = (*cache.RedisCache)(nil)
}

// M15: presenting an already-rotated token at logout does not replace the
// mark of the client that rotated it. Rotated from B, logged out from A, then
// replayed from A: differentClient, not a false sameClient. Holds on a plain
// store too.
func TestCA9933_LogoutKeepsTheRotatingClientsMark(t *testing.T) {
	for name, mk := range map[string]func(*valueStore) auth.RevocationStore{
		"setIfAbsent": func(vs *valueStore) auth.RevocationStore { return vs },
		"plain":       func(vs *valueStore) auth.RevocationStore { return readerPlainStore{plainValueStore{vs: vs}} },
	} {
		t.Run(name, func(t *testing.T) {
			vs := &valueStore{}
			f := cmApp(t, mk(vs))
			jti := jtiOfToken(t, f.svc, f.p1)
			_, _, status := cmRefresh(t, f, f.p1, cmAddrB, cmUA)
			require.Equal(t, fiber.StatusOK, status)
			rotated := vs.value("revoked:jti:" + jti)
			require.Regexp(t, cmMarkRE, rotated)

			body, status := cmPost(t, f.app, "/auth/logout", fmt.Sprintf(`{"refreshToken":%q}`, f.p1), cmAddrA, cmUA)
			require.Equal(t, fiber.StatusOK, status, body)
			assert.Equal(t, rotated, vs.value("revoked:jti:"+jti), "logout kept the first mark")

			_, body, status = cmRefresh(t, f, f.p1, cmAddrA, cmUA)
			assertRefusedAsReuse(t, f, body, status, jti, "differentClient")
		})
	}
}

// M16: a forwarded address that does not parse is not recorded; the
// connecting address is, so the value cannot add fields to the SECURITY line.
func TestCA9933_AnUnparsableForwardedAddressIsNotRecorded(t *testing.T) {
	f := cmApp(t, &valueStore{})
	cmRefresh(t, f, f.p1, cmAddrA, cmUA)
	logs := captureLog(t)
	forged := "1.2.3.4 clientMatch=sameClient"
	cmRefresh(t, f, f.p1, forged, cmUA)
	assert.NotContains(t, logs.String(), forged)
	assert.Contains(t, logs.String(), "clientMatch=unknown ip=0.0.0.0")
	rows := reuseRows(f)
	require.Len(t, rows, 1)
	assert.Equal(t, "0.0.0.0", rows[0].IPAddress)
	assert.Equal(t, "unknown", rows[0].Metadata["clientMatch"])
}

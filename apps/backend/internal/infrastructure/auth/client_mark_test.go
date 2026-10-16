package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The client class groups an IPv4 address (mapped or not) exactly and an
// IPv6 address by its /64; an address that does not parse has none.
func TestClientClass(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"203.0.113.7", "4:203.0.113.7"},
		{" 203.0.113.7 ", "4:203.0.113.7"},
		{"::ffff:203.0.113.7", "4:203.0.113.7"},
		{"2001:db8:1:2::1", "6:2001:db8:1:2::"},
		{"2001:db8:1:2:aaaa:bbbb:cccc:dddd", "6:2001:db8:1:2::"},
	} {
		got, ok := clientClass(c.in)
		assert.True(t, ok, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
	for _, in := range []string{"", "not-an-address", "203.0.113.7:443"} {
		_, ok := clientClass(in)
		assert.False(t, ok, in)
	}
}

// The mark is keyed by JWT_SECRET, bound to the jti, and legacy "1" when
// there is no jti or no class.
func TestClientMark(t *testing.T) {
	a := &JWTService{secret: []byte("secret-a-0123456789-0123456789-01")}
	b := &JWTService{secret: []byte("secret-b-0123456789-0123456789-01")}
	cl := Client{Address: "203.0.113.7", UserAgent: "ua/1"}

	m := a.clientMark("jti-1", cl)
	assert.Regexp(t, clientMarkRE, m)
	assert.Equal(t, m, a.clientMark("jti-1", cl), "deterministic")
	assert.Equal(t, m, a.clientMark("jti-1", Client{Address: "::ffff:203.0.113.7", UserAgent: "ua/1"}), "mapped IPv4 is the same client")
	assert.NotEqual(t, m, b.clientMark("jti-1", cl), "another secret, another key")
	assert.NotEqual(t, m, a.clientMark("jti-2", cl), "bound to the jti")
	assert.NotEqual(t, m, a.clientMark("jti-1", Client{Address: "203.0.113.7", UserAgent: "ua/2"}))
	assert.Equal(t, legacyRevokedValue, a.clientMark("", cl))
	assert.Equal(t, legacyRevokedValue, a.clientMark("jti-1", Client{Address: "nope"}))
}

// CompareClients applies the mark's rule to a stored client record: the same
// address class and user agent is sameClient, another address or user agent
// is differentClient, and an address without a class on either side is
// unknown.
func TestCompareClients(t *testing.T) {
	stored := Client{Address: "203.0.113.7", UserAgent: "ua/1"}
	for _, c := range []struct {
		name    string
		current Client
		want    string
	}{
		{"same address and user agent", Client{Address: "203.0.113.7", UserAgent: "ua/1"}, ClientMatchSame},
		{"mapped IPv4 is the same address", Client{Address: "::ffff:203.0.113.7", UserAgent: "ua/1"}, ClientMatchSame},
		{"another address", Client{Address: "198.51.100.9", UserAgent: "ua/1"}, ClientMatchDifferent},
		{"another user agent", Client{Address: "203.0.113.7", UserAgent: "ua/2"}, ClientMatchDifferent},
		{"current address has no class", Client{Address: "not-an-address", UserAgent: "ua/1"}, ClientMatchUnknown},
		{"current address is empty", Client{Address: "", UserAgent: "ua/1"}, ClientMatchUnknown},
	} {
		assert.Equal(t, c.want, CompareClients(stored, c.current), c.name)
	}
	v6 := Client{Address: "2001:db8:1:2::1", UserAgent: "ua/1"}
	assert.Equal(t, ClientMatchSame, CompareClients(v6, Client{Address: "2001:db8:1:2:aaaa:bbbb:cccc:dddd", UserAgent: "ua/1"}), "IPv6 compares by the /64")
	assert.Equal(t, ClientMatchDifferent, CompareClients(v6, Client{Address: "2001:db8:1:3::1", UserAgent: "ua/1"}), "another /64")
	assert.Equal(t, ClientMatchUnknown, CompareClients(Client{Address: "nope", UserAgent: "ua/1"}, stored), "stored address has no class")
	assert.Equal(t, ClientMatchUnknown, CompareClients(Client{}, Client{}), "no address on either side")
}

// Without a revoker there is nothing to read: unknown.
func TestClassifyReuseWithoutARevokerIsUnknown(t *testing.T) {
	s := &JWTService{secret: []byte("secret-a-0123456789-0123456789-01")}
	assert.Equal(t, ClientMatchUnknown, s.ClassifyReuse(context.Background(), "jti-1", Client{Address: "203.0.113.7"}))
}

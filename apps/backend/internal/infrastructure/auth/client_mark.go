package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"regexp"
	"strings"
)

// A refresh-token reuse is either a legitimate client racing itself (two tabs,
// two SDK processes presenting the same token) or a replay by someone else.
// Enforcement treats both the same; the record says which it looks like. At
// rotation and at logout the value of the jti's denylist key is a client mark
// instead of "1": a keyed, truncated hash of the jti, the client's address
// class and its user agent. A confirmed reuse reads it back and compares.
//
// The mark holds no address or user agent in clear, cannot be reversed
// without the key, and is bound to the jti, so marks do not link tokens. The
// key is derived from JWT_SECRET: every instance shares it, and rotating the
// secret ends every token, so an old mark can only become unknown.

// ClientMatch values recorded on a refresh_token_reuse.
const (
	ClientMatchSame      = "sameClient"
	ClientMatchDifferent = "differentClient"
	ClientMatchUnknown   = "unknown"
)

// legacyRevokedValue is the denylist value when there is no mark to write:
// every value before marks existed, an access token, a family key, a client
// whose address does not parse.
const legacyRevokedValue = "1"

const clientMarkKeyLabel = "aim refresh client mark v1"

var clientMarkRE = regexp.MustCompile(`^v1:[0-9a-f]{32}$`)

// Client is the presenter of a token as the route saw it: the client address
// (the trusted-proxy address, see middleware.ClientIP) and the raw
// User-Agent header.
type Client struct {
	Address   string
	UserAgent string
}

// RevocationStoreReader is the optional read form of the store, used only to
// classify a confirmed reuse. *cache.RedisCache satisfies it structurally.
type RevocationStoreReader interface {
	Get(ctx context.Context, key string, dest interface{}) error
}

// clientClass groups an address: an IPv4 address (including an IPv4-mapped
// one) exactly, an IPv6 address by its /64, since temporary addresses rotate
// inside it. An address that does not parse has no class.
func clientClass(address string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(address))
	if ip == nil {
		return "", false
	}
	if v4 := ip.To4(); v4 != nil {
		return "4:" + v4.String(), true
	}
	return "6:" + ip.Mask(net.CIDRMask(64, 128)).String(), true
}

// clientMark is the denylist value for jti as presented by client, or the
// legacy value when there is no key, no jti or no address class.
func (s *JWTService) clientMark(jti string, client Client) string {
	class, ok := clientClass(client.Address)
	if !ok || jti == "" || len(s.secret) == 0 {
		return legacyRevokedValue
	}
	kmac := hmac.New(sha256.New, s.secret)
	kmac.Write([]byte(clientMarkKeyLabel))
	mac := hmac.New(sha256.New, kmac.Sum(nil))
	mac.Write([]byte(jti))
	mac.Write([]byte{0})
	mac.Write([]byte(class))
	mac.Write([]byte{0})
	mac.Write([]byte(client.UserAgent))
	return "v1:" + hex.EncodeToString(mac.Sum(nil)[:16])
}

// ClassifyReuse compares the mark stored for a reused jti with the client
// presenting it now. It is unknown when the store cannot read, the read
// fails or misses, the stored value is not a mark (a legacy "1"), or the
// current address does not parse; never a verdict it cannot support.
func (s *JWTService) ClassifyReuse(ctx context.Context, jti string, client Client) string {
	if s.revoker == nil || jti == "" {
		return ClientMatchUnknown
	}
	stored, ok := s.revoker.readJTI(ctx, jti)
	if !ok || !clientMarkRE.MatchString(stored) {
		return ClientMatchUnknown
	}
	current := s.clientMark(jti, client)
	if !clientMarkRE.MatchString(current) {
		return ClientMatchUnknown
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(current)) == 1 {
		return ClientMatchSame
	}
	return ClientMatchDifferent
}

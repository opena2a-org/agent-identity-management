package config

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SECURITY: SHA-256 digests of known development-only secrets, and of template
// placeholders long enough to pass the length check, that must never be
// used in any environment. Hash-only storage keeps the plaintexts out of the
// compiled binary and the source-of-record (CWE-798). To verify whether a
// suspect string is on the blocklist during a security review:
//
//	echo -n "<suspect string>" | shasum -a 256
//
// and compare against the digests below. The corresponding plaintexts live only
// in the test file (config_test.go), where they are part of the documented
// negative-test surface, not shipped code.
//
// The list lives in its own file so that .hmaignore can exempt it, and only
// it, from HackMyAgent's AST-CRED-003: the scanner reads a 64-character hex
// digest under a name containing "secret" as a hardcoded secret, though a
// digest opens nothing. config.go stays under every credential check.
var insecureDevSecretDigests = []string{
	"ed6328f5592d88d85070fdf00a39c1c94caf46e9a175e30776c7e95c37e61576",
	"849f4281c67d78f9cabb75eee0297b69e21dcde5e78dc6c75638c4d21deae10b",
	"a1ad7292753ed8898e898b79c0d8b4fe5d5a620323d390371d83b640a6c2d473",
	"e8cdbad8a2a71625ade7cd908b26f6522040a56af09ba9b82387185627c2b596",
	"8413be1bcea1ec20652c60b3c01c6ba7f8a09d3e1c4c11f1d872e737d9a9a4ad",
	"b508342c88497e5bc592721e44ef7b27642741df360a10fde1d48be4b44aefa7",
	"bc43a14beca8866dbece708e622e3733d7580d1ea130403ff658a13eafc38f26",
}

// isKnownDevSecret reports whether s matches any of the hashed dev-secret
// digests. Input is whitespace-trimmed before hashing so that a paste-with-
// trailing-newline doesn't bypass the blocklist; case is preserved on
// purpose so that a legitimate mixed-case secret which happens to share its
// lowercase form with a dev string is not rejected.
func isKnownDevSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	sum := sha256.Sum256([]byte(s))
	got := hex.EncodeToString(sum[:])
	for _, want := range insecureDevSecretDigests {
		if got == want {
			return true
		}
	}
	return false
}

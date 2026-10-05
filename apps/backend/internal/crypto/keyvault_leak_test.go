package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A master key, whether supplied or generated, must not be readable from
// anything the key vault writes: its errors, standard output, standard error,
// or the standard log package (which the default log/slog logger also writes
// through). The tests below look for any run of leakWindow characters of the
// key in each form it takes when printed, so a key ID or prefix fails them as
// well as the whole key.

// leakWindow is the shortest run of a key rendering that counts as a leak. It
// catches a short prefix, while a chance match between a random key and fixed
// message text stays negligible (one in 2^24 per window for hex, 2^36 for
// base64).
const leakWindow = 6

// fixedTestKey returns n bytes (n at most 32) derived from label, so each test
// key is fixed across runs and distinct from the others.
func fixedTestKey(label string, n int) []byte {
	sum := sha256.Sum256([]byte(label))
	return sum[:n]
}

// keyRenderings returns the forms key bytes take when printed: hex in either
// case, standard and URL-safe base64, and the decimal byte list that fmt and
// log print for a []byte.
func keyRenderings(key []byte) []string {
	h := hex.EncodeToString(key)
	return []string{
		h,
		strings.ToUpper(h),
		base64.StdEncoding.EncodeToString(key),
		base64.URLEncoding.EncodeToString(key),
		strings.Trim(fmt.Sprint(key), "[]"),
	}
}

// assertNoKeyFragment fails if any leakWindow-long run of a rendering appears
// in text.
func assertNoKeyFragment(t *testing.T, text string, renderings []string) {
	t.Helper()
	for _, r := range renderings {
		for i := 0; i+leakWindow <= len(r); i++ {
			if frag := r[i : i+leakWindow]; strings.Contains(text, frag) {
				t.Errorf("%q from key rendering %q appears in:\n%s", frag, r, text)
				return
			}
		}
	}
}

// redirectFile points *f at a pipe and returns a function that restores *f and
// returns everything written to the pipe.
func redirectFile(t *testing.T, f **os.File) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := *f
	*f = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		done <- buf.String()
	}()
	return func() string {
		*f = orig
		_ = w.Close()
		return <-done
	}
}

// captureKeyVaultOutput runs fn and returns what it wrote to standard output,
// standard error and the standard log package.
func captureKeyVaultOutput(t *testing.T, fn func()) string {
	t.Helper()
	var logged bytes.Buffer
	prevWriter, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	}()

	readStdout := redirectFile(t, &os.Stdout)
	readStderr := redirectFile(t, &os.Stderr)
	fn()
	stderr := readStderr()
	stdout := readStdout()
	return stdout + stderr + logged.String()
}

// The leak tests pass vacuously if the capture misses a channel, so check it
// sees each one the key vault could write to.
func TestCaptureKeyVaultOutput_SeesEveryChannel(t *testing.T) {
	out := captureKeyVaultOutput(t, func() {
		fmt.Println("to-stdout")
		fmt.Fprintln(os.Stderr, "to-stderr")
		log.Println("to-log")
		slog.Info("to-slog")
	})
	for _, want := range []string{"to-stdout", "to-stderr", "to-log", "to-slog"} {
		assert.Contains(t, out, want)
	}
}

func TestKeyVault_MasterKeyNeverReachesOutput(t *testing.T) {
	supplied := fixedTestKey("supplied master key", 32)
	suppliedBase64 := base64.StdEncoding.EncodeToString(supplied)

	t.Run("generated for development", func(t *testing.T) {
		t.Setenv("KEYVAULT_MASTER_KEY", "")
		t.Setenv("ENVIRONMENT", "development")
		var kv *KeyVault
		var err error
		out := captureKeyVaultOutput(t, func() { kv, err = NewKeyVaultFromEnv() })
		require.NoError(t, err)
		assertNoKeyFragment(t, out, keyRenderings(kv.masterKey))
	})

	t.Run("supplied to NewKeyVault", func(t *testing.T) {
		var err error
		out := captureKeyVaultOutput(t, func() { _, err = NewKeyVault(suppliedBase64) })
		require.NoError(t, err)
		assertNoKeyFragment(t, out, keyRenderings(supplied))
	})

	t.Run("supplied in KEYVAULT_MASTER_KEY", func(t *testing.T) {
		t.Setenv("KEYVAULT_MASTER_KEY", suppliedBase64)
		t.Setenv("ENVIRONMENT", "production")
		var err error
		out := captureKeyVaultOutput(t, func() { _, err = NewKeyVaultFromEnv() })
		require.NoError(t, err)
		assertNoKeyFragment(t, out, keyRenderings(supplied))
	})

	t.Run("rotated from one key to another", func(t *testing.T) {
		current := fixedTestKey("current master key", 32)
		kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(current))
		require.NoError(t, err)
		encrypted, err := kv.EncryptPrivateKey(testAgentID, "agent-private-key")
		require.NoError(t, err)
		out := captureKeyVaultOutput(t, func() { _, err = kv.RotatePrivateKey(testAgentID, encrypted, suppliedBase64) })
		require.NoError(t, err)
		assertNoKeyFragment(t, out, append(keyRenderings(current), keyRenderings(supplied)...))
	})
}

func TestKeyVault_RejectedMasterKeyNeverReachesErrorOrOutput(t *testing.T) {
	wrongLength := fixedTestKey("wrong-length master key", 31)
	allZero := make([]byte, 32)
	undecodable := []byte(base64.StdEncoding.EncodeToString(fixedTestKey("undecodable master key", 32)))
	undecodable[20] = '*'

	values := []struct {
		name       string
		value      string
		renderings []string
		wantErr    string
	}{
		{
			name:       "undecodable",
			value:      string(undecodable),
			renderings: []string{string(undecodable)},
			wantErr:    "failed to decode master key",
		},
		{
			// keyRenderings includes the standard base64 value supplied.
			name:       "wrong length",
			value:      base64.StdEncoding.EncodeToString(wrongLength),
			renderings: keyRenderings(wrongLength),
			wantErr:    "master key must be 32 bytes",
		},
		{
			// Decodes and has the right length, so only the content check
			// refuses it.
			name:       "all zero bytes",
			value:      base64.StdEncoding.EncodeToString(allZero),
			renderings: keyRenderings(allZero),
			wantErr:    "master key must not be all zero bytes",
		},
	}

	// Each entry point prepares its state and returns the call that rejects
	// the value.
	entryPoints := []struct {
		name    string
		prepare func(t *testing.T, value string) func() error
	}{
		{
			name: "NewKeyVault",
			prepare: func(t *testing.T, value string) func() error {
				return func() error { _, err := NewKeyVault(value); return err }
			},
		},
		{
			name: "KEYVAULT_MASTER_KEY",
			prepare: func(t *testing.T, value string) func() error {
				t.Setenv("KEYVAULT_MASTER_KEY", value)
				t.Setenv("ENVIRONMENT", "production")
				return func() error { _, err := NewKeyVaultFromEnv(); return err }
			},
		},
		{
			name: "RotatePrivateKey",
			prepare: func(t *testing.T, value string) func() error {
				kv, err := NewKeyVault(base64.StdEncoding.EncodeToString(fixedTestKey("current master key", 32)))
				require.NoError(t, err)
				encrypted, err := kv.EncryptPrivateKey(testAgentID, "agent-private-key")
				require.NoError(t, err)
				return func() error { _, err := kv.RotatePrivateKey(testAgentID, encrypted, value); return err }
			},
		},
	}

	for _, v := range values {
		for _, ep := range entryPoints {
			t.Run(v.name+" via "+ep.name, func(t *testing.T) {
				call := ep.prepare(t, v.value)
				var err error
				out := captureKeyVaultOutput(t, func() { err = call() })
				require.Error(t, err)
				assert.Contains(t, err.Error(), v.wantErr)
				assertNoKeyFragment(t, err.Error()+"\n"+out, v.renderings)
			})
		}
	}
}

package auth

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// An SDK-download token family lives at most SDK_TOKEN_FAMILY_MAX_AGE from the
// download, however often it rotates. The value is read once at boot beside
// JWT_SESSION_MAX_AGE: unset, unreadable or non-positive takes the default of
// 8760h (365 days) with a warning, the value in force is logged, and no value
// switches the bound off.

func sdkAgeLogCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

func sdkAgeService(t *testing.T, value string) (*JWTService, *bytes.Buffer) {
	t.Helper()
	logs := sdkAgeLogCapture(t)
	t.Setenv("JWT_SECRET", familyTestSecret)
	t.Setenv("SDK_TOKEN_FAMILY_MAX_AGE", value)
	return NewJWTService(), logs
}

// G1: unset, the bound is 365 days, logged at boot without a warning.
func TestSDKFamilyAge_DefaultIs365Days(t *testing.T) {
	svc, logs := sdkAgeService(t, "")
	assert.Equal(t, 8760*time.Hour, svc.SDKTokenFamilyMaxAge())
	assert.Contains(t, logs.String(), "SDK-download token families live at most 8760h0m0s (SDK_TOKEN_FAMILY_MAX_AGE)")
	assert.NotContains(t, logs.String(), "WARNING: SDK_TOKEN_FAMILY_MAX_AGE")
}

// G2: a value that is not a positive duration warns and takes the default.
func TestSDKFamilyAge_UnreadableOrNonPositiveValueWarnsAndUsesTheDefault(t *testing.T) {
	for _, value := range []string{"soon", "-1h", "0", "365"} {
		t.Run(value, func(t *testing.T) {
			svc, logs := sdkAgeService(t, value)
			assert.Equal(t, 8760*time.Hour, svc.SDKTokenFamilyMaxAge())
			assert.Contains(t, logs.String(), "WARNING: SDK_TOKEN_FAMILY_MAX_AGE must be a positive duration, using default 8760h")
			assert.Contains(t, logs.String(), "live at most 8760h0m0s")
		})
	}
}

// G3: a positive duration is the bound, with no floor.
func TestSDKFamilyAge_ValueFromEnvironment(t *testing.T) {
	for value, want := range map[string]time.Duration{"1h": time.Hour, "720h": 720 * time.Hour, "30s": 30 * time.Second} {
		t.Run(value, func(t *testing.T) {
			svc, logs := sdkAgeService(t, value)
			assert.Equal(t, want, svc.SDKTokenFamilyMaxAge())
			assert.NotContains(t, logs.String(), "WARNING: SDK_TOKEN_FAMILY_MAX_AGE")
			assert.Contains(t, logs.String(), "live at most "+want.String())
		})
	}
}

// G4: a service built without the constructor (a handler test's struct
// literal) reports the default rather than no bound.
func TestSDKFamilyAge_ZeroServiceReportsTheDefault(t *testing.T) {
	assert.Equal(t, 8760*time.Hour, (&JWTService{}).SDKTokenFamilyMaxAge())
}

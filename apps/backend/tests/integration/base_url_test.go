package integration

import (
	"testing"
)

// The suite reads one address variable, BACKEND_URL. TEST_BASE_URL is read
// only when BACKEND_URL is unset. Both the package-level helpers (getBaseURL)
// and the TestContext helpers (GetTestConfig) must resolve the same address,
// so these tests need no running backend.

func TestBaseURL_BackendURLIsReadByEveryHelper(t *testing.T) {
	t.Setenv("BACKEND_URL", "http://backend-url.invalid:18080")
	t.Setenv("TEST_BASE_URL", "http://test-base-url.invalid:18081")

	if got := getBaseURL(); got != "http://backend-url.invalid:18080" {
		t.Errorf("getBaseURL() = %q, want the BACKEND_URL value", got)
	}
	if got := GetTestConfig().BaseURL; got != "http://backend-url.invalid:18080" {
		t.Errorf("GetTestConfig().BaseURL = %q, want the BACKEND_URL value", got)
	}
}

func TestBaseURL_TestBaseURLIsReadWhenBackendURLIsUnset(t *testing.T) {
	t.Setenv("BACKEND_URL", "")
	t.Setenv("TEST_BASE_URL", "http://test-base-url.invalid:18081")

	if got := getBaseURL(); got != "http://test-base-url.invalid:18081" {
		t.Errorf("getBaseURL() = %q, want the TEST_BASE_URL value", got)
	}
	if got := GetTestConfig().BaseURL; got != "http://test-base-url.invalid:18081" {
		t.Errorf("GetTestConfig().BaseURL = %q, want the TEST_BASE_URL value", got)
	}
}

func TestBaseURL_DefaultsToLocalhost8080(t *testing.T) {
	t.Setenv("BACKEND_URL", "")
	t.Setenv("TEST_BASE_URL", "")

	if got := getBaseURL(); got != "http://localhost:8080" {
		t.Errorf("getBaseURL() = %q, want http://localhost:8080", got)
	}
	if got := GetTestConfig().BaseURL; got != "http://localhost:8080" {
		t.Errorf("GetTestConfig().BaseURL = %q, want http://localhost:8080", got)
	}
}

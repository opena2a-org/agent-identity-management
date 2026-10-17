package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// noContainerMarkers points containerMarkerFiles at paths that do not exist
// and unsets KUBERNETES_SERVICE_HOST for the length of the test, so
// RunningInContainer reports false wherever the test runs.
func noContainerMarkers(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	markers := []string{filepath.Join(dir, ".dockerenv"), filepath.Join(dir, ".containerenv")}
	saved := containerMarkerFiles
	containerMarkerFiles = markers
	t.Cleanup(func() { containerMarkerFiles = saved })
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	os.Unsetenv("KUBERNETES_SERVICE_HOST")
	return markers
}

func TestRunningInContainer(t *testing.T) {
	t.Run("no marker file and KUBERNETES_SERVICE_HOST unset", func(t *testing.T) {
		noContainerMarkers(t)
		if RunningInContainer() {
			t.Fatal("RunningInContainer() = true, want false")
		}
	})
	t.Run("KUBERNETES_SERVICE_HOST set to the empty string", func(t *testing.T) {
		noContainerMarkers(t)
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if RunningInContainer() {
			t.Fatal("RunningInContainer() = true, want false")
		}
	})
	t.Run("KUBERNETES_SERVICE_HOST set", func(t *testing.T) {
		noContainerMarkers(t)
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
		if !RunningInContainer() {
			t.Fatal("RunningInContainer() = false, want true")
		}
	})
	for i, name := range []string{"/.dockerenv", "/run/.containerenv"} {
		t.Run(name+" exists", func(t *testing.T) {
			markers := noContainerMarkers(t)
			if err := os.WriteFile(markers[i], nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if !RunningInContainer() {
				t.Fatal("RunningInContainer() = false, want true")
			}
		})
	}
	if got := strings.Join(containerMarkerFiles, " "); got != "/.dockerenv /run/.containerenv" {
		t.Fatalf("containerMarkerFiles = %q, want /.dockerenv /run/.containerenv", got)
	}
}

func TestListenHost(t *testing.T) {
	cases := []struct {
		name        string
		set         bool
		value       string
		inContainer bool
		want        string
	}{
		{name: "unset outside a container", want: "127.0.0.1"},
		{name: "unset in a container", inContainer: true, want: "0.0.0.0"},
		{name: "set to the empty string outside a container", set: true, want: "127.0.0.1"},
		{name: "set to the empty string in a container", set: true, inContainer: true, want: "0.0.0.0"},
		{name: "set outside a container", set: true, value: "0.0.0.0", want: "0.0.0.0"},
		{name: "set in a container", set: true, value: "127.0.0.1", inContainer: true, want: "127.0.0.1"},
		{name: "set to an IPv6 literal", set: true, value: "::1", want: "::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOST", tc.value)
			if !tc.set {
				os.Unsetenv("HOST")
			}
			inContainer := tc.inContainer
			if got := ListenHost(func() bool { return inContainer }); got != tc.want {
				t.Fatalf("ListenHost() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLoad_Host pins Load's Server.Host: HOST when set, and 127.0.0.1 when it
// is unset and no container is detected.
func TestLoad_Host(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "aim")
	t.Setenv("POSTGRES_DB", "aim")
	t.Setenv("JWT_SECRET", strings.Repeat("a", 64))
	markers := noContainerMarkers(t)

	t.Setenv("HOST", "")
	os.Unsetenv("HOST")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with HOST unset: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host with HOST unset outside a container = %q, want 127.0.0.1", cfg.Server.Host)
	}

	if err := os.WriteFile(markers[0], nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with HOST unset in a container: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("Server.Host with HOST unset in a container = %q, want 0.0.0.0", cfg.Server.Host)
	}

	t.Setenv("HOST", "::1")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with HOST=::1: %v", err)
	}
	if cfg.Server.Host != "::1" {
		t.Fatalf("Server.Host with HOST=::1 = %q, want ::1", cfg.Server.Host)
	}
}

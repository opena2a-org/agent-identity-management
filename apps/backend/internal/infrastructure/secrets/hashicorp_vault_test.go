package secrets

import (
	"encoding/json"
	"testing"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/google/uuid"
)

func TestVaultConfigFromMap(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		m := map[string]interface{}{
			"address":   "https://vault.example.com:8200",
			"mountPath": "kv",
			"role":      "aim-role",
		}
		cfg, err := VaultConfigFromMap(m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Address != "https://vault.example.com:8200" {
			t.Errorf("address = %q, want %q", cfg.Address, "https://vault.example.com:8200")
		}
		if cfg.MountPath != "kv" {
			t.Errorf("mountPath = %q, want %q", cfg.MountPath, "kv")
		}
		if cfg.AuthPath != "jwt" {
			t.Errorf("authPath = %q, want %q (default)", cfg.AuthPath, "jwt")
		}
	})

	t.Run("defaults applied", func(t *testing.T) {
		m := map[string]interface{}{
			"address": "https://vault.example.com",
		}
		cfg, err := VaultConfigFromMap(m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MountPath != "secret" {
			t.Errorf("mountPath = %q, want %q (default)", cfg.MountPath, "secret")
		}
		if cfg.AuthPath != "jwt" {
			t.Errorf("authPath = %q, want %q (default)", cfg.AuthPath, "jwt")
		}
	})

	t.Run("missing address", func(t *testing.T) {
		m := map[string]interface{}{
			"mountPath": "kv",
		}
		_, err := VaultConfigFromMap(m)
		if err == nil {
			t.Fatal("expected error for missing address")
		}
	})
}

func TestHashiCorpVaultBackendType(t *testing.T) {
	b := &HashiCorpVaultBackend{mountPath: "secret"}
	if b.Type() != domainsecrets.BackendTypeVault {
		t.Errorf("Type() = %q, want %q", b.Type(), domainsecrets.BackendTypeVault)
	}
}

func TestHashiCorpVaultBackendSecretPath(t *testing.T) {
	b := &HashiCorpVaultBackend{mountPath: "secret"}
	id := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	path := b.secretPath(id)
	expected := "aim-secrets/12345678-1234-1234-1234-123456789abc"
	if path != expected {
		t.Errorf("secretPath() = %q, want %q", path, expected)
	}
}

func TestInterfaceToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    []byte
		wantErr bool
	}{
		{"byte slice", []byte{1, 2, 3}, []byte{1, 2, 3}, false},
		{"string", "hello", []byte("hello"), false},
		{"float64 array", []interface{}{float64(65), float64(66)}, []byte{65, 66}, false},
		{"json number array", []interface{}{json.Number("65"), json.Number("66")}, []byte{65, 66}, false},
		{"nil", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interfaceToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("interfaceToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != string(tt.want) {
				t.Errorf("interfaceToBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInterfaceToInt(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{"float64", float64(42), 42},
		{"json number", json.Number("7"), 7},
		{"int", 3, 3},
		{"string", "5", 5},
		{"unknown", struct{}{}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interfaceToInt(tt.input)
			if got != tt.want {
				t.Errorf("interfaceToInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ===========================
// PostgresConfig Tests
// ===========================

func TestPostgresConfig_NewPostgresConfig_WithEnvVars(t *testing.T) {
	// Set required environment variables
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_USER", "testuser")
	t.Setenv("POSTGRES_PASSWORD", "testpass")

	config := NewPostgresConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, "5432", config.Port) // default
	assert.Equal(t, "testdb", config.Database)
	assert.Equal(t, "testuser", config.User)
	assert.Equal(t, "testpass", config.Password)
	assert.Equal(t, "disable", config.SSLMode) // default
	assert.Equal(t, 100, config.MaxConnections)
	assert.Equal(t, 5*time.Minute, config.ConnMaxLifetime)
}

func TestPostgresConfig_NewPostgresConfig_WithCustomPort(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_USER", "testuser")
	t.Setenv("POSTGRES_PASSWORD", "testpass")

	config := NewPostgresConfig()

	assert.Equal(t, "5433", config.Port)
}

func TestPostgresConfig_NewPostgresConfig_WithCustomSSLMode(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_USER", "testuser")
	t.Setenv("POSTGRES_PASSWORD", "testpass")
	t.Setenv("POSTGRES_SSL_MODE", "require")

	config := NewPostgresConfig()

	assert.Equal(t, "require", config.SSLMode)
}

func TestPostgresConfig_PanicsWithoutHost(t *testing.T) {
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_USER", "testuser")
	t.Setenv("POSTGRES_PASSWORD", "testpass")

	assert.Panics(t, func() {
		NewPostgresConfig()
	})
}

func TestPostgresConfig_PanicsWithoutDB(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "testuser")
	t.Setenv("POSTGRES_PASSWORD", "testpass")

	assert.Panics(t, func() {
		NewPostgresConfig()
	})
}

func TestPostgresConfig_PanicsWithoutUser(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_PASSWORD", "testpass")

	assert.Panics(t, func() {
		NewPostgresConfig()
	})
}

func TestPostgresConfig_PanicsWithoutPassword(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_DB", "testdb")
	t.Setenv("POSTGRES_USER", "testuser")

	assert.Panics(t, func() {
		NewPostgresConfig()
	})
}

// ===========================
// getEnv Tests
// ===========================

func TestGetEnv_WithValue(t *testing.T) {
	t.Setenv("TEST_VAR", "test_value")

	result := getEnv("TEST_VAR", "default")

	assert.Equal(t, "test_value", result)
}

func TestGetEnv_WithFallback(t *testing.T) {
	// TEST_VAR_MISSING should not be set

	result := getEnv("TEST_VAR_MISSING", "fallback_value")

	assert.Equal(t, "fallback_value", result)
}

func TestGetEnv_EmptyValue(t *testing.T) {
	t.Setenv("TEST_EMPTY_VAR", "")

	result := getEnv("TEST_EMPTY_VAR", "fallback")

	assert.Equal(t, "fallback", result)
}

// ===========================
// getEnvRequired Tests
// ===========================

func TestGetEnvRequired_WithValue(t *testing.T) {
	t.Setenv("REQUIRED_VAR", "required_value")

	result := getEnvRequired("REQUIRED_VAR")

	assert.Equal(t, "required_value", result)
}

func TestGetEnvRequired_PanicsWhenMissing(t *testing.T) {
	assert.Panics(t, func() {
		getEnvRequired("MISSING_REQUIRED_VAR")
	})
}

// ===========================
// Connect Tests (without actual DB)
// ===========================

func TestConnect_InvalidHost(t *testing.T) {
	config := &PostgresConfig{
		Host:            "invalid-host-that-does-not-exist",
		Port:            "5432",
		Database:        "testdb",
		User:            "testuser",
		Password:        "testpass",
		SSLMode:         "disable",
		MaxConnections:  10,
		ConnMaxLifetime: time.Minute,
	}

	db, err := Connect(config)

	// Should fail to connect (either open or ping will fail)
	if db != nil {
		defer db.Close()
	}
	assert.Error(t, err)
}

func TestConnect_InvalidPort(t *testing.T) {
	config := &PostgresConfig{
		Host:            "localhost",
		Port:            "99999", // invalid port
		Database:        "testdb",
		User:            "testuser",
		Password:        "testpass",
		SSLMode:         "disable",
		MaxConnections:  10,
		ConnMaxLifetime: time.Minute,
	}

	db, err := Connect(config)

	if db != nil {
		defer db.Close()
	}
	assert.Error(t, err)
}

// ===========================
// PostgresConfig Fields Tests
// ===========================

func TestPostgresConfig_DefaultValues(t *testing.T) {
	config := &PostgresConfig{}

	// Check zero values
	assert.Empty(t, config.Host)
	assert.Empty(t, config.Port)
	assert.Empty(t, config.Database)
	assert.Empty(t, config.User)
	assert.Empty(t, config.Password)
	assert.Empty(t, config.SSLMode)
	assert.Equal(t, 0, config.MaxConnections)
	assert.Equal(t, time.Duration(0), config.ConnMaxLifetime)
}

func TestPostgresConfig_CustomValues(t *testing.T) {
	config := &PostgresConfig{
		Host:            "db.example.com",
		Port:            "5433",
		Database:        "production",
		User:            "admin",
		Password:        "secret",
		SSLMode:         "verify-full",
		MaxConnections:  200,
		ConnMaxLifetime: 10 * time.Minute,
	}

	assert.Equal(t, "db.example.com", config.Host)
	assert.Equal(t, "5433", config.Port)
	assert.Equal(t, "production", config.Database)
	assert.Equal(t, "admin", config.User)
	assert.Equal(t, "secret", config.Password)
	assert.Equal(t, "verify-full", config.SSLMode)
	assert.Equal(t, 200, config.MaxConnections)
	assert.Equal(t, 10*time.Minute, config.ConnMaxLifetime)
}

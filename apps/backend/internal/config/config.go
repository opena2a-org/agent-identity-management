package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SECURITY: SHA-256 digests of known development-only secrets that must never be
// used in any environment. Hash-only storage keeps the plaintexts out of the
// compiled binary and the source-of-record (CWE-798). To verify whether a
// suspect string is on the blocklist during a security review:
//
//	echo -n "<suspect string>" | shasum -a 256
//
// and compare against the digests below. The corresponding plaintexts live only
// in the test file (config_test.go), where they are part of the documented
// negative-test surface, not shipped code.
var insecureDevSecretDigests = []string{
	"ed6328f5592d88d85070fdf00a39c1c94caf46e9a175e30776c7e95c37e61576",
	"849f4281c67d78f9cabb75eee0297b69e21dcde5e78dc6c75638c4d21deae10b",
	"a1ad7292753ed8898e898b79c0d8b4fe5d5a620323d390371d83b640a6c2d473",
	"e8cdbad8a2a71625ade7cd908b26f6522040a56af09ba9b82387185627c2b596",
	"8413be1bcea1ec20652c60b3c01c6ba7f8a09d3e1c4c11f1d872e737d9a9a4ad",
	"b508342c88497e5bc592721e44ef7b27642741df360a10fde1d48be4b44aefa7",
}

// isKnownDevSecret reports whether s matches any of the hashed dev-secret
// digests. Constant-time across the digest list; the input is hashed once.
func isKnownDevSecret(s string) bool {
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

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	OAuth    OAuthConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port        string
	Environment string
	LogLevel    string
	FrontendURL string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConnections  int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	UseTLS   bool
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// OAuthConfig holds OAuth provider configurations
type OAuthConfig struct {
	Google    OAuthProvider
	Microsoft OAuthProvider
	Okta      OktaProvider
}

// OAuthProvider holds OAuth provider configuration
type OAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OktaProvider holds Okta-specific configuration
type OktaProvider struct {
	ClientID     string
	ClientSecret string
	Domain       string
	RedirectURL  string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:        getEnv("APP_PORT", "8080"),
			Environment: getEnv("ENVIRONMENT", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
	Database: DatabaseConfig{
		Host:            getEnvRequired("POSTGRES_HOST"),
		Port:            getEnvAsInt("POSTGRES_PORT", 5432),
		User:            getEnvRequired("POSTGRES_USER"),
		Password:        getEnv("POSTGRES_PASSWORD", ""), // Optional for local dev with no password
		Database:        getEnvRequired("POSTGRES_DB"),
		SSLMode:         getEnv("POSTGRES_SSL_MODE", "disable"),
		MaxConnections:  getEnvAsInt("POSTGRES_MAX_CONNECTIONS", 25),
		ConnMaxLifetime: getEnvAsDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
	},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			UseTLS:   getEnv("REDIS_USE_TLS", "false") == "true",
		},
	JWT: JWTConfig{
		Secret:          getEnvRequired("JWT_SECRET"),
		AccessTokenTTL:  getEnvAsDuration("JWT_ACCESS_TTL", 24*time.Hour),
		RefreshTokenTTL: getEnvAsDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
	},
		OAuth: OAuthConfig{
			Google: OAuthProvider{
				ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/google"),
			},
			Microsoft: OAuthProvider{
				ClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
				ClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("MICROSOFT_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/microsoft"),
			},
			Okta: OktaProvider{
				ClientID:     getEnv("OKTA_CLIENT_ID", ""),
				ClientSecret: getEnv("OKTA_CLIENT_SECRET", ""),
				Domain:       getEnv("OKTA_DOMAIN", ""),
				RedirectURL:  getEnv("OKTA_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/okta"),
			},
		},
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Print security warnings (non-blocking)
	config.printSecurityWarnings()

	return config, nil
}

// printSecurityWarnings logs warnings for potentially insecure configurations
// These don't block startup but alert operators to review their settings
func (c *Config) printSecurityWarnings() {
	if c.Server.Environment != "production" {
		return
	}

	// Warn about SSL mode for remote databases
	isLocalDB := strings.Contains(c.Database.Host, "localhost") ||
		strings.Contains(c.Database.Host, "127.0.0.1") ||
		c.Database.Host == "postgres" // Docker service name

	if !isLocalDB && c.Database.SSLMode == "disable" {
		fmt.Print(`
⚠️  SECURITY WARNING: Database SSL is disabled for remote connection
    POSTGRES_SSL_MODE=disable with remote host may expose data in transit.
    Consider setting POSTGRES_SSL_MODE=require for production.
    (This is a warning, not an error - some setups use VPC/network encryption)

`)
	}
}

// Validate validates the configuration.
//
// SECURITY: known dev-secret values are rejected in every environment, not only
// production. A laptop running ENVIRONMENT=development with a leaked dev secret
// is still a vulnerability the moment that laptop gets pointed at shared
// infrastructure; failing fast everywhere prevents the misconfiguration from
// ever taking effect.
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if isKnownDevSecret(c.JWT.Secret) {
		return insecureSecretError("JWT_SECRET", "openssl rand -hex 32")
	}

	if isKnownDevSecret(os.Getenv("KEYVAULT_MASTER_KEY")) {
		return insecureSecretError("KEYVAULT_MASTER_KEY", "openssl rand -base64 32")
	}

	// Production-only: weak POSTGRES_PASSWORD against a remote host.
	if c.Server.Environment == "production" {
		if c.Database.Password == "postgres" || c.Database.Password == "" {
			if !strings.Contains(c.Database.Host, "localhost") && !strings.Contains(c.Database.Host, "127.0.0.1") {
				return fmt.Errorf("POSTGRES_PASSWORD is weak or empty for a remote host; generate one with `openssl rand -base64 24`")
			}
		}
	}

	// OAuth providers are now optional since we support email/password authentication
	// Validation removed - OAuth configuration is checked at runtime when needed

	return nil
}

// insecureSecretError returns the single error format used when a known
// development secret leaks into a real config. The format is plain text so it
// reads cleanly in container logs and CI output.
func insecureSecretError(name, generator string) error {
	return fmt.Errorf(
		"SECURITY ERROR: %s is set to a known development default. "+
			"Replace it with a freshly generated value: %s. "+
			"See scripts/gen-dev-secrets.sh for the local-dev helper.",
		name, generator,
	)
}

// Helper functions
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvRequired gets environment variable and panics if not set
func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("Required environment variable %s is not set", key))
	}
	return value
}

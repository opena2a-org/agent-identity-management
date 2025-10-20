package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

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
		Password:        getEnvRequired("POSTGRES_PASSWORD"),
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

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	// OAuth providers are now optional since we support email/password authentication
	// Validation removed - OAuth configuration is checked at runtime when needed

	return nil
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

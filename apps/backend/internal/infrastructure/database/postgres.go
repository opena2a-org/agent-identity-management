package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConfig holds database configuration
type PostgresConfig struct {
	Host            string
	Port            string
	Database        string
	User            string
	Password        string
	SSLMode         string
	MaxConnections  int
	ConnMaxLifetime time.Duration
}

// NewPostgresConfig creates config from environment variables
func NewPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		Host:            getEnvRequired("POSTGRES_HOST"),
		Port:            getEnv("POSTGRES_PORT", "5432"),
		Database:        getEnvRequired("POSTGRES_DB"),
		User:            getEnvRequired("POSTGRES_USER"),
		Password:        getEnvRequired("POSTGRES_PASSWORD"),
		SSLMode:         getEnv("POSTGRES_SSL_MODE", "disable"),
		MaxConnections:  100,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// getEnvRequired gets environment variable and panics if not set
func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("Required environment variable " + key + " is not set")
	}
	return value
}

// Connect establishes a connection to PostgreSQL
func Connect(config *PostgresConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Database,
		config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxConnections / 2)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

package main

import (
	"log"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
)

// cyberArkCCPBackend returns the secrets backend that reads a namespace's
// credential from CyberArk through the Central Credential Provider, built from
// the CYBERARK_CCP_* variables, and nil when CYBERARK_CCP_URL is not set. When
// a setting keeps the backend from being built, the server logs a warning and
// starts without it, as it does for the other secrets backends, and refuses a
// resolve request for a namespace on it.
func cyberArkCCPBackend(c config.CyberArkCCPConfig) *infrasecrets.CyberArkCCPBackend {
	if c.URL == "" {
		return nil
	}
	backend, err := infrasecrets.NewCyberArkCCPBackend(&infrasecrets.CyberArkCCPConfig{
		CCPURL:     c.URL,
		AppID:      c.AppID,
		Safe:       c.Safe,
		CertPath:   c.CertPath,
		KeyPath:    c.KeyPath,
		CACertPath: c.CACertPath,
	})
	if err != nil {
		log.Printf("WARNING: CyberArk CCP backend configured but failed to initialize: %v", err)
		return nil
	}
	log.Printf("CyberArk CCP backend registered at %s for safe %q", c.URL, c.Safe)
	return backend
}

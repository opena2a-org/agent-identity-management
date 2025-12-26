package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===========================
// NewSecurityHandler Tests
// ===========================

func TestNewSecurityHandler_NilDeps(t *testing.T) {
	handler := NewSecurityHandler(nil, nil, nil, nil, nil)
	assert.NotNil(t, handler)
}

// Note: All SecurityHandler methods use unsafe type assertions for orgID:
//   orgID := c.Locals("organization_id").(uuid.UUID)
// This means they will panic if no org context is set, but will hit nil
// services if org context is set. Tests for these handlers would require
// mocked services to be meaningful.

// ===========================
// SecurityHandler Service Getter Tests
// ===========================

func TestSecurityHandler_getSecurityService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &SecurityHandler{}
	result := handler.getSecurityService()
	assert.Nil(t, result)
}

func TestSecurityHandler_getCapabilityService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &SecurityHandler{}
	result := handler.getCapabilityService()
	assert.Nil(t, result)
}

func TestSecurityHandler_getAlertService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &SecurityHandler{}
	result := handler.getAlertService()
	assert.Nil(t, result)
}

func TestSecurityHandler_getAgentService_NilInterfaceReturnsNil(t *testing.T) {
	handler := &SecurityHandler{}
	result := handler.getAgentService()
	assert.Nil(t, result)
}

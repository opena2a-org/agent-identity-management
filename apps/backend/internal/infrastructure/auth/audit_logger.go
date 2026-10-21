package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AuditLogger writes one audit log entry. *application.AuditService satisfies
// it.
type AuditLogger interface {
	LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
}

// SetAuditLogger attaches the audit log AuthMiddleware records a refusal in:
// a request it refuses at a credential route because the access token's
// session (token family) was revoked is recorded as credential_mint_refused.
// The server attaches its audit service; a JWTService without one (a test
// that builds the middleware alone) leaves the middleware logging the
// refusal's SECURITY line only.
func (s *JWTService) SetAuditLogger(a AuditLogger) {
	s.auditLogger = a
}

// AuditLogger returns the attached audit log, nil when none is attached.
func (s *JWTService) AuditLogger() AuditLogger {
	if s == nil {
		return nil
	}
	return s.auditLogger
}

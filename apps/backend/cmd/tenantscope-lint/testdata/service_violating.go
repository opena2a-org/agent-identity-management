// Test fixture for the class-#3 service-param scan: a service method
// that accepts orgID uuid.UUID but NEVER references it in the body.
// The lint MUST flag this method (the downstream repo call likely
// runs WHERE id = $1 system-wide and is a cross-tenant IDOR).
//
//go:build ignore_lint_testdata

package testdata

import (
	"context"

	"github.com/google/uuid"
)

type LintTestViolatingService struct {
	repo interface {
		AcknowledgeByID(id uuid.UUID) error
	}
}

// AcknowledgeAlert accepts orgID but never references it. The repo
// call `AcknowledgeByID(alertID)` runs system-wide. The lint MUST
// flag this.
func (s *LintTestViolatingService) AcknowledgeAlert(ctx context.Context, alertID uuid.UUID, orgID uuid.UUID, userID uuid.UUID) error {
	// Note: no reference to `orgID` anywhere below this line.
	return s.repo.AcknowledgeByID(alertID)
}

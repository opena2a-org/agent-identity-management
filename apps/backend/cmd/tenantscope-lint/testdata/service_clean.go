// Test fixture for the class-#3 service-param scan: a service method
// that accepts orgID uuid.UUID AND uses it in the body. The lint
// must NOT flag this method.
//
//go:build ignore_lint_testdata

package testdata

import (
	"context"

	"github.com/google/uuid"
)

type LintTestCleanService struct {
	repo interface {
		AcknowledgeByOrg(id, orgID uuid.UUID) error
	}
}

// AcknowledgeAlert uses orgID in the repository call. Body references
// the param, so the lint passes.
func (s *LintTestCleanService) AcknowledgeAlert(ctx context.Context, alertID uuid.UUID, orgID uuid.UUID) error {
	return s.repo.AcknowledgeByOrg(alertID, orgID)
}

// CheckByOrg uses organizationID (alternate spelling) in a conditional;
// any reference in the body counts as "used."
func (s *LintTestCleanService) CheckByOrg(ctx context.Context, alertID uuid.UUID, organizationID uuid.UUID) error {
	if organizationID == uuid.Nil {
		return nil
	}
	return s.repo.AcknowledgeByOrg(alertID, organizationID)
}

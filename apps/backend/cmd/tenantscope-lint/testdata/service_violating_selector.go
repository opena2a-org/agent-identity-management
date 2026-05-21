// Test fixture for the class-#3 service-param scan, H1 regression case:
// a service method that accepts orgID uuid.UUID, never references it
// at the top level, but contains a selector expression
// `s.repo.OrganizationID` that mentions a STRUCT FIELD with the same
// name. A naive ast.Inspect walk would incorrectly count the selector
// RHS as a param match and pass the method. The lint MUST flag this
// method.
//
//go:build ignore_lint_testdata

package testdata

import (
	"context"

	"github.com/google/uuid"
)

// Resource is a mock domain type with an OrganizationID field. The
// field name deliberately matches one of the orgParamNames in the
// lint so the selector-shadow false-negative can be exercised.
type ResourceWithOrgIDField struct {
	OrganizationID uuid.UUID
}

type LintTestViolatingSelectorService struct {
	repo interface {
		Get() *ResourceWithOrgIDField
		AcknowledgeByID(id uuid.UUID) error
	}
}

// AcknowledgeAlert accepts OrganizationID as a param (PascalCase
// variant) but never references it. The body contains a selector
// `r.OrganizationID` that the lint's identMatchVisitor MUST NOT count
// as a parameter reference.
func (s *LintTestViolatingSelectorService) AcknowledgeAlert(ctx context.Context, alertID uuid.UUID, OrganizationID uuid.UUID) error {
	r := s.repo.Get()
	_ = r.OrganizationID // selector — must NOT satisfy the lint
	return s.repo.AcknowledgeByID(alertID)
}

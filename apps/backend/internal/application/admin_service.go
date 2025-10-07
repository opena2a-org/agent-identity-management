package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AdminService handles administrative operations
type AdminService struct {
	userRepo domain.UserRepository
	orgRepo  domain.OrganizationRepository
}

// NewAdminService creates a new admin service
func NewAdminService(
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
) *AdminService {
	return &AdminService{
		userRepo: userRepo,
		orgRepo:  orgRepo,
	}
}

// GetAllUsers returns all users in admin's organization
func (s *AdminService) GetAllUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
	return s.userRepo.GetByOrganization(adminOrgID)
}

// GetPendingUsers returns users awaiting approval
func (s *AdminService) GetPendingUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error) {
	return s.userRepo.GetByOrganizationAndStatus(adminOrgID, domain.UserStatusPending)
}

// ApproveUser approves a pending user
func (s *AdminService) ApproveUser(ctx context.Context, userID, adminID uuid.UUID) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Status != domain.UserStatusPending {
		return fmt.Errorf("user is not pending approval (status: %s)", user.Status)
	}

	now := time.Now()
	user.Status = domain.UserStatusActive
	user.ApprovedBy = &adminID
	user.ApprovedAt = &now

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to approve user: %w", err)
	}

	return nil
}

// RejectUser rejects a pending user by deleting their account
func (s *AdminService) RejectUser(ctx context.Context, userID, adminID uuid.UUID, reason string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Status != domain.UserStatusPending {
		return fmt.Errorf("user is not pending approval (status: %s)", user.Status)
	}

	// TODO: Log rejection reason in audit log

	// Delete the user
	if err := s.userRepo.Delete(userID); err != nil {
		return fmt.Errorf("failed to reject user: %w", err)
	}

	return nil
}

// UpdateUserRole updates a user's role
func (s *AdminService) UpdateUserRole(ctx context.Context, userID uuid.UUID, role domain.UserRole) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Status != domain.UserStatusActive {
		return fmt.Errorf("cannot change role of non-active user (status: %s)", user.Status)
	}

	if err := s.userRepo.UpdateRole(userID, role); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	return nil
}

// SuspendUser suspends a user account
func (s *AdminService) SuspendUser(ctx context.Context, userID, adminID uuid.UUID) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Status == domain.UserStatusSuspended {
		return fmt.Errorf("user is already suspended")
	}

	user.Status = domain.UserStatusSuspended
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	return nil
}

// ActivateUser activates a suspended user account
func (s *AdminService) ActivateUser(ctx context.Context, userID, adminID uuid.UUID) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Status == domain.UserStatusActive {
		return fmt.Errorf("user is already active")
	}

	user.Status = domain.UserStatusActive
	now := time.Now()
	if user.ApprovedBy == nil {
		user.ApprovedBy = &adminID
		user.ApprovedAt = &now
	}

	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to activate user: %w", err)
	}

	return nil
}

// DeactivateUser permanently deactivates a user account
func (s *AdminService) DeactivateUser(ctx context.Context, userID, adminID uuid.UUID) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	user.Status = domain.UserStatusDeactivated
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	return nil
}

// GetOrganizationSettings retrieves organization settings
func (s *AdminService) GetOrganizationSettings(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error) {
	return s.orgRepo.GetByID(orgID)
}

// UpdateOrganizationSettings updates organization settings
func (s *AdminService) UpdateOrganizationSettings(ctx context.Context, orgID uuid.UUID, autoApproveSSO bool) error {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	org.AutoApproveSSO = autoApproveSSO
	if err := s.orgRepo.Update(org); err != nil {
		return fmt.Errorf("failed to update organization settings: %w", err)
	}

	return nil
}

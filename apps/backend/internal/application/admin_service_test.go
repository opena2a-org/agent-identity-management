package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockUserRepoForAdmin implements UserRepository for admin service tests
type MockUserRepoForAdmin struct {
	users       map[uuid.UUID]*domain.User
	getByIDErr  error
	updateErr   error
	deleteErr   error
	updateRoleErr error
}

func NewMockUserRepoForAdmin() *MockUserRepoForAdmin {
	return &MockUserRepoForAdmin{
		users: make(map[uuid.UUID]*domain.User),
	}
}

func (m *MockUserRepoForAdmin) AddUser(user *domain.User) {
	m.users[user.ID] = user
}

func (m *MockUserRepoForAdmin) Create(user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepoForAdmin) GetByID(id uuid.UUID) (*domain.User, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	user, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *MockUserRepoForAdmin) GetByEmail(email string) (*domain.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *MockUserRepoForAdmin) GetByPasswordResetToken(resetToken string) (*domain.User, error) {
	for _, user := range m.users {
		if user.PasswordResetToken != nil && *user.PasswordResetToken == resetToken {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *MockUserRepoForAdmin) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	var result []*domain.User
	for _, user := range m.users {
		if user.OrganizationID == orgID {
			result = append(result, user)
		}
	}
	return result, nil
}

func (m *MockUserRepoForAdmin) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	var result []*domain.User
	for _, user := range m.users {
		if user.OrganizationID == orgID && user.Status == status {
			result = append(result, user)
		}
	}
	return result, nil
}

func (m *MockUserRepoForAdmin) Update(user *domain.User) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepoForAdmin) UpdateRole(id uuid.UUID, role domain.UserRole) error {
	if m.updateRoleErr != nil {
		return m.updateRoleErr
	}
	user, ok := m.users[id]
	if !ok {
		return errors.New("user not found")
	}
	user.Role = role
	return nil
}

func (m *MockUserRepoForAdmin) Delete(id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.users, id)
	return nil
}

func (m *MockUserRepoForAdmin) CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error) {
	count := 0
	for _, user := range m.users {
		if user.OrganizationID == orgID && user.Status == domain.UserStatusActive {
			count++
		}
	}
	return count, nil
}

// MockOrgRepository implements OrganizationRepository for admin service tests
type MockOrgRepository struct {
	orgs       map[uuid.UUID]*domain.Organization
	getByIDErr error
	updateErr  error
}

func NewMockOrgRepository() *MockOrgRepository {
	return &MockOrgRepository{
		orgs: make(map[uuid.UUID]*domain.Organization),
	}
}

func (m *MockOrgRepository) AddOrg(org *domain.Organization) {
	m.orgs[org.ID] = org
}

func (m *MockOrgRepository) Create(org *domain.Organization) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	m.orgs[org.ID] = org
	return nil
}

func (m *MockOrgRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	org, ok := m.orgs[id]
	if !ok {
		return nil, errors.New("organization not found")
	}
	return org, nil
}

func (m *MockOrgRepository) GetByDomain(domainName string) (*domain.Organization, error) {
	for _, org := range m.orgs {
		if org.Domain == domainName {
			return org, nil
		}
	}
	return nil, errors.New("organization not found")
}

func (m *MockOrgRepository) Update(org *domain.Organization) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.orgs[org.ID] = org
	return nil
}

func (m *MockOrgRepository) Delete(id uuid.UUID) error {
	delete(m.orgs, id)
	return nil
}

// Helper to create test user
func createAdminTestUser(orgID uuid.UUID, status domain.UserStatus) *domain.User {
	return &domain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          "test@example.com",
		Name:           "Test User",
		Role:           domain.RoleMember,
		Status:         status,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func TestNewAdminService(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)

	assert.NotNil(t, service)
}

func TestAdminService_GetAllUsers(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	otherOrgID := uuid.New()

	// Add users to different orgs
	user1 := createAdminTestUser(orgID, domain.UserStatusActive)
	user2 := createAdminTestUser(orgID, domain.UserStatusPending)
	user3 := createAdminTestUser(otherOrgID, domain.UserStatusActive)

	userRepo.AddUser(user1)
	userRepo.AddUser(user2)
	userRepo.AddUser(user3)

	users, err := service.GetAllUsers(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestAdminService_GetPendingUsers(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()

	// Add mix of pending and active users
	pending1 := createAdminTestUser(orgID, domain.UserStatusPending)
	pending2 := createAdminTestUser(orgID, domain.UserStatusPending)
	active := createAdminTestUser(orgID, domain.UserStatusActive)

	userRepo.AddUser(pending1)
	userRepo.AddUser(pending2)
	userRepo.AddUser(active)

	users, err := service.GetPendingUsers(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, users, 2)
	for _, user := range users {
		assert.Equal(t, domain.UserStatusPending, user.Status)
	}
}

func TestAdminService_ApproveUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusPending)
	userRepo.AddUser(user)

	err := service.ApproveUser(ctx, user.ID, adminID)
	require.NoError(t, err)

	// Verify user was approved
	updated, _ := userRepo.GetByID(user.ID)
	assert.Equal(t, domain.UserStatusActive, updated.Status)
	assert.Equal(t, &adminID, updated.ApprovedBy)
	assert.NotNil(t, updated.ApprovedAt)
}

func TestAdminService_ApproveUser_NotPending(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive) // Already active
	userRepo.AddUser(user)

	err := service.ApproveUser(ctx, user.ID, adminID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestAdminService_ApproveUser_NotFound(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	err := service.ApproveUser(ctx, uuid.New(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get user")
}

func TestAdminService_RejectUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusPending)
	userRepo.AddUser(user)

	err := service.RejectUser(ctx, user.ID, adminID, "Does not meet requirements")
	require.NoError(t, err)

	// Verify user was deleted
	_, err = userRepo.GetByID(user.ID)
	assert.Error(t, err)
}

func TestAdminService_RejectUser_NotPending(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.RejectUser(ctx, user.ID, adminID, "reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestAdminService_UpdateUserRole(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	user.Role = domain.RoleMember
	userRepo.AddUser(user)

	err := service.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	require.NoError(t, err)

	updated, _ := userRepo.GetByID(user.ID)
	assert.Equal(t, domain.RoleAdmin, updated.Role)
}

func TestAdminService_UpdateUserRole_NotActive(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusSuspended)
	userRepo.AddUser(user)

	err := service.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-active user")
}

func TestAdminService_SuspendUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.SuspendUser(ctx, user.ID, adminID)
	require.NoError(t, err)

	updated, _ := userRepo.GetByID(user.ID)
	assert.Equal(t, domain.UserStatusSuspended, updated.Status)
}

func TestAdminService_SuspendUser_AlreadySuspended(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusSuspended)
	userRepo.AddUser(user)

	err := service.SuspendUser(ctx, user.ID, adminID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already suspended")
}

func TestAdminService_ActivateUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusSuspended)
	userRepo.AddUser(user)

	err := service.ActivateUser(ctx, user.ID, adminID)
	require.NoError(t, err)

	updated, _ := userRepo.GetByID(user.ID)
	assert.Equal(t, domain.UserStatusActive, updated.Status)
	assert.Nil(t, updated.DeletedAt)
}

func TestAdminService_ActivateUser_AlreadyActive(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.ActivateUser(ctx, user.ID, adminID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")
}

func TestAdminService_DeactivateUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.DeactivateUser(ctx, user.ID, adminID)
	require.NoError(t, err)

	updated, _ := userRepo.GetByID(user.ID)
	assert.Equal(t, domain.UserStatusDeactivated, updated.Status)
	assert.NotNil(t, updated.DeletedAt)
}

func TestAdminService_DeactivateUser_AlreadyDeactivated(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	now := time.Now()
	user := createAdminTestUser(orgID, domain.UserStatusDeactivated)
	user.DeletedAt = &now
	userRepo.AddUser(user)

	err := service.DeactivateUser(ctx, user.ID, adminID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already deactivated")
}

func TestAdminService_PermanentlyDeleteUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	now := time.Now()
	user := createAdminTestUser(orgID, domain.UserStatusDeactivated)
	user.DeletedAt = &now // Already deactivated
	userRepo.AddUser(user)

	err := service.PermanentlyDeleteUser(ctx, user.ID, adminID)
	require.NoError(t, err)

	// Verify user was deleted
	_, err = userRepo.GetByID(user.ID)
	assert.Error(t, err)
}

func TestAdminService_PermanentlyDeleteUser_SelfDeletion(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	// Try to delete self
	err := service.PermanentlyDeleteUser(ctx, user.ID, user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete your own account")
}

func TestAdminService_PermanentlyDeleteUser_ActiveUser(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	adminID := uuid.New()

	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.PermanentlyDeleteUser(ctx, user.ID, adminID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "should be deactivated first")
}

func TestAdminService_GetOrganizationSettings(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Test Org",
		EnforcementMode: domain.EnforcementModeStrict,
	}
	orgRepo.AddOrg(org)

	retrieved, err := service.GetOrganizationSettings(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.ID, retrieved.ID)
	assert.Equal(t, domain.EnforcementModeStrict, retrieved.EnforcementMode)
}

func TestAdminService_GetEnforcementSettings_Strict(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Strict Org",
		EnforcementMode: domain.EnforcementModeStrict,
	}
	orgRepo.AddOrg(org)

	settings, err := service.GetEnforcementSettings(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.EnforcementModeStrict, settings.EnforcementMode)
	assert.Equal(t, "Strict Mode", settings.Description)
	assert.Contains(t, settings.Explanation, "blocked")
}

func TestAdminService_GetEnforcementSettings_Monitoring(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Monitoring Org",
		EnforcementMode: domain.EnforcementModeMonitoring,
	}
	orgRepo.AddOrg(org)

	settings, err := service.GetEnforcementSettings(ctx, org.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.EnforcementModeMonitoring, settings.EnforcementMode)
	assert.Equal(t, "Monitoring Mode", settings.Description)
	assert.Contains(t, settings.Explanation, "logged but allowed")
}

func TestAdminService_GetEnforcementSettings_Unknown(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Unknown Org",
		EnforcementMode: domain.EnforcementMode("unknown"),
	}
	orgRepo.AddOrg(org)

	settings, err := service.GetEnforcementSettings(ctx, org.ID)
	require.NoError(t, err)
	// Should default to monitoring
	assert.Equal(t, domain.EnforcementModeMonitoring, settings.EnforcementMode)
}

func TestAdminService_UpdateEnforcementMode(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Test Org",
		EnforcementMode: domain.EnforcementModeMonitoring,
	}
	orgRepo.AddOrg(org)

	err := service.UpdateEnforcementMode(ctx, org.ID, domain.EnforcementModeStrict)
	require.NoError(t, err)

	updated, _ := orgRepo.GetByID(org.ID)
	assert.Equal(t, domain.EnforcementModeStrict, updated.EnforcementMode)
}

func TestAdminService_UpdateEnforcementMode_Invalid(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	org := &domain.Organization{
		ID:              uuid.New(),
		Name:            "Test Org",
		EnforcementMode: domain.EnforcementModeMonitoring,
	}
	orgRepo.AddOrg(org)

	err := service.UpdateEnforcementMode(ctx, org.ID, domain.EnforcementMode("invalid"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid enforcement mode")
}

func TestAdminService_UpdateEnforcementMode_OrgNotFound(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	err := service.UpdateEnforcementMode(ctx, uuid.New(), domain.EnforcementModeStrict)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get organization")
}

func TestAdminService_ApproveUser_UpdateError(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	userRepo.updateErr = errors.New("database error")
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	user := createAdminTestUser(orgID, domain.UserStatusPending)
	userRepo.AddUser(user)

	err := service.ApproveUser(ctx, user.ID, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to approve user")
}

func TestAdminService_UpdateUserRole_Error(t *testing.T) {
	userRepo := NewMockUserRepoForAdmin()
	userRepo.updateRoleErr = errors.New("database error")
	orgRepo := NewMockOrgRepository()
	service := NewAdminService(userRepo, orgRepo)
	ctx := context.Background()

	orgID := uuid.New()
	user := createAdminTestUser(orgID, domain.UserStatusActive)
	userRepo.AddUser(user)

	err := service.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update user role")
}

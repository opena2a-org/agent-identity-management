package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ====================
// Mock Repositories
// ====================

// MockUserRepository for testing
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByPasswordResetToken(resetToken string) (*domain.User, error) {
	args := m.Called(resetToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	args := m.Called(orgID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepository) Update(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateRole(id uuid.UUID, role domain.UserRole) error {
	args := m.Called(id, role)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepository) CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error) {
	args := m.Called(orgID, withinMinutes)
	return args.Int(0), args.Error(1)
}

// MockOrganizationRepository for testing
type MockOrganizationRepository struct {
	mock.Mock
}

func (m *MockOrganizationRepository) Create(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) GetByDomain(domainName string) (*domain.Organization, error) {
	args := m.Called(domainName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrganizationRepository) Update(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrganizationRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockAPIKeyRepository for testing
type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Create(apiKey *domain.APIKey) error {
	args := m.Called(apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByHash(hash string) (*domain.APIKey, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByAgent(agentID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) UpdateLastUsed(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Revoke(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockEmailService for testing
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	args := m.Called(to, subject, body, isHTML)
	return args.Error(0)
}

func (m *MockEmailService) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
	args := m.Called(template, to, data)
	return args.Error(0)
}

func (m *MockEmailService) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	args := m.Called(recipients, subject, body, isHTML)
	return args.Error(0)
}

func (m *MockEmailService) ValidateConnection() error {
	args := m.Called()
	return args.Error(0)
}

// ====================
// Test Helper Functions
// ====================

// createTestUser creates a test user with default values
func createTestUser(email string) *domain.User {
	passwordHasher := auth.NewPasswordHasher()
	hashedPassword, _ := passwordHasher.HashPassword("SecurePass123!")

	return &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          email,
		Name:           "Test User",
		Role:           domain.RoleMember,
		Provider:       "local",
		ProviderID:     email,
		Status:         domain.UserStatusActive,
		PasswordHash:   &hashedPassword,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// createTestOrganization creates a test organization
func createTestOrganization() *domain.Organization {
	return &domain.Organization{
		ID:        uuid.New(),
		Name:      "Test Organization",
		Domain:    "test-org.com",
		PlanType:  "pro",
		MaxAgents: 100,
		MaxUsers:  10,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// createTestAPIKey creates a test API key
func createTestAPIKey(orgID, userID uuid.UUID) *domain.APIKey {
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	agentID := uuid.New()
	return &domain.APIKey{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AgentID:        agentID,
		Name:           "Test API Key",
		KeyHash:        "test-hash",
		Prefix:         "aim_test",
		IsActive:       true,
		ExpiresAt:      &expiresAt,
		CreatedBy:      userID,
		CreatedAt:      time.Now(),
	}
}

// ====================
// LoginWithPassword Tests
// ====================

func TestAuthService_LoginWithPassword_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(nil)

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "SecurePass123!")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.ID)
	assert.Equal(t, user.Email, result.Email)
	assert.NotNil(t, result.LastLoginAt)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	mockUserRepo.On("GetByEmail", "nonexistent@example.com").Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "nonexistent@example.com", "password")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid credentials")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_WrongPassword(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "WrongPassword123!")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid credentials")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_DeactivatedUser(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	user.Status = domain.UserStatusDeactivated

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "SecurePass123!")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "deactivated")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_SoftDeletedUser(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	deletedAt := time.Now()
	user.DeletedAt = &deletedAt

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "SecurePass123!")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "deactivated")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_NoPasswordHash(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	user.PasswordHash = nil // User has no password (e.g., OAuth-only user)

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "SecurePass123!")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "local authentication not configured")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithPassword_UpdateLastLoginFails(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	result, err := service.LoginWithPassword(ctx, "test@example.com", "SecurePass123!")

	// Assert - Login should succeed even if updating last_login_at fails
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.ID)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// GetUserByID Tests
// ====================

func TestAuthService_GetUserByID_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUserByID(ctx, user.ID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.ID)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_GetUserByID_NotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	userID := uuid.New()
	mockUserRepo.On("GetByID", userID).Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	result, err := service.GetUserByID(ctx, userID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// GetUserByEmail Tests
// ====================

func TestAuthService_GetUserByEmail_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByEmail", "test@example.com").Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUserByEmail(ctx, "test@example.com")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.Email, result.Email)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_GetUserByEmail_NotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	mockUserRepo.On("GetByEmail", "nonexistent@example.com").Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	result, err := service.GetUserByEmail(ctx, "nonexistent@example.com")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// GetUsersByOrganization Tests
// ====================

func TestAuthService_GetUsersByOrganization_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	orgID := uuid.New()
	users := []*domain.User{
		createTestUser("user1@example.com"),
		createTestUser("user2@example.com"),
	}

	mockUserRepo.On("GetByOrganization", orgID).Return(users, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUsersByOrganization(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_GetUsersByOrganization_EmptyOrg(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	orgID := uuid.New()
	mockUserRepo.On("GetByOrganization", orgID).Return([]*domain.User{}, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUsersByOrganization(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_GetUsersByOrganization_Error(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	orgID := uuid.New()
	mockUserRepo.On("GetByOrganization", orgID).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	result, err := service.GetUsersByOrganization(ctx, orgID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// UpdateUserRole Tests
// ====================

func TestAuthService_UpdateUserRole_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(nil)

	// Act
	ctx := context.Background()
	result, err := service.UpdateUserRole(ctx, user.ID, user.OrganizationID, domain.RoleAdmin, adminID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.RoleAdmin, result.Role)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_UpdateUserRole_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	userID := uuid.New()
	orgID := uuid.New()
	adminID := uuid.New()

	mockUserRepo.On("GetByID", userID).Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	result, err := service.UpdateUserRole(ctx, userID, orgID, domain.RoleAdmin, adminID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_UpdateUserRole_WrongOrganization(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	differentOrgID := uuid.New()
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	result, err := service.UpdateUserRole(ctx, user.ID, differentOrgID, domain.RoleAdmin, adminID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "user not found in organization")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_UpdateUserRole_UpdateFails(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	result, err := service.UpdateUserRole(ctx, user.ID, user.OrganizationID, domain.RoleAdmin, adminID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// DeactivateUser Tests
// ====================

func TestAuthService_DeactivateUser_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(nil)

	// Act
	ctx := context.Background()
	err := service.DeactivateUser(ctx, user.ID, user.OrganizationID, adminID)

	// Assert
	assert.NoError(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_DeactivateUser_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	userID := uuid.New()
	orgID := uuid.New()
	adminID := uuid.New()

	mockUserRepo.On("GetByID", userID).Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	err := service.DeactivateUser(ctx, userID, orgID, adminID)

	// Assert
	assert.Error(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_DeactivateUser_WrongOrganization(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	differentOrgID := uuid.New()
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	err := service.DeactivateUser(ctx, user.ID, differentOrgID, adminID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found in organization")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_DeactivateUser_SelfDeactivation(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act - Admin tries to deactivate themselves
	ctx := context.Background()
	err := service.DeactivateUser(ctx, user.ID, user.OrganizationID, user.ID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot deactivate your own account")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_DeactivateUser_UpdateFails(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	adminID := uuid.New()

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.DeactivateUser(ctx, user.ID, user.OrganizationID, adminID)

	// Assert
	assert.Error(t, err)

	mockUserRepo.AssertExpectations(t)
}

// ====================
// ChangePassword Tests
// ====================

func TestAuthService_ChangePassword_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	user.ForcePasswordChange = true

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(nil)

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, user.ID, "SecurePass123!", "NewSecurePass123!")

	// Assert
	assert.NoError(t, err)

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	userID := uuid.New()
	mockUserRepo.On("GetByID", userID).Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, userID, "OldPass123!", "NewPass123!")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_NoPasswordHash(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	user.PasswordHash = nil

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, user.ID, "OldPass123!", "NewPass123!")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password not configured")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, user.ID, "WrongPassword123!", "NewPass123!")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current password is incorrect")

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_WeakNewPassword(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, user.ID, "SecurePass123!", "weak")

	// Assert
	assert.Error(t, err)
	// Should fail validation

	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ChangePassword_UpdateFails(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")

	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.ChangePassword(ctx, user.ID, "SecurePass123!", "NewSecurePass123!")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update password")

	mockUserRepo.AssertExpectations(t)
}

// ====================
// ValidateAPIKey Tests
// ====================

func TestAuthService_ValidateAPIKey_Success(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)

	// Calculate the hash the same way the service does
	rawKey := "aim_test_12345678901234567890"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)
	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockOrgRepo.On("GetByID", org.ID).Return(org, nil)
	mockAPIKeyRepo.On("UpdateLastUsed", apiKey.ID).Return(nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, user.ID, result.User.ID)
	assert.Equal(t, org.ID, result.Organization.ID)
	assert.Equal(t, apiKey.ID, result.APIKey.ID)

	mockAPIKeyRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockOrgRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_InvalidKey(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	rawKey := "invalid_key"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(nil, errors.New("not found"))

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_KeyNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	rawKey := "aim_test_nonexistent"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(nil, nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid API key")

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_InactiveKey(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)
	apiKey.IsActive = false

	rawKey := "aim_test_inactive"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "inactive")

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_ExpiredKey(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)
	expiredTime := time.Now().Add(-24 * time.Hour)
	apiKey.ExpiresAt = &expiredTime

	rawKey := "aim_test_expired"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "expired")

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_UserNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)

	rawKey := "aim_test_nouser"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)
	mockUserRepo.On("GetByID", user.ID).Return(nil, errors.New("user not found"))

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to retrieve user")

	mockAPIKeyRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_OrganizationNotFound(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)

	rawKey := "aim_test_noorg"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)
	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockOrgRepo.On("GetByID", org.ID).Return(nil, errors.New("org not found"))

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to retrieve organization")

	mockAPIKeyRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockOrgRepo.AssertExpectations(t)
}

func TestAuthService_ValidateAPIKey_UpdateLastUsedFails(t *testing.T) {
	// Arrange
	mockUserRepo := new(MockUserRepository)
	mockOrgRepo := new(MockOrganizationRepository)
	mockAPIKeyRepo := new(MockAPIKeyRepository)
	mockEmailService := new(MockEmailService)

	service := NewAuthService(mockUserRepo, mockOrgRepo, mockAPIKeyRepo, nil, mockEmailService)

	user := createTestUser("test@example.com")
	org := createTestOrganization()
	apiKey := createTestAPIKey(org.ID, user.ID)

	rawKey := "aim_test_updatefail"
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey := base64.StdEncoding.EncodeToString(hash[:])
	apiKey.KeyHash = hashedKey

	mockAPIKeyRepo.On("GetByHash", hashedKey).Return(apiKey, nil)
	mockUserRepo.On("GetByID", user.ID).Return(user, nil)
	mockOrgRepo.On("GetByID", org.ID).Return(org, nil)
	mockAPIKeyRepo.On("UpdateLastUsed", apiKey.ID).Return(errors.New("update failed"))

	// Act - Validation should succeed even if UpdateLastUsed fails (non-critical)
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, rawKey)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)

	mockAPIKeyRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
	mockOrgRepo.AssertExpectations(t)
}

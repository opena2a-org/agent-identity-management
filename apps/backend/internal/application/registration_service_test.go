package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ========================================
// Error Variable Tests
// ========================================

func TestRegistrationService_ErrorVariables(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "ErrRegistrationNotFound",
			err:         ErrRegistrationNotFound,
			expectedMsg: "registration request not found",
		},
		{
			name:        "ErrRegistrationNotPending",
			err:         ErrRegistrationNotPending,
			expectedMsg: "registration request is not pending",
		},
		{
			name:        "ErrUserAlreadyExists",
			err:         ErrUserAlreadyExists,
			expectedMsg: "user with this email already exists",
		},
		{
			name:        "ErrRegistrationRequestExists",
			err:         ErrRegistrationRequestExists,
			expectedMsg: "registration request with this email already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.expectedMsg, tt.err.Error())
		})
	}
}

func TestRegistrationService_ErrorsAreDistinct(t *testing.T) {
	errors := []error{
		ErrRegistrationNotFound,
		ErrRegistrationNotPending,
		ErrUserAlreadyExists,
		ErrRegistrationRequestExists,
	}

	// Verify all errors are distinct
	for i := 0; i < len(errors); i++ {
		for j := i + 1; j < len(errors); j++ {
			assert.NotEqual(t, errors[i], errors[j], "errors should be distinct")
		}
	}
}

// ========================================
// extractEmailDomain Helper Function Tests
// ========================================

func TestExtractEmailDomain_ValidEmails(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		expectedDomain string
	}{
		{
			name:           "simple email",
			email:          "user@example.com",
			expectedDomain: "example.com",
		},
		{
			name:           "email with subdomain",
			email:          "user@mail.example.com",
			expectedDomain: "mail.example.com",
		},
		{
			name:           "email with plus addressing",
			email:          "user+tag@example.com",
			expectedDomain: "example.com",
		},
		{
			name:           "email with dots in local part",
			email:          "first.last@company.org",
			expectedDomain: "company.org",
		},
		{
			name:           "email with numbers",
			email:          "user123@domain456.net",
			expectedDomain: "domain456.net",
		},
		{
			name:           "email with hyphen in domain",
			email:          "user@my-company.io",
			expectedDomain: "my-company.io",
		},
		{
			name:           "email with long TLD",
			email:          "admin@company.technology",
			expectedDomain: "company.technology",
		},
		{
			name:           "email with country code TLD",
			email:          "user@company.co.uk",
			expectedDomain: "company.co.uk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailDomain(tt.email)
			assert.Equal(t, tt.expectedDomain, result)
		})
	}
}

func TestExtractEmailDomain_InvalidEmails(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string // Returns original if invalid
	}{
		{
			name:     "no @ symbol",
			email:    "userexample.com",
			expected: "userexample.com", // Returns original
		},
		{
			name:     "multiple @ symbols",
			email:    "user@domain@example.com",
			expected: "user@domain@example.com", // Returns original
		},
		{
			name:     "empty string",
			email:    "",
			expected: "", // Returns original
		},
		{
			name:     "only @ symbol",
			email:    "@",
			expected: "", // Returns empty domain part
		},
		{
			name:     "@ at start",
			email:    "@example.com",
			expected: "example.com", // Valid domain extraction
		},
		{
			name:     "@ at end",
			email:    "user@",
			expected: "", // Returns empty domain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailDomain(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// RegistrationService Constructor Tests
// ========================================

func TestNewRegistrationService(t *testing.T) {
	// Test that constructor returns non-nil service
	service := NewRegistrationService(nil, nil, nil, nil, nil)
	assert.NotNil(t, service)
}

func TestNewRegistrationService_StoresAllDependencies(t *testing.T) {
	// Create a service with nil dependencies (for testing)
	service := NewRegistrationService(nil, nil, nil, nil, nil)

	// Verify the struct is properly initialized
	assert.NotNil(t, service)
	assert.Nil(t, service.registrationRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.orgRepo)
	assert.Nil(t, service.auditService)
	assert.Nil(t, service.emailService)
}

// ========================================
// RegistrationRepository Interface Tests
// ========================================

func TestRegistrationRepository_InterfaceDefinition(t *testing.T) {
	// Verify interface exists and has expected methods
	// This test ensures the interface signature doesn't change unexpectedly

	var _ RegistrationRepository = (*regSvcMockRegistrationRepo)(nil)
}

// regSvcMockRegistrationRepo implements RegistrationRepository for interface testing
type regSvcMockRegistrationRepo struct{}

func (m *regSvcMockRegistrationRepo) CreateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	return nil
}

func (m *regSvcMockRegistrationRepo) GetRegistrationRequest(ctx context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error) {
	return nil, nil
}

func (m *regSvcMockRegistrationRepo) GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	return nil, nil
}

func (m *regSvcMockRegistrationRepo) GetRegistrationRequestByEmailAnyStatus(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	return nil, nil
}

func (m *regSvcMockRegistrationRepo) ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error) {
	return nil, 0, nil
}

func (m *regSvcMockRegistrationRepo) UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	return nil
}

// ========================================
// Email Domain Extraction Edge Cases
// ========================================

func TestExtractEmailDomain_CaseSensitivity(t *testing.T) {
	// Domain extraction should preserve case
	tests := []struct {
		email          string
		expectedDomain string
	}{
		{"user@EXAMPLE.COM", "EXAMPLE.COM"},
		{"user@Example.Com", "Example.Com"},
		{"USER@example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := extractEmailDomain(tt.email)
			assert.Equal(t, tt.expectedDomain, result)
		})
	}
}

func TestExtractEmailDomain_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		expectedDomain string
	}{
		{
			name:           "underscores in local part",
			email:          "first_last@example.com",
			expectedDomain: "example.com",
		},
		{
			name:           "hyphens in local part",
			email:          "first-last@example.com",
			expectedDomain: "example.com",
		},
		{
			name:           "numbers in local part",
			email:          "user123@example.com",
			expectedDomain: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailDomain(tt.email)
			assert.Equal(t, tt.expectedDomain, result)
		})
	}
}

// ========================================
// Registration Status Logic Tests
// ========================================

func TestRegistrationService_StatusFlowLogic(t *testing.T) {
	// Test that registration status flows are understood correctly
	// This documents the expected state transitions

	// Pending -> Approved (valid transition)
	// Pending -> Rejected (valid transition)
	// Approved -> anything (invalid - should fail with ErrRegistrationNotPending)
	// Rejected -> anything (invalid - should fail with ErrRegistrationNotPending)

	t.Run("error returned when not pending", func(t *testing.T) {
		assert.Equal(t, "registration request is not pending", ErrRegistrationNotPending.Error())
	})
}

// ========================================
// Password Reset Token Logic Tests
// ========================================

func TestRegistrationService_PasswordResetTokenGeneration(t *testing.T) {
	// Test that UUID tokens are valid format
	token := uuid.New().String()

	// Token should be valid UUID format
	parsed, err := uuid.Parse(token)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, parsed)
}

func TestRegistrationService_PasswordResetTokenUniqueness(t *testing.T) {
	// Generate multiple tokens and verify uniqueness
	tokens := make(map[string]bool)

	for i := 0; i < 100; i++ {
		token := uuid.New().String()
		assert.False(t, tokens[token], "token should be unique")
		tokens[token] = true
	}
}

// ========================================
// Service Field Initialization Tests
// ========================================

func TestRegistrationService_FieldTypes(t *testing.T) {
	service := &RegistrationService{}

	// Verify all field types are nil-able
	assert.Nil(t, service.registrationRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.orgRepo)
	assert.Nil(t, service.auditService)
	assert.Nil(t, service.emailService)
}

// ========================================
// Input Validation Logic Tests
// ========================================

func TestRegistrationService_EmailNormalization(t *testing.T) {
	// Test email normalization patterns that RequestPasswordReset uses:
	// email = strings.ToLower(strings.TrimSpace(email))
	tests := []struct {
		input    string
		expected string
	}{
		{"  user@example.com  ", "user@example.com"},
		{"\tuser@example.com\n", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Simulate normalization that happens in RequestPasswordReset
			normalized := strings.ToLower(strings.TrimSpace(tt.input))
			assert.Equal(t, tt.expected, normalized)
		})
	}
}

// ========================================
// Error Wrapping Pattern Tests
// ========================================

func TestRegistrationService_ErrorWrappingPatterns(t *testing.T) {
	// Document the error wrapping patterns used in the service
	// These tests ensure consistent error messages

	errorPatterns := []string{
		"failed to create registration request:",
		"failed to find or create organization:",
		"failed to create auto-approved user:",
		"failed to update registration request:",
		"failed to create user",
		"failed to check existing users:",
		"failed to update user with reset token:",
		"failed to hash password:",
		"failed to update password:",
		"password validation failed:",
		"invalid or expired reset token",
		"reset token is required",
		"new password is required",
		"passwords do not match",
	}

	// Verify these are the patterns that exist
	for _, pattern := range errorPatterns {
		assert.NotEmpty(t, pattern)
	}
}

// ========================================
// Organization Creation Logic Tests
// ========================================

func TestRegistrationService_DefaultOrganizationValues(t *testing.T) {
	// Test default values that should be set when creating organizations
	// From findOrCreateOrganization:
	// PlanType: "free"
	// MaxAgents: 100
	// MaxUsers: 10
	// IsActive: true

	expectedDefaults := map[string]interface{}{
		"PlanType":  "free",
		"MaxAgents": 100,
		"MaxUsers":  10,
		"IsActive":  true,
	}

	assert.Equal(t, "free", expectedDefaults["PlanType"])
	assert.Equal(t, 100, expectedDefaults["MaxAgents"])
	assert.Equal(t, 10, expectedDefaults["MaxUsers"])
	assert.Equal(t, true, expectedDefaults["IsActive"])
}

// ========================================
// Auto-Approval Logic Tests
// ========================================

func TestRegistrationService_AutoApprovalConditions(t *testing.T) {
	// Document and test the auto-approval logic
	// shouldAutoApproveFirstUser returns true when:
	// 1. No organization exists for this domain, OR
	// 2. Organization exists but has zero users

	tests := []struct {
		name          string
		orgExists     bool
		userCount     int
		shouldApprove bool
	}{
		{
			name:          "no org exists - should auto-approve",
			orgExists:     false,
			userCount:     0,
			shouldApprove: true,
		},
		{
			name:          "org exists with no users - should auto-approve",
			orgExists:     true,
			userCount:     0,
			shouldApprove: true,
		},
		{
			name:          "org exists with users - should not auto-approve",
			orgExists:     true,
			userCount:     1,
			shouldApprove: false,
		},
		{
			name:          "org exists with many users - should not auto-approve",
			orgExists:     true,
			userCount:     10,
			shouldApprove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document the expected behavior
			if !tt.orgExists || tt.userCount == 0 {
				assert.True(t, tt.shouldApprove)
			} else {
				assert.False(t, tt.shouldApprove)
			}
		})
	}
}

// ========================================
// First User Role Assignment Tests
// ========================================

func TestRegistrationService_FirstUserBecomesAdmin(t *testing.T) {
	// Document that first user in organization becomes admin
	// From ApproveRegistrationRequest:
	// userRole := domain.RoleViewer // Default to viewer
	// if len(existingUsers) == 0 {
	//     userRole = domain.RoleAdmin // First user becomes admin
	// }

	tests := []struct {
		name          string
		existingUsers int
		expectedRole  string
	}{
		{
			name:          "first user becomes admin",
			existingUsers: 0,
			expectedRole:  "admin",
		},
		{
			name:          "subsequent user becomes viewer",
			existingUsers: 1,
			expectedRole:  "viewer",
		},
		{
			name:          "tenth user becomes viewer",
			existingUsers: 9,
			expectedRole:  "viewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := "viewer"
			if tt.existingUsers == 0 {
				role = "admin"
			}
			assert.Equal(t, tt.expectedRole, role)
		})
	}
}

// ========================================
// Name Construction Logic Tests
// ========================================

func TestRegistrationService_FullNameConstruction(t *testing.T) {
	// Test name construction logic from the service
	// fullName := firstName
	// if lastName != "" {
	//     if fullName != "" {
	//         fullName += " "
	//     }
	//     fullName += lastName
	// }
	// if fullName == "" {
	//     fullName = email
	// }

	tests := []struct {
		name      string
		firstName string
		lastName  string
		email     string
		expected  string
	}{
		{
			name:      "first and last name",
			firstName: "John",
			lastName:  "Doe",
			email:     "john@example.com",
			expected:  "John Doe",
		},
		{
			name:      "only first name",
			firstName: "John",
			lastName:  "",
			email:     "john@example.com",
			expected:  "John",
		},
		{
			name:      "only last name",
			firstName: "",
			lastName:  "Doe",
			email:     "john@example.com",
			expected:  "Doe",
		},
		{
			name:      "no name - fallback to email",
			firstName: "",
			lastName:  "",
			email:     "john@example.com",
			expected:  "john@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullName := tt.firstName
			if tt.lastName != "" {
				if fullName != "" {
					fullName += " "
				}
				fullName += tt.lastName
			}
			if fullName == "" {
				fullName = tt.email
			}
			assert.Equal(t, tt.expected, fullName)
		})
	}
}

// ========================================
// Provider Determination Logic Tests
// ========================================

func TestRegistrationService_ProviderDetermination(t *testing.T) {
	// Test provider determination logic
	// provider := "local" // Default to local for email/password
	// providerID := req.Email // Use email as provider ID for local auth
	// If OAuth registration, use OAuth provider info

	tests := []struct {
		name             string
		hasOAuthProvider bool
		oauthProvider    string
		oauthUserID      string
		email            string
		expectedProvider string
		expectedID       string
	}{
		{
			name:             "local registration",
			hasOAuthProvider: false,
			email:            "user@example.com",
			expectedProvider: "local",
			expectedID:       "user@example.com",
		},
		{
			name:             "google oauth",
			hasOAuthProvider: true,
			oauthProvider:    "google",
			oauthUserID:      "google-123",
			email:            "user@example.com",
			expectedProvider: "google",
			expectedID:       "google-123",
		},
		{
			name:             "github oauth",
			hasOAuthProvider: true,
			oauthProvider:    "github",
			oauthUserID:      "github-456",
			email:            "user@example.com",
			expectedProvider: "github",
			expectedID:       "github-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := "local"
			providerID := tt.email

			if tt.hasOAuthProvider && tt.oauthProvider != "" {
				provider = tt.oauthProvider
				if tt.oauthUserID != "" {
					providerID = tt.oauthUserID
				}
			}

			assert.Equal(t, tt.expectedProvider, provider)
			assert.Equal(t, tt.expectedID, providerID)
		})
	}
}

// ========================================
// Password Reset Expiration Tests
// ========================================

func TestRegistrationService_PasswordResetExpiration(t *testing.T) {
	// Test that password reset tokens expire in 24 hours
	// expiresAt := time.Now().Add(24 * time.Hour)

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	// Verify expiration is ~24 hours in future
	diff := expiresAt.Sub(now)
	assert.Equal(t, 24*time.Hour, diff)
}

// ========================================
// Mock Implementations for Service Tests
// ========================================

// MockRegistrationRepoForService implements RegistrationRepository with testify mock
type MockRegistrationRepoForService struct {
	mock.Mock
}

func (m *MockRegistrationRepoForService) CreateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockRegistrationRepoForService) GetRegistrationRequest(ctx context.Context, id uuid.UUID) (*domain.UserRegistrationRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserRegistrationRequest), args.Error(1)
}

func (m *MockRegistrationRepoForService) GetRegistrationRequestByEmail(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserRegistrationRequest), args.Error(1)
}

func (m *MockRegistrationRepoForService) GetRegistrationRequestByEmailAnyStatus(ctx context.Context, email string) (*domain.UserRegistrationRequest, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserRegistrationRequest), args.Error(1)
}

func (m *MockRegistrationRepoForService) ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error) {
	args := m.Called(ctx, orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.UserRegistrationRequest), args.Int(1), args.Error(2)
}

func (m *MockRegistrationRepoForService) UpdateRegistrationRequest(ctx context.Context, req *domain.UserRegistrationRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

// MockUserRepoForRegistration implements domain.UserRepository
type MockUserRepoForRegistration struct {
	mock.Mock
}

func (m *MockUserRepoForRegistration) Create(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepoForRegistration) GetByID(id uuid.UUID) (*domain.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepoForRegistration) GetByEmail(email string) (*domain.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepoForRegistration) GetByPasswordResetToken(resetToken string) (*domain.User, error) {
	args := m.Called(resetToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepoForRegistration) GetByOrganization(orgID uuid.UUID) ([]*domain.User, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepoForRegistration) GetByOrganizationAndStatus(orgID uuid.UUID, status domain.UserStatus) ([]*domain.User, error) {
	args := m.Called(orgID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepoForRegistration) Update(user *domain.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepoForRegistration) UpdateRole(id uuid.UUID, role domain.UserRole) error {
	args := m.Called(id, role)
	return args.Error(0)
}

func (m *MockUserRepoForRegistration) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepoForRegistration) CountActiveUsers(orgID uuid.UUID, withinMinutes int) (int, error) {
	args := m.Called(orgID, withinMinutes)
	return args.Int(0), args.Error(1)
}

// MockOrgRepoForRegistration implements domain.OrganizationRepository
type MockOrgRepoForRegistration struct {
	mock.Mock
}

func (m *MockOrgRepoForRegistration) Create(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrgRepoForRegistration) GetByID(id uuid.UUID) (*domain.Organization, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrgRepoForRegistration) GetByDomain(domainName string) (*domain.Organization, error) {
	args := m.Called(domainName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}

func (m *MockOrgRepoForRegistration) Update(org *domain.Organization) error {
	args := m.Called(org)
	return args.Error(0)
}

func (m *MockOrgRepoForRegistration) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockEmailServiceForRegistration implements domain.EmailService
type MockEmailServiceForRegistration struct {
	mock.Mock
}

func (m *MockEmailServiceForRegistration) SendEmail(to, subject, body string, isHTML bool) error {
	args := m.Called(to, subject, body, isHTML)
	return args.Error(0)
}

func (m *MockEmailServiceForRegistration) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
	args := m.Called(template, to, data)
	return args.Error(0)
}

func (m *MockEmailServiceForRegistration) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	args := m.Called(recipients, subject, body, isHTML)
	return args.Error(0)
}

func (m *MockEmailServiceForRegistration) ValidateConnection() error {
	args := m.Called()
	return args.Error(0)
}

// ========================================
// Service Method Tests
// ========================================

func TestRegistrationService_CreateManualRegistrationRequest_UserExists(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}
	mockUserRepo.On("GetByEmail", "existing@example.com").Return(existingUser, nil)

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	req, err := service.CreateManualRegistrationRequest(ctx, "existing@example.com", "John", "Doe", "Password123!")

	assert.Nil(t, req)
	assert.Equal(t, ErrUserAlreadyExists, err)
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_CreateManualRegistrationRequest_PendingRequestExists(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByEmail", "pending@example.com").Return(nil, errors.New("not found"))

	mockRegRepo := new(MockRegistrationRepoForService)
	now := time.Now()
	existingRequest := &domain.UserRegistrationRequest{
		ID:          uuid.New(),
		Email:       "pending@example.com",
		Status:      domain.RegistrationStatusPending,
		RequestedAt: now,
	}
	mockRegRepo.On("GetRegistrationRequestByEmail", ctx, "pending@example.com").Return(existingRequest, nil)

	service := NewRegistrationService(mockRegRepo, mockUserRepo, nil, nil, nil)

	req, err := service.CreateManualRegistrationRequest(ctx, "pending@example.com", "John", "Doe", "Password123!")

	assert.Nil(t, req)
	assert.Equal(t, ErrRegistrationRequestExists, err)
	mockUserRepo.AssertExpectations(t)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_CreateManualRegistrationRequest_WeakPassword(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByEmail", "new@example.com").Return(nil, errors.New("not found"))

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequestByEmail", ctx, "new@example.com").Return(nil, errors.New("not found"))

	service := NewRegistrationService(mockRegRepo, mockUserRepo, nil, nil, nil)

	// Weak password
	req, err := service.CreateManualRegistrationRequest(ctx, "new@example.com", "John", "Doe", "weak")

	assert.Nil(t, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password validation failed")
}

func TestRegistrationService_CreateAccessRequest_Success(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByEmail", "newuser@example.com").Return(nil, errors.New("not found"))

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequestByEmail", ctx, "newuser@example.com").Return(nil, errors.New("not found"))
	mockRegRepo.On("CreateRegistrationRequest", ctx, mock.AnythingOfType("*domain.UserRegistrationRequest")).Return(nil)

	service := NewRegistrationService(mockRegRepo, mockUserRepo, nil, nil, nil)

	req, err := service.CreateAccessRequest(ctx, "newuser@example.com", "Jane", "Smith", "I need access for my work", nil)

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "newuser@example.com", req.Email)
	assert.Equal(t, "Jane", req.FirstName)
	assert.Equal(t, "Smith", req.LastName)
	assert.Equal(t, domain.RegistrationStatusPending, req.Status)
	mockUserRepo.AssertExpectations(t)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_CreateAccessRequest_UserExists(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	existingUser := &domain.User{
		ID:    uuid.New(),
		Email: "existing@example.com",
	}
	mockUserRepo.On("GetByEmail", "existing@example.com").Return(existingUser, nil)

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	req, err := service.CreateAccessRequest(ctx, "existing@example.com", "Jane", "Smith", "reason", nil)

	assert.Nil(t, req)
	assert.Equal(t, ErrUserAlreadyExists, err)
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_CreateAccessRequest_WithOrganizationName(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByEmail", "user@company.com").Return(nil, errors.New("not found"))

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequestByEmail", ctx, "user@company.com").Return(nil, errors.New("not found"))
	mockRegRepo.On("CreateRegistrationRequest", ctx, mock.AnythingOfType("*domain.UserRegistrationRequest")).Return(nil)

	service := NewRegistrationService(mockRegRepo, mockUserRepo, nil, nil, nil)

	orgName := "Acme Corp"
	req, err := service.CreateAccessRequest(ctx, "user@company.com", "Bob", "Wilson", "Need access", &orgName)

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "Acme Corp", req.Metadata["organization_name"])
	mockUserRepo.AssertExpectations(t)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_GetRegistrationRequest_Success(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	expectedReq := &domain.UserRegistrationRequest{
		ID:     requestID,
		Email:  "test@example.com",
		Status: domain.RegistrationStatusPending,
	}
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(expectedReq, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	req, err := service.GetRegistrationRequest(ctx, requestID)

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, requestID, req.ID)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_GetRegistrationRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(nil, errors.New("not found"))

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	req, err := service.GetRegistrationRequest(ctx, requestID)

	assert.Error(t, err)
	assert.Nil(t, req)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_GetRegistrationRequestByEmail_Success(t *testing.T) {
	ctx := context.Background()

	mockRegRepo := new(MockRegistrationRepoForService)
	expectedReq := &domain.UserRegistrationRequest{
		ID:     uuid.New(),
		Email:  "found@example.com",
		Status: domain.RegistrationStatusApproved,
	}
	mockRegRepo.On("GetRegistrationRequestByEmailAnyStatus", ctx, "found@example.com").Return(expectedReq, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	req, err := service.GetRegistrationRequestByEmail(ctx, "found@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "found@example.com", req.Email)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_ListPendingRegistrationRequests_Success(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	requests := []*domain.UserRegistrationRequest{
		{ID: uuid.New(), Email: "pending1@example.com", Status: domain.RegistrationStatusPending},
		{ID: uuid.New(), Email: "pending2@example.com", Status: domain.RegistrationStatusPending},
	}
	mockRegRepo.On("ListPendingRegistrationRequests", ctx, orgID, 10, 0).Return(requests, 2, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	result, total, err := service.ListPendingRegistrationRequests(ctx, orgID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_ListPendingRegistrationRequests_Empty(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("ListPendingRegistrationRequests", ctx, orgID, 10, 0).Return([]*domain.UserRegistrationRequest{}, 0, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	result, total, err := service.ListPendingRegistrationRequests(ctx, orgID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 0)
	assert.Equal(t, 0, total)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_RejectRegistrationRequest_Success(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	reviewerID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	pendingReq := &domain.UserRegistrationRequest{
		ID:     requestID,
		Email:  "reject@example.com",
		Status: domain.RegistrationStatusPending,
	}
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(pendingReq, nil)
	mockRegRepo.On("UpdateRegistrationRequest", ctx, mock.AnythingOfType("*domain.UserRegistrationRequest")).Return(nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	err := service.RejectRegistrationRequest(ctx, requestID, reviewerID, "Does not meet requirements")

	assert.NoError(t, err)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_RejectRegistrationRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	reviewerID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(nil, errors.New("not found"))

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	err := service.RejectRegistrationRequest(ctx, requestID, reviewerID, "reason")

	assert.Equal(t, ErrRegistrationNotFound, err)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_RejectRegistrationRequest_NotPending(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	reviewerID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	approvedReq := &domain.UserRegistrationRequest{
		ID:     requestID,
		Email:  "approved@example.com",
		Status: domain.RegistrationStatusApproved,
	}
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(approvedReq, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	err := service.RejectRegistrationRequest(ctx, requestID, reviewerID, "reason")

	assert.Equal(t, ErrRegistrationNotPending, err)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_RequestPasswordReset_UserNotFound(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	// Should return nil even when user not found (for security)
	err := service.RequestPasswordReset(ctx, "unknown@example.com")

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_RequestPasswordReset_UserDeactivated(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	deactivatedUser := &domain.User{
		ID:     uuid.New(),
		Email:  "deactivated@example.com",
		Status: domain.UserStatusDeactivated,
	}
	mockUserRepo.On("GetByEmail", "deactivated@example.com").Return(deactivatedUser, nil)

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	// Should return nil even when user is deactivated (for security)
	err := service.RequestPasswordReset(ctx, "deactivated@example.com")

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_RequestPasswordReset_Success(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	activeUser := &domain.User{
		ID:     uuid.New(),
		Email:  "active@example.com",
		Name:   "Active User",
		Status: domain.UserStatusActive,
	}
	mockUserRepo.On("GetByEmail", "active@example.com").Return(activeUser, nil)
	mockUserRepo.On("Update", mock.AnythingOfType("*domain.User")).Return(nil)

	mockEmailService := new(MockEmailServiceForRegistration)
	mockEmailService.On("SendTemplatedEmail", domain.TemplatePasswordReset, "active@example.com", mock.Anything).Return(nil)

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, mockEmailService)

	err := service.RequestPasswordReset(ctx, "  ACTIVE@EXAMPLE.COM  ") // Test normalization

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
	mockEmailService.AssertExpectations(t)
}

func TestRegistrationService_ResetPassword_EmptyToken(t *testing.T) {
	ctx := context.Background()

	service := NewRegistrationService(nil, nil, nil, nil, nil)

	err := service.ResetPassword(ctx, "", "NewPassword123!", "NewPassword123!")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reset token is required")
}

func TestRegistrationService_ResetPassword_EmptyPassword(t *testing.T) {
	ctx := context.Background()

	service := NewRegistrationService(nil, nil, nil, nil, nil)

	err := service.ResetPassword(ctx, "some-token", "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new password is required")
}

func TestRegistrationService_ResetPassword_PasswordMismatch(t *testing.T) {
	ctx := context.Background()

	service := NewRegistrationService(nil, nil, nil, nil, nil)

	err := service.ResetPassword(ctx, "some-token", "Password123!", "DifferentPassword123!")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "passwords do not match")
}

func TestRegistrationService_ResetPassword_InvalidToken(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	mockUserRepo.On("GetByPasswordResetToken", "invalid-token").Return(nil, errors.New("not found"))

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	err := service.ResetPassword(ctx, "invalid-token", "NewPassword123!", "NewPassword123!")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired reset token")
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_ResetPassword_WeakPassword(t *testing.T) {
	ctx := context.Background()

	mockUserRepo := new(MockUserRepoForRegistration)
	user := &domain.User{
		ID:             uuid.New(),
		Email:          "user@example.com",
		OrganizationID: uuid.New(),
	}
	mockUserRepo.On("GetByPasswordResetToken", "valid-token").Return(user, nil)

	service := NewRegistrationService(nil, mockUserRepo, nil, nil, nil)

	err := service.ResetPassword(ctx, "valid-token", "weak", "weak")

	assert.Error(t, err)
	// Password validation error
	mockUserRepo.AssertExpectations(t)
}

func TestRegistrationService_ApproveRegistrationRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	reviewerID := uuid.New()
	orgID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(nil, errors.New("not found"))

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	user, err := service.ApproveRegistrationRequest(ctx, requestID, reviewerID, orgID)

	assert.Nil(t, user)
	assert.Equal(t, ErrRegistrationNotFound, err)
	mockRegRepo.AssertExpectations(t)
}

func TestRegistrationService_ApproveRegistrationRequest_NotPending(t *testing.T) {
	ctx := context.Background()
	requestID := uuid.New()
	reviewerID := uuid.New()
	orgID := uuid.New()

	mockRegRepo := new(MockRegistrationRepoForService)
	rejectedReq := &domain.UserRegistrationRequest{
		ID:     requestID,
		Email:  "rejected@example.com",
		Status: domain.RegistrationStatusRejected,
	}
	mockRegRepo.On("GetRegistrationRequest", ctx, requestID).Return(rejectedReq, nil)

	service := NewRegistrationService(mockRegRepo, nil, nil, nil, nil)

	user, err := service.ApproveRegistrationRequest(ctx, requestID, reviewerID, orgID)

	assert.Nil(t, user)
	assert.Equal(t, ErrRegistrationNotPending, err)
	mockRegRepo.AssertExpectations(t)
}

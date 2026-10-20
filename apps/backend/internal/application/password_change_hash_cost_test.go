package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// ChangePassword writes the new password's hash at auth.BcryptCost, or at the
// cost of the hasher SetPasswordHasher attached.
func TestAuthService_ChangePassword_HashesAtBcryptCostUnlessAHasherIsAttached(t *testing.T) {
	minCost, err := auth.NewPasswordHasherWithCost(bcrypt.MinCost)
	require.NoError(t, err)
	cases := []struct {
		name   string
		hasher *auth.PasswordHasher
		cost   int
	}{
		{"no hasher attached", nil, auth.BcryptCost},
		{"a hasher at bcrypt's minimum cost", minCost, bcrypt.MinCost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			users := new(MockUserRepository)
			service := NewAuthService(users, new(MockOrganizationRepository), new(MockAPIKeyRepository), nil, nil, new(MockEmailService))
			if tc.hasher != nil {
				service.SetPasswordHasher(tc.hasher)
			}
			current, err := bcrypt.GenerateFromPassword([]byte("SecurePass123!"), bcrypt.MinCost)
			require.NoError(t, err)
			currentHash := string(current)
			user := &domain.User{ID: uuid.New(), OrganizationID: uuid.New(), Email: "change@example.com",
				Role: domain.RoleMember, Status: domain.UserStatusActive, PasswordHash: &currentHash}
			var written *domain.User
			users.On("GetByID", user.ID).Return(user, nil)
			users.On("Update", mock.MatchedBy(func(u *domain.User) bool { written = u; return true })).Return(nil)

			require.NoError(t, service.ChangePassword(context.Background(), user.ID, "SecurePass123!", "NewSecurePass123!"))
			require.NotNil(t, written)
			require.NotNil(t, written.PasswordHash)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*written.PasswordHash), []byte("NewSecurePass123!")))
			cost, err := bcrypt.Cost([]byte(*written.PasswordHash))
			require.NoError(t, err)
			assert.Equal(t, tc.cost, cost)
			users.AssertExpectations(t)
		})
	}
}

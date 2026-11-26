package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// RegistrationChallengeService issues the challenges an agent signs to show,
// at registration, that it holds its private key, and uses them up. What a
// signed challenge is worth is decided in agent_registration_verification.go.
type RegistrationChallengeService struct {
	repo domain.RegistrationChallengeRepository
	now  func() time.Time
}

// NewRegistrationChallengeService creates the service.
func NewRegistrationChallengeService(repo domain.RegistrationChallengeRepository) *RegistrationChallengeService {
	return &RegistrationChallengeService{repo: repo, now: time.Now}
}

// IssuedRegistrationChallenge is what POST /api/v1/agents/registration-challenges
// answers.
type IssuedRegistrationChallenge struct {
	ID uuid.UUID `json:"challengeId"`
	// Challenge is the 32 random bytes in standard padded base64, the text the
	// agent signs as it is.
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Issue creates a challenge for the organization and the user that asked, valid
// for domain.RegistrationChallengeLifetime.
func (s *RegistrationChallengeService) Issue(ctx context.Context, orgID, principalID uuid.UUID) (*IssuedRegistrationChallenge, error) {
	raw := make([]byte, domain.RegistrationChallengeSize)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate registration challenge: %w", err)
	}
	issuedAt := s.now().UTC()
	challenge := &domain.RegistrationChallenge{
		ID:             uuid.New(),
		OrganizationID: orgID,
		PrincipalID:    principalID,
		Challenge:      raw,
		IssuedAt:       issuedAt,
		ExpiresAt:      issuedAt.Add(domain.RegistrationChallengeLifetime),
	}
	if err := s.repo.Create(ctx, challenge); err != nil {
		return nil, err
	}
	return &IssuedRegistrationChallenge{
		ID:        challenge.ID,
		Challenge: base64.StdEncoding.EncodeToString(raw),
		ExpiresAt: challenge.ExpiresAt,
	}, nil
}

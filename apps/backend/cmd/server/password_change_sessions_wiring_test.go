package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The auth middleware reads the user record behind each access token it
// accepts through the JWT service's session-state reader: a password change
// ends the sessions signed in before it, and while an account must change its
// password its session is admitted only to GET /api/v1/auth/me and
// POST /api/v1/auth/change-password. A JWT service with no reader reads
// nothing and refuses nothing, so the server must attach the user repository.
func TestServer_AttachesTheUserRecordToTheJWTService(t *testing.T) {
	var _ auth.SessionStateReader = (*repository.UserRepository)(nil)
	src := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	assert.Equal(t, 1, strings.Count(src, "jwtService.SetSessionStateReader(repos.User)"),
		"cmd/server/main.go attaches the user repository as the session-state reader once")
}

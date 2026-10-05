package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sdkTokenListRepo is a minimal domain.SDKTokenRepository stub that only
// implements GetByUserID, the one method ListUserTokens reaches.
type sdkTokenListRepo struct {
	domain.SDKTokenRepository
	tokens []*domain.SDKToken
}

func (r *sdkTokenListRepo) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	return r.tokens, nil
}

func listSDKTokens(t *testing.T, tokens []*domain.SDKToken) map[string][]map[string]interface{} {
	t.Helper()
	handler := &SDKTokenHandler{sdkTokenService: application.NewSDKTokenService(&sdkTokenListRepo{tokens: tokens})}

	app := fiber.New()
	app.Get("/users/me/sdk-tokens", withSDKTokenContext(handler.ListUserTokens))

	resp, err := app.Test(httptest.NewRequest("GET", "/users/me/sdk-tokens", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string][]map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &out), "body: %s", body)
	return out
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// A token created by refresh-token rotation stores its lineage under
// snake_case keys. The list response pins the token shape and maps every
// metadata key to camelCase, so the API keeps one naming convention.
func TestSDKTokenHandler_ListUserTokens_RotatedTokenShape(t *testing.T) {
	parentID := uuid.New()
	deviceName := "laptop"
	now := time.Now().UTC().Truncate(time.Second)

	rotated := &domain.SDKToken{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OrganizationID: uuid.New(),
		TokenHash:      "stored-hash-never-returned",
		TokenID:        "jti-new",
		DeviceName:     &deviceName,
		UsageCount:     3,
		CreatedAt:      now,
		ExpiresAt:      now.Add(90 * 24 * time.Hour),
		Metadata: map[string]interface{}{
			"source":        "token_rotation",
			"rotated_from":  "jti-old",
			"rotationCount": float64(2),
			"parent_token":  parentID.String(),
		},
	}

	out := listSDKTokens(t, []*domain.SDKToken{rotated})
	require.Len(t, out["tokens"], 1)
	token := out["tokens"][0]

	assert.Equal(t, []string{
		"createdAt", "deviceName", "expiresAt", "id", "metadata",
		"organizationId", "tokenId", "usageCount", "userId",
	}, sortedKeys(token), "token response shape")

	metadata, ok := token["metadata"].(map[string]interface{})
	require.True(t, ok, "metadata must be a JSON object, got %T", token["metadata"])
	assert.Equal(t, map[string]interface{}{
		"source":        "token_rotation",
		"rotatedFrom":   "jti-old",
		"rotationCount": float64(2),
		"parentToken":   parentID.String(),
	}, metadata)

	for key := range metadata {
		assert.False(t, strings.Contains(key, "_"), "metadata key %q is not camelCase", key)
	}

	// The handler must not rewrite the row it was handed.
	assert.Contains(t, rotated.Metadata, "parent_token")
	assert.Contains(t, rotated.Metadata, "rotated_from")
}

// A key already stored in camelCase wins over a snake_case key that maps
// to the same name, whatever the map's iteration order.
func TestSDKTokenHandler_ListUserTokens_CamelCaseKeyWinsOnCollision(t *testing.T) {
	token := &domain.SDKToken{
		ID: uuid.New(),
		Metadata: map[string]interface{}{
			"parent_token": "snake",
			"parentToken":  "camel",
		},
	}

	for i := 0; i < 20; i++ {
		out := listSDKTokens(t, []*domain.SDKToken{token})
		require.Len(t, out["tokens"], 1)
		assert.Equal(t, map[string]interface{}{"parentToken": "camel"}, out["tokens"][0]["metadata"])
	}
}

// A token with no metadata keeps omitting the field.
func TestSDKTokenHandler_ListUserTokens_NoMetadataOmitsField(t *testing.T) {
	out := listSDKTokens(t, []*domain.SDKToken{{ID: uuid.New()}})
	require.Len(t, out["tokens"], 1)
	assert.NotContains(t, out["tokens"][0], "metadata")
}

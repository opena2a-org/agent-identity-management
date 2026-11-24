package handlers

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The TypeScript and Java SDKs send a card update as the request pinned in
// sdk/typescript/tests/fixtures/sdk_requests/a2a_update_agent_card.json;
// updateAgentCard.wire.test.ts and A2AAgentCardUpdateWireTest hold each client
// to it. The test below holds the server to it: PUT /a2a/cards/:id stores the
// document the request carries and answers its skills as strings. Both SDKs
// sent the card fields themselves, skills as objects, which the route does not
// read and answered 400 (#652).

func TestSDKAgentCardUpdateRequestIsStoredByUpdateAgentCard(t *testing.T) {
	fx := readTSRequestFixture(t, "a2a_update_agent_card")
	require.Equal(t, fiber.MethodPut, fx.Method)
	require.Equal(t, "/api/v1/a2a/cards/{agentId}", fx.Path)
	require.Equal(t, "A2AAgentCardParsed", fx.RequestType)

	var body struct {
		CardData json.RawMessage `json:"cardData"`
	}
	decodeTSBody(t, fx, &body)
	var document domain.A2AAgentCardParsed
	decodeTSBody(t, pqcRequestFixture{Body: body.CardData, RequestType: fx.RequestType}, &document)
	require.NotEmpty(t, document.Skills, "the pinned document declares a skill")

	f := newCardReplaceFixture(t, asTheAgent)
	var data, hash driver.Value
	f.expectAgentRead()
	f.expectCardUpsert(&data, &hash)
	f.expectSkillUpserts(len(document.Skills))

	status, out := f.put(t, string(fx.Body))

	require.Equal(t, fiber.StatusOK, status, "answer: %v", out)
	skills := make([]interface{}, 0, len(document.Skills))
	for _, skill := range document.Skills {
		skills = append(skills, skill.ID)
	}
	assert.Equal(t, skills, out["skills"], "the answer names the document's skills by identifier, as strings")
	assert.Equal(t, f.agentID.String(), out["agentId"])
	assert.True(t, strings.HasPrefix(out["cardUrl"].(string), "inline://"), "cardUrl: %v", out["cardUrl"])
	_, err := uuid.Parse(out["cardId"].(string))
	assert.NoError(t, err, "cardId: %v", out["cardId"])
	assert.JSONEq(t, string(body.CardData), string(data.([]byte)), "the stored document is the one the request carries")
	assert.Equal(t, cardHashOf(string(body.CardData)), hash)
	assert.NoError(t, f.mock.ExpectationsWereMet())
}

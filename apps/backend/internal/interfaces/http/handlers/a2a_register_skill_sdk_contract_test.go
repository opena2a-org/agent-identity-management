package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The TypeScript SDK registers a skill as the request pinned in
// sdk/typescript/tests/fixtures/sdk_requests/a2a_register_skill.json;
// registerSkill.wire.test.ts holds the client to it. The test below holds the
// server to it: POST /a2a/skills binds every member, the example as a string,
// and stores the skill. The client sent examples as objects, which the route
// cannot bind and answered 400 (#652).

func TestSDKRegisterSkillRequestWithOneExampleIsAccepted(t *testing.T) {
	fx := readTSRequestFixture(t, "a2a_register_skill")
	require.Equal(t, fiber.MethodPost, fx.Method)
	require.Equal(t, "/api/v1/a2a/skills", fx.Path)

	var body struct {
		AgentID     string   `json:"agentId"`
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		InputModes  []string `json:"inputModes"`
		OutputModes []string `json:"outputModes"`
		Examples    []string `json:"examples"`
	}
	decodeTSBody(t, fx, &body)
	require.Len(t, body.Examples, 1, "the pinned request carries one example")

	site := bodyChannelSites[0]
	require.Equal(t, "RegisterSkill", site.name)
	f := newBodyChannelFixture(true)
	status, raw := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
		c.Locals("organization_id", f.callerOrg)
		c.Locals("agent_id", f.ownedAgent)
	}, string(fx.Body))

	require.Equal(t, fiber.StatusCreated, status, "answer: %s", raw)
	skill := f.writer.registeredSkill
	require.NotNil(t, skill, "the skill reaches the service")
	assert.Equal(t, f.ownedAgent, skill.AgentID)
	assert.Equal(t, body.ID, skill.SkillID)
	assert.Equal(t, body.Name, skill.Name)
	assert.Equal(t, body.Description, skill.Description)
	assert.Equal(t, body.Tags, skill.Tags)
	assert.Equal(t, body.InputModes, skill.InputModes)
	assert.Equal(t, body.OutputModes, skill.OutputModes)
	assert.Equal(t, body.Examples, skill.Examples, "the example is stored as the string the SDK sent")
	assert.Contains(t, raw, `"examples":["Summarize the nightly report"]`)
}

func TestRegisterSkillRefusesAnExampleObject(t *testing.T) {
	site := bodyChannelSites[0]
	f := newBodyChannelFixture(true)
	status, raw := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
		c.Locals("organization_id", f.callerOrg)
		c.Locals("agent_id", f.ownedAgent)
	}, `{"name":"Add","examples":[{"name":"add","input":{"a":2}}]}`)

	assert.Equal(t, fiber.StatusBadRequest, status, "answer: %s", raw)
	assert.Nil(t, f.writer.registeredSkill, "nothing is stored")
}

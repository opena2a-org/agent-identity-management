package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listEventsRepo answers the two reads behind ListVerificationEvents, the
// organization's events and one agent's events, with the same result, and
// records which read was made.
type listEventsRepo struct {
	domain.VerificationEventRepository
	events []*domain.VerificationEvent
	total  int
	err    error
	reads  []string
}

func (r *listEventsRepo) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	r.reads = append(r.reads, "organization")
	return r.events, r.total, r.err
}

func (r *listEventsRepo) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	r.reads = append(r.reads, "agent")
	return r.events, r.total, r.err
}

// listEventsResponse calls ListVerificationEvents as a caller of orgID, with
// agentID an agent of that organization, and returns the status and the
// decoded JSON body.
func listEventsResponse(t *testing.T, orgID, agentID uuid.UUID, events *listEventsRepo, target string) (int, map[string]any) {
	t.Helper()
	agents := &verificationEventTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			if id != agentID {
				return nil, errors.New("agent not found")
			}
			return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
		},
	}
	h := NewVerificationEventHandler(application.NewVerificationEventService(events, nil, nil), agents, nil)
	app := fiber.New()
	app.Get("/verification-events", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return h.ListVerificationEvents(c)
	})

	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body), "body: %s", raw)
	return resp.StatusCode, body
}

// The list filtered by agent_id answers a failed read and an offset that is
// not an integer the way the organization's list does: a failed read is a 500,
// and the offset is read as 0.
func TestListVerificationEvents_AgentFilterAnswersLikeTheOrganizationList(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()

	for _, filter := range []struct {
		name  string
		query string
	}{
		{name: "organization", query: ""},
		{name: "agent", query: "agent_id=" + agentID.String() + "&"},
	} {
		t.Run(filter.name+"/failed read", func(t *testing.T) {
			events := &listEventsRepo{err: errors.New("read failed")}

			status, body := listEventsResponse(t, orgID, agentID, events, "/verification-events?"+filter.query+"limit=10")

			assert.Equal(t, []string{filter.name}, events.reads)
			assert.Equal(t, fiber.StatusInternalServerError, status)
			assert.Equal(t, map[string]any{"error": "Failed to retrieve verification events"}, body)
		})

		for _, offset := range []string{"abc", "1.5"} {
			t.Run(filter.name+"/offset "+offset, func(t *testing.T) {
				events := &listEventsRepo{
					events: []*domain.VerificationEvent{{ID: uuid.New(), OrganizationID: orgID}},
					total:  1,
				}

				status, body := listEventsResponse(t, orgID, agentID, events, "/verification-events?"+filter.query+"offset="+offset)

				assert.Equal(t, []string{filter.name}, events.reads)
				assert.Equal(t, fiber.StatusOK, status, "body: %v", body)
				assert.EqualValues(t, 0, body["offset"])
				assert.EqualValues(t, 50, body["limit"])
				assert.EqualValues(t, 1, body["total"])
				assert.Len(t, body["events"], 1)
			})
		}
	}
}

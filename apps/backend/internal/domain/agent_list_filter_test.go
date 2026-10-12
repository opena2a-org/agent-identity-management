package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func agentsForListFilter() []*Agent {
	return []*Agent{
		{Name: "a", Status: AgentStatusVerified, AgentType: AgentTypeLangChain},
		{Name: "b", Status: AgentStatusPending, AgentType: AgentTypeLangChain},
		{Name: "c", Status: AgentStatusVerified, AgentType: AgentTypeCrewAI},
		{Name: "d", Status: AgentStatusSuspended, AgentType: AgentTypeAI},
		{Name: "e", Status: AgentStatusVerified, AgentType: AgentTypeLangChain},
	}
}

func agentNames(agents []*Agent) []string {
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	return names
}

// An empty field does not narrow the list, a set field keeps the agents that
// hold exactly its value, and both set keep the agents that hold both.
func TestAgentListFilter_KeepsTheAgentsThatHoldItsValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter AgentListFilter
		want   []string
	}{
		{name: "empty", filter: AgentListFilter{}, want: []string{"a", "b", "c", "d", "e"}},
		{name: "status", filter: AgentListFilter{Status: AgentStatusVerified}, want: []string{"a", "c", "e"}},
		{name: "agent type", filter: AgentListFilter{AgentType: AgentTypeLangChain}, want: []string{"a", "b", "e"}},
		{name: "status and agent type", filter: AgentListFilter{Status: AgentStatusVerified, AgentType: AgentTypeLangChain}, want: []string{"a", "e"}},
		{name: "a status no agent holds", filter: AgentListFilter{Status: "no-such-status"}, want: []string{}},
		{name: "a type no agent holds", filter: AgentListFilter{AgentType: "no-such-type"}, want: []string{}},
		{name: "another case of a status", filter: AgentListFilter{Status: "Verified"}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, total := tc.filter.Page(agentsForListFilter(), 0, 0)
			assert.Equal(t, tc.want, agentNames(page))
			assert.Equal(t, len(tc.want), total)
		})
	}
}

// The page is cut from the agents that pass, and the total counts all of
// them on every page.
func TestAgentListFilter_PagesTheAgentsThatPass(t *testing.T) {
	verified := AgentListFilter{Status: AgentStatusVerified}
	for _, tc := range []struct {
		name          string
		limit, offset int
		want          []string
	}{
		{name: "first page", limit: 2, offset: 0, want: []string{"a", "c"}},
		{name: "second page", limit: 2, offset: 2, want: []string{"e"}},
		{name: "past the end", limit: 2, offset: 3, want: []string{}},
		{name: "a limit above the count", limit: 50, offset: 1, want: []string{"c", "e"}},
		{name: "no limit ignores the offset", limit: 0, offset: 2, want: []string{"a", "c", "e"}},
		{name: "a negative offset is the start", limit: 1, offset: -3, want: []string{"a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, total := verified.Page(agentsForListFilter(), tc.limit, tc.offset)
			assert.Equal(t, tc.want, agentNames(page))
			assert.Equal(t, 3, total)
		})
	}
}

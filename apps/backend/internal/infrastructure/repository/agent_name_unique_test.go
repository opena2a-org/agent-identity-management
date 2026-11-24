package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// An insert the agents name constraint refuses is ErrAgentNameTaken; any other
// error, a unique violation of another constraint included, passes unchanged.
func TestAgentInsertErrorMapsOnlyTheNameConstraint(t *testing.T) {
	nameTaken := &pq.Error{Code: "23505", Constraint: agentNameConstraint}
	assert.ErrorIs(t, agentInsertError(nameTaken), ErrAgentNameTaken)
	assert.ErrorIs(t, agentInsertError(fmt.Errorf("insert agent: %w", nameTaken)), ErrAgentNameTaken)

	otherUnique := &pq.Error{Code: "23505", Constraint: "agents_pkey"}
	assert.Same(t, otherUnique, agentInsertError(otherUnique))

	foreignKey := &pq.Error{Code: "23503", Constraint: agentNameConstraint}
	assert.Same(t, foreignKey, agentInsertError(foreignKey))

	other := errors.New("connection reset")
	assert.Same(t, other, agentInsertError(other))
	assert.NoError(t, agentInsertError(nil))
}

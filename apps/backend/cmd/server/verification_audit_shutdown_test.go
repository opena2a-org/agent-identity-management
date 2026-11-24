package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// recordingAuditQueue stands in for the verification audit queue at shutdown.
type recordingAuditQueue struct {
	hadDeadline bool
	err         error
}

func (q *recordingAuditQueue) Close(ctx context.Context) error {
	_, q.hadDeadline = ctx.Deadline()
	return q.err
}

func TestCloseVerificationAuditBoundsTheDrainAndLogsAFailure(t *testing.T) {
	buf := captureLog(t)
	queue := &recordingAuditQueue{err: errors.New("2 audit entries not written at shutdown")}
	closeVerificationAudit(queue, time.Second)
	assert.True(t, queue.hadDeadline, "the drain is bounded, so a database that stopped answering cannot hold shutdown open")
	assert.Contains(t, buf.String(), "2 audit entries not written at shutdown")
}

func TestCloseVerificationAuditWithoutAQueue(t *testing.T) {
	closeVerificationAudit(nil, time.Second)
	var queue *application.AuditQueue
	require.NoError(t, queue.Close(context.Background()))
	closeVerificationAudit(queue, time.Second)
}

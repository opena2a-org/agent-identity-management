//go:build integration

package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The verification audit queue against the audit_logs table: its batches are
// multi-row inserts, a row the table refuses (here an organization that does
// not exist) does not take the batch with it, every other entry is stored
// with the id and time Log gave it, and every stored entry reaches OnCreate,
// which is how the SIEM sees it.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestVerificationAuditQueueRoundTrip ./internal/application/...

func TestVerificationAuditQueueRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping verification audit queue round trip")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	ctx := context.Background()

	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	suffix := orgID.String()[:8]
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})
	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
		orgID, "audit-queue-org-"+suffix, "audit-queue-"+suffix+".invalid")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, provider, provider_id)
		 VALUES ($1, $2, $3, 'audit queue user', 'local', $4)`,
		userID, orgID, "audit-queue-"+suffix+"@example.invalid", userID.String())
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id)
		 VALUES ($1, $2, $2, 'service', $3, $4)`,
		agentID, "audit-queue-agent-"+suffix, userID, orgID)
	require.NoError(t, err)

	prevLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prevLog) })

	repo := repository.NewAuditLogRepository(db)
	var mu sync.Mutex
	given := map[uuid.UUID]int{}
	repo.OnCreate(func(entry *domain.AuditLog) {
		mu.Lock()
		defer mu.Unlock()
		given[entry.ID]++
	})
	queue := NewAuditQueue(repo, AuditQueueConfig{MaxBatch: 25, FlushInterval: time.Hour, MaxAttempts: 2})

	const decisions = 60
	ids := make([]uuid.UUID, 0, decisions)
	var refused *domain.AuditLog
	for i := 0; i < decisions; i++ {
		entry := &domain.AuditLog{
			OrganizationID: orgID,
			UserID:         &userID,
			AgentID:        &agentID,
			Action:         "db:write",
			ResourceType:   "agent_action",
			ResourceID:     agentID,
			IPAddress:      "203.0.113.7",
			UserAgent:      "aim-sdk-python/1.0",
			Metadata:       map[string]interface{}{"n": i, "autoApproved": i%2 == 0},
		}
		require.NoError(t, queue.Log(ctx, entry))
		ids = append(ids, entry.ID)
		if i == decisions/2 {
			// In the middle of a batch, an entry the table refuses.
			refused = &domain.AuditLog{
				OrganizationID: uuid.New(),
				Action:         "db:write",
				ResourceType:   "agent_action",
				ResourceID:     agentID,
			}
			require.NoError(t, queue.Log(ctx, refused))
		}
	}

	closeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	require.Error(t, queue.Close(closeCtx), "Close reports the entry the table refused")
	assert.EqualValues(t, 1, queue.Lost())
	assert.EqualValues(t, decisions, queue.Written())

	rows, err := db.QueryContext(ctx,
		`SELECT id, metadata::text, timestamp FROM audit_logs WHERE organization_id = $1`, orgID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	stored := map[uuid.UUID]map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var metadata string
		var at time.Time
		require.NoError(t, rows.Scan(&id, &metadata, &at))
		var decoded map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(metadata), &decoded))
		stored[id] = decoded
		assert.WithinDuration(t, time.Now(), at, 5*time.Minute)
	}
	require.NoError(t, rows.Err())

	require.Len(t, stored, decisions, "every entry but the refused one is stored")
	for i, id := range ids {
		require.Contains(t, stored, id)
		assert.EqualValues(t, i, stored[id]["n"])
		assert.Equal(t, i%2 == 0, stored[id]["autoApproved"])
	}
	var refusedRows int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM audit_logs WHERE id = $1`, refused.ID).Scan(&refusedRows))
	assert.Zero(t, refusedRows)

	mu.Lock()
	for _, id := range ids {
		assert.Equal(t, 1, given[id], "each stored entry reaches OnCreate once")
	}
	assert.NotContains(t, given, refused.ID)
	mu.Unlock()

	// A batch given again after a write whose outcome was not known skips
	// the rows already stored.
	first := &domain.AuditLog{ID: ids[0], OrganizationID: orgID, Action: "db:write", ResourceType: "agent_action", ResourceID: agentID}
	require.NoError(t, repo.CreateBatch([]*domain.AuditLog{first}))
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM audit_logs WHERE organization_id = $1`, orgID).Scan(&count))
	assert.Equal(t, decisions, count)
}

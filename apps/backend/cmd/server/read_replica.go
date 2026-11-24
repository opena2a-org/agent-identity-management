package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/database"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// wireReadReplica points the verification path's reads at the read replica
// POSTGRES_REPLICA_URL names. It returns the replica pool for the caller to
// close, or nil when no replica is configured or it cannot be opened, in which
// case every read stays on the primary.
//
// A replica lags the primary, so each verification asks the replica fence
// first: the replica answers only when it has replayed every write the primary
// had committed, and the primary answers otherwise. A revocation therefore
// takes effect on the next verification, replica or not. Writes, and every
// read outside the verification, stay on the primary.
func wireReadReplica(cfg config.DatabaseConfig, primary *sql.DB, census *database.Census, agents *application.AgentService, repos *Repositories) *sql.DB {
	if cfg.ReplicaURL == "" {
		return nil
	}
	replica, err := database.OpenReplicaPool(cfg.ReplicaURL, cfg.PoolRole, census)
	if err != nil {
		// The setting holds a password, so the driver's error is not logged.
		log.Println("Read replica not used: POSTGRES_REPLICA_URL is not a valid connection string")
		return nil
	}
	setPoolLimits(replica, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := replica.PingContext(ctx); err != nil {
		// The pool stays: the fence sends every read to the primary while the
		// replica cannot answer, and the replica serves again when it can.
		log.Printf("Read replica unreachable at startup (verification reads use the primary until it answers): %v", err)
	}
	fence, err := database.NewReplicaFence(primary, replica)
	if err != nil {
		_ = replica.Close()
		log.Printf("Read replica not used: %v", err)
		return nil
	}

	replicaX := sqlx.NewDb(replica, "postgres")
	agents.SetVerifyReads(&application.ReplicaVerifyReads{
		Primary: application.VerifyReads{Agent: repos.Agent, Capability: repos.Capability, Organization: repos.Organization},
		Replica: application.VerifyReads{
			Agent:        repository.NewAgentRepository(replica),
			Capability:   repository.NewCapabilityRepository(replicaX),
			Organization: repository.NewOrganizationRepository(replica),
		},
		Fence: fence.Replica,
	})
	log.Println("Read replica configured: verification reads use it while it holds every committed write, the primary otherwise")
	return replica
}

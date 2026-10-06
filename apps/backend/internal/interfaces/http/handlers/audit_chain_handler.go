package handlers

import (
	"log"

	"github.com/gofiber/fiber/v3"

	recordstore "github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// AuditChainHandler serves an organization's audit record chain to the
// organization's admins.
type AuditChainHandler struct {
	db recordstore.Querier
}

// NewAuditChainHandler returns a handler that reads chains through db.
func NewAuditChainHandler(db recordstore.Querier) *AuditChainHandler {
	return &AuditChainHandler{db: db}
}

// chainHeadResponse is the body of GET /admin/audit-logs/chain/head.
type chainHeadResponse struct {
	recordstore.ChainStatus
	// LatestCheckpoint is always null: no chain checkpoint is written yet.
	LatestCheckpoint *struct{} `json:"latestCheckpoint"`
}

// GetChainHead reports the state of the caller's organization's chain, as
// the one chain-state read of the record store reads it: chainState, chainId,
// head and latestCheckpoint, and reason when the chain cannot be extended.
// An organization with no chain gets 200 with chainState notStarted and
// chainId, head and latestCheckpoint null.
//
// GET /api/v1/admin/audit-logs/chain/head
func (h *AuditChainHandler) GetChainHead(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	status, err := recordstore.ReadChainState(c.Context(), h.db, orgID.String())
	if err != nil {
		log.Printf("audit chain head: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read the audit record chain",
		})
	}
	return c.JSON(chainHeadResponse{ChainStatus: status})
}

package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// webhookEventDispatcher is the part of WebhookService that the services
// sending webhook events use.
type webhookEventDispatcher interface {
	TriggerEvent(ctx context.Context, orgID uuid.UUID, event domain.WebhookEvent, data map[string]interface{}) error
}

// asWebhookEventDispatcher returns webhookService as a dispatcher, or nil for a
// nil service: a nil *WebhookService stored in the interface would compare
// non-nil, and the services would call through it.
func asWebhookEventDispatcher(webhookService *WebhookService) webhookEventDispatcher {
	if webhookService == nil {
		return nil
	}
	return webhookService
}

// sendWebhookEvent hands event and its data to d for the webhooks of orgID. It
// does nothing when d is nil. The data must be built before the call: the
// dispatch runs on its own goroutine, after the request that caused the event
// may have returned, so it uses a background context and not the request's.
func sendWebhookEvent(d webhookEventDispatcher, orgID uuid.UUID, event domain.WebhookEvent, data map[string]interface{}) {
	if d == nil {
		return
	}
	go func() {
		if err := d.TriggerEvent(context.Background(), orgID, event, data); err != nil {
			fmt.Printf("Failed to trigger %s webhook: %v\n", event, err)
		}
	}()
}

// agentEventData is the data of an agent.* event: the agent as it stands
// after the change.
func agentEventData(agent *domain.Agent) map[string]interface{} {
	return map[string]interface{}{
		"agentId":     agent.ID.String(),
		"name":        agent.Name,
		"displayName": agent.DisplayName,
		"agentType":   string(agent.AgentType),
		"status":      string(agent.Status),
		"trustScore":  agent.TrustScore,
	}
}

// trustScoreEventData is the data of a trust_score.* event.
func trustScoreEventData(agent *domain.Agent, previousScore, trustScore float64) map[string]interface{} {
	return map[string]interface{}{
		"agentId":       agent.ID.String(),
		"agentName":     agent.Name,
		"previousScore": previousScore,
		"trustScore":    trustScore,
	}
}

// apiKeyEventData is the data of an api_key.* event. It never carries the key
// or its hash.
func apiKeyEventData(key *domain.APIKey) map[string]interface{} {
	data := map[string]interface{}{
		"apiKeyId": key.ID.String(),
		"agentId":  key.AgentID.String(),
		"name":     key.Name,
		"prefix":   key.Prefix,
	}
	if key.ExpiresAt != nil {
		data["expiresAt"] = *key.ExpiresAt
	}
	return data
}

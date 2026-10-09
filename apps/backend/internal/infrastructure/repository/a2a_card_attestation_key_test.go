package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

var cardColumns = []string{
	"id", "agent_id", "card_url", "card_data", "card_hash", "protocol_version",
	"attestation_signature", "attestation_issued_at", "attestation_expires_at",
	"is_valid", "validation_error", "last_fetched_at", "fetch_count", "created_at", "updated_at",
	"attestation_key_id", "attestation_alg",
}

func TestCardCreateWritesTheAttestationKeyIDAndAlg(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	kid := "4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f"
	alg := "EdDSA"
	now := time.Now().UTC()
	card := &domain.A2AAgentCard{
		ID:                   uuid.New(),
		AgentID:              uuid.New(),
		CardURL:              "inline://agent",
		CardData:             []byte(`{"name":"card"}`),
		CardHash:             "hash",
		AttestationSignature: "c2lnbmF0dXJl",
		AttestationKeyID:     &kid,
		AttestationAlg:       &alg,
		AttestationIssuedAt:  &now,
		AttestationExpiresAt: &now,
		IsValid:              true,
	}

	mock.ExpectQuery(regexp.QuoteMeta("attestation_key_id = EXCLUDED.attestation_key_id")).
		WithArgs(
			card.ID, card.AgentID, card.CardURL, sqlmock.AnyArg(), card.CardHash, card.ProtocolVersion,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			true, sqlmock.AnyArg(), sqlmock.AnyArg(), 0, sqlmock.AnyArg(), sqlmock.AnyArg(),
			kid, alg,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "fetch_count"}).
			AddRow(card.ID, now, now, 1))

	require.NoError(t, NewA2AAgentCardRepository(db).Create(context.Background(), card))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCardReadReturnsNilKeyIDForAttestationsIssuedBeforeKeyIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	oldCard, newCard := uuid.New(), uuid.New()
	kid := "4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f4f0c1a2b3c4d5e6f"

	mock.ExpectQuery(regexp.QuoteMeta("attestation_key_id, attestation_alg")).
		WithArgs(oldCard).
		WillReturnRows(sqlmock.NewRows(cardColumns).AddRow(
			uuid.New(), oldCard, "inline://old", []byte(`{}`), "hash", "1.0.0",
			"c2ln", now, now, true, nil, now, 1, now, now,
			nil, nil,
		))
	mock.ExpectQuery(regexp.QuoteMeta("attestation_key_id, attestation_alg")).
		WithArgs(newCard).
		WillReturnRows(sqlmock.NewRows(cardColumns).AddRow(
			uuid.New(), newCard, "inline://new", []byte(`{}`), "hash", "1.0.0",
			"c2ln", now, now, true, nil, now, 1, now, now,
			kid, "EdDSA",
		))

	repo := NewA2AAgentCardRepository(db)

	old, err := repo.GetByAgentID(context.Background(), oldCard)
	require.NoError(t, err)
	assert.Nil(t, old.AttestationKeyID)
	assert.Nil(t, old.AttestationAlg)
	assert.Equal(t, "c2ln", old.AttestationSignature, "an earlier attestation keeps its signature")

	fresh, err := repo.GetByAgentID(context.Background(), newCard)
	require.NoError(t, err)
	require.NotNil(t, fresh.AttestationKeyID)
	assert.Equal(t, kid, *fresh.AttestationKeyID)
	require.NotNil(t, fresh.AttestationAlg)
	assert.Equal(t, "EdDSA", *fresh.AttestationAlg)

	require.NoError(t, mock.ExpectationsWereMet())
}

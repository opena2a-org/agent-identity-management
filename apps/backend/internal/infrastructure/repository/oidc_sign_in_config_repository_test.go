package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

func newOIDCSignInConfigRepo(t *testing.T) (*OIDCSignInConfigRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return NewOIDCSignInConfigRepository(db), mock
}

// Each query names the key of one organization, so a configuration is read,
// written or removed only for the organization it belongs to.
func TestOIDCSignInConfigRepository_PutWritesTheOrganizationsRow(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org, actor := uuid.New(), uuid.New()
	cfg := &domain.OIDCSignInConfig{OrganizationID: org, Issuer: "https://idp.example.com", ClientID: "aim-client",
		SealedClientSecret: "sealed", UpdatedBy: actor}

	want, err := json.Marshal(map[string]string{"issuer": "https://idp.example.com", "clientId": "aim-client",
		"sealedClientSecret": "sealed", "updatedBy": actor.String()})
	require.NoError(t, err)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO system_config (key, value, description, updated_at)")).
		WithArgs("oidc_sign_in:"+org.String(), jsonArg{want: string(want)}, "OIDC sign-in configuration of organization "+org.String()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Put(context.Background(), cfg))
}

func TestOIDCSignInConfigRepository_PutRefusesAConfigurationWithoutAnOrganization(t *testing.T) {
	repo, _ := newOIDCSignInConfigRepo(t)
	assert.Error(t, repo.Put(context.Background(), &domain.OIDCSignInConfig{Issuer: "https://idp.example.com"}))
}

func TestOIDCSignInConfigRepository_GetReadsTheOrganizationsRow(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org, actor := uuid.New(), uuid.New()
	updated := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT value, updated_at FROM system_config WHERE key = $1")).
		WithArgs("oidc_sign_in:" + org.String()).
		WillReturnRows(sqlmock.NewRows([]string{"value", "updated_at"}).AddRow(
			`{"issuer":"https://idp.example.com","clientId":"aim-client","sealedClientSecret":"sealed","updatedBy":"`+actor.String()+`"}`,
			updated))

	cfg, err := repo.Get(context.Background(), org)
	require.NoError(t, err)
	assert.Equal(t, &domain.OIDCSignInConfig{OrganizationID: org, Issuer: "https://idp.example.com", ClientID: "aim-client",
		SealedClientSecret: "sealed", UpdatedBy: actor, UpdatedAt: updated}, cfg)
}

func TestOIDCSignInConfigRepository_GetWithoutARowIsNotConfigured(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT value, updated_at FROM system_config WHERE key = $1")).
		WithArgs("oidc_sign_in:" + org.String()).
		WillReturnError(sql.ErrNoRows)

	_, err := repo.Get(context.Background(), org)
	assert.ErrorIs(t, err, domain.ErrOIDCSignInNotConfigured)
}

func TestOIDCSignInConfigRepository_DeleteRemovesTheOrganizationsRow(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org := uuid.New()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM system_config WHERE key = $1")).
		WithArgs("oidc_sign_in:" + org.String()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, repo.Delete(context.Background(), org))
}

// jsonArg matches a query argument that is the JSON document want, whatever
// the order of its members.
type jsonArg struct{ want string }

func (a jsonArg) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	var got, want map[string]interface{}
	if json.Unmarshal([]byte(s), &got) != nil || json.Unmarshal([]byte(a.want), &want) != nil {
		return false
	}
	return assert.ObjectsAreEqual(want, got)
}

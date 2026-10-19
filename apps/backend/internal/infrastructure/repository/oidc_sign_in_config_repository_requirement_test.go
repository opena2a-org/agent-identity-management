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

// A configuration that requires provider sign-in carries
// "requireProviderSignIn": true in the row's document; one that does not
// writes the document it wrote before the requirement existed
// (TestOIDCSignInConfigRepository_PutWritesTheOrganizationsRow), and such a
// document reads as requiring none.

func TestOIDCSignInConfigRepository_PutWritesTheRequirementOfProviderSignIn(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org, actor := uuid.New(), uuid.New()
	cfg := &domain.OIDCSignInConfig{OrganizationID: org, Issuer: "https://idp.example.com", ClientID: "aim-client",
		SealedClientSecret: "sealed", UpdatedBy: actor, RequireProviderSignIn: true}

	want := `{"issuer":"https://idp.example.com","clientId":"aim-client","sealedClientSecret":"sealed","updatedBy":"` + actor.String() +
		`","requireProviderSignIn":true}`
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO system_config (key, value, description, updated_at)")).
		WithArgs("oidc_sign_in:"+org.String(), jsonArg{want: want}, "OIDC sign-in configuration of organization "+org.String()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Put(context.Background(), cfg))
}

func TestOIDCSignInConfigRepository_GetReadsTheRequirementOfProviderSignIn(t *testing.T) {
	org, actor := uuid.New(), uuid.New()
	updated := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	document := `{"issuer":"https://idp.example.com","clientId":"aim-client","sealedClientSecret":"sealed","updatedBy":"` +
		actor.String() + `"`

	for name, tc := range map[string]struct {
		document string
		want     bool
	}{
		"required":                          {document + `,"requireProviderSignIn":true}`, true},
		"not required":                      {document + `,"requireProviderSignIn":false}`, false},
		"stored before the requirement was": {document + `}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			repo, mock := newOIDCSignInConfigRepo(t)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT value, updated_at FROM system_config WHERE key = $1")).
				WithArgs("oidc_sign_in:" + org.String()).
				WillReturnRows(sqlmock.NewRows([]string{"value", "updated_at"}).AddRow(tc.document, updated))

			cfg, err := repo.Get(context.Background(), org)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.RequireProviderSignIn)
			assert.Equal(t, "https://idp.example.com", cfg.Issuer)
		})
	}
}

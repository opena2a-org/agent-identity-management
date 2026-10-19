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

// A configuration that provisions users carries its provisioning in the row's
// document; one that does not writes the document it wrote before
// provisioning existed (TestOIDCSignInConfigRepository_PutWritesTheOrganizationsRow).

func TestOIDCSignInConfigRepository_PutWritesTheProvisioning(t *testing.T) {
	repo, mock := newOIDCSignInConfigRepo(t)
	org, actor := uuid.New(), uuid.New()
	cfg := &domain.OIDCSignInConfig{OrganizationID: org, Issuer: "https://idp.example.com", ClientID: "aim-client",
		SealedClientSecret: "sealed", UpdatedBy: actor,
		Provisioning: domain.OIDCProvisioning{Enabled: true, Role: domain.RoleMember, EmailDomains: []string{"example.com", "example.org"}}}

	want := `{"issuer":"https://idp.example.com","clientId":"aim-client","sealedClientSecret":"sealed","updatedBy":"` + actor.String() +
		`","provisioning":{"enabled":true,"role":"member","emailDomains":["example.com","example.org"]}}`
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO system_config (key, value, description, updated_at)")).
		WithArgs("oidc_sign_in:"+org.String(), jsonArg{want: want}, "OIDC sign-in configuration of organization "+org.String()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Put(context.Background(), cfg))
}

func TestOIDCSignInConfigRepository_GetReadsTheProvisioning(t *testing.T) {
	org, actor := uuid.New(), uuid.New()
	updated := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	document := func(provisioning string) string {
		return `{"issuer":"https://idp.example.com","clientId":"aim-client","sealedClientSecret":"sealed","updatedBy":"` +
			actor.String() + `","provisioning":` + provisioning + `}`
	}

	for name, tc := range map[string]struct {
		provisioning string
		want         domain.OIDCProvisioning
	}{
		"enabled": {`{"enabled":true,"role":"viewer","emailDomains":["example.com"]}`,
			domain.OIDCProvisioning{Enabled: true, Role: domain.RoleViewer, EmailDomains: []string{"example.com"}}},
		"not enabled": {`{"enabled":false,"role":"viewer","emailDomains":["example.com"]}`, domain.OIDCProvisioning{}},
		"null":        {`null`, domain.OIDCProvisioning{}},
	} {
		t.Run(name, func(t *testing.T) {
			repo, mock := newOIDCSignInConfigRepo(t)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT value, updated_at FROM system_config WHERE key = $1")).
				WithArgs("oidc_sign_in:" + org.String()).
				WillReturnRows(sqlmock.NewRows([]string{"value", "updated_at"}).AddRow(document(tc.provisioning), updated))

			cfg, err := repo.Get(context.Background(), org)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Provisioning)
			assert.Equal(t, "https://idp.example.com", cfg.Issuer)
		})
	}
}

package handlers

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The TypeScript and Java SDKs send A2A security settings as the request pinned in
// sdk/typescript/tests/fixtures/sdk_requests/a2a_update_security_settings.json;
// securitySettings.wire.test.ts and A2ASecuritySettingsWireTest hold each client
// to it. The tests below hold the server to it: every member is one
// A2ASecuritySettings reads, every settings member is in it, and
// PUT /a2a/security/settings stores each value it carries. Both SDKs named
// request signing requireSignedRequests and the nonce window maxTimestampAge,
// which the server does not read (#652).

// a2aSecuritySettingsRecordMembers are the members of A2ASecuritySettings that
// describe the stored record rather than a setting.
var a2aSecuritySettingsRecordMembers = map[string]bool{
	"id": true, "organizationId": true, "createdBy": true, "updatedBy": true, "createdAt": true, "updatedAt": true,
}

var a2aSecuritySettingsColumns = []string{"id", "organization_id", "enforcement_mode", "require_verified_skills",
	"min_trust_score", "min_attestation_count", "require_request_signing",
	"require_nonce_validation", "nonce_validity_seconds", "allowed_agent_ids",
	"blocked_agent_ids", "max_requests_per_minute", "max_requests_per_hour",
	"created_by", "updated_by", "created_at", "updated_at"}

func TestSDKSecuritySettingsRequestCarriesEverySettingsMemberTheServerReads(t *testing.T) {
	fx := readTSRequestFixture(t, "a2a_update_security_settings")
	require.Equal(t, fiber.MethodPut, fx.Method)
	require.Equal(t, "/api/v1/a2a/security/settings", fx.Path)
	require.Equal(t, "A2ASecuritySettings", fx.RequestType)

	var settings domain.A2ASecuritySettings
	decodeTSBody(t, fx, &settings)

	typ := reflect.TypeOf(settings)
	var want []string
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if !a2aSecuritySettingsRecordMembers[name] {
			want = append(want, name)
		}
	}
	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fx.Body, &members))
	var got []string
	for name := range members {
		got = append(got, name)
	}
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got, "the SDK request carries every settings member of A2ASecuritySettings and no record member")
	assert.Len(t, want, typ.NumField()-len(a2aSecuritySettingsRecordMembers), "each record member is a member of A2ASecuritySettings")
}

func TestSDKSecuritySettingsRequestIsStoredByUpdateSecuritySettings(t *testing.T) {
	fx := readTSRequestFixture(t, "a2a_update_security_settings")
	orgID := uuid.New()
	db, mock := newSQLMock(t)

	// The stored settings differ from the request in every setting.
	now := time.Now()
	mock.ExpectQuery(`FROM a2a_security_settings WHERE organization_id = \$1`).WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows(a2aSecuritySettingsColumns).AddRow(
			uuid.New(), orgID, "monitor", false, 0.1, 0, false, false, 30,
			[]byte(`[]`), []byte(`[]`), 10, 100, nil, nil, now, now))
	mock.ExpectExec(`UPDATE a2a_security_settings SET`).
		WithArgs("strict", true, 0.7, 3, true, true, 300,
			[]byte(`["7c9e6679-7425-40de-944b-e07fc1f90ae7"]`),
			[]byte(`["550e8400-e29b-41d4-a716-446655440000"]`),
			60, 1000, nil, orgID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc := application.NewA2AService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		repository.NewA2ASecuritySettingsRepository(db),
		nil, nil, nil, nil, nil,
	)
	h := NewA2AHandler(svc, nil, nil)
	app := fiber.New()
	app.Put(fx.Path, func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return h.UpdateSecuritySettings(c)
	})

	status, got := sendTSRequest(t, app, fx.Method, fx.Path, fx.Body)

	require.Equal(t, fiber.StatusOK, status, "answer: %v", got)
	assert.Equal(t, true, got["requireRequestSigning"])
	assert.Equal(t, float64(300), got["nonceValiditySeconds"])
	assert.Equal(t, orgID.String(), got["organizationId"])
	assert.NoError(t, mock.ExpectationsWereMet(), "the update stores each value the request carries")
}

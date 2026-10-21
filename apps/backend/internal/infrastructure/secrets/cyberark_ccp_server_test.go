package secrets_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/cyberark/ccptest"
)

// The CyberArk CCP backend is run against ccptest, the CCP stub that serves
// TLS and requires a client certificate signed by its own CA.

const (
	ccpTestAppID = "AIM-Server"
	ccpTestSafe  = "AIM-Secrets"
)

func newStubCyberArkCCPBackend(t *testing.T, srv *ccptest.Server, mutate func(*secrets.CyberArkCCPConfig)) *secrets.CyberArkCCPBackend {
	t.Helper()
	cfg := &secrets.CyberArkCCPConfig{
		CCPURL:     srv.URL,
		AppID:      ccpTestAppID,
		Safe:       ccpTestSafe,
		CertPath:   srv.ClientCertPath,
		KeyPath:    srv.ClientKeyPath,
		CACertPath: srv.CACertPath,
	}
	if mutate != nil {
		mutate(cfg)
	}
	backend, err := secrets.NewCyberArkCCPBackend(cfg)
	require.NoError(t, err)
	return backend
}

// vaultForNamespace puts an account for the namespace in the stub, under the
// name the backend asks for.
func vaultForNamespace(srv *ccptest.Server, nsID uuid.UUID, content string) {
	srv.AddAccount(ccptest.Account{
		AppID: ccpTestAppID, Safe: ccpTestSafe, Object: "aim-secrets-" + nsID.String(),
		Content: content, UserName: "svc_aim",
	})
}

func TestCyberArkCCPBackendReadsTheNamespaceAccountThroughCCP(t *testing.T) {
	srv := ccptest.New(t)
	nsID := uuid.New()
	vaultForNamespace(srv, nsID, "FAKE-CCP-CONTENT-1")
	vaultForNamespace(srv, uuid.New(), "FAKE-CCP-CONTENT-2")
	backend := newStubCyberArkCCPBackend(t, srv, nil)

	got, err := backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, nsID, got.NamespaceID)
	assert.Equal(t, []byte("FAKE-CCP-CONTENT-1"), got.EncryptedBlob)
	assert.Equal(t, domainsecrets.BackendTypeCyberArkCCP, backend.Type())
	assert.Equal(t, "aim-secrets-"+nsID.String(), secrets.CyberArkCCPObjectName(nsID))

	// The wire, as the stub saw it: one GET naming the application, the safe
	// and the namespace's account, over a connection that presented the
	// client certificate.
	requests := srv.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, http.MethodGet, requests[0].Method)
	assert.Equal(t, ccptest.ClientCommonName, requests[0].ClientCommonName)
	assert.Equal(t, map[string][]string{
		"AppID":  {ccpTestAppID},
		"Safe":   {ccpTestSafe},
		"Object": {"aim-secrets-" + nsID.String()},
	}, map[string][]string(requests[0].Params))
}

func TestCyberArkCCPBackendReportsANamespaceWithNoAccountAsNotFound(t *testing.T) {
	srv := ccptest.New(t)
	vaultForNamespace(srv, uuid.New(), "FAKE-CCP-CONTENT-1")
	backend := newStubCyberArkCCPBackend(t, srv, nil)

	nsID := uuid.New()
	_, err := backend.Retrieve(nsID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
	assert.EqualError(t, err, "no credential found in cyberark for namespace "+nsID.String())
}

func TestCyberArkCCPBackendReportsOtherFailuresAsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mutate       func(*secrets.CyberArkCCPConfig)
		setup        func(*ccptest.Server)
		reachesStub  bool
		wantContains string
	}{
		{
			name:         "CCP refuses the application",
			mutate:       func(c *secrets.CyberArkCCPConfig) { c.AppID = "Another-App" },
			reachesStub:  true,
			wantContains: "status 403",
		},
		{
			name: "CCP cannot serve the request",
			setup: func(s *ccptest.Server) {
				s.AnswerEvery(http.StatusInternalServerError, "APPAP282E The password is being changed by the CPM.")
			},
			reachesStub:  true,
			wantContains: "status 500",
		},
		{
			name:         "the server certificate is not one the CA bundle verifies",
			mutate:       func(c *secrets.CyberArkCCPConfig) { c.CACertPath = ccptest.UnrelatedCABundle(t) },
			reachesStub:  false,
			wantContains: "CCP request failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := ccptest.New(t)
			nsID := uuid.New()
			vaultForNamespace(srv, nsID, "FAKE-CCP-CONTENT-1")
			if tc.setup != nil {
				tc.setup(srv)
			}
			backend := newStubCyberArkCCPBackend(t, srv, tc.mutate)

			got, err := backend.Retrieve(nsID)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.ErrorIs(t, err, domainsecrets.ErrBackendUnavailable)
			assert.Contains(t, err.Error(), tc.wantContains)
			if tc.reachesStub {
				assert.Len(t, srv.Requests(), 1)
			} else {
				assert.Empty(t, srv.Requests(), "a refused handshake reaches no handler")
			}
		})
	}
}

func TestCyberArkCCPBackendRefusesAnAccountWithNoContent(t *testing.T) {
	srv := ccptest.New(t)
	nsID := uuid.New()
	vaultForNamespace(srv, nsID, "")
	backend := newStubCyberArkCCPBackend(t, srv, nil)

	got, err := backend.Retrieve(nsID)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.EqualError(t, err, "cyberark returned an account with no content for namespace "+nsID.String())
}

// CCP only retrieves accounts: a store or a rotation is refused without a
// request, and deleting a namespace leaves its account in the safe.
func TestCyberArkCCPBackendIsReadOnly(t *testing.T) {
	srv := ccptest.New(t)
	nsID := uuid.New()
	vaultForNamespace(srv, nsID, "FAKE-CCP-CONTENT-1")
	backend := newStubCyberArkCCPBackend(t, srv, nil)

	want := "secrets backend is read-only: the credential of namespace " + nsID.String() +
		" is kept in CyberArk safe AIM-Secrets as account aim-secrets-" + nsID.String()
	err := backend.Store(nsID, []byte("FAKE-CCP-CONTENT-2"), "X25519-ChaCha20-Poly1305")
	assert.ErrorIs(t, err, domainsecrets.ErrBackendReadOnly)
	assert.EqualError(t, err, want)
	err = backend.Rotate(nsID, []byte("FAKE-CCP-CONTENT-2"), "X25519-ChaCha20-Poly1305")
	assert.ErrorIs(t, err, domainsecrets.ErrBackendReadOnly)
	assert.EqualError(t, err, want)

	require.NoError(t, backend.Delete(nsID))
	assert.Empty(t, srv.Requests(), "store, rotate and delete send nothing to CCP")

	got, err := backend.Retrieve(nsID)
	require.NoError(t, err)
	assert.Equal(t, []byte("FAKE-CCP-CONTENT-1"), got.EncryptedBlob, "the account is read as it was vaulted")
}

func TestNewCyberArkCCPBackendChecksItsSettings(t *testing.T) {
	srv := ccptest.New(t)
	for _, tc := range []struct {
		name   string
		mutate func(*secrets.CyberArkCCPConfig)
		want   string
	}{
		{"no application ID", func(c *secrets.CyberArkCCPConfig) { c.AppID = "" }, "cyberark: AppID is required"},
		{"no safe", func(c *secrets.CyberArkCCPConfig) { c.Safe = "" }, "cyberark: Safe is required"},
		{"a URL that is not https", func(c *secrets.CyberArkCCPConfig) { c.CCPURL = "http://ccp.example.com" },
			"cyberark: CCPURL must be an https URL with a host"},
		{"a client certificate without its key", func(c *secrets.CyberArkCCPConfig) { c.KeyPath = "" },
			"cyberark: CertPath and KeyPath must both be set or both be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &secrets.CyberArkCCPConfig{
				CCPURL: srv.URL, AppID: ccpTestAppID, Safe: ccpTestSafe,
				CertPath: srv.ClientCertPath, KeyPath: srv.ClientKeyPath, CACertPath: srv.CACertPath,
			}
			tc.mutate(cfg)
			backend, err := secrets.NewCyberArkCCPBackend(cfg)
			assert.Nil(t, backend)
			assert.EqualError(t, err, tc.want)
		})
	}
	assert.Empty(t, srv.Requests(), "building the backend sends nothing to CCP")
}

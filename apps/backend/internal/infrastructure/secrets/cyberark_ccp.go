package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/cyberark"
)

// cyberArkCCPObjectPrefix starts the name of the CyberArk account that holds a
// namespace's credential.
const cyberArkCCPObjectPrefix = "aim-secrets-"

// CyberArkCCPObjectName returns the name of the CyberArk account that holds the
// credential of a namespace: aim-secrets-<namespace id>. It holds only
// lowercase letters, digits and hyphens, none of the characters the CCP client
// refuses in a parameter value.
func CyberArkCCPObjectName(namespaceID uuid.UUID) string {
	return cyberArkCCPObjectPrefix + namespaceID.String()
}

// CyberArkCCPConfig holds the settings of the CyberArk Central Credential
// Provider backend.
type CyberArkCCPConfig struct {
	// CCPURL is the https base URL of the CCP web service.
	CCPURL string
	// AppID is the application ID AIM is registered under in CyberArk.
	AppID string
	// Safe is the safe that holds the namespaces' accounts.
	Safe string
	// CertPath and KeyPath hold the PEM client certificate AIM presents to
	// CCP and its private key; both are set or neither is.
	CertPath string
	KeyPath  string
	// CACertPath is a PEM bundle that verifies the CCP server; empty means the
	// system roots.
	CACertPath string
}

// CyberArkCCPBackend implements SecretsBackend by reading a namespace's
// credential from CyberArk through the Central Credential Provider. CCP only
// retrieves accounts, so the backend is read-only: the credential is stored and
// rotated in CyberArk. No business logic here (CR-009).
type CyberArkCCPBackend struct {
	connector *cyberark.Connector
	safe      string
}

// NewCyberArkCCPBackend checks cfg and builds the CCP client. AppID and Safe
// are required, and refused when every request naming them would be refused
// before it is sent; cyberark.NewConnector checks the URL and loads the client
// key pair and the CA bundle. An error names the setting at fault.
func NewCyberArkCCPBackend(cfg *CyberArkCCPConfig) (*CyberArkCCPBackend, error) {
	if cfg.AppID == "" {
		return nil, errors.New("cyberark: AppID is required")
	}
	if cfg.Safe == "" {
		return nil, errors.New("cyberark: Safe is required")
	}
	probe := cyberark.CCPRequest{AppID: cfg.AppID, Safe: cfg.Safe, Object: CyberArkCCPObjectName(uuid.Nil)}
	if err := probe.Validate(); err != nil {
		return nil, err
	}
	connector, err := cyberark.NewConnector(cyberark.Config{
		CCPURL:     cfg.CCPURL,
		AppID:      cfg.AppID,
		CertPath:   cfg.CertPath,
		KeyPath:    cfg.KeyPath,
		CACertPath: cfg.CACertPath,
	})
	if err != nil {
		return nil, err
	}
	return &CyberArkCCPBackend{connector: connector, safe: cfg.Safe}, nil
}

// Store refuses: the credential is stored in CyberArk.
func (b *CyberArkCCPBackend) Store(namespaceID uuid.UUID, _ []byte, _ string) error {
	return b.readOnly(namespaceID)
}

// Retrieve asks CCP for the account named CyberArkCCPObjectName(namespaceID) in
// the configured safe and returns its content as the credential. A 404 from CCP
// is a namespace with no credential; any other failure wraps
// ErrBackendUnavailable. An account with empty content is refused.
func (b *CyberArkCCPBackend) Retrieve(namespaceID uuid.UUID) (*domainsecrets.SecretCredential, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cyberark.DefaultTimeout)
	defer cancel()

	account, err := b.connector.RetrieveCredential(ctx, cyberark.CCPRequest{
		Safe:   b.safe,
		Object: CyberArkCCPObjectName(namespaceID),
	})
	if errors.Is(err, cyberark.ErrCCPNotFound) {
		return nil, fmt.Errorf("no credential found in cyberark for namespace %s", namespaceID)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: cyberark retrieve failed: %v", domainsecrets.ErrBackendUnavailable, err)
	}
	if account.Content == "" {
		return nil, fmt.Errorf("cyberark returned an account with no content for namespace %s", namespaceID)
	}
	return &domainsecrets.SecretCredential{
		NamespaceID:   namespaceID,
		EncryptedBlob: []byte(account.Content),
	}, nil
}

// Rotate refuses: the credential is rotated in CyberArk.
func (b *CyberArkCCPBackend) Rotate(namespaceID uuid.UUID, _ []byte, _ string) error {
	return b.readOnly(namespaceID)
}

// Delete sends nothing to CyberArk: AIM does not own the account, which stays
// in the safe after its namespace is deleted.
func (b *CyberArkCCPBackend) Delete(uuid.UUID) error {
	return nil
}

func (b *CyberArkCCPBackend) Type() domainsecrets.BackendType {
	return domainsecrets.BackendTypeCyberArkCCP
}

func (b *CyberArkCCPBackend) readOnly(namespaceID uuid.UUID) error {
	return fmt.Errorf("%w: the credential of namespace %s is kept in CyberArk safe %s as account %s",
		domainsecrets.ErrBackendReadOnly, namespaceID, b.safe, CyberArkCCPObjectName(namespaceID))
}

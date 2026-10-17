package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// SECURITY: SHA-256 digests of known development-only secrets, and of template
// placeholders long enough to pass the length check, that must never be
// used in any environment. Hash-only storage keeps the plaintexts out of the
// compiled binary and the source-of-record (CWE-798). To verify whether a
// suspect string is on the blocklist during a security review:
//
//	echo -n "<suspect string>" | shasum -a 256
//
// and compare against the digests below. The corresponding plaintexts live only
// in the test file (config_test.go), where they are part of the documented
// negative-test surface, not shipped code.
var insecureDevSecretDigests = []string{
	"ed6328f5592d88d85070fdf00a39c1c94caf46e9a175e30776c7e95c37e61576",
	"849f4281c67d78f9cabb75eee0297b69e21dcde5e78dc6c75638c4d21deae10b",
	"a1ad7292753ed8898e898b79c0d8b4fe5d5a620323d390371d83b640a6c2d473",
	"e8cdbad8a2a71625ade7cd908b26f6522040a56af09ba9b82387185627c2b596",
	"8413be1bcea1ec20652c60b3c01c6ba7f8a09d3e1c4c11f1d872e737d9a9a4ad",
	"b508342c88497e5bc592721e44ef7b27642741df360a10fde1d48be4b44aefa7",
	"bc43a14beca8866dbece708e622e3733d7580d1ea130403ff658a13eafc38f26",
}

// isKnownDevSecret reports whether s matches any of the hashed dev-secret
// digests. Input is whitespace-trimmed before hashing so that a paste-with-
// trailing-newline doesn't bypass the blocklist; case is preserved on
// purpose so that a legitimate mixed-case secret which happens to share its
// lowercase form with a dev string is not rejected.
func isKnownDevSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	sum := sha256.Sum256([]byte(s))
	got := hex.EncodeToString(sum[:])
	for _, want := range insecureDevSecretDigests {
		if got == want {
			return true
		}
	}
	return false
}

// Config holds all configuration for the application. Load reads every
// environment variable the server process reads itself; the variables read by
// other packages (sign-in token lifetimes, rate limits, email delivery,
// telemetry, the key vault) are read there.
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	JWT         JWTConfig
	OAuth       OAuthConfig
	Auth        AuthConfig
	Admin       AdminSeedConfig
	Registry    RegistryConfig
	Secrets     SecretsBackendsConfig
	FGA         FGAConfig
	Email       EmailConfig
	SIEM        SIEMConfig
	SigningKeys SigningKeysConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	// Host is the address the API listener binds, read by ListenHost: HOST
	// when it is set and not empty, otherwise 0.0.0.0 in a container and
	// 127.0.0.1 outside one. See issue #757.
	Host        string
	Port        string
	Environment string
	LogLevel    string
	FrontendURL string
	// MetricsAuthToken is METRICS_AUTH_TOKEN as set. The metrics wiring in
	// cmd/server treats a value shorter than 32 characters as unset. With a
	// token, GET /metrics answers on the API listener and on the dedicated
	// metrics listener behind an `Authorization: Bearer <token>` check.
	// Without one, the API listener has no /metrics route and the dedicated
	// listener (METRICS_LISTEN_ADDR) serves it only on a loopback address.
	// See issue #348.
	MetricsAuthToken string
	// MetricsListenAddr is the address of the dedicated metrics listener
	// (METRICS_LISTEN_ADDR), DefaultMetricsListenAddr when it is unset.
	MetricsListenAddr string
	// AllowedOrigins are the CORS origins (ALLOWED_ORIGINS, comma-separated,
	// split as written). The CORS middleware validates them.
	AllowedOrigins []string
}

// DefaultMetricsListenAddr is loopback on the port the OpenTelemetry
// Prometheus exporter also defaults to; 9090 is Prometheus itself.
const DefaultMetricsListenAddr = "127.0.0.1:9464"

// defaultAllowedOrigin is the dashboard's development origin, the only CORS
// origin when ALLOWED_ORIGINS is unset.
const defaultAllowedOrigin = "http://localhost:3000"

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConnections  int
	ConnMaxLifetime time.Duration
	// PoolRole is the role the request pool sets on every connection it opens
	// (POSTGRES_POOL_ROLE). Empty, the default, sets none.
	PoolRole string
	// ScopeCensus turns on the row security census of the request pool
	// (POSTGRES_SCOPE_CENSUS=on). Off by default.
	ScopeCensus bool
	// AppUser is the login role `aim-server migrate` provisions for the
	// server (POSTGRES_APP_USER); empty provisions none. AppPassword is its
	// password (POSTGRES_APP_PASSWORD).
	AppUser     string
	AppPassword string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	UseTLS   bool
	// Configured is true when REDIS_HOST names a Redis. The localhost default
	// is still tried when it is unset, but does not make Redis configured.
	Configured bool
	// Disabled is true when REDIS_HOST is set to the empty string: the server
	// makes no Redis connection, not even to the localhost default.
	Disabled bool
}

// JWTConfig holds JWT configuration. Token lifetimes (JWT_ACCESS_TTL,
// JWT_REFRESH_TTL, JWT_SESSION_MAX_AGE) are read by auth.NewJWTService.
type JWTConfig struct {
	Secret string
}

// OAuthConfig holds OAuth provider configurations
type OAuthConfig struct {
	Google    OAuthProvider
	Microsoft OAuthProvider
	Okta      OktaProvider
}

// OAuthProvider holds OAuth provider configuration
type OAuthProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OktaProvider holds Okta-specific configuration
type OktaProvider struct {
	ClientID     string
	ClientSecret string
	Domain       string
	RedirectURL  string
}

// AuthConfig holds the token revocation setting.
type AuthConfig struct {
	// RevocationFailOpen lets a request through when the revocation store
	// cannot be reached (AUTH_REVOCATION_FAIL_OPEN=true, in any case). Off by
	// default: a store outage refuses the request.
	RevocationFailOpen bool
}

// AdminSeedConfig holds the first administrator the server seeds at startup.
type AdminSeedConfig struct {
	// Password is ADMIN_PASSWORD; empty seeds no administrator.
	Password string
	// Email is ADMIN_EMAIL, admin@opena2a.org when it is unset.
	Email string
	// Name is ADMIN_NAME, "System Administrator" when it is unset.
	Name string
}

// RegistryConfig holds the OpenA2A Registry settings.
type RegistryConfig struct {
	// BridgeURL is REGISTRY_BRIDGE_URL, https://api.oa2a.org when it is unset.
	BridgeURL string
	// BridgeEnabled turns on the attestation push (REGISTRY_BRIDGE_ENABLED,
	// exactly "true").
	BridgeEnabled bool
	// ATCIssuerURI is ATC_ISSUER_URI, https://aim.opena2a.org when it is unset.
	ATCIssuerURI string
	// ATCCRLEndpoint is ATC_CRL_ENDPOINT, the Registry's latest CRL when it is
	// unset.
	ATCCRLEndpoint string
}

// SecretsBackendsConfig holds the external secrets backends. Each one is
// turned on by its first variable: VAULT_ADDR, AWS_SECRETS_REGION,
// AZURE_KEYVAULT_URL, GCP_SECRETS_PROJECT and OP_CONNECT_HOST.
type SecretsBackendsConfig struct {
	Vault         VaultConfig
	AWS           AWSSecretsConfig
	AzureVaultURL string
	GCPProject    string
	OnePassword   OnePasswordConfig
}

// VaultConfig holds the HashiCorp Vault backend settings (VAULT_*).
type VaultConfig struct {
	Addr      string
	MountPath string
	AuthPath  string
	Role      string
	Token     string
	Namespace string
}

// AWSSecretsConfig holds the AWS Secrets Manager backend settings.
type AWSSecretsConfig struct {
	Region   string
	KMSKeyID string
	Prefix   string
}

// OnePasswordConfig holds the 1Password Connect backend settings.
type OnePasswordConfig struct {
	ConnectHost  string
	ConnectToken string
	VaultUUID    string
}

// FGAConfig holds the authorization engine's endpoints; empty keeps the
// engine's own.
type FGAConfig struct {
	DaemonURL      string
	RegistryASCURL string
}

// EmailConfig holds the email settings the startup line names. The email
// package reads the same two variables, with the same rule, to choose the
// provider and the sender.
type EmailConfig struct {
	// Provider is EMAIL_PROVIDER, azure when it is unset.
	Provider    string
	FromAddress string
}

// SIEMConfig holds the SIEM_* variables as set. siem.SettingsFromEnv reads
// them through Value and validates them.
type SIEMConfig struct {
	SplunkHECURL   string
	SplunkHECToken string
	SplunkIndex    string
	MinSeverity    string
}

// Value returns the named SIEM_* variable as Load read it, and "" for a name
// SIEMConfig does not hold.
func (s SIEMConfig) Value(name string) string {
	switch name {
	case "SIEM_SPLUNK_HEC_URL":
		return s.SplunkHECURL
	case "SIEM_SPLUNK_HEC_TOKEN":
		return s.SplunkHECToken
	case "SIEM_SPLUNK_INDEX":
		return s.SplunkIndex
	case "SIEM_MIN_SEVERITY":
		return s.MinSeverity
	}
	return ""
}

// SigningKeysConfig holds the provisioned server signing keys, one
// AIM_SIGNING_KEY_<PURPOSE> and one AIM_SIGNING_KEY_<PURPOSE>_RETIRED per
// purpose, as set. crypto.LoadSigningKeyRing reads them through Value and
// validates them.
type SigningKeysConfig struct {
	CardAttestation        string
	CardAttestationRetired string
	ATCIssuer              string
	ATCIssuerRetired       string
}

// Value returns the named AIM_SIGNING_KEY_* variable as Load read it, and ""
// for a name SigningKeysConfig does not hold.
func (k SigningKeysConfig) Value(name string) string {
	switch name {
	case "AIM_SIGNING_KEY_CARD_ATTESTATION":
		return k.CardAttestation
	case "AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED":
		return k.CardAttestationRetired
	case "AIM_SIGNING_KEY_ATC_ISSUER":
		return k.ATCIssuer
	case "AIM_SIGNING_KEY_ATC_ISSUER_RETIRED":
		return k.ATCIssuerRetired
	}
	return ""
}

// Load loads configuration from environment variables.
//
// One rule holds for every variable: an empty value is read as unset, so it
// gives the default, and a value is used as written, never trimmed, so a
// value of spaces is a value. REDIS_HOST is the one exception: set and empty,
// it turns Redis off. A number or a duration that does not parse gives the
// default. A flag is on only for its word: "true" for REDIS_USE_TLS and
// REGISTRY_BRIDGE_ENABLED, "true" in any case for AUTH_REVOCATION_FAIL_OPEN,
// and "on" for POSTGRES_SCOPE_CENSUS. JWT_SECRET, POSTGRES_HOST, POSTGRES_USER
// and POSTGRES_DB are required.
func Load() (*Config, error) {
	database, err := LoadDatabase()
	if err != nil {
		return nil, err
	}
	config := &Config{
		Server:   LoadServer(),
		Database: database,
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			UseTLS:   getEnvFlag("REDIS_USE_TLS", "true"),

			Configured: getenv("REDIS_HOST") != "",
			Disabled:   isSetAndEmpty("REDIS_HOST"),
		},
		JWT: JWTConfig{
			Secret: getEnvRequired("JWT_SECRET"),
		},
		OAuth: OAuthConfig{
			Google: OAuthProvider{
				ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/google"),
			},
			Microsoft: OAuthProvider{
				ClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
				ClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("MICROSOFT_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/microsoft"),
			},
			Okta: OktaProvider{
				ClientID:     getEnv("OKTA_CLIENT_ID", ""),
				ClientSecret: getEnv("OKTA_CLIENT_SECRET", ""),
				Domain:       getEnv("OKTA_DOMAIN", ""),
				RedirectURL:  getEnv("OKTA_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback/okta"),
			},
		},
		Auth: AuthConfig{
			RevocationFailOpen: strings.EqualFold(getenv("AUTH_REVOCATION_FAIL_OPEN"), "true"),
		},
		Admin: LoadAdminSeed(),
		Registry: RegistryConfig{
			BridgeURL:      getEnv("REGISTRY_BRIDGE_URL", "https://api.oa2a.org"),
			BridgeEnabled:  getEnvFlag("REGISTRY_BRIDGE_ENABLED", "true"),
			ATCIssuerURI:   getEnv("ATC_ISSUER_URI", "https://aim.opena2a.org"),
			ATCCRLEndpoint: getEnv("ATC_CRL_ENDPOINT", "https://registry.opena2a.org/api/v1/crl/latest"),
		},
		Secrets: SecretsBackendsConfig{
			Vault: VaultConfig{
				Addr:      getEnv("VAULT_ADDR", ""),
				MountPath: getEnv("VAULT_MOUNT_PATH", "secret"),
				AuthPath:  getEnv("VAULT_AUTH_PATH", "jwt"),
				Role:      getEnv("VAULT_ROLE", ""),
				Token:     getEnv("VAULT_TOKEN", ""),
				Namespace: getEnv("VAULT_NAMESPACE", ""),
			},
			AWS: AWSSecretsConfig{
				Region:   getEnv("AWS_SECRETS_REGION", ""),
				KMSKeyID: getEnv("AWS_SECRETS_KMS_KEY_ID", ""),
				Prefix:   getEnv("AWS_SECRETS_PREFIX", "aim-secrets/"),
			},
			AzureVaultURL: getEnv("AZURE_KEYVAULT_URL", ""),
			GCPProject:    getEnv("GCP_SECRETS_PROJECT", ""),
			OnePassword: OnePasswordConfig{
				ConnectHost:  getEnv("OP_CONNECT_HOST", ""),
				ConnectToken: getEnv("OP_CONNECT_TOKEN", ""),
				VaultUUID:    getEnv("OP_VAULT_UUID", ""),
			},
		},
		FGA: FGAConfig{
			DaemonURL:      getEnv("FGA_DAEMON_URL", ""),
			RegistryASCURL: getEnv("FGA_REGISTRY_ASC_URL", ""),
		},
		Email: EmailConfig{
			Provider:    getEnv("EMAIL_PROVIDER", "azure"),
			FromAddress: getEnv("EMAIL_FROM_ADDRESS", ""),
		},
		SIEM: SIEMConfig{
			SplunkHECURL:   getEnv("SIEM_SPLUNK_HEC_URL", ""),
			SplunkHECToken: getEnv("SIEM_SPLUNK_HEC_TOKEN", ""),
			SplunkIndex:    getEnv("SIEM_SPLUNK_INDEX", ""),
			MinSeverity:    getEnv("SIEM_MIN_SEVERITY", ""),
		},
		SigningKeys: SigningKeysConfig{
			CardAttestation:        getEnv("AIM_SIGNING_KEY_CARD_ATTESTATION", ""),
			CardAttestationRetired: getEnv("AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED", ""),
			ATCIssuer:              getEnv("AIM_SIGNING_KEY_ATC_ISSUER", ""),
			ATCIssuerRetired:       getEnv("AIM_SIGNING_KEY_ATC_ISSUER_RETIRED", ""),
		},
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Print security warnings (non-blocking)
	config.printSecurityWarnings()

	return config, nil
}

// LoadServer reads the server settings, the same variables Load reads for
// Config.Server.
func LoadServer() ServerConfig {
	allowedOrigins := []string{defaultAllowedOrigin}
	if origins := getenv("ALLOWED_ORIGINS"); origins != "" {
		allowedOrigins = strings.Split(origins, ",")
	}
	return ServerConfig{
		Host:              ListenHost(RunningInContainer),
		Port:              getEnv("APP_PORT", "8080"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:3000"),
		MetricsAuthToken:  getEnv("METRICS_AUTH_TOKEN", ""),
		MetricsListenAddr: getEnv("METRICS_LISTEN_ADDR", DefaultMetricsListenAddr),
		AllowedOrigins:    allowedOrigins,
	}
}

// LoadAdminSeed reads the ADMIN_* variables, the same ones Load reads for
// Config.Admin.
func LoadAdminSeed() AdminSeedConfig {
	return AdminSeedConfig{
		Password: getEnv("ADMIN_PASSWORD", ""),
		Email:    getEnv("ADMIN_EMAIL", "admin@opena2a.org"),
		Name:     getEnv("ADMIN_NAME", "System Administrator"),
	}
}

// LoadDatabase reads the database connection settings, the same POSTGRES_*
// variables Load reads. `aim-server migrate` reads only these, so the
// migration step needs no other secret in its environment. A missing
// POSTGRES_HOST, POSTGRES_USER or POSTGRES_DB is an error that names every
// missing variable.
func LoadDatabase() (DatabaseConfig, error) {
	var missing []string
	required := func(key string) string {
		value := getenv(key)
		if value == "" {
			missing = append(missing, key)
		}
		return value
	}
	database := DatabaseConfig{
		Host:            required("POSTGRES_HOST"),
		Port:            getEnvAsInt("POSTGRES_PORT", 5432),
		User:            required("POSTGRES_USER"),
		Password:        getEnv("POSTGRES_PASSWORD", ""), // Optional for local dev with no password
		Database:        required("POSTGRES_DB"),
		SSLMode:         getEnv("POSTGRES_SSL_MODE", "disable"),
		MaxConnections:  getEnvAsInt("POSTGRES_MAX_CONNECTIONS", 25),
		ConnMaxLifetime: getEnvAsDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
		PoolRole:        getEnv("POSTGRES_POOL_ROLE", ""),
		ScopeCensus:     getEnvFlag("POSTGRES_SCOPE_CENSUS", "on"),
		AppUser:         getEnv("POSTGRES_APP_USER", ""),
		AppPassword:     getEnv("POSTGRES_APP_PASSWORD", ""),
	}
	if len(missing) > 0 {
		return DatabaseConfig{}, fmt.Errorf("required environment variable(s) not set: %s", strings.Join(missing, ", "))
	}
	return database, nil
}

// URL returns the settings as a postgres:// connection URL. The user and the
// password are percent-encoded in it, and lib/pq decodes them, so a single
// quote, a backslash or any other character in either reaches PostgreSQL as
// it is configured.
//
// A host that begins with a slash is the directory of a Unix socket, as it is
// in the server's connection string. A URL's host cannot hold a slash, so that
// directory and the port go in the host and port query parameters, which
// lib/pq reads as the same two settings.
func (d DatabaseConfig) URL() string {
	u := url.URL{
		Scheme: "postgres",
		Path:   "/" + d.Database,
	}
	query := url.Values{"sslmode": {d.SSLMode}}
	if strings.HasPrefix(d.Host, "/") {
		query.Set("host", d.Host)
		query.Set("port", strconv.Itoa(d.Port))
	} else {
		u.Host = net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
	}
	u.RawQuery = query.Encode()
	if d.Password != "" {
		u.User = url.UserPassword(d.User, d.Password)
	} else {
		u.User = url.User(d.User)
	}
	return u.String()
}

// printSecurityWarnings logs warnings for potentially insecure configurations
// These don't block startup but alert operators to review their settings
func (c *Config) printSecurityWarnings() {
	if c.Server.Environment != "production" {
		return
	}

	// Warn about SSL mode for remote databases
	isLocalDB := strings.Contains(c.Database.Host, "localhost") ||
		strings.Contains(c.Database.Host, "127.0.0.1") ||
		c.Database.Host == "postgres" // Docker service name

	if !isLocalDB && c.Database.SSLMode == "disable" {
		fmt.Print(`
⚠️  SECURITY WARNING: Database SSL is disabled for remote connection
    POSTGRES_SSL_MODE=disable with remote host may expose data in transit.
    Consider setting POSTGRES_SSL_MODE=require for production.
    (This is a warning, not an error - some setups use VPC/network encryption)

`)
	}
}

// Validate validates the configuration.
//
// SECURITY: known dev-secret values are rejected in every environment, not only
// production. A laptop running ENVIRONMENT=development with a leaked dev secret
// is still a vulnerability the moment that laptop gets pointed at shared
// infrastructure; failing fast everywhere prevents the misconfiguration from
// ever taking effect.
func (c *Config) Validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if isKnownDevSecret(c.JWT.Secret) {
		return insecureSecretError("JWT_SECRET", "openssl rand -hex 32")
	}

	if isKnownDevSecret(getenv("KEYVAULT_MASTER_KEY")) {
		return insecureSecretError("KEYVAULT_MASTER_KEY", "openssl rand -base64 32")
	}

	// Production-only: weak POSTGRES_PASSWORD against a remote host.
	if c.Server.Environment == "production" {
		if c.Database.Password == "postgres" || c.Database.Password == "" {
			if !strings.Contains(c.Database.Host, "localhost") && !strings.Contains(c.Database.Host, "127.0.0.1") {
				return fmt.Errorf("POSTGRES_PASSWORD is weak or empty for a remote host; generate one with `openssl rand -base64 24`")
			}
		}
	}

	// OAuth providers are now optional since we support email/password authentication
	// Validation removed - OAuth configuration is checked at runtime when needed

	return nil
}

// insecureSecretError returns the single error format used when a known
// development secret leaks into a real config. The format is plain text so it
// reads cleanly in container logs and CI output. The "every environment"
// clause is intentional — operators conditioned by other tools may try
// toggling ENVIRONMENT=development to bypass; this check now runs in every
// environment, so that workaround no longer exists.
func insecureSecretError(name, generator string) error {
	return fmt.Errorf(
		"SECURITY ERROR: %s is set to a known development default; "+
			"this is rejected in every environment (development, staging, production). "+
			"Replace it with a freshly generated value: %s. "+
			"See scripts/gen-dev-secrets.sh for the local-dev helper.",
		name, generator,
	)
}

// containerMarkerFiles are files a container runtime writes into the root of
// a container's file system: /.dockerenv (Docker) and /run/.containerenv
// (Podman).
var containerMarkerFiles = []string{"/.dockerenv", "/run/.containerenv"}

// RunningInContainer reports whether KUBERNETES_SERVICE_HOST (set by
// Kubernetes in a pod's containers) is non-empty or one of
// containerMarkerFiles exists.
func RunningInContainer() bool {
	if getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	for _, path := range containerMarkerFiles {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// ListenHost returns the address the API listener binds: HOST when it is set
// and not empty. Otherwise it returns 0.0.0.0 when inContainer reports true,
// so a published container port reaches the listener, and 127.0.0.1 when it
// reports false, so a run on a workstation accepts no connection from its
// network.
func ListenHost(inContainer func() bool) string {
	if host := getenv("HOST"); host != "" {
		return host
	}
	if inContainer() {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

// lookupEnv is the one place this package reads the environment; every
// helper below reads through it.
var lookupEnv = os.LookupEnv

// getenv returns the value of key, "" when it is unset.
func getenv(key string) string {
	value, _ := lookupEnv(key)
	return value
}

// getEnv returns the value of key, or defaultValue when it is unset or empty.
func getEnv(key, defaultValue string) string {
	value := getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvFlag reports whether key is set to word, exactly.
func getEnvFlag(key, word string) bool {
	return getenv(key) == word
}

// isSetAndEmpty reports whether key is in the environment with an empty value,
// which getEnv cannot tell apart from an unset key.
func isSetAndEmpty(key string) bool {
	value, set := lookupEnv(key)
	return set && value == ""
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvRequired gets environment variable and panics if not set
func getEnvRequired(key string) string {
	value := getenv(key)
	if value == "" {
		panic(fmt.Sprintf("Required environment variable %s is not set", key))
	}
	return value
}

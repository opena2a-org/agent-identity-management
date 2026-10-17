//go:build integration

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/telemetry"
)

// The OTLP exporter variables a run could carry from its environment.
var otlpExporterVariables = []string{
	"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_INSECURE",
	"OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_CLIENT_KEY", "OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_INSECURE", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE",
	"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_INSECURE", "OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE",
	"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_INSECURE", "OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE",
}

// A failed sign-in and an FGA decision, sent through the server's route
// table with the server's telemetry configuration, reach an OTLP/gRPC
// receiver built from the published protos: a SERVER span for each request
// with the HTTP semantic-convention attributes, and the fga.authorize span as
// a child of its request's span. An https endpoint reaches a receiver that
// serves only TLS, verified with OTEL_EXPORTER_OTLP_CERTIFICATE, with both
// the trace and the log exports; an http endpoint reaches a plaintext
// receiver although OTEL_EXPORTER_OTLP_INSECURE says false, because the
// scheme wins.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestOTelHTTPSpansRoundTrip ./cmd/server/
func TestOTelHTTPSpansRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping OpenTelemetry HTTP span integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "otel-integration-secret-of-at-least-32-chars")
	for _, name := range otlpExporterVariables {
		t.Setenv(name, "")
		require.NoError(t, os.Unsetenv(name))
	}

	t.Run("an https endpoint reaches a TLS receiver", func(t *testing.T) {
		caFile, serverTLS := loopbackCertificates(t)
		receiver := startOTLPReceiver(t, serverTLS)
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://"+receiver.addr)
		t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caFile)
		otelHTTPSpansRoundTrip(t, db, receiver, true)
	})
	t.Run("an http endpoint is plaintext whatever OTEL_EXPORTER_OTLP_INSECURE says", func(t *testing.T) {
		receiver := startOTLPReceiver(t, nil)
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+receiver.addr)
		t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")
		otelHTTPSpansRoundTrip(t, db, receiver, false)
	})
	t.Run("a loopback host:port with no INSECURE setting is plaintext", func(t *testing.T) {
		receiver := startOTLPReceiver(t, nil)
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.addr)
		otelHTTPSpansRoundTrip(t, db, receiver, false)
	})
}

func otelHTTPSpansRoundTrip(t *testing.T, db *sql.DB, receiver *otlpReceiver, wantTLS bool) {
	ctx := context.Background()
	org, user, agent := uuid.New(), uuid.New(), uuid.New()
	name := "otel-spans-" + uuid.NewString()[:8]
	email := name + "@example.invalid"
	password := "Pw-" + uuid.NewString()
	wrongPassword := "Wrong-" + uuid.NewString()
	queryMarker := "q" + strings.ReplaceAll(uuid.NewString(), "-", "")
	headerMarker := "h" + strings.ReplaceAll(uuid.NewString(), "-", "")
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`, org, name, name+".invalid")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, organization_id, email, name, provider, provider_id, role, password_hash, status)
		VALUES ($1, $2, $3, 'otel test user', 'local', $4, 'manager', $5, 'active')`,
		user, org, email, user.String(), hash)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id, status, trust_score)
		VALUES ($1, $2, $2, 'service', $3, $4, 'verified', 0.8)`, agent, name+"-agent", user, org)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_failures WHERE email = $1`, email)
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, org)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, org)
	})

	tracerBefore, meterBefore, loggerBefore := otel.GetTracerProvider(), otel.GetMeterProvider(), global.GetLoggerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(tracerBefore)
		otel.SetMeterProvider(meterBefore)
		global.SetLoggerProvider(loggerBefore)
	})
	shutdown, err := telemetry.Init(ctx, telemetryConfig())
	require.NoError(t, err)
	flushed := false
	flush := func() {
		if flushed {
			return
		}
		flushed = true
		flushCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		assert.NoError(t, shutdown(flushCtx), "flushing the exporters")
	}
	t.Cleanup(flush)

	repos, oauthRepo := initRepositories(db)
	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	authService := application.NewAuthService(repos.User, repos.Organization, repos.APIKey, repos.AuthFailure, nil, nil)
	agentService := application.NewAgentService(repos.Agent, nil, nil, nil, repos.Alert, nil, repos.Capability, nil, nil, repos.User, repos.Organization, nil)
	// Built after Init, so its tracer is the installed provider's, with the
	// logger main gives it, so its decision log reaches the receiver too.
	fga := application.NewFGAEngine(db, agentService, telemetry.Logger("aim-fga"))
	t.Cleanup(func() { _ = fga.Shutdown(ctx) })
	services := &Services{
		Auth:    authService,
		Audit:   auditService,
		Agent:   agentService,
		FGA:     fga,
		Secrets: &application.SecretsService{},
	}
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, jwtService, repos.Organization, auditService, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(oauthRepo, repos.User, repos.Organization, auditService, nil),
			authService, jwtService, auditService),
		Authorize: handlers.NewAuthorizeHandler(fga, repos.Agent),
	}

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	app.Use(trace.Middleware())
	app.Use(middleware.HTTPServerSpans())
	mountAPIRoutes(app, h, services, jwtService, nil, db)

	const userAgent = "otel-round-trip-test/1.0"
	do := func(target, bearer string, body interface{}) (int, string) {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(fiber.MethodPost, target, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("X-Otel-Probe", headerMarker)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		// A password check takes longer than app.Test's default second under -race.
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer resp.Body.Close()
		answer, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(answer)
	}

	status, body := do("/api/v1/public/login?probe="+queryMarker, "", map[string]string{"email": email, "password": wrongPassword})
	require.Equal(t, fiber.StatusUnauthorized, status, body)

	manager, err := jwtService.GenerateAccessToken(user.String(), org.String(), email, string(domain.RoleManager))
	require.NoError(t, err)
	status, body = do(fmt.Sprintf("/api/v1/agents/%s/authorize?probe=%s", agent, queryMarker), manager, map[string]string{"capability": "files:read"})
	require.Equal(t, fiber.StatusOK, status, body)

	flush()

	exports := receiver.exports()
	require.NotEmpty(t, exports, "the receiver got no trace export")
	var spans []*tracepb.Span
	for _, e := range exports {
		assert.Equal(t, wantTLS, e.tls, "a trace export arrived over TLS = %v; want %v", e.tls, wantTLS)
		spans = append(spans, e.spans...)
	}
	logExports := receiver.logs.exports()
	require.NotEmpty(t, logExports, "the receiver got no log export")
	for _, overTLS := range logExports {
		assert.Equal(t, wantTLS, overTLS, "a log export arrived over TLS = %v; want %v", overTLS, wantTLS)
	}

	signIn := findSpan(spans, "POST /api/v1/public/login")
	require.NotNil(t, signIn, "no span named POST /api/v1/public/login among %v", spanNames(spans))
	assert.Equal(t, tracepb.Span_SPAN_KIND_SERVER, signIn.GetKind())
	signInAttrs := spanAttributes(signIn)
	assert.Equal(t, "POST", signInAttrs["http.request.method"])
	assert.Equal(t, "/api/v1/public/login", signInAttrs["http.route"])
	assert.Equal(t, "/api/v1/public/login", signInAttrs["url.path"])
	assert.Equal(t, "401", signInAttrs["http.response.status_code"])
	assert.Equal(t, "http", signInAttrs["url.scheme"])
	assert.Equal(t, userAgent, signInAttrs["user_agent.original"])
	assert.Equal(t, "1.1", signInAttrs["network.protocol.version"])
	for _, key := range []string{"server.address", "server.port", "client.address"} {
		assert.NotEmpty(t, signInAttrs[key], "the sign-in span carries %s", key)
	}
	assert.Equal(t, tracepb.Status_STATUS_CODE_UNSET, signIn.GetStatus().GetCode(), "a 4xx answer leaves a server span's status unset")

	authorizeRequest := findSpan(spans, "POST /api/v1/agents/:id/authorize")
	require.NotNil(t, authorizeRequest, "no span named POST /api/v1/agents/:id/authorize among %v", spanNames(spans))
	assert.Equal(t, tracepb.Span_SPAN_KIND_SERVER, authorizeRequest.GetKind())
	assert.Equal(t, "200", spanAttributes(authorizeRequest)["http.response.status_code"])
	decision := findSpan(spans, "fga.authorize")
	require.NotNil(t, decision, "no fga.authorize span among %v", spanNames(spans))
	assert.Equal(t, hex.EncodeToString(authorizeRequest.GetTraceId()), hex.EncodeToString(decision.GetTraceId()),
		"fga.authorize is in its request's trace")
	assert.Equal(t, hex.EncodeToString(authorizeRequest.GetSpanId()), hex.EncodeToString(decision.GetParentSpanId()),
		"fga.authorize is a child of its request's server span")

	for _, s := range spans {
		for key, value := range spanAttributes(s) {
			for _, secret := range []string{queryMarker, headerMarker, manager, password, wrongPassword, email} {
				assert.NotContains(t, key+"="+value, secret, "span %q attribute %s records a query string, header or body", s.GetName(), key)
			}
		}
	}
}

func findSpan(spans []*tracepb.Span, name string) *tracepb.Span {
	for _, s := range spans {
		if s.GetName() == name {
			return s
		}
	}
	return nil
}

func spanNames(spans []*tracepb.Span) []string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.GetName())
	}
	return names
}

// spanAttributes gives a span's attributes as text, an integer in decimal.
func spanAttributes(s *tracepb.Span) map[string]string {
	attrs := map[string]string{}
	for _, kv := range s.GetAttributes() {
		v := kv.GetValue()
		switch {
		case v.GetStringValue() != "":
			attrs[kv.GetKey()] = v.GetStringValue()
		case v.GetIntValue() != 0:
			attrs[kv.GetKey()] = fmt.Sprint(v.GetIntValue())
		default:
			attrs[kv.GetKey()] = v.String()
		}
	}
	return attrs
}

// otlpExport is one trace export request a receiver accepted.
type otlpExport struct {
	tls   bool
	spans []*tracepb.Span
}

// otlpReceiver is an OTLP/gRPC collector on a loopback port. It records each
// trace export request and whether it arrived over TLS, records whether each
// log export arrived over TLS, and accepts metric exports without recording
// them.
type otlpReceiver struct {
	collectortracepb.UnimplementedTraceServiceServer
	addr string
	logs *otlpLogsSink

	mu       sync.Mutex
	received []otlpExport
}

// overTLS reports whether the request of ctx arrived over TLS.
func overTLS(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}
	_, isTLS := p.AuthInfo.(credentials.TLSInfo)
	return isTLS
}

func (r *otlpReceiver) Export(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	e := otlpExport{tls: overTLS(ctx)}
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			e.spans = append(e.spans, ss.GetSpans()...)
		}
	}
	r.mu.Lock()
	r.received = append(r.received, e)
	r.mu.Unlock()
	return &collectortracepb.ExportTraceServiceResponse{}, nil
}

func (r *otlpReceiver) exports() []otlpExport {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]otlpExport(nil), r.received...)
}

type otlpMetricsSink struct {
	collectormetricspb.UnimplementedMetricsServiceServer
}

func (otlpMetricsSink) Export(context.Context, *collectormetricspb.ExportMetricsServiceRequest) (*collectormetricspb.ExportMetricsServiceResponse, error) {
	return &collectormetricspb.ExportMetricsServiceResponse{}, nil
}

type otlpLogsSink struct {
	collectorlogspb.UnimplementedLogsServiceServer

	mu      sync.Mutex
	overTLS []bool
}

func (s *otlpLogsSink) Export(ctx context.Context, _ *collectorlogspb.ExportLogsServiceRequest) (*collectorlogspb.ExportLogsServiceResponse, error) {
	s.mu.Lock()
	s.overTLS = append(s.overTLS, overTLS(ctx))
	s.mu.Unlock()
	return &collectorlogspb.ExportLogsServiceResponse{}, nil
}

func (s *otlpLogsSink) exports() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]bool(nil), s.overTLS...)
}

// startOTLPReceiver serves OTLP/gRPC on 127.0.0.1: TLS only when serverTLS
// is given, plaintext only otherwise.
func startOTLPReceiver(t *testing.T, serverTLS *tls.Config) *otlpReceiver {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var opts []grpc.ServerOption
	if serverTLS != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(serverTLS)))
	}
	srv := grpc.NewServer(opts...)
	r := &otlpReceiver{addr: lis.Addr().String(), logs: &otlpLogsSink{}}
	collectortracepb.RegisterTraceServiceServer(srv, r)
	collectormetricspb.RegisterMetricsServiceServer(srv, otlpMetricsSink{})
	collectorlogspb.RegisterLogsServiceServer(srv, r.logs)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return r
}

// loopbackCertificates generates a CA and a server certificate it signs for
// the IP address 127.0.0.1. It writes the CA certificate to a file and
// returns its path with the server's TLS configuration.
func loopbackCertificates(t *testing.T) (caFile string, serverTLS *tls.Config) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "otlp receiver test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	require.NoError(t, err)

	caFile = filepath.Join(t.TempDir(), "otlp-receiver-ca.crt")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	return caFile, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{{Certificate: [][]byte{serverDER}, PrivateKey: serverKey}},
	}
}

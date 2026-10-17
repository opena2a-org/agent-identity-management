package telemetry

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
)

// env is a fixed environment for resolveTransport.
type env map[string]string

func (e env) lookup(name string) (string, bool) {
	v, ok := e[name]
	return v, ok
}

// Each row of the endpoint and TLS rules, for the traces signal.
func TestResolveTransport_EndpointAndTLS(t *testing.T) {
	cases := []struct {
		name          string
		env           env
		configured    string
		insecure      bool
		wantEndpoint  string
		wantPlaintext bool
	}{
		{name: "unset is localhost:4317 in plaintext", env: env{},
			wantEndpoint: "localhost:4317", wantPlaintext: true},
		{name: "https is TLS whatever INSECURE says",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "true"},
			wantEndpoint: "collector.example:4317"},
		{name: "https to a loopback host is TLS",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://127.0.0.1:4317"},
			wantEndpoint: "127.0.0.1:4317"},
		{name: "http is plaintext whatever INSECURE says",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "false"},
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "the scheme is read without regard to case",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "HTTP://collector.example:4317"},
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "no scheme with INSECURE=true is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "true"},
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "no scheme with INSECURE=TRUE is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "TRUE"},
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "no scheme to a non-loopback host name is TLS",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317"},
			wantEndpoint: "collector.example:4317"},
		{name: "no scheme to a non-loopback address is TLS",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "10.0.0.5:4317"},
			wantEndpoint: "10.0.0.5:4317"},
		{name: "no scheme to localhost is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "localhost:4317"},
			wantEndpoint: "localhost:4317", wantPlaintext: true},
		{name: "no scheme to 127.0.0.1 is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "127.0.0.1:4317"},
			wantEndpoint: "127.0.0.1:4317", wantPlaintext: true},
		{name: "no scheme to another 127.0.0.0/8 address is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "127.4.5.6:4317"},
			wantEndpoint: "127.4.5.6:4317", wantPlaintext: true},
		{name: "no scheme to ::1 is plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "[::1]:4317"},
			wantEndpoint: "[::1]:4317", wantPlaintext: true},
		{name: "no scheme to a loopback host with INSECURE=false is TLS",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "127.0.0.1:4317", "OTEL_EXPORTER_OTLP_INSECURE": "false"},
			wantEndpoint: "127.0.0.1:4317"},
		{name: "the per-signal endpoint wins",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://all.example:4317", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https://traces.example:4317"},
			wantEndpoint: "traces.example:4317"},
		{name: "an empty per-signal endpoint falls back to the generic one",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://all.example:4317", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": ""},
			wantEndpoint: "all.example:4317", wantPlaintext: true},
		{name: "the per-signal INSECURE wins",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "false", "OTEL_EXPORTER_OTLP_TRACES_INSECURE": "true"},
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "Config.Endpoint wins over the variable",
			env:        env{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://all.example:4317"},
			configured: "https://configured.example:4317", wantEndpoint: "configured.example:4317"},
		{name: "Config.Insecure makes an endpoint without a scheme plaintext",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317"},
			insecure:     true,
			wantEndpoint: "collector.example:4317", wantPlaintext: true},
		{name: "an https scheme wins over Config.Insecure",
			env:          env{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://collector.example:4317"},
			insecure:     true,
			wantEndpoint: "collector.example:4317"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTransport(tc.env.lookup, signalTraces, tc.configured, tc.insecure)
			if err != nil {
				t.Fatalf("resolveTransport error = %v; want nil", err)
			}
			if got.endpoint != tc.wantEndpoint {
				t.Errorf("endpoint = %q; want %q", got.endpoint, tc.wantEndpoint)
			}
			if got.plaintext != tc.wantPlaintext {
				t.Errorf("plaintext = %v; want %v", got.plaintext, tc.wantPlaintext)
			}
			if got.plaintext != (got.tls == nil) {
				t.Errorf("plaintext = %v with tls config %v; want a tls config exactly when TLS is used", got.plaintext, got.tls)
			}
			wantProtocol, wantURL := "tls", "https://"+tc.wantEndpoint
			if tc.wantPlaintext {
				wantProtocol, wantURL = "insecure", "http://"+tc.wantEndpoint
			}
			if u := got.url(); u != wantURL {
				t.Errorf("url = %q; want %q", u, wantURL)
			}
			if p := got.credentials().Info().SecurityProtocol; p != wantProtocol {
				t.Errorf("credentials protocol = %q; want %q", p, wantProtocol)
			}
		})
	}
}

// A per-signal variable decides only its own signal.
func TestResolveTransport_PerSignalVariableDecidesOnlyItsSignal(t *testing.T) {
	e := env{
		"OTEL_EXPORTER_OTLP_ENDPOINT":        "http://all.example:4317",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https://traces.example:4317",
	}
	for signal, want := range map[string]string{signalTraces: "traces.example:4317", signalMetrics: "all.example:4317", signalLogs: "all.example:4317"} {
		got, err := resolveTransport(e.lookup, signal, "", false)
		if err != nil {
			t.Fatalf("%s: resolveTransport error = %v", signal, err)
		}
		if got.endpoint != want {
			t.Errorf("%s: endpoint = %q; want %q", signal, got.endpoint, want)
		}
		if got.plaintext != (signal != signalTraces) {
			t.Errorf("%s: plaintext = %v", signal, got.plaintext)
		}
	}
}

// The CA file verifies the collector and the client key pair is presented.
func TestResolveTransport_CertificateFiles(t *testing.T) {
	dir := t.TempDir()
	caFile, certFile, keyFile := writeTestCertificates(t, dir)

	got, err := resolveTransport(env{
		"OTEL_EXPORTER_OTLP_ENDPOINT":           "https://collector.example:4317",
		"OTEL_EXPORTER_OTLP_CERTIFICATE":        caFile,
		"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": certFile,
		"OTEL_EXPORTER_OTLP_CLIENT_KEY":         keyFile,
	}.lookup, signalTraces, "", false)
	if err != nil {
		t.Fatalf("resolveTransport error = %v; want nil", err)
	}
	if got.tls == nil || got.tls.RootCAs == nil {
		t.Fatalf("tls = %+v; want the CA file's pool as RootCAs", got.tls)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatal(err)
	}
	want := x509.NewCertPool()
	want.AppendCertsFromPEM(caPEM)
	if !got.tls.RootCAs.Equal(want) {
		t.Errorf("RootCAs is not the pool of the CA file")
	}
	if len(got.tls.Certificates) != 1 {
		t.Errorf("client certificates = %d; want 1", len(got.tls.Certificates))
	}

	// Without a CA file, the system roots verify the collector.
	got, err = resolveTransport(env{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://collector.example:4317"}.lookup, signalTraces, "", false)
	if err != nil {
		t.Fatalf("resolveTransport error = %v; want nil", err)
	}
	if got.tls.RootCAs != nil || len(got.tls.Certificates) != 0 {
		t.Errorf("tls = %+v; want the system roots and no client certificate", got.tls)
	}
}

// A certificate variable that cannot be used is an error naming the
// variable, whatever the endpoint's scheme.
func TestResolveTransport_UnusableCertificateFileFails(t *testing.T) {
	dir := t.TempDir()
	caFile, certFile, keyFile := writeTestCertificates(t, dir)
	missing := filepath.Join(dir, "missing.crt")
	notPEM := filepath.Join(dir, "not-pem.crt")
	if err := os.WriteFile(notPEM, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		env      env
		wantName string
	}{
		{name: "an unreadable CA file",
			env:      env{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://collector.example:4317", "OTEL_EXPORTER_OTLP_CERTIFICATE": missing},
			wantName: "OTEL_EXPORTER_OTLP_CERTIFICATE"},
		{name: "an unreadable CA file with an http endpoint",
			env:      env{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector.example:4317", "OTEL_EXPORTER_OTLP_CERTIFICATE": missing},
			wantName: "OTEL_EXPORTER_OTLP_CERTIFICATE"},
		{name: "an unreadable per-signal CA file",
			env:      env{"OTEL_EXPORTER_OTLP_CERTIFICATE": caFile, "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE": missing},
			wantName: "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE"},
		{name: "a CA file holding no certificate",
			env:      env{"OTEL_EXPORTER_OTLP_CERTIFICATE": notPEM},
			wantName: "OTEL_EXPORTER_OTLP_CERTIFICATE"},
		{name: "an unreadable client certificate",
			env:      env{"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": missing, "OTEL_EXPORTER_OTLP_CLIENT_KEY": keyFile},
			wantName: "OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE"},
		{name: "an unreadable client key",
			env:      env{"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": certFile, "OTEL_EXPORTER_OTLP_CLIENT_KEY": missing},
			wantName: "OTEL_EXPORTER_OTLP_CLIENT_KEY"},
		{name: "a client certificate without its key",
			env:      env{"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": certFile},
			wantName: "OTEL_EXPORTER_OTLP_CLIENT_KEY"},
		{name: "a client key without its certificate",
			env:      env{"OTEL_EXPORTER_OTLP_CLIENT_KEY": keyFile},
			wantName: "OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE"},
		{name: "a client key that does not match its certificate",
			env:      env{"OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE": certFile, "OTEL_EXPORTER_OTLP_CLIENT_KEY": notPEM},
			wantName: "OTEL_EXPORTER_OTLP_CLIENT_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveTransport(tc.env.lookup, signalTraces, "", false)
			if err == nil {
				t.Fatalf("resolveTransport error = nil; want an error naming %s", tc.wantName)
			}
			if !strings.Contains(err.Error(), tc.wantName) {
				t.Errorf("error = %q; want it to name %s", err, tc.wantName)
			}
		})
	}
}

// An endpoint or INSECURE value the rules do not define is an error naming
// the variable.
func TestResolveTransport_RefusesUndefinedValues(t *testing.T) {
	cases := []struct {
		name     string
		env      env
		wantName string
	}{
		{name: "a scheme other than http or https",
			env:      env{"OTEL_EXPORTER_OTLP_ENDPOINT": "grpc://collector.example:4317"},
			wantName: "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{name: "a URL with no host",
			env:      env{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https://"},
			wantName: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"},
		{name: "an INSECURE value that is not a boolean",
			env:      env{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector.example:4317", "OTEL_EXPORTER_OTLP_INSECURE": "yes"},
			wantName: "OTEL_EXPORTER_OTLP_INSECURE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveTransport(tc.env.lookup, signalTraces, "", false)
			if err == nil || !strings.Contains(err.Error(), tc.wantName) {
				t.Errorf("error = %v; want an error naming %s", err, tc.wantName)
			}
		})
	}
}

// Init fails on a certificate file it cannot read, names the variable and
// installs no provider, so the server continues without telemetry.
func TestInit_UnreadableCertificateFailsAndInstallsNoProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", filepath.Join(t.TempDir(), "missing.crt"))

	tracerBefore := otel.GetTracerProvider()
	meterBefore := otel.GetMeterProvider()
	loggerBefore := global.GetLoggerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(tracerBefore)
		otel.SetMeterProvider(meterBefore)
		global.SetLoggerProvider(loggerBefore)
	})

	shutdown, err := Init(context.Background(), Config{})
	if err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_CERTIFICATE") {
		t.Errorf("Init error = %v; want an error naming OTEL_EXPORTER_OTLP_CERTIFICATE", err)
	}
	if shutdown != nil {
		t.Errorf("Init returned a shutdown func; want none when it fails")
	}
	if otel.GetTracerProvider() != tracerBefore || otel.GetMeterProvider() != meterBefore || global.GetLoggerProvider() != loggerBefore {
		t.Errorf("Init installed a provider")
	}
}

// writeTestCertificates writes a self-signed CA certificate, and a client
// certificate with its key, in PEM to dir and returns their paths.
func writeTestCertificates(t *testing.T, dir string) (caFile, certFile, keyFile string) {
	t.Helper()
	write := func(name, blockType string, der []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	newCert := func(serial int64, cn string, isCA bool) []byte {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(serial),
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  isCA,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		if !isCA {
			keyDER, err := x509.MarshalECPrivateKey(key)
			if err != nil {
				t.Fatal(err)
			}
			keyFile = write("client-key.txt", "EC PRIVATE KEY", keyDER)
		}
		return der
	}
	caFile = write("ca.crt", "CERTIFICATE", newCert(1, "transport test CA", true))
	certFile = write("client.crt", "CERTIFICATE", newCert(2, "transport test client", false))
	return caFile, certFile, keyFile
}

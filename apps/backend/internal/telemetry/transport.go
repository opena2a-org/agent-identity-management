package telemetry

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// The signals Init exports, as they appear in the per-signal variable names
// (OTEL_EXPORTER_OTLP_TRACES_ENDPOINT and so on).
const (
	signalTraces  = "TRACES"
	signalMetrics = "METRICS"
	signalLogs    = "LOGS"
)

// transport is how one signal's OTLP/gRPC exporter reaches the collector.
type transport struct {
	// endpoint is the gRPC target, host:port.
	endpoint string
	// plaintext is true for an unencrypted connection.
	plaintext bool
	// tls verifies the collector and carries the client's key pair. It is
	// nil when plaintext is true.
	tls *tls.Config
}

// credentials returns the transport credentials an exporter dials with.
// Init passes them explicitly, so they win over what the exporter library
// reads from the environment itself.
func (t transport) credentials() credentials.TransportCredentials {
	if t.plaintext {
		return insecure.NewCredentials()
	}
	return credentials.NewTLS(t.tls)
}

// url returns the endpoint in URL form, with the scheme of the decided
// connection.
func (t transport) url() string {
	if t.plaintext {
		return "http://" + t.endpoint
	}
	return "https://" + t.endpoint
}

// lookupFunc reads an environment variable, as os.LookupEnv does.
type lookupFunc func(string) (string, bool)

// resolveTransport decides one signal's endpoint and TLS setting as the
// OTLP exporter specification defines them
// (https://opentelemetry.io/docs/specs/otel/protocol/exporter/). The
// endpoint is configured, the one passed in Config, when it is not empty,
// and is read from the environment otherwise. A per-signal variable wins
// over its generic one where it is set and not empty.
//
//   - An endpoint with the scheme https uses TLS and one with http is
//     plaintext, whatever OTEL_EXPORTER_OTLP_INSECURE says.
//   - An endpoint with no scheme (host:port) is plaintext when
//     OTEL_EXPORTER_OTLP_INSECURE is true or insecure is set, uses TLS when
//     the variable is false, and with neither uses TLS unless the host is
//     loopback (localhost, 127.0.0.0/8 or ::1).
//   - No endpoint at all is localhost:4317, so plaintext.
//
// For TLS, OTEL_EXPORTER_OTLP_CERTIFICATE names the CA file that verifies
// the collector (the system roots when unset), and
// OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE with OTEL_EXPORTER_OTLP_CLIENT_KEY
// name the client's key pair. A file that cannot be read or parsed is an
// error naming its variable, whichever scheme the endpoint has.
func resolveTransport(lookup lookupFunc, signal, configured string, insecureConfigured bool) (transport, error) {
	value, name := configured, "Config.Endpoint"
	if value == "" {
		value, name = signalValue(lookup, signal, "ENDPOINT")
	}
	if value == "" {
		value, name = defaultEndpoint, "the default endpoint"
	}

	var t transport
	host := ""
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return transport{}, fmt.Errorf("telemetry: %s: %w", name, err)
		}
		switch strings.ToLower(u.Scheme) {
		case "https":
		case "http":
			t.plaintext = true
		default:
			return transport{}, fmt.Errorf("telemetry: %s=%q: the scheme is %q; use https:// or http://", name, value, u.Scheme)
		}
		if u.Host == "" {
			return transport{}, fmt.Errorf("telemetry: %s=%q names no host", name, value)
		}
		t.endpoint, host = u.Host, u.Hostname()
	} else {
		t.endpoint, host = value, value
		if h, _, err := net.SplitHostPort(value); err == nil {
			host = h
		}
		insecureValue, insecureName := signalValue(lookup, signal, "INSECURE")
		switch {
		case insecureConfigured:
			t.plaintext = true
		case insecureValue != "":
			switch strings.ToLower(insecureValue) {
			case "true":
				t.plaintext = true
			case "false":
			default:
				return transport{}, fmt.Errorf("telemetry: %s=%q: want true or false", insecureName, insecureValue)
			}
		default:
			t.plaintext = isLoopback(host)
		}
	}

	tlsConfig, err := tlsFromEnv(lookup, signal)
	if err != nil {
		return transport{}, err
	}
	if !t.plaintext {
		t.tls = tlsConfig
	}
	return t, nil
}

// signalValue returns OTEL_EXPORTER_OTLP_<signal>_<suffix> when it is set
// and not empty, or else OTEL_EXPORTER_OTLP_<suffix>, with the name of the
// variable the value came from.
func signalValue(lookup lookupFunc, signal, suffix string) (value, name string) {
	name = "OTEL_EXPORTER_OTLP_" + signal + "_" + suffix
	if v, ok := lookup(name); ok && v != "" {
		return v, name
	}
	name = "OTEL_EXPORTER_OTLP_" + suffix
	v, _ := lookup(name)
	return v, name
}

// isLoopback reports whether host is localhost or a loopback address.
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// tlsFromEnv builds the TLS configuration from the certificate variables.
func tlsFromEnv(lookup lookupFunc, signal string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if path, name := signalValue(lookup, signal, "CERTIFICATE"); path != "" {
		caPEM, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("telemetry: %s: read the CA certificate: %w", name, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("telemetry: %s=%q holds no PEM certificate", name, path)
		}
		cfg.RootCAs = pool
	}

	certPath, certName := signalValue(lookup, signal, "CLIENT_CERTIFICATE")
	keyPath, keyName := signalValue(lookup, signal, "CLIENT_KEY")
	switch {
	case certPath == "" && keyPath == "":
	case certPath == "":
		return nil, fmt.Errorf("telemetry: %s is set and %s is not; set both or neither", keyName, certName)
	case keyPath == "":
		return nil, fmt.Errorf("telemetry: %s is set and %s is not; set both or neither", certName, keyName)
	default:
		certPEM, err := os.ReadFile(filepath.Clean(certPath))
		if err != nil {
			return nil, fmt.Errorf("telemetry: %s: read the client certificate: %w", certName, err)
		}
		keyPEM, err := os.ReadFile(filepath.Clean(keyPath))
		if err != nil {
			return nil, fmt.Errorf("telemetry: %s: read the client key: %w", keyName, err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("telemetry: %s and %s: %w", certName, keyName, err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

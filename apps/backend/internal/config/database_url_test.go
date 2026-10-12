package config

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/lib/pq"
)

// errNothingDialed is what recordingDialer answers every dial with.
var errNothingDialed = errors.New("the test dials nothing")

// recordingDialer records where the driver would connect and connects
// nowhere.
type recordingDialer struct {
	network, address string
}

func (r *recordingDialer) Dial(network, address string) (net.Conn, error) {
	r.network, r.address = network, address
	return nil, errNothingDialed
}

func (r *recordingDialer) DialTimeout(network, address string, _ time.Duration) (net.Conn, error) {
	return r.Dial(network, address)
}

// A connection URL built from the POSTGRES_* settings carries every user,
// password and database name to the driver as it is configured, including a
// single quote and a backslash, which end or escape a quoted value in a
// key=value connection string. The driver accepts the URL and dials the
// configured host and port, for a host name, an IP address and the directory
// of a Unix socket, which the URL carries in its query since a URL's host
// cannot hold a slash.
func TestDatabaseConfigURL_CarriesEverySettingUnchanged(t *testing.T) {
	hosts := []struct {
		host             string
		network, address string
	}{
		{"db.internal", "tcp", "db.internal:6543"},
		{"::1", "tcp", "[::1]:6543"},
		{"/var/run/postgresql", "unix", "/var/run/postgresql/.s.PGSQL.6543"},
		{"/cloudsql/project:region:instance", "unix", "/cloudsql/project:region:instance/.s.PGSQL.6543"},
		{"/srv/pg sockets?&=#%+", "unix", "/srv/pg sockets?&=#%+/.s.PGSQL.6543"},
	}
	for _, h := range hosts {
		for _, password := range []string{`it's`, `back\slash`, `ends-with\`, `p'a\ss w@rd:/?#%`} {
			d := DatabaseConfig{Host: h.host, Port: 6543, User: `corp\aim owner`, Password: password, Database: "identity records", SSLMode: "require"}
			dsn := d.URL()

			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("host %q, password %q: the URL does not parse: %v", h.host, password, err)
			}
			got, ok := u.User.Password()
			if !ok || got != password {
				t.Errorf("host %q, password %q: the URL carries the password %q", h.host, password, got)
			}
			if u.User.Username() != d.User {
				t.Errorf("host %q, password %q: the URL carries the user %q, want %q", h.host, password, u.User.Username(), d.User)
			}
			if u.Path != "/identity records" || u.Query().Get("sslmode") != "require" {
				t.Errorf("host %q, password %q: path %q, sslmode %q", h.host, password, u.Path, u.Query().Get("sslmode"))
			}

			connector, err := pq.NewConnector(dsn)
			if err != nil {
				t.Errorf("host %q, password %q: the driver refuses the URL: %v", h.host, password, err)
				continue
			}
			dialer := &recordingDialer{}
			connector.Dialer(dialer)
			if _, err := connector.Connect(context.Background()); !errors.Is(err, errNothingDialed) {
				t.Errorf("host %q, password %q: the driver did not reach the dial: %v", h.host, password, err)
			}
			if dialer.network != h.network || dialer.address != h.address {
				t.Errorf("host %q, password %q: the driver dials %s %q, want %s %q", h.host, password, dialer.network, dialer.address, h.network, h.address)
			}
		}
	}
}

// With no password, as LoadDatabase allows for local development, the URL
// names the user alone. For the directory of a Unix socket the URL's host is
// empty, and the directory and the port are its host and port parameters.
func TestDatabaseConfigURL_NoPassword(t *testing.T) {
	d := DatabaseConfig{Host: "db", Port: 5432, User: "aim", Database: "aim", SSLMode: "disable"}
	if got, want := d.URL(), "postgres://aim@db:5432/aim?sslmode=disable"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}

	d.Host = "/var/run/postgresql"
	if got, want := d.URL(), "postgres://aim@/aim?host=%2Fvar%2Frun%2Fpostgresql&port=5432&sslmode=disable"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
}

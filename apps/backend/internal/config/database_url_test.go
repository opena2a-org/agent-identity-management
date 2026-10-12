package config

import (
	"net/url"
	"testing"

	"github.com/lib/pq"
)

// A connection URL built from the POSTGRES_* settings carries every user,
// password and database name to the driver as it is configured, including a
// single quote and a backslash, which end or escape a quoted value in a
// key=value connection string.
func TestDatabaseConfigURL_CarriesEverySettingUnchanged(t *testing.T) {
	for _, password := range []string{`it's`, `back\slash`, `ends-with\`, `p'a\ss w@rd:/?#%`} {
		d := DatabaseConfig{Host: "db.internal", Port: 6543, User: `corp\aim owner`, Password: password, Database: "identity records", SSLMode: "require"}
		dsn := d.URL()

		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("password %q: the URL does not parse: %v", password, err)
		}
		got, ok := u.User.Password()
		if !ok || got != password {
			t.Errorf("password %q: the URL carries the password %q", password, got)
		}
		if u.User.Username() != d.User {
			t.Errorf("password %q: the URL carries the user %q, want %q", password, u.User.Username(), d.User)
		}
		if u.Host != "db.internal:6543" || u.Path != "/identity records" || u.Query().Get("sslmode") != "require" {
			t.Errorf("password %q: host %q, path %q, sslmode %q", password, u.Host, u.Path, u.Query().Get("sslmode"))
		}
		if _, err := pq.NewConnector(dsn); err != nil {
			t.Errorf("password %q: the driver refuses the URL: %v", password, err)
		}
	}
}

// With no password, as LoadDatabase allows for local development, the URL
// names the user alone.
func TestDatabaseConfigURL_NoPassword(t *testing.T) {
	d := DatabaseConfig{Host: "db", Port: 5432, User: "aim", Database: "aim", SSLMode: "disable"}
	if got, want := d.URL(), "postgres://aim@db:5432/aim?sslmode=disable"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
}

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// ErrDriverEnvironmentRefused opens what DriverEnvironmentRefusal answers for
// a connection string the driver would accept but for a variable of the
// environment. The variables at fault follow it, by name and never by value.
var ErrDriverEnvironmentRefused = errors.New("the driver refuses a setting it reads from the environment, not from the database connection string")

// driverEnvironmentSettings are the settings the driver reads from the
// environment and accepts one value of. Each is listed with the connection
// string setting that carries the accepted value, and with that value as an
// error names it. A setting in the connection string comes before the
// variable of the same setting.
var driverEnvironmentSettings = []struct {
	variable string
	setting  string
	accepted string
}{
	{"PGCLIENTENCODING", "client_encoding='UTF8'", "UTF8"},
	{"PGDATESTYLE", "datestyle='ISO, MDY'", `"ISO, MDY"`},
}

// DriverEnvironmentRefusal returns an error that names the environment
// variables for which the driver refuses dsn, and nil when the driver refuses
// dsn whatever those variables hold. It asks the driver: dsn is parsed again
// with the accepted settings written before it, which override the variables
// and which the settings of dsn itself still override. It reads no variable
// and holds no part of dsn. aim-bootstrap and aim-breakglass call it when the
// driver refuses their connection string.
func DriverEnvironmentRefusal(dsn string) error {
	// The driver reads a URL as the key=value string ParseURL makes of it.
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := pq.ParseURL(dsn)
		if err != nil {
			return nil
		}
		dsn = parsed
	}
	// acceptedWithout parses dsn with every variable overridden but the one at
	// index left, and with all of them overridden when left is -1.
	acceptedWithout := func(left int) bool {
		var settings []string
		for i, s := range driverEnvironmentSettings {
			if i != left {
				settings = append(settings, s.setting)
			}
		}
		_, err := pq.NewConnector(strings.Join(append(settings, dsn), " "))
		return err == nil
	}
	if !acceptedWithout(-1) {
		return nil
	}
	var refused []string
	for i, s := range driverEnvironmentSettings {
		if !acceptedWithout(i) {
			refused = append(refused, s.variable+" must be unset or "+s.accepted)
		}
	}
	if len(refused) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrDriverEnvironmentRefused, strings.Join(refused, ", and "))
}

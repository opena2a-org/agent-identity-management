package database

import "database/sql/driver"

// WrapConnector exposes newConnector to the external tests, which drive the
// census through an in-memory driver.
func WrapConnector(base driver.Connector, role string, census *Census) driver.Connector {
	return newConnector(base, role, census)
}

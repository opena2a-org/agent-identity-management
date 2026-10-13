package handlers

import "time"

// utcTime returns the same instant in UTC, or nil for nil. A time.Time prints
// with the location it carries, so a value read with a session's time zone or
// clocked on a server with a local offset would serialise with that offset.
// Every response that names a lastLoginAt passes it through here, so the API
// names the instant in one form (RFC 3339 with a Z suffix) at every route.
func utcTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

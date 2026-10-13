package handlers

import "time"

// utcTime returns the same instant in UTC, or nil for nil. A time.Time prints
// with the location it carries, so a value read with a session's time zone or
// clocked on a server with a local offset would serialise with that offset.
// Every handler in this package that answers with the lastLoginAt of a user a
// service handed it passes the value through here, so the API names the
// instant in one form (RFC 3339 with a Z suffix) at every route. The
// placeholder a sign-in builds for an account that awaits approval is not
// passed through: the handler builds it without a lastLoginAt.
func utcTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

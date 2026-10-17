package siem

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SignInFailure is a password sign-in a sign-in route refused. It carries no
// submitted email, password or token.
type SignInFailure struct {
	Time           time.Time
	Method         string // how the caller tried to sign in: "password"
	Reason         string // the check that refused the sign-in
	Route          string
	IPAddress      string
	UserAgent      string
	UserID         uuid.UUID // uuid.Nil unless the submitted email matched a user
	OrganizationID uuid.UUID // uuid.Nil unless the submitted email matched a user
}

// EventFromSignInFailure is the SIEM event of a refused sign-in, at severity
// low. It names the user and organization only when the email matched a
// user.
func EventFromSignInFailure(f SignInFailure) *SIEMEvent {
	details := map[string]interface{}{
		"method": f.Method,
		"reason": f.Reason,
		"route":  f.Route,
	}
	// The address and user agent are copied: the handler reads them as views
	// of the request buffer (c.Get, and c.IP behind a trusted proxy), which
	// the server rewrites when it reads the next request, and the forwarder
	// serializes the event after the request has ended. The observer that
	// builds this event runs inside the request, so here the views are still
	// this sign-in's own.
	if f.IPAddress != "" {
		details["ipAddress"] = strings.Clone(f.IPAddress)
	}
	if f.UserAgent != "" {
		details["userAgent"] = strings.Clone(f.UserAgent)
	}
	if f.UserID != uuid.Nil {
		details["userId"] = f.UserID.String()
	}
	if f.OrganizationID != uuid.Nil {
		details["organizationId"] = f.OrganizationID.String()
	}
	return &SIEMEvent{
		EventType: EventSignInFailed,
		Timestamp: f.Time.UTC(),
		Severity:  SeverityLow,
		Details:   details,
		Source:    "opena2a-aim",
	}
}

// FGADenial is an FGA decision that did not allow the request.
type FGADenial struct {
	Time           time.Time
	AgentID        uuid.UUID
	OrganizationID uuid.UUID // the agent's organization; uuid.Nil when it was not read
	Capability     string    // the requested capability
	Outcome        string    // DENY, DENY_CONTEXT, DENY_CHAIN, DENY_ATTRIBUTE or ERROR
	DeniedBy       string
	DeniedReason   string
}

// EventFromFGADenial is the SIEM event of a denied FGA decision, at severity
// medium.
func EventFromFGADenial(d FGADenial) *SIEMEvent {
	details := map[string]interface{}{
		"deniedBy":     d.DeniedBy,
		"deniedReason": d.DeniedReason,
	}
	if d.OrganizationID != uuid.Nil {
		details["organizationId"] = d.OrganizationID.String()
	}
	return &SIEMEvent{
		EventType:  EventFGADenied,
		Timestamp:  d.Time.UTC(),
		AgentID:    d.AgentID.String(),
		Capability: d.Capability,
		Outcome:    d.Outcome,
		Severity:   SeverityMedium,
		Details:    details,
		Source:     "opena2a-aim",
	}
}

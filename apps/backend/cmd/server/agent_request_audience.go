package main

import (
	"fmt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// agentRequestAudienceStartupLine returns the line the server logs at startup
// for AIM_PUBLIC_ORIGIN, the audience a signed agent request
// (domain.AgentRequestSchemeID) must name. A set value is quoted as
// config.Load holds it, so its case, its spaces and its path show, and a line
// feed or a carriage return in it is escaped and cannot start another line of
// the log. The value is not secret: a signed agent request names it in a
// request header.
//
// No route mounts AgentRequestV2Middleware, and the line says so; a test
// fails when a Go file of the backend outside the tests uses the middleware
// while the line still says it.
func agentRequestAudienceStartupLine(server config.ServerConfig) string {
	const subject = "Agent requests signed as " + domain.AgentRequestSchemeID + ": no route accepts them yet"
	if server.PublicOrigin == "" {
		return subject + " (AIM_PUBLIC_ORIGIN, the audience they must name, is not set)"
	}
	return fmt.Sprintf("%s (AIM_PUBLIC_ORIGIN, the audience they must name, is %q)", subject, server.PublicOrigin)
}

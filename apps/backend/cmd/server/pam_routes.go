package main

import "github.com/gofiber/fiber/v3"

// pamRouteDeps is the middleware and handlers the privileged access
// management routes mount. Production passes the real ones; a test passes
// stand-ins and still gets the real paths and the real middleware order.
type pamRouteDeps struct {
	Authenticate   fiber.Handler // dashboard session (JWT)
	Limit          fiber.Handler
	RequireMember  fiber.Handler // member, manager or admin
	RequireManager fiber.Handler // manager or admin
	RequireAdmin   fiber.Handler // admin

	ListTiers                   fiber.Handler
	RequestApproval             fiber.Handler
	ListApprovals               fiber.Handler
	GetApproval                 fiber.Handler
	DecideApproval              fiber.Handler
	DeclareEmergency            fiber.Handler
	ListEmergencies             fiber.Handler
	ExtendEmergency             fiber.Handler
	RevokeEmergency             fiber.Handler
	CreateCampaign              fiber.Handler
	ListCampaigns               fiber.Handler
	RecordCertificationDecision fiber.Handler
	ListCertificationDecisions  fiber.Handler
}

// registerPAMRoutes mounts the routes under /pam with the session check, the
// rate limiter and the role gate attached per route. Any signed-in user of an
// organization reads its tiers, approvals, emergency declarations, campaigns
// and certification decisions. A member, manager or admin requests an
// approval; an admin decides one, declares, extends and revokes an emergency
// and creates a campaign; a manager or admin records a certification
// decision.
func registerPAMRoutes(r fiber.Router, d pamRouteDeps) {
	r.Get("/pam/tiers", d.Authenticate, d.Limit, d.ListTiers)

	r.Post("/pam/approvals", d.Authenticate, d.Limit, d.RequireMember, d.RequestApproval)
	r.Get("/pam/approvals", d.Authenticate, d.Limit, d.ListApprovals)
	r.Get("/pam/approvals/:id", d.Authenticate, d.Limit, d.GetApproval)
	r.Post("/pam/approvals/:id/decision", d.Authenticate, d.Limit, d.RequireAdmin, d.DecideApproval)

	r.Post("/pam/emergencies", d.Authenticate, d.Limit, d.RequireAdmin, d.DeclareEmergency)
	r.Get("/pam/emergencies", d.Authenticate, d.Limit, d.ListEmergencies)
	r.Post("/pam/emergencies/:id/extend", d.Authenticate, d.Limit, d.RequireAdmin, d.ExtendEmergency)
	r.Post("/pam/emergencies/:id/revoke", d.Authenticate, d.Limit, d.RequireAdmin, d.RevokeEmergency)

	r.Post("/pam/campaigns", d.Authenticate, d.Limit, d.RequireAdmin, d.CreateCampaign)
	r.Get("/pam/campaigns", d.Authenticate, d.Limit, d.ListCampaigns)
	r.Post("/pam/campaigns/:id/decisions", d.Authenticate, d.Limit, d.RequireManager, d.RecordCertificationDecision)
	r.Get("/pam/campaigns/:id/decisions", d.Authenticate, d.Limit, d.ListCertificationDecisions)
}

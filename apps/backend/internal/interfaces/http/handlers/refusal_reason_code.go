package handlers

import "github.com/gofiber/fiber/v3"

// withReasonCode adds a refusal's machine-readable reason to body under reasonCode, the
// API's refusal-code member, and under code, the member these refusals first shipped with.
// Both are written from the one value, so a client reading either reads the same reason.
func withReasonCode(body fiber.Map, reason string) fiber.Map {
	body["reasonCode"] = reason
	body["code"] = reason
	return body
}

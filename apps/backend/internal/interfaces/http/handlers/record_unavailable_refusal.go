package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
)

// RecordUnavailableCode is the machine-readable reason of a change refused because its
// audit record could not be written: an expansion or a destruction of an agent's
// authorization state is made only together with its record, so nothing was changed.
const RecordUnavailableCode = "recordUnavailable"

// recordUnavailableRetryAfter is how long, in seconds, a failed record write keeps the
// server's record path unavailable (store.RecordPathWindow). The refusal sends it as
// Retry-After.
var recordUnavailableRetryAfter = int(store.RecordPathWindow / time.Second)

// RecordUnavailableMessage is the error line of that refusal. It names no cause: the
// cause is a database or chain error, which stays in the server log.
var RecordUnavailableMessage = fmt.Sprintf(
	"Nothing was changed: AIM writes an audit record of this change together with the change, and the record could not be written. Try again in %d minutes. If it fails again, an administrator can find the cause in the AIM server log.",
	(recordUnavailableRetryAfter+59)/60,
)

// respondRecordUnavailable answers a change the record path refused (err wraps
// transition.ErrRecordUnavailable) with 503, Retry-After and RecordUnavailableCode, and
// reports true. For any other error it writes nothing and reports false.
func respondRecordUnavailable(c fiber.Ctx, err error) (bool, error) {
	if !errors.Is(err, transition.ErrRecordUnavailable) {
		return false, nil
	}
	c.Set(fiber.HeaderRetryAfter, strconv.Itoa(recordUnavailableRetryAfter))
	return true, c.Status(fiber.StatusServiceUnavailable).JSON(withReasonCode(fiber.Map{
		"error": RecordUnavailableMessage,
	}, RecordUnavailableCode))
}

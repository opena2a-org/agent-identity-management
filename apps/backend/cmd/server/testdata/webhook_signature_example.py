# The verification example of docs/API.md, section "Verifying a delivery",
# copied verbatim between the two marker lines below
# (TestAPIDocsWebhookVerificationExampleIsTheFixture holds them equal), and a
# driver that runs it on deliveries TestWebhookReplayGuardRoundTrip captured.
#
#   python3 -I cmd/server/testdata/webhook_signature_example.py < cases.json
#
# cases.json is a list of {"name", "secret", "timestamp", "signature",
# "bodyHex", "now"}; the driver prints "<name>: verified" or "<name>: refused"
# for each case.

# --- docs/API.md example begins ---
import hashlib
import hmac

TOLERANCE_SECONDS = 300


def verify_delivery(secret, timestamp, signature, body, now):
    """Return True when a delivery is signed with secret and was sent within
    5 minutes of now.

    timestamp and signature are the X-Webhook-Timestamp and X-Webhook-Signature
    header values, body is the raw request body as bytes, and now is the
    receiver's clock in Unix seconds, for example time.time().
    """
    signed = timestamp.encode() + b"." + body
    expected = "v1=" + hmac.new(secret.encode(), signed, hashlib.sha256).hexdigest()
    entries = [entry.strip() for entry in signature.split(",")]
    if not any(hmac.compare_digest(expected.encode(), entry.encode()) for entry in entries):
        return False
    return abs(now - int(timestamp)) <= TOLERANCE_SECONDS
# --- docs/API.md example ends ---

import json
import sys


def main():
    for case in json.load(sys.stdin):
        verified = verify_delivery(
            case["secret"],
            case["timestamp"],
            case["signature"],
            bytes.fromhex(case["bodyHex"]),
            case["now"],
        )
        print("%s: %s" % (case["name"], "verified" if verified else "refused"))


if __name__ == "__main__":
    main()

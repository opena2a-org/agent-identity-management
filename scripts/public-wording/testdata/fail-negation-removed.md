Fixture for scripts/test-check-public-wording.sh. The statements below are deliberately wrong;
each line marked with a hit comment must be reported, and no other line may be.

An accurate negative stops being allowlisted when its negation is removed:

<!-- hit --> The `audit_logs` table records actions and is access-controlled, and is append-only at the database layer and is cryptographically signed.

<!-- hit --> AU-9 is met by a tamper-evident signing scheme.

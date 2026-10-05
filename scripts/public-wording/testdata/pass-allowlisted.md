Fixture for scripts/test-check-public-wording.sh. Every sentence below is on the allowlist:
it uses a barred term to say, accurately, what the code does not do. No line may be reported.

As the table row reads in SECURITY.md:

| FedRAMP AC-2 / AU-9 (Account Management / Audit Protection) | Roadmap | The `audit_logs` table records actions and is access-controlled, but is not append-only at the database layer and is not cryptographically signed. AU-9's "protection of audit information" requirement is partially met by RBAC; it is not met by a tamper-evident signing scheme. We do not claim FedRAMP authorization. |

The same sentences wrapped over several lines:

The `audit_logs` table records actions and is access-controlled, but is not append-only
at the database layer and is not cryptographically signed.

AU-9's "protection of audit information" requirement is partially met by RBAC; it is
not met by a tamper-evident signing scheme.


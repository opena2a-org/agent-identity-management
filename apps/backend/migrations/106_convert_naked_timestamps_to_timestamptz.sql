-- Migration 106: convert every `timestamp without time zone` column to `timestamptz`.
--
-- WHY
-- ---
-- `lib/pq` sends a Go `time.Time` as its wall clock. A `timestamp` column drops the offset,
-- and on read `lib/pq` labels the naked value UTC. The instant therefore shifts by the writing
-- process's UTC offset. Measured with the repo's pinned lib/pq v1.10.9 on PostgreSQL 16,
-- writing one intended instant (2026-08-06T12:00:00Z) from four process zones:
--
--   process zone     offset   read back (naked)      read back (timestamptz)  drift
--   Etc/UTC          +0       2026-08-06T12:00:00Z   2026-08-06T12:00:00Z     +0h
--   Europe/Berlin    +2       2026-08-06T14:00:00Z   2026-08-06T12:00:00Z     +2h
--   Asia/Tokyo       +9       2026-08-06T21:00:00Z   2026-08-06T12:00:00Z     +9h
--   America/Denver   -6       2026-08-06T06:00:00Z   2026-08-06T12:00:00Z     -6h
--
-- East of UTC an API key stays valid past its stated expiry by the offset, at the
-- `time.Now().After(*apiKey.ExpiresAt)` check in api_key_service.go. West of UTC it expires
-- early. A `timestamptz` column is correct in every zone.
--
-- `api_keys.expires_at` is the only converted column where a wrong instant changes an ACCESS
-- DECISION. Stating the other named columns precisely, because it is easy to overclaim here:
--   * `agents.pqc_key_expires_at` is NOT read by EnforceKeyExpiry -- that reads
--     `agent.KeyExpiresAt`, i.e. `key_expires_at`, which was already timestamptz. The naked
--     column reaches three JSON responses and no enforcement path.
--   * `agent_capabilities.revoked_at` is enforced only as `revoked_at IS NULL`, which is
--     zone-independent. The stored instant was wrong for audit and display, not for access.
--   * `audit_logs.timestamp` is an evidentiary-integrity problem: the recorded time of an
--     audited action depended on the writer's zone.
--
-- There is a second, independent path that no amount of Go-side discipline reaches: the
-- PostgreSQL *session* zone. The DSN sets no TimeZone, so every `DEFAULT NOW()` on these
-- columns -- and migration 092's `NEW.verified_at := NOW()` trigger -- casts a timestamptz
-- into a naked column under whatever zone the operator's session has. Converting the column
-- type is the only fix that closes both paths.
--
-- SCOPE
-- -----
-- All 42 naked columns, not a security-semantic subset. The boundary is the column TYPE, not
-- its name: a name-based predicate over expires/revoked/grace finds only 3 of the 42 and misses
-- `audit_logs.timestamp`, `agent_capabilities.granted_at` and `agents.verified_at`.
--
-- EXISTING ROWS
-- -------------
-- The conversion reinterprets each stored wall-clock value as UTC. For rows written by a
-- process at UTC that preserves the instant exactly; for rows written at an offset it adopts
-- the shifted instant that the application was already reading back. Operators can check their
-- own data before running this -- `agents` writes one timestamptz and one naked column from the
-- same clock in a single INSERT, so any historical drift is visible directly:
--
--   SELECT count(*) AS rows_checked,
--          max(abs(extract(epoch FROM
--            ((pqc_key_created_at AT TIME ZONE 'UTC') - key_created_at)))) AS max_drift_seconds
--   FROM agents
--   WHERE pqc_key_created_at IS NOT NULL AND key_created_at IS NOT NULL;
--
-- A `rows_checked` of 0 means the probe proved nothing -- it is not a pass. Drift under a
-- minute means the rows were written at UTC and the reinterpretation is exact.
--
-- FORM
-- ----
-- The implicit cast under a UTC session does not rewrite the TABLE HEAP: verified by comparing
-- `pg_class.relfilenode` before and after on all affected tables at 5,000,000 rows (unchanged).
-- The `USING col AT TIME ZONE 'UTC'` spelling produces identical values but forces a full heap
-- rewrite (relfilenode changes), so it is not used here.
--
-- INDEXES ON A CONVERTED COLUMN ARE STILL REBUILT, and that is the real cost. 20 indexes touch
-- these columns. Measured at 2,000,000 rows: the ALTER takes 0.33 ms with no index on the
-- column and 238 ms with one btree -- i.e. the cost is an index build, and it scales with the
-- table. Every ACCESS EXCLUSIVE lock is held until COMMIT, so all the tables below are locked
-- for as long as the slowest index rebuild takes. `lock_timeout` bounds how long we WAIT for a
-- lock, not how long we hold one.
--
-- `SET LOCAL TimeZone` below is load-bearing, not cosmetic. Under a session at, say,
-- America/Denver the same implicit cast silently converts a stored `12:00:00` to the instant
-- `18:00:00Z` -- six hours wrong, no error raised. The assertion that follows turns that silent
-- corruption into a hard abort. Both the correctness of the cast and the no-rewrite
-- optimization depend on the session being UTC.
--
-- No BEGIN/COMMIT here: cmd/migrate/main.go and cmd/server's runMigrations both run each
-- migration inside one transaction, which is also what scopes the SET LOCAL statements below.
--
-- RUN THIS THROUGH ONE OF THOSE RUNNERS. Applied by hand with `psql -f` and no explicit
-- transaction, every guarantee above inverts: `SET LOCAL` outside a transaction is a no-op that
-- only WARNs, the assertion's EXCEPTION does not stop the script without ON_ERROR_STOP, and a
-- failing postcondition cannot roll back statements that already autocommitted. If you must run
-- it manually, use `psql -v ON_ERROR_STOP=1 --single-transaction`.

SET LOCAL TimeZone = 'UTC';

-- Fail fast rather than queueing an ACCESS EXCLUSIVE request in front of every subsequent
-- read and write to these tables. A pending lock behind one long-running SELECT on audit_logs
-- would stall audit writes and saturate the connection pool. Timing out is retryable; that is
-- not.
SET LOCAL lock_timeout = '5s';

-- `SET LOCAL` outside a transaction is a no-op that raises only a WARNING. If that ever
-- happens, the casts below would silently reinterpret every value in the ambient zone.
DO $$
BEGIN
    IF current_setting('TimeZone') <> 'UTC' THEN
        RAISE EXCEPTION
            'migration 106 requires the session TimeZone to be UTC, got %. '
            'Values would be reinterpreted in the wrong zone.', current_setting('TimeZone');
    END IF;
END $$;

ALTER TABLE agent_actions
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE agent_behavioral_baselines
    ALTER COLUMN baseline_period_end   TYPE timestamptz,
    ALTER COLUMN baseline_period_start TYPE timestamptz,
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz;

ALTER TABLE agent_capabilities
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN granted_at            TYPE timestamptz,
    ALTER COLUMN revoked_at            TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz;

ALTER TABLE agent_compliance_events
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE agent_health_checks
    ALTER COLUMN check_time            TYPE timestamptz,
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE agent_user_feedback
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE agents
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN last_active           TYPE timestamptz,
    ALTER COLUMN pqc_key_created_at    TYPE timestamptz,
    ALTER COLUMN pqc_key_expires_at    TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz,
    ALTER COLUMN verified_at           TYPE timestamptz;

ALTER TABLE alerts
    ALTER COLUMN acknowledged_at       TYPE timestamptz,
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE api_keys
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN expires_at            TYPE timestamptz,
    ALTER COLUMN last_used_at          TYPE timestamptz;

ALTER TABLE audit_logs
    ALTER COLUMN "timestamp"           TYPE timestamptz;

ALTER TABLE capability_violations
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE isolation_attestations
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN reported_at           TYPE timestamptz;

ALTER TABLE nanomind_tme_evaluations
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN evaluated_at          TYPE timestamptz;

ALTER TABLE organizations
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz;

ALTER TABLE trust_scores
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN last_calculated       TYPE timestamptz;

ALTER TABLE user_feedback
    ALTER COLUMN created_at            TYPE timestamptz;

ALTER TABLE user_registration_requests
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN requested_at          TYPE timestamptz,
    ALTER COLUMN reviewed_at           TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz;

ALTER TABLE users
    ALTER COLUMN created_at            TYPE timestamptz,
    ALTER COLUMN last_login_at         TYPE timestamptz,
    ALTER COLUMN updated_at            TYPE timestamptz;

-- The migration bookkeeping table itself. The two runners disagreed about its type:
-- cmd/migrate creates `applied_at TIMESTAMPTZ`, but cmd/server (the container's CMD, which
-- auto-migrates at boot) created it naked. So on every container-bootstrapped deployment this
-- column is `timestamp without time zone`, and the postcondition below would abort the whole
-- migration on exactly the deployments that matter most.
--
-- `CREATE TABLE IF NOT EXISTS` never changes an existing table, so cmd/server's corrected DDL
-- only helps brand-new databases; this statement is what fixes the ones already out there.
-- Converting the table we are about to INSERT into is safe: the same transaction already holds
-- the lock, and on a database created by cmd/migrate this is a no-op.
ALTER TABLE schema_migrations
    ALTER COLUMN applied_at            TYPE timestamptz;

-- The statements above are enumerated against this repo's schema. A deployment carrying extra
-- tables -- aim-cloud's superset, or a self-hoster's own -- may hold naked columns this file
-- does not name. Abort rather than half-converting: a partially converted schema is the state
-- this migration exists to eliminate.
--
-- This reads pg_attribute rather than information_schema.columns, for two reasons that were
-- both measured rather than assumed:
--
--   1. information_schema is PRIVILEGE-FILTERED. It shows only objects the current role has
--      some privilege on, so the same query returned 5 rows as superuser and 0 rows as a
--      dedicated migration role against an identical schema. The guard would have passed
--      silently in precisely the case it exists for -- extra tables owned by another role.
--   2. information_schema.columns reports an array column's data_type as 'ARRAY', so a
--      `timestamp without time zone[]` column is invisible to a data_type comparison.
--
-- Checking atttypid directly against both the scalar and array types is privilege-independent
-- and also reaches matviews, views, foreign tables and standalone composite types. Objects
-- owned by an extension are excluded: their schema is the extension's business, not ours.
DO $$
DECLARE
    remaining int;
    offenders text;
BEGIN
    SELECT count(*),
           coalesce(string_agg(n.nspname || '.' || c.relname || '.' || a.attname,
                               ', ' ORDER BY n.nspname, c.relname, a.attname), '')
      INTO remaining, offenders
      FROM pg_attribute a
      JOIN pg_class c     ON c.oid = a.attrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
      JOIN pg_type ty     ON ty.oid = a.atttypid
     WHERE a.attnum > 0
       AND NOT a.attisdropped
       -- Direct naked timestamp, or a DOMAIN over one. A domain's atttypid is the domain's own
       -- OID, so comparing atttypid alone would let `CREATE DOMAIN d AS TIMESTAMP` through --
       -- and that is the one shape the source lint also misses, so the guard would have no
       -- half covering it.
       AND (
             a.atttypid IN ('pg_catalog.timestamp'::regtype, 'pg_catalog.timestamp[]'::regtype)
          OR (ty.typtype = 'd'
              AND ty.typbasetype IN ('pg_catalog.timestamp'::regtype,
                                     'pg_catalog.timestamp[]'::regtype))
       )
       AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
       -- Another session's TEMP table is visible in pg_class and lives in a pg_temp_N schema.
       -- Without this, the migration aborts based on what unrelated sessions happen to be
       -- doing, and names a schema the operator cannot inspect.
       AND c.relpersistence <> 't'
       AND c.relkind IN ('r', 'p', 'm', 'v', 'f', 'c')
       -- pg_depend.objid is only meaningful together with its classid; without the filter any
       -- extension-owned function or type whose OID happens to equal a table's OID would
       -- silently exempt that table.
       AND NOT EXISTS (
           SELECT 1 FROM pg_depend d
            WHERE d.classid = 'pg_class'::regclass
              AND d.objid = c.oid
              AND d.deptype = 'e'
       );

    IF remaining <> 0 THEN
        RAISE EXCEPTION
            'migration 106 left % naked timestamp column(s): %', remaining, offenders;
    END IF;
END $$;

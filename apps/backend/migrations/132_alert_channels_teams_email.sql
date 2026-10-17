-- Migration 132: alert channels to Microsoft Teams workflows and email.
--
-- Widens the kind check of alert_channels (migration 131) to two more kinds:
--
--   teams  config {"webhookUrl": ...}: the https URL of a Teams workflow's
--          "When a Teams webhook request is received" trigger. It is the
--          credential that posts to the workflow, masked in every answer as
--          a Slack webhook URL is.
--   email  config {"recipients": [...]}: 1 to 20 bare email addresses, sent
--          each alert through the server's email service. Addresses are not
--          credentials and answers carry them as stored.
--
-- Idempotent: the check is dropped if it exists and added again.

ALTER TABLE alert_channels DROP CONSTRAINT IF EXISTS alert_channels_kind_check;

ALTER TABLE alert_channels
    ADD CONSTRAINT alert_channels_kind_check CHECK (kind IN ('slack', 'pagerduty', 'teams', 'email'));

COMMENT ON TABLE alert_channels IS
    'Destinations an organization''s alerts are sent to (Slack incoming webhooks, PagerDuty Events API v2, Microsoft Teams workflows, email addresses); config holds the destination: a credential for slack, pagerduty and teams, the recipient addresses for email';

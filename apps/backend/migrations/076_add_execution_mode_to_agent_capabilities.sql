-- Add per-capability execution mode
-- Controls whether capability executes immediately, with notification, or requires human review
-- Default 'auto' preserves existing behavior (backward compatible)

ALTER TABLE agent_capabilities ADD COLUMN IF NOT EXISTS execution_mode VARCHAR(20) NOT NULL DEFAULT 'auto';

COMMENT ON COLUMN agent_capabilities.execution_mode IS 'Execution mode: auto (immediate), notify (execute + alert), review (queue for human approval)';

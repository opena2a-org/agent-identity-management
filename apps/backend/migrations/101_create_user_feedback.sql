-- Migration 101: User feedback for the trust score User Feedback factor
-- Backs trust factor 8 (user_feedback, 2% weight). Replaces the hardcoded
-- 0.5 neutral the calculator returned when no feedback collection existed.

CREATE TABLE IF NOT EXISTS user_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_feedback_agent_id ON user_feedback(agent_id);
CREATE INDEX IF NOT EXISTS idx_user_feedback_agent_created ON user_feedback(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_feedback_organization_id ON user_feedback(organization_id);

-- Chat System Tables Migration
-- Creates tables for chat functionality with message limits and activity tracking
-- Migration: 050_create_chat_system_tables.sql
-- Date: 2025-10-29

-- Chat conversations table
CREATE TABLE IF NOT EXISTS chat_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT 'New Conversation',
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, archived, deleted
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ,
    
    -- Indexes for performance
    CONSTRAINT chk_conversation_status CHECK (status IN ('active', 'archived', 'deleted'))
);

-- Indexes for chat conversations
CREATE INDEX IF NOT EXISTS idx_chat_conversations_org_id ON chat_conversations(organization_id);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_id ON chat_conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_agent_id ON chat_conversations(agent_id);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_status ON chat_conversations(status);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_last_message ON chat_conversations(last_message_at);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_created_at ON chat_conversations(created_at);

-- Chat messages table
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    message_type VARCHAR(50) NOT NULL DEFAULT 'text', -- text, image, file, system
    content TEXT NOT NULL,
    role VARCHAR(50) NOT NULL, -- user, agent, system
    metadata JSONB DEFAULT '{}'::jsonb,
    parent_message_id UUID REFERENCES chat_messages(id), -- for threading
    is_edited BOOLEAN DEFAULT FALSE,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Indexes for performance
    CONSTRAINT chk_message_type CHECK (message_type IN ('text', 'image', 'file', 'system')),
    CONSTRAINT chk_message_role CHECK (role IN ('user', 'agent', 'system'))
);

-- Indexes for chat messages
CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_id ON chat_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_agent_id ON chat_messages(agent_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_role ON chat_messages(role);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_parent_id ON chat_messages(parent_message_id);

-- User daily message limits table
CREATE TABLE IF NOT EXISTS user_daily_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    message_count INTEGER NOT NULL DEFAULT 0,
    daily_limit INTEGER NOT NULL DEFAULT 5000,
    is_limit_exceeded BOOLEAN DEFAULT FALSE,
    last_reset_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Ensure one record per user per day
    UNIQUE(user_id, date)
);

-- Indexes for daily limits
CREATE INDEX IF NOT EXISTS idx_user_daily_limits_user_id ON user_daily_limits(user_id);
CREATE INDEX IF NOT EXISTS idx_user_daily_limits_org_id ON user_daily_limits(organization_id);
CREATE INDEX IF NOT EXISTS idx_user_daily_limits_date ON user_daily_limits(date);
CREATE INDEX IF NOT EXISTS idx_user_daily_limits_exceeded ON user_daily_limits(is_limit_exceeded);

-- Agent activity tracking table
CREATE TABLE IF NOT EXISTS agent_activity_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    conversation_id UUID REFERENCES chat_conversations(id) ON DELETE SET NULL,
    activity_type VARCHAR(100) NOT NULL, -- message_sent, message_received, conversation_started, conversation_ended, limit_exceeded, etc.
    activity_data JSONB DEFAULT '{}'::jsonb,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Indexes for performance
    CONSTRAINT chk_activity_type CHECK (activity_type IN (
        'message_sent', 'message_received', 'conversation_started', 'conversation_ended',
        'limit_exceeded', 'agent_response_generated', 'user_typing', 'user_stopped_typing',
        'conversation_archived', 'conversation_deleted', 'message_edited', 'file_uploaded'
    ))
);

-- Agent activity logs table
CREATE TABLE IF NOT EXISTS agent_activity_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    conversation_id UUID REFERENCES chat_conversations(id) ON DELETE SET NULL,
    activity_type VARCHAR(100) NOT NULL,
    activity_data JSONB DEFAULT '{}'::jsonb,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for activity logs
CREATE INDEX IF NOT EXISTS idx_agent_activity_org_id ON agent_activity_logs(organization_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_agent_id ON agent_activity_logs(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_user_id ON agent_activity_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_conversation_id ON agent_activity_logs(conversation_id);
CREATE INDEX IF NOT EXISTS idx_agent_activity_type ON agent_activity_logs(activity_type);
CREATE INDEX IF NOT EXISTS idx_agent_activity_created_at ON agent_activity_logs(created_at);

-- Chat system configuration table
CREATE TABLE IF NOT EXISTS chat_system_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    config_key VARCHAR(100) NOT NULL,
    config_value JSONB NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Ensure one config per key per organization
    UNIQUE(organization_id, config_key)
);

-- Indexes for chat config
CREATE INDEX IF NOT EXISTS idx_chat_config_org_id ON chat_system_config(organization_id);
CREATE INDEX IF NOT EXISTS idx_chat_config_key ON chat_system_config(config_key);
CREATE INDEX IF NOT EXISTS idx_chat_config_active ON chat_system_config(is_active);

-- Insert default chat system configurations
INSERT INTO chat_system_config (organization_id, config_key, config_value, description) 
SELECT 
    o.id,
    'daily_message_limit',
    '5000'::jsonb,
    'Maximum number of messages a user can send per day'
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM chat_system_config 
    WHERE organization_id = o.id AND config_key = 'daily_message_limit'
);

INSERT INTO chat_system_config (organization_id, config_key, config_value, description) 
SELECT 
    o.id,
    'chat_enabled',
    'true'::jsonb,
    'Whether chat functionality is enabled for the organization'
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM chat_system_config 
    WHERE organization_id = o.id AND config_key = 'chat_enabled'
);

INSERT INTO chat_system_config (organization_id, config_key, config_value, description) 
SELECT 
    o.id,
    'max_conversations_per_user',
    '100'::jsonb,
    'Maximum number of active conversations per user'
FROM organizations o
WHERE NOT EXISTS (
    SELECT 1 FROM chat_system_config 
    WHERE organization_id = o.id AND config_key = 'max_conversations_per_user'
);

-- Comments for documentation
COMMENT ON TABLE chat_conversations IS 'Stores chat conversations between users and agents';
COMMENT ON TABLE chat_messages IS 'Stores individual messages within conversations';
COMMENT ON TABLE user_daily_limits IS 'Tracks daily message limits for users';
COMMENT ON TABLE agent_activity_logs IS 'Logs all agent-related activities for monitoring and analytics';
COMMENT ON TABLE chat_system_config IS 'Configuration settings for chat system per organization';

COMMENT ON COLUMN chat_conversations.status IS 'Conversation status: active, archived, deleted';
COMMENT ON COLUMN chat_messages.message_type IS 'Type of message: text, image, file, system';
COMMENT ON COLUMN chat_messages.role IS 'Who sent the message: user, agent, system';
COMMENT ON COLUMN user_daily_limits.daily_limit IS 'Maximum messages allowed per day for this user';
COMMENT ON COLUMN agent_activity_logs.activity_type IS 'Type of activity being logged';
COMMENT ON COLUMN agent_activity_logs.activity_data IS 'Additional data about the activity in JSON format';

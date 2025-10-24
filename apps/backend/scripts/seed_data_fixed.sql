-- Seed Data for AIM Testing (Schema-Correct Version)
-- Run with: export PGPASSWORD=postgres && psql -h localhost -U postgres -d identity -f scripts/seed_data_fixed.sql

-- Create test organization
INSERT INTO organizations (id, name, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'Test Organization', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test users with different roles (provider and provider_id are required)
INSERT INTO users (id, organization_id, email, name, role, provider, provider_id, created_at, updated_at)
VALUES
  ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   'admin@aim.test', 'Admin User', 'admin', 'test', 'admin-test-001', NOW(), NOW()),

  ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111',
   'manager@aim.test', 'Manager User', 'manager', 'test', 'manager-test-001', NOW(), NOW()),

  ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111',
   'member@aim.test', 'Member User', 'member', 'test', 'member-test-001', NOW(), NOW()),

  ('55555555-5555-5555-5555-555555555555', '11111111-1111-1111-1111-111111111111',
   'viewer@aim.test', 'Viewer User', 'viewer', 'test', 'viewer-test-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test agents (check actual schema for required fields)
INSERT INTO agents (id, organization_id, name, agent_type, description, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111',
   'Test AI Agent', 'ai_agent', 'A test AI agent for demonstration', NOW(), NOW()),

  ('77777777-7777-7777-7777-777777777777', '11111111-1111-1111-1111-111111111111',
   'Test MCP Server Agent', 'mcp_server', 'A test MCP server agent for demonstration', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Create test trust scores (check actual schema)
INSERT INTO trust_scores (agent_id, score, factors, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666666', 85.5, '{"verification": 30, "security": 20, "community": 15, "uptime": 15, "incidents": 10, "compliance": 5, "activity": 3, "verified_date": 2}'::jsonb, NOW(), NOW()),

  ('77777777-7777-7777-7777-777777777777', 92.0, '{"verification": 30, "security": 20, "community": 15, "uptime": 15, "incidents": 10, "compliance": 5, "activity": 3, "verified_date": 2}'::jsonb, NOW(), NOW())
ON CONFLICT (agent_id) DO NOTHING;

SELECT '✅ Seed data inserted successfully' as status;

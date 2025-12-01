/**
 * Complete AIM API Documentation
 *
 * This file contains the complete documentation for all 160 API endpoints.
 * Organized by functional category with full request/response schemas.
 *
 * Silicon Valley Standard: Complete, accurate, executable API docs.
 */

export interface APIEndpoint {
  method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  path: string;
  description: string;
  summary: string;
  auth: string;
  requestSchema?: {
    type: string;
    properties: Record<
      string,
      { type: string; description: string; required?: boolean; example?: any }
    >;
  };
  responseSchema?: {
    type: string;
    properties: Record<
      string,
      { type: string; description: string; example?: any }
    >;
  };
  example: string;
  requiresAuth: boolean;
  tags: string[];
  roleRequired?: string;
}

export interface EndpointCategory {
  category: string;
  description: string;
  icon: string;
  endpoints: APIEndpoint[];
}

export const apiDocumentation: EndpointCategory[] = [
  {
    category: "Authentication & Authorization",
    description:
      "User authentication, JWT tokens, password management, and session control",
    icon: "Lock",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/public/login",
        description:
          "Authenticate user with email and password. Returns JWT access token and refresh token.",
        summary: "Login with email/password",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["auth", "public"],
        requestSchema: {
          type: "object",
          properties: {
            email: {
              type: "string",
              description: "User email address",
              required: true,
              example: "admin@example.com",
            },
            password: {
              type: "string",
              description: "User password",
              required: true,
              example: "SecurePassword123!",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            token: {
              type: "string",
              description: "JWT access token",
              example: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
            },
            refreshToken: {
              type: "string",
              description: "Refresh token for obtaining new access tokens",
            },
            user: { type: "object", description: "User profile information" },
          },
        },
        example: `{
  "email": "admin@example.com",
  "password": "SecurePassword123!"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/auth/login/local",
        description:
          "Alternative local authentication endpoint. Same as /public/login.",
        summary: "Local login (alternative)",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["auth", "public"],
        requestSchema: {
          type: "object",
          properties: {
            email: {
              type: "string",
              description: "User email",
              required: true,
            },
            password: {
              type: "string",
              description: "User password",
              required: true,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            token: { type: "string", description: "JWT token" },
            user: { type: "object", description: "User data" },
          },
        },
        example: `{
  "email": "user@company.com",
  "password": "MyPassword123"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/auth/logout",
        description:
          "Invalidate current session and JWT token. Clears authentication state.",
        summary: "Logout current session",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["auth"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/auth/refresh",
        description:
          "Obtain new access token using refresh token. Implements token rotation for security.",
        summary: "Refresh access token",
        auth: "Refresh Token",
        requiresAuth: true,
        tags: ["auth"],
        requestSchema: {
          type: "object",
          properties: {
            refreshToken: {
              type: "string",
              description: "Valid refresh token",
              required: true,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            token: { type: "string", description: "New JWT access token" },
            refreshToken: {
              type: "string",
              description: "New refresh token (token rotation)",
            },
          },
        },
        example: `{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}`,
      },
      {
        method: "GET",
        path: "/api/v1/auth/me",
        description:
          "Get current authenticated user profile. Returns user info, role, and organization.",
        summary: "Get current user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["auth", "user"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "User ID (UUID)" },
            email: { type: "string", description: "User email" },
            role: {
              type: "string",
              description: "User role (admin, manager, member, viewer)",
            },
            organizationId: { type: "string", description: "Organization ID" },
            isApproved: {
              type: "boolean",
              description: "User approval status",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/auth/sdk/recover",
        description:
          "Recover revoked SDK tokens. Allows zero-downtime token rotation by recovering previously revoked tokens.",
        summary: "Recover revoked SDK token",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["auth", "sdk", "recovery"],
        requestSchema: {
          type: "object",
          properties: {
            token: {
              type: "string",
              description: "Revoked token to recover",
              required: true,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Recovery success status" },
            token: { type: "string", description: "Recovered token" },
          },
        },
        example: `{
  "token": "aim_sdk_xxxxxxxxxxxxx"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/organizations/current",
        description:
          "Get current user's organization details. Returns organization name, settings, and metadata.",
        summary: "Get current organization",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["auth", "organization"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Organization ID" },
            name: { type: "string", description: "Organization name" },
            settings: { type: "object", description: "Organization settings" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/auth/change-password",
        description:
          "Change password for authenticated user. Requires old password verification.",
        summary: "Change password",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["auth", "security"],
        requestSchema: {
          type: "object",
          properties: {
            oldPassword: {
              type: "string",
              description: "Current password",
              required: true,
            },
            newPassword: {
              type: "string",
              description: "New password (min 8 chars)",
              required: true,
            },
          },
        },
        example: `{
  "oldPassword": "OldPassword123!",
  "newPassword": "NewSecurePassword456!"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/public/forgot-password",
        description:
          "Request password reset email. Sends reset link to user email.",
        summary: "Request password reset",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["auth", "public", "password"],
        requestSchema: {
          type: "object",
          properties: {
            email: {
              type: "string",
              description: "User email address",
              required: true,
            },
          },
        },
        example: `{
  "email": "user@company.com"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/public/reset-password",
        description:
          "Reset password using token from email. Completes password reset flow.",
        summary: "Reset password with token",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["auth", "public", "password"],
        requestSchema: {
          type: "object",
          properties: {
            token: {
              type: "string",
              description: "Reset token from email",
              required: true,
            },
            newPassword: {
              type: "string",
              description: "New password",
              required: true,
            },
          },
        },
        example: `{
  "token": "abc123-reset-token-def456",
  "newPassword": "NewPassword789!"
}`,
      },
    ],
  },

  {
    category: "User Registration & Onboarding",
    description:
      "Self-service user registration, approval workflows, and access requests",
    icon: "User",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/public/register",
        description:
          "Register new user account. Creates pending user awaiting admin approval.",
        summary: "Register new user",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["registration", "public"],
        requestSchema: {
          type: "object",
          properties: {
            email: {
              type: "string",
              description: "User email",
              required: true,
            },
            password: {
              type: "string",
              description: "User password (min 8 chars)",
              required: true,
            },
            name: { type: "string", description: "Full name", required: true },
            organization: {
              type: "string",
              description: "Organization name (optional)",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            requestId: {
              type: "string",
              description: "Registration request ID",
            },
            status: {
              type: "string",
              description: "Registration status (pending)",
            },
            message: { type: "string", description: "Next steps message" },
          },
        },
        example: `{
  "email": "newuser@company.com",
  "password": "SecurePass123!",
  "name": "John Doe",
  "organization": "Acme Corp"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/public/register/:requestId/status",
        description:
          "Check registration request status. Polls for admin approval.",
        summary: "Check registration status",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["registration", "public"],
        responseSchema: {
          type: "object",
          properties: {
            status: {
              type: "string",
              description: "Status (pending, approved, rejected)",
            },
            requestId: {
              type: "string",
              description: "Registration request ID",
            },
            message: { type: "string", description: "Status message" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/public/request-access",
        description:
          "Request platform access without password. Admin creates account and sends credentials.",
        summary: "Request platform access",
        auth: "None (Public)",
        requiresAuth: false,
        tags: ["registration", "public"],
        requestSchema: {
          type: "object",
          properties: {
            email: {
              type: "string",
              description: "User email",
              required: true,
            },
            name: { type: "string", description: "Full name", required: true },
            reason: { type: "string", description: "Access reason (optional)" },
          },
        },
        example: `{
  "email": "contractor@external.com",
  "name": "Jane Smith",
  "reason": "Need access for Q4 audit"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/public/change-password",
        description:
          "Forced password change on first login. Security policy requirement.",
        summary: "Forced password change",
        auth: "Temporary Token",
        requiresAuth: false,
        tags: ["auth", "security"],
        requestSchema: {
          type: "object",
          properties: {
            token: {
              type: "string",
              description: "Temporary login token",
              required: true,
            },
            newPassword: {
              type: "string",
              description: "New password",
              required: true,
            },
          },
        },
        example: `{
  "token": "temp-token-xyz",
  "newPassword": "MyNewPassword123!"
}`,
      },
    ],
  },

  {
    category: "Agent Lifecycle Management",
    description:
      "Complete agent CRUD operations, verification, suspension, and credential management",
    icon: "Bot",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/public/agents/register",
        description:
          "ONE-LINE agent registration. Creates agent with Ed25519 keypair automatically.",
        summary: "Register new agent (one-line)",
        auth: "None (Public) or Bearer Token",
        requiresAuth: false,
        tags: ["agents", "public"],
        requestSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Agent name",
              required: true,
              example: "my-assistant",
            },
            type: {
              type: "string",
              description: "Agent type (ai_agent, mcp_server, automation_bot)",
              required: true,
            },
            description: { type: "string", description: "Agent description" },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            agentId: { type: "string", description: "New agent ID (UUID)" },
            publicKey: {
              type: "string",
              description: "Ed25519 public key (base64)",
            },
            privateKey: {
              type: "string",
              description: "Ed25519 private key (base64) - SAVE THIS!",
            },
            trustScore: {
              type: "number",
              description: "Initial trust score (50.0)",
            },
          },
        },
        example: `{
  "name": "customer-support-agent",
  "type": "ai_agent",
  "description": "AI agent for customer support"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/agents",
        description:
          "List all agents in organization. Supports filtering and pagination.",
        summary: "List agents",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents"],
        roleRequired: "viewer",
        responseSchema: {
          type: "object",
          properties: {
            agents: { type: "array", description: "Array of agent objects" },
            total: { type: "number", description: "Total agent count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/agents",
        description:
          "Create new agent. Auto-generates Ed25519 keypair and assigns initial trust score.",
        summary: "Create agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "Agent name", required: true },
            type: { type: "string", description: "Agent type", required: true },
            description: { type: "string", description: "Agent description" },
            tags: { type: "array", description: "Agent tags (optional)" },
          },
        },
        example: `{
  "name": "data-processor",
  "type": "automation_bot",
  "description": "Automated data processing agent",
  "tags": ["production", "critical"]
}`,
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id",
        description:
          "Get agent details by ID. Returns full agent profile with trust score.",
        summary: "Get agent by ID",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents"],
        example: "No request body required",
      },
      {
        method: "PUT",
        path: "/api/v1/agents/:id",
        description:
          "Update agent information. Can modify name, description, and tags.",
        summary: "Update agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "New agent name" },
            description: { type: "string", description: "New description" },
            tags: { type: "array", description: "Updated tags" },
          },
        },
        example: `{
  "name": "updated-agent-name",
  "description": "Updated description",
  "tags": ["updated", "tags"]
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/agents/:id",
        description:
          "Permanently delete agent. Requires manager role. Cannot be undone.",
        summary: "Delete agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["agents"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/verify",
        description: "Manually verify agent identity. Manager-only operation.",
        summary: "Verify agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["agents", "verification"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/suspend",
        description:
          "Suspend agent operations. Temporarily disables all agent actions.",
        summary: "Suspend agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["agents", "lifecycle"],
        requestSchema: {
          type: "object",
          properties: {
            reason: {
              type: "string",
              description: "Suspension reason",
              required: true,
            },
          },
        },
        example: `{
  "reason": "Security investigation"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/reactivate",
        description: "Reactivate suspended agent. Restores agent operations.",
        summary: "Reactivate agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["agents", "lifecycle"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/rotate-credentials",
        description:
          "Rotate agent Ed25519 keypair. Generates new keys and invalidates old ones.",
        summary: "Rotate credentials",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents", "security"],
        responseSchema: {
          type: "object",
          properties: {
            publicKey: { type: "string", description: "New public key" },
            privateKey: {
              type: "string",
              description: "New private key - SAVE THIS!",
            },
            rotatedAt: { type: "string", description: "Rotation timestamp" },
          },
        },
        example: "{}",
      },
      {
        method: "PUT",
        path: "/api/v1/agents/:id/keys",
        description:
          "Register agent's Ed25519 public key from SDK. Allows SDK-generated keys to be registered with AIM backend.",
        summary: "Register SDK key",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents", "sdk", "credentials"],
        requestSchema: {
          type: "object",
          properties: {
            publicKey: {
              type: "string",
              description: "Ed25519 public key (base64)",
              required: true,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Registration success" },
            publicKey: { type: "string", description: "Registered public key" },
          },
        },
        example: `{
  "publicKey": "base64_encoded_public_key"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id/credentials",
        description:
          "Get raw Ed25519 public/private keys. For manual integration.",
        summary: "Get agent credentials",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "credentials"],
        responseSchema: {
          type: "object",
          properties: {
            publicKey: {
              type: "string",
              description: "Ed25519 public key (base64)",
            },
            privateKey: {
              type: "string",
              description: "Ed25519 private key (base64)",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id/sdk",
        description:
          "Download pre-configured SDK with embedded credentials. Python/Node.js/Go.",
        summary: "Download agent SDK",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "sdk"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id/key-vault",
        description:
          "Get agent key vault info: public key, expiration, rotation status.",
        summary: "Get key vault info",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "security"],
        responseSchema: {
          type: "object",
          properties: {
            publicKey: { type: "string", description: "Current public key" },
            expiresAt: { type: "string", description: "Key expiration date" },
            lastRotated: { type: "string", description: "Last rotation date" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id/audit-logs",
        description: "Get audit logs for specific agent. Supports pagination.",
        summary: "Get agent audit logs",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "audit"],
        responseSchema: {
          type: "object",
          properties: {
            logs: { type: "array", description: "Audit log entries" },
            total: { type: "number", description: "Total logs count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/verify-action",
        description:
          "CORE: Runtime action verification. Verifies agent can perform action.",
        summary: "Verify agent action",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "runtime", "core"],
        requestSchema: {
          type: "object",
          properties: {
            action: {
              type: "string",
              description: "Action name",
              required: true,
            },
            resourceType: { type: "string", description: "Resource type" },
            context: { type: "object", description: "Action context" },
          },
        },
        example: `{
  "action": "send_email",
  "resourceType": "email",
  "context": {
    "recipient": "user@example.com",
    "subject": "Test Email"
  }
}`,
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/log-action/:audit_id",
        description:
          "CORE: Log action result. Records verification outcome for audit.",
        summary: "Log action result",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "runtime", "core"],
        requestSchema: {
          type: "object",
          properties: {
            status: {
              type: "string",
              description: "Action status (success, failed)",
              required: true,
            },
            result: { type: "object", description: "Action result data" },
          },
        },
        example: `{
  "status": "success",
  "result": {
    "emailSent": true,
    "messageId": "msg_123"
  }
}`,
      },
    ],
  },

  {
    category: "Agent-MCP Relationships",
    description:
      "Manage which MCP servers each agent talks to, auto-detection, and mapping",
    icon: "Plug",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/agents/:id/mcp-servers",
        description:
          "Get all MCP servers this agent talks to. Returns relationship list.",
        summary: "Get agent MCP servers",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["agents", "mcp", "relationships"],
        responseSchema: {
          type: "object",
          properties: {
            mcpServers: {
              type: "array",
              description: "MCP servers agent communicates with",
            },
            count: { type: "number", description: "Total MCP server count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "PUT",
        path: "/api/v1/agents/:id/mcp-servers",
        description:
          'Add MCP servers to agent (bulk). Creates "talks_to" relationships.',
        summary: "Add MCP servers to agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents", "mcp", "relationships"],
        requestSchema: {
          type: "object",
          properties: {
            mcpServerIds: {
              type: "array",
              description: "Array of MCP server IDs",
              required: true,
            },
          },
        },
        example: `{
  "mcpServerIds": [
    "uuid-mcp-1",
    "uuid-mcp-2",
    "uuid-mcp-3"
  ]
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/agents/:id/mcp-servers/:mcp_id",
        description:
          'Remove single MCP server from agent. Deletes "talks_to" relationship.',
        summary: "Remove MCP server from agent",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents", "mcp", "relationships"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/mcp-servers/detect",
        description:
          "Auto-detect MCP servers from agent config. Parses Claude Desktop config.",
        summary: "Auto-detect MCP servers",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["agents", "mcp", "detection"],
        requestSchema: {
          type: "object",
          properties: {
            configPath: {
              type: "string",
              description: "Path to Claude Desktop config",
            },
            configData: { type: "object", description: "Config JSON data" },
          },
        },
        example: `{
  "configPath": "~/Library/Application Support/Claude/claude_desktop_config.json"
}`,
      },
    ],
  },

  {
    category: "Trust Score Management",
    description:
      "ML-powered 8-factor trust scoring, history tracking, and manual adjustments",
    icon: "Shield",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/agents/:id/trust-score",
        description:
          "Get current agent trust score (0-100). ML algorithm based on 8 factors.",
        summary: "Get trust score",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["trust", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            trustScore: {
              type: "number",
              description: "Current score (0-100)",
            },
            lastCalculated: {
              type: "string",
              description: "Last calculation time",
            },
            factors: { type: "object", description: "Contributing factors" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/agents/:id/trust-score/history",
        description:
          "Get trust score history over time. Returns time-series data.",
        summary: "Get trust score history",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["trust", "agents", "history"],
        responseSchema: {
          type: "object",
          properties: {
            history: {
              type: "array",
              description: "Trust score history entries",
            },
            count: { type: "number", description: "Total entries" },
          },
        },
        example: "No request body required",
      },
      {
        method: "PUT",
        path: "/api/v1/agents/:id/trust-score",
        description:
          "Manually update trust score. Admin-only. Creates audit log entry.",
        summary: "Update trust score (manual)",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["trust", "agents", "admin"],
        requestSchema: {
          type: "object",
          properties: {
            trustScore: {
              type: "number",
              description: "New score (0-100)",
              required: true,
            },
            reason: {
              type: "string",
              description: "Change reason",
              required: true,
            },
          },
        },
        example: `{
  "trustScore": 85.5,
  "reason": "Manual adjustment after security review"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/agents/:id/trust-score/recalculate",
        description: "Trigger trust score recalculation. Re-runs ML algorithm.",
        summary: "Recalculate trust score",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["trust", "agents"],
        example: "{}",
      },
      {
        method: "GET",
        path: "/api/v1/trust-score/agents/:id",
        description:
          "Get trust score (alternative endpoint). Same as /agents/:id/trust-score.",
        summary: "Get trust score (alt)",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["trust"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/trust-score/agents/:id/breakdown",
        description:
          "Detailed trust score breakdown. Shows 8 factors with weights and contributions.",
        summary: "Get trust score breakdown",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["trust", "analytics"],
        responseSchema: {
          type: "object",
          properties: {
            trustScore: { type: "number", description: "Overall score" },
            factors: { type: "object", description: "Breakdown by factor" },
            weights: { type: "object", description: "Factor weights" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/trust-score/agents/:id/history",
        description: "Trust score history (alternative endpoint).",
        summary: "Get history (alt)",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["trust", "history"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/trust-score/calculate/:id",
        description: "Calculate trust score for agent. Manager-only.",
        summary: "Calculate trust score",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["trust"],
        example: "{}",
      },
    ],
  },

  {
    category: "MCP Server Management",
    description:
      "Register, manage, and verify MCP servers with cryptographic authentication",
    icon: "Server",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/mcp-servers",
        description:
          "List all MCP servers in organization. Supports filtering by status.",
        summary: "List MCP servers",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp"],
        responseSchema: {
          type: "object",
          properties: {
            mcpServers: {
              type: "array",
              description: "Array of MCP server objects",
            },
            total: { type: "number", description: "Total count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers",
        description:
          "Register new MCP server. Creates server with Ed25519 keypair.",
        summary: "Register MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["mcp"],
        requestSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Server name",
              required: true,
            },
            url: { type: "string", description: "Server URL", required: true },
            description: { type: "string", description: "Server description" },
          },
        },
        example: `{
  "name": "filesystem-mcp",
  "url": "http://localhost:3000",
  "description": "Filesystem MCP server"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id",
        description: "Get MCP server details by ID.",
        summary: "Get MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp"],
        example: "No request body required",
      },
      {
        method: "PUT",
        path: "/api/v1/mcp-servers/:id",
        description: "Update MCP server info.",
        summary: "Update MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["mcp"],
        example: `{
  "name": "updated-server-name",
  "url": "http://new-url.com"
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/mcp-servers/:id",
        description: "Delete MCP server. Manager-only.",
        summary: "Delete MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["mcp"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/verify",
        description: "Verify MCP server cryptographically.",
        summary: "Verify MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["mcp", "verification"],
        example: "{}",
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/agents",
        description: "Get all agents talking to this MCP server.",
        summary: "Get MCP server agents",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp", "relationships"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/rotate-credentials",
        description: "Rotate MCP server credentials.",
        summary: "Rotate MCP credentials",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["mcp", "security"],
        example: "{}",
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/health",
        description: "Check MCP server health status.",
        summary: "Check MCP health",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp", "monitoring"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/logs",
        description: "Get MCP server logs.",
        summary: "Get MCP logs",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp", "monitoring"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/suspend",
        description: "Suspend MCP server.",
        summary: "Suspend MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["mcp", "lifecycle"],
        example: `{
  "reason": "Security review"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/reactivate",
        description: "Reactivate suspended MCP server.",
        summary: "Reactivate MCP server",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["mcp", "lifecycle"],
        example: "{}",
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/credentials",
        description: "Get MCP server credentials.",
        summary: "Get MCP credentials",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["mcp", "credentials"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/attest",
        description:
          "Submit cryptographic attestation that agent has verified MCP server capabilities. Zero-effort verification using Ed25519 signatures.",
        summary: "Attest MCP server",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["mcp", "attestation", "security", "verification"],
        requestSchema: {
          type: "object",
          properties: {
            capabilities_verified: {
              type: "array",
              description: "List of MCP capabilities agent verified",
              required: true,
              example: ["read_file", "write_file", "execute_command"],
            },
            attestation_method: {
              type: "string",
              description: "How attestation was performed",
              example: "runtime_detection",
            },
            metadata: {
              type: "object",
              description: "Additional attestation metadata",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            attestationId: {
              type: "string",
              description: "Attestation record ID",
            },
            status: {
              type: "string",
              description: "Attestation status (verified)",
            },
            timestamp: {
              type: "string",
              description: "Attestation timestamp",
            },
          },
        },
        example: `{
  "capabilities_verified": ["read_file", "write_file"],
  "attestation_method": "runtime_detection",
  "metadata": {
    "sdk_version": "1.0.0"
  }
}`,
      },
      {
        method: "POST",
        path: "/api/v1/mcp-servers/:id/manual-attest",
        description:
          "Manually attest MCP server capabilities. For non-SDK users or manual verification workflows.",
        summary: "Manual MCP attestation",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["mcp", "attestation", "manual"],
        requestSchema: {
          type: "object",
          properties: {
            capabilities_verified: {
              type: "array",
              description: "List of capabilities verified",
              required: true,
            },
            notes: {
              type: "string",
              description: "Attestation notes",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            attestationId: { type: "string", description: "Attestation ID" },
            status: { type: "string", description: "Attestation status" },
          },
        },
        example: `{
  "capabilities_verified": ["read_file", "write_file"],
  "notes": "Manually verified capabilities via testing"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/attestations",
        description:
          "Get all agent attestations for this MCP server. Shows which agents have verified this MCP.",
        summary: "Get MCP attestations",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["mcp", "attestation"],
        responseSchema: {
          type: "object",
          properties: {
            attestations: {
              type: "array",
              description: "List of attestation records",
            },
            count: { type: "number", description: "Total attestations" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/mcp-servers/:id/agents",
        description:
          "Get all agents connected to this MCP server (via attestation). Shows agent-MCP relationships.",
        summary: "Get connected agents",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["mcp", "relationships", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            agents: {
              type: "array",
              description: "List of connected agents",
            },
            count: { type: "number", description: "Total connected agents" },
          },
        },
        example: "No request body required",
      },
    ],
  },

  {
    category: "API Key Management",
    description:
      "Create, manage, and revoke SHA-256 hashed API keys with expiration",
    icon: "Key",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/api-keys",
        description: "List all API keys for organization.",
        summary: "List API keys",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["api-keys"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/api-keys",
        description: "Create new API key. SHA-256 hashed before storage.",
        summary: "Create API key",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["api-keys"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "Key name", required: true },
            expiresAt: {
              type: "string",
              description: "Expiration date (optional)",
            },
            scopes: {
              type: "array",
              description: "Permission scopes (optional)",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            apiKey: {
              type: "string",
              description: "Plain-text API key - SAVE THIS!",
            },
            keyId: { type: "string", description: "Key ID" },
            createdAt: { type: "string", description: "Creation timestamp" },
          },
        },
        example: `{
  "name": "production-api-key",
  "expiresAt": "2025-12-31T23:59:59Z",
  "scopes": ["agents:read", "agents:write"]
}`,
      },
      {
        method: "PATCH",
        path: "/api/v1/api-keys/:id/disable",
        description:
          "Disable API key without deleting. Allows temporary suspension of API key access.",
        summary: "Disable API key",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["api-keys", "security"],
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Disable success" },
            keyId: { type: "string", description: "Disabled key ID" },
          },
        },
        example: "No request body required",
      },
      {
        method: "DELETE",
        path: "/api/v1/api-keys/:id",
        description: "Revoke API key. Immediate invalidation.",
        summary: "Revoke API key",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["api-keys"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/api-keys/:id/usage",
        description: "Get API key usage statistics.",
        summary: "Get key usage stats",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["api-keys", "analytics"],
        example: "No request body required",
      },
    ],
  },

  {
    category: "Verification Events",
    description:
      "Agent verification history, real-time events, and verification statistics",
    icon: "Activity",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/verification-events",
        description:
          "Get all verification events. Supports filtering and pagination.",
        summary: "List verification events",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "events"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/recent",
        description: "Get recent verification events (last 24 hours).",
        summary: "Recent verification events",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "events"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/statistics",
        description:
          "Get verification statistics: success rate, failure rate, total count.",
        summary: "Verification statistics",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "analytics"],
        responseSchema: {
          type: "object",
          properties: {
            total: { type: "number", description: "Total verifications" },
            successRate: {
              type: "number",
              description: "Success rate percentage",
            },
            failureRate: {
              type: "number",
              description: "Failure rate percentage",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/:id",
        description: "Get specific verification event details.",
        summary: "Get verification event",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "events"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/agent/:agentId",
        description: "Get verification events for specific agent.",
        summary: "Agent verification events",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["verification", "agents"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/timeframe",
        description: "Get verification events within timeframe.",
        summary: "Events by timeframe",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "analytics"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/verification-events",
        description: "Create verification event manually.",
        summary: "Create verification event",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["verification", "admin"],
        example: `{
  "agentId": "uuid-agent-1",
  "status": "success",
  "method": "cryptographic"
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/verification-events/:id",
        description: "Delete verification event. Admin-only.",
        summary: "Delete verification event",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["verification", "admin"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/failed",
        description: "Get all failed verification events.",
        summary: "Failed verifications",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "security"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/successful",
        description: "Get all successful verification events.",
        summary: "Successful verifications",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/stats",
        description:
          "Get aggregated verification statistics. Returns overall verification metrics and trends.",
        summary: "Verification stats",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verification", "analytics"],
        responseSchema: {
          type: "object",
          properties: {
            total_verifications: {
              type: "number",
              description: "Total verification count",
            },
            success_rate: {
              type: "number",
              description: "Success rate percentage",
            },
            avg_response_time: {
              type: "number",
              description: "Average response time (ms)",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/agent/:id",
        description:
          "Get all verification events for specific agent. Shows agent's verification history.",
        summary: "Agent verification events",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["verification", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            events: {
              type: "array",
              description: "Verification events for agent",
            },
            count: { type: "number", description: "Total events" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/verification-events/mcp/:id",
        description:
          "Get all verification events for specific MCP server. Shows MCP's verification history.",
        summary: "MCP verification events",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["verification", "mcp"],
        responseSchema: {
          type: "object",
          properties: {
            events: {
              type: "array",
              description: "Verification events for MCP",
            },
            count: { type: "number", description: "Total events" },
          },
        },
        example: "No request body required",
      },
    ],
  },

  // Webhooks category hidden per user requirements

  {
    category: "Tags",
    description: "Tag management for categorizing agents and MCP servers",
    icon: "Tag",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/tags",
        description: "List all tags in organization.",
        summary: "List tags",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["tags"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/tags",
        description: "Create new tag.",
        summary: "Create tag",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["tags"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "Tag name", required: true },
            color: { type: "string", description: "Tag color (hex)" },
          },
        },
        example: `{
  "name": "production",
  "color": "#FF5733"
}`,
      },
      {
        method: "PUT",
        path: "/api/v1/tags/:id",
        description: "Update tag.",
        summary: "Update tag",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["tags"],
        example: `{
  "name": "updated-tag-name",
  "color": "#00FF00"
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/tags/:id",
        description: "Delete tag.",
        summary: "Delete tag",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["tags"],
        example: "No request body required",
      },
    ],
  },

  {
    category: "Detection & Auto-Discovery",
    description:
      "Automatic detection of agents and MCP servers from config files",
    icon: "Search",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/detection/scan",
        description: "Scan for agents and MCP servers.",
        summary: "Scan for entities",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["detection"],
        example: `{
  "configPath": "~/Library/Application Support/Claude/claude_desktop_config.json"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/detection/history",
        description: "Get detection scan history.",
        summary: "Detection history",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/detection/auto-register",
        description: "Auto-register detected entities.",
        summary: "Auto-register entities",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["detection"],
        example: `{
  "entities": [
    {"name": "agent-1", "type": "ai_agent"},
    {"name": "mcp-server-1", "type": "mcp_server"}
  ]
}`,
      },
      {
        method: "GET",
        path: "/api/v1/detection/unregistered",
        description: "Get detected but unregistered entities.",
        summary: "Unregistered entities",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/detection/agents/:id/report",
        description:
          "Report MCP detection from SDK agent. Allows agents to report detected MCP servers and integration status.",
        summary: "SDK: Report MCP detection",
        auth: "Ed25519 (Agent Signature) or Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection", "sdk", "mcp"],
        requestSchema: {
          type: "object",
          properties: {
            mcp_servers: {
              type: "array",
              description: "Detected MCP servers",
              required: true,
            },
            detection_method: {
              type: "string",
              description: "How MCPs were detected",
              example: "config_file_scan",
            },
            metadata: {
              type: "object",
              description: "Detection metadata",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            reportId: { type: "string", description: "Detection report ID" },
            detected_count: { type: "number", description: "Number of MCPs detected" },
          },
        },
        example: `{
  "mcp_servers": [
    {"name": "filesystem", "url": "http://localhost:3000"},
    {"name": "database", "url": "http://localhost:3001"}
  ],
  "detection_method": "config_file_scan"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/detection/agents/:id/status",
        description:
          "Get MCP detection status for agent. Shows which MCPs agent has detected and integration status.",
        summary: "Get agent detection status",
        auth: "Ed25519 (Agent Signature) or Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            detection_count: {
              type: "number",
              description: "Total MCPs detected",
            },
            last_detection: {
              type: "string",
              description: "Last detection timestamp",
            },
            detected_mcps: {
              type: "array",
              description: "List of detected MCP servers",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/detection/agents/:id/capabilities/report",
        description:
          "Report agent's detected capabilities. Allows agents to self-report their detected capabilities.",
        summary: "Report agent capabilities",
        auth: "Ed25519 (Agent Signature) or Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection", "capabilities"],
        requestSchema: {
          type: "object",
          properties: {
            capabilities: {
              type: "array",
              description: "Detected capabilities",
              required: true,
            },
            detection_method: {
              type: "string",
              description: "How capabilities were detected",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            reportId: { type: "string", description: "Capability report ID" },
            capabilities_count: {
              type: "number",
              description: "Number of capabilities detected",
            },
          },
        },
        example: `{
  "capabilities": ["read_file", "write_file", "execute_command"],
  "detection_method": "runtime_analysis"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/detection/agents/:id/capabilities/latest",
        description:
          "Get latest capability detection report for agent. Shows most recent capability detection results.",
        summary: "Get latest capability report",
        auth: "Ed25519 (Agent Signature) or Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["detection", "capabilities"],
        responseSchema: {
          type: "object",
          properties: {
            capabilities: {
              type: "array",
              description: "Detected capabilities",
            },
            detected_at: {
              type: "string",
              description: "Detection timestamp",
            },
            detection_method: {
              type: "string",
              description: "Detection method used",
            },
          },
        },
        example: "No request body required",
      },
    ],
  },

  {
    category: "SDK & Integration",
    description: "SDK downloads, tokens, and integration helpers",
    icon: "Download",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/sdk/download",
        description: "Download SDK (Python/Node.js/Go).",
        summary: "Download SDK",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/sdk-tokens",
        description: "List SDK tokens.",
        summary: "List SDK tokens",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/sdk-tokens",
        description: "Create SDK token.",
        summary: "Create SDK token",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["sdk"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "Token name", required: true },
            expiresAt: { type: "string", description: "Expiration date" },
          },
        },
        example: `{
  "name": "dev-sdk-token",
  "expiresAt": "2025-12-31"
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/sdk-tokens/:id",
        description: "Revoke SDK token.",
        summary: "Revoke SDK token",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "member",
        tags: ["sdk"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/sdk/docs",
        description: "Get SDK documentation.",
        summary: "SDK documentation",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/users/me/sdk-tokens",
        description:
          "List all SDK tokens for current user. Shows active and revoked tokens with usage statistics.",
        summary: "List user SDK tokens",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk", "tokens"],
        responseSchema: {
          type: "object",
          properties: {
            tokens: {
              type: "array",
              description: "SDK tokens list",
            },
            total: { type: "number", description: "Total token count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/users/me/sdk-tokens/count",
        description:
          "Get count of active SDK tokens for current user. Quick check for token management.",
        summary: "Get active token count",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk", "tokens"],
        responseSchema: {
          type: "object",
          properties: {
            active_count: {
              type: "number",
              description: "Number of active tokens",
            },
            total_count: { type: "number", description: "Total tokens (including revoked)" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/users/me/sdk-tokens/:id/revoke",
        description:
          "Revoke specific SDK token. Immediately invalidates token for security.",
        summary: "Revoke SDK token",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk", "tokens", "security"],
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Revocation success" },
            tokenId: { type: "string", description: "Revoked token ID" },
          },
        },
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/users/me/sdk-tokens/revoke-all",
        description:
          "Revoke all SDK tokens for current user. Emergency security action for compromised credentials.",
        summary: "Revoke all SDK tokens",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["sdk", "tokens", "security"],
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Revocation success" },
            revoked_count: {
              type: "number",
              description: "Number of tokens revoked",
            },
          },
        },
        example: "{}",
      },
    ],
  },

  {
    category: "Security & Alerts",
    description: "Security monitoring, threat detection, and alert management",
    icon: "AlertTriangle",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/admin/alerts",
        description: "Get all security alerts.",
        summary: "List alerts",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["alerts", "security"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/alerts/unacknowledged",
        description: "Get unacknowledged alerts.",
        summary: "Unacknowledged alerts",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["alerts"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/alerts/unacknowledged/count",
        description: "Get count of unacknowledged alerts.",
        summary: "Unack alert count",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["alerts"],
        responseSchema: {
          type: "object",
          properties: {
            count: { type: "number", description: "Number of unack alerts" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/alerts/:id/acknowledge",
        description: "Acknowledge alert.",
        summary: "Acknowledge alert",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["alerts"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/admin/alerts/:id/resolve",
        description:
          "Resolve acknowledged alert. Marks alert as resolved after remediation.",
        summary: "Resolve alert",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["alerts"],
        requestSchema: {
          type: "object",
          properties: {
            resolution_notes: {
              type: "string",
              description: "Notes about how alert was resolved",
            },
          },
        },
        example: `{
  "resolution_notes": "Updated security policy to prevent this issue"
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/admin/alerts/:id",
        description: "Delete alert.",
        summary: "Delete alert",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["alerts"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/security/dashboard",
        description: "Get security dashboard metrics.",
        summary: "Security dashboard",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["security", "analytics"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/security/alerts",
        description:
          "List security alerts with pagination support. Returns alerts with severity levels (critical, high, medium, low), status (unacknowledged, acknowledged, resolved), and detailed alert information.",
        summary: "List Security Alerts",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["security", "alerts"],
        responseSchema: {
          type: "object",
          properties: {
            alerts: {
              type: "array",
              description: "Array of alert objects",
            },
            pagination: {
              type: "object",
              description: "Pagination metadata (total, limit, offset)",
            },
            summary: {
              type: "object",
              description: "Alert summary (total, unacknowledged count)",
            },
          },
        },
        example:
          "?limit=20&offset=0 for paginated results (default limit is 20)",
      },
      {
        method: "GET",
        path: "/api/v1/security/threats",
        description: "Get active security threats.",
        summary: "Active threats",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["security"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/security/anomalies",
        description: "Get detected anomalies.",
        summary: "Detected anomalies",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["security"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/security/scan",
        description: "Trigger security scan.",
        summary: "Security scan",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["security"],
        example: "{}",
      },
      {
        method: "GET",
        path: "/api/v1/security/reports",
        description: "Get security reports.",
        summary: "Security reports",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["security", "reports"],
        example: "No request body required",
      },
    ],
  },

  {
    category: "Security Policies",
    description:
      "Define and enforce security policies across agents and MCP servers",
    icon: "ShieldCheck",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/admin/security-policies",
        description: "List all security policies.",
        summary: "List policies",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/security-policies",
        description: "Create security policy.",
        summary: "Create policy",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        requestSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Policy name",
              required: true,
            },
            rules: {
              type: "object",
              description: "Policy rules",
              required: true,
            },
            enabled: { type: "boolean", description: "Policy enabled" },
          },
        },
        example: `{
  "name": "Trust Score Minimum Policy",
  "rules": {
    "minTrustScore": 70,
    "action": "suspend"
  },
  "enabled": true
}`,
      },
      {
        method: "GET",
        path: "/api/v1/admin/security-policies/:id",
        description: "Get policy details.",
        summary: "Get policy",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        example: "No request body required",
      },
      {
        method: "PUT",
        path: "/api/v1/admin/security-policies/:id",
        description: "Update security policy.",
        summary: "Update policy",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        example: `{
  "enabled": false
}`,
      },
      {
        method: "DELETE",
        path: "/api/v1/admin/security-policies/:id",
        description: "Delete security policy.",
        summary: "Delete policy",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        example: "No request body required",
      },
      {
        method: "PATCH",
        path: "/api/v1/admin/security-policies/:id/toggle",
        description:
          "Toggle security policy enabled/disabled state. Quick enable/disable without full update.",
        summary: "Toggle policy",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Policy ID" },
            enabled: {
              type: "boolean",
              description: "New enabled state",
            },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/security-policies/:id/violations",
        description: "Get policy violations.",
        summary: "Policy violations",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["policies", "compliance"],
        example: "No request body required",
      },
    ],
  },

  {
    category: "User Management (Admin)",
    description:
      "Admin-only user management, role assignments, and access control",
    icon: "Users",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/admin/users",
        description: "List all users. Admin-only.",
        summary: "List users",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/users/pending",
        description: "Get pending user registrations.",
        summary: "Pending users",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/users/:id/approve",
        description: "Approve user registration.",
        summary: "Approve user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        requestSchema: {
          type: "object",
          properties: {
            role: {
              type: "string",
              description: "Assign role (admin, manager, member, viewer)",
              required: true,
            },
          },
        },
        example: `{
  "role": "member"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/admin/users/:id/reject",
        description: "Reject user registration.",
        summary: "Reject user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        requestSchema: {
          type: "object",
          properties: {
            reason: { type: "string", description: "Rejection reason" },
          },
        },
        example: `{
  "reason": "Invalid organization"
}`,
      },
      {
        method: "PUT",
        path: "/api/v1/admin/users/:id/role",
        description: "Update user role.",
        summary: "Update user role",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        requestSchema: {
          type: "object",
          properties: {
            role: { type: "string", description: "New role", required: true },
          },
        },
        example: `{
  "role": "manager"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/admin/users/:id/deactivate",
        description: "Deactivate user account (soft delete).",
        summary: "Deactivate user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/admin/users/:id/activate",
        description:
          "Activate previously deactivated user account. Restores user access.",
        summary: "Activate user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        example: "{}",
      },
      {
        method: "DELETE",
        path: "/api/v1/admin/users/:id",
        description: "Delete user permanently (hard delete).",
        summary: "Delete user",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/users/:id",
        description:
          "Get detailed user information. Shows user profile, role, organization, and activity.",
        summary: "Get user details",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "User ID" },
            email: { type: "string", description: "User email" },
            role: { type: "string", description: "User role" },
            organizationId: { type: "string", description: "Organization ID" },
            isApproved: { type: "boolean", description: "Approval status" },
            createdAt: { type: "string", description: "Account creation date" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/users/:id/reset-password",
        description:
          "Reset user password and send reset email. Admin-initiated password reset for user support.",
        summary: "Reset user password",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["admin", "users", "security"],
        responseSchema: {
          type: "object",
          properties: {
            success: { type: "boolean", description: "Reset success" },
            message: {
              type: "string",
              description: "Confirmation message",
            },
          },
        },
        example: "{}",
      },
    ],
  },

  {
    category: "Capability Requests",
    description: "Agent capability approval workflow (AIVF-style)",
    icon: "CheckSquare",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/admin/capability-requests",
        description: "List all capability requests.",
        summary: "List capability requests",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["capabilities", "admin"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/capability-requests/:id/approve",
        description: "Approve capability request.",
        summary: "Approve capability",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["capabilities"],
        example: "{}",
      },
      {
        method: "POST",
        path: "/api/v1/admin/capability-requests/:id/reject",
        description: "Reject capability request.",
        summary: "Reject capability",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["capabilities"],
        requestSchema: {
          type: "object",
          properties: {
            reason: { type: "string", description: "Rejection reason" },
          },
        },
        example: `{
  "reason": "Security concern"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/admin/capability-requests/:id",
        description: "Get capability request details.",
        summary: "Get capability request",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["capabilities"],
        example: "No request body required",
      },
    ],
  },

  {
    category: "Compliance & Audit",
    description: "SOC 2, HIPAA, GDPR compliance reporting and audit logs",
    icon: "ClipboardCheck",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/admin/compliance",
        description: "Get compliance dashboard.",
        summary: "Compliance dashboard",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/compliance/audit-logs",
        description: "Get audit logs for compliance.",
        summary: "Compliance audit logs",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance", "audit"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/compliance/access-reviews",
        description: "Get access review reports.",
        summary: "Access reviews",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/compliance/data-retention",
        description: "Get data retention policy status.",
        summary: "Data retention",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/compliance/export",
        description: "Export compliance report (SOC 2, HIPAA, GDPR).",
        summary: "Export compliance report",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance", "reports"],
        requestSchema: {
          type: "object",
          properties: {
            reportType: {
              type: "string",
              description: "Report type (soc2, hipaa, gdpr)",
              required: true,
            },
            startDate: { type: "string", description: "Start date" },
            endDate: { type: "string", description: "End date" },
          },
        },
        example: `{
  "reportType": "soc2",
  "startDate": "2025-01-01",
  "endDate": "2025-12-31"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/admin/compliance/violations",
        description: "Get compliance violations.",
        summary: "Compliance violations",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/admin/compliance/certifications",
        description: "Get compliance certifications status.",
        summary: "Certifications status",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/admin/compliance/automated-check",
        description: "Trigger automated compliance check.",
        summary: "Automated compliance check",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "admin",
        tags: ["compliance"],
        example: "{}",
      },
    ],
  },

  {
    category: "SDK API - Testing & Workflows",
    description:
      "SDK endpoints for testing alerts, verifications, and capability workflows",
    icon: "Zap",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/sdk-api/agents",
        description: "Register agent via SDK. Auto-generates credentials.",
        summary: "SDK: Register agent",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "agents"],
        requestSchema: {
          type: "object",
          properties: {
            name: { type: "string", description: "Agent name", required: true },
            type: { type: "string", description: "Agent type", required: true },
            description: { type: "string", description: "Agent description" },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            agentId: { type: "string", description: "Agent ID" },
            name: { type: "string", description: "Agent name" },
            trustScore: { type: "number", description: "Initial trust score" },
          },
        },
        example: `{
  "name": "test-agent",
  "type": "ai_agent",
  "description": "Test agent for SDK"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/sdk-api/agents/:id",
        description: "Get agent details via SDK.",
        summary: "SDK: Get agent",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Agent ID" },
            name: { type: "string", description: "Agent name" },
            trust_score: { type: "number", description: "Current trust score" },
            status: { type: "string", description: "Agent status" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/sdk-api/verifications",
        description:
          "Verify agent action. Tests capability-based access control. Creates alerts on violations.",
        summary: "SDK: Verify action",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "verification", "testing"],
        requestSchema: {
          type: "object",
          properties: {
            agent_id: {
              type: "string",
              description: "Agent ID",
              required: true,
            },
            action_type: {
              type: "string",
              description: "Action type (e.g., execute_code, write_database)",
              required: true,
            },
            resource: {
              type: "string",
              description: "Resource being accessed",
              required: true,
            },
            context: { type: "object", description: "Action context metadata" },
            timestamp: { type: "string", description: "ISO 8601 timestamp" },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Verification ID" },
            status: { type: "string", description: "allowed or blocked" },
            agent_id: { type: "string", description: "Agent ID" },
            action_type: { type: "string", description: "Action type" },
            alert_created: {
              type: "boolean",
              description: "Whether alert was created",
            },
          },
        },
        example: `{
  "agent_id": "uuid-agent-123",
  "action_type": "execute_code",
  "resource": "eval(user_input)",
  "context": {
    "code": "print('hello')",
    "risk": "high"
  },
  "timestamp": "2025-10-23T10:00:00Z"
}`,
      },
      {
        method: "POST",
        path: "/api/v1/sdk-api/agents/:id/capability-requests",
        description:
          "Request new capability for agent. Creates request pending admin approval.",
        summary: "SDK: Request capability",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "capabilities", "testing"],
        requestSchema: {
          type: "object",
          properties: {
            capability_type: {
              type: "string",
              description: "Capability type (e.g., write_database, send_email)",
              required: true,
            },
            reason: {
              type: "string",
              description: "Business justification",
              required: true,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Request ID" },
            agent_id: { type: "string", description: "Agent ID" },
            capability_type: {
              type: "string",
              description: "Requested capability",
            },
            status: {
              type: "string",
              description: "pending, approved, rejected",
            },
            reason: { type: "string", description: "Request reason" },
            created_at: { type: "string", description: "Creation timestamp" },
          },
        },
        example: `{
  "capability_type": "write_database",
  "reason": "Need to update user records for analytics"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/sdk-api/agents/:id/capabilities",
        description: "Get agent capabilities.",
        summary: "SDK: Get capabilities",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "capabilities"],
        responseSchema: {
          type: "object",
          properties: {
            capabilities: {
              type: "array",
              description: "List of granted capabilities",
            },
            count: { type: "number", description: "Total count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/sdk-api/agents/:id/alerts",
        description: "Get security alerts for agent.",
        summary: "SDK: Get alerts",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "alerts", "testing"],
        responseSchema: {
          type: "object",
          properties: {
            alerts: { type: "array", description: "Security alerts" },
            count: { type: "number", description: "Total alerts" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/sdk-api/agents/:id/activities",
        description: "Log agent activity.",
        summary: "SDK: Log activity",
        auth: "API Key (X-API-Key)",
        requiresAuth: true,
        tags: ["sdk", "activities"],
        requestSchema: {
          type: "object",
          properties: {
            action_type: {
              type: "string",
              description: "Activity type",
              required: true,
            },
            resource: { type: "string", description: "Resource accessed" },
            status: { type: "string", description: "success or failed" },
            metadata: { type: "object", description: "Additional metadata" },
          },
        },
        example: `{
  "action_type": "read_files",
  "resource": "config.json",
  "status": "success",
  "metadata": {
    "file_size": 1024
  }
}`,
      },
      {
        method: "POST",
        path: "/api/v1/sdk-api/agents/:id/mcp-connections",
        description:
          "Record agent-MCP connection when agent uses MCP tool. Automatically tracks which MCP servers agent communicates with.",
        summary: "SDK: Record MCP connection",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["sdk", "mcp", "connections"],
        requestSchema: {
          type: "object",
          properties: {
            mcp_server_id: {
              type: "string",
              description: "MCP server ID",
              required: true,
            },
            connection_method: {
              type: "string",
              description: "How connection was established",
              example: "use_mcp_tool",
            },
            metadata: {
              type: "object",
              description: "Connection metadata",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            connectionId: { type: "string", description: "Connection record ID" },
            status: { type: "string", description: "Connection status" },
          },
        },
        example: `{
  "mcp_server_id": "uuid-mcp-123",
  "connection_method": "use_mcp_tool",
  "metadata": {
    "tool_name": "read_file"
  }
}`,
      },
      {
        method: "GET",
        path: "/api/v1/sdk-api/agents/:id/mcp-servers",
        description:
          "List all MCP servers available to agent's organization. SDK agents use this to discover available MCPs.",
        summary: "SDK: List available MCP servers",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["sdk", "mcp"],
        responseSchema: {
          type: "object",
          properties: {
            mcpServers: {
              type: "array",
              description: "Available MCP servers",
            },
            count: { type: "number", description: "Total count" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/sdk-api/agents/:id/mcp-servers",
        description:
          "Create new MCP server via SDK. Allows agents to register new MCP servers they discover.",
        summary: "SDK: Create MCP server",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["sdk", "mcp"],
        requestSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "MCP server name",
              required: true,
            },
            url: {
              type: "string",
              description: "MCP server URL",
              required: true,
            },
            capabilities: {
              type: "array",
              description: "Detected capabilities",
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            mcpServerId: { type: "string", description: "Created MCP server ID" },
            name: { type: "string", description: "MCP server name" },
          },
        },
        example: `{
  "name": "filesystem-mcp",
  "url": "http://localhost:3000",
  "capabilities": ["read_file", "write_file"]
}`,
      },
      {
        method: "GET",
        path: "/api/v1/sdk-api/agents/:identifier",
        description:
          "Get agent by ID or name (SDK). Allows SDK to fetch agent details using either ID or name.",
        summary: "SDK: Get agent by identifier",
        auth: "Ed25519 (Agent Signature)",
        requiresAuth: true,
        tags: ["sdk", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Agent ID" },
            name: { type: "string", description: "Agent name" },
            trustScore: { type: "number", description: "Current trust score" },
          },
        },
        example: "No request body required",
      },
    ],
  },

  {
    category: "Analytics & Reporting",
    description: "Usage statistics, trends, and business intelligence",
    icon: "BarChart3",
    endpoints: [
      {
        method: "GET",
        path: "/api/v1/analytics/usage",
        description: "Get usage statistics.",
        summary: "Usage statistics",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/analytics/activity",
        description:
          "Get activity summary for agents and MCP servers over a specified time period (default 7 days). Includes verification counts, attestation counts, and recent activity events.",
        summary: "Get Activity Summary",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics", "activity"],
        responseSchema: {
          type: "object",
          properties: {
            period: {
              type: "object",
              description: "Time period for the summary",
            },
            summary: {
              type: "object",
              description:
                "Summary statistics (total_agents, total_mcp_servers, verification_count, etc.)",
            },
            activity_by_day: {
              type: "array",
              description: "Daily activity breakdown",
            },
            recent_activity: {
              type: "array",
              description: "Most recent activity events",
            },
          },
        },
        example: "?days=30 for 30-day summary (default is 7 days)",
      },
      {
        method: "GET",
        path: "/api/v1/analytics/trends",
        description: "Get usage trends over time.",
        summary: "Usage trends",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/analytics/trust-scores",
        description: "Get trust score analytics.",
        summary: "Trust score analytics",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics", "trust"],
        example: "No request body required",
      },
      {
        method: "GET",
        path: "/api/v1/analytics/agents/summary",
        description: "Get agent summary statistics.",
        summary: "Agent summary",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics", "agents"],
        responseSchema: {
          type: "object",
          properties: {
            total_agents: { type: "number", description: "Total agent count" },
            verified_agents: { type: "number", description: "Verified count" },
            total_users: { type: "number", description: "Total users" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/analytics/reports/generate",
        description: "Generate custom analytics report.",
        summary: "Generate report",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["analytics", "reports"],
        requestSchema: {
          type: "object",
          properties: {
            reportType: {
              type: "string",
              description: "Report type",
              required: true,
            },
            dateRange: { type: "object", description: "Date range" },
            metrics: { type: "array", description: "Metrics to include" },
          },
        },
        example: `{
  "reportType": "monthly_summary",
  "dateRange": {
    "start": "2025-01-01",
    "end": "2025-01-31"
  },
  "metrics": ["agents", "verifications", "trust_scores"]
}`,
      },
    ],
  },
  {
    category: "Verifications",
    description: "Agent action verification and approval workflows",
    icon: "Shield",
    endpoints: [
      {
        method: "POST",
        path: "/api/v1/verifications",
        description: "Request verification for an agent action before execution. Creates a verification request that must be approved/rejected by an authorized user.",
        summary: "Request agent action verification",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["verifications", "security"],
        requestSchema: {
          type: "object",
          properties: {
            agent_id: {
              type: "string",
              description: "Agent ID requesting verification",
              required: true,
            },
            action_type: {
              type: "string",
              description: "Type of action (e.g., 'deploy', 'delete', 'modify')",
              required: true,
            },
            action_details: {
              type: "object",
              description: "Details about the action to be verified",
              required: true,
            },
            risk_level: {
              type: "string",
              description: "Risk level: low, medium, high, critical",
              required: false,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            verification_id: { type: "string", description: "Unique verification ID" },
            status: { type: "string", description: "Verification status (pending, approved, rejected)" },
            expires_at: { type: "string", description: "Expiration timestamp" },
          },
        },
        example: `{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "action_type": "deploy",
  "action_details": {
    "resource": "production-database",
    "changes": ["schema_migration", "index_creation"]
  },
  "risk_level": "high"
}`,
      },
      {
        method: "GET",
        path: "/api/v1/verifications/:id",
        description: "Get verification status and details by verification ID. Check if a verification request has been approved, rejected, or is still pending.",
        summary: "Get verification status",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        tags: ["verifications", "security"],
        responseSchema: {
          type: "object",
          properties: {
            id: { type: "string", description: "Verification ID" },
            agent_id: { type: "string", description: "Agent ID" },
            status: { type: "string", description: "Current status (pending, approved, rejected, expired)" },
            action_type: { type: "string", description: "Type of action" },
            action_details: { type: "object", description: "Action details" },
            requested_at: { type: "string", description: "Request timestamp" },
            reviewed_at: { type: "string", description: "Review timestamp (if reviewed)" },
            reviewed_by: { type: "string", description: "User who reviewed (if reviewed)" },
            expires_at: { type: "string", description: "Expiration timestamp" },
          },
        },
        example: "No request body required",
      },
      {
        method: "POST",
        path: "/api/v1/verifications/:id/result",
        description: "Submit verification result (approve or reject). Called by authorized users to approve/reject agent action verification requests.",
        summary: "Submit verification result",
        auth: "Bearer Token (JWT)",
        requiresAuth: true,
        roleRequired: "manager",
        tags: ["verifications", "security"],
        requestSchema: {
          type: "object",
          properties: {
            status: {
              type: "string",
              description: "Verification result: 'approved' or 'rejected'",
              required: true,
            },
            comments: {
              type: "string",
              description: "Optional comments about the decision",
              required: false,
            },
            conditions: {
              type: "array",
              description: "Optional conditions for approval (e.g., time windows, monitoring requirements)",
              required: false,
            },
          },
        },
        responseSchema: {
          type: "object",
          properties: {
            verification_id: { type: "string", description: "Verification ID" },
            status: { type: "string", description: "Updated status" },
            reviewed_at: { type: "string", description: "Review timestamp" },
            reviewed_by: { type: "string", description: "Reviewer user ID" },
          },
        },
        example: `{
  "status": "approved",
  "comments": "Approved with monitoring",
  "conditions": [
    "rollback_plan_required",
    "monitoring_enabled"
  ]
}`,
      },
    ],
  },
];

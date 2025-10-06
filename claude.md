# 🧠 Claude Code Workflow for Agent Identity Management

## 🚨 CRITICAL RULES - AVOID COMMON PITFALLS

### 1. **Naming Consistency is SACRED**
**PROBLEM**: Using different names for the same concept causes bugs that are hard to find.

**BAD EXAMPLES**:
- `lastCalculated` in one file, `calculatedAt` in another ❌
- `auth_token` in backend, `authToken` in frontend ❌
- `mcp_server_id` in database, `serverId` in TypeScript ❌

**SOLUTION**:
- **Before creating any new field/variable, check ALL existing code for similar concepts**
- **Use exact same naming across backend, frontend, and database**
- **Document naming conventions in this file and follow them strictly**

**NAMING CONVENTIONS FOR THIS PROJECT**:

#### Database (PostgreSQL - snake_case)
```sql
-- Timestamps
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
last_verified_at TIMESTAMPTZ
acknowledged_at TIMESTAMPTZ

-- IDs
organization_id UUID
user_id UUID
agent_id UUID
mcp_server_id UUID

-- Status fields
is_active BOOLEAN
is_verified BOOLEAN

-- Scores
trust_score DECIMAL(5,2)
```

#### Backend (Go - camelCase for JSON, PascalCase for structs)
```go
// Struct fields (PascalCase)
type Agent struct {
    ID              uuid.UUID
    OrganizationID  uuid.UUID
    CreatedAt       time.Time
    UpdatedAt       time.Time
    LastVerifiedAt  *time.Time
    IsActive        bool
    TrustScore      float64
}

// JSON fields (camelCase - MUST match frontend exactly!)
type AgentResponse struct {
    ID             string  `json:"id"`
    OrganizationID string  `json:"organizationId"`
    CreatedAt      string  `json:"createdAt"`
    UpdatedAt      string  `json:"updatedAt"`
    LastVerifiedAt *string `json:"lastVerifiedAt"`
    IsActive       bool    `json:"isActive"`
    TrustScore     float64 `json:"trustScore"`
}
```

#### Frontend (TypeScript - camelCase)
```typescript
// MUST match backend JSON exactly!
interface Agent {
  id: string;
  organizationId: string;
  createdAt: string;
  updatedAt: string;
  lastVerifiedAt: string | null;
  isActive: boolean;
  trustScore: number;
}

// NO variations like:
// - organization_id ❌
// - orgId ❌
// - lastCalculated ❌
// - calculatedAt ❌
```

### 2. **Icon Library Consistency**
**PROBLEM**: Mixing icon libraries causes build failures.

**RULE**: This project uses **lucide-react** exclusively.

```typescript
// CORRECT ✅
import { Check, X, AlertTriangle, Clipboard, Download } from 'lucide-react';

// WRONG ❌
import { CheckIcon } from '@heroicons/react/24/outline';
```

### 3. **Authentication Token Storage**
**PROBLEM**: Inconsistent localStorage key names.

**RULE**: Always use `auth_token` (with underscore).

```typescript
// CORRECT ✅
localStorage.getItem('auth_token')
localStorage.setItem('auth_token', token)

// WRONG ❌
localStorage.getItem('authToken')
localStorage.getItem('token')
```

### 4. **Before Adding ANY New Field**
**MANDATORY CHECKLIST**:
- [ ] Search codebase for similar concepts (timestamps, IDs, booleans, etc.)
- [ ] Check database schema for existing naming patterns
- [ ] Check backend structs for existing JSON tag patterns
- [ ] Check frontend interfaces for existing field names
- [ ] Choose name that matches existing convention exactly
- [ ] Document new field in this file if it's a new pattern

---

## 🔑 Golden Rules

### File Management
- ✅ Keep files under 500 lines - split into modules when approaching limit
- ✅ Use markdown files for planning (PLANNING.md, TASK.md, PROJECT_STATUS.md)
- ✅ One task per message for best results
- ✅ Start fresh conversations when threads get long (>20 messages)

### Testing
- ✅ **Test with Chrome DevTools MCP before marking frontend complete**
- ✅ Write tests as you build, not after
- ✅ Every function needs unit tests (success case, edge case, failure case)
- ✅ Backend tests in `apps/backend/tests/`
- ✅ Frontend tests in `apps/web/__tests__/`

### Documentation
- ✅ Update README.md when features change
- ✅ Update PLANNING.md when architecture changes
- ✅ Update TASK.md after completing tasks
- ✅ Docstrings/JSDoc for all public functions
- ✅ Comments for complex logic (explain WHY, not WHAT)

### Security
- ✅ Never hardcode secrets (use environment variables)
- ✅ Never commit .env files
- ✅ Hash API keys before storage
- ✅ All endpoints require authentication

---

## 📁 Project Structure Awareness

### Always Reference These Files First
1. **CLAUDE_CONTEXT.md** - Complete build instructions, tech stack, requirements
2. **PROJECT_OVERVIEW.md** - Vision, strategy, product roadmap
3. **30_HOUR_BUILD_PLAN.md** - Build phases and milestones
4. **TASK.md** - Current tasks and backlog
5. **This file (claude.md)** - Naming conventions and pitfall avoidance

### File Organization
```
agent-identity-management/
├── apps/
│   ├── backend/              # Go backend
│   │   ├── cmd/
│   │   │   └── server/       # Main entry point
│   │   ├── internal/
│   │   │   ├── domain/       # Business logic (pure)
│   │   │   ├── application/  # Use cases
│   │   │   ├── infrastructure/ # External dependencies
│   │   │   └── interfaces/   # HTTP handlers
│   │   ├── migrations/       # Database migrations
│   │   └── tests/            # Backend tests
│   ├── web/                  # Next.js frontend
│   │   ├── app/              # App Router pages
│   │   ├── components/       # React components
│   │   ├── lib/              # Utilities
│   │   └── __tests__/        # Frontend tests
│   └── docs/                 # Docusaurus documentation
├── packages/
│   ├── ui/                   # Shared components
│   └── types/                # Shared TypeScript types
└── infrastructure/
    ├── docker/               # Dockerfiles
    └── k8s/                  # Kubernetes manifests
```

---

## 🧪 Testing Workflow

### Backend (Go)
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/domain/...
```

**Test Structure**:
```go
func TestAgentService_CreateAgent(t *testing.T) {
    // Setup
    mockDB := setupMockDB()
    service := NewAgentService(mockDB)

    // Test case
    t.Run("creates agent successfully", func(t *testing.T) {
        agent, err := service.CreateAgent(context.Background(), validInput)
        assert.NoError(t, err)
        assert.NotNil(t, agent)
    })

    t.Run("returns error for invalid input", func(t *testing.T) {
        _, err := service.CreateAgent(context.Background(), invalidInput)
        assert.Error(t, err)
    })
}
```

### Frontend (Next.js + TypeScript)
```bash
# Run all tests
npm test

# Run with coverage
npm test -- --coverage

# Run in watch mode
npm test -- --watch
```

**Test Structure**:
```typescript
describe('AgentRegistrationForm', () => {
  it('renders form correctly', () => {
    render(<AgentRegistrationForm />);
    expect(screen.getByLabelText('Agent Name')).toBeInTheDocument();
  });

  it('submits form with valid data', async () => {
    const onSubmit = jest.fn();
    render(<AgentRegistrationForm onSubmit={onSubmit} />);

    await userEvent.type(screen.getByLabelText('Agent Name'), 'Test Agent');
    await userEvent.click(screen.getByRole('button', { name: /register/i }));

    expect(onSubmit).toHaveBeenCalledWith({ name: 'Test Agent' });
  });
});
```

### Chrome DevTools MCP Testing (MANDATORY for Frontend)
**Before marking ANY frontend feature complete**:
```typescript
// 1. Navigate to page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/agents/new" })

// 2. Take snapshot to see elements
mcp__chrome-devtools__take_snapshot()

// 3. Fill form (using UIDs from snapshot)
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "name-input-uid", value: "test-agent" },
    { uid: "type-select-uid", value: "ai_agent" }
  ]
})

// 4. Submit
mcp__chrome-devtools__click({ uid: "submit-button-uid" })

// 5. Verify API call
mcp__chrome-devtools__list_network_requests({ resourceTypes: ["xhr", "fetch"] })

// 6. Verify UI update
mcp__chrome-devtools__take_screenshot()
```

---

## 🔄 Task Management Workflow

### Before Starting Work
1. Read TASK.md to see current priorities
2. Check if your task is listed
3. If not, add it with brief description and date

### During Work
1. Mark task as "in progress" in TASK.md
2. If you discover new sub-tasks, add them under "Discovered During Work"
3. Update PLANNING.md if architecture changes

### After Completing Work
1. ✅ Mark task complete in TASK.md
2. ✅ Update README.md if user-facing changes
3. ✅ Ensure tests pass
4. ✅ **Test with Chrome DevTools MCP if frontend**
5. ✅ Commit with clear message

---

## 💬 Effective Prompting

### Good Prompts (Specific, Single Task)
```
✅ "Update the trust score calculation to include uptime percentage (25% weight)"
✅ "Add filtering by agent type to the GET /agents endpoint"
✅ "Fix the authentication bug where auth_token is not being sent in API requests"
```

### Bad Prompts (Vague, Multiple Tasks)
```
❌ "Fix all the bugs"
❌ "Make the UI better"
❌ "Update agents API, fix auth, and add documentation"
```

### When Asking for Help
```
✅ Include error message in full
✅ Share relevant file paths
✅ Describe what you've already tried
✅ Provide expected vs actual behavior
```

---

## 🐛 Common Issues & Solutions

### Issue: "Cannot find module"
**Solution**: Check import paths match actual file structure
```typescript
// If you see this error, verify:
// 1. File exists at path
// 2. Import path is correct (relative vs absolute)
// 3. TypeScript paths in tsconfig.json are correct
```

### Issue: "Type mismatch between frontend and backend"
**Solution**: Check JSON tags match TypeScript interface exactly
```go
// Backend
type Agent struct {
    TrustScore float64 `json:"trustScore"` // ✅ camelCase
}

// Frontend
interface Agent {
  trustScore: number; // ✅ MUST match exactly
}
```

### Issue: "localStorage auth token not found"
**Solution**: Always use `auth_token` key consistently
```typescript
// Check both possible keys for compatibility
const token = localStorage.getItem('auth_token') || localStorage.getItem('authToken');
```

---

## 🎯 Quality Checklist

Before marking ANY task complete:

### Code Quality
- [ ] No hardcoded values (use constants/env vars)
- [ ] Error handling implemented
- [ ] Logging added for important operations
- [ ] Type safety (Go types, TypeScript interfaces)

### Testing
- [ ] Unit tests written and passing
- [ ] Integration tests if applicable
- [ ] **Chrome DevTools MCP testing done (frontend)**
- [ ] No console errors

### Documentation
- [ ] Function/method docstrings added
- [ ] README.md updated if needed
- [ ] TASK.md marked complete
- [ ] Comments added for complex logic

### Naming Consistency
- [ ] Checked existing code for similar concepts
- [ ] Used exact same naming convention
- [ ] Database snake_case, JSON camelCase, TypeScript camelCase
- [ ] No variations of same concept

---

## 🚀 Deployment Preparation

### Environment Variables
**Never commit these!**
```bash
# Backend (.env)
DATABASE_URL=postgresql://...
REDIS_URL=redis://...
JWT_SECRET=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...

# Frontend (.env.local)
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### Docker Build
```bash
# Build backend
docker build -f infrastructure/docker/Dockerfile.backend -t agent-identity-backend .

# Build frontend
docker build -f infrastructure/docker/Dockerfile.frontend -t agent-identity-frontend .

# Run with Docker Compose
docker compose up -d
```

### Pre-Deployment Checklist
- [ ] All tests passing
- [ ] Environment variables configured
- [ ] Database migrations ready
- [ ] Docker images build successfully
- [ ] Health checks working
- [ ] Security scan passed (Trivy)
- [ ] Load test passed (k6)

---

## 📚 Key Resources

### Documentation to Reference
- **Go Fiber v3**: https://docs.gofiber.io/
- **Next.js 15**: https://nextjs.org/docs
- **PostgreSQL 16**: https://www.postgresql.org/docs/16/
- **Shadcn/ui**: https://ui.shadcn.com/
- **Lucide React**: https://lucide.dev/

### MCP Tools Available
- **chrome-devtools**: Browser testing (MANDATORY for frontend)
- **WebSearch**: Research and documentation lookup
- **filesystem**: File operations
- **github**: Repository operations

---

## 🎓 Remember

1. **Consistency beats cleverness** - use boring, predictable names
2. **Test before marking complete** - especially with Chrome DevTools MCP
3. **One task at a time** - focused work yields better results
4. **Document as you go** - future you will thank present you
5. **When in doubt, check existing code** - follow established patterns

---

**Last Updated**: October 5, 2025
**Project**: Agent Identity Management (OpenA2A)
**Repository**: https://github.com/opena2a-org/agent-identity-management

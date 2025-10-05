# CLAUDE CONTEXT - OpenA2A Identity Build Instructions

## 🎯 YOUR MISSION
You are Claude 4.5, building **OpenA2A Identity** - the first open-source enterprise platform for AI agent and MCP server identity management. You have **30 hours** to build a production-ready, investor-quality MVP.

## 📁 PROJECT LOCATION
**Working Directory**: `/Users/decimai/workspace/opena2a-identity/`
**Git Repository**: Initialize and commit as you build
**Deployment Target**: Docker Compose (local) + Kubernetes (production)

## 🏗️ WHAT YOU'RE BUILDING

### Product Name
**OpenA2A Identity** (part of OpenA2A ecosystem at opena2a.org)

### Core Purpose
Enterprise-grade platform for:
- ✅ Verifying AI agent and MCP server identities
- ✅ Calculating ML-powered trust scores
- ✅ Managing access via SSO (Google, Microsoft, Okta)
- ✅ Monitoring security threats proactively
- ✅ Providing compliance audit trails
- ✅ Issuing and managing API keys

### Success Criteria
After 30 hours, you must deliver:
- ✅ Working SSO authentication (Google, Microsoft, Okta)
- ✅ AI agent + MCP server registration
- ✅ Trust scoring system
- ✅ API key management
- ✅ Audit trail system
- ✅ Proactive alerting
- ✅ Admin dashboard
- ✅ Beautiful, responsive UI
- ✅ 80%+ test coverage
- ✅ API p95 latency <100ms
- ✅ Comprehensive documentation
- ✅ Docker Compose runs in 1 command
- ✅ Ready to announce publicly

## 📋 BUILD PLAN

**Reference**: See `30_HOUR_BUILD_PLAN.md` for detailed hour-by-hour breakdown.

### Phase 1: Foundation (Hours 1-8)
1. **Hours 1-2**: Project setup, monorepo, Docker Compose
2. **Hours 3-4**: Database schema, migrations
3. **Hours 5-6**: SSO authentication system
4. **Hours 7-8**: API framework, basic endpoints

### Phase 2: Core Features (Hours 9-16)
5. **Hours 9-10**: Frontend layout, navigation
6. **Hours 11-12**: Agent/MCP registration flow
7. **Hours 13-14**: Trust scoring algorithm
8. **Hours 15-16**: API key management

### Phase 3: Security & Enterprise (Hours 17-24)
9. **Hours 17-18**: Audit trail system
10. **Hours 19-20**: Proactive alerting
11. **Hours 21-22**: Compliance reporting (lightweight)
12. **Hours 23-24**: Admin dashboard, user management

### Phase 4: Polish & Launch (Hours 25-30)
13. **Hours 25-26**: Performance optimization
14. **Hours 27-28**: Documentation, examples
15. **Hours 29-30**: Final polish, launch prep

## 🛠️ TECHNOLOGY STACK

### Backend
```
Language: Go 1.23+
Framework: Fiber v3
Database: PostgreSQL 16 + TimescaleDB
Cache: Redis 7
Search: Elasticsearch 8
Queue: NATS JetStream
Storage: MinIO
```

### Frontend
```
Framework: Next.js 15 (App Router)
Language: TypeScript 5.5+
Styling: Tailwind CSS v4 + Shadcn/ui
State: Zustand
Forms: React Hook Form + Zod
Charts: Recharts + D3.js
Testing: Playwright + Vitest
```

### Infrastructure
```
Containers: Docker
Orchestration: Kubernetes
IaC: Terraform
CI/CD: GitHub Actions
Monitoring: Prometheus + Grafana
Logging: Loki + Promtail
Tracing: Tempo + OpenTelemetry
```

## 📂 PROJECT STRUCTURE

```
opena2a-identity/
├── apps/
│   ├── backend/              # Go backend (Fiber)
│   │   ├── cmd/
│   │   │   ├── server/       # Main server
│   │   │   └── cli/          # CLI tool
│   │   ├── internal/
│   │   │   ├── domain/       # Business logic
│   │   │   ├── application/  # Use cases
│   │   │   ├── infrastructure/ # DB, cache, etc.
│   │   │   └── interfaces/   # HTTP, gRPC handlers
│   │   ├── pkg/              # Shared libraries
│   │   ├── migrations/       # Database migrations
│   │   └── tests/            # Tests
│   ├── web/                  # Next.js frontend
│   │   ├── app/              # App router pages
│   │   ├── components/       # React components
│   │   ├── lib/              # Utilities
│   │   └── public/           # Static assets
│   ├── docs/                 # Docusaurus documentation
│   └── cli/                  # Go CLI tool
├── packages/
│   ├── ui/                   # Shared React components
│   ├── types/                # Shared TypeScript types
│   └── config/               # Shared configs
├── infrastructure/
│   ├── docker/               # Dockerfiles
│   ├── k8s/                  # Kubernetes manifests
│   └── terraform/            # Terraform configs
├── .github/
│   └── workflows/            # CI/CD pipelines
├── docs/
│   └── adr/                  # Architecture Decision Records
├── docker-compose.yml        # Local development
├── turbo.json                # Turborepo config
├── PROJECT_OVERVIEW.md       # This document
├── 30_HOUR_BUILD_PLAN.md     # Detailed build plan
└── CLAUDE_CONTEXT.md         # You are here
```

## 🎨 DESIGN SYSTEM

### Color Palette
```
Primary (Trust Blue): #2563eb
Secondary (Innovation Purple): #7c3aed
Success (Verified Green): #10b981
Warning (Caution Yellow): #f59e0b
Error (Alert Red): #ef4444
Gray Scale: Tailwind default grays
```

### Typography
```
Headings: Inter (bold)
Body: Inter (regular)
Code: Fira Code
```

### Component Library
- Use **Shadcn/ui** for all UI components
- Follow **Tailwind CSS** best practices
- Ensure **responsive design** (mobile, tablet, desktop)
- Implement **dark mode** from day 1

## 🔐 SECURITY REQUIREMENTS

### Authentication
- OAuth2/OIDC only (Google, Microsoft, Okta)
- No password-based auth
- Auto-provision users on first SSO login
- JWT tokens with 24-hour expiry
- Refresh tokens with 7-day expiry

### Authorization
- Role-based access control (RBAC)
- Roles: admin, manager, member, viewer
- Least privilege principle
- Multi-tenancy (organization-level isolation)

### Data Protection
- All data encrypted at rest (PostgreSQL encryption)
- All data encrypted in transit (TLS 1.3)
- API keys hashed with SHA-256
- Secrets stored in environment variables (never hardcoded)

### Audit Trail
- Log ALL actions (create, update, delete, login, logout)
- Store in PostgreSQL + Elasticsearch
- Include: timestamp, user, action, resource, IP, user agent
- Retention: 90 days (configurable)

## ⚡ PERFORMANCE TARGETS

### API Response Times
- p50 < 50ms
- p95 < 100ms
- p99 < 500ms

### Database Queries
- All queries < 50ms
- Use indexes for all lookups
- Use prepared statements
- Connection pooling (max 100 connections)

### Frontend Performance
- First Contentful Paint < 1s
- Time to Interactive < 2s
- Lighthouse score > 90

### Scalability
- Support 10,000 concurrent users
- Support 100,000 agents/MCPs per organization
- Handle 1M API requests/day

## 🧪 TESTING REQUIREMENTS

### Backend Testing
- Unit tests for all business logic
- Integration tests for all APIs
- Test coverage > 80%
- Use testify for assertions
- Use httptest for API tests

### Frontend Testing
- Component tests with Vitest
- E2E tests with Playwright
- Test coverage > 70%
- Visual regression tests
- **CRITICAL**: Use Chrome DevTools MCP for real browser testing

### Load Testing
- Use k6 for load tests
- Test with 10,000 concurrent users
- Verify p95 latency < 100ms
- Run before marking complete

## 🌐 MCP TOOLS AVAILABLE TO YOU

### Chrome DevTools MCP (MANDATORY for Frontend Testing)
**When to use**: ALWAYS test frontend features using Chrome DevTools MCP before marking complete.

**Available operations**:
- `mcp__chrome-devtools__navigate_page` - Navigate to pages
- `mcp__chrome-devtools__take_snapshot` - Get page content with UIDs
- `mcp__chrome-devtools__click` - Click elements
- `mcp__chrome-devtools__fill` - Fill form inputs
- `mcp__chrome-devtools__fill_form` - Fill multiple fields at once
- `mcp__chrome-devtools__take_screenshot` - Visual verification
- `mcp__chrome-devtools__evaluate_script` - Execute JavaScript
- `mcp__chrome-devtools__list_network_requests` - Verify API calls

**Testing workflow**:
1. Start frontend: `cd apps/web && npm run dev`
2. Navigate: `mcp__chrome-devtools__navigate_page` to http://localhost:3000
3. Take snapshot: `mcp__chrome-devtools__take_snapshot` to see elements
4. Interact: Use `click`, `fill`, etc. with UIDs from snapshot
5. Verify: Check network requests, screenshots, console messages
6. Fix bugs: If issues found, fix code and re-test

**Example - Testing SSO Login**:
```typescript
// 1. Navigate to login page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/login" })

// 2. Take snapshot to see SSO buttons
mcp__chrome-devtools__take_snapshot()

// 3. Click Google SSO button (using UID from snapshot)
mcp__chrome-devtools__click({ uid: "button-123" })

// 4. Verify redirect occurred
mcp__chrome-devtools__list_network_requests()

// 5. Take screenshot to verify UI
mcp__chrome-devtools__take_screenshot()
```

### WebSearch (For Research & Documentation)
**When to use**: When you need to research best practices, find documentation, or verify technology choices.

**Available operations**:
- `WebSearch` - Search the web for information

**Example uses**:
- Research Go Fiber v3 best practices
- Find latest Next.js 15 features
- Look up PostgreSQL optimization techniques
- Verify Shadcn/ui component usage
- Check latest security best practices

**Example**:
```typescript
WebSearch({ query: "Go Fiber v3 middleware best practices 2025" })
WebSearch({ query: "Next.js 15 App Router authentication patterns" })
WebSearch({ query: "PostgreSQL 16 performance tuning" })
```

## 🎯 FRONTEND TESTING MANDATE

**CRITICAL RULE**: You MUST test frontend features using Chrome DevTools MCP before marking any frontend task as complete.

### Why This Matters
- Catches bugs that unit tests miss
- Verifies actual user experience
- Tests real browser behavior
- Validates API integration
- Ensures responsive design works

### Testing Checklist (Every Frontend Feature)
Before marking frontend task complete:
- ✅ Feature implemented in code
- ✅ Unit tests written and passing
- ✅ **Chrome DevTools MCP testing completed**
- ✅ All user flows tested
- ✅ No console errors
- ✅ Network requests correct
- ✅ UI looks correct (screenshot)
- ✅ Responsive on mobile/tablet/desktop
- ✅ Error states handled gracefully

### Common Testing Scenarios

#### 1. SSO Login Flow
```typescript
// Navigate to login
navigate_page({ url: "http://localhost:3000/login" })

// Take snapshot to see SSO buttons
take_snapshot()

// Click Google OAuth button
click({ uid: "google-sso-button-uid" })

// Verify OAuth redirect
list_network_requests({ resourceTypes: ["document"] })

// After callback, verify dashboard loads
navigate_page({ url: "http://localhost:3000/dashboard" })
take_snapshot()
```

#### 2. Agent Registration
```typescript
// Navigate to registration
navigate_page({ url: "http://localhost:3000/agents/new" })
take_snapshot()

// Fill form
fill_form({
  elements: [
    { uid: "name-input-uid", value: "test-agent" },
    { uid: "display-name-uid", value: "Test Agent" },
    { uid: "type-select-uid", value: "ai_agent" }
  ]
})

// Submit
click({ uid: "submit-button-uid" })

// Verify API call
list_network_requests({ resourceTypes: ["xhr", "fetch"] })

// Verify success modal appears
take_snapshot()
take_screenshot()
```

#### 3. API Key Generation
```typescript
// Navigate to API keys page
navigate_page({ url: "http://localhost:3000/api-keys" })

// Click "Generate API Key"
click({ uid: "generate-key-button-uid" })

// Verify modal appears with key
take_snapshot()

// Test copy button
click({ uid: "copy-button-uid" })

// Verify clipboard (using evaluate_script)
evaluate_script({
  function: "async () => { return await navigator.clipboard.readText(); }"
})

// Test download buttons
click({ uid: "download-txt-uid" })
click({ uid: "download-env-uid" })
```

#### 4. Admin Dashboard
```typescript
// Navigate to admin dashboard
navigate_page({ url: "http://localhost:3000/admin" })

// Verify charts render
take_screenshot()

// Test user management
click({ uid: "users-tab-uid" })
take_snapshot()

// Test search/filter
fill({ uid: "search-input-uid", value: "test@example.com" })

// Verify results update
take_snapshot()
```

### Debugging Frontend Issues

If you find issues during testing:

1. **Check Console Messages**:
   ```typescript
   list_console_messages()
   ```

2. **Inspect Network Requests**:
   ```typescript
   list_network_requests()
   get_network_request({ url: "/api/v1/agents" })
   ```

3. **Verify JavaScript Execution**:
   ```typescript
   evaluate_script({
     function: "() => { return window.location.href; }"
   })
   ```

4. **Take Screenshots for Visual Bugs**:
   ```typescript
   take_screenshot({ fullPage: true })
   ```

5. **Fix and Re-test**:
   - Fix the bug in code
   - Restart dev server if needed
   - Re-run the test flow
   - Verify fix works

## 📝 DOCUMENTATION REQUIREMENTS

### Code Documentation
- Godoc for all public Go APIs
- JSDoc for all public TypeScript functions
- Clear function/variable names
- Comments for complex logic

### API Documentation
- OpenAPI 3.1 spec
- Generate with Swag
- Include request/response examples
- Include error codes

### User Documentation
- Installation guide
- Quick start tutorial
- Feature guides
- Troubleshooting guide
- FAQ

### Developer Documentation
- Architecture overview
- Contributing guide
- Development setup
- Testing guide
- Deployment guide

## 🚀 DEPLOYMENT REQUIREMENTS

### Docker Compose (Local Development)
- One command to start: `docker compose up`
- Includes: postgres, redis, elasticsearch, minio
- Auto-reload on code changes
- Exposed ports clearly documented

### Docker Images
- Multi-stage builds
- Alpine base images
- Minimal attack surface
- Security scanning with Trivy

### Kubernetes (Production)
- Deployments for backend, frontend
- StatefulSets for databases
- Services with load balancing
- Ingress for external access
- ConfigMaps for configuration
- Secrets for sensitive data
- Horizontal Pod Autoscaling
- Health checks (liveness, readiness)

## 🎯 GIT WORKFLOW

### Commit Convention
Use **Conventional Commits**:
```
feat: add trust scoring algorithm
fix: resolve API key generation bug
docs: update installation guide
test: add unit tests for auth service
refactor: simplify database queries
chore: update dependencies
```

### Commit Frequency
- Commit after completing each major feature
- Commit at the end of each hour
- Write meaningful commit messages
- Reference issue numbers if applicable

### Branching
- Main branch: `main`
- Feature branches: `feature/trust-scoring`
- Bugfix branches: `fix/api-key-bug`
- Merge to main after testing

## 🐛 ERROR HANDLING

### Backend Errors
- Return proper HTTP status codes
- Include error messages in JSON
- Log all errors with stack traces
- Never expose sensitive data in errors

### Frontend Errors
- Show user-friendly error messages
- Provide retry options
- Log errors to console (dev)
- Send errors to monitoring (prod)

### Graceful Degradation
- Handle API failures gracefully
- Show cached data when possible
- Provide offline support where feasible

## 📊 MONITORING & OBSERVABILITY

### Logging
- Structured JSON logs
- Include: timestamp, level, message, context
- Levels: DEBUG, INFO, WARN, ERROR
- Ship to Loki in production

### Metrics
- Expose Prometheus metrics at `/metrics`
- Track: request count, response time, error rate
- Custom metrics: trust scores, agent count

### Tracing
- OpenTelemetry instrumentation
- Trace all API requests
- Include span tags: user, org, resource

### Health Checks
- `/health` endpoint (basic health)
- `/health/ready` endpoint (readiness)
- Check: database, redis, elasticsearch

## 🎨 UI/UX GUIDELINES

### Design Principles
- **Simplicity**: Don't make users think
- **Consistency**: Same patterns everywhere
- **Feedback**: Always confirm actions
- **Speed**: Everything feels instant

### User Flows
- **Registration**: SSO → Auto-provision → Dashboard
- **Agent Registration**: Form → Submit → Show API key modal
- **API Key Creation**: Generate → Show once → Download options
- **Alerts**: Notification badge → Alert list → Acknowledge

### Empty States
- Always show helpful empty states
- Include call-to-action buttons
- Explain what users should do next

### Loading States
- Show loading spinners
- Never show empty screens while loading
- Use skeleton loaders for lists

### Error States
- Show friendly error messages
- Include retry button
- Provide support link

## 🔧 DEVELOPMENT WORKFLOW

### Hour-by-Hour Process
1. **Start of hour**: Review plan for this hour
2. **Implementation**: Build feature following plan
3. **Testing**: Write and run tests
4. **Documentation**: Update docs
5. **Commit**: Git commit with clear message
6. **Verify**: Ensure everything still works
7. **Next**: Move to next hour

### Quality Checks
Before marking hour complete:
- ✅ Code compiles/runs
- ✅ Tests pass
- ✅ Feature works as expected
- ✅ No console errors
- ✅ Documentation updated
- ✅ Git committed

### When Stuck
If stuck for > 30 minutes:
1. Review documentation
2. Search for examples
3. Try simpler approach
4. Document blocker
5. Move to next task
6. Return later

## 📢 FINAL LAUNCH CHECKLIST

Before marking 30 hours complete:

### Technical
- ✅ All features working
- ✅ Tests passing (>80% coverage)
- ✅ Performance targets met
- ✅ Security scan passed
- ✅ Documentation complete
- ✅ Docker Compose works
- ✅ Kubernetes manifests ready

### Product
- ✅ Beautiful UI
- ✅ Responsive design
- ✅ Error handling
- ✅ Loading states
- ✅ Empty states

### Marketing
- ✅ README compelling
- ✅ Screenshots added
- ✅ Demo video recorded
- ✅ Blog post drafted
- ✅ Social media posts ready

### Community
- ✅ LICENSE file (Apache 2.0)
- ✅ CONTRIBUTING.md
- ✅ CODE_OF_CONDUCT.md
- ✅ Issue templates
- ✅ PR template

## 🎉 SUCCESS CRITERIA

You will have succeeded when:
1. **User can log in via SSO** in < 30 seconds
2. **User can register an agent** in < 1 minute
3. **Trust score appears immediately** after registration
4. **API key generated and downloadable** securely
5. **Admin can manage users** with full CRUD
6. **Audit logs capture everything** and are searchable
7. **Alerts trigger proactively** for critical issues
8. **UI is polished and responsive** on all devices
9. **API responds in < 100ms** (p95)
10. **Documentation is comprehensive** and clear

## 🚀 READY TO BUILD

You have everything you need:
- ✅ Clear mission and vision
- ✅ Detailed 30-hour plan
- ✅ Complete technology stack
- ✅ Quality standards
- ✅ Success criteria

**Your only job**: Execute the 30-hour build plan methodically, hour by hour, feature by feature, test by test.

**Remember**:
- Quality over speed (but be efficient)
- Test everything (don't skip tests)
- Document as you go (don't defer)
- Commit frequently (preserve progress)
- Stay focused (follow the plan)

---

# 🎯 START COMMAND

When you're ready to start building, say:

**"I'm ready to build OpenA2A Identity. Starting Hour 1: Project Setup & Infrastructure."**

Then proceed through the 30-hour plan, hour by hour, until complete.

**Let's build something incredible.** 🚀

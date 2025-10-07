# 🤝 Contributing to Agent Identity Management

Thank you for your interest in contributing to Agent Identity Management! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Issue Guidelines](#issue-guidelines)

## Code of Conduct

### Our Pledge

We pledge to make participation in our project a harassment-free experience for everyone, regardless of age, body size, disability, ethnicity, gender identity and expression, level of experience, nationality, personal appearance, race, religion, or sexual identity and orientation.

### Our Standards

**Positive behavior includes:**
- Using welcoming and inclusive language
- Being respectful of differing viewpoints
- Gracefully accepting constructive criticism
- Focusing on what is best for the community
- Showing empathy towards other members

**Unacceptable behavior includes:**
- Trolling, insulting/derogatory comments, and personal attacks
- Public or private harassment
- Publishing others' private information without permission
- Other conduct which could reasonably be considered inappropriate

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Docker Compose
- Git
- pnpm 8+

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/identity.git
   cd identity
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/opena2a/identity.git
   ```

### Set Up Development Environment

```bash
# Start infrastructure
docker-compose up -d

# Install dependencies
cd apps/backend && go mod download
cd ../web && pnpm install

# Run database migrations
cd apps/backend
go run cmd/migrate/main.go up

# Start backend
go run cmd/server/main.go

# Start frontend (new terminal)
cd apps/web
pnpm dev
```

## Development Workflow

### 1. Create a Branch

```bash
# Update main
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

**Branch naming conventions:**
- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation updates
- `refactor/description` - Code refactoring
- `test/description` - Test additions/updates
- `chore/description` - Maintenance tasks

### 2. Make Changes

- Write clear, concise code
- Follow coding standards (see below)
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

```bash
# Backend tests
cd apps/backend
go test ./...

# Frontend tests
cd apps/web
pnpm test

# E2E tests
pnpm test:e2e

# Lint
cd apps/backend && golangci-lint run
cd apps/web && pnpm lint
```

### 4. Commit Changes

```bash
# Stage changes
git add .

# Commit with conventional commit message
git commit -m "feat: add agent verification workflow"
```

### 5. Push and Create PR

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create pull request on GitHub
```

## Coding Standards

### Go (Backend)

**Style Guide:**
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use `golangci-lint` for linting

**Naming Conventions:**
```go
// Packages: lowercase, single word
package repository

// Interfaces: noun or adjective
type AgentRepository interface {}

// Structs: PascalCase
type AgentService struct {}

// Functions: camelCase for private, PascalCase for public
func (s *AgentService) CreateAgent() {}
func (s *AgentService) validateInput() {}

// Constants: PascalCase
const MaxRetries = 3
```

**Code Organization:**
```go
// 1. Package declaration
package application

// 2. Imports (grouped: stdlib, external, internal)
import (
    "context"
    "fmt"

    "github.com/google/uuid"

    "github.com/opena2a/identity/backend/internal/domain"
)

// 3. Constants
const DefaultLimit = 50

// 4. Types
type Service struct {}

// 5. Constructor
func New() *Service {}

// 6. Methods
func (s *Service) Method() {}
```

**Error Handling:**
```go
// Always check errors
result, err := operation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use custom errors for domain logic
var ErrAgentNotFound = errors.New("agent not found")
```

**Comments:**
```go
// Public functions must have godoc comments
// CreateAgent creates a new agent in the system.
// It validates the input, generates an ID, and persists to the database.
func CreateAgent(ctx context.Context, req *CreateAgentRequest) (*Agent, error) {
    // Implementation
}
```

### TypeScript/React (Frontend)

**Style Guide:**
- Use TypeScript strict mode
- Follow [Airbnb React Style Guide](https://github.com/airbnb/javascript/tree/master/react)
- Use Prettier for formatting
- Use ESLint for linting

**Naming Conventions:**
```typescript
// Components: PascalCase
function AgentCard() {}

// Hooks: camelCase, prefix with "use"
function useAgents() {}

// Constants: UPPER_SNAKE_CASE
const API_BASE_URL = "http://localhost:8080"

// Types/Interfaces: PascalCase
interface Agent {
  id: string
}

// Variables/Functions: camelCase
const agentList = []
function fetchAgents() {}
```

**Component Structure:**
```typescript
'use client'

// 1. Imports
import { useState } from 'react'
import { Button } from '@/components/ui/button'

// 2. Types
interface AgentCardProps {
  agent: Agent
}

// 3. Component
export default function AgentCard({ agent }: AgentCardProps) {
  // Hooks
  const [loading, setLoading] = useState(false)

  // Handlers
  const handleClick = () => {}

  // Render
  return <div>...</div>
}
```

**Best Practices:**
```typescript
// Use explicit return types
function fetchAgents(): Promise<Agent[]> {
  return api.listAgents()
}

// Use const for components
const AgentList = () => {}

// Destructure props
function AgentCard({ id, name }: AgentCardProps) {}

// Use optional chaining
const email = user?.email ?? 'unknown'
```

### SQL

```sql
-- Use uppercase for keywords
SELECT id, name FROM agents WHERE status = 'verified';

-- Use snake_case for identifiers
CREATE TABLE agent_metadata (
    agent_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL
);

-- Add comments for complex queries
-- Calculate average trust score for verified agents
-- in the last 30 days
SELECT AVG(trust_score)
FROM agents
WHERE status = 'verified'
  AND verified_at >= NOW() - INTERVAL '30 days';
```

## Testing Guidelines

### Backend Tests

**Unit Tests:**
```go
// File: agent_service_test.go
func TestAgentService_CreateAgent(t *testing.T) {
    // Arrange
    service := NewAgentService(mockRepo)
    req := &CreateAgentRequest{
        Name: "test-agent",
    }

    // Act
    agent, err := service.CreateAgent(context.Background(), req)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "test-agent", agent.Name)
}
```

**Table-Driven Tests:**
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"invalid email", "invalid", true},
        {"empty email", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, want error %v", err, tt.wantErr)
            }
        })
    }
}
```

### Frontend Tests

**Component Tests:**
```typescript
// File: AgentCard.test.tsx
import { render, screen } from '@testing-library/react'
import AgentCard from './AgentCard'

describe('AgentCard', () => {
  it('renders agent name', () => {
    const agent = { id: '1', name: 'Test Agent' }
    render(<AgentCard agent={agent} />)
    expect(screen.getByText('Test Agent')).toBeInTheDocument()
  })
})
```

**Integration Tests:**
```typescript
describe('Agent creation flow', () => {
  it('creates agent successfully', async () => {
    // Arrange
    const user = userEvent.setup()
    render(<CreateAgentPage />)

    // Act
    await user.type(screen.getByLabelText('Name'), 'Test Agent')
    await user.click(screen.getByText('Create'))

    // Assert
    expect(await screen.findByText('Agent created')).toBeInTheDocument()
  })
})
```

### Test Coverage

Aim for:
- **Backend**: 80%+ coverage
- **Frontend**: 70%+ coverage
- **Critical paths**: 100% coverage (auth, payments, data mutations)

```bash
# Check coverage
go test -cover ./...
pnpm test -- --coverage
```

## Pull Request Process

### Before Submitting

1. **Rebase on latest main:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all tests:**
   ```bash
   go test ./...
   pnpm test
   ```

3. **Run linters:**
   ```bash
   golangci-lint run
   pnpm lint
   ```

4. **Update documentation** if needed

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Comments added for complex code
- [ ] Documentation updated
- [ ] No new warnings generated
- [ ] Tests pass locally
```

### Review Process

1. **Automated Checks**: CI must pass
2. **Code Review**: At least one approval required
3. **Address Feedback**: Make requested changes
4. **Merge**: Squash and merge or rebase

## Commit Message Guidelines

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test additions/updates
- `chore`: Maintenance tasks
- `perf`: Performance improvements

**Examples:**
```bash
feat(agents): add agent verification workflow

Implement verification workflow with trust score calculation.
Adds new endpoint POST /api/v1/agents/:id/verify

Closes #123

---

fix(auth): handle expired OAuth tokens

Properly refresh tokens when they expire instead of
throwing error to user.

Fixes #456

---

docs(api): update authentication examples

Add examples for all three OAuth providers
```

## Issue Guidelines

### Bug Reports

```markdown
**Describe the bug**
A clear description of the bug

**To Reproduce**
Steps to reproduce:
1. Go to '...'
2. Click on '...'
3. See error

**Expected behavior**
What should happen

**Screenshots**
If applicable

**Environment:**
- OS: [e.g., macOS 13.0]
- Browser: [e.g., Chrome 120]
- Version: [e.g., 1.0.0]

**Additional context**
Any other relevant information
```

### Feature Requests

```markdown
**Is your feature request related to a problem?**
Clear description of the problem

**Describe the solution you'd like**
Clear description of what you want

**Describe alternatives you've considered**
Alternative solutions or features

**Additional context**
Any other relevant information
```

## Development Tips

### Hot Reload

**Backend:**
```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

**Frontend:**
```bash
# Already built-in
pnpm dev
```

### Debugging

**Backend:**
```bash
# Enable debug logging
export LOG_LEVEL=debug
go run cmd/server/main.go
```

**Frontend:**
```typescript
// Use React DevTools
// Add debug logs
console.log('Debug:', data)
```

### Database Changes

```bash
# Create migration
go run cmd/migrate/main.go create add_column_to_agents

# Edit migration files in migrations/
# Then run:
go run cmd/migrate/main.go up
```

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/opena2a/identity/discussions)
- **Bugs**: [GitHub Issues](https://github.com/opena2a/identity/issues)
- **Chat**: [Discord](https://discord.gg/opena2a)

## Recognition

Contributors will be recognized in:
- README.md contributors section
- Release notes
- Monthly contributor highlights

Thank you for contributing! 🎉

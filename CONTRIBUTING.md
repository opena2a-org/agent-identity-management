# Contributing to Agent Identity Management

Thank you for your interest in contributing to the Agent Identity Management (AIM) platform! This document provides guidelines and instructions for contributing to this open-source project.

## 🌟 Ways to Contribute

- **Report Bugs**: Found a bug? Open an issue with detailed reproduction steps
- **Suggest Features**: Have an idea? Open an issue to discuss it
- **Improve Documentation**: Help make our docs clearer and more comprehensive
- **Submit Code**: Fix bugs, implement features, or improve performance
- **Review Pull Requests**: Help review and test community contributions

## 🚀 Getting Started

### Prerequisites

- **Go**: 1.23 or higher
- **Node.js**: 18 or higher
- **PostgreSQL**: 16 or higher
- **Docker**: (optional) for containerized development
- **Git**: for version control

### Setting Up Development Environment

1. **Fork and Clone**
   ```bash
   git clone https://github.com/YOUR_USERNAME/agent-identity-management.git
   cd agent-identity-management
   ```

2. **Install Dependencies**
   ```bash
   # Backend dependencies
   cd apps/backend
   go mod download

   # Frontend dependencies
   cd ../web
   npm install
   ```

3. **Set Up Database**
   ```bash
   # Using Docker Compose (recommended)
   docker compose up -d postgres

   # Run migrations
   cd apps/backend
   go run cmd/server/main.go migrate
   ```

4. **Configure Environment**
   ```bash
   # Backend
   cp apps/backend/.env.example apps/backend/.env
   # Edit .env with your configuration

   # Frontend
   cp apps/web/.env.example apps/web/.env.local
   # Edit .env.local with your configuration
   ```

5. **Run Development Servers**
   ```bash
   # Terminal 1: Backend
   cd apps/backend
   go run cmd/server/main.go

   # Terminal 2: Frontend
   cd apps/web
   npm run dev
   ```

6. **Access the Application**
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - API Documentation: http://localhost:8080/swagger

## 📝 Development Guidelines

### Code Style

**Go Backend**:
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` for formatting
- Run `go vet` and `golint` before committing
- Write meaningful variable and function names
- Add comments for complex logic

**TypeScript/React Frontend**:
- Follow the project's ESLint configuration
- Use TypeScript for all new code
- Follow React best practices and hooks guidelines
- Use functional components over class components
- Keep components small and focused

### Testing Requirements

All contributions must include appropriate tests:

**Backend**:
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

**Frontend**:
```bash
# Run all tests
npm test

# Run with coverage
npm test -- --coverage
```

### Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples**:
```
feat(auth): add OAuth2 support for Microsoft
fix(api): resolve race condition in token refresh
docs(readme): update installation instructions
test(agents): add integration tests for agent registration
```

## 🔄 Pull Request Process

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/bug-description
   ```

2. **Make Your Changes**
   - Write code following our style guidelines
   - Add tests for new functionality
   - Update documentation as needed
   - Ensure all tests pass

3. **Commit Your Changes**
   ```bash
   git add .
   git commit -m "feat(scope): description of changes"
   ```

4. **Push to Your Fork**
   ```bash
   git push origin feature/your-feature-name
   ```

5. **Open a Pull Request**
   - Go to the original repository on GitHub
   - Click "New Pull Request"
   - Select your fork and branch
   - Fill out the PR template with:
     - Clear description of changes
     - Related issue numbers
     - Screenshots (if UI changes)
     - Testing steps

6. **Address Review Feedback**
   - Respond to comments
   - Make requested changes
   - Push updates to your branch

7. **Merge**
   - Once approved, a maintainer will merge your PR
   - Delete your branch after merge

## 🐛 Reporting Bugs

When reporting bugs, please include:

- **AIM Version**: Which version are you using?
- **Environment**: OS, Go version, Node version
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Expected Behavior**: What should happen?
- **Actual Behavior**: What actually happens?
- **Error Messages**: Full error messages and stack traces
- **Screenshots**: If applicable

## 💡 Suggesting Features

When suggesting features:

- **Use Case**: Describe the problem this feature solves
- **Proposed Solution**: Your idea for how to implement it
- **Alternatives**: Other solutions you've considered
- **Additional Context**: Any other relevant information

## 📚 Documentation

Good documentation is crucial! When contributing docs:

- Use clear, concise language
- Include code examples
- Add screenshots for UI features
- Update the DOCUMENTATION_INDEX.md
- Check for spelling and grammar

## 🔒 Security

If you discover a security vulnerability:

1. **DO NOT** open a public issue
2. Email info@opena2a.org with details
3. Wait for confirmation before disclosing publicly

## 📜 Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.

## 📞 Getting Help

- **GitHub Discussions**: For questions and general discussions
- **GitHub Issues**: For bug reports and feature requests
- **Documentation**: Check our [documentation](./docs)
- **Examples**: See [examples](./docs/examples) for usage patterns

## 🏗️ Project Structure

```
agent-identity-management/
├── apps/
│   ├── backend/          # Go backend
│   │   ├── cmd/          # Main applications
│   │   ├── internal/     # Private application code
│   │   ├── migrations/   # Database migrations
│   │   └── tests/        # Backend tests
│   └── web/              # Next.js frontend
│       ├── app/          # App Router pages
│       ├── components/   # React components
│       ├── lib/          # Utilities
│       └── __tests__/    # Frontend tests
├── docs/                 # Documentation
├── infrastructure/       # Deployment configurations
└── tests/                # E2E tests
```

## 🎯 Development Priorities

Current focus areas for contributions:

1. **Testing**: Increase test coverage
2. **Documentation**: Improve user guides and examples
3. **Performance**: Optimize API response times
4. **Security**: Enhance security features
5. **Integrations**: Add third-party integrations

## ✅ Quality Checklist

Before submitting a PR, ensure:

- [ ] Code follows project style guidelines
- [ ] All tests pass locally
- [ ] New code has test coverage
- [ ] Documentation is updated
- [ ] Commit messages follow convention
- [ ] No sensitive data in commits
- [ ] PR description is clear and complete

## 🙏 Thank You!

Your contributions make AIM better for everyone. We appreciate your time and effort in improving this project!

---

**Questions?** Open an issue or start a discussion. We're here to help!

**License**: This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see [LICENSE](LICENSE) for details.

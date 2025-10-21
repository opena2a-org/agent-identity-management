# SDK Publishing Guide - npm, PyPI, and Go Modules

## Overview

This guide provides step-by-step instructions for publishing the AIM SDKs to public package managers. Users can install via package managers for manual configuration, or download pre-configured SDKs with embedded credentials from the AIM dashboard.

**Two Installation Methods**:
1. **Package Manager** (npm, PyPI, Go modules) - Generic SDK, requires manual configuration
2. **AIM Dashboard Download** - Pre-configured with embedded agent credentials

---

## 1. Python SDK - Publishing to PyPI

### Prerequisites
- PyPI account (create at https://pypi.org/account/register/)
- PyPI API token (create at https://pypi.org/manage/account/token/)
- Install build tools: `pip install build twine`

### Package Preparation

**File**: `sdks/python/pyproject.toml`
```toml
[build-system]
requires = ["setuptools>=61.0", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "aim-sdk"
version = "1.0.0"
description = "Enterprise-grade identity verification SDK for AI agents and MCP servers"
readme = "README.md"
authors = [
    { name = "OpenA2A", email = "hello@opena2a.org" }
]
license = { text = "Apache-2.0" }
classifiers = [
    "Development Status :: 4 - Beta",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: Apache Software License",
    "Programming Language :: Python :: 3",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
    "Programming Language :: Python :: 3.11",
    "Programming Language :: Python :: 3.12",
    "Topic :: Software Development :: Libraries :: Python Modules",
    "Topic :: Security",
]
keywords = ["ai", "security", "identity", "agent", "mcp", "verification"]
dependencies = [
    "requests>=2.31.0",
    "cryptography>=41.0.0",
    "keyring>=24.0.0",
    "pydantic>=2.0.0",
]
requires-python = ">=3.9"

[project.urls]
Homepage = "https://github.com/opena2a-org/agent-identity-management"
Documentation = "https://docs.opena2a.org"
Repository = "https://github.com/opena2a-org/agent-identity-management"
"Bug Tracker" = "https://github.com/opena2a-org/agent-identity-management/issues"

[tool.setuptools]
packages = ["aim_sdk"]
```

### Publishing Steps

```bash
# 1. Navigate to Python SDK directory
cd sdks/python

# 2. Clean previous builds
rm -rf dist/ build/ *.egg-info

# 3. Build the package
python -m build

# 4. Check the package (optional but recommended)
twine check dist/*

# 5. Upload to PyPI (production)
twine upload dist/*
# Enter your PyPI API token when prompted

# 6. Verify installation
pip install aim-sdk
python -c "from aim_sdk import secure; print('✅ SDK installed successfully')"
```

### PyPI API Token Configuration (Recommended)
```bash
# Create ~/.pypirc file
cat > ~/.pypirc << 'EOF'
[pypi]
username = __token__
password = pypi-AgEIcHlwaS5vcmc...YOUR_API_TOKEN_HERE

[testpypi]
username = __token__
password = pypi-AgENdGVzdC5weXBpLm9yZw...YOUR_TEST_TOKEN_HERE
EOF

chmod 600 ~/.pypirc
```

### Testing on TestPyPI First (Recommended)
```bash
# Upload to TestPyPI first
twine upload --repository testpypi dist/*

# Test installation from TestPyPI
pip install --index-url https://test.pypi.org/simple/ aim-sdk
```

---

## 2. JavaScript/TypeScript SDK - Publishing to npm

### Prerequisites
- npm account (create at https://www.npmjs.com/signup)
- npm CLI installed: `npm install -g npm`
- Login to npm: `npm login`

### Package Preparation

**File**: `sdks/javascript/package.json`
```json
{
  "name": "@aim/sdk",
  "version": "1.0.0",
  "description": "Enterprise-grade identity verification SDK for AI agents and MCP servers",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "files": [
    "dist",
    "README.md",
    "LICENSE"
  ],
  "scripts": {
    "build": "tsc",
    "prepublishOnly": "npm run build",
    "test": "jest"
  },
  "keywords": [
    "ai",
    "security",
    "identity",
    "agent",
    "mcp",
    "verification",
    "ed25519",
    "cryptography"
  ],
  "author": "OpenA2A <hello@opena2a.org>",
  "license": "Apache-2.0",
  "repository": {
    "type": "git",
    "url": "https://github.com/opena2a-org/agent-identity-management.git",
    "directory": "sdks/javascript"
  },
  "bugs": {
    "url": "https://github.com/opena2a-org/agent-identity-management/issues"
  },
  "homepage": "https://github.com/opena2a-org/agent-identity-management#readme",
  "dependencies": {
    "axios": "^1.6.0",
    "tweetnacl": "^1.0.3",
    "tweetnacl-util": "^0.15.1"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0",
    "jest": "^29.0.0"
  },
  "engines": {
    "node": ">=16.0.0"
  }
}
```

**File**: `sdks/javascript/.npmignore`
```
src/
tsconfig.json
*.test.ts
*.spec.ts
__tests__/
.env
node_modules/
```

### Publishing Steps

```bash
# 1. Navigate to JavaScript SDK directory
cd sdks/javascript

# 2. Install dependencies
npm install

# 3. Build TypeScript to JavaScript
npm run build

# 4. Test the build locally
npm link
cd /tmp/test-project
npm link @aim/sdk
node -e "const { secure } = require('@aim/sdk'); console.log('✅ SDK works');"
cd -
npm unlink -g @aim/sdk

# 5. Check package contents
npm pack --dry-run

# 6. Login to npm (if not already logged in)
npm login

# 7. Publish to npm
npm publish --access public

# 8. Verify installation
npm install @aim/sdk
node -e "const { secure } = require('@aim/sdk'); console.log('✅ SDK installed');"
```

### Scoped Package (@aim/sdk) vs Unscoped (aim-sdk)
- **Scoped** (`@aim/sdk`): Requires organization on npm, looks more professional
- **Unscoped** (`aim-sdk`): Simpler, no organization needed

To create npm organization:
1. Go to https://www.npmjs.com/org/create
2. Create organization named "aim" or "opena2a"
3. Then use `@aim/sdk` or `@opena2a/sdk`

### Publishing Beta/Alpha Versions
```bash
# Publish beta version
npm version prerelease --preid=beta
npm publish --tag beta

# Users install with: npm install @aim/sdk@beta
```

---

## 3. Go SDK - Publishing as Go Module

### Prerequisites
- Git repository with the Go SDK code
- GitHub account (or GitLab, Bitbucket)
- Git tags for versioning

### Go Module Characteristics
- **No central registry** like npm or PyPI
- Uses Git repositories directly
- Versioned with Git tags
- Installed with: `go get github.com/org/repo`

### Package Preparation

**File**: `sdks/go/go.mod`
```go
module github.com/opena2a-org/agent-identity-management/sdks/go

go 1.21

require (
    github.com/google/uuid v1.6.0
    golang.org/x/crypto v0.18.0
)
```

**Verify Package Structure**:
```bash
sdks/go/
├── go.mod
├── go.sum
├── README.md
├── registration.go      # Main package file
├── registration_test.go
└── examples/
    └── basic.go
```

### Publishing Steps

```bash
# 1. Ensure code is pushed to GitHub
cd /Users/decimai/workspace/agent-identity-management
git add sdks/go/
git commit -m "feat: add Go SDK v1.0.0"
git push origin main

# 2. Create and push a version tag
git tag sdks/go/v1.0.0
git push origin sdks/go/v1.0.0

# 3. Verify the module is accessible
go list -m github.com/opena2a-org/agent-identity-management/sdks/go@v1.0.0

# 4. Test installation
cd /tmp
mkdir test-go-sdk
cd test-go-sdk
go mod init test
go get github.com/opena2a-org/agent-identity-management/sdks/go@v1.0.0

# 5. Create test file
cat > main.go << 'EOF'
package main

import (
    "fmt"
    aimsdk "github.com/opena2a-org/agent-identity-management/sdks/go"
)

func main() {
    fmt.Println("✅ AIM Go SDK imported successfully")
}
EOF

go run main.go
```

### Go Module Versioning
- **v1.x.x**: `go get github.com/org/repo/sdks/go@v1.2.3`
- **v2+**: Create `/v2` directory: `github.com/org/repo/sdks/go/v2`

### Making Go SDK Discoverable
1. **Add pkg.go.dev metadata** in `registration.go`:
```go
// Package aimsdk provides enterprise-grade identity verification for AI agents.
//
// AIM SDK enables cryptographic verification, trust scoring, and audit logging
// for AI agents and MCP servers. Secure your agents with Ed25519 signing.
//
// Quick Start:
//
//	client := aimsdk.NewClient()
//	agent, err := client.Secure(ctx, aimsdk.SecureOptions{
//		Name: "my-agent",
//	})
//
// For more examples, see: https://github.com/opena2a-org/agent-identity-management
package aimsdk
```

2. **Submit to pkg.go.dev**: Automatic after first `go get`

---

## 4. User Installation & Configuration (Package Manager Version)

### Python (PyPI)
```bash
# Install
pip install aim-sdk

# Configure with environment variables
export AIM_API_URL="https://api.aim.example.com"
export AIM_AGENT_ID="your-agent-id"
export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

# Or configure programmatically
from aim_sdk import secure

agent = secure(
    name="my-agent",
    api_url="https://api.aim.example.com",
    private_key="your-private-key"  # Optional: reads from env or keyring
)
```

### JavaScript (npm)
```bash
# Install
npm install @aim/sdk

# Configure with environment variables
export AIM_API_URL="https://api.aim.example.com"
export AIM_AGENT_ID="your-agent-id"
export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

# Or configure programmatically
import { secure } from '@aim/sdk';

const agent = secure({
    name: 'my-agent',
    apiUrl: 'https://api.aim.example.com',
    privateKey: process.env.AIM_PRIVATE_KEY
});
```

### Go (Go Modules)
```bash
# Install
go get github.com/opena2a-org/agent-identity-management/sdks/go

# Configure with environment variables
export AIM_API_URL="https://api.aim.example.com"
export AIM_AGENT_ID="your-agent-id"
export AIM_PRIVATE_KEY="your-64-char-hex-private-key"

# Or configure programmatically
import aimsdk "github.com/opena2a-org/agent-identity-management/sdks/go"

client := aimsdk.NewClient()
agent, err := client.Secure(ctx, aimsdk.SecureOptions{
    Name:       "my-agent",
    APIURL:     "https://api.aim.example.com",
    PrivateKey: os.Getenv("AIM_PRIVATE_KEY"),
})
```

---

## 5. Versioning Strategy

### Semantic Versioning (semver)
- **MAJOR.MINOR.PATCH** (e.g., 1.2.3)
- **MAJOR**: Breaking changes (e.g., 1.x.x → 2.0.0)
- **MINOR**: New features, backward compatible (e.g., 1.2.x → 1.3.0)
- **PATCH**: Bug fixes, backward compatible (e.g., 1.2.3 → 1.2.4)

### Initial Release
- Start with **v1.0.0** for stable release
- Use **v0.x.x** for pre-1.0 development (signals "not stable yet")

### Release Process
```bash
# 1. Update version in package files
# Python: pyproject.toml (version = "1.1.0")
# JavaScript: package.json (version: "1.1.0")
# Go: git tag sdks/go/v1.1.0

# 2. Update CHANGELOG.md
# Document what changed in this version

# 3. Commit version bump
git add .
git commit -m "chore: bump version to 1.1.0"
git push

# 4. Create git tag
git tag v1.1.0
git push origin v1.1.0

# 5. Publish to package managers
# (see publishing steps above)
```

---

## 6. Continuous Deployment (Optional)

### Automated Publishing with GitHub Actions

**File**: `.github/workflows/publish-sdks.yml`
```yaml
name: Publish SDKs

on:
  push:
    tags:
      - 'v*.*.*'

jobs:
  publish-python:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          pip install build twine

      - name: Build package
        working-directory: sdks/python
        run: python -m build

      - name: Publish to PyPI
        working-directory: sdks/python
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_API_TOKEN }}
        run: twine upload dist/*

  publish-javascript:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          registry-url: 'https://registry.npmjs.org'

      - name: Install dependencies
        working-directory: sdks/javascript
        run: npm install

      - name: Build
        working-directory: sdks/javascript
        run: npm run build

      - name: Publish to npm
        working-directory: sdks/javascript
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
        run: npm publish --access public

  publish-go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Verify Go module
        working-directory: sdks/go
        run: go mod verify

      - name: Run tests
        working-directory: sdks/go
        run: go test -v ./...

      - name: Tag already pushed - Go SDK is published
        run: echo "✅ Go SDK published via git tag"
```

### Required GitHub Secrets
- `PYPI_API_TOKEN`: PyPI API token
- `NPM_TOKEN`: npm access token

---

## 7. Documentation Updates

### README Badges (Optional but Professional)

**Python**:
```markdown
[![PyPI version](https://badge.fury.io/py/aim-sdk.svg)](https://badge.fury.io/py/aim-sdk)
[![Downloads](https://pepy.tech/badge/aim-sdk)](https://pepy.tech/project/aim-sdk)
```

**JavaScript**:
```markdown
[![npm version](https://badge.fury.io/js/%40aim%2Fsdk.svg)](https://badge.fury.io/js/%40aim%2Fsdk)
[![Downloads](https://img.shields.io/npm/dm/@aim/sdk.svg)](https://npmjs.com/package/@aim/sdk)
```

**Go**:
```markdown
[![Go Reference](https://pkg.go.dev/badge/github.com/opena2a-org/agent-identity-management/sdks/go.svg)](https://pkg.go.dev/github.com/opena2a-org/agent-identity-management/sdks/go)
```

### Update Installation Instructions in README
Add both installation methods:
1. **Quick Start** (AIM Dashboard download with embedded creds)
2. **Package Manager** (npm/PyPI/Go modules with manual config)

---

## 8. Pre-Publishing Checklist

### For All SDKs
- [ ] README.md is comprehensive and up-to-date
- [ ] LICENSE file exists (Apache 2.0)
- [ ] Version numbers are consistent
- [ ] CHANGELOG.md documents changes
- [ ] Tests pass (`pytest`, `npm test`, `go test`)
- [ ] Examples work and are tested
- [ ] Security vulnerabilities checked (`npm audit`, `safety check`)

### Python Specific
- [ ] `pyproject.toml` has correct metadata
- [ ] Package builds successfully: `python -m build`
- [ ] Tested on TestPyPI first
- [ ] All dependencies listed in `dependencies`

### JavaScript Specific
- [ ] `package.json` has correct metadata
- [ ] TypeScript compiles without errors: `npm run build`
- [ ] `.npmignore` excludes source files
- [ ] `files` field in package.json is correct

### Go Specific
- [ ] `go.mod` has correct module path
- [ ] All dependencies vendored or in `go.mod`
- [ ] Package documentation is clear
- [ ] Git tag follows Go versioning (v1.0.0)

---

## 9. Support & Maintenance

### Responding to Issues
- Monitor GitHub issues for bug reports
- Respond to npm/PyPI support requests
- Keep dependencies updated (Dependabot)

### Deprecation Strategy
If you need to deprecate a version:
- **npm**: `npm deprecate @aim/sdk@1.0.0 "Version 1.0.0 is deprecated, use 1.1.0+"`
- **PyPI**: Use "yanked" releases (hidden but still installable)
- **Go**: Update README with deprecation notice

---

## 10. Summary - Quick Publishing Commands

### Python (PyPI)
```bash
cd sdks/python
python -m build
twine upload dist/*
```

### JavaScript (npm)
```bash
cd sdks/javascript
npm install && npm run build
npm publish --access public
```

### Go (Git Tags)
```bash
git tag sdks/go/v1.0.0
git push origin sdks/go/v1.0.0
```

---

## Questions?

Contact the OpenA2A team:
- **GitHub**: https://github.com/opena2a-org/agent-identity-management
- **Email**: hello@opena2a.org
- **Documentation**: https://docs.opena2a.org

---

**Last Updated**: October 12, 2025
**Version**: 1.0.0

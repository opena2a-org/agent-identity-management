# Documentation Directory Structure

This directory contains all project documentation, organized for easy navigation and maintenance.

## Directory Overview

### 📊 `/status/`
**Completion status reports and milestone markers**

Contains markdown files documenting completed features, implementation summaries, and milestone achievements. Files typically follow the pattern `*_COMPLETE.md`.

Examples:
- `AGENT_VERIFICATION_COMPLETE.md` - Agent verification feature completion
- `API_KEY_MANAGEMENT_COMPLETE.md` - API key management completion
- `TRUST_SCORING_API_COMPLETE.md` - Trust scoring API completion

### 📈 `/reports/`
**Test results, security assessments, and analysis reports**

Contains comprehensive test reports, E2E test summaries, security audit results, and performance analysis documents.

Examples:
- `E2E_TEST_SUMMARY.md` - End-to-end testing results
- `SECURITY_TEST_RESULTS.md` - Security audit findings
- `API_TEST_REPORT.md` - API endpoint testing reports

### 📋 `/planning/`
**Project planning, roadmaps, and strategy documents**

Contains high-level planning documents, implementation roadmaps, sprint plans, and strategic vision documents.

Examples:
- `AIM_COMPLETE_IMPLEMENTATION_ROADMAP.md` - Complete project roadmap
- `AIM_VISION.md` - Product vision and strategy
- `30_HOUR_BUILD_PLAN.md` - Sprint planning documents

### 📖 `/guides/`
**Developer guides and reference documentation**

Contains API references, endpoint documentation, developer guides, and "how-to" documentation.

Examples:
- `API_ENDPOINT_SUMMARY.md` - API endpoint reference
- `API_REFERENCE.md` - Comprehensive API documentation
- `CLAUDE_CONTEXT.md` - Claude Code workflow and context

### 🏗️ `/architecture/`
**Architecture decisions and system design documents**

For architecture decision records (ADRs), system design documents, and technical architecture diagrams.

*Currently empty - to be populated with architecture documentation*

### 🧪 `/testing/`
**Testing strategies and test documentation**

For test plans, testing strategies, quality assurance documentation, and testing best practices.

*Currently empty - to be populated with testing documentation*

## Contributing to Documentation

### Where to Put New Documentation

- **Feature completion**: → `/status/`
- **Test results**: → `/reports/`
- **Planning docs**: → `/planning/`
- **API/developer guides**: → `/guides/`
- **Architecture decisions**: → `/architecture/`
- **Testing strategies**: → `/testing/`

### Naming Conventions

- Use SCREAMING_SNAKE_CASE for markdown files (e.g., `FEATURE_NAME_COMPLETE.md`)
- Use descriptive names that clearly indicate content
- Add date suffix for time-sensitive reports (e.g., `SECURITY_AUDIT_2025_10_08.md`)

### Keeping Documentation Updated

- Move completed feature docs to `/status/` when done
- Archive old reports in `/reports/archive/` if needed
- Update planning docs when roadmap changes
- Keep guides in sync with actual implementation

## Quick Navigation

| Looking for... | Go to... |
|----------------|----------|
| What's been completed | `/status/` |
| Test results | `/reports/` |
| Project roadmap | `/planning/` |
| API documentation | `/guides/` |
| System design | `/architecture/` |
| Testing strategy | `/testing/` |

---

**Last Updated**: October 8, 2025
**Project**: Agent Identity Management (AIM)
**Repository**: https://github.com/opena2a-org/agent-identity-management

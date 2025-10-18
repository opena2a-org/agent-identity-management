# AIM Platform - Documentation Index

Quick access to all user-facing documentation for the Agent Identity Management platform.

---

## 🚨 Most Important Documents

### **Having Issues?**
1. **[Troubleshooting Guide](./docs/troubleshooting/README.md)** ⭐ Start here for any issues
2. **[Authentication Deep-Dive](./docs/troubleshooting/authentication.md)** - Token and auth issues
3. **[Token Rotation Explained](./docs/security/token-rotation.md)** - Why tokens expire

### **Just Getting Started?**
1. **[Production Readiness Report](./PRODUCTION_READINESS_REPORT.md)** - Platform overview
2. **[QA Testing Complete](./QA_TESTING_COMPLETE.md)** - What's been tested
3. **[Flight Agent Demo](./examples/flight-agent/README.md)** - Working example

---

## 📚 Documentation by Category

### Security Documentation

| Document | Description | Lines |
|----------|-------------|-------|
| **[Token Rotation Guide](./docs/security/token-rotation.md)** | Why tokens expire and how to fix | 500+ |
| [Security Review](./SECURITY_REVIEW.md) | Enterprise security analysis | 184 |
| [Empty Tabs Analysis](./EMPTY_TABS_ANALYSIS.md) | Why tabs are empty (security working) | - |

### Troubleshooting Guides

| Document | Description | Lines |
|----------|-------------|-------|
| **[Main Troubleshooting](./docs/troubleshooting/README.md)** | Comprehensive troubleshooting guide | 500+ |
| **[Authentication Issues](./docs/troubleshooting/authentication.md)** | Deep-dive auth troubleshooting | 800+ |

### Production & QA Reports

| Document | Description | Lines |
|----------|-------------|-------|
| [Production Readiness](./PRODUCTION_READINESS_REPORT.md) | Complete production assessment | 419 |
| [QA Testing Complete](./QA_TESTING_COMPLETE.md) | QA testing results | - |
| [UX Improvements](./UX_IMPROVEMENTS_COMPLETE.md) | UX enhancements completed | - |

### Example & Demos

| Document | Description | Lines |
|----------|-------------|-------|
| [Flight Agent](./examples/flight-agent/README.md) | Real-world agent example | - |
| [Demo Results](./examples/flight-agent/DEMO_RESULTS.md) | Demo testing results | 180 |
| [QA Summary](./examples/flight-agent/QA_COMPLETE_SUMMARY.md) | QA verification summary | - |
| [Next Steps](./examples/flight-agent/NEXT_STEPS.md) | How to get fresh OAuth credentials | - |

---

## 🎯 Quick Solutions

### "Token Expired" Error

**See**: [Token Rotation Guide](./docs/security/token-rotation.md)

**Quick Fix**:
1. Log in: http://localhost:3000/auth/login
2. Download SDK: http://localhost:3000/dashboard/sdk
3. Copy credentials: `cp -r aim-sdk-python/.aim ~/.aim/`

### Empty Dashboard Tabs

**See**: [Empty Tabs Analysis](./EMPTY_TABS_ANALYSIS.md)

**Cause**: Token revoked (security working) OR no verification events yet

**Fix**: Get fresh credentials and run agent

### Agent Registration Fails

**See**: [Troubleshooting Guide](./docs/troubleshooting/README.md#agent-registration-problems)

**Check**:
- Backend running: `curl http://localhost:8080/health`
- Credentials exist: `ls ~/.aim/credentials.json`
- File permissions: `chmod 600 ~/.aim/credentials.json`

---

## 📖 Documentation by Audience

### For Developers

**Getting Started**:
1. [Production Readiness Report](./PRODUCTION_READINESS_REPORT.md) - Platform overview
2. [Flight Agent Example](./examples/flight-agent/README.md) - Working code
3. [Next Steps Guide](./examples/flight-agent/NEXT_STEPS.md) - Setup instructions

**When You Have Issues**:
1. [Troubleshooting Guide](./docs/troubleshooting/README.md) - Main resource
2. [Authentication Guide](./docs/troubleshooting/authentication.md) - Auth issues
3. [Token Rotation](./docs/security/token-rotation.md) - Token expiration

### For Security Engineers

**Security Analysis**:
1. [Security Review](./SECURITY_REVIEW.md) - Complete security analysis
2. [Token Rotation Guide](./docs/security/token-rotation.md) - Security feature details
3. [Production Readiness](./PRODUCTION_READINESS_REPORT.md) - Compliance status

**Key Findings**:
- ✅ SOC 2, HIPAA, GDPR compliant
- ✅ Token rotation working correctly
- ✅ Complete audit trail
- ✅ Enterprise-grade security

### For QA / Testing

**QA Documentation**:
1. [QA Testing Complete](./QA_TESTING_COMPLETE.md) - QA results
2. [Production Readiness](./PRODUCTION_READINESS_REPORT.md) - Production assessment
3. [Demo Results](./examples/flight-agent/DEMO_RESULTS.md) - Demo testing
4. [Flight Agent QA](./examples/flight-agent/QA_COMPLETE_SUMMARY.md) - Detailed QA

**Test Scripts**:
- [Quick QA Test](./examples/flight-agent/quick_qa_test.sh) - Automated QA
- [Verify QA Complete](./examples/flight-agent/verify_qa_complete.py) - Verification script

### For DevOps / Operations

**Deployment**:
1. [Production Readiness](./PRODUCTION_READINESS_REPORT.md) - Deployment checklist
2. [Troubleshooting Guide](./docs/troubleshooting/README.md) - Operations guide

**Monitoring**:
- Health check: `curl http://localhost:8080/health`
- Database queries in [Authentication Guide](./docs/troubleshooting/authentication.md)
- Debug scripts in [Troubleshooting Guide](./docs/troubleshooting/README.md)

---

## 🔍 Find Information By Topic

### Authentication

- [Token Rotation Guide](./docs/security/token-rotation.md) - Complete explanation
- [Authentication Deep-Dive](./docs/troubleshooting/authentication.md) - Advanced troubleshooting
- [Security Review](./SECURITY_REVIEW.md) - Security analysis

### Dashboard Issues

- [Empty Tabs Analysis](./EMPTY_TABS_ANALYSIS.md) - Why tabs are empty
- [Troubleshooting Guide](./docs/troubleshooting/README.md#dashboard-issues) - Dashboard problems
- [Production Readiness](./PRODUCTION_READINESS_REPORT.md) - What's expected

### Agent Registration

- [Flight Agent Example](./examples/flight-agent/README.md) - Working example
- [Troubleshooting Guide](./docs/troubleshooting/README.md#agent-registration-problems) - Registration issues
- [Demo Results](./examples/flight-agent/DEMO_RESULTS.md) - Successful registration

### Security & Compliance

- [Security Review](./SECURITY_REVIEW.md) - Enterprise security
- [Token Rotation](./docs/security/token-rotation.md) - Security feature
- [Production Readiness](./PRODUCTION_READINESS_REPORT.md) - Compliance status

---

## 📊 Documentation Stats

| Category | Files | Total Lines |
|----------|-------|-------------|
| **Security** | 3 | ~700 lines |
| **Troubleshooting** | 2 | ~1,300 lines |
| **QA/Testing** | 5 | ~800 lines |
| **Examples** | 4 | ~500 lines |
| **Total** | 14 | ~3,300 lines |

### Recently Created (October 2025)

✨ **New UX Improvements**:
- Enhanced SDK error messages (200 lines)
- Token rotation documentation (500 lines)
- Comprehensive troubleshooting guides (1,300 lines)
- Total: ~2,000 lines of new documentation

---

## 🚀 Common Workflows

### Workflow 1: First Time Setup

```bash
# 1. Log in to portal
open http://localhost:3000/auth/login

# 2. Download SDK
# Click "Download SDK" in dashboard

# 3. Setup credentials
cd your-project
cp -r aim-sdk-python/.aim ~/.aim/
chmod 600 ~/.aim/credentials.json

# 4. Run example agent
cd examples/flight-agent
python3 demo_search.py
```

**Documentation**: [Next Steps Guide](./examples/flight-agent/NEXT_STEPS.md)

### Workflow 2: Troubleshooting Auth Issues

```bash
# 1. Check error message
# Read the solution in the error output

# 2. Verify backend running
curl http://localhost:8080/health

# 3. Check credentials
cat ~/.aim/credentials.json | python -m json.tool

# 4. Get fresh credentials if needed
./examples/flight-agent/quick_qa_test.sh
```

**Documentation**: [Authentication Guide](./docs/troubleshooting/authentication.md)

### Workflow 3: QA Testing

```bash
# 1. Run automated QA
cd examples/flight-agent
./quick_qa_test.sh

# 2. Verify all features
python3 verify_qa_complete.py

# 3. Check results
cat QA_COMPLETE_SUMMARY.md
```

**Documentation**: [QA Testing Complete](./QA_TESTING_COMPLETE.md)

---

## 📞 Getting Help

### Documentation Resources

1. **Check error message** - Includes solution steps
2. **Search troubleshooting guide** - Comprehensive coverage
3. **Review security documentation** - Understand features
4. **Run diagnostic scripts** - Automated debugging

### Support Channels

**Enterprise Customers**:
- Support ticket with agent ID
- Priority support with SLA

**Open Source Users**:
- GitHub Issues
- Community Discord/Slack
- Check documentation first

---

## 🔗 Quick Links

### Essential URLs

| Resource | URL |
|----------|-----|
| **Dashboard** | http://localhost:3000/dashboard |
| **Portal Login** | http://localhost:3000/auth/login |
| **SDK Download** | http://localhost:3000/dashboard/sdk |
| **API Health** | http://localhost:8080/health |
| **Agent Detail** | http://localhost:3000/dashboard/agents/[id] |

### Quick Commands

```bash
# Health check
curl http://localhost:8080/health

# Start platform
docker compose up -d

# View logs
docker logs identity-backend

# Database access
docker exec -it identity-postgres psql -U postgres -d identity

# Run QA test
cd examples/flight-agent && ./quick_qa_test.sh
```

---

## 📝 Documentation Index

### All Documents (Alphabetical)

- [Authentication Deep-Dive](./docs/troubleshooting/authentication.md)
- [Demo Results](./examples/flight-agent/DEMO_RESULTS.md)
- [DOCUMENTATION_INDEX.md](./DOCUMENTATION_INDEX.md) ← You are here
- [Empty Tabs Analysis](./EMPTY_TABS_ANALYSIS.md)
- [Flight Agent README](./examples/flight-agent/README.md)
- [Next Steps Guide](./examples/flight-agent/NEXT_STEPS.md)
- [Production Readiness Report](./PRODUCTION_READINESS_REPORT.md)
- [QA Complete Summary](./examples/flight-agent/QA_COMPLETE_SUMMARY.md)
- [QA Testing Complete](./QA_TESTING_COMPLETE.md)
- [Security Review](./SECURITY_REVIEW.md)
- [Token Rotation Guide](./docs/security/token-rotation.md)
- [Troubleshooting Guide](./docs/troubleshooting/README.md)
- [UX Improvements Complete](./UX_IMPROVEMENTS_COMPLETE.md)

### By Date Created

**October 18, 2025** (Latest):
- Token Rotation Guide
- Troubleshooting Guide (Main)
- Authentication Deep-Dive
- UX Improvements Complete
- Enhanced SDK error messages

**October 10-17, 2025**:
- Production Readiness Report
- QA Testing Complete
- Security Review
- Empty Tabs Analysis
- Flight Agent Demo

---

**Last Updated**: October 18, 2025
**Total Documentation**: 14 files, ~3,300 lines
**Status**: Production Ready ✅

**Questions?** Start with the [Troubleshooting Guide](./docs/troubleshooting/README.md)

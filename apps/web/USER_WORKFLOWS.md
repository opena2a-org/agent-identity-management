# AIM Platform - User Workflows & Personas

## Executive Summary

The Agent Identity Management (AIM) platform is a **runtime verification platform** that provides enterprises with complete visibility and control over AI agents and MCP servers. Every action an AI agent attempts must be verified BEFORE execution, ensuring enterprises can trust their AI infrastructure.

This document outlines comprehensive user personas and detailed workflows that demonstrate how different roles interact with AIM to achieve their goals.

---

## User Personas

### 1. Security Administrator (Sarah Chen)

**Role & Background**
- Chief Information Security Officer (CISO) at a Fortune 500 enterprise
- 15+ years experience in enterprise security
- Responsible for all security threats, compliance, and incident response
- Reports directly to CTO/CEO on security posture

**Goals Using AIM**
- Monitor all AI agent activities in real-time
- Detect and respond to security threats immediately
- Ensure compliance with SOC 2, GDPR, and industry regulations
- Generate audit reports for board meetings and regulators
- Maintain zero-trust security posture for AI infrastructure

**Pain Points AIM Solves**
- No visibility into what AI agents are doing
- Cannot detect unauthorized access or data exfiltration
- Manual threat investigation takes hours/days
- Compliance reporting is time-consuming and error-prone
- No runtime verification before agent actions execute

**Key Workflows**
- Monitor Security Dashboard for active threats
- Investigate security incidents and suspend compromised agents
- Generate compliance reports for auditors
- Review verification audit trails
- Configure security policies and alerts

**Tech Proficiency**: Expert
**Daily AIM Usage**: 4-6 hours

---

### 2. DevOps Engineer (Michael Rodriguez)

**Role & Background**
- Senior DevOps Engineer at a fast-growing tech startup
- 8+ years experience with cloud infrastructure and CI/CD
- Manages 50+ AI agents and 20+ MCP servers
- On-call rotation for production incidents

**Goals Using AIM**
- Register and verify new AI agents quickly
- Manage MCP server configurations and health
- Monitor agent performance and trust scores
- Troubleshoot verification failures
- Ensure agents have appropriate permissions

**Pain Points AIM Solves**
- No centralized agent registry or version control
- Cannot verify agent authenticity or integrity
- Difficult to track which agents have access to what
- No visibility into agent performance metrics
- Manual verification processes are slow

**Key Workflows**
- Register new AI agents with required capabilities
- Configure and verify MCP servers
- Monitor agent trust scores and verification status
- Troubleshoot denied verification requests
- Update agent configurations and permissions

**Tech Proficiency**: Expert
**Daily AIM Usage**: 2-4 hours

---

### 3. Compliance Manager (Olivia Thompson)

**Role & Background**
- Compliance and Risk Manager at a regulated financial institution
- 12+ years experience in regulatory compliance
- Ensures adherence to SOX, PCI-DSS, and financial regulations
- Responsible for audit preparation and regulatory reporting

**Goals Using AIM**
- Generate comprehensive audit trails for regulators
- Prove compliance with AI governance policies
- Track all agent activities for forensic analysis
- Export verification logs for long-term retention
- Demonstrate runtime verification controls

**Pain Points AIM Solves**
- No audit trail of AI agent activities
- Cannot prove compliance to regulators
- Difficult to generate reports showing verification controls
- No evidence of runtime security checks
- Manual log aggregation is time-consuming

**Key Workflows**
- Review verification audit trails
- Generate compliance reports (daily/weekly/monthly)
- Export verification logs for archival
- Search and filter historical verifications
- Create custom compliance dashboards

**Tech Proficiency**: Intermediate
**Daily AIM Usage**: 1-2 hours

---

### 4. Data Analyst (James Park)

**Role & Background**
- Senior Data Analyst at a healthcare technology company
- 6+ years experience with data analysis and ML models
- Uses AI agents for data processing and analytics
- Works with sensitive patient health information (PHI)

**Goals Using AIM**
- Use AI agents safely for data analysis
- Understand what permissions agents have
- Verify agents before sharing sensitive data
- Monitor agent behavior for anomalies
- Ensure HIPAA compliance in agent operations

**Pain Points AIM Solves**
- Uncertainty about agent capabilities and permissions
- Fear of data leakage or unauthorized access
- No way to verify agent trustworthiness
- Cannot track what agents did with sensitive data
- Compliance concerns with AI usage

**Key Workflows**
- Check agent trust scores before use
- Review agent verification status and capabilities
- Monitor agent activities when processing sensitive data
- Report suspicious agent behavior
- Request new agent registrations from DevOps

**Tech Proficiency**: Intermediate
**Daily AIM Usage**: 30 minutes - 1 hour

---

### 5. IT Director (Robert Kim)

**Role & Background**
- IT Director at a global enterprise
- 20+ years experience in enterprise IT management
- Oversees all technology operations and strategy
- Budget owner for security and infrastructure tools

**Goals Using AIM**
- Gain visibility into entire AI infrastructure
- Make strategic decisions about AI agent usage
- Ensure ROI on AI investments
- Maintain high availability and performance
- Demonstrate governance to executive leadership

**Pain Points AIM Solves**
- No centralized view of AI agent landscape
- Cannot measure AI agent ROI or effectiveness
- Lack of governance and control over AI
- Difficult to justify AI security investments
- No metrics for executive reporting

**Key Workflows**
- Review executive dashboard for high-level metrics
- Monitor system health and performance
- Analyze agent usage trends and patterns
- Generate executive reports for board meetings
- Make strategic decisions on agent policies

**Tech Proficiency**: Advanced
**Daily AIM Usage**: 30 minutes - 1 hour

---

### 6. MCP Server Developer (Alex Martinez)

**Role & Background**
- Senior Software Engineer at a technology platform company
- 10+ years experience building distributed systems and APIs
- Develops and maintains Model Context Protocol (MCP) servers
- Responsible for cryptographic security and API integrations

**Goals Using AIM**
- Register MCP servers with cryptographic identity verification
- Manage public/private key pairs for secure communication
- Rotate cryptographic keys according to security policies
- Monitor MCP server health and verification status
- Ensure compliance with enterprise security standards

**Pain Points AIM Solves**
- No centralized registry for MCP server identity
- Manual key management is error-prone and insecure
- Difficult to prove cryptographic authenticity
- No automated key rotation workflows
- Lack of verification history and audit trails
- Cannot easily monitor server health and status

**Key Workflows**
- Register new MCP servers with cryptographic public keys
- Rotate public keys every 90 days (security compliance)
- View cryptographic identity details and fingerprints
- Monitor verification status and history
- Download public keys for distribution
- Troubleshoot verification failures

**Tech Proficiency**: Expert
**Daily AIM Usage**: 1-3 hours

---

## Complete User Workflows

### Workflow 1: First-Time Setup & Onboarding (IT Director)

**Starting Point**: User receives AIM invitation email

**Steps**:
1. **Initial Access**
   - Click invitation link in email
   - Redirected to AIM login page (`/login`)
   - See Google OAuth and Microsoft SSO options
   - Click "Sign in with Google"

2. **OAuth Authentication**
   - Redirected to Google OAuth consent screen
   - Grant permissions to AIM platform
   - Redirected back to AIM with auth token
   - Token stored in browser localStorage

3. **Onboarding Tour** (First-time users only)
   - Welcome modal appears: "Welcome to AIM - Runtime Verification for AI"
   - Step 1: Overview of runtime verification concept
   - Step 2: Key features tour (Dashboard, Agents, Security, etc.)
   - Step 3: Quick actions guide
   - Option to skip tour or take it
   - Modal dismissible with "X" or "Get Started" button

4. **Dashboard First Load**
   - Redirected to `/dashboard`
   - See dashboard with initial mock data or empty state
   - System shows "Getting Started" checklist:
     - ✓ Sign in complete
     - ☐ Invite team members
     - ☐ Register first agent
     - ☐ Configure security policies

5. **Invite Team Members**
   - Click "Invite Team" in header or checklist
   - Modal opens: "Invite Team Members"
   - Form fields:
     - Email addresses (comma-separated or multiple inputs)
     - Role selection (Admin, Manager, Member, Viewer)
     - Custom message (optional)
   - Click "Send Invitations"
   - Success notification: "Invitations sent to 3 team members"
   - Checklist updates: ✓ Invite team members

6. **Configure Organization Settings**
   - Navigate to `/dashboard/settings` (click user avatar → Settings)
   - Organization tab selected by default
   - Configure:
     - Organization name
     - Logo upload
     - Default timezone
     - Verification policies
     - Alert preferences
   - Click "Save Settings"
   - Success notification: "Settings saved successfully"

**Success Criteria**:
- User successfully authenticated with OAuth
- Completed onboarding tour (or skipped)
- Invited at least one team member
- Configured basic organization settings
- Ready to use AIM platform

**Error Handling**:
- OAuth failure: Show error message with retry button
- Invitation email failure: Show which emails failed, allow retry
- Settings save failure: Show error, preserve form data, allow retry

**Time to Complete**: 5-10 minutes

---

### Workflow 2: Register New AI Agent (DevOps Engineer)

**Starting Point**: DevOps engineer on Dashboard or Agents page

**Steps**:
1. **Navigate to Agents Registry**
   - Click "Agents" in sidebar navigation
   - Agent Registry page loads (`/dashboard/agents`)
   - See list of existing agents with stats
   - See "Register Agent" button in top-right

2. **Open Registration Modal**
   - Click "Register Agent" button
   - Modal appears: "Register New Agent"
   - Form sections:
     - Basic Information
     - Capabilities & Permissions
     - Security Configuration
     - Advanced Settings

3. **Fill Basic Information**
   - **Agent Name** (required): `claude-code-assistant`
   - **Display Name** (required): `Claude Code Assistant`
   - **Agent Type** (required): Select "AI Agent" or "MCP Server"
   - **Version** (required): `1.0.0`
   - **Description** (optional): "AI-powered code assistant for development"

4. **Configure Capabilities**
   - Select capabilities from predefined list:
     - ☑ File System Access (read/write)
     - ☑ Database Queries (read-only)
     - ☐ Network Requests (external APIs)
     - ☑ Code Execution (sandboxed)
     - ☐ Secret Access (vault/env vars)
   - Each capability shows description and risk level
   - Warning shown for high-risk capabilities

5. **Set Permissions & Scope**
   - **Access Scope**: Select from dropdown
     - Organization-wide
     - Specific teams/departments
     - Specific users only
   - **Resource Limits**:
     - Max API calls per hour: 1000
     - Max data transfer per day: 10GB
     - Allowed IP ranges: 10.0.0.0/8 (optional)
   - **Runtime Policies**:
     - Require approval for: [File deletion, External API calls]
     - Auto-deny: [System file access, Root commands]

6. **Security Configuration**
   - **Cryptographic Verification**: Toggle ON/OFF
   - Upload public key for signature verification (.pem file)
   - Or paste public key directly (textarea)
   - **Trust Score Requirements**:
     - Minimum trust score to execute: 70 (slider 0-100)
     - Auto-suspend if trust score drops below: 50

7. **Review & Submit**
   - Review all configuration in summary panel
   - Warnings highlighted (e.g., "High-risk capability: Network Requests")
   - Click "Register Agent"
   - Loading spinner shows
   - API request: `POST /api/v1/agents`

8. **Registration Success**
   - Modal closes automatically
   - Success notification: "Agent registered successfully!"
   - Page refreshes with new agent in list
   - New agent shows status: "Pending Verification"
   - Agent card highlights with pulse animation

9. **Verify Agent (Optional but Recommended)**
   - Find newly registered agent in list
   - Click "Verify" button next to agent
   - Verification modal appears showing:
     - Agent details
     - Public key fingerprint
     - Verification checklist
   - Click "Verify Now"
   - System performs cryptographic verification
   - Success: Status changes to "Verified" with green checkmark
   - Trust score initializes at 100%

**Success Criteria**:
- Agent successfully registered in database
- Agent appears in registry with correct details
- Agent has initial status (Pending or Verified)
- Agent has unique ID and API key generated
- DevOps engineer can see agent in list

**Error Handling**:
- Invalid form data: Show field-level validation errors in red
- Duplicate agent name: "Agent name already exists. Use a different name."
- Public key invalid: "Invalid public key format. Please upload valid PEM file."
- API failure: "Registration failed. Please try again." with retry button
- Network timeout: Show timeout error with retry button

**Decision Points**:
- Choose AI Agent vs MCP Server type
- Select specific capabilities needed
- Decide on access scope (wide vs restricted)
- Enable/disable cryptographic verification
- Set trust score thresholds

**Time to Complete**: 3-5 minutes

---

### Workflow 3: Monitor Security Threats (Security Admin)

**Starting Point**: Security admin receives alert notification

**Steps**:
1. **Receive Alert Notification**
   - Browser notification appears: "Critical Security Threat Detected"
   - Or email alert: "AIM Alert: Unauthorized Access Attempt"
   - Click notification to open AIM

2. **Navigate to Security Dashboard**
   - Automatically redirected to `/dashboard/security`
   - Or manually click "Security" in sidebar
   - Dashboard loads with real-time threat data

3. **View Active Threats Overview**
   - See threat metrics in stat cards:
     - Total Threats: 127 (+15.2%)
     - Active Threats: 8 (-12.5%)
     - Critical Incidents: 2 (+5.1%)
     - Anomalies Detected: 45 (+8.3%)
   - See threat trend chart (last 7 days)
   - See severity distribution chart

4. **Review Recent Threats Table**
   - Table shows latest threats with columns:
     - Threat Type
     - Agent Name
     - Severity (Critical/High/Medium/Low badges)
     - Status (Active/Mitigated/Resolved badges)
     - Detected At
     - Actions
   - Critical threat highlighted in red:
     - "Unauthorized Access Attempt"
     - Agent: "Security Monitor"
     - Severity: Critical
     - Status: Active
     - Detected: "2 minutes ago"

5. **Investigate Threat Details**
   - Click "View" (eye icon) on critical threat
   - Threat detail modal opens with tabs:
     - **Overview**: Threat description, timeline, affected resources
     - **Agent Context**: Agent details, trust score, recent activities
     - **Evidence**: Logs, verification records, network traces
     - **Recommendations**: Suggested actions, similar past incidents

6. **Analyze Threat Evidence**
   - Review in "Evidence" tab:
     - Failed authentication logs (10+ attempts)
     - Source IP: 192.168.1.100 (suspicious)
     - Target resources: /api/v1/admin/users
     - Payload analysis: SQL injection pattern detected
     - Related verifications: 3 denied in last 5 minutes

7. **Take Immediate Action**
   - Click "Suspend Agent" button in modal
   - Confirmation dialog: "Are you sure you want to suspend this agent?"
   - Warning: "This will immediately block all agent activities"
   - Click "Confirm Suspension"
   - API request: `POST /api/v1/agents/{id}/suspend`
   - Agent status changes to "Suspended"
   - Success notification: "Agent suspended successfully"

8. **Create Incident Report**
   - In threat detail modal, click "Create Incident"
   - Incident creation form appears:
     - **Title**: Auto-populated from threat type
     - **Severity**: Pre-selected from threat severity
     - **Description**: Threat details auto-filled, editable
     - **Affected Systems**: Auto-detected, can add more
     - **Assigned To**: Select team member (dropdown)
     - **Tags**: Add tags (e.g., "sql-injection", "auth-failure")
   - Click "Create Incident"
   - Incident created with unique ID: INC-2025-001
   - Notification sent to assigned team member

9. **Review Related Verifications**
   - Navigate to "Verifications" tab in threat modal
   - See all verifications from this agent in last 24h
   - Filter by status: Denied
   - See pattern: All denied verifications to admin endpoints
   - Export verifications as CSV for analysis
   - Click "Export" → Select date range → Download CSV

10. **Update Threat Status**
    - After investigation complete, return to threat detail
    - Click "Update Status" dropdown
    - Select "Mitigated" (threat contained, monitoring)
    - Add notes: "Agent suspended, incident created, team notified"
    - Click "Save"
    - Threat status updates in dashboard
    - Active threats count decrements

11. **Set Up Monitoring**
    - Click "Set Alert Rule" in threat modal
    - Configure new alert rule:
      - **Condition**: Failed auth attempts > 5 in 5 minutes
      - **From Agent**: Any agent or specific agent
      - **Action**: Suspend agent + Notify security team
      - **Severity**: Critical
    - Click "Create Alert Rule"
    - Future threats auto-detected and actioned

**Success Criteria**:
- Threat identified and investigated
- Compromised agent suspended immediately
- Incident report created and assigned
- Related verifications reviewed and exported
- Alert rule configured for future prevention
- No data breach or unauthorized access occurred

**Error Handling**:
- Cannot suspend agent: "Insufficient permissions" → Contact admin
- Incident creation fails: "Failed to create incident" → Retry or create manually
- Export fails: "Export failed. Try reducing date range." → Adjust filters
- Alert rule invalid: Field-level validation errors shown

**Decision Points**:
- Suspend agent immediately vs investigate further?
- Create incident vs mark as false positive?
- Assign to specific team member or escalate?
- Set automated response rules or manual review?

**Time to Complete**: 5-15 minutes (depending on severity)

---

### Workflow 4: Runtime Verification Flow (System/Agent Perspective)

**Context**: This workflow shows what happens when an AI agent attempts an action

**Starting Point**: AI agent needs to access a file

**Steps - Agent Perspective**:
1. **Agent Initiates Action**
   - AI agent (Claude Code Assistant) receives user command: "Read config.json"
   - Agent prepares to execute file read operation
   - Before executing, agent calls AIM verification API
   - API request: `POST /api/v1/verify/action`
   - Request body:
     ```json
     {
       "agent_id": "agt_001",
       "action_type": "file_read",
       "resource": "/app/config/config.json",
       "context": {
         "user_id": "usr_123",
         "session_id": "sess_456",
         "timestamp": "2025-01-20T16:30:00Z"
       }
     }
     ```

2. **AIM Receives Verification Request**
   - Request arrives at verification service
   - Service extracts agent_id and looks up agent in registry
   - Checks agent status: Must be "Verified" or "Active"
   - If suspended/revoked: Immediate denial

3. **Capability Check**
   - System checks agent's registered capabilities
   - Agent "agt_001" has capability: "File System Access (read)"
   - Action "file_read" matches capability: ✓ Pass
   - If capability missing: Deny with reason "Missing capability"

4. **Scope & Permission Validation**
   - Check if resource is within agent's allowed scope
   - Agent scope: `/app/` directory tree (configured during registration)
   - Requested file: `/app/config/config.json` → Within scope ✓
   - If outside scope: Deny with reason "Resource out of scope"

5. **Policy Enforcement**
   - Check organization runtime policies
   - Policy: "Sensitive files require approval"
   - Check if config.json marked as sensitive: Yes
   - Auto-approval conditions:
     - User is admin: No (user is developer)
     - Read-only operation: Yes ✓
     - Working hours: Yes ✓
   - Result: Auto-approved (read-only + working hours)

6. **Trust Score Evaluation**
   - Retrieve agent's current trust score: 95%
   - Check minimum required: 70%
   - Trust score sufficient: ✓ Pass
   - If trust score too low: Deny with reason "Trust score below threshold"

7. **Anomaly Detection** (Real-time ML)
   - Compare request pattern with agent's normal behavior
   - Normal pattern: Agent reads config files 2-3 times/day
   - Current request: 1st time today
   - Anomaly score: 12% (low, normal)
   - Threshold: 80% (high confidence = anomaly)
   - Result: No anomaly detected ✓

8. **Cryptographic Verification** (if enabled)
   - Check if agent signed the request
   - Extract signature from request header
   - Retrieve agent's public key from registry
   - Verify signature: `verify(signature, payload, public_key)`
   - Signature valid: ✓ Pass
   - If signature invalid: Deny with reason "Invalid signature"

9. **Rate Limiting**
   - Check agent's API call quota
   - Configured limit: 1000 calls/hour
   - Current usage: 47 calls this hour
   - Within limit: ✓ Pass
   - If exceeded: Deny with reason "Rate limit exceeded"

10. **Generate Verification Decision**
    - All checks passed: APPROVE
    - Create verification record in database:
      ```json
      {
        "id": "ver_789",
        "agent_id": "agt_001",
        "action_type": "file_read",
        "resource": "/app/config/config.json",
        "status": "approved",
        "duration_ms": 42,
        "checks_passed": [
          "agent_status", "capability", "scope",
          "policy", "trust_score", "anomaly",
          "signature", "rate_limit"
        ],
        "timestamp": "2025-01-20T16:30:00Z"
      }
      ```
    - Return to agent: `200 OK { "approved": true, "verification_id": "ver_789" }`

11. **Agent Executes Action**
    - Agent receives approval from AIM
    - Agent executes: `fs.readFile('/app/config/config.json')`
    - Action completes successfully
    - Agent reports back to AIM: `POST /api/v1/verify/ver_789/complete`
    - Body: `{ "status": "success", "duration_ms": 15 }`

12. **Audit Trail Updated**
    - Verification record marked as completed
    - Audit log entry created with full context
    - Trust score updated (maintained at 95% for normal behavior)
    - Metrics updated (successful verification count +1)

**Steps - Admin Perspective** (Viewing in Real-time):

1. **Admin Monitoring Dashboard**
   - Security admin on Security Dashboard (`/dashboard/security`)
   - Real-time verification stream shown in "Live Activity" panel
   - New entry appears: "Claude Code Assistant - File Read Request"

2. **View Verification Details**
   - Click on verification in live stream
   - Modal shows verification details:
     - Agent: Claude Code Assistant
     - Action: Read /app/config/config.json
     - Status: Approved ✓
     - Duration: 42ms
     - All checks: ✓ Passed
     - Trust score impact: No change (95%)

3. **Review in Verifications Page**
   - Navigate to `/dashboard/verifications`
   - See verification in table (top row, most recent)
   - Can filter by agent, date, status
   - Can export for audit purposes

**Success Criteria**:
- Agent action verified BEFORE execution
- All security checks passed
- Verification completed in <100ms
- Audit trail created for compliance
- No unauthorized access occurred
- Trust score maintained/updated appropriately

**Denial Scenario** (Alternative Path):

1. Agent attempts: Delete `/app/config/backup.json`
2. AIM checks: Delete requires "File System Access (write)" capability
3. Agent has only "File System Access (read)"
4. AIM denies: `403 Forbidden { "approved": false, "reason": "Missing capability: file_delete" }`
5. Agent does NOT execute action
6. User notified: "Action denied: Insufficient permissions"
7. Verification record created with status: "denied"
8. Security admin alerted if repeated denial attempts

**Time to Complete**: <100ms (automated, real-time)

---

### Workflow 5: Review Audit Trail & Generate Report (Compliance Manager)

**Starting Point**: Compliance manager preparing for quarterly audit

**Steps**:
1. **Navigate to Verifications Page**
   - Click "Verifications" in sidebar
   - Verifications page loads (`/dashboard/verifications`)
   - See comprehensive verification history

2. **Set Date Range Filter**
   - Click date range dropdown (currently "Last 24 Hours")
   - Select "Custom Range"
   - Date picker modal appears
   - Select start date: January 1, 2025
   - Select end date: March 31, 2025 (Q1)
   - Click "Apply"
   - Table refreshes with Q1 verifications

3. **Apply Additional Filters**
   - Status filter: Select "All Status" (include approved, denied, pending)
   - Agent filter: Type to search or select specific agents
   - Action type filter: Select multiple (File Access, Database Query, API Call)
   - Apply filters
   - Results count updates: "Showing 15,432 verifications"

4. **Review Verification Statistics**
   - View stat cards above table:
     - Total Verifications (Q1): 15,432 (+18.2%)
     - Success Rate: 97% (+3.1%)
     - Denied: 462 (-12.5%)
     - Avg Response Time: 45ms (-8.3%)
   - Review verification trend chart for Q1
   - Identify patterns or anomalies

5. **Search Specific Verifications**
   - Use search bar: "database query"
   - Table filters to show only database-related verifications
   - See results: 2,341 database query verifications
   - Can click into any verification for details

6. **View Individual Verification Details**
   - Click "View" (eye icon) on a verification
   - Detail modal shows:
     - **Overview**: Verification ID, timestamp, status, duration
     - **Agent Details**: Name, ID, version, trust score at time
     - **Action Context**: What was requested, which resource
     - **Security Checks**: All checks performed and results
     - **Decision Reasoning**: Why approved/denied
     - **Related Events**: Other verifications in same session
     - **Compliance Tags**: Auto-tagged based on action type

7. **Export Verification Logs**
   - Click "Export" button above table
   - Export modal appears with options:
     - **Format**: CSV, JSON, PDF Report
     - **Date Range**: Q1 2025 (pre-filled)
     - **Filters**: Current filters applied (optional to include)
     - **Include**: Checkboxes for:
       - ☑ Verification details
       - ☑ Agent information
       - ☑ Security check results
       - ☑ Compliance tags
       - ☐ Raw request/response (optional)
   - Click "Export as CSV"
   - Download starts: `aim-verifications-q1-2025.csv`

8. **Generate Compliance Report**
   - Click "Generate Report" button
   - Report configuration wizard appears:

   **Step 1: Report Type**
   - Select report type:
     - ○ Quarterly Compliance Report ✓
     - ○ Security Audit Report
     - ○ Agent Activity Summary
     - ○ Custom Report
   - Click "Next"

   **Step 2: Report Scope**
   - Date range: Q1 2025 (auto-filled)
   - Include sections:
     - ☑ Executive Summary
     - ☑ Verification Statistics
     - ☑ Security Incidents
     - ☑ Agent Registry Status
     - ☑ Compliance Adherence
     - ☑ Recommendations
   - Click "Next"

   **Step 3: Report Format**
   - Output format: PDF (professional layout)
   - Branding: Include company logo ✓
   - Sign report: Digital signature ✓
   - Click "Generate Report"

9. **Report Generation**
   - Progress indicator shows: "Generating report... 45%"
   - System compiles:
     - Verification statistics
     - Security incident summaries
     - Agent status overview
     - Compliance metrics
     - Charts and graphs
   - Progress: 100% Complete
   - Success notification: "Report generated successfully!"

10. **Review Generated Report**
    - Report preview modal opens
    - PDF preview shows:
      - **Cover Page**: "Q1 2025 Compliance Report - AIM Platform"
      - **Executive Summary**: High-level findings
      - **Verification Metrics**:
        - Total: 15,432 verifications
        - Success rate: 97%
        - Average response time: 45ms
        - Zero breaches detected
      - **Security Incidents**:
        - 3 critical incidents (all resolved)
        - 12 high-severity threats (all mitigated)
      - **Compliance Status**:
        - SOC 2: ✓ Compliant
        - GDPR: ✓ Compliant
        - HIPAA: ✓ Compliant (healthcare org)
      - **Agent Registry**:
        - 834 registered agents
        - 812 verified (97%)
        - 22 pending verification
      - **Recommendations**:
        - Increase verification threshold for external API calls
        - Implement additional MFA for admin agents
        - Review and rotate API keys quarterly

11. **Download & Share Report**
    - Click "Download PDF"
    - Report downloads: `AIM-Q1-2025-Compliance-Report.pdf`
    - Click "Share Report"
    - Share modal appears:
      - **Recipients**: Add email addresses
      - **Message**: Optional cover message
      - **Access**: View only (no download) or Full access
      - **Expiration**: 30 days (configurable)
    - Click "Send"
    - Report shared with audit committee and regulators

12. **Schedule Recurring Reports**
    - Click "Schedule Reports" in header
    - Scheduling modal appears:
    - **Report Type**: Quarterly Compliance Report
    - **Frequency**: Quarterly (every 3 months)
    - **Recipients**: audit@company.com, compliance@company.com
    - **Delivery**: Email PDF + Upload to cloud storage
    - **Next Run**: April 1, 2025
    - Click "Save Schedule"
    - Success: "Report scheduled successfully"

**Success Criteria**:
- Successfully filtered Q1 verifications
- Reviewed detailed verification logs
- Exported raw data in CSV format
- Generated professional compliance report
- Report includes all required compliance sections
- Report shared with relevant stakeholders
- Recurring reports scheduled for future quarters
- Demonstrates complete audit trail and compliance

**Error Handling**:
- No verifications found: "No verifications match your filters. Try adjusting date range."
- Export fails: "Export failed. Data set too large. Try smaller date range."
- Report generation fails: "Report generation failed. Please try again or contact support."
- Share fails: "Failed to send report. Check email addresses and try again."

**Decision Points**:
- Choose date range for audit period
- Select which filters to apply
- Decide on export format (CSV vs JSON vs PDF)
- Choose report type and sections to include
- Determine who to share report with
- Schedule future reports or one-time only

**Time to Complete**: 10-20 minutes

---

### Workflow 6: Manage MCP Servers (DevOps Engineer)

**Starting Point**: DevOps engineer needs to add new MCP server

**Steps**:
1. **Navigate to MCP Servers Page**
   - Click "MCP Servers" in sidebar
   - MCP Servers page loads (`/dashboard/mcp`)
   - See existing MCP servers with status

2. **Review Current MCP Servers**
   - See stat cards:
     - Total MCP Servers: 6
     - Active Servers: 4
     - Verified: 4
     - Last Verification: 2h ago
   - Review servers table with columns:
     - Name, URL, Status, Verification Status, Last Verified, Actions

3. **Click Register MCP Server**
   - Click "Register MCP Server" button (top-right)
   - Registration modal appears: "Register MCP Server"
   - Form sections:
     - Basic Information
     - Connection Details
     - Security Configuration
     - Verification Settings

4. **Fill MCP Server Details**
   - **Basic Information**:
     - Name (required): `AWS Lambda Connector`
     - Display Name (required): `AWS Lambda Connector MCP`
     - Description: "MCP server for AWS Lambda function invocations"
     - Version: `1.0.0`

   - **Connection Details**:
     - Server URL (required): `https://mcp.company.com/aws-lambda`
     - Protocol: MCP v1.0 (dropdown)
     - Port: 443 (auto-filled for https)
     - Health Check Endpoint: `/health`
     - Timeout (seconds): 30

5. **Configure Security**
   - **Authentication Method**:
     - ○ API Key
     - ○ OAuth 2.0
     - ○ Mutual TLS ✓ (selected)
     - ○ HMAC Signature

   - **For Mutual TLS**:
     - Upload Client Certificate (.pem file)
     - Upload Private Key (.key file)
     - CA Certificate (optional, for self-signed)
     - Verify certificate chain: ✓ Enabled

   - **Access Control**:
     - Allowed IP Ranges: `10.0.0.0/8, 192.168.1.0/24`
     - Allowed Agents: Select specific agents or "All Verified Agents"
     - Rate Limit: 100 requests/minute

6. **Verification Configuration**
   - **Verification Method**:
     - ☑ Cryptographic signature verification
     - ☑ TLS certificate validation
     - ☑ Health check endpoint verification
     - ☑ Capability enumeration

   - **Verification Schedule**:
     - Initial verification: Immediate ✓
     - Re-verification interval: Every 24 hours
     - Auto-suspend on failure: ✓ Enabled
     - Notification on verification failure: ✓ Enabled

7. **Review & Submit**
   - Review summary of configuration
   - Warnings/notices shown:
     - ⚠ "Mutual TLS requires valid certificates"
     - ℹ "Health check will run every 5 minutes"
   - Click "Register MCP Server"
   - Loading indicator shows

8. **Registration & Initial Verification**
   - API request: `POST /api/v1/mcp-servers`
   - Server registered with status: "Pending Verification"
   - Immediate verification starts automatically:
     - Step 1: Connect to server URL ✓
     - Step 2: Verify TLS certificate ✓
     - Step 3: Check health endpoint ✓
     - Step 4: Enumerate capabilities ✓
     - Step 5: Validate cryptographic signature ✓
   - Verification complete in 3.2 seconds
   - Status updates to "Verified" with green checkmark

9. **View Server Details**
   - Modal closes, table refreshes
   - New server appears in list
   - Click server name to view details
   - Detail modal shows:
     - **Overview**: Status, uptime, last health check
     - **Capabilities**: List of server capabilities
     - **Certificates**: Certificate details, expiration dates
     - **Verification History**: Past verification results
     - **Connected Agents**: Which agents use this server
     - **Metrics**: Request count, success rate, avg latency

10. **Configure Server Capabilities**
    - In server details modal, click "Edit Capabilities"
    - Capability configuration form:
      - **Available Capabilities** (from server):
        - ☑ Lambda Invocation (sync/async)
        - ☑ Function Listing
        - ☐ Function Deployment (disabled for security)
        - ☑ Log Retrieval
        - ☐ Environment Variable Access (disabled)
      - **Permission Level** per capability:
        - Lambda Invocation: Requires approval
        - Function Listing: Auto-approved
        - Log Retrieval: Auto-approved
    - Click "Update Capabilities"
    - Capabilities updated and propagated to agents

11. **Test Server Connection**
    - Click "Test Connection" button in server details
    - Test modal appears with live progress:
      - Connecting... ✓
      - Authenticating... ✓
      - Verifying TLS... ✓
      - Health check... ✓
      - Capability test... ✓
    - Test results shown:
      - Response time: 124ms
      - All tests passed ✓
      - Server is operational
    - Click "Close"

12. **Monitor Server Health**
    - Return to MCP Servers list
    - See server with "Active" status and "Verified" badge
    - Health indicator: Green dot (healthy)
    - Last verified: "5 minutes ago"
    - Click "View Logs" to see connection logs
    - Can set up alerts for server failures

**Success Criteria**:
- MCP server successfully registered
- Cryptographic verification passed
- Server showing "Active" and "Verified" status
- Capabilities configured and enforced
- Health checks running automatically
- Ready for agent connections

**Error Handling**:
- Invalid URL: "Cannot reach server. Check URL and network connectivity."
- Certificate invalid: "TLS certificate validation failed. Upload valid certificate."
- Health check fails: "Health endpoint returned error. Check server status."
- Signature verification fails: "Cryptographic signature invalid. Check public key."
- Connection timeout: "Server connection timed out. Increase timeout or check server."

**Decision Points**:
- Choose authentication method (API Key, OAuth, mTLS, HMAC)
- Select which capabilities to enable/disable
- Configure approval requirements per capability
- Set verification interval (hourly, daily, weekly)
- Enable/disable auto-suspension on failures

**Time to Complete**: 5-10 minutes

---

### Workflow 7: Respond to Security Incident (Security Admin)

**Starting Point**: Critical security alert received

**Steps**:
1. **Receive Critical Alert**
   - Email alert: "CRITICAL: Data Exfiltration Attempt Detected"
   - SMS alert: "AIM CRITICAL ALERT: Unusual data transfer detected"
   - Browser notification: "Critical Security Incident - Immediate Action Required"
   - Slack/Teams notification: "@security-team - CRITICAL incident detected"

2. **Emergency Access**
   - Click email alert link → Auto-login to AIM (SSO)
   - Redirected to incident detail page
   - Or manually: Click notification → Login → Security Dashboard

3. **View Incident Dashboard**
   - Security Dashboard shows critical banner:
     - 🚨 "CRITICAL INCIDENT IN PROGRESS"
     - "Data Exfiltration Risk Detected - Agent: Workflow Automation"
     - "Detected: 2 minutes ago"
     - "Status: ACTIVE"
   - Incident highlighted in red at top of threats table

4. **Open Incident Details**
   - Click incident to open detail modal
   - **Incident Header**:
     - ID: INC-2025-042
     - Severity: CRITICAL (red badge)
     - Status: ACTIVE (pulsing red)
     - Created: 2 minutes ago
     - Assigned: Unassigned

5. **Review Incident Evidence**
   - **Overview Tab**:
     - Threat Type: Data Exfiltration Risk
     - Agent: Workflow Automation (agt_005)
     - Description: "Large volume data transfer to external endpoint detected"
     - Risk Level: CRITICAL - Potential data breach
     - Affected Resources: Customer database, Payment records
     - Detection Method: Anomaly detection (ML-based)

   - **Evidence Tab**:
     - Suspicious Activity Timeline:
       - 14:45:00 - Agent requested database query (100K records)
       - 14:45:32 - Query approved (within normal scope)
       - 14:46:15 - Large data export initiated (2.3GB)
       - 14:46:45 - External API call to unknown endpoint
       - 14:47:02 - Data transfer detected to 203.0.113.45
       - 14:47:30 - Anomaly alert triggered
       - 14:47:45 - Security team notified

   - **Network Analysis**:
     - Destination IP: 203.0.113.45 (Unknown, not whitelisted)
     - Domain: data-collector[.]suspicious[.]com
     - Data transferred: 2.3GB (estimated)
     - Transfer rate: 50MB/s (unusually high)
     - Protocol: HTTPS (encrypted)
     - Threat Intelligence: IP flagged in threat feeds

6. **Immediate Containment**
   - Click "EMERGENCY SUSPEND" button
   - Confirmation: "CRITICAL ACTION: Suspend Agent Immediately?"
   - Warning: "This will terminate all active operations"
   - Checkboxes:
     - ☑ Suspend agent immediately
     - ☑ Block all network access
     - ☑ Revoke active sessions
     - ☑ Alert all administrators
   - Type "CONFIRM" to proceed (safety check)
   - Click "SUSPEND NOW"
   - Agent suspended in <1 second
   - Notification: "Agent suspended. All operations terminated."

7. **Assess Data Breach Scope**
   - Click "Analyze Impact" button
   - Impact analysis runs automatically:
     - **Data Accessed**:
       - Customer table: 100,000 records queried
       - Payment table: 50,000 records queried
       - User table: 25,000 records accessed
     - **Sensitive Fields**:
       - PII: Names, emails, addresses
       - Payment: Credit card numbers (last 4 digits)
       - Auth: Hashed passwords (not plaintext)
     - **Regulatory Impact**:
       - GDPR: 175,000 EU data subjects affected
       - PCI-DSS: 50,000 payment records accessed
       - CCPA: 80,000 California residents affected
   - Breach severity: HIGH (requires regulatory notification)

8. **Investigate Root Cause**
   - Click "Forensic Analysis" tab
   - System analyzes:
     - **Agent Behavior Pattern**:
       - Normal: 5-10 queries/day, avg 100 records
       - Today: 1 query, 100K records (anomaly score: 98%)
       - Trust score before incident: 90%
       - Trust score after detection: 12% (dropped)

     - **Attack Vector Analysis**:
       - Agent credentials: Valid, not compromised
       - Request source: Internal IP (VPN connection)
       - User session: Legitimate user session token
       - Conclusion: Compromised user account or social engineering

   - **Related Incidents**:
     - Similar incident: INC-2025-015 (3 weeks ago)
     - Same agent, different user account
     - Pattern: Compromised accounts, not agent itself

9. **Initiate Incident Response**
   - Click "Create Response Plan"
   - Automated response plan generated:
     - **Immediate Actions** (0-1 hour):
       - ✓ Suspend compromised agent
       - ☐ Revoke all API keys for this agent
       - ☐ Force password reset for affected user
       - ☐ Block suspicious IP address
       - ☐ Isolate affected systems

     - **Short-term Actions** (1-24 hours):
       - ☐ Notify legal team (data breach)
       - ☐ Notify affected customers
       - ☐ Contact regulatory authorities (72h GDPR deadline)
       - ☐ Preserve forensic evidence
       - ☐ Review similar agent configurations

     - **Long-term Actions** (1-7 days):
       - ☐ Implement additional MFA for all agents
       - ☐ Review and tighten data access policies
       - ☐ Conduct security awareness training
       - ☐ Update incident response procedures

10. **Execute Response Actions**
    - Check off each action as completed:
    - ✓ Suspend agent (already done)
    - Click "Revoke API Keys" → All keys revoked
    - Click "Force Password Reset" → User notified, must reset
    - Click "Block IP" → IP added to firewall blocklist
    - Click "Isolate Systems" → Affected databases taken offline

11. **Notify Stakeholders**
    - Click "Notify Stakeholders"
    - Notification wizard:
      - **Internal Notifications**:
        - ☑ Executive team (CEO, CTO, CISO)
        - ☑ Legal department
        - ☑ IT security team
        - ☑ Data privacy officer
        - Template: Critical incident briefing

      - **External Notifications**:
        - ☐ Customers (175,000 affected) - Draft ready
        - ☐ Regulatory authorities (GDPR, PCI-DSS)
        - ☐ Cyber insurance provider
        - ☐ Law enforcement (if required)

    - Click "Send Internal Notifications" → Sent immediately
    - External notifications: Review and approve before sending

12. **Document Incident**
    - Click "Generate Incident Report"
    - Report auto-populated with:
      - Incident timeline (minute-by-minute)
      - Evidence collected (logs, network traces)
      - Actions taken (suspension, containment)
      - Impact assessment (data, regulatory, financial)
      - Root cause analysis
      - Recommendations
    - Review and edit report
    - Add manual notes and observations
    - Click "Finalize Report"
    - Report saved: `INC-2025-042-Final-Report.pdf`

13. **Post-Incident Actions**
    - Update incident status: "Contained" → "Investigating"
    - Schedule post-incident review meeting
    - Create follow-up tasks:
      - Implement improved anomaly detection rules
      - Add data exfiltration prevention (DLP) controls
      - Review all agents with similar patterns
      - Update security policies
    - Monitor for related incidents

14. **Close Incident**
    - After investigation complete (days/weeks later)
    - Return to incident detail
    - Click "Close Incident"
    - Closure form:
      - Final status: Resolved
      - Resolution: "Compromised user account, not agent vulnerability"
      - Lessons learned: "Implement stricter user authentication"
      - Preventive measures: "Added MFA, tighter data policies"
    - Click "Close Incident"
    - Incident closed, archived for compliance

**Success Criteria**:
- Incident detected within minutes
- Agent suspended immediately (<1 second)
- Data breach scope assessed accurately
- Root cause identified (compromised account)
- Regulatory notifications completed on time
- All affected systems isolated
- Comprehensive incident report created
- No further data exfiltration
- Preventive measures implemented

**Error Handling**:
- Suspend fails: "Manual intervention required. Contact infrastructure team."
- Impact analysis fails: "Unable to assess impact. Review logs manually."
- Notification fails: "Failed to send notifications. Retry or manual contact."
- Evidence collection incomplete: "Some logs unavailable. Check backup systems."

**Decision Points**:
- Suspend agent immediately vs investigate first?
- Notify customers immediately or wait for investigation?
- Contact law enforcement or handle internally?
- Public disclosure or confidential handling?
- Temporary suspension or permanent revocation?

**Time to Complete**:
- Emergency response: 5-15 minutes
- Full investigation: Hours to days
- Incident closure: Days to weeks

---

### Workflow 8: Register MCP Server with Cryptographic Identity (MCP Server Developer)

**Starting Point**: MCP Server Developer has a new Model Context Protocol server to register

**Steps**:
1. **Navigate to MCP Servers Registry**
   - Click "MCP Servers" in sidebar navigation
   - MCP Registry page loads (`/dashboard/mcp`)
   - See list of existing MCP servers with verification status
   - Click "Register MCP Server" button in top-right

2. **Open MCP Registration Modal**
   - Modal appears: "Register MCP Server"
   - Form sections visible:
     - Basic Information
     - Connection Configuration
     - Cryptographic Identity (NEW)
     - Verification Settings

3. **Fill Basic Information**
   - **Server Name** (required): `filesystem-mcp`
   - **Display Name** (required): `Filesystem MCP Server`
   - **Description** (optional): "MCP server providing file system access capabilities"
   - **Version** (required): `1.0.0`
   - **Server URL** (required): `https://mcp.company.com/filesystem`

4. **Configure Cryptographic Identity** 🔐
   - **Public Key** (optional but recommended):
     - See large textarea with placeholder:
       ```
       -----BEGIN PUBLIC KEY-----
       ...
       -----END PUBLIC KEY-----
       ```
     - Paste PEM-formatted RSA-2048 public key
     - Help text shown: "Paste PEM-formatted public key for cryptographic verification"

   - **Key Type** (dropdown):
     - Select from options:
       - RSA-2048 (recommended for most use cases)
       - RSA-4096 (maximum security)
       - Ed25519 (modern, fast)
       - ECDSA P-256 (compact, efficient)
     - Select: RSA-2048

   - **Verification URL** (optional):
     - Input: `https://mcp.company.com/filesystem/verify`
     - Help text: "Endpoint for cryptographic challenge-response verification"
     - Used for runtime verification via challenge-response protocol

5. **Review Cryptographic Configuration**
   - Preview section shows:
     - ✓ Public key detected (2048-bit RSA)
     - ✓ Key fingerprint will be generated (SHA-256)
     - ✓ Verification endpoint configured
     - ℹ "Your server must sign responses with the corresponding private key"

6. **Submit Registration**
   - Review all configuration in summary panel
   - Click "Register MCP Server"
   - Loading spinner shows: "Registering MCP server..."
   - API request: `POST /api/v1/mcp-servers` with crypto fields:
     ```json
     {
       "name": "filesystem-mcp",
       "url": "https://mcp.company.com/filesystem",
       "public_key": "-----BEGIN PUBLIC KEY-----...",
       "key_type": "RSA-2048",
       "verification_url": "https://mcp.company.com/filesystem/verify"
     }
     ```

7. **Registration Success & Verification**
   - Modal closes automatically
   - Success notification: "MCP Server registered successfully!"
   - Page refreshes with new server in list
   - New server shows:
     - Status: "Active"
     - Verification Status: "Unverified" (yellow badge)
     - Public Key: "✓ Configured"
   - Server card highlighted with pulse animation

8. **Initial Cryptographic Verification**
   - Find newly registered server in table
   - Click "Verify" button next to server
   - Verification modal appears showing:
     - **Server Details**: Name, URL, version
     - **Public Key Fingerprint**: SHA-256 hash displayed
     - **Verification Method**: Challenge-response protocol
     - **Verification Checklist**:
       - ☐ Connect to verification endpoint
       - ☐ Send cryptographic challenge
       - ☐ Verify signature response
       - ☐ Validate certificate chain

   - Click "Verify Now"
   - Verification process runs:
     - Connecting to verification URL... ✓
     - Sending challenge (random nonce)... ✓
     - Receiving signed response... ✓
     - Verifying signature with public key... ✓
     - All checks passed in 1.2 seconds

   - Success modal:
     - "✓ MCP Server Verified Successfully"
     - Status changes to "Verified" (green checkmark)
     - Trust score initializes at 100%
     - Last Verified: "Just now"

9. **View Cryptographic Identity Details**
   - Click server name or "View" icon
   - MCP Detail Modal opens with tabs:
     - **Overview**: Basic info, status, uptime
     - **Cryptographic Identity** 🔐 (NEW):
       - Public Key Fingerprint (SHA-256): `4a:3b:2c:...`
       - Key Type: RSA-2048
       - Key Length: 2048 bits
       - Verification Status: ✓ Verified
       - Last Verified: timestamp
       - Verification Method: Challenge-response
       - Download Public Key button (.pem file)
     - **Capabilities**: Available MCP capabilities
     - **Verification History**: Past verification attempts
     - **Connected Agents**: Which agents use this server

10. **Download Public Key Certificate**
    - In Cryptographic Identity tab, click "Download Public Key"
    - Browser downloads: `filesystem-mcp-public-key.pem`
    - File contains PEM-formatted public key
    - Can be shared with other systems for verification
    - SHA-256 fingerprint included in filename metadata

**Success Criteria**:
- MCP server successfully registered with public key
- Public key fingerprint (SHA-256) calculated and displayed
- Server cryptographically verified via challenge-response
- Verification status shows "Verified" with green badge
- Public key downloadable as .pem file
- Trust score initialized at 100%
- Ready for agent connections with cryptographic authentication

**Error Handling**:
- Invalid public key format: "Invalid PEM format. Please check your public key."
- Key type mismatch: "Key type doesn't match detected algorithm. Check key type selection."
- Verification URL unreachable: "Cannot reach verification endpoint. Check URL and network."
- Signature verification failed: "Cryptographic verification failed. Check private key matches public key."
- Expired certificate: "TLS certificate expired. Update certificate and retry."

**Decision Points**:
- Include cryptographic identity or register without? (Recommended: WITH crypto)
- Choose RSA-2048 vs RSA-4096 vs Ed25519 vs ECDSA? (Most common: RSA-2048)
- Provide verification URL or skip? (Recommended for production servers)
- Verify immediately or later? (Best practice: Verify immediately)

**Time to Complete**: 5-7 minutes

---

### Workflow 9: Rotate MCP Server Public Key (MCP Server Developer)

**Starting Point**: MCP Server Developer needs to rotate keys for security compliance

**Context**: Organizations require cryptographic key rotation every 90 days for security

**Steps**:
1. **Navigate to MCP Server Details**
   - Go to `/dashboard/mcp`
   - Find server that needs key rotation (e.g., "filesystem-mcp")
   - Notice warning badge: "⚠ Key Rotation Due" (if 90+ days old)
   - Click server name to open detail modal

2. **Review Current Key Status**
   - In Cryptographic Identity tab, see:
     - Current Public Key Fingerprint: `4a:3b:2c:...`
     - Key Age: 92 days
     - Rotation Status: ⚠ Overdue (90 day policy)
     - Last Verified: 2 days ago
     - Recommendation: "Rotate key for security compliance"

3. **Initiate Key Rotation**
   - Click "Rotate Key" button in Cryptographic Identity section
   - Key Rotation Modal appears:
     - **Current Key**: Fingerprint displayed (read-only)
     - **New Key**: Upload or paste new public key
     - **Rotation Method**:
       - ○ Immediate rotation (service interruption possible)
       - ○ Gradual rollover (supports both keys temporarily) ✓
     - **Verification**: New verification required after rotation

4. **Generate New Key Pair** (External)
   - Developer generates new RSA-2048 key pair locally:
     ```bash
     openssl genrsa -out private-new.pem 2048
     openssl rsa -in private-new.pem -pubout -out public-new.pem
     ```
   - Reads new public key from `public-new.pem`

5. **Upload New Public Key**
   - In rotation modal, paste new public key:
     ```
     -----BEGIN PUBLIC KEY-----
     MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
     -----END PUBLIC KEY-----
     ```
   - System validates:
     - ✓ Valid PEM format
     - ✓ Valid RSA key
     - ✓ Correct key length (2048 bits)
     - ✓ Different from current key

   - New key fingerprint shown: `7d:9e:1f:...`
   - Confirm new key is different from old key ✓

6. **Configure Rotation Strategy**
   - Select "Gradual Rollover" option
   - Rollover period: 7 days (configurable 1-30 days)
   - During rollover:
     - Both old and new keys accepted for verification
     - Agents can transition gradually
     - No service interruption
   - After rollover period: Old key automatically deactivated

7. **Update Verification URL (If Changed)**
   - Verification URL: `https://mcp.company.com/filesystem/verify`
   - Same endpoint, no change needed
   - If endpoint changed:
     - Update to new verification URL
     - Test connectivity before saving

8. **Review Rotation Summary**
   - Summary panel shows:
     - **Current Key**: `4a:3b:2c:...` (will be deprecated)
     - **New Key**: `7d:9e:1f:...` (will become active)
     - **Rollover Period**: 7 days (both keys valid)
     - **Automatic Deactivation**: [Date 7 days from now]
     - **Re-verification**: Required immediately
   - Warning: "Ensure your server is updated with new private key"

9. **Execute Key Rotation**
   - Click "Rotate Key"
   - Confirmation dialog: "Are you sure you want to rotate cryptographic keys?"
   - Checkbox: ☑ "I have updated my server with the new private key"
   - Click "Confirm Rotation"
   - API request: `PUT /api/v1/mcp-servers/{id}/rotate-key`
   - Loading: "Rotating keys..."
   - Success notification: "Key rotation initiated successfully!"

10. **Verify New Key**
    - Rotation modal shows verification prompt:
      - "New key added. Verification required."
      - Click "Verify New Key"

    - Verification process with NEW key:
      - Connecting to verification URL... ✓
      - Sending challenge signed with NEW key requirement... ✓
      - Server signs response with NEW private key... ✓
      - AIM verifies signature with NEW public key... ✓
      - Verification successful in 1.5 seconds

    - Success: "✓ New key verified successfully!"
    - Both keys now active during rollover period

11. **Monitor Rollover Period**
    - Return to MCP server detail
    - Cryptographic Identity section shows:
      - **Active Keys**: 2 keys
        - Key 1 (Old): `4a:3b:2c:...` - Expires in 7 days
        - Key 2 (New): `7d:9e:1f:...` - Primary key ✓
      - **Rollover Status**: In progress (Day 1 of 7)
      - **Recommendation**: "Update all agents to use new key"

    - Progress indicator: Day 1/7, Day 2/7, etc.
    - Email reminders sent: Day 3, Day 6 (before deactivation)

12. **Complete Rollover**
    - After 7 days, automatic deactivation occurs:
      - Old key removed from active keys
      - Only new key remains: `7d:9e:1f:...`
      - Status updates: "Active" (single key)
      - Rotation completed notification sent

    - Or manual early completion:
      - Click "Complete Rollover Early"
      - Confirm all agents updated
      - Old key immediately deactivated
      - New key becomes sole active key

13. **Update Rotation Records**
    - Cryptographic Identity section shows:
      - **Key Rotation History**:
        - January 20, 2025: Rotated from `4a:3b:2c:...` to `7d:9e:1f:...`
        - October 22, 2024: Rotated from `1f:6a:8b:...` to `4a:3b:2c:...`
      - **Next Rotation Due**: April 20, 2025 (90 days from now)
      - **Compliance Status**: ✓ Compliant (rotated within policy)

**Success Criteria**:
- New public key uploaded and validated
- Old and new keys both active during rollover period
- New key cryptographically verified
- Zero service interruption during rotation
- Old key automatically deactivated after rollover period
- Rotation history recorded for compliance
- Next rotation date scheduled (90 days)

**Error Handling**:
- Invalid new key: "Invalid public key format. Check PEM encoding."
- New key same as old: "New key must be different from current key."
- Verification of new key fails: "Cannot verify new key. Check private key matches."
- Server not updated: "Verification failed. Ensure server uses new private key."
- Rollover period too short: "Minimum rollover period is 1 day."

**Decision Points**:
- Immediate rotation vs gradual rollover? (Recommended: Gradual for zero downtime)
- Rollover period duration? (Common: 7 days, strict: 1 day, flexible: 30 days)
- Complete rollover early or wait full period? (Depends on agent migration status)
- Key type change or keep same? (Usually keep same type for consistency)

**Time to Complete**: 3-5 minutes (setup) + 7 days (rollover period)

---

### Workflow 10: View MCP Server Cryptographic Details (Security Admin / DevOps)

**Starting Point**: User needs to inspect MCP server cryptographic identity and verification status

**Steps**:
1. **Navigate to MCP Servers**
   - Click "MCP Servers" in sidebar
   - MCP Registry page loads (`/dashboard/mcp`)
   - See table of registered MCP servers

2. **Quick View from Table**
   - Table columns show:
     - Name
     - URL
     - Status (Active/Inactive/Suspended)
     - Verification Status (Verified ✓ / Unverified ⚠ / Failed ❌)
     - Public Key (icon: ✓ Configured / ⚠ Missing / 🔄 Rotating)
     - Last Verified (timestamp)
     - Actions (View/Edit/Verify/Delete icons)

   - Hover over "Public Key" icon:
     - Tooltip shows: "RSA-2048 key configured"
     - Fingerprint preview: `4a:3b:2c:...` (first 12 chars)

3. **Open Server Detail Modal**
   - Click server name or "View" (eye icon)
   - MCP Server Detail Modal opens full-screen
   - Tabs available:
     - Overview
     - Cryptographic Identity 🔐
     - Capabilities
     - Verification History
     - Connected Agents
     - Security Events

4. **View Cryptographic Identity Tab**
   - Click "Cryptographic Identity" tab
   - Complete cryptographic information displayed:

   **Section 1: Public Key Information**
   - **Key Fingerprint (SHA-256)**:
     - Full fingerprint displayed in monospace font:
       `4a:3b:2c:1d:9e:8f:7a:6b:5c:4d:3e:2f:1a:0b:9c:8d:
        7e:6f:5a:4b:3c:2d:1e:0f:9a:8b:7c:6d:5e:4f:3a:2b`
     - Copy button next to fingerprint
     - Help text: "Unique identifier for this public key"

   - **Key Type**: RSA-2048
   - **Key Length**: 2048 bits
   - **Algorithm**: RSA with SHA-256
   - **Key Format**: PEM (Privacy Enhanced Mail)
   - **Created**: October 22, 2024
   - **Age**: 90 days (shown with color: green <60, yellow 60-90, red >90)

   **Section 2: Verification Status**
   - **Verification Status**: ✓ Verified (green badge with checkmark)
   - **Verification Method**: Challenge-Response Protocol
   - **Last Verified**: January 18, 2025 at 14:35:22 UTC (2 days ago)
   - **Verification Frequency**: Every 24 hours (configurable)
   - **Next Verification**: January 21, 2025 at 14:35:22 UTC (in 22 hours)
   - **Auto-Suspend on Failure**: ✓ Enabled
   - **Consecutive Failures**: 0 (threshold: 3)

   **Section 3: Verification Endpoint**
   - **Verification URL**: `https://mcp.company.com/filesystem/verify`
   - **Protocol**: HTTPS/TLS 1.3
   - **Response Time**: 127ms (average of last 10 verifications)
   - **Success Rate**: 100% (98 of 98 verification attempts)
   - **Test Connection** button (real-time connectivity test)

   **Section 4: Key Rotation**
   - **Rotation Status**: ✓ Up to date
   - **Last Rotation**: October 22, 2024 (90 days ago)
   - **Next Rotation Due**: January 20, 2026 (90-day policy)
   - **Rotation Policy**: Required every 90 days
   - **Rotate Key** button (initiates rotation workflow)

   **Section 5: Public Key Certificate**
   - **Download Options**:
     - Download as PEM (.pem file)
     - Download as DER (.der file)
     - Copy to Clipboard (base64 encoded)
     - View Raw Key (expandable textarea with full PEM)

   - Click "Download as PEM":
     - File downloads: `filesystem-mcp-public-key.pem`
     - Contains PEM-formatted public key
     - Includes metadata comment with fingerprint

   **Section 6: Security Compliance**
   - **Compliance Status**:
     - ✓ Key strength compliant (2048-bit minimum)
     - ✓ Key age compliant (<90 days policy)
     - ✓ Verification frequency compliant (daily requirement)
     - ✓ TLS version compliant (1.2+ required)
   - **Security Score**: 95/100 (excellent)
   - **Recommendations**: None (all checks passed)

5. **View Verification History**
   - Click "Verification History" tab
   - Table shows all past verification attempts:
     - Timestamp
     - Method (Challenge-Response, Certificate, Health Check)
     - Status (Success ✓ / Failed ❌)
     - Duration (ms)
     - Response Time (ms)
     - Error Message (if failed)

   - Filter options:
     - Date range selector
     - Status filter (All / Success / Failed)
     - Method filter

   - Export verification history as CSV
   - See verification trend chart (success rate over time)

6. **Test Cryptographic Verification**
   - Click "Verify Now" button in Cryptographic Identity tab
   - Real-time verification modal appears:
     - Progress shown step-by-step:
       - ✓ Connecting to verification endpoint... (120ms)
       - ✓ Sending cryptographic challenge (nonce: f4a2b9...)... (45ms)
       - ✓ Receiving signed response... (210ms)
       - ✓ Verifying signature with public key... (38ms)
       - ✓ Validating TLS certificate... (92ms)
       - ✓ All checks passed (505ms total)

   - Result: "✓ Verification Successful"
   - New verification record added to history
   - Last Verified timestamp updated to "Just now"

7. **View Connected Agents**
   - Click "Connected Agents" tab
   - See which agents use this MCP server:
     - Agent Name
     - Agent ID
     - Connection Status (Active/Inactive)
     - Last Request (timestamp)
     - Request Count (last 24h)
     - Trust Score

   - Example data:
     - "Claude Code Assistant" - Active - Last request: 5 min ago - 127 requests - Trust: 95%
     - "Data Analyst Agent" - Active - Last request: 1 hour ago - 43 requests - Trust: 88%

8. **View Security Events**
   - Click "Security Events" tab
   - See security-related events for this MCP server:
     - Failed verification attempts
     - Suspicious connection patterns
     - Certificate expiration warnings
     - Key rotation events
     - Access denied events

   - Filter by severity (Critical/High/Medium/Low)
   - Export security events for compliance review

9. **Download Cryptographic Report**
   - Click "Generate Crypto Report" button
   - Report generation modal:
     - Select report sections:
       - ☑ Public key information
       - ☑ Verification history
       - ☑ Key rotation history
       - ☑ Security compliance status
       - ☑ Connected agents
     - Format: PDF (with digital signature) or JSON

   - Click "Generate Report"
   - Download: `filesystem-mcp-crypto-report-2025-01-20.pdf`
   - Report contains:
     - Server identity summary
     - Public key fingerprint and details
     - Complete verification history
     - Compliance attestation
     - Security recommendations

10. **Compare with Other Servers** (Optional)
    - Click "Compare Servers" button
    - Multi-select other MCP servers to compare:
      - ☑ Filesystem MCP
      - ☑ Database MCP
      - ☑ API Gateway MCP

    - Comparison table shows:
      - Key Type (RSA-2048 vs Ed25519 vs RSA-4096)
      - Key Age (days)
      - Verification Status
      - Success Rate (%)
      - Security Score (0-100)
      - Last Rotation Date

    - Identify servers needing attention:
      - ⚠ Database MCP: Key age 95 days (overdue rotation)
      - ✓ Filesystem MCP: All compliant
      - ✓ API Gateway MCP: All compliant

**Success Criteria**:
- Complete cryptographic identity information accessible
- Public key fingerprint (SHA-256) clearly displayed
- Verification status and history visible
- Key rotation information available
- Public key downloadable in multiple formats
- Compliance status clearly indicated
- Security events tracked and exportable
- Connected agents visible with trust scores

**Use Cases**:
- **Security Audit**: Review cryptographic configuration before compliance audit
- **Troubleshooting**: Diagnose verification failures by checking key details
- **Compliance**: Generate reports showing cryptographic controls
- **Key Management**: Monitor key age and plan rotations
- **Incident Response**: Investigate security events related to MCP server
- **Onboarding**: Verify new MCP server configuration is correct

**Information Displayed**:
- SHA-256 fingerprint (unique identifier)
- Key type and length (RSA-2048, RSA-4096, Ed25519, ECDSA-P256)
- Verification status (Verified, Unverified, Failed)
- Verification method (Challenge-Response, Certificate, Health Check)
- Last verification timestamp
- Key age and rotation due date
- Verification endpoint URL and response time
- Success rate and verification history
- Security compliance status
- Connected agents using this server

**Time to Complete**: 1-2 minutes

---

## Edge Cases & Error Scenarios

### 1. Network Connectivity Issues
- **Scenario**: API request fails due to network timeout
- **User Impact**: Cannot load dashboard, agents, or verifications
- **Handling**:
  - Show user-friendly error: "Connection lost. Retrying..."
  - Auto-retry with exponential backoff (3 attempts)
  - If all retries fail: "Cannot connect to AIM. Check your connection."
  - Fallback to cached data (if available)
  - Show "Offline Mode" indicator

### 2. Concurrent Verification Requests
- **Scenario**: Same agent makes 100 verification requests simultaneously
- **User Impact**: Potential performance degradation, rate limiting
- **Handling**:
  - Rate limiter activates (configured per agent)
  - Excess requests queued (up to limit)
  - If queue full: Deny with "Rate limit exceeded"
  - Admin alerted if unusual spike detected
  - Auto-scaling if traffic is legitimate

### 3. Expired Session During Critical Action
- **Scenario**: User's session expires while suspending agent
- **User Impact**: Action may fail, agent may not be suspended
- **Handling**:
  - Detect expired token before action
  - Show re-authentication modal: "Session expired. Please sign in again."
  - Preserve action state (which agent, what action)
  - After re-auth: Ask "Continue suspending agent X?"
  - Complete action seamlessly

### 4. Agent Registration During Downtime
- **Scenario**: Backend service down when registering agent
- **User Impact**: Agent registration fails
- **Handling**:
  - Detect service unavailability
  - Save registration data locally (IndexedDB)
  - Show: "Service temporarily unavailable. Your request is saved."
  - Auto-retry when service recovers
  - Notify user when registration completes
  - Fallback: Export registration as JSON for manual submission

### 5. Verification Deadlock
- **Scenario**: Agent A requires verification from Agent B, but B requires verification from A
- **User Impact**: Both agents stuck, cannot proceed
- **Handling**:
  - Deadlock detection algorithm activates
  - Break cycle: Approve one verification manually
  - Admin alerted: "Verification deadlock detected"
  - Suggested resolution: "Grant temporary approval to Agent A"
  - Log incident for policy review

### 6. Malformed Verification Request
- **Scenario**: Agent sends invalid verification request (missing required fields)
- **User Impact**: Verification fails, agent action blocked
- **Handling**:
  - Validate request schema immediately
  - Deny with detailed error: "Missing required field: action_type"
  - Return error to agent with exact validation issues
  - Agent can retry with corrected request
  - Log malformed requests (potential attack indicator)

### 7. Database Consistency Issues
- **Scenario**: Agent record exists but verification table out of sync
- **User Impact**: Inconsistent data shown to users
- **Handling**:
  - Background consistency check runs periodically
  - Detect inconsistency: Agent verified but no verification record
  - Auto-repair: Re-run verification to create record
  - Admin alerted: "Data inconsistency detected and repaired"
  - Manual reconciliation tool available in admin panel

### 8. Certificate Expiration
- **Scenario**: MCP server's TLS certificate expires
- **User Impact**: Server verification fails, marked as "failed"
- **Handling**:
  - Alert sent 30 days before expiration
  - Warning shown 7 days before expiration
  - On expiration day: Server auto-suspended
  - Admin notified: "MCP Server suspended: Certificate expired"
  - Renewal workflow: Upload new certificate → Re-verify → Resume

### 9. Trust Score Manipulation
- **Scenario**: Attacker tries to artificially inflate agent trust score
- **User Impact**: Potential security bypass
- **Handling**:
  - Trust score calculation uses cryptographic signatures
  - Tampering detection: Verify signature on each score update
  - If tampering detected: Freeze trust score, alert security team
  - Agent auto-suspended if tampering confirmed
  - Forensic investigation initiated

### 10. Regulatory Requirement Changes
- **Scenario**: New GDPR requirement mandates additional verification checks
- **User Impact**: Current verifications may not meet new standards
- **Handling**:
  - Policy update notification to all admins
  - Grace period: 30 days to comply
  - Show compliance status: "23 agents require re-verification"
  - Bulk re-verification tool available
  - After grace period: Non-compliant agents auto-suspended

---

## Success Metrics

### Platform-Level Metrics
- **Verification Performance**:
  - Average response time: <100ms
  - 99th percentile: <500ms
  - Success rate: >99%

- **Security Metrics**:
  - Threats detected: Track monthly trend
  - Mean time to detect (MTTD): <5 minutes
  - Mean time to respond (MTTR): <15 minutes
  - Zero successful breaches

- **User Adoption**:
  - Daily active users: Track growth
  - Feature utilization: >70% of features used
  - User satisfaction: >4.5/5 stars

### Workflow-Specific Metrics

**Agent Registration (Workflow 2)**:
- Time to register: <5 minutes
- Success rate: >95%
- Verification time: <3 seconds
- User abandonment: <5%

**Security Threat Response (Workflow 3)**:
- Alert-to-action time: <2 minutes
- False positive rate: <10%
- Incident containment: <5 minutes
- Resolution time: <24 hours (critical)

**Compliance Reporting (Workflow 5)**:
- Report generation time: <30 seconds
- Report accuracy: 100%
- Export success rate: >99%
- User satisfaction: >4.5/5

**MCP Server Management (Workflow 6)**:
- Registration time: <10 minutes
- Verification success: >90%
- Server uptime: >99.9%
- Health check frequency: Every 5 minutes

---

## Conclusion

The AIM platform provides comprehensive runtime verification for AI agents, enabling enterprises to:

1. **Trust AI Infrastructure**: Every action verified before execution
2. **Maintain Security**: Real-time threat detection and response
3. **Ensure Compliance**: Complete audit trails and automated reporting
4. **Enable Governance**: Centralized control and visibility
5. **Scale Confidently**: Support thousands of agents and verifications

These workflows demonstrate how different personas interact with AIM to achieve their goals, from registering agents to responding to security incidents. Each workflow is designed to be:

- **User-friendly**: Clear steps, minimal clicks, intuitive UI
- **Secure**: Runtime verification, cryptographic checks, audit trails
- **Compliant**: Full traceability, regulatory reporting, data retention
- **Reliable**: Error handling, retries, fallbacks, monitoring
- **Scalable**: Handles high volume, concurrent requests, real-time processing

The platform serves as the **foundation of trust for enterprise AI**, ensuring that AI agents operate within defined boundaries and organizations maintain complete control.

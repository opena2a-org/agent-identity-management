# MCP Cryptographic Identity - UI Guide

## Registration Modal

### New Fields Layout

```
┌─────────────────────────────────────────────────────────┐
│ Register MCP Server                                  [X]│
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Server Name *                                          │
│ [e.g., File Server MCP________________________]       │
│                                                         │
│ Server URL *                                           │
│ [https://mcp.example.com___________________]          │
│                                                         │
│ Description                                            │
│ [Brief description of what this MCP...               ]│
│ [                                                     ]│
│ [                                                     ]│
│                                                         │
│ Public Key (optional)                                  │
│ [-----BEGIN PUBLIC KEY-----                          ]│
│ [...                                                  ]│
│ [-----END PUBLIC KEY-----                            ]│
│ [                                                     ]│
│ [                                                     ]│
│ [                                                     ]│
│ 📝 Paste PEM-formatted public key for verification    │
│                                                         │
│ Key Type                                               │
│ [RSA-2048 ▼]                                          │
│   • RSA-2048                                           │
│   • RSA-4096                                           │
│   • Ed25519                                            │
│   • ECDSA P-256                                        │
│                                                         │
│ Verification URL (optional)                            │
│ [https://mcp-server.example.com/verify__________]     │
│ 🔗 Endpoint for cryptographic challenge-response      │
│                                                         │
│ Status                                                 │
│ [Pending ▼]                                           │
│                                                         │
├─────────────────────────────────────────────────────────┤
│                              [Cancel] [Register Server]│
└─────────────────────────────────────────────────────────┘
```

## Detail Modal - Cryptographic Identity Section

### When Public Key Exists

```
┌──────────────────────────────────────────────────────────────┐
│ [🛡️] File Operations Server                              [X]│
│     mcp_001                                                  │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│ Status: [Active]    Verification: [✓ Verified]             │
│                                                              │
│ Description                                                  │
│ Provides secure file system operations and management       │
│                                                              │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━│
│                                                              │
│ [🔑] Cryptographic Identity                                 │
│ ┌────────────────────────────────────────────────────────┐ │
│ │                                                        │ │
│ │ Public Key Fingerprint (SHA-256)                       │ │
│ │ ┌──────────────────────────────────────────────────┐  │ │
│ │ │ 4a:b2:c3:d4:e5:f6:07:18:29:3a:4b:5c:6d:7e:8f:9a │  │ │
│ │ └──────────────────────────────────────────────────┘  │ │
│ │                                                        │ │
│ │ Key Type                                               │ │
│ │ RSA-2048                                               │ │
│ │                                                        │ │
│ │ Public Key                                     [📥]    │ │
│ │ ┌──────────────────────────────────────────────────┐  │ │
│ │ │ -----BEGIN PUBLIC KEY-----                       │  │ │
│ │ │ MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB...       │  │ │
│ │ │ -----END PUBLIC KEY-----                         │  │ │
│ │ └──────────────────────────────────────────────────┘  │ │
│ │                                                        │ │
│ │ [✓ Cryptographically Verified]         [🔄 Verify Now]│ │
│ │                                                        │ │
│ └────────────────────────────────────────────────────────┘ │
│                                                              │
│ Recent Activity                                              │
│ • ✓ MCP server registered - Jan 15, 2025, 10:00 AM         │
│ • ✓ Cryptographic verification completed - Jan 20, 14:30   │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ [🗑️ Delete]                              [✏️ Edit Server]  │
└──────────────────────────────────────────────────────────────┘
```

### When No Public Key

```
┌──────────────────────────────────────────────────────────────┐
│ [🛡️] API Integration Hub                                 [X]│
│     mcp_004                                                  │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│ Status: [Pending]    Verification: [⏰ Unverified]          │
│                                                              │
│ Server URL                                                   │
│ https://mcp.example.com/api-hub                             │
│                                                              │
│ Verification URL                                             │
│ Not configured                                               │
│                                                              │
│ Created: Jan 19, 2025, 3:45 PM                              │
│ Last Verified: Never                                         │
│                                                              │
│ Recent Activity                                              │
│ • ✓ MCP server registered - Jan 19, 2025, 3:45 PM          │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ [🗑️ Delete]                              [✏️ Edit Server]  │
└──────────────────────────────────────────────────────────────┘
```

## Main Page - Server List

### Table View with Actions

```
┌──────────────────────────────────────────────────────────────────────────┐
│ MCP Servers                                     [+ Register MCP Server] │
│ Manage Model Context Protocol (MCP) servers and their cryptographic...  │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│ [Total: 6]  [Active: 4]  [Verified: 4]  [Last Verification: 2h ago]   │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│ Name                 URL              Status   Verification   Actions   │
├──────────────────────────────────────────────────────────────────────────┤
│ [🟣] File Ops       mcp.example.../  [Active] [✓ Verified]  [👁️][Verify]│
│     mcp_001         filesystem                 Jan 20, 14:30  [✏️][🗑️]  │
│                                                                          │
│ [🟣] Database       mcp.example.../  [Active] [✓ Verified]  [👁️][Verify]│
│     mcp_002         database                   Jan 20, 12:15  [✏️][🗑️]  │
│                                                                          │
│ [🟣] Cloud Storage  mcp.example.../  [Active] [✓ Verified]  [👁️][Verify]│
│     mcp_003         cloud-storage              Jan 20, 16:00  [✏️][🗑️]  │
│                                                                          │
│ [🟣] API Hub        mcp.example.../  [Pending][⏰ Unverified][👁️][Verify]│
│     mcp_004         api-hub                    Never          [✏️][🗑️]  │
│                                                                          │
│ [🟣] Analytics      mcp.example.../  [Active] [✓ Verified]  [👁️][Verify]│
│     mcp_005         analytics                  Jan 20, 10:30  [✏️][🗑️]  │
│                                                                          │
│ [🟣] Legacy Bridge  mcp.example.../  [Inactive][❌ Failed]   [👁️][Verify]│
│     mcp_006         legacy                     Jan 19, 08:00  [✏️][🗑️]  │
└──────────────────────────────────────────────────────────────────────────┘
```

### Action Button Descriptions

- **👁️ Eye Icon**: View full server details including cryptographic identity
- **Verify Button**: Trigger cryptographic verification (blue badge)
- **✏️ Edit Icon**: Open registration modal with pre-filled data
- **🗑️ Delete Icon**: Delete server with confirmation

### Verification Status Badges

- **[✓ Verified]** - Green badge with checkmark icon
- **[⏰ Unverified]** - Yellow badge with clock icon
- **[❌ Failed]** - Red badge with X icon

## Information Card

```
┌──────────────────────────────────────────────────────────────────┐
│ [🛡️] About MCP Server Verification                              │
│                                                                  │
│ Model Context Protocol (MCP) servers must be verified before    │
│ they can interact with AI agents. Cryptographic verification    │
│ uses public key infrastructure to ensure servers meet security  │
│ standards and operate within defined boundaries. Regular        │
│ re-verification is recommended to maintain trust scores.        │
└──────────────────────────────────────────────────────────────────┘
```

## Key User Flows

### 1. Register New MCP Server with Crypto
```
User clicks [+ Register MCP Server]
  → Modal opens
  → Fill: Name, URL, Description
  → Paste: Public Key (PEM format)
  → Select: Key Type (RSA-2048)
  → Enter: Verification URL
  → Click [Register Server]
  → Server appears in table with "Unverified" status
```

### 2. View Cryptographic Details
```
User clicks [👁️] icon on server row
  → Detail modal opens
  → Displays cryptographic identity section
  → Shows fingerprint, key type, public key
  → User can download public key
  → User can verify server
```

### 3. Verify Server
```
Method 1: From table
  User clicks [Verify] button
  → API call to verify endpoint
  → Status updates to "Verified"
  → Last verified timestamp updates

Method 2: From detail modal
  User clicks [🔄 Verify Now]
  → Button shows "Verifying..." with spinner
  → API call completes
  → Badge updates to "Cryptographically Verified"
  → Timestamp updates
```

### 4. Edit Crypto Fields
```
User clicks [✏️] or [Edit Server] in modal
  → Registration modal opens in edit mode
  → All fields pre-filled including crypto fields
  → User can update public key, key type, verification URL
  → Click [Update Server]
  → Changes saved and reflected immediately
```

### 5. Download Public Key
```
User opens detail modal
  → Cryptographic identity section visible
  → Click [📥] download icon
  → Browser downloads {server_name}_public_key.pem
  → File contains PEM-formatted public key
```

## Visual Indicators

### Status Colors
- **Green**: Active, Verified
- **Yellow**: Pending, Unverified
- **Red**: Failed verification
- **Gray**: Inactive

### Icons
- 🛡️ Shield: Security/verification
- 🔑 Key: Cryptographic identity
- 👁️ Eye: View details
- 📥 Download: Export public key
- 🔄 Refresh: Verify/re-verify
- ✏️ Edit: Modify server
- 🗑️ Trash: Delete server

### Badges
- Rounded pills with icon + text
- Color-coded by status
- Consistent across all views

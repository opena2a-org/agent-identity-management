package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

// SDKHandler handles SDK download operations
type SDKHandler struct {
	jwtService      *auth.JWTService
	sdkTokenRepo    domain.SDKTokenRepository
}

// NewSDKHandler creates a new SDK handler
func NewSDKHandler(jwtService *auth.JWTService, sdkTokenRepo domain.SDKTokenRepository) *SDKHandler {
	return &SDKHandler{
		jwtService:   jwtService,
		sdkTokenRepo: sdkTokenRepo,
	}
}

// SDKCredentials represents the credentials file embedded in SDK
// These are stored in .aim/sdk_credentials.json (NOT credentials.json)
// This separation prevents collisions with agent credentials
type SDKCredentials struct {
	SchemaVersion string `json:"schemaVersion"`       // Schema version for future evolution
	Type          string `json:"type"`                // Credential type: "sdk_oauth"
	AIMUrl        string `json:"aimUrl"`
	RefreshToken  string `json:"refreshToken"`
	SDKTokenID    string `json:"sdkTokenId"`          // For usage tracking via X-SDK-Token header
	UserID        string `json:"userId"`
	Email         string `json:"email"`
}

// DownloadSDK generates a pre-configured SDK with embedded credentials
// @Summary Download pre-configured Python SDK
// @Description Downloads production-ready Python SDK with embedded OAuth credentials for zero-config usage. Go and JavaScript SDKs planned for Q1-Q2 2026.
// @Tags sdk
// @Produce application/zip
// @Param sdk query string false "SDK type (only 'python' supported)" default(python)
// @Success 200 {file} binary "SDK zip file"
// @Failure 400 {object} ErrorResponse "Invalid SDK type - only Python supported"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sdk/download [get]
// @Security BearerAuth
func (h *SDKHandler) DownloadSDK(c fiber.Ctx) error {
	// Get SDK type from query parameter (default to python for backward compatibility)
	sdkType := c.Query("sdk", "python")

	// Validate SDK type - Python and Java SDKs are production-ready
	// Go and JavaScript SDKs planned for Q1-Q2 2026 release
	validSDKs := map[string]bool{
		"python": true,
		"java":   true,
	}

	if !validSDKs[sdkType] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid SDK type '%s'. Available SDKs: 'python', 'java'. Go and JavaScript SDKs planned for Q1-Q2 2026.", sdkType),
		})
	}

	// Get authenticated user from context (set by AuthMiddleware)
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	organizationID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization not found",
		})
	}

	email, ok := c.Locals("email").(string)
	if !ok {
		email = ""
	}

	role, ok := c.Locals("role").(string)
	if !ok {
		role = "member"
	}

	// Generate SDK refresh token (90 days)
	refreshToken, err := h.jwtService.GenerateSDKRefreshToken(
		userID.String(),
		organizationID.String(),
		email,
		role,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to generate SDK token: %v", err),
		})
	}

	// Extract token ID (JTI) from JWT for tracking
	tokenID, err := h.jwtService.GetTokenID(refreshToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to extract token ID: %v", err),
		})
	}

	// Hash the token for secure storage (SHA-256)
	hasher := sha256.New()
	hasher.Write([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))

	// Get client IP and user agent
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// Parse User-Agent into friendly device name with SDK type
	baseDeviceName := h.parseDeviceName(userAgent)
	deviceName := fmt.Sprintf("%s SDK (%s)", getSDKDisplayName(sdkType), baseDeviceName)
	deviceFingerprint := h.generateDeviceFingerprint(userAgent, ipAddress)

	// Track SDK token in database for security (revocation, monitoring)
	sdkToken := &domain.SDKToken{
		ID:                uuid.New(),
		UserID:            userID,
		OrganizationID:    organizationID,
		TokenHash:         tokenHash,
		TokenID:           tokenID,
		DeviceName:        &deviceName,
		DeviceFingerprint: &deviceFingerprint,
		IPAddress:         &ipAddress,
		UserAgent:         &userAgent,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(90 * 24 * time.Hour), // 90 days
		Metadata:          map[string]interface{}{
			"source": "sdk_download",
		},
	}

	// SECURITY: Track SDK token (errors don't block download)
	// No logging to prevent information leakage
	_ = h.sdkTokenRepo.Create(sdkToken)

	// Get AIM URL from environment or use request base URL
	aimURL := os.Getenv("AIM_PUBLIC_URL")
	if aimURL == "" {
		aimURL = c.BaseURL()
	}

	// Create credentials object with schema version and type
	// These fields help the SDK distinguish between SDK credentials and agent credentials
	credentials := SDKCredentials{
		SchemaVersion: "1.0",      // Schema version for future evolution
		Type:          "sdk_oauth", // Explicitly mark as SDK OAuth credentials
		AIMUrl:        aimURL,
		RefreshToken:  refreshToken,
		SDKTokenID:    tokenID,    // Include SDK token ID for usage tracking
		UserID:        userID.String(),
		Email:         email,
	}

	// Generate SDK zip with embedded credentials
	zipData, version, err := h.createSDKZip(credentials, sdkType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create SDK package: %v", err),
		})
	}

	// Set response headers for file download with version
	filename := fmt.Sprintf("aim-sdk-%s-v%s.zip", sdkType, version)
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	return c.Send(zipData)
}

// createSDKZip creates a zip file with SDK and embedded credentials
// Returns: (zipData []byte, version string, error)
func (h *SDKHandler) createSDKZip(credentials SDKCredentials, sdkType string) ([]byte, string, error) {
	// Create in-memory zip buffer
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Get SDK root directory based on type
	// Use environment variable if set, otherwise use relative path from project root
	sdkBaseDir := os.Getenv("SDK_BASE_DIR")
	if sdkBaseDir == "" {
		// Default: relative to binary location (./sdk from /root/ in container, ../../sdk in dev)
		sdkBaseDir = filepath.Join(".", "sdk")
		// Check if SDK exists at ./sdk (production/container), otherwise try ../../sdk (development)
		if _, err := os.Stat(filepath.Join(sdkBaseDir, sdkType)); os.IsNotExist(err) {
			sdkBaseDir = filepath.Join("..", "..", "sdk")
		}
	}
	sdkRoot := filepath.Join(sdkBaseDir, sdkType)

	// Read VERSION file to get SDK version
	version := "1.0.0" // Default version
	versionPath := filepath.Join(sdkRoot, "VERSION")
	if versionData, err := os.ReadFile(versionPath); err == nil {
		version = strings.TrimSpace(string(versionData))
	}

	zipPrefix := fmt.Sprintf("aim-sdk-%s", sdkType)

	// Add SDK files to zip
	err := filepath.Walk(sdkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain directories and files
		if info.IsDir() {
			dirName := filepath.Base(path)
			skipDirs := []string{
				"__pycache__", ".pytest_cache", "*.egg-info", ".git",
				"node_modules", ".next", "dist", "build", "target",
				".idea", ".vscode",
			}
			for _, skip := range skipDirs {
				if dirName == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip test files, compiled files, and build artifacts
		fileName := filepath.Base(path)
		ext := filepath.Ext(fileName)
		skipExts := []string{".pyc", ".pyo", ".so", ".dylib", ".exe", ".o"}
		for _, skipExt := range skipExts {
			if ext == skipExt {
				return nil
			}
		}
		if fileName == ".DS_Store" || fileName == "Thumbs.db" {
			return nil
		}

		// Calculate relative path within zip
		relPath, err := filepath.Rel(sdkRoot, path)
		if err != nil {
			return err
		}

		// Create zip entry with SDK-specific prefix
		zipFile, err := zipWriter.Create(filepath.Join(zipPrefix, relPath))
		if err != nil {
			return err
		}

		// Read and write file content
		fileData, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = zipFile.Write(fileData)
		return err
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to add SDK files: %w", err)
	}

	// Create credentials file in .aim directory
	credentialsJSON, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Add credentials file to zip (in .aim directory)
	// Using sdk_credentials.json to avoid collision with agent credentials
	credPath := filepath.Join(zipPrefix, ".aim", "sdk_credentials.json")
	credFile, err := zipWriter.Create(credPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create credentials file: %w", err)
	}

	_, err = credFile.Write(credentialsJSON)
	if err != nil {
		return nil, "", fmt.Errorf("failed to write credentials: %w", err)
	}

	// Add README with SDK-specific setup instructions
	setupInstructions := h.generateSetupInstructions(sdkType, zipPrefix)

	readmePath := filepath.Join(zipPrefix, "QUICKSTART.md")
	readmeFile, err := zipWriter.Create(readmePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create README: %w", err)
	}

	_, err = readmeFile.Write([]byte(setupInstructions))
	if err != nil {
		return nil, "", fmt.Errorf("failed to write README: %w", err)
	}

	// Close zip writer
	err = zipWriter.Close()
	if err != nil {
		return nil, "", fmt.Errorf("failed to close zip: %w", err)
	}

	return buf.Bytes(), version, nil
}

// parseDeviceName extracts a friendly device name from User-Agent string
// Examples: "Chrome on macOS", "Firefox on Windows 11", "Safari on iPhone"
func (h *SDKHandler) parseDeviceName(userAgent string) string {
	if userAgent == "" {
		return "Unknown Device"
	}

	browser := "Unknown Browser"
	os := "Unknown OS"

	// Detect browser
	switch {
	case containsUA(userAgent, "Chrome") && !containsUA(userAgent, "Edg"):
		browser = "Chrome"
	case containsUA(userAgent, "Firefox"):
		browser = "Firefox"
	case containsUA(userAgent, "Safari") && !containsUA(userAgent, "Chrome"):
		browser = "Safari"
	case containsUA(userAgent, "Edg"):
		browser = "Edge"
	case containsUA(userAgent, "Opera") || containsUA(userAgent, "OPR"):
		browser = "Opera"
	}

	// Detect operating system
	switch {
	case containsUA(userAgent, "Windows NT 10.0"):
		os = "Windows 10/11"
	case containsUA(userAgent, "Windows NT 6.3"):
		os = "Windows 8.1"
	case containsUA(userAgent, "Windows NT 6.2"):
		os = "Windows 8"
	case containsUA(userAgent, "Windows NT 6.1"):
		os = "Windows 7"
	case containsUA(userAgent, "Windows"):
		os = "Windows"
	case containsUA(userAgent, "Mac OS X"):
		os = "macOS"
	case containsUA(userAgent, "Linux"):
		os = "Linux"
	case containsUA(userAgent, "iPhone"):
		os = "iPhone"
	case containsUA(userAgent, "iPad"):
		os = "iPad"
	case containsUA(userAgent, "Android"):
		os = "Android"
	}

	return fmt.Sprintf("%s on %s", browser, os)
}

// generateDeviceFingerprint creates a unique fingerprint from User-Agent and IP
// Used to detect when same device downloads SDK multiple times
func (h *SDKHandler) generateDeviceFingerprint(userAgent, ipAddress string) string {
	hasher := sha256.New()
	hasher.Write([]byte(userAgent + "|" + ipAddress))
	hash := hasher.Sum(nil)
	// Return first 16 chars of hex hash for readability
	return hex.EncodeToString(hash)[:16]
}

// Helper function to check if User-Agent string contains substring
func containsUA(s, substr string) bool {
	return len(s) >= len(substr) && stringContainsUA(s, substr)
}

func stringContainsUA(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getSDKDisplayName converts SDK type to capitalized display name
func getSDKDisplayName(sdkType string) string {
	switch sdkType {
	case "python":
		return "Python"
	case "java":
		return "Java"
	case "go":
		return "Go"
	case "javascript":
		return "JavaScript"
	default:
		return sdkType
	}
}

// generateSetupInstructions creates SDK-specific setup instructions
func (h *SDKHandler) generateSetupInstructions(sdkType, zipPrefix string) string {
	switch sdkType {
	case "python":
		return h.generatePythonInstructions(zipPrefix)
	case "java":
		return h.generateJavaInstructions(zipPrefix)
	case "go":
		return h.generateGoInstructions(zipPrefix)
	case "javascript":
		return h.generateJavaScriptInstructions(zipPrefix)
	default:
		return "# AIM SDK\n\nPlease refer to the SDK documentation for usage instructions."
	}
}

// generatePythonInstructions creates Python-specific setup instructions
func (h *SDKHandler) generatePythonInstructions(zipPrefix string) string {
	return `# AIM Python SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ` + "```bash\n   unzip " + zipPrefix + ".zip\n   cd " + zipPrefix + "\n   ```" + `

2. Install the SDK:
   ` + "```bash\n   pip install -e .\n   ```" + `

## Usage

The SDK is already configured with your identity. Just use it!

` + "```python\n" +
		`from aim_sdk import AIMClient

# Zero configuration needed! Your credentials are embedded.
client = AIMClient()

# Register an agent
agent = client.register_agent(
    name="my-awesome-agent",
    agent_type="ai_agent",
    description="An agent that does amazing things"
)

print(f"Agent registered! ID: {agent['id']}")
print(f"Trust Score: {agent.get('trust_score', 'N/A')}")
` + "```" + `

## Automatic Authentication

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens when they expire
- ✅ Work for 90 days without re-authentication

## Security

Your SDK credentials are stored in ` + "`.aim/sdk_credentials.json`" + `. Keep this file secure!

Agent credentials (after registration) are stored separately in ` + "`~/.aim/agents/{agent-name}.json`" + `.

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the included test files.
`
}

// generateJavaInstructions creates Java-specific setup instructions
func (h *SDKHandler) generateJavaInstructions(zipPrefix string) string {
	return `# AIM Java SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ` + "```bash\n   unzip " + zipPrefix + ".zip\n   cd " + zipPrefix + "\n   ```" + `

2. Install the SDK to your local Maven repository:
   ` + "```bash\n   mvn install -DskipTests\n   ```" + `

3. Add to your project's pom.xml:
   ` + "```xml\n   <dependency>\n       <groupId>org.opena2a</groupId>\n       <artifactId>aim-sdk</artifactId>\n       <version>1.0.0</version>\n   </dependency>\n   ```" + `

## Usage

The SDK is already configured with your identity. Just use it!

` + "```java\n" +
		`import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;

public class MyAgent {
    public static void main(String[] args) throws Exception {
        // Zero configuration needed! Your credentials are embedded.
        // The SDK automatically:
        // - Loads credentials from ~/.aim/sdk_credentials.json
        // - Refreshes tokens before expiry
        // - Retries on transient failures
        // - Recovers from token revocation

        AIMClient agent = AIMClient.secure(
            "my-awesome-agent",
            List.of("db:read", "db:write"),
            AgentType.AI_AGENT
        );

        System.out.println("Agent registered! Name: " + agent.getAgentName());

        // Verify capability before performing action
        VerificationResult result = agent.verifyCapability("db:read", "users_table");
        if (result.isVerified()) {
            // Perform the action
            String data = agent.performAction("db:read", "users_table",
                () -> queryDatabase("SELECT * FROM users"));
            System.out.println("Data: " + data);
        }
    }
}
` + "```" + `

## Spring Boot Integration

Use the ` + "`@SecureAction`" + ` annotation for declarative security:

` + "```java\n" +
		`@Service
public class DataService {

    @SecureAction(capability = "db:read", resourceId = "users")
    public List<User> getUsers() {
        return userRepository.findAll();
    }

    @SecureAction(capability = "db:write", resourceId = "orders", riskLevel = RiskLevel.HIGH)
    public Order createOrder(OrderRequest request) {
        return orderRepository.save(new Order(request));
    }
}
` + "```" + `

## Automatic Token Management

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens 60 seconds before expiry
- ✅ Retry failed requests with exponential backoff
- ✅ Recover automatically from token revocation
- ✅ Work for 90 days without re-authentication

**Bulletproof by design** - developers never need to think about token management.

## Security

Your SDK credentials are stored in ` + "`.aim/sdk_credentials.json`" + `. Keep this file secure!

Agent credentials (after registration) are stored separately in ` + "`~/.aim/agents/{agent-name}.json`" + `.

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the ` + "`examples/`" + ` directory.
`
}

// generateGoInstructions creates Go-specific setup instructions
func (h *SDKHandler) generateGoInstructions(zipPrefix string) string {
	return `# AIM Go SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ` + "```bash\n   unzip " + zipPrefix + ".zip\n   cd " + zipPrefix + "\n   ```" + `

2. Initialize Go module (if needed):
   ` + "```bash\n   go mod init your-project\n   go mod tidy\n   ```" + `

## Usage

The SDK is already configured with your identity. Just use it!

` + "```go\n" +
		`package main

import (
    "fmt"
    "log"

    "github.com/opena2a/aim-sdk-go/client"
)

func main() {
    // Zero configuration needed! Your credentials are embedded.
    c, err := client.NewClient()
    if err != nil {
        log.Fatal(err)
    }

    // Register an agent
    agent, err := c.RegisterAgent(&client.RegisterAgentRequest{
        Name:        "my-awesome-agent",
        AgentType:   "ai_agent",
        Description: "An agent that does amazing things",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Agent registered! ID: %s\n", agent.ID)
    fmt.Printf("Trust Score: %.2f\n", agent.TrustScore)
}
` + "```" + `

## Automatic Authentication

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens when they expire
- ✅ Work for 90 days without re-authentication

## Security

Your SDK credentials are stored in ` + "`.aim/sdk_credentials.json`" + `. Keep this file secure!

Agent credentials (after registration) are stored separately in ` + "`~/.aim/agents/{agent-name}.json`" + `.

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the included test files.
`
}

// generateJavaScriptInstructions creates JavaScript-specific setup instructions
func (h *SDKHandler) generateJavaScriptInstructions(zipPrefix string) string {
	return `# AIM JavaScript SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ` + "```bash\n   unzip " + zipPrefix + ".zip\n   cd " + zipPrefix + "\n   ```" + `

2. Install dependencies:
   ` + "```bash\n   npm install\n   # or\n   yarn install\n   ```" + `

## Usage

The SDK is already configured with your identity. Just use it!

` + "```javascript\n" +
		`const { AIMClient } = require('@opena2a/aim-sdk');

async function main() {
  // Zero configuration needed! Your credentials are embedded.
  const client = new AIMClient();

  // Register an agent
  const agent = await client.registerAgent({
    name: 'my-awesome-agent',
    agentType: 'ai_agent',
    description: 'An agent that does amazing things'
  });

  console.log('Agent registered! ID:', agent.id);
  console.log('Trust Score:', agent.trustScore);
}

main().catch(console.error);
` + "```" + `

## TypeScript Support

This SDK includes full TypeScript definitions!

` + "```typescript\n" +
		`import { AIMClient, Agent, RegisterAgentRequest } from '@opena2a/aim-sdk';

async function registerAgent(): Promise<Agent> {
  const client = new AIMClient();

  const request: RegisterAgentRequest = {
    name: 'my-awesome-agent',
    agentType: 'ai_agent',
    description: 'An agent that does amazing things'
  };

  return await client.registerAgent(request);
}
` + "```" + `

## Automatic Authentication

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens when they expire
- ✅ Work for 90 days without re-authentication

## Security

Your SDK credentials are stored in ` + "`.aim/sdk_credentials.json`" + `. Keep this file secure!

Agent credentials (after registration) are stored separately in ` + "`~/.aim/agents/{agent-name}.json`" + `.

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the included test files.
`
}

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
type SDKCredentials struct {
	AIMUrl       string `json:"aim_url"`
	RefreshToken string `json:"refresh_token"`
	SDKTokenID   string `json:"sdk_token_id"` // For usage tracking via X-SDK-Token header
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
}

// DownloadSDK generates a pre-configured SDK with embedded credentials
// @Summary Download pre-configured SDK
// @Description Downloads Python SDK with embedded OAuth credentials for zero-config usage
// @Tags sdk
// @Produce application/zip
// @Success 200 {file} binary "SDK zip file"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/sdk/download [get]
// @Security BearerAuth
func (h *SDKHandler) DownloadSDK(c fiber.Ctx) error {
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

	// Track SDK token in database for security (revocation, monitoring)
	sdkToken := &domain.SDKToken{
		ID:             uuid.New(),
		UserID:         userID,
		OrganizationID: organizationID,
		TokenHash:      tokenHash,
		TokenID:        tokenID,
		IPAddress:      &ipAddress,
		UserAgent:      &userAgent,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(90 * 24 * time.Hour), // 90 days
		Metadata:       map[string]interface{}{
			"source": "sdk_download",
		},
	}

	err = h.sdkTokenRepo.Create(sdkToken)
	if err != nil {
		// Log error but don't fail download (tracking is not critical for download)
		fmt.Printf("Warning: Failed to track SDK token: %v\n", err)
	}

	// Get AIM URL from environment or use request base URL
	aimURL := os.Getenv("AIM_PUBLIC_URL")
	if aimURL == "" {
		aimURL = c.BaseURL()
	}

	// Create credentials object
	credentials := SDKCredentials{
		AIMUrl:       aimURL,
		RefreshToken: refreshToken,
		SDKTokenID:   tokenID, // Include SDK token ID for usage tracking
		UserID:       userID.String(),
		Email:        email,
	}

	// Generate SDK zip with embedded credentials
	zipData, err := h.createSDKZip(credentials)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create SDK package: %v", err),
		})
	}

	// Set response headers for file download
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename=aim-sdk-python.zip")
	c.Set("Content-Length", fmt.Sprintf("%d", len(zipData)))

	return c.Send(zipData)
}

// createSDKZip creates a zip file with SDK and embedded credentials
func (h *SDKHandler) createSDKZip(credentials SDKCredentials) ([]byte, error) {
	// Create in-memory zip buffer
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Get SDK root directory (relative to backend)
	sdkRoot := "../../sdks/python"

	// Add SDK files to zip
	err := filepath.Walk(sdkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain directories and files
		if info.IsDir() {
			dirName := filepath.Base(path)
			if dirName == "__pycache__" || dirName == ".pytest_cache" ||
			   dirName == "*.egg-info" || dirName == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip test files and compiled files
		fileName := filepath.Base(path)
		if filepath.Ext(fileName) == ".pyc" ||
		   filepath.Ext(fileName) == ".pyo" ||
		   fileName == ".DS_Store" {
			return nil
		}

		// Calculate relative path within zip
		relPath, err := filepath.Rel(sdkRoot, path)
		if err != nil {
			return err
		}

		// Create zip entry
		zipFile, err := zipWriter.Create(filepath.Join("aim-sdk-python", relPath))
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
		return nil, fmt.Errorf("failed to add SDK files: %w", err)
	}

	// Create credentials file in .aim directory
	credentialsJSON, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Add credentials file to zip
	credFile, err := zipWriter.Create("aim-sdk-python/.aim/credentials.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials file: %w", err)
	}

	_, err = credFile.Write(credentialsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to write credentials: %w", err)
	}

	// Add README with setup instructions
	setupInstructions := `# AIM SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ` + "```bash\n   unzip aim-sdk-python.zip\n   cd aim-sdk-python\n   ```" + `

2. Install the SDK:
   ` + "```bash\n   pip install -e .\n   ```" + `

## Usage

The SDK is already configured with your identity. Just use it!

` + "```python\n" +
		`from aim_sdk import register_agent

# Zero configuration needed! Your credentials are embedded.
agent = register_agent(
    name="my-awesome-agent",
    display_name="My Awesome Agent",
    description="An agent that does amazing things",
    agent_type="ai_agent"
)

print(f"Agent registered! ID: {agent.agent_id}")
print(f"Your agent is visible at: {agent.aim_url}/dashboard/agents")
` + "```" + `

## Automatic Authentication

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens when they expire
- ✅ Work for 90 days without re-authentication

## Security

Your credentials are stored in ` + "`.aim/credentials.json`" + `. Keep this file secure!

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the included test files.
`

	readmeFile, err := zipWriter.Create("aim-sdk-python/QUICKSTART.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create README: %w", err)
	}

	_, err = readmeFile.Write([]byte(setupInstructions))
	if err != nil {
		return nil, fmt.Errorf("failed to write README: %w", err)
	}

	// Close zip writer
	err = zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close zip: %w", err)
	}

	return buf.Bytes(), nil
}

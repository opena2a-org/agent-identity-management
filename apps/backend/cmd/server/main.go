package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/config"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
	"github.com/opena2a/identity/backend/internal/infrastructure/cache"
	"github.com/opena2a/identity/backend/internal/infrastructure/oauth"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
	"github.com/opena2a/identity/backend/internal/interfaces/http/handlers"
	"github.com/opena2a/identity/backend/internal/interfaces/http/middleware"
)

// @title Agent Identity Management API
// @version 1.0
// @description Enterprise-grade identity verification and security platform for AI agents and MCP servers
// @contact.name OpenA2A Team
// @contact.url https://opena2a.org
// @contact.email hello@opena2a.org
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Wrap database with sqlx for OAuth repository
	dbx := sqlx.NewDb(db, "postgres")

	// Initialize Redis
	redisClient, err := initRedis(cfg)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer redisClient.Close()

	// Initialize repositories
	repos := initRepositories(db, dbx)
	oauthRepo := repository.NewOAuthRepositoryPostgres(dbx)

	// Initialize cache
	cacheService, err := cache.NewRedisCache(&cache.CacheConfig{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Fatal("Failed to initialize cache:", err)
	}

	// Initialize infrastructure services
	jwtService := auth.NewJWTService()

	legacyOAuthService := auth.NewOAuthService()

	// Initialize OAuth providers
	oauthProviders := initOAuthProviders(cfg)

	// Initialize application services
	services, keyVault := initServices(db, repos, cacheService, oauthRepo, jwtService, oauthProviders)

	// Initialize handlers
	h := initHandlers(services, repos, jwtService, legacyOAuthService, keyVault)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Agent Identity Management",
		ServerHeader: "AIM/1.0",
		ErrorHandler: customErrorHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// Global middleware
	app.Use(middleware.RecoveryMiddleware())
	app.Use(middleware.LoggerMiddleware())

	// CORS with allowed origins from environment
	// IMPORTANT: Frontend ALWAYS runs on port 3000, backend on port 8080
	allowedOrigins := []string{
		"http://localhost:3000",
	}
	if customOrigins := os.Getenv("ALLOWED_ORIGINS"); customOrigins != "" {
		allowedOrigins = []string{customOrigins}
	}
	app.Use(middleware.CORSMiddleware(allowedOrigins))

	// Health check (no auth required)
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "agent-identity-management",
			"time":    time.Now().UTC(),
		})
	})

	app.Get("/health/ready", func(c fiber.Ctx) error {
		// Check database
		if err := db.Ping(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"ready": false,
				"error": "database unavailable",
			})
		}

		// Check Redis
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"ready": false,
				"error": "redis unavailable",
			})
		}

		return c.JSON(fiber.Map{
			"ready":    true,
			"database": "connected",
			"redis":    "connected",
		})
	})

	// ⭐ SDK API routes - MUST be at app level to avoid middleware inheritance
	// These routes use API key authentication for SDK/programmatic access
	sdkAPI := app.Group("/api/v1/sdk-api")
	sdkAPI.Use(middleware.APIKeyMiddleware(db))
	sdkAPI.Use(middleware.RateLimitMiddleware())
	sdkAPI.Post("/agents/:id/capabilities", h.Capability.GrantCapability)  // SDK capability reporting
	sdkAPI.Post("/agents/:id/mcp-servers", h.Agent.AddMCPServersToAgent)  // SDK MCP registration
	sdkAPI.Post("/agents/:id/detection/report", h.Detection.ReportDetection) // SDK MCP detection and integration reporting

	// API v1 routes (JWT authenticated)
	v1 := app.Group("/api/v1")
	setupRoutes(v1, h, jwtService, repos.SDKToken, db)

	// Start server
	port := cfg.Server.Port
	log.Printf("🚀 Agent Identity Management API starting on port %s", port)
	log.Printf("📊 Database: %s@%s:%d", cfg.Database.User, cfg.Database.Host, cfg.Database.Port)
	log.Printf("💾 Redis: %s:%d", cfg.Redis.Host, cfg.Redis.Port)

	// Check OAuth configuration from environment
	googleConfigured := os.Getenv("GOOGLE_CLIENT_ID") != ""
	microsoftConfigured := os.Getenv("MICROSOFT_CLIENT_ID") != ""
	oktaConfigured := os.Getenv("OKTA_CLIENT_ID") != ""
	log.Printf("🔐 OAuth Providers: Google=%v, Microsoft=%v, Okta=%v",
		googleConfigured,
		microsoftConfigured,
		oktaConfigured,
	)

	// Graceful shutdown
	go func() {
		if err := app.Listen(":" + port); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	if err := app.Shutdown(); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func initDatabase(cfg *config.Config) (*sql.DB, error) {
	// Build connection string manually with explicit PostgreSQL URL format
	// This ensures TCP connection even on Mac with local PostgreSQL
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.Database.MaxConnections)
	db.SetMaxIdleConns(cfg.Database.MaxConnections / 2)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("✅ Database connected")
	return db, nil
}

func initRedis(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("✅ Redis connected")
	return client, nil
}

type Repositories struct {
	User              *repository.UserRepository
	Organization      *repository.OrganizationRepository
	Agent             *repository.AgentRepository
	APIKey            *repository.APIKeyRepository
	TrustScore        *repository.TrustScoreRepository
	AuditLog          *repository.AuditLogRepository
	Alert             *repository.AlertRepository
	MCPServer         *repository.MCPServerRepository
	MCPCapability     *repository.MCPServerCapabilityRepository // ✅ For MCP server capabilities
	Security          *repository.SecurityRepository
	SecurityPolicy    *repository.SecurityPolicyRepository       // ✅ For configurable security policies
	Webhook           *repository.WebhookRepository
	VerificationEvent *repository.VerificationEventRepositorySimple
	Tag               *repository.TagRepository
	SDKToken          domain.SDKTokenRepository
	Capability        domain.CapabilityRepository
	CapabilityRequest domain.CapabilityRequestRepository // ✅ For capability expansion approval workflow
}

func initRepositories(db *sql.DB, dbx *sqlx.DB) *Repositories {
	return &Repositories{
		User:              repository.NewUserRepository(db),
		Organization:      repository.NewOrganizationRepository(db),
		Agent:             repository.NewAgentRepository(db),
		APIKey:            repository.NewAPIKeyRepository(db),
		TrustScore:        repository.NewTrustScoreRepository(db),
		AuditLog:          repository.NewAuditLogRepository(db),
		Alert:             repository.NewAlertRepository(db),
		MCPServer:         repository.NewMCPServerRepository(db),
		MCPCapability:     repository.NewMCPServerCapabilityRepository(db), // ✅ For MCP server capabilities
		Security:          repository.NewSecurityRepository(db),
		SecurityPolicy:    repository.NewSecurityPolicyRepository(db),       // ✅ For configurable security policies
		Webhook:           repository.NewWebhookRepository(db),
		VerificationEvent: repository.NewVerificationEventRepository(db),
		Tag:               repository.NewTagRepository(db),
		SDKToken:          repository.NewSDKTokenRepository(db),
		Capability:        repository.NewCapabilityRepository(dbx),
		CapabilityRequest: repository.NewCapabilityRequestRepository(dbx), // ✅ For capability expansion approval workflow
	}
}

type Services struct {
	Auth              *application.AuthService
	Admin             *application.AdminService
	Agent             *application.AgentService
	APIKey            *application.APIKeyService
	Trust             *application.TrustCalculator
	Audit             *application.AuditService
	Alert             *application.AlertService
	Compliance        *application.ComplianceService
	MCP               *application.MCPService
	MCPCapability     *application.MCPCapabilityService // ✅ For MCP server capability management
	Security          *application.SecurityService
	SecurityPolicy    *application.SecurityPolicyService // ✅ For policy-based enforcement
	Webhook           *application.WebhookService
	VerificationEvent *application.VerificationEventService
	OAuth             *application.OAuthService
	Tag               *application.TagService
	SDKToken          *application.SDKTokenService
	Capability        *application.CapabilityService
	CapabilityRequest *application.CapabilityRequestService // ✅ For capability expansion approval workflow
	Detection         *application.DetectionService // ✅ For MCP auto-detection (SDK + Direct API)
}

func initServices(db *sql.DB, repos *Repositories, cacheService *cache.RedisCache, oauthRepo *repository.OAuthRepositoryPostgres, jwtService *auth.JWTService, oauthProviders map[domain.OAuthProvider]application.OAuthProvider) (*Services, *crypto.KeyVault) {
	// ✅ Initialize KeyVault for secure private key storage
	keyVault, err := crypto.NewKeyVaultFromEnv()
	if err != nil {
		log.Fatal("Failed to initialize KeyVault:", err)
	}
	log.Println("✅ KeyVault initialized for automatic key generation")

	// ✅ Initialize Security Policy Service for policy-based enforcement
	securityPolicyService := application.NewSecurityPolicyService(
		repos.SecurityPolicy,
		repos.Alert,
	)

	// Create services
	authService := application.NewAuthService(
		repos.User,
		repos.Organization,
		repos.APIKey,
		securityPolicyService, // ✅ For auto-creating default policies
	)

	adminService := application.NewAdminService(
		repos.User,
		repos.Organization,
	)

	auditService := application.NewAuditService(repos.AuditLog)

	trustCalculator := application.NewTrustCalculator(
		repos.TrustScore,
		repos.APIKey,
		repos.AuditLog,
		repos.Capability, // ✅ NEW: Add capability repository for risk scoring
	)

	agentService := application.NewAgentService(
		repos.Agent,
		trustCalculator,
		repos.TrustScore,
		keyVault,              // ✅ NEW: Inject KeyVault for automatic key generation
		repos.Alert,           // ✅ NEW: Inject AlertRepository for security alerts
		securityPolicyService, // ✅ NEW: Inject SecurityPolicyService for policy evaluation
		repos.Capability,      // ✅ NEW: Inject CapabilityRepository for capability checks
	)

	apiKeyService := application.NewAPIKeyService(
		repos.APIKey,
		repos.Agent,
	)

	alertService := application.NewAlertService(
		repos.Alert,
		repos.Agent,
	)

	complianceService := application.NewComplianceService(
		repos.AuditLog,
		repos.Agent,
		repos.User,
	)

	// ✅ Initialize MCP capability service BEFORE MCP service
	mcpCapabilityService := application.NewMCPCapabilityService(
		repos.MCPCapability,
		repos.MCPServer,
	)

	mcpService := application.NewMCPService(
		repos.MCPServer,
		repos.VerificationEvent,
		repos.User,
		keyVault,             // ✅ For automatic key generation
		mcpCapabilityService, // ✅ For automatic capability detection
	)

	securityService := application.NewSecurityService(
		repos.Security,
		repos.Agent,
		repos.Alert,  // ✅ For converting alerts to threats (NO MOCK DATA!)
	)

	webhookService := application.NewWebhookService(
		repos.Webhook,
	)

	// Initialize drift detection service for WHO/WHAT verification
	driftDetectionService := application.NewDriftDetectionService(
		repos.Agent,
		repos.Alert,
	)

	verificationEventService := application.NewVerificationEventService(
		repos.VerificationEvent,
		repos.Agent,
		driftDetectionService,
	)

	oauthService := application.NewOAuthService(
		oauthRepo,
		repos.User,
		repos.Organization,
		authService,
		auditService,
		jwtService,
		oauthProviders,
	)

	tagService := application.NewTagService(
		repos.Tag,
		repos.Agent,
		repos.MCPServer,
	)

	sdkTokenService := application.NewSDKTokenService(
		repos.SDKToken,
	)

	capabilityService := application.NewCapabilityService(
		repos.Capability,
		repos.Agent,
		repos.AuditLog,
		trustCalculator,
		repos.TrustScore,
	)

	capabilityRequestService := application.NewCapabilityRequestService(
		repos.CapabilityRequest,
		repos.Capability,
		repos.Agent,
	)

	detectionService := application.NewDetectionService(
		db,
		trustCalculator, // ✅ NEW: Inject trust calculator for proper risk assessment
		repos.Agent,     // ✅ NEW: Inject agent repository to fetch agent data
	)

	return &Services{
		Auth:              authService,
		Admin:             adminService,
		Agent:             agentService,
		APIKey:            apiKeyService,
		Trust:             trustCalculator,
		Audit:             auditService,
		Alert:             alertService,
		Compliance:        complianceService,
		MCP:               mcpService,
		MCPCapability:     mcpCapabilityService, // ✅ For MCP server capability management
		Security:          securityService,
		SecurityPolicy:    securityPolicyService, // ✅ For policy-based enforcement
		Webhook:           webhookService,
		VerificationEvent: verificationEventService,
		OAuth:             oauthService,
		Tag:               tagService,
		SDKToken:          sdkTokenService,
		Capability:        capabilityService,
		CapabilityRequest: capabilityRequestService, // ✅ For capability expansion approval workflow
		Detection:         detectionService, // ✅ For MCP auto-detection (SDK + Direct API)
	}, keyVault
}

type Handlers struct {
	Auth              *handlers.AuthHandler
	Agent             *handlers.AgentHandler
	APIKey            *handlers.APIKeyHandler
	TrustScore        *handlers.TrustScoreHandler
	Admin             *handlers.AdminHandler
	Compliance        *handlers.ComplianceHandler
	MCP               *handlers.MCPHandler
	Security          *handlers.SecurityHandler
	SecurityPolicy    *handlers.SecurityPolicyHandler // ✅ For policy management
	Analytics         *handlers.AnalyticsHandler
	Webhook           *handlers.WebhookHandler
	VerificationEvent *handlers.VerificationEventHandler
	OAuth             *handlers.OAuthHandler
	PublicAgent       *handlers.PublicAgentHandler
	PublicRegistration *handlers.PublicRegistrationHandler
	Tag               *handlers.TagHandler
	SDK               *handlers.SDKHandler
	SDKToken          *handlers.SDKTokenHandler
	AuthRefresh       *handlers.AuthRefreshHandler
	Capability        *handlers.CapabilityHandler
	Detection         *handlers.DetectionHandler // ✅ For MCP auto-detection (SDK + Direct API)
	CapabilityRequest *handlers.CapabilityRequestHandlers // ✅ For capability request approval
}

func initHandlers(services *Services, repos *Repositories, jwtService *auth.JWTService, oauthService *auth.OAuthService, keyVault *crypto.KeyVault) *Handlers {
	return &Handlers{
		Auth: handlers.NewAuthHandler(
			services.Auth,
			oauthService,
			jwtService,
		),
		Agent: handlers.NewAgentHandler(
			services.Agent,
			services.MCP, // ✅ Inject MCPService for auto-detect MCPs feature
			services.Audit,
		),
		APIKey: handlers.NewAPIKeyHandler(
			services.APIKey,
			services.Audit,
		),
		TrustScore: handlers.NewTrustScoreHandler(
			services.Trust,
			services.Agent,
			services.Audit,
		),
		Admin: handlers.NewAdminHandler(
			services.Auth,
			services.Admin,
			services.Agent,
			services.MCP,
			services.Audit,
			services.Alert,
			services.OAuth,
		),
		Compliance: handlers.NewComplianceHandler(
			services.Compliance,
			services.Audit,
		),
		MCP: handlers.NewMCPHandler(
			services.MCP,
			services.MCPCapability, // ✅ For capability endpoint
			services.Audit,
			repos.Agent, // ✅ For agent relationships ("Talks To")
		),
		Security: handlers.NewSecurityHandler(
			services.Security,
			services.Audit,
		),
		SecurityPolicy: handlers.NewSecurityPolicyHandler(
			services.SecurityPolicy,
		),
		Analytics: handlers.NewAnalyticsHandler(
			services.Agent,
			services.Audit,
			services.MCP,
			services.VerificationEvent,
		),
		Webhook: handlers.NewWebhookHandler(
			services.Webhook,
			services.Audit,
		),
		VerificationEvent: handlers.NewVerificationEventHandler(
			services.VerificationEvent,
		),
		OAuth: handlers.NewOAuthHandler(
			services.OAuth,
			services.Auth,
		),
		PublicAgent: handlers.NewPublicAgentHandler(
			services.Agent,
			services.Auth,
			keyVault,
		),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			services.OAuth,
			services.Auth,
			jwtService,
		),
		Tag: handlers.NewTagHandler(
			services.Tag,
		),
		SDK: handlers.NewSDKHandler(
			jwtService,
			repos.SDKToken,
		),
		SDKToken: handlers.NewSDKTokenHandler(
			services.SDKToken,
		),
		AuthRefresh: handlers.NewAuthRefreshHandler(
			jwtService,
			services.SDKToken,
		),
		Capability: handlers.NewCapabilityHandler(
			services.Capability,
		),
		Detection: handlers.NewDetectionHandler(
			services.Detection,
			services.Audit,
		),
		CapabilityRequest: handlers.NewCapabilityRequestHandlers(
			services.CapabilityRequest,
		),
	}
}

func initOAuthProviders(cfg *config.Config) map[domain.OAuthProvider]application.OAuthProvider {
	providers := make(map[domain.OAuthProvider]application.OAuthProvider)

	// Google OAuth
	if googleClientID := os.Getenv("GOOGLE_CLIENT_ID"); googleClientID != "" {
		googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
		googleRedirectURI := os.Getenv("GOOGLE_REDIRECT_URI")
		if googleRedirectURI == "" {
			googleRedirectURI = "http://localhost:8080/api/v1/oauth/google/callback"
		}

		providers[domain.OAuthProviderGoogle] = oauth.NewGoogleProvider(
			googleClientID,
			googleClientSecret,
			googleRedirectURI,
		)
		log.Println("✅ Google OAuth provider configured")
	}

	// Microsoft OAuth
	if microsoftClientID := os.Getenv("MICROSOFT_CLIENT_ID"); microsoftClientID != "" {
		microsoftClientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET")
		microsoftTenantID := os.Getenv("MICROSOFT_TENANT_ID")
		if microsoftTenantID == "" {
			microsoftTenantID = "common"
		}
		microsoftRedirectURI := os.Getenv("MICROSOFT_REDIRECT_URI")
		if microsoftRedirectURI == "" {
			microsoftRedirectURI = "http://localhost:8080/api/v1/oauth/microsoft/callback"
		}

		providers[domain.OAuthProviderMicrosoft] = oauth.NewMicrosoftProvider(
			microsoftClientID,
			microsoftClientSecret,
			microsoftRedirectURI,
			microsoftTenantID,
		)
		log.Println("✅ Microsoft OAuth provider configured")
	}

	// Okta OAuth
	if oktaClientID := os.Getenv("OKTA_CLIENT_ID"); oktaClientID != "" {
		oktaClientSecret := os.Getenv("OKTA_CLIENT_SECRET")
		oktaDomain := os.Getenv("OKTA_DOMAIN")
		oktaRedirectURI := os.Getenv("OKTA_REDIRECT_URI")
		if oktaRedirectURI == "" {
			oktaRedirectURI = "http://localhost:8080/api/v1/oauth/okta/callback"
		}

		if oktaDomain != "" {
			providers[domain.OAuthProviderOkta] = oauth.NewOktaProvider(
				oktaDomain,
				oktaClientID,
				oktaClientSecret,
				oktaRedirectURI,
			)
			log.Println("✅ Okta OAuth provider configured")
		} else {
			log.Println("⚠️  Okta OAuth provider not configured: missing OKTA_DOMAIN")
		}
	}

	return providers
}

func setupRoutes(v1 fiber.Router, h *Handlers, jwtService *auth.JWTService, sdkTokenRepo domain.SDKTokenRepository, db *sql.DB) {
	// SDK Token Tracking Middleware - TEMPORARILY DISABLED for debugging
	// sdkTokenTrackingMiddleware := middleware.NewSDKTokenTrackingMiddleware(sdkTokenRepo)
	// v1.Use(sdkTokenTrackingMiddleware.Handler()) // Apply to all API routes

	// ✅ Public routes (NO authentication required) - Self-registration API
	public := v1.Group("/public")
	public.Use(middleware.OptionalAuthMiddleware(jwtService)) // Try to extract user from JWT if present
	public.Post("/agents/register", h.PublicAgent.Register)   // 🚀 ONE-LINE agent registration
	public.Post("/register", h.PublicRegistration.RegisterUser) // 🚀 User registration
	public.Get("/register/:requestId/status", h.PublicRegistration.CheckRegistrationStatus) // Check registration status
	public.Post("/login", h.PublicRegistration.Login) // 🚀 Public login

	// Auth routes (no authentication required)
	auth := v1.Group("/auth")
	auth.Post("/login/local", h.Auth.LocalLogin)     // Local email/password login
	auth.Get("/login/:provider", h.Auth.Login)       // OAuth login
	auth.Get("/callback/:provider", h.Auth.Callback) // OAuth callback
	auth.Post("/logout", h.Auth.Logout)
	auth.Post("/refresh", h.AuthRefresh.RefreshToken) // Refresh access token (with token rotation)

	// Authenticated auth routes (authentication required)
	authProtected := v1.Group("/auth")
	authProtected.Use(middleware.AuthMiddleware(jwtService)) // Apply middleware using Use() instead of inline
	authProtected.Get("/me", h.Auth.Me)
	authProtected.Post("/change-password", h.Auth.ChangePassword)

	// SDK routes (authentication required) - Download pre-configured SDK
	sdk := v1.Group("/sdk")
	sdk.Use(middleware.AuthMiddleware(jwtService))
	sdk.Get("/download", h.SDK.DownloadSDK) // Download Python SDK with embedded credentials

	// SDK Token Management routes (authentication required)
	sdkTokens := v1.Group("/users/me/sdk-tokens")
	sdkTokens.Use(middleware.AuthMiddleware(jwtService))
	sdkTokens.Get("/", h.SDKToken.ListUserTokens)             // List all SDK tokens
	sdkTokens.Get("/count", h.SDKToken.GetActiveTokenCount)   // Get active token count
	sdkTokens.Post("/:id/revoke", h.SDKToken.RevokeToken)     // Revoke specific token
	sdkTokens.Post("/revoke-all", h.SDKToken.RevokeAllTokens) // Revoke all tokens

	// Note: SDK API routes moved to app level (main.go line 159) to avoid middleware inheritance

	// ⭐ MCP Detection endpoints - Using DIFFERENT path to avoid agents group conflict
	// Path: /api/v1/detection/agents/:id/report (instead of /api/v1/agents/:id/detection/report)
	// ✅ FIX: Use JWT authentication for web UI access, API key for SDK programmatic access
	detection := v1.Group("/detection")
	detection.Use(middleware.AuthMiddleware(jwtService)) // ✅ CHANGED: Use JWT middleware for web UI
	detection.Use(middleware.RateLimitMiddleware())
	detection.Post("/agents/:id/report", h.Detection.ReportDetection)
	detection.Get("/agents/:id/status", h.Detection.GetDetectionStatus) // ✅ Now accessible from web UI with JWT
	// ⭐ Agent Capability Detection endpoints - Report detected agent capabilities
	detection.Post("/agents/:id/capabilities/report", h.Detection.ReportCapabilities)

	// Agents routes - All other agent endpoints with JWT authentication
	agents := v1.Group("/agents")
	agents.Use(middleware.AuthMiddleware(jwtService))
	agents.Use(middleware.RateLimitMiddleware())
	agents.Get("/", h.Agent.ListAgents)
	agents.Post("/", middleware.MemberMiddleware(), h.Agent.CreateAgent)
	agents.Get("/:id", h.Agent.GetAgent)
	agents.Put("/:id", middleware.MemberMiddleware(), h.Agent.UpdateAgent)
	agents.Delete("/:id", middleware.ManagerMiddleware(), h.Agent.DeleteAgent)
	agents.Post("/:id/verify", middleware.ManagerMiddleware(), h.Agent.VerifyAgent)
	// Runtime verification endpoints - CORE functionality
	agents.Post("/:id/verify-action", h.Agent.VerifyAction)
	agents.Post("/:id/log-action/:audit_id", h.Agent.LogActionResult)
	// SDK download endpoint - Download Python/Node.js/Go SDK with embedded credentials
	agents.Get("/:id/sdk", h.Agent.DownloadSDK)
	// Credentials endpoint - Get raw Ed25519 public/private keys for manual integration
	agents.Get("/:id/credentials", h.Agent.GetCredentials)
	// MCP Server relationship management - "talks_to" endpoints
	agents.Get("/:id/mcp-servers", h.Agent.GetAgentMCPServers)                                                   // Get MCP servers agent talks to
	agents.Put("/:id/mcp-servers", middleware.MemberMiddleware(), h.Agent.AddMCPServersToAgent)                  // Add MCP servers (bulk)
	agents.Delete("/:id/mcp-servers/bulk", middleware.MemberMiddleware(), h.Agent.BulkRemoveMCPServersFromAgent) // Remove multiple MCPs
	agents.Delete("/:id/mcp-servers/:mcp_id", middleware.MemberMiddleware(), h.Agent.RemoveMCPServerFromAgent)   // Remove single MCP
	agents.Post("/:id/mcp-servers/detect", middleware.MemberMiddleware(), h.Agent.DetectAndMapMCPServers)        // Auto-detect MCPs from config

	// API keys routes (authentication required)
	apiKeys := v1.Group("/api-keys")
	apiKeys.Use(middleware.AuthMiddleware(jwtService))
	apiKeys.Use(middleware.RateLimitMiddleware())
	apiKeys.Get("/", h.APIKey.ListAPIKeys)
	apiKeys.Post("/", middleware.MemberMiddleware(), h.APIKey.CreateAPIKey)
	apiKeys.Patch("/:id/disable", middleware.MemberMiddleware(), h.APIKey.DisableAPIKey)
	apiKeys.Delete("/:id", middleware.MemberMiddleware(), h.APIKey.DeleteAPIKey)

	// Trust score routes (authentication required)
	trust := v1.Group("/trust-score")
	trust.Use(middleware.AuthMiddleware(jwtService))
	trust.Post("/calculate/:id", middleware.ManagerMiddleware(), h.TrustScore.CalculateTrustScore)
	trust.Get("/agents/:id", h.TrustScore.GetTrustScore)
	trust.Get("/agents/:id/history", h.TrustScore.GetTrustScoreHistory)
	trust.Get("/trends", h.TrustScore.GetTrustScoreTrends)

	// Admin routes (admin only)
	admin := v1.Group("/admin")
	admin.Use(middleware.AuthMiddleware(jwtService))
	admin.Use(middleware.AdminMiddleware())
	admin.Use(middleware.RateLimitMiddleware())

	// User management
	admin.Get("/users", h.Admin.ListUsers)
	admin.Get("/users/pending", h.Admin.GetPendingUsers)
	admin.Post("/users/:id/approve", h.Admin.ApproveUser)
	admin.Post("/users/:id/reject", h.Admin.RejectUser)
	admin.Put("/users/:id/role", h.Admin.UpdateUserRole)
	admin.Delete("/users/:id", h.Admin.DeactivateUser)

	// Registration request management (for pending OAuth registrations)
	admin.Post("/registration-requests/:id/approve", h.Admin.ApproveRegistrationRequest)
	admin.Post("/registration-requests/:id/reject", h.Admin.RejectRegistrationRequest)

	// Organization settings
	admin.Get("/organization/settings", h.Admin.GetOrganizationSettings)
	admin.Put("/organization/settings", h.Admin.UpdateOrganizationSettings)

	// Audit logs
	admin.Get("/audit-logs", h.Admin.GetAuditLogs)

	// Alerts
	admin.Get("/alerts", h.Admin.GetAlerts)
	admin.Post("/alerts/:id/acknowledge", h.Admin.AcknowledgeAlert)
	admin.Post("/alerts/:id/resolve", h.Admin.ResolveAlert)
	admin.Post("/alerts/:id/approve-drift", h.Admin.ApproveDrift)

	// Dashboard stats
	admin.Get("/dashboard/stats", h.Admin.GetDashboardStats)

	// Security Policy Management routes (admin only)
	admin.Get("/security-policies", h.SecurityPolicy.ListPolicies)
	admin.Get("/security-policies/:id", h.SecurityPolicy.GetPolicy)
	admin.Post("/security-policies", h.SecurityPolicy.CreatePolicy)
	admin.Put("/security-policies/:id", h.SecurityPolicy.UpdatePolicy)
	admin.Delete("/security-policies/:id", h.SecurityPolicy.DeletePolicy)
	admin.Patch("/security-policies/:id/toggle", h.SecurityPolicy.TogglePolicy)

	// Capability Request Management routes (admin only)
	admin.Get("/capability-requests", h.CapabilityRequest.ListCapabilityRequests)
	admin.Get("/capability-requests/:id", h.CapabilityRequest.GetCapabilityRequest)
	admin.Post("/capability-requests/:id/approve", h.CapabilityRequest.ApproveCapabilityRequest)
	admin.Post("/capability-requests/:id/reject", h.CapabilityRequest.RejectCapabilityRequest)

	// Compliance routes (admin only)
	// Basic compliance features - Advanced features (SOC 2, HIPAA, GDPR, ISO 27001) reserved for premium
	compliance := v1.Group("/compliance")
	compliance.Use(middleware.AuthMiddleware(jwtService))
	compliance.Use(middleware.AdminMiddleware())
	compliance.Use(middleware.StrictRateLimitMiddleware())
	compliance.Get("/status", h.Compliance.GetComplianceStatus)
	compliance.Get("/metrics", h.Compliance.GetComplianceMetrics)
	compliance.Get("/audit-log/export", h.Compliance.ExportAuditLog)
	compliance.Get("/access-review", h.Compliance.GetAccessReview)
	compliance.Post("/check", h.Compliance.RunComplianceCheck)

	// MCP Server routes (authentication required)
	mcpServers := v1.Group("/mcp-servers")
	mcpServers.Use(middleware.AuthMiddleware(jwtService))
	mcpServers.Use(middleware.RateLimitMiddleware())
	mcpServers.Get("/", h.MCP.ListMCPServers)
	mcpServers.Post("/", middleware.MemberMiddleware(), h.MCP.CreateMCPServer)
	mcpServers.Get("/:id", h.MCP.GetMCPServer)
	mcpServers.Put("/:id", middleware.MemberMiddleware(), h.MCP.UpdateMCPServer)
	mcpServers.Delete("/:id", middleware.ManagerMiddleware(), h.MCP.DeleteMCPServer)
	mcpServers.Post("/:id/verify", middleware.ManagerMiddleware(), h.MCP.VerifyMCPServer)
	mcpServers.Post("/:id/keys", middleware.MemberMiddleware(), h.MCP.AddPublicKey)
	mcpServers.Get("/:id/verification-status", h.MCP.GetVerificationStatus)
	mcpServers.Get("/:id/capabilities", h.MCP.GetMCPServerCapabilities) // ✅ Get detected capabilities
	mcpServers.Get("/:id/agents", h.MCP.GetMCPServerAgents)             // ✅ Get agents that talk to this MCP server
	// Runtime verification endpoint - CORE functionality
	mcpServers.Post("/:id/verify-action", h.MCP.VerifyMCPAction)

	// Security routes (admin/manager)
	security := v1.Group("/security")
	security.Use(middleware.AuthMiddleware(jwtService))
	security.Use(middleware.ManagerMiddleware())
	security.Use(middleware.RateLimitMiddleware())
	security.Get("/threats", h.Security.GetThreats)
	security.Get("/anomalies", h.Security.GetAnomalies)
	security.Get("/metrics", h.Security.GetSecurityMetrics)
	security.Get("/scan/:id", h.Security.RunSecurityScan)
	security.Get("/incidents", h.Security.GetIncidents)
	security.Post("/incidents/:id/resolve", h.Security.ResolveIncident)

	// Analytics routes (authentication required)
	analytics := v1.Group("/analytics")
	analytics.Use(middleware.AuthMiddleware(jwtService))
	analytics.Use(middleware.RateLimitMiddleware())
	analytics.Get("/dashboard", h.Analytics.GetDashboardStats) // Viewer-accessible dashboard stats
	analytics.Get("/usage", h.Analytics.GetUsageStatistics)
	analytics.Get("/trends", h.Analytics.GetTrustScoreTrends)
	analytics.Get("/reports/generate", h.Analytics.GenerateReport)
	analytics.Get("/agents/activity", h.Analytics.GetAgentActivity)

	// Webhook routes (authentication required)
	webhooks := v1.Group("/webhooks")
	webhooks.Use(middleware.AuthMiddleware(jwtService))
	webhooks.Use(middleware.RateLimitMiddleware())
	webhooks.Post("/", middleware.MemberMiddleware(), h.Webhook.CreateWebhook)
	webhooks.Get("/", h.Webhook.ListWebhooks)
	webhooks.Get("/:id", h.Webhook.GetWebhook)
	webhooks.Delete("/:id", middleware.MemberMiddleware(), h.Webhook.DeleteWebhook)
	webhooks.Post("/:id/test", middleware.MemberMiddleware(), h.Webhook.TestWebhook)

	// Verification Event routes (authentication required) - Real-time monitoring
	verificationEvents := v1.Group("/verification-events")
	verificationEvents.Use(middleware.AuthMiddleware(jwtService))
	verificationEvents.Use(middleware.RateLimitMiddleware())
	verificationEvents.Get("/", h.VerificationEvent.ListVerificationEvents)
	verificationEvents.Get("/recent", h.VerificationEvent.GetRecentEvents)
	verificationEvents.Get("/statistics", h.VerificationEvent.GetStatistics)
	verificationEvents.Get("/:id", h.VerificationEvent.GetVerificationEvent)
	verificationEvents.Post("/", middleware.MemberMiddleware(), h.VerificationEvent.CreateVerificationEvent)
	verificationEvents.Delete("/:id", middleware.ManagerMiddleware(), h.VerificationEvent.DeleteVerificationEvent)

	// OAuth routes (self-registration and admin approval)
	oauth := v1.Group("/oauth")
	oauth.Get("/:provider/login", h.OAuth.InitiateOAuth)
	oauth.Get("/:provider/callback", h.OAuth.HandleOAuthCallback)

	// Admin registration approval routes
	adminRegistrations := v1.Group("/admin/registration-requests")
	adminRegistrations.Use(middleware.AuthMiddleware(jwtService))
	adminRegistrations.Use(middleware.AdminMiddleware())
	adminRegistrations.Get("/", h.OAuth.ListPendingRegistrationRequests)
	adminRegistrations.Post("/:id/approve", h.OAuth.ApproveRegistrationRequest)
	adminRegistrations.Post("/:id/reject", h.OAuth.RejectRegistrationRequest)

	// Tag routes (authentication required)
	tags := v1.Group("/tags")
	tags.Use(middleware.AuthMiddleware(jwtService))
	tags.Use(middleware.RateLimitMiddleware())
	tags.Get("/", h.Tag.GetTags)
	tags.Post("/", middleware.MemberMiddleware(), h.Tag.CreateTag)
	tags.Delete("/:id", middleware.ManagerMiddleware(), h.Tag.DeleteTag)

	// Agent tag routes (under /agents/:id/tags)
	agents.Get("/:id/tags", h.Tag.GetAgentTags)
	agents.Post("/:id/tags", middleware.MemberMiddleware(), h.Tag.AddTagsToAgent)
	agents.Delete("/:id/tags/:tagId", middleware.MemberMiddleware(), h.Tag.RemoveTagFromAgent)
	agents.Get("/:id/tags/suggestions", h.Tag.SuggestTagsForAgent)

	// Agent capability routes (under /agents/:id/capabilities)
	agents.Get("/:id/capabilities", h.Capability.GetAgentCapabilities)
	agents.Post("/:id/capabilities", middleware.ManagerMiddleware(), h.Capability.GrantCapability)
	agents.Delete("/:id/capabilities/:capabilityId", middleware.ManagerMiddleware(), h.Capability.RevokeCapability)

	// Agent violation routes (under /agents/:id/violations)
	agents.Get("/:id/violations", h.Capability.GetViolationsByAgent)

	// Capability Request routes (authentication required)
	capabilityRequests := v1.Group("/capability-requests")
	capabilityRequests.Use(middleware.AuthMiddleware(jwtService))
	capabilityRequests.Use(middleware.RateLimitMiddleware())
	capabilityRequests.Post("/", h.CapabilityRequest.CreateCapabilityRequest) // Any authenticated user can request capabilities

	// MCP server tag routes (under /mcp-servers/:id/tags)
	mcpServers.Get("/:id/tags", h.Tag.GetMCPServerTags)
	mcpServers.Post("/:id/tags", middleware.MemberMiddleware(), h.Tag.AddTagsToMCPServer)
	mcpServers.Delete("/:id/tags/:tagId", middleware.MemberMiddleware(), h.Tag.RemoveTagFromMCPServer)
	mcpServers.Get("/:id/tags/suggestions", h.Tag.SuggestTagsForMCPServer)
}

func customErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error":     true,
		"message":   message,
		"timestamp": time.Now().UTC(),
	})
}

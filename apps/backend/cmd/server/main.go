package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"

	// "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/jmoiron/sqlx"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/config"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
	"github.com/opena2a/identity/backend/internal/infrastructure/cache"
	"github.com/opena2a/identity/backend/internal/infrastructure/email"
	"github.com/opena2a/identity/backend/internal/infrastructure/metrics"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
	"github.com/opena2a/identity/backend/internal/interfaces/http/handlers"
	"github.com/opena2a/identity/backend/internal/interfaces/http/middleware"
)

// @title Agent Identity Management API
// @version 1.0
// @description production-ready identity verification and security platform for AI agents and MCP servers
// @contact.name OpenA2A Team
// @contact.url https://opena2a.org
// @contact.email info@opena2a.org
// @license.name AGPL-3.0
// @license.url https://www.gnu.org/licenses/agpl-3.0.html
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Track start time for uptime calculation
	startTime := time.Now()

	// Load environment variables from project root
	// Backend runs from apps/backend, so go up 2 directories to find root .env
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("No .env file found in project root, using environment variables")
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

	// ⚡ Run database migrations automatically on startup
	// This ensures production deployments have correct schema without manual intervention
	if err := runMigrations(db); err != nil {
		log.Fatal("❌ Database migrations failed:", err)
	}
	log.Println("✅ Database migrations completed successfully")

	// Initialize Redis (optional - used for caching only)
	redisClient, err := initRedis(cfg)
	if err != nil {
		log.Printf("⚠️  Redis connection failed: %v", err)
		log.Println("ℹ️  AIM will continue without caching (Redis is optional)")
		redisClient = nil // Continue without Redis
	} else {
		defer redisClient.Close()
	}

	// Initialize repositories
	repos, oauthRepo := initRepositories(db)

	// Initialize cache (optional - skip if Redis is unavailable)
	var cacheService *cache.RedisCache
	if redisClient != nil {
		cacheService, err = cache.NewRedisCache(&cache.CacheConfig{
			Host:     cfg.Redis.Host,
			Port:     cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err != nil {
			log.Printf("⚠️  Cache initialization failed: %v", err)
			log.Println("ℹ️  AIM will continue without caching")
			cacheService = nil
		} else {
			log.Println("✅ Cache service initialized")
		}
	} else {
		log.Println("ℹ️  Cache service skipped (Redis unavailable)")
		cacheService = nil
	}

	// Initialize infrastructure services
	jwtService := auth.NewJWTService()

	// Initialize email service
	emailService, err := initEmailService()
	if err != nil {
		log.Printf("⚠️  Email service initialization failed: %v", err)
		log.Println("ℹ️  AIM will continue without email notifications")
		emailService = nil // Continue without email
	}

	// Initialize application services
	services, keyVault := initServices(db, repos, cacheService, oauthRepo, jwtService, emailService)

	// Initialize handlers
	h := initHandlers(services, repos, jwtService, keyVault, cfg, db)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:           "Agent Identity Management",
		ServerHeader:      "AIM/1.0",
		ErrorHandler:      customErrorHandler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadBufferSize:    16384, // 16KB header buffer (default is 4096) for OAuth callback URLs
		DisableKeepalive:  false,
		StreamRequestBody: false,
	})

	// Prometheus metrics endpoint (no auth required)
	// CRITICAL: Must be registered BEFORE Prometheus middleware to avoid circular recording
	app.Get("/metrics", metrics.PrometheusHandler())

	// Global middleware
	app.Use(middleware.RecoveryMiddleware())
	app.Use(middleware.LoggerMiddleware())
	app.Use(metrics.PrometheusMiddleware())    // Prometheus metrics collection
	app.Use(middleware.AnalyticsTracking(db)) // Real-time API call tracking
	// app.Use(middleware.RequestLoggerMiddleware())

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

		// Check Redis (optional - skip if not configured)
		redisStatus := "not configured"
		if redisClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				redisStatus = "unavailable (optional)"
			} else {
				redisStatus = "connected"
			}
		}

		return c.JSON(fiber.Map{
			"ready":    true,
			"database": "connected",
			"redis":    redisStatus,
		})
	})

	// System status endpoint (no auth required)
	app.Get("/api/v1/status", func(c fiber.Ctx) error {
		// Get environment (default to "development" if not set)
		environment := os.Getenv("ENVIRONMENT")
		if environment == "" {
			environment = "development"
		}

		// Check database status
		dbStatus := "healthy"
		if err := db.Ping(); err != nil {
			dbStatus = "unavailable"
		}

		// Check Redis status (optional)
		redisStatus := "not configured"
		if redisClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				redisStatus = "unavailable"
			} else {
				redisStatus = "healthy"
			}
		}

		// Check email service status
		emailStatus := "unavailable"
		if emailService != nil {
			emailStatus = "healthy"
		}

		return c.JSON(fiber.Map{
			"status":      "operational",
			"version":     "1.0.0",
			"environment": environment,
			"uptime":      time.Since(startTime).Seconds(),
			"services": fiber.Map{
				"database": dbStatus,
				"redis":    redisStatus,
				"email":    emailStatus,
			},
			"features": fiber.Map{
				"oauth":              false, // OAuth disabled
				"email_registration": true,
				"mcp_auto_detection": true,
				"trust_scoring":      true,
			},
		})
	})

	// ✅ Action verification for SDK (signature-based auth, NO API key required)
	// IMPORTANT: Register directly on app (not through group) to avoid API key middleware
	// These endpoints verify Ed25519 signatures instead of requiring API keys
	app.Post("/api/v1/sdk-api/verifications", middleware.RateLimitMiddleware(), h.Verification.CreateVerification)
	app.Get("/api/v1/sdk-api/verifications/:id", middleware.RateLimitMiddleware(), h.Verification.GetVerification)
	app.Post("/api/v1/sdk-api/verifications/:id/result", middleware.RateLimitMiddleware(), h.Verification.SubmitVerificationResult)

	// ⭐ SDK API routes - MUST be at app level to avoid middleware inheritance
	// These routes use Ed25519 agent authentication for SDK/programmatic access
	// Allows both Ed25519 (agent signatures) and JWT (user tokens) authentication
	sdkAPI := app.Group("/api/v1/sdk-api")
	sdkAPI.Use(middleware.Ed25519AgentMiddleware(services.Agent)) // Validates agent signatures, passes through JWT
	sdkAPI.Use(middleware.RateLimitMiddleware())
	sdkAPI.Get("/agents/:identifier", h.Agent.GetAgentByIdentifier)                             // Get agent by ID or name (SDK)
	sdkAPI.Post("/agents/:id/capabilities", h.Capability.GrantCapability)                       // SDK capability reporting
	sdkAPI.Post("/agents/:id/capability-requests", h.CapabilityRequest.CreateCapabilityRequest) // SDK capability request creation
	sdkAPI.Post("/agents/:id/mcp-servers", h.MCP.CreateMCPServer)                               // SDK MCP registration (create new MCP server)
	sdkAPI.Get("/agents/:id/mcp-servers", h.MCP.ListMCPServers)                                 // SDK list MCP servers for agent's org
	sdkAPI.Post("/agents/:id/mcp-connections", h.MCPAttestation.RecordMCPConnection)            // SDK record agent-MCP connection (use_mcp_tool)
	sdkAPI.Post("/agents/:id/detection/report", h.Detection.ReportDetection)                    // SDK MCP detection and integration reporting

	// API v1 routes (JWT authenticated)
	v1 := app.Group("/api/v1")
	setupRoutes(v1, h, services, jwtService, repos.SDKToken, db)

	// Start server
	port := cfg.Server.Port
	log.Printf("🚀 Agent Identity Management API starting on port %s", port)
	log.Printf("📊 Database: %s@%s:%d", cfg.Database.User, cfg.Database.Host, cfg.Database.Port)
	if redisClient != nil {
		log.Printf("💾 Redis: %s:%d (connected)", cfg.Redis.Host, cfg.Redis.Port)
	} else {
		log.Printf("💾 Redis: disabled (running without caching)")
	}

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
	// Build connection string using key=value format to avoid URL encoding issues
	// This format works better with passwords containing special characters
	connStr := fmt.Sprintf("host=%s port=%d user=%s password='%s' dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
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
	User               *repository.UserRepository
	Organization       *repository.OrganizationRepository
	Agent              *repository.AgentRepository
	APIKey             *repository.APIKeyRepository
	TrustScore         *repository.TrustScoreRepository
	AuditLog           *repository.AuditLogRepository
	Alert              *repository.AlertRepository
	MCPServer          *repository.MCPServerRepository
	MCPCapability      *repository.MCPServerCapabilityRepository // ✅ For MCP server capabilities
	MCPAttestation     *repository.MCPAttestationRepository      // ✅ For agent attestation of MCPs
	AgentMCPConnection *repository.AgentMCPConnectionRepository  // ✅ For agent-MCP connections
	Security           *repository.SecurityRepository
	SecurityPolicy     *repository.SecurityPolicyRepository // ✅ For configurable security policies
	Webhook            *repository.WebhookRepository
	VerificationEvent  *repository.VerificationEventRepositorySimple
	Tag                *repository.TagRepository
	SDKToken           domain.SDKTokenRepository
	Capability         domain.CapabilityRepository
	CapabilityRequest  domain.CapabilityRequestRepository // ✅ For capability expansion approval workflow
}

func initRepositories(db *sql.DB) (*Repositories, *repository.OAuthRepositoryPostgres) {
	// Wrap database with sqlx for repositories that need it (registration and capability repositories)
	dbx := sqlx.NewDb(db, "postgres")

	// Initialize registration repository for user registration workflow
	oauthRepo := repository.NewOAuthRepositoryPostgres(dbx)

	return &Repositories{
		User:               repository.NewUserRepository(db),
		Organization:       repository.NewOrganizationRepository(db),
		Agent:              repository.NewAgentRepository(db),
		APIKey:             repository.NewAPIKeyRepository(db),
		TrustScore:         repository.NewTrustScoreRepository(db),
		AuditLog:           repository.NewAuditLogRepository(db),
		Alert:              repository.NewAlertRepository(db),
		MCPServer:          repository.NewMCPServerRepository(db),
		MCPCapability:      repository.NewMCPServerCapabilityRepository(db), // ✅ For MCP server capabilities
		MCPAttestation:     repository.NewMCPAttestationRepository(db),      // ✅ For agent attestation of MCPs
		AgentMCPConnection: repository.NewAgentMCPConnectionRepository(dbx), // ✅ For agent-MCP connections
		Security:           repository.NewSecurityRepository(db),
		SecurityPolicy:     repository.NewSecurityPolicyRepository(db), // ✅ For configurable security policies
		Webhook:            repository.NewWebhookRepository(db),
		VerificationEvent:  repository.NewVerificationEventRepository(db),
		Tag:                repository.NewTagRepository(db),
		SDKToken:           repository.NewSDKTokenRepository(db),
		Capability:         repository.NewCapabilityRepository(dbx),
		CapabilityRequest:  repository.NewCapabilityRequestRepository(dbx), // ✅ For capability expansion approval workflow
	}, oauthRepo
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
	MCPCapability     *application.MCPCapabilityService  // ✅ For MCP server capability management
	MCPAttestation    *application.MCPAttestationService // ✅ For agent attestation of MCPs
	Security          *application.SecurityService
	SecurityPolicy    *application.SecurityPolicyService // ✅ For policy-based enforcement
	Webhook           *application.WebhookService
	VerificationEvent *application.VerificationEventService
	Registration      *application.RegistrationService // ✅ Email/password registration workflow (replaced OAuth)
	Tag               *application.TagService
	SDKToken          *application.SDKTokenService
	Capability        *application.CapabilityService
	CapabilityRequest *application.CapabilityRequestService // ✅ For capability expansion approval workflow
	Detection         *application.DetectionService         // ✅ For MCP auto-detection (SDK + Direct API)
}

func initServices(db *sql.DB, repos *Repositories, cacheService *cache.RedisCache, oauthRepo *repository.OAuthRepositoryPostgres, jwtService *auth.JWTService, emailService domain.EmailService) (*Services, *crypto.KeyVault) {
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
		emailService,          // ✅ For sending welcome/approval emails
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
		repos.Capability,
		repos.Agent, // For fetching agent data
		repos.Alert, // For security alerts scoring
	)

	// ✅ Initialize drift detection service BEFORE verification event service
	driftDetectionService := application.NewDriftDetectionService(
		repos.Agent,
		repos.Alert,
	)

	// ✅ Initialize verification event service BEFORE agent service
	verificationEventService := application.NewVerificationEventService(
		repos.VerificationEvent,
		repos.Agent,
		driftDetectionService,
	)

	agentService := application.NewAgentService(
		repos.Agent,
		trustCalculator,
		repos.TrustScore,
		keyVault,                 // ✅ NEW: Inject KeyVault for automatic key generation
		repos.Alert,              // ✅ NEW: Inject AlertRepository for security alerts
		securityPolicyService,    // ✅ NEW: Inject SecurityPolicyService for policy evaluation
		repos.Capability,         // ✅ NEW: Inject CapabilityRepository for capability checks
		verificationEventService, // ✅ NEW: Inject VerificationEventService for creating verification events
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
		keyVault,                 // ✅ For automatic key generation
		mcpCapabilityService,     // ✅ For automatic capability detection
		repos.MCPCapability,      // ✅ For creating SDK capabilities
		repos.AgentMCPConnection, // ✅ For tracking agent-MCP connections
		repos.Agent,              // ✅ For connected agents tracking
	)

	// ✅ Initialize MCP Attestation Service for agent attestation of MCPs
	mcpAttestationService := application.NewMCPAttestationService(
		repos.MCPAttestation,
		repos.Agent,
		repos.MCPServer,
		repos.User,
		repos.AgentMCPConnection,
	)

	securityService := application.NewSecurityService(
		repos.Security,
		repos.Agent,
		repos.Alert, // ✅ For converting alerts to threats (NO MOCK DATA!)
	)

	webhookService := application.NewWebhookService(
		repos.Webhook,
	)

	// Initialize RegistrationService for email/password user registration workflow
	registrationService := application.NewRegistrationService(
		oauthRepo, // Still uses oauth_repository for now (will be renamed in later step)
		repos.User,
		repos.Organization, // ✅ NEW: Organization repository for auto-creating orgs
		auditService,
		emailService, // ✅ NEW: Email service for password reset and admin notifications
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
		MCPCapability:     mcpCapabilityService,  // ✅ For MCP server capability management
		MCPAttestation:    mcpAttestationService, // ✅ For agent attestation of MCPs
		Security:          securityService,
		SecurityPolicy:    securityPolicyService, // ✅ For policy-based enforcement
		Webhook:           webhookService,
		VerificationEvent: verificationEventService,
		Registration:      registrationService, // ✅ Email/password registration workflow (replaced OAuth)
		Tag:               tagService,
		SDKToken:          sdkTokenService,
		Capability:        capabilityService,
		CapabilityRequest: capabilityRequestService, // ✅ For capability expansion approval workflow
		Detection:         detectionService,         // ✅ For MCP auto-detection (SDK + Direct API)
	}, keyVault
}

type Handlers struct {
	Auth               *handlers.AuthHandler
	Agent              *handlers.AgentHandler
	APIKey             *handlers.APIKeyHandler
	TrustScore         *handlers.TrustScoreHandler
	Admin              *handlers.AdminHandler
	Compliance         *handlers.ComplianceHandler
	MCP                *handlers.MCPHandler
	MCPAttestation     *handlers.MCPAttestationHandler // ✅ For agent attestation of MCPs
	Security           *handlers.SecurityHandler
	SecurityPolicy     *handlers.SecurityPolicyHandler // ✅ For policy management
	Analytics          *handlers.AnalyticsHandler
	Webhook            *handlers.WebhookHandler
	Verification       *handlers.VerificationHandler // ✅ For POST /verifications endpoint
	VerificationEvent  *handlers.VerificationEventHandler
	PublicAgent        *handlers.PublicAgentHandler
	PublicRegistration *handlers.PublicRegistrationHandler
	Tag                *handlers.TagHandler
	SDK                *handlers.SDKHandler
	SDKToken           *handlers.SDKTokenHandler
	AuthRefresh        *handlers.AuthRefreshHandler
	SDKTokenRecovery   *handlers.SDKTokenRecoveryHandler
	Capability         *handlers.CapabilityHandler
	Detection          *handlers.DetectionHandler          // ✅ For MCP auto-detection (SDK + Direct API)
	CapabilityRequest  *handlers.CapabilityRequestHandlers // ✅ For capability request approval
}

func initHandlers(services *Services, repos *Repositories, jwtService *auth.JWTService, keyVault *crypto.KeyVault, cfg *config.Config, db *sql.DB) *Handlers {
	return &Handlers{
		Auth: handlers.NewAuthHandler(
			services.Auth,
			jwtService,
			repos.Organization,
		),
		Agent: handlers.NewAgentHandler(
			services.Agent,
			services.MCP, // ✅ Inject MCPService for auto-detect MCPs feature
			services.Audit,
			services.APIKey,
			handlers.NewTrustScoreHandler(services.Trust, services.Agent, services.Audit),
			services.Alert,             // ✅ For creating security alerts on capability violations
			services.VerificationEvent, // ✅ For recording action verification attempts in Security Dashboard
			services.Capability,
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
			services.Registration, // ✅ Renamed from OAuth to Registration
		),
		Compliance: handlers.NewComplianceHandler(
			services.Compliance,
			services.Audit,
		),
		MCP: handlers.NewMCPHandler(
			services.MCP,
			services.MCPCapability, // ✅ For capability endpoint
			services.Audit,
			repos.Agent,             // ✅ For agent relationships ("Talks To")
			repos.VerificationEvent, // ✅ For verification events endpoint
		),
		MCPAttestation: handlers.NewMCPAttestationHandler(
			services.MCPAttestation,
			services.Audit,
		),
		Security: handlers.NewSecurityHandler(
			services.Security,
			services.Audit,
			services.Alert,
			services.Agent,
		),
		SecurityPolicy: handlers.NewSecurityPolicyHandler(
			services.SecurityPolicy,
		),
		Analytics: handlers.NewAnalyticsHandler(
			services.Agent,
			services.Audit,
			services.MCP,
			services.VerificationEvent,
			services.Auth,     // ✅ For fetching user counts
			services.Alert,    // ✅ For fetching alert counts
			services.Security, // ✅ For fetching incident counts
			db,                // Database connection for real-time analytics
		),
		Webhook: handlers.NewWebhookHandler(
			services.Webhook,
			services.Audit,
		),
		Verification: handlers.NewVerificationHandler(
			services.Agent,
			services.Audit,
			services.Trust,
			services.VerificationEvent,
		),
		VerificationEvent: handlers.NewVerificationEventHandler(
			services.VerificationEvent,
		),
		PublicAgent: handlers.NewPublicAgentHandler(
			services.Agent,
			services.Auth,
			keyVault,
		),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			services.Registration, // ✅ Renamed from OAuth to Registration
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
		SDKTokenRecovery: handlers.NewSDKTokenRecoveryHandler(
			services.SDKToken,
			jwtService,
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

func initEmailService() (domain.EmailService, error) {
	// Initialize email service from environment variables
	service, err := email.NewEmailService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize email service: %w", err)
	}

	// Validate connection
	if err := service.ValidateConnection(); err != nil {
		return nil, fmt.Errorf("email service connection validation failed: %w", err)
	}

	// Log successful initialization
	provider := os.Getenv("EMAIL_PROVIDER")
	if provider == "" {
		provider = "azure"
	}
	fromAddress := os.Getenv("EMAIL_FROM_ADDRESS")
	log.Printf("✅ Email service initialized (provider: %s, from: %s)", provider, fromAddress)

	return service, nil
}

func setupRoutes(v1 fiber.Router, h *Handlers, services *Services, jwtService *auth.JWTService, sdkTokenRepo domain.SDKTokenRepository, db *sql.DB) {
	// SDK Token Tracking Middleware - TEMPORARILY DISABLED for debugging
	// sdkTokenTrackingMiddleware := middleware.NewSDKTokenTrackingMiddleware(sdkTokenRepo)
	// v1.Use(sdkTokenTrackingMiddleware.Handler()) // Apply to all API routes

	// ✅ Public routes (NO authentication required) - Self-registration API
	public := v1.Group("/public")
	public.Use(middleware.OptionalAuthMiddleware(jwtService))                               // Try to extract user from JWT if present
	public.Post("/agents/register", h.PublicAgent.Register)                                 // 🚀 ONE-LINE agent registration
	public.Post("/register", h.PublicRegistration.RegisterUser)                             // 🚀 User registration
	public.Get("/register/:requestId/status", h.PublicRegistration.CheckRegistrationStatus) // Check registration status
	public.Post("/login", h.PublicRegistration.Login)                                       // 🚀 Public login
	public.Post("/change-password", h.PublicRegistration.ChangePassword)                    // 🚀 Forced password change (enterprise security)
	public.Post("/forgot-password", h.PublicRegistration.ForgotPassword)                    // 🚀 Password reset request
	public.Post("/reset-password", h.PublicRegistration.ResetPassword)                      // 🚀 Password reset with token
	public.Post("/request-access", h.PublicRegistration.RequestAccess)                      // 🚀 Request platform access (no password required)

	// Auth routes (no authentication required)
	auth := v1.Group("/auth")
	auth.Post("/login/local", h.Auth.LocalLogin) // Local email/password login
	auth.Post("/logout", h.Auth.Logout)
	auth.Post("/refresh", h.AuthRefresh.RefreshToken)                 // Refresh access token (with token rotation)
	auth.Post("/sdk/recover", h.SDKTokenRecovery.RecoverRevokedToken) // Recover revoked SDK tokens (zero downtime!)

	// Authenticated auth routes (authentication required)
	authProtected := v1.Group("/auth")
	authProtected.Use(middleware.AuthMiddleware(jwtService)) // Apply middleware using Use() instead of inline
	authProtected.Get("/me", h.Auth.Me)
	authProtected.Post("/change-password", h.Auth.ChangePassword)

	// Organization routes (authentication required)
	organizations := v1.Group("/organizations")
	organizations.Use(middleware.AuthMiddleware(jwtService))
	organizations.Get("/current", h.Auth.GetCurrentOrganization)

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
	detection.Use(middleware.Ed25519AgentMiddleware(services.Agent)) // ✅ Try Ed25519 first (for SDK agents)
	detection.Use(middleware.AuthMiddleware(jwtService))             // ✅ Fallback to JWT (for web UI)
	detection.Use(middleware.RateLimitMiddleware())
	detection.Post("/agents/:id/report", h.Detection.ReportDetection)
	detection.Get("/agents/:id/status", h.Detection.GetDetectionStatus) // ✅ Now accessible from web UI with JWT
	// ⭐ Agent Capability Detection endpoints - Report detected agent capabilities
	detection.Post("/agents/:id/capabilities/report", h.Detection.ReportCapabilities)
	detection.Get("/agents/:id/capabilities/latest", h.Detection.GetLatestCapabilityReport) // ✅ Fetch latest capability report

	// Agents routes - All other agent endpoints with dual authentication (Ed25519 or JWT)
	agents := v1.Group("/agents")
	agents.Use(middleware.Ed25519AgentMiddleware(services.Agent)) // ✅ Try Ed25519 first (for SDK agents)
	agents.Use(middleware.AuthMiddleware(jwtService))             // ✅ Fallback to JWT (for web UI)
	agents.Use(middleware.RateLimitMiddleware())
	agents.Get("/", h.Agent.ListAgents)
	agents.Post("/", middleware.MemberMiddleware(), h.Agent.CreateAgent)
	agents.Get("/:id", h.Agent.GetAgent)
	agents.Put("/:id", middleware.MemberMiddleware(), h.Agent.UpdateAgent)
	agents.Delete("/:id", middleware.ManagerMiddleware(), h.Agent.DeleteAgent)
	agents.Post("/:id/verify", middleware.ManagerMiddleware(), h.Agent.VerifyAgent)
	// Agent lifecycle management endpoints
	agents.Post("/:id/suspend", middleware.ManagerMiddleware(), h.Agent.SuspendAgent)
	agents.Post("/:id/reactivate", middleware.ManagerMiddleware(), h.Agent.ReactivateAgent)
	agents.Post("/:id/rotate-credentials", middleware.MemberMiddleware(), h.Agent.RotateCredentials)
	agents.Put("/:id/keys", middleware.MemberMiddleware(), h.Agent.UpdateAgentKeys) // SDK key registration
	// Runtime verification endpoints - CORE functionality
	agents.Post("/:id/verify-action", h.Agent.VerifyAction)
	agents.Post("/:id/log-action/:audit_id", h.Agent.LogActionResult)
	// SDK download endpoint - Download Python/Node.js/Go SDK with embedded credentials
	agents.Get("/:id/sdk", h.Agent.DownloadSDK)
	// Credentials endpoint - Get raw Ed25519 public/private keys for manual integration
	agents.Get("/:id/credentials", h.Agent.GetCredentials)
	// MCP Server relationship management - "talks_to" endpoints
	agents.Get("/:id/mcp-servers", h.MCPAttestation.GetAgentMCPServers)                                        // ✅ Get MCP servers agent is connected to (via attestation)
	agents.Put("/:id/mcp-servers", middleware.MemberMiddleware(), h.Agent.AddMCPServersToAgent)                // Add MCP servers (bulk)
	agents.Delete("/:id/mcp-servers/:mcp_id", middleware.MemberMiddleware(), h.Agent.RemoveMCPServerFromAgent) // Remove single MCP
	agents.Post("/:id/mcp-servers/detect", middleware.MemberMiddleware(), h.Agent.DetectAndMapMCPServers)      // Auto-detect MCPs from config
	// Trust Score management - RESTful endpoints under /agents/:id/trust-score/*
	agents.Get("/:id/trust-score", h.Agent.GetAgentTrustScore)                                                      // Get current trust score
	agents.Get("/:id/trust-score/history", h.Agent.GetAgentTrustScoreHistory)                                       // Get trust score history
	agents.Put("/:id/trust-score", middleware.AdminMiddleware(), h.Agent.UpdateAgentTrustScore)                     // Manually update score (admin)
	agents.Post("/:id/trust-score/recalculate", middleware.ManagerMiddleware(), h.Agent.RecalculateAgentTrustScore) // Recalculate score
	// Agent security endpoints - Key vault and audit logs per agent
	agents.Get("/:id/key-vault", h.Agent.GetAgentKeyVault)   // Get agent's key vault info (public key, expiration, rotation status)
	agents.Get("/:id/audit-logs", h.Agent.GetAgentAuditLogs) // Get audit logs for specific agent (with pagination)

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
	trust.Get("/agents/:id/breakdown", h.TrustScore.GetTrustScoreBreakdown) // Detailed breakdown with weights and contributions
	trust.Get("/agents/:id/history", h.TrustScore.GetTrustScoreHistory)

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

	// User lifecycle management (soft delete and hard delete)
	admin.Post("/users/:id/deactivate", h.Admin.DeactivateUser) // Soft delete - sets deleted_at
	admin.Post("/users/:id/activate", h.Admin.ActivateUser)     // Reactivate - clears deleted_at
	admin.Delete("/users/:id", h.Admin.PermanentlyDeleteUser)   // Hard delete - removes from database

	// Registration request management (for pending OAuth registrations)
	admin.Post("/registration-requests/:id/approve", h.Admin.ApproveRegistrationRequest)
	admin.Post("/registration-requests/:id/reject", h.Admin.RejectRegistrationRequest)

	// Organization settings (read-only - no SSO auto-approve in Community)
	admin.Get("/organization/settings", h.Admin.GetOrganizationSettings)

	// Audit logs
	admin.Get("/audit-logs", h.Admin.GetAuditLogs)

	// Alerts
	admin.Get("/alerts", h.Admin.GetAlerts)
	admin.Get("/alerts/unacknowledged/count", h.Admin.GetUnacknowledgedAlertCount)
	admin.Post("/alerts/:id/acknowledge", h.Admin.AcknowledgeAlert)
	admin.Post("/alerts/:id/resolve", h.Admin.ResolveAlert)

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
	compliance.Use(middleware.RateLimitMiddleware()) // Changed from StrictRateLimitMiddleware to allow multiple simultaneous requests
	compliance.Get("/status", h.Compliance.GetComplianceStatus)
	compliance.Get("/metrics", h.Compliance.GetComplianceMetrics)
	compliance.Get("/audit-log/access-review", h.Compliance.GetAccessReview)
	compliance.Get("/access-review", h.Compliance.GetAccessReview)
	compliance.Post("/check", h.Compliance.RunComplianceCheck)
	compliance.Get("/export", h.Compliance.ExportComplianceReport) // Export compliance report
	// Data retention and violations endpoints removed

	// MCP Server routes (authentication required)
	// ✅ Agent Attestation endpoints - Revolutionary zero-effort MCP verification
	// CRITICAL: These MUST be registered BEFORE JWT-protected routes to avoid middleware conflicts
	// These endpoints use Ed25519 authentication (agent-to-backend) instead of JWT (user-to-backend)
	mcpServersAgentAuth := v1.Group("/mcp-servers")
	mcpServersAgentAuth.Use(middleware.Ed25519AgentMiddleware(services.Agent)) // Ed25519 signature verification
	mcpServersAgentAuth.Use(middleware.RateLimitMiddleware())
	mcpServersAgentAuth.Post("/:id/attest", h.MCPAttestation.AttestMCP)               // ✅ Submit agent attestation (Ed25519 signed)
	mcpServersAgentAuth.Get("/:id/attestations", h.MCPAttestation.GetMCPAttestations) // ✅ Get all attestations for this MCP
	mcpServersAgentAuth.Get("/:id/agents", h.MCPAttestation.GetConnectedAgents)       // ✅ Get agents connected to this MCP (via attestation)

	// Standard MCP Server management endpoints - Use JWT authentication (user-to-backend)
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
	mcpServers.Get("/:id/capabilities", h.MCP.GetMCPServerCapabilities)                                    // ✅ Get detected capabilities
	mcpServers.Get("/:id/verification-events", h.MCP.GetMCPVerificationEvents)                             // ✅ Get verification events for MCP server
	mcpServers.Post("/:id/manual-attest", middleware.MemberMiddleware(), h.MCPAttestation.ManualAttestMCP) // ✅ Manual attestation (non-SDK users)
	// Runtime verification endpoint - CORE functionality
	mcpServers.Post("/:id/verify-action", h.MCP.VerifyMCPAction)

	// Security routes (admin/manager)
	security := v1.Group("/security")
	security.Use(middleware.AuthMiddleware(jwtService))
	security.Use(middleware.ManagerMiddleware())
	security.Use(middleware.RateLimitMiddleware())
	security.Get("/dashboard", h.Security.GetSecurityDashboard)
	security.Get("/alerts", h.Security.ListSecurityAlerts)
	security.Get("/threats", h.Security.GetThreats)
	security.Get("/anomalies", h.Security.GetAnomalies)
	security.Get("/metrics", h.Security.GetSecurityMetrics)

	// Analytics routes (authentication required)
	analytics := v1.Group("/analytics")
	analytics.Use(middleware.AuthMiddleware(jwtService))
	analytics.Use(middleware.RateLimitMiddleware())
	analytics.Get("/dashboard", h.Analytics.GetDashboardStats) // Viewer-accessible dashboard stats
	analytics.Get("/usage", h.Analytics.GetUsageStatistics)
	analytics.Get("/activity", h.Analytics.GetActivitySummary)
	analytics.Get("/trends", h.Analytics.GetTrustScoreTrends)
	analytics.Get("/verification-activity", h.Analytics.GetVerificationActivity) // New endpoint for chart
	analytics.Get("/agents/activity", h.Analytics.GetAgentActivity)

	// Webhook routes (authentication required)
	webhooks := v1.Group("/webhooks")
	webhooks.Use(middleware.AuthMiddleware(jwtService))
	webhooks.Use(middleware.RateLimitMiddleware())
	webhooks.Post("/", middleware.MemberMiddleware(), h.Webhook.CreateWebhook)
	webhooks.Get("/", h.Webhook.ListWebhooks)
	webhooks.Get("/:id", h.Webhook.GetWebhook)
	webhooks.Put("/:id", middleware.MemberMiddleware(), h.Webhook.UpdateWebhook) // Update webhook
	webhooks.Delete("/:id", middleware.MemberMiddleware(), h.Webhook.DeleteWebhook)
	webhooks.Post("/:id/test", h.Webhook.TestWebhook) // Test webhook endpoint

	// Verification routes (authentication required) - Agent action verification
	verifications := v1.Group("/verifications")
	verifications.Use(middleware.AuthMiddleware(jwtService))
	verifications.Use(middleware.RateLimitMiddleware())
	verifications.Post("/", h.Verification.CreateVerification)                 // Request verification for agent action
	verifications.Get("/:id", h.Verification.GetVerification)                  // Get verification status by ID
	verifications.Post("/:id/result", h.Verification.SubmitVerificationResult) // Submit verification result

	// Verification Event routes (authentication required) - Real-time monitoring
	verificationEvents := v1.Group("/verification-events")
	verificationEvents.Use(middleware.AuthMiddleware(jwtService))
	verificationEvents.Use(middleware.RateLimitMiddleware())
	verificationEvents.Get("/", h.VerificationEvent.ListVerificationEvents)
	verificationEvents.Get("/recent", h.VerificationEvent.GetRecentEvents)
	verificationEvents.Get("/statistics", h.VerificationEvent.GetStatistics)
	verificationEvents.Get("/stats", h.VerificationEvent.GetVerificationStats)           // ✅ Get aggregated verification stats
	verificationEvents.Get("/agent/:id", h.VerificationEvent.GetAgentVerificationEvents) // ✅ Get events for specific agent
	verificationEvents.Get("/mcp/:id", h.VerificationEvent.GetMCPVerificationEvents)     // ✅ Get events for specific MCP server
	verificationEvents.Get("/:id", h.VerificationEvent.GetVerificationEvent)
	verificationEvents.Post("/", middleware.MemberMiddleware(), h.VerificationEvent.CreateVerificationEvent)
	verificationEvents.Delete("/:id", middleware.ManagerMiddleware(), h.VerificationEvent.DeleteVerificationEvent)

	// Tag routes (authentication required)
	tags := v1.Group("/tags")
	tags.Use(middleware.AuthMiddleware(jwtService))
	tags.Use(middleware.RateLimitMiddleware())
	tags.Get("/", h.Tag.GetTags)
	tags.Post("/", middleware.MemberMiddleware(), h.Tag.CreateTag)
	tags.Put("/:id", middleware.MemberMiddleware(), h.Tag.UpdateTag)
	tags.Get("/popular", h.Tag.GetPopularTags)
	tags.Get("/search", h.Tag.SearchTags)
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

	// Capabilities routes (authentication required) - List all available capability types
	capabilities := v1.Group("/capabilities")
	capabilities.Use(middleware.AuthMiddleware(jwtService))
	capabilities.Get("/", h.Capability.ListCapabilities)

	// Capability Request routes (authentication required)
	capabilityRequests := v1.Group("/capability-requests")
	capabilityRequests.Use(middleware.AuthMiddleware(jwtService))
	capabilityRequests.Use(middleware.RateLimitMiddleware())

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

	// 🔍 LOG ALL ERRORS for debugging
	log.Printf("❌ ERROR [%d] %s %s - %v", code, c.Method(), c.Path(), err)

	return c.Status(code).JSON(fiber.Map{
		"error":     true,
		"message":   message,
		"timestamp": time.Now().UTC(),
	})
}

// runMigrations executes all pending database migrations automatically on startup
// This ensures production deployments have the correct schema without manual intervention
func runMigrations(db *sql.DB) error {
	log.Println("🔄 Running database migrations...")

	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get migration files from migrations directory
	files, err := getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	if len(files) == 0 {
		log.Println("ℹ️  No migration files found")
		return nil
	}

	// Get applied migrations from database
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	pendingCount := 0
	for _, file := range files {
		version := getMigrationVersion(file)
		if applied[version] {
			log.Printf("⏭️  Skipping %s (already applied)", file)
			continue
		}

		log.Printf("🔄 Applying %s...", file)

		// Read migration file
		content, err := ioutil.ReadFile(filepath.Join("migrations", file))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		// Execute migration in a transaction for safety
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction for %s: %w", file, err)
		}

		// Execute migration SQL
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute %s: %w", file, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}

		log.Printf("✅ Applied %s", file)
		pendingCount++
	}

	if pendingCount == 0 {
		log.Println("ℹ️  All migrations already applied (database is up to date)")
	} else {
		log.Printf("✅ Successfully applied %d pending migration(s)", pendingCount)
	}

	return nil
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func createMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			version VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(query)
	return err
}

// getMigrationFiles returns sorted list of .up.sql migration files
func getMigrationFiles() ([]string, error) {
	files, err := ioutil.ReadDir("migrations")
	if err != nil {
		// If migrations directory doesn't exist, return empty list (not an error)
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		// Only include .up.sql files for forward migrations
		if strings.HasSuffix(file.Name(), ".up.sql") ||
			(strings.HasSuffix(file.Name(), ".sql") && !strings.Contains(file.Name(), ".down.sql")) {
			migrations = append(migrations, file.Name())
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}

// getAppliedMigrations returns map of already-applied migration versions
func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

// getMigrationVersion extracts version from migration filename
func getMigrationVersion(filename string) string {
	// Use full filename as version for unique tracking
	return filename
}

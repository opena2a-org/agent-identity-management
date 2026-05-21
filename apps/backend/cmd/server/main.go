package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"database/sql"
	"fmt"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	domainsecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	infraatc "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email"
	infrasecrets "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/telemetry"
)

// @title Agent Identity Management API
// @version 1.0
// @description production-ready identity verification and security platform for AI agents and MCP servers
// @contact.name OpenA2A Team
// @contact.url https://opena2a.org
// @contact.email info@opena2a.org
// @license.name Apache-2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0
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

	// Initialize OpenTelemetry (traces + metrics + logs to OTLP collector).
	// Defaults to localhost:4317 (the demo stack at apps/backend/deployments/otel-demo).
	// Override with OTEL_EXPORTER_OTLP_ENDPOINT. Failure is non-fatal: the server
	// boots without telemetry if the collector is unreachable.
	otelShutdown, otelErr := telemetry.Init(context.Background(), telemetry.Config{
		ServiceName:    "aim-backend",
		ServiceVersion: "0.x",
		Insecure:       true,
	})
	if otelErr != nil {
		log.Printf("⚠️  OpenTelemetry init failed: %v (continuing without telemetry)", otelErr)
	} else {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(shutdownCtx); err != nil {
				log.Printf("OpenTelemetry shutdown error: %v", err)
			}
		}()
		log.Println("✅ OpenTelemetry initialized (OTLP gRPC)")
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

	// Override default admin password if ADMIN_PASSWORD env var is set
	if err := applyAdminPasswordOverride(db); err != nil {
		log.Printf("⚠️  Failed to apply admin password override: %v", err)
	}

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

	// Wire up alert repository to MCPCapability repository for drift alerts
	repos.MCPCapability.SetAlertRepository(repos.Alert)

	// Initialize cache (optional - skip if Redis is unavailable)
	var cacheService *cache.RedisCache
	if redisClient != nil {
		cacheService, err = cache.NewRedisCache(&cache.CacheConfig{
			Host:     cfg.Redis.Host,
			Port:     cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			UseTLS:   cfg.Redis.UseTLS,
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
	app.Use(middleware.SecurityHeadersMiddleware()) // SECURITY: Add HTTP security headers (HSTS, CSP, etc.)
	app.Use(middleware.LoggerMiddleware())
	app.Use(metrics.PrometheusMiddleware())   // Prometheus metrics collection
	app.Use(middleware.AnalyticsTracking(db)) // Real-time API call tracking
	// app.Use(middleware.RequestLoggerMiddleware())

	// CORS with allowed origins from environment
	// IMPORTANT: Frontend ALWAYS runs on port 3000, backend on port 8080
	// SECURITY: CORS origins are validated by CORSMiddleware to reject wildcards
	allowedOrigins := []string{
		"http://localhost:3000",
	}
	if customOrigins := os.Getenv("ALLOWED_ORIGINS"); customOrigins != "" {
		// Support multiple comma-separated origins in environment variable
		allowedOrigins = strings.Split(customOrigins, ",")
	}
	// CORSMiddleware will validate and sanitize these origins (rejecting wildcards)
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
				"oauth":              os.Getenv("GOOGLE_CLIENT_ID") != "",
				"email_registration": true,
				"mcp_auto_detection": true,
				"trust_scoring":      true,
			},
		})
	})

	// A2A Discovery endpoint at root (no auth required)
	// Standard A2A protocol requires /.well-known/agent.json at the root
	app.Get("/.well-known/agent.json", h.A2A.GetPublicAgentCard)

	// AIP Discovery endpoint (no auth required)
	// Returns provider capabilities, supported agent types, and endpoint directory
	app.Get("/.well-known/aip", h.AIP.WellKnownAIP)

	// DID Resolution endpoint (no auth required)
	// Resolves did:aip:aim_<uuid> to a W3C DID Document
	app.Get("/api/v1/did/*", h.AIP.ResolveDID)

	// NOTE: Revocation list route moved into setupRoutes() to avoid Fiber v3 beta
	// route shadowing when the /api/v1 group is registered with middleware.

	// Action verification for SDK (signature-based auth, NO API key required)
	// IMPORTANT: Register directly on app (not through group) to avoid API key middleware
	// These endpoints verify Ed25519 signatures instead of requiring API keys
	app.Post("/api/v1/sdk-api/verifications", middleware.RateLimitMiddleware(), h.Verification.CreateVerification)
	app.Get("/api/v1/sdk-api/verifications/:id", middleware.RateLimitMiddleware(), h.Verification.GetVerification)
	app.Post("/api/v1/sdk-api/verifications/:id/result", middleware.RateLimitMiddleware(), h.Verification.SubmitVerificationResult)
	app.Post("/api/v1/sdk-api/verifications/:id/execution-status", middleware.RateLimitMiddleware(), h.Verification.UpdateExecutionStatus)

	// ⭐ SDK API routes - MUST be at app level to avoid middleware inheritance
	// These routes use Ed25519 agent authentication for SDK/programmatic access
	// Allows both Ed25519 (agent signatures) and JWT (user tokens) authentication
	sdkAPI := app.Group("/api/v1/sdk-api")
	sdkAPI.Use(middleware.PQCAgentMiddleware(services.Agent)) // Validates agent signatures (Ed25519, ML-DSA, or hybrid), passes through JWT
	sdkAPI.Use(middleware.AuthMiddleware(jwtService))             // Fallback to JWT if Ed25519 not present
	sdkAPI.Use(middleware.RateLimitMiddleware())
	sdkAPI.Get("/agents/:identifier", h.Agent.GetAgentByIdentifier)                                // Get agent by ID or name (SDK)
	sdkAPI.Post("/agents/:id/capabilities", h.Capability.GrantCapability)                          // SDK capability reporting (legacy)
	sdkAPI.Post("/agents/:id/capabilities/register", h.Capability.RegisterCapability)              // SDK capability registration (respects enforcement mode)
	sdkAPI.Get("/agents/:id/capability-requests", h.CapabilityRequest.ListAgentCapabilityRequests) // SDK list agent's capability requests
	sdkAPI.Post("/agents/:id/capability-requests", h.CapabilityRequest.CreateCapabilityRequest)    // SDK capability request creation
	sdkAPI.Post("/agents/:id/mcp-servers", h.MCP.CreateMCPServer)                               // SDK MCP registration (create new MCP server)
	sdkAPI.Get("/agents/:id/mcp-servers", h.MCP.ListMCPServers)                                 // SDK list MCP servers for agent's org
	sdkAPI.Get("/agents/:id/mcp-servers/by-name", h.MCP.GetMCPServerByName)                    // SDK get MCP by name (capability caching)
	sdkAPI.Post("/agents/:id/mcp-connections", h.MCPAttestation.RecordMCPConnection)            // SDK record agent-MCP connection (use_mcp_tool)
	sdkAPI.Post("/agents/:id/mcp-usage-report", h.MCPAttestation.RecordMCPUsageReport)         // SDK MCP supply chain usage analytics
	sdkAPI.Post("/agents/:id/detection/report", h.Detection.ReportDetection)                    // SDK MCP detection and integration reporting
	sdkAPI.Post("/agents/:id/heartbeat", h.Lifecycle.Heartbeat)                                  // SDK agent heartbeat (liveness)

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

	// Start background job for automatic JIT access expiration
	// This ensures expired verifications are marked as such even if not queried
	stopExpirationJob := startExpirationCleanupJob(db)
	defer close(stopExpirationJob)

	// Start Registry Bridge background job (opt-in via REGISTRY_BRIDGE_ENABLED=true)
	if services.RegistryBridge != nil {
		stopRegistryBridge := startRegistryBridgeJob(services.RegistryBridge)
		defer close(stopRegistryBridge)
	}

	// Start Community Intelligence background job (pushes every 6 hours for opted-in orgs)
	if services.CommunityIntelligence != nil {
		stopCI := startCommunityIntelligenceJob(services.CommunityIntelligence)
		defer close(stopCI)
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

	// Drain the FGA async intent-check worker pool. Bound the wait at 10s
	// so we don't deadlock shutdown on a hung NanoMind daemon — the
	// per-call HTTP timeout (800ms) caps individual workers regardless.
	if services.FGA != nil {
		fgaShutdownCtx, fgaShutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := services.FGA.Shutdown(fgaShutdownCtx); err != nil {
			log.Printf("⚠️  FGA engine shutdown timed out: %v", err)
		}
		fgaShutdownCancel()
	}

	log.Println("Server exited")
}

func initDatabase(cfg *config.Config) (*sql.DB, error) {
	// Build connection string using key=value format to avoid URL encoding issues
	// This format works better with passwords containing special characters
	// SECURITY: Escape single quotes in password for libpq connection string format
	// PostgreSQL libpq requires single quotes to be doubled (''), not backslash-escaped
	escapedPassword := strings.ReplaceAll(cfg.Database.Password, "'", "''")
	connStr := fmt.Sprintf("host=%s port=%d user=%s password='%s' dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		escapedPassword,
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
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	if cfg.Redis.UseTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

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
	SecurityPolicy     *repository.SecurityPolicyRepository   // ✅ For configurable security policies
	BehaviorBaseline   *repository.BehaviorBaselineRepository // ✅ For intelligent behavioral anomaly detection
	Webhook            *repository.WebhookRepository
	VerificationEvent  *repository.VerificationEventRepositorySimple
	Tag                *repository.TagRepository
	SDKToken           domain.SDKTokenRepository
	Capability         domain.CapabilityRepository
	CapabilityRequest  domain.CapabilityRequestRepository // ✅ For capability expansion approval workflow
	AuthFailure        *repository.AuthFailureRepository  // ✅ For failed authentication monitoring
	DataTransfer       *repository.DataTransferRepository // ✅ For data exfiltration detection
	// A2A (Agent-to-Agent) protocol repositories
	A2AAgentCard    *repository.A2AAgentCardRepository
	A2ASkill        *repository.A2ASkillRepository
	A2ATask         *repository.A2ATaskRepository
	A2APeerTrust    *repository.A2APeerTrustRepository
	A2AConsent      *repository.A2AConsentRepository
	A2ATrustScore   *repository.A2ATrustScoreRepository
	A2ARequestNonce   *repository.A2ARequestNonceRepository
	A2APolicy         *repository.A2APolicyRepository
	A2AAttestation        *repository.A2AAgentAttestationRepository
	A2ARevokedAgent       *repository.A2ARevokedAgentRepository
	A2ASecuritySettings   *repository.A2ASecuritySettingsRepository
	A2ASecurityViolation  *repository.A2ASecurityViolationRepository
	DeviceCode              *repository.DeviceCodeRepository
	CommunityIntelligence   *repository.CommunityIntelligenceRepository // For community intelligence opt-in telemetry
	Remediation             *repository.RemediationRepository            // For remediation tracking (agentpwn + HMA)
	// Secrets management repositories
	SecretNamespace     *repository.SecretNamespaceRepository
	SecretCredential    *repository.SecretCredentialRepository
	SecretAudit         *repository.SecretAuditRepository
	SecretBackendConfig *repository.SecretBackendConfigRepository
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
		SecurityPolicy:     repository.NewSecurityPolicyRepository(db),   // ✅ For configurable security policies
		BehaviorBaseline:   repository.NewBehaviorBaselineRepository(db), // ✅ For intelligent behavioral anomaly detection
		Webhook:            repository.NewWebhookRepository(db),
		VerificationEvent:  repository.NewVerificationEventRepository(db),
		Tag:                repository.NewTagRepository(db),
		SDKToken:           repository.NewSDKTokenRepository(db),
		Capability:         repository.NewCapabilityRepository(dbx),
		CapabilityRequest:  repository.NewCapabilityRequestRepository(dbx), // ✅ For capability expansion approval workflow
		AuthFailure:        repository.NewAuthFailureRepository(db),        // ✅ For failed authentication monitoring
		DataTransfer:       repository.NewDataTransferRepository(db),       // ✅ For data exfiltration detection
		// A2A (Agent-to-Agent) protocol repositories
		A2AAgentCard:    repository.NewA2AAgentCardRepository(db),
		A2ASkill:        repository.NewA2ASkillRepository(db),
		A2ATask:         repository.NewA2ATaskRepository(db),
		A2APeerTrust:    repository.NewA2APeerTrustRepository(db),
		A2AConsent:      repository.NewA2AConsentRepository(db),
		A2ATrustScore:   repository.NewA2ATrustScoreRepository(db),
		A2ARequestNonce:   repository.NewA2ARequestNonceRepository(db),
		A2APolicy:         repository.NewA2APolicyRepository(db),
		A2AAttestation:        repository.NewA2AAgentAttestationRepository(db),
		A2ARevokedAgent:       repository.NewA2ARevokedAgentRepository(db),
		A2ASecuritySettings:   repository.NewA2ASecuritySettingsRepository(db),
		A2ASecurityViolation:  repository.NewA2ASecurityViolationRepository(db),
		DeviceCode:            repository.NewDeviceCodeRepository(db),
		CommunityIntelligence: repository.NewCommunityIntelligenceRepository(db),
		Remediation:           repository.NewRemediationRepository(db),
		// Secrets management
		SecretNamespace:     repository.NewSecretNamespaceRepository(db),
		SecretCredential:    repository.NewSecretCredentialRepository(db),
		SecretAudit:         repository.NewSecretAuditRepository(db),
		SecretBackendConfig: repository.NewSecretBackendConfigRepository(db),
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
	MCPCapability     *application.MCPCapabilityService     // ✅ For MCP server capability management
	MCPAttestation    *application.MCPAttestationService    // ✅ For agent attestation of MCPs
	Security          *application.SecurityService
	SecurityPolicy    *application.SecurityPolicyService    // ✅ For policy-based enforcement
	BehaviorAnalysis  *application.BehaviorAnalysisService  // ✅ For intelligent behavioral anomaly detection
	Webhook           *application.WebhookService
	VerificationEvent *application.VerificationEventService
	Registration      *application.RegistrationService // ✅ Email/password registration workflow (replaced OAuth)
	Tag               *application.TagService
	SDKToken          *application.SDKTokenService
	Capability        *application.CapabilityService
	CapabilityRequest *application.CapabilityRequestService // ✅ For capability expansion approval workflow
	Detection         *application.DetectionService         // ✅ For MCP auto-detection (SDK + Direct API)
	A2A               *application.A2AService               // ✅ For A2A (Agent-to-Agent) protocol
	DeviceAuth        *application.DeviceAuthService        // For OAuth Device Authorization Grant (RFC 8628)
	RegistryBridge          *application.RegistryBridgeService          // For OpenA2A Registry attestation contribution
	CommunityIntelligence   *application.CommunityIntelligenceService   // For community intelligence opt-in telemetry
	Remediation             *application.RemediationService             // For remediation tracking (agentpwn + HMA)
	Secrets                 *application.SecretsService                 // For identity-native secrets management
	FGA                     *application.FGAEngine                      // Fine-Grained Authorization engine (5-step decision flow)
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
		repos.AuditLog,
	)

	// Create services
	authService := application.NewAuthService(
		repos.User,
		repos.Organization,
		repos.APIKey,
		repos.AuthFailure,     // ✅ For failed authentication monitoring
		securityPolicyService, // ✅ For auto-creating default policies
		emailService,          // ✅ For sending welcome/approval emails
	)

	adminService := application.NewAdminService(
		repos.User,
		repos.Organization,
	)

	auditService := application.NewAuditService(repos.AuditLog)

	trustCalculator := application.NewTrustCalculatorWithVerification(
		repos.TrustScore,
		repos.APIKey,
		repos.AuditLog,
		repos.Capability,
		repos.Agent,             // For fetching agent data
		repos.Alert,             // For security alerts scoring
		repos.VerificationEvent, // For real verification statistics
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

	// Construct the capability-request workflow service BEFORE the agent
	// service. AgentService.UpdateAgent (re-registration) routes new
	// capability declarations through CapabilityRequestService so the
	// mode-aware policy (monitoring auto-approves, strict creates pending
	// request) is enforced in one place rather than duplicated.
	capabilityRequestService := application.NewCapabilityRequestService(
		repos.CapabilityRequest,
		repos.Capability,
		repos.Agent,
		repos.Organization,
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
		repos.Tag,                // ✅ NEW: Inject TagRepository for tagging agents during registration
		repos.User,               // ✅ NEW: Inject UserRepository for audit trail (creator/updater info)
		repos.Organization,       // ✅ NEW: Inject OrganizationRepository for checking enforcement mode
		capabilityRequestService, // NEW: For mode-aware capability approval workflow on re-registration
	)

	apiKeyService := application.NewAPIKeyService(
		repos.APIKey,
		repos.Agent,
	)

	alertService := application.NewAlertService(
		repos.Alert,
		repos.Agent,
		db,
	)

	// ✅ Initialize BehaviorAnalysisService for intelligent anomaly detection
	behaviorAnalysisService := application.NewBehaviorAnalysisService(
		db,
		repos.BehaviorBaseline,
		alertService,
	)

	// ✅ Wire BehaviorAnalysisService into SecurityPolicyService for smart detection
	securityPolicyService.SetBehaviorAnalysis(behaviorAnalysisService)

	// ✅ Wire AgentRepository into SecurityPolicyService for trust score enforcement (suspending agents)
	securityPolicyService.SetAgentRepository(repos.Agent)

	// ✅ Wire DataTransferRepository into SecurityPolicyService for exfiltration detection
	securityPolicyService.SetDataTransferRepository(repos.DataTransfer)

	complianceService := application.NewComplianceService(
		repos.AuditLog,
		repos.Agent,
		repos.User,
		repos.MCPServer,
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
		repos.Tag,                // ✅ For tagging MCP servers during registration
	)

	// ✅ Initialize MCP Attestation Service for agent attestation of MCPs
	// Uses alert-enabled constructor to create security alerts for low-trust attestation attempts
	mcpAttestationService := application.NewMCPAttestationServiceWithAlerts(
		repos.MCPAttestation,
		repos.Agent,
		repos.MCPServer,
		repos.User,
		repos.AgentMCPConnection,
		repos.MCPCapability, // ✅ For syncing capabilities from attestations
		repos.Alert,         // ✅ For low-trust attestation alerts
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
		repos.Alert,
		trustCalculator,
		repos.TrustScore,
	)

	detectionService := application.NewDetectionService(
		db,
		trustCalculator, // ✅ NEW: Inject trust calculator for proper risk assessment
		repos.Agent,     // ✅ NEW: Inject agent repository to fetch agent data
	)

	// ✅ Initialize A2A (Agent-to-Agent) protocol service
	a2aService := application.NewA2AService(
		repos.A2AAgentCard,
		repos.A2ASkill,
		repos.A2ATask,
		repos.A2APeerTrust,
		repos.A2AConsent,
		repos.A2ATrustScore,
		repos.A2ARequestNonce,
		repos.A2APolicy,
		repos.A2AAttestation,
		repos.A2ARevokedAgent,
		repos.A2ASecuritySettings,
		repos.A2ASecurityViolation,
		repos.Agent,
		keyVault,
	)

	// Device Authorization Grant (RFC 8628) for CLI login
	aimBaseURL := os.Getenv("AIM_BASE_URL")
	if aimBaseURL == "" {
		aimBaseURL = "http://localhost:8080"
	}
	deviceAuthService := application.NewDeviceAuthService(
		repos.DeviceCode,
		jwtService,
		authService,
		aimBaseURL,
	)

	// Registry Bridge: aggregate attestation data and push to OpenA2A Registry
	registryURL := os.Getenv("REGISTRY_BRIDGE_URL")
	if registryURL == "" {
		registryURL = "https://api.oa2a.org"
	}
	var registryBridgeService *application.RegistryBridgeService
	if os.Getenv("REGISTRY_BRIDGE_ENABLED") == "true" {
		registryBridgeService = application.NewRegistryBridgeService(
			repos.Organization,
			repos.Agent,
			repos.MCPAttestation,
			repos.MCPServer,
			repos.AuditLog,
			registryURL,
		)
	}

	// Community Intelligence: opt-in anonymized trust factor telemetry
	ciService := application.NewCommunityIntelligenceService(
		repos.Organization,
		repos.Agent,
		repos.TrustScore,
		repos.CommunityIntelligence,
		registryURL,
	)

	// Secrets management: ATC verifier (real + JWT shim composite) + backends + service
	jwtShimVerifier := infraatc.NewJWTShimVerifier(keyVault.GetServerSigningKey(), "aim-server")

	// Real ATC verifier with trusted issuers and CRL client
	issuerURI := getEnvOrDefault("ATC_ISSUER_URI", "https://aim.opena2a.org")
	crlEndpoint := getEnvOrDefault("ATC_CRL_ENDPOINT", "https://registry.opena2a.org/api/v1/crl/latest")
	crlClient, err := infraatc.NewCachedCRLClient(crlEndpoint)
	if err != nil {
		log.Fatalf("invalid CRL endpoint: %v", err)
	}
	issuerPubKey, ok := keyVault.GetServerSigningKey().Public().(ed25519.PublicKey)
	if !ok || len(issuerPubKey) != ed25519.PublicKeySize {
		log.Fatalf("server signing key is not a valid Ed25519 public key")
	}
	// Defensive copy: isolate from any future mutation of the source key.
	issuerPubKeyCopy := make([]byte, ed25519.PublicKeySize)
	copy(issuerPubKeyCopy, issuerPubKey)
	trustedIssuers := []atcdomain.TrustedIssuer{
		{
			URI:       issuerURI,
			PublicKey: issuerPubKeyCopy,
		},
	}
	realATCVerifier := infraatc.NewRealATCVerifier(trustedIssuers, crlClient)

	// Composite verifier: tries real ATC first, falls back to JWT shim
	atcVerifier := infraatc.NewCompositeVerifier(realATCVerifier, jwtShimVerifier)
	aimNativeBackend := infrasecrets.NewAIMNativeBackend(repos.SecretCredential)
	secretsBackends := map[domainsecrets.BackendType]domainsecrets.SecretsBackend{
		domainsecrets.BackendTypeAIMNative: aimNativeBackend,
	}

	// Register external secrets backends from environment config.
	// HashiCorp Vault: enabled when VAULT_ADDR is set.
	if vaultAddr := os.Getenv("VAULT_ADDR"); vaultAddr != "" {
		vaultCfg := &infrasecrets.VaultConfig{
			Address:   vaultAddr,
			MountPath: getEnvOrDefault("VAULT_MOUNT_PATH", "secret"),
			AuthPath:  getEnvOrDefault("VAULT_AUTH_PATH", "jwt"),
			Role:      os.Getenv("VAULT_ROLE"),
			Token:     os.Getenv("VAULT_TOKEN"),
			Namespace: os.Getenv("VAULT_NAMESPACE"),
		}
		vaultBackend, err := infrasecrets.NewHashiCorpVaultBackend(vaultCfg)
		if err != nil {
			log.Printf("WARNING: HashiCorp Vault backend configured but failed to initialize: %v", err)
		} else {
			secretsBackends[domainsecrets.BackendTypeVault] = vaultBackend
			log.Printf("HashiCorp Vault backend registered at %s", vaultAddr)
		}
	}

	// AWS Secrets Manager: enabled when AWS_SECRETS_REGION is set.
	if awsRegion := os.Getenv("AWS_SECRETS_REGION"); awsRegion != "" {
		awsCfg := &infrasecrets.AWSConfig{
			Region:   awsRegion,
			KMSKeyID: os.Getenv("AWS_SECRETS_KMS_KEY_ID"),
			Prefix:   getEnvOrDefault("AWS_SECRETS_PREFIX", "aim-secrets/"),
		}
		awsBackend, err := infrasecrets.NewAWSSecretsBackend(awsCfg)
		if err != nil {
			log.Printf("WARNING: AWS Secrets Manager backend configured but failed to initialize: %v", err)
		} else {
			secretsBackends[domainsecrets.BackendTypeAWSKMS] = awsBackend
			log.Printf("AWS Secrets Manager backend registered in %s", awsRegion)
		}
	}


	// Azure Key Vault: enabled when AZURE_KEYVAULT_URL is set.
	if azureVaultURL := os.Getenv("AZURE_KEYVAULT_URL"); azureVaultURL != "" {
		azureCfg := &infrasecrets.AzureKeyVaultConfig{
			VaultURL: azureVaultURL,
		}
		azureBackend, err := infrasecrets.NewAzureKeyVaultBackend(azureCfg)
		if err != nil {
			log.Printf("WARNING: Azure Key Vault backend configured but failed to initialize: %v", err)
		} else {
			secretsBackends[domainsecrets.BackendTypeAzureKV] = azureBackend
			log.Printf("Azure Key Vault backend registered at %s", azureVaultURL)
		}
	}

	// GCP Secret Manager: enabled when GCP_SECRETS_PROJECT is set.
	if gcpProject := os.Getenv("GCP_SECRETS_PROJECT"); gcpProject != "" {
		gcpCfg := &infrasecrets.GCPSecretConfig{
			ProjectID: gcpProject,
		}
		gcpBackend, err := infrasecrets.NewGCPSecretBackend(gcpCfg)
		if err != nil {
			log.Printf("WARNING: GCP Secret Manager backend configured but failed to initialize: %v", err)
		} else {
			secretsBackends[domainsecrets.BackendTypeGCPSM] = gcpBackend
			log.Printf("GCP Secret Manager backend registered for project %s", gcpProject)
		}
	}

	// 1Password: enabled when OP_CONNECT_HOST is set.
	if opHost := os.Getenv("OP_CONNECT_HOST"); opHost != "" {
		opCfg := &infrasecrets.OnePasswordConfig{
			ConnectHost:  opHost,
			ConnectToken: os.Getenv("OP_CONNECT_TOKEN"),
			VaultUUID:    os.Getenv("OP_VAULT_UUID"),
		}
		opBackend, err := infrasecrets.NewOnePasswordBackend(opCfg)
		if err != nil {
			log.Printf("WARNING: 1Password backend configured but failed to initialize: %v", err)
		} else {
			secretsBackends[domainsecrets.BackendType1Password] = opBackend
			log.Printf("1Password backend registered at %s", opHost)
		}
	}

	secretsService := application.NewSecretsService(
		repos.SecretNamespace,
		repos.SecretAudit,
		repos.Agent,
		repos.Capability,
		atcVerifier,
		secretsBackends,
	)

	// Fine-Grained Authorization engine. Wired with the OTel-bridged slog
	// logger so per-decision log lines auto-attach trace.id / span.id and
	// surface in Loki under {service_name="aim-backend"} alongside the
	// fga.authorize trace tree in Tempo.
	fgaEngine := application.NewFGAEngine(db, agentService, telemetry.Logger("aim-fga"))
	if v := os.Getenv("FGA_DAEMON_URL"); v != "" {
		fgaEngine.SetDaemonURL(v)
	}
	if v := os.Getenv("FGA_REGISTRY_ASC_URL"); v != "" {
		fgaEngine.SetRegistryASCURL(v)
	}

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
		MCPCapability:     mcpCapabilityService,     // ✅ For MCP server capability management
		MCPAttestation:    mcpAttestationService,    // ✅ For agent attestation of MCPs
		Security:          securityService,
		SecurityPolicy:    securityPolicyService,    // ✅ For policy-based enforcement
		BehaviorAnalysis:  behaviorAnalysisService,  // ✅ For intelligent behavioral anomaly detection
		Webhook:           webhookService,
		VerificationEvent: verificationEventService,
		Registration:      registrationService, // ✅ Email/password registration workflow (replaced OAuth)
		Tag:               tagService,
		SDKToken:          sdkTokenService,
		Capability:        capabilityService,
		CapabilityRequest: capabilityRequestService, // ✅ For capability expansion approval workflow
		Detection:         detectionService,         // ✅ For MCP auto-detection (SDK + Direct API)
		A2A:               a2aService,               // ✅ For A2A (Agent-to-Agent) protocol
		DeviceAuth:        deviceAuthService,        // For OAuth Device Authorization Grant (RFC 8628)
		RegistryBridge:          registryBridgeService,    // For OpenA2A Registry attestation contribution
		CommunityIntelligence:   ciService,                 // For community intelligence opt-in telemetry
		Remediation:             application.NewRemediationService(repos.Remediation), // For remediation tracking
		Secrets:                 secretsService,                                       // For identity-native secrets management
		FGA:                     fgaEngine,                                            // Fine-Grained Authorization engine
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
	MCPGraph           *handlers.MCPGraphHandler           // ✅ For MCP-Agent connection graph visualization
	MCPDiscovery       *handlers.MCPDiscoveryHandler       // ✅ For MCP discovery dashboard
	SupplyChain        *handlers.SupplyChainHandler        // ✅ For MCP supply chain analytics
	A2A                *handlers.A2AHandler                // For A2A (Agent-to-Agent) protocol
	OAuthToken         *handlers.OAuthTokenHandler         // For OAuth 2.0 token endpoint (RFC 6749)
	Lifecycle          *handlers.LifecycleHandler          // For agent lifecycle (heartbeat, revocations, bulk status)
	DeviceAuth         *handlers.DeviceAuthHandler         // For OAuth Device Authorization Grant (RFC 8628)
	RegistryBridge          *handlers.RegistryBridgeHandler          // For OpenA2A Registry attestation contribution
	CommunityIntelligence   *handlers.CommunityIntelligenceHandler   // For community intelligence opt-in telemetry
	AIP                     *handlers.AIPHandler                     // For AIP discovery and DID resolution
	Remediation             *handlers.RemediationHandler             // For remediation tracking (agentpwn + HMA)
	Secrets                 *handlers.SecretsHandler                 // For identity-native secrets management
	Authorize               *handlers.AuthorizeHandler               // POST /agents/:id/authorize -- FGA decision endpoint
}

func initHandlers(services *Services, repos *Repositories, jwtService *auth.JWTService, keyVault *crypto.KeyVault, cfg *config.Config, db *sql.DB) *Handlers {
	return &Handlers{
		Auth: handlers.NewAuthHandler(
			services.Auth,
			jwtService,
			repos.Organization,
			services.Audit, // ✅ For audit logging login/logout events
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
			services.Tag,              // ✅ For fetching agent tags in responses
			repos.Organization,        // ✅ For enforcement mode lookup in verify-capability
			services.MCPAttestation,   // ✅ For getting MCP connections via attestations
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
			services.Security,     // ✅ For security incidents tracking
			repos.User,            // A3d-v: ApproveUser / RejectUser handler-layer LoadOwned guard
		),
		Compliance: handlers.NewComplianceHandler(
			services.Compliance,
			services.Audit,
		),
		MCP: handlers.NewMCPHandler(
			services.MCP,
			services.MCPCapability,  // ✅ For capability endpoint
			services.Audit,
			repos.Agent,             // ✅ For agent relationships ("Talks To")
			repos.VerificationEvent, // ✅ For verification events endpoint
			services.Tag,            // ✅ For fetching MCP server tags in responses
			services.MCPAttestation, // ✅ For rich audit trail with attestations
		),
		MCPAttestation: handlers.NewMCPAttestationHandler(
			services.MCPAttestation,
			services.Audit,
			repos.Agent,
			repos.MCPServer,
		),
		Security: handlers.NewSecurityHandler(
			services.Security,
			services.Audit,
			services.Alert,
			services.Agent,
			services.Capability,
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
			services.Alert,
			services.Trust,
			services.VerificationEvent,
			repos.Organization,
			services.FGA,
		),
		VerificationEvent: handlers.NewVerificationEventHandler(
			services.VerificationEvent,
			repos.Agent,     // A3d-iv: agent-keyed verification-event reads verify agent.OrganizationID via LoadOwned
			repos.MCPServer, // A3d-iv: MCP-keyed verification-event reads verify mcpServer.OrganizationID via LoadOwned
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
			repos.Agent, // A3d-i: agent-scoped tag handlers verify agent.OrganizationID via LoadOwned
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
			services.CapabilityRequest,
			repos.Agent,
			repos.Organization,
		),
		Detection: handlers.NewDetectionHandler(
			services.Detection,
			services.Audit,
		),
		CapabilityRequest: handlers.NewCapabilityRequestHandlers(
			services.CapabilityRequest,
			repos.Agent,
		),
		MCPGraph: handlers.NewMCPGraphHandler(
			services.MCP,
			repos.Agent,
			repos.MCPAttestation,
		),
		MCPDiscovery: handlers.NewMCPDiscoveryHandler(
			services.MCP,
			repos.Agent,
		),
		SupplyChain: handlers.NewSupplyChainHandler(
			repos.AgentMCPConnection,
			repos.Agent,
			repos.MCPServer,
			repos.MCPCapability,
		),
		A2A: handlers.NewA2AHandler(
			services.A2A,
			services.Agent,
			services.Audit,
		),
		OAuthToken: handlers.NewOAuthTokenHandler(jwtService, repos.Agent),
		Lifecycle: handlers.NewLifecycleHandler(
			services.Agent,
			repos.Agent,
		),
		DeviceAuth: handlers.NewDeviceAuthHandler(
			services.DeviceAuth,
		),
		RegistryBridge: handlers.NewRegistryBridgeHandler(
			services.RegistryBridge,
		),
		CommunityIntelligence: handlers.NewCommunityIntelligenceHandler(
			services.CommunityIntelligence,
		),
		AIP: handlers.NewAIPHandler(
			repos.Agent,
		),
		Remediation: handlers.NewRemediationHandler(
			services.Remediation,
		),
		Secrets: handlers.NewSecretsHandler(
			services.Secrets,
		),
		Authorize: handlers.NewAuthorizeHandler(services.FGA),
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
	// SDK Token Tracking Middleware - Detects SDK registrations via X-SDK-Token header and User-Agent
	sdkTokenTrackingMiddleware := middleware.NewSDKTokenTrackingMiddleware(sdkTokenRepo)
	v1.Use(sdkTokenTrackingMiddleware.Handler()) // Apply to all API routes

	// Revocation list (public, rate-limited, cacheable, no auth required)
	// Registered inside the v1 group to avoid Fiber v3 beta route shadowing
	// when the /api/v1 group is registered with middleware on the app.
	v1.Get("/revocations", middleware.RateLimitMiddleware(), h.Lifecycle.GetRevocationList)

	// Public routes (NO authentication required) - Self-registration API
	public := v1.Group("/public")
	public.Use(middleware.StrictRateLimitMiddleware())                                       // SECURITY: Strict rate limiting on public endpoints
	public.Use(middleware.OptionalAuthMiddleware(jwtService))                               // Try to extract user from JWT if present
	public.Post("/agents/register", h.PublicAgent.Register)                                 // 🚀 ONE-LINE agent registration
	public.Post("/register", h.PublicRegistration.RegisterUser)                             // 🚀 User registration
	public.Get("/register/:requestId/status", h.PublicRegistration.CheckRegistrationStatus) // Check registration status
	public.Post("/login", h.PublicRegistration.Login)                                       // 🚀 Public login
	public.Post("/change-password", h.PublicRegistration.ChangePassword)                    // 🚀 Forced password change (enterprise security)
	public.Post("/forgot-password", h.PublicRegistration.ForgotPassword)                    // 🚀 Password reset request
	public.Post("/reset-password", h.PublicRegistration.ResetPassword)                      // 🚀 Password reset with token
	public.Post("/request-access", h.PublicRegistration.RequestAccess)                      // 🚀 Request platform access (no password required)

	// OAuth 2.0 token endpoint (RFC 6749 / RFC 7523 jwt-bearer grant)
	oauth := v1.Group("/oauth")
	oauth.Use(middleware.StrictRateLimitMiddleware())
	oauth.Post("/token", h.OAuthToken.HandleTokenRequest)

	// Device Authorization Grant (RFC 8628) for CLI login
	deviceAuth := v1.Group("/oauth/device")
	deviceAuth.Use(middleware.StrictRateLimitMiddleware())
	deviceAuth.Post("/code", h.DeviceAuth.RequestDeviceCode)
	deviceAuth.Post("/token", h.DeviceAuth.PollDeviceToken)
	deviceAuth.Get("/verify", h.DeviceAuth.GetDeviceVerification)

	// Device approval requires authentication (user must be logged in)
	deviceAuthProtected := v1.Group("/oauth/device")
	deviceAuthProtected.Use(middleware.AuthMiddleware(jwtService))
	deviceAuthProtected.Post("/approve", h.DeviceAuth.ApproveDevice)

	// Auth routes (no authentication required, strict rate limiting)
	auth := v1.Group("/auth")
	auth.Use(middleware.StrictRateLimitMiddleware()) // SECURITY: Strict rate limiting on auth endpoints
	auth.Post("/login/local", h.Auth.LocalLogin) // Local email/password login
	auth.Post("/logout", h.Auth.Logout)
	auth.Post("/refresh", h.AuthRefresh.RefreshToken)                 // Refresh access token (with token rotation)

	// Authenticated auth routes (authentication required)
	authProtected := v1.Group("/auth")
	authProtected.Use(middleware.AuthMiddleware(jwtService)) // Apply middleware using Use() instead of inline
	authProtected.Get("/me", h.Auth.Me)
	authProtected.Post("/change-password", h.Auth.ChangePassword)
	authProtected.Post("/sdk/recover", h.SDKTokenRecovery.RecoverRevokedToken) // SECURITY: Requires auth to prevent revocation bypass

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
	detection.Use(middleware.PQCAgentMiddleware(services.Agent)) // ✅ Try Ed25519/ML-DSA/hybrid first (for SDK agents)
	detection.Use(middleware.AuthMiddleware(jwtService))             // ✅ Fallback to JWT (for web UI)
	detection.Use(middleware.RateLimitMiddleware())
	detection.Post("/agents/:id/report", h.Detection.ReportDetection)
	detection.Get("/agents/:id/status", h.Detection.GetDetectionStatus) // ✅ Now accessible from web UI with JWT
	// ⭐ Agent Capability Detection endpoints - Report detected agent capabilities
	detection.Post("/agents/:id/capabilities/report", h.Detection.ReportCapabilities)
	detection.Get("/agents/:id/capabilities/latest", h.Detection.GetLatestCapabilityReport) // ✅ Fetch latest capability report

	// Agents routes - All other agent endpoints with quad authentication (ATC, API key, Ed25519, or JWT)
	agents := v1.Group("/agents")
	agents.Use(middleware.ATCAuthMiddleware(services.Secrets.ATCVerifier())) // ✅ ATC first (for DECIMA agents, Phase 7)
	agents.Use(middleware.OptionalAPIKeyMiddleware(db))            // ✅ Try API key (for dashboard-generated keys)
	agents.Use(middleware.PQCAgentMiddleware(services.Agent)) // ✅ Then try Ed25519/ML-DSA/hybrid (for SDK agents)
	agents.Use(middleware.AuthMiddleware(jwtService))             // ✅ Fallback to JWT (for web UI)
	agents.Use(middleware.RateLimitMiddleware())
	agents.Get("/", h.Agent.ListAgents)
	agents.Post("/bulk-status", h.Lifecycle.BulkStatus) // Bulk agent status lookup
	agents.Post("/", middleware.MemberMiddleware(), h.Agent.CreateAgent)
	agents.Get("/:id", h.Agent.GetAgent)
	agents.Put("/:id", middleware.MemberMiddleware(), h.Agent.UpdateAgent)
	agents.Delete("/:id", middleware.ManagerMiddleware(), h.Agent.DeleteAgent)
	agents.Post("/:id/verify", middleware.ManagerMiddleware(), h.Agent.VerifyAgent)
	// Agent lifecycle management endpoints
	agents.Post("/:id/suspend", middleware.ManagerMiddleware(), h.Agent.SuspendAgent)
	agents.Post("/:id/reactivate", middleware.ManagerMiddleware(), h.Agent.ReactivateAgent)
	agents.Post("/:id/revoke", middleware.ManagerMiddleware(), h.Agent.RevokeAgent)
	agents.Post("/:id/rotate-credentials", middleware.MemberMiddleware(), h.Agent.RotateCredentials)
	agents.Put("/:id/keys", middleware.MemberMiddleware(), h.Agent.UpdateAgentKeys) // SDK key registration
	// Runtime verification endpoints - CORE functionality
	agents.Post("/:id/verify-capability", h.Agent.VerifyCapability)
	agents.Post("/:id/log-capability/:audit_id", h.Agent.LogCapabilityResult)
	// Fine-Grained Authorization (5-step decision flow with OTel span tree)
	agents.Post("/:id/authorize", h.Authorize.Authorize)
	// SDK download endpoint - Download Python/Node.js/Go SDK with embedded credentials
	agents.Get("/:id/sdk", h.Agent.DownloadSDK)
	// Credentials endpoint - Get raw Ed25519 public/private keys for manual integration
	agents.Get("/:id/credentials", h.Agent.GetCredentials)
	// MCP Server relationship management - "talks_to" endpoints
	agents.Get("/:id/mcp-servers", h.Agent.GetAgentMCPServers)                                                  // ✅ Dashboard: Get MCP servers from talks_to field
	agents.Put("/:id/mcp-servers", middleware.MemberMiddleware(), h.Agent.AddMCPServersToAgent)                // Add MCP servers (bulk)
	agents.Delete("/:id/mcp-servers/:mcp_id", middleware.MemberMiddleware(), h.Agent.RemoveMCPServerFromAgent) // Remove single MCP
	// Attestation revocation for supply chain security (when agent key is compromised)
	agents.Post("/:id/attestations/revoke-all", middleware.ManagerMiddleware(), h.MCPAttestation.RevokeAllAttestationsByAgent) // 🔴 Revoke ALL attestations by this agent
	agents.Post("/:id/mcp-servers/detect", middleware.MemberMiddleware(), h.Agent.DetectAndMapMCPServers)      // Auto-detect MCPs from config
	// Trust Score management - RESTful endpoints under /agents/:id/trust-score/*
	agents.Get("/:id/trust-score", h.Agent.GetAgentTrustScore)                                                      // Get current trust score
	agents.Get("/:id/trust-score/history", h.Agent.GetAgentTrustScoreHistory)                                       // Get trust score history
	agents.Put("/:id/trust-score", middleware.AdminMiddleware(), h.Agent.UpdateAgentTrustScore)                     // Manually update score (admin)
	agents.Post("/:id/trust-score/recalculate", middleware.ManagerMiddleware(), h.Agent.RecalculateAgentTrustScore) // Recalculate score
	// Agent-specific alerts endpoint
	agents.Get("/:id/alerts", h.Agent.GetAgentAlerts) // Get alerts for this agent (trust score drops, security events, etc.)
	// Agent security endpoints - Key vault and audit logs per agent
	agents.Get("/:id/key-vault", h.Agent.GetAgentKeyVault)   // Get agent's key vault info (public key, expiration, rotation status)
	agents.Get("/:id/audit-logs", h.Agent.GetAgentAuditLogs) // Get audit logs for specific agent (with pagination)
	agents.Get("/:id/activity", h.Agent.GetAgentActivity)    // Get actions PERFORMED BY the agent (attestations, verifications)
	// Post-Quantum Cryptography (PQC) endpoints
	agents.Get("/:id/pqc-key", h.Agent.GetPQCKeyVault)                                  // Get agent's PQC key vault info
	agents.Post("/:id/pqc-key", middleware.MemberMiddleware(), h.Agent.RegisterPQCKey)  // Register PQC public key
	agents.Put("/:id/pqc-key", middleware.MemberMiddleware(), h.Agent.RotatePQCKey)     // Rotate PQC key
	agents.Post("/:id/hybrid-mode", middleware.MemberMiddleware(), h.Agent.SetHybridMode) // Enable/disable hybrid mode

	// Cryptographic algorithms info (public)
	crypto := v1.Group("/crypto")
	crypto.Get("/algorithms", h.Agent.GetSupportedAlgorithms) // List supported algorithms (Ed25519, ML-DSA, hybrid)

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

	// Remediation tracking (public, rate-limited, no auth -- receives reports from agentpwn and HMA)
	remediation := v1.Group("/remediation")
	remediation.Use(middleware.RateLimitMiddleware())
	remediation.Post("/track", h.Remediation.RecordFinding)
	remediation.Post("/remediated", h.Remediation.RecordRemediation)
	remediation.Get("/stats", h.Remediation.GetStats)
	remediation.Get("/recent", h.Remediation.ListRecent)

	// Secrets management routes
	// POST /resolve uses ATCAuthMiddleware -> PQCAgentMiddleware (agent auth), all others use JWT
	secretsResolve := v1.Group("/secrets")
	secretsResolve.Use(middleware.ATCAuthMiddleware(services.Secrets.ATCVerifier()))
	secretsResolve.Use(middleware.PQCAgentMiddleware(services.Agent))
	secretsResolve.Use(middleware.RateLimitMiddleware())
	secretsResolve.Post("/resolve", h.Secrets.ResolveCredential)

	secretsMgmt := v1.Group("/secrets")
	secretsMgmt.Use(middleware.AuthMiddleware(jwtService))
	secretsMgmt.Use(middleware.RateLimitMiddleware())
	secretsMgmt.Post("/namespaces", middleware.MemberMiddleware(), h.Secrets.CreateNamespace)
	secretsMgmt.Get("/namespaces", h.Secrets.ListNamespaces)
	secretsMgmt.Get("/namespaces/:id", h.Secrets.GetNamespace)
	secretsMgmt.Delete("/namespaces/:id", middleware.MemberMiddleware(), h.Secrets.DeleteNamespace)
	secretsMgmt.Post("/namespaces/:id/credentials", middleware.MemberMiddleware(), h.Secrets.StoreCredential)
	secretsMgmt.Post("/namespaces/:id/rotate", middleware.MemberMiddleware(), h.Secrets.RotateCredential)
	secretsMgmt.Get("/audit", h.Secrets.GetAuditLog)

	// Community Intelligence opt-in telemetry (authentication required)
	ci := v1.Group("/community-intelligence")
	ci.Use(middleware.AuthMiddleware(jwtService))
	ci.Use(middleware.RateLimitMiddleware())
	ci.Post("/enable", h.CommunityIntelligence.EnableOptIn)
	ci.Post("/disable", h.CommunityIntelligence.DisableOptIn)
	ci.Get("/status", h.CommunityIntelligence.GetOptInStatus)
	ci.Get("/benchmarks", h.CommunityIntelligence.GetCommunityBenchmarks)

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

	// Enforcement settings (Security → Enforcement Mode)
	admin.Get("/enforcement-settings", h.Admin.GetEnforcementSettings)
	admin.Put("/enforcement-settings", h.Admin.UpdateEnforcementSettings)

	// Audit logs
	admin.Get("/audit-logs", h.Admin.GetAuditLogs)
	admin.Get("/audit-logs/export", h.Admin.ExportAuditLogs)
	admin.Get("/audit-logs/:id", h.Admin.GetAuditLogByID)

	// Alerts
	admin.Get("/alerts", h.Admin.GetAlerts)
	admin.Get("/alerts/unacknowledged/count", h.Admin.GetUnacknowledgedAlertCount)
	admin.Post("/alerts/bulk-acknowledge", h.Admin.BulkAcknowledgeAlerts)
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

	// Verification Approval Management routes (admin only - for require_approval decorator)
	admin.Get("/verifications/pending", h.Verification.ListPendingVerifications)
	admin.Post("/verifications/:id/approve", h.Verification.ApproveVerification)
	admin.Post("/verifications/:id/deny", h.Verification.DenyVerification)

	// Registry Bridge (admin only - manual trigger for OpenA2A Registry contribution)
	admin.Post("/registry-bridge/push", h.RegistryBridge.TriggerPush)

	// Community Intelligence (admin only - manual trigger for CI data push)
	admin.Post("/community-intelligence/push", h.CommunityIntelligence.TriggerPush)

	// Compliance routes (admin only)
	// SOC 2 and HIPAA compliance features
	compliance := v1.Group("/compliance")
	compliance.Use(middleware.AuthMiddleware(jwtService))
	compliance.Use(middleware.AdminMiddleware())
	compliance.Use(middleware.RateLimitMiddleware())
	compliance.Get("/status", h.Compliance.GetComplianceStatus)
	compliance.Get("/metrics", h.Compliance.GetComplianceMetrics)
	compliance.Get("/audit-log/access-review", h.Compliance.GetAccessReview)
	compliance.Get("/access-review", h.Compliance.GetAccessReview)
	compliance.Post("/check", h.Compliance.RunComplianceCheck)
	compliance.Get("/export", h.Compliance.ExportComplianceReport) // Export compliance report (CSV/JSON)
	// Compliance score trending - track scores over time
	compliance.Get("/trending", h.Compliance.GetComplianceTrending)       // Get compliance score history
	compliance.Post("/snapshot", h.Compliance.RecordComplianceSnapshot)   // Record point-in-time compliance score
	// Evidence collection for auditors
	compliance.Get("/evidence", h.Compliance.ListEvidence)                         // List collected evidence
	compliance.Post("/evidence/collect", h.Compliance.CollectEvidence)             // Collect evidence for a check
	compliance.Get("/evidence/check/:checkName", h.Compliance.GetEvidenceForCheck) // Get evidence for specific check

	// MCP Server routes (authentication required)
	// ✅ Agent Attestation endpoints - Revolutionary zero-effort MCP verification
	// CRITICAL: These MUST be registered BEFORE JWT-protected routes to avoid middleware conflicts
	// These endpoints use Ed25519 authentication (agent-to-backend) instead of JWT (user-to-backend)
	mcpServersAgentAuth := v1.Group("/mcp-servers")
	mcpServersAgentAuth.Use(middleware.PQCAgentMiddleware(services.Agent)) // PQC signature verification (Ed25519, ML-DSA, or hybrid)
	mcpServersAgentAuth.Use(middleware.RateLimitMiddleware())
	mcpServersAgentAuth.Get("/:id/challenge", h.MCPAttestation.GetAttestationChallenge)  // 🔐 Get challenge for proof of key possession (MUST be called before /attest)
	mcpServersAgentAuth.Post("/:id/attest", h.MCPAttestation.AttestMCP)                  // ✅ Submit agent attestation (Ed25519 signed, with challenge)
	mcpServersAgentAuth.Get("/:id/attestations", h.MCPAttestation.GetMCPAttestations)    // ✅ Get all attestations for this MCP
	// NOTE: /:id/agents moved to JWT-authenticated mcpServers group to fix route conflict
	// The Ed25519 middleware's c.Next() was forwarding JWT requests to wrong handler

	// Standard MCP Server management endpoints - Use JWT authentication (user-to-backend)
	mcpServers := v1.Group("/mcp-servers")
	mcpServers.Use(middleware.AuthMiddleware(jwtService))
	mcpServers.Use(middleware.RateLimitMiddleware())
	mcpServers.Get("/", h.MCP.ListMCPServers)
	mcpServers.Get("/graph", h.MCPGraph.GetConnectionGraph)         // ✅ MCP-Agent network graph (must be before /:id routes)
	mcpServers.Get("/discovered", h.MCPDiscovery.GetDiscoveredMCPs) // ✅ MCP Discovery dashboard (must be before /:id routes)
	mcpServers.Get("/by-name", h.MCP.GetMCPServerByName)            // ✅ Get MCP by name (for SDK capability caching)
	mcpServers.Post("/", middleware.MemberMiddleware(), h.MCP.CreateMCPServer)
	mcpServers.Get("/:id", h.MCP.GetMCPServer)
	mcpServers.Put("/:id", middleware.MemberMiddleware(), h.MCP.UpdateMCPServer)
	mcpServers.Delete("/:id", middleware.ManagerMiddleware(), h.MCP.DeleteMCPServer)
	mcpServers.Post("/:id/verify", middleware.ManagerMiddleware(), h.MCP.VerifyMCPServer)
	mcpServers.Post("/:id/keys", middleware.MemberMiddleware(), h.MCP.AddPublicKey)
	mcpServers.Get("/:id/verification-status", h.MCP.GetVerificationStatus)
	mcpServers.Get("/:id/capabilities", h.MCP.GetMCPServerCapabilities)                                    // ✅ Get detected capabilities
	mcpServers.Post("/:id/detect-capabilities", middleware.MemberMiddleware(), h.MCP.DetectCapabilities) // ✅ Manually trigger capability detection
	mcpServers.Get("/:id/verification-events", h.MCP.GetMCPVerificationEvents)                             // ✅ Get verification events for MCP server
	mcpServers.Get("/:id/consensus-status", h.MCPAttestation.GetConsensusStatus)                           // 🔐 Multi-agent consensus status for verification
	mcpServers.Post("/:id/manual-attest", middleware.MemberMiddleware(), h.MCPAttestation.ManualAttestMCP) // ✅ Manual attestation (non-SDK users)
	mcpServers.Get("/:id/agents", h.MCP.GetMCPServerAgents)  // ✅ Dashboard: Get agents with this MCP in talks_to field
	// Runtime verification endpoint - CORE functionality
	mcpServers.Post("/:id/verify-capability", h.MCP.VerifyMCPCapability)
	mcpServers.Get("/:id/connections", h.MCPGraph.GetMCPServerConnections) // ✅ Get connection graph for specific MCP server
	mcpServers.Get("/:id/audit-logs", h.MCP.GetMCPServerAuditLogs)         // ✅ Get audit logs for specific MCP server

	// Attestation management routes (authentication required)
	// 🔴 Supply chain security: Revoke attestations when agent keys are compromised
	attestations := v1.Group("/attestations")
	attestations.Use(middleware.AuthMiddleware(jwtService))
	attestations.Use(middleware.RateLimitMiddleware())
	attestations.Post("/:attestation_id/revoke", middleware.ManagerMiddleware(), h.MCPAttestation.RevokeAttestation) // 🔴 Revoke single attestation

	// ⭐ Supply Chain Analytics routes (authentication required)
	// Dashboard for MCP supply chain visibility: connections, attestations, trends
	supplyChain := v1.Group("/supply-chain")
	supplyChain.Use(middleware.AuthMiddleware(jwtService))
	supplyChain.Use(middleware.RateLimitMiddleware())
	supplyChain.Get("/analytics", h.SupplyChain.GetSupplyChainAnalytics)       // 📊 Full supply chain dashboard data
	supplyChain.Get("/drift-alerts", h.SupplyChain.GetCapabilityDriftAlerts)   // 🔔 Capability drift detection alerts

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
	security.Get("/violations", h.Security.GetViolations)

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

	// ============================================================================
	// A2A (Agent-to-Agent) Protocol Routes
	// Google A2A protocol implementation with AIM security enhancements
	// ============================================================================

	// A2A signature verification (no auth - validates incoming A2A requests)
	v1.Post("/a2a/verify", h.A2A.VerifyRequest)

	// A2A routes (multi-auth: API key, agent signature, or JWT)
	a2a := v1.Group("/a2a")
	a2a.Use(middleware.OptionalAPIKeyMiddleware(db))
	a2a.Use(middleware.PQCAgentMiddleware(services.Agent))
	a2a.Use(middleware.AuthMiddleware(jwtService))
	a2a.Use(middleware.RateLimitMiddleware())

	// Agent Card management
	a2a.Post("/agents/:id/card", middleware.MemberMiddleware(), h.A2A.RegisterAgentCard)
	a2a.Get("/agents/:id/card", h.A2A.GetAgentCard)
	a2a.Post("/agents/:id/card/refresh", middleware.MemberMiddleware(), h.A2A.RefreshCardAttestation)

	// Request signing (SDK/programmatic)
	a2a.Post("/agents/:id/sign", h.A2A.SignRequest)

	// A2A Trust Scores
	a2a.Get("/agents/:id/trust-score", h.A2A.GetA2ATrustScore)
	a2a.Post("/agents/:id/trust-score/compute", middleware.ManagerMiddleware(), h.A2A.ComputeA2ATrustScore)
	a2a.Get("/agents/:id/peers/:peer_id/trust", h.A2A.GetPeerTrustScore)

	// A2A Skills
	a2a.Get("/agents/:id/skills", h.A2A.GetAgentSkills)
	a2a.Get("/skills/search", h.A2A.SearchSkills)

	// A2A Intent-Based Discovery
	a2a.Get("/route", h.A2A.RouteByIntent)
	a2a.Get("/capable-of", h.A2A.CapableOf)

	// A2A Tasks (audit trail)
	a2a.Get("/tasks", h.A2A.ListTasks)
	a2a.Post("/tasks", h.A2A.LogTask)
	a2a.Put("/tasks/:id/state", h.A2A.UpdateTaskState)

	// A2A Consent Management (GDPR/PSD2 compliance)
	a2a.Get("/consents", h.A2A.ListAllConsents)
	a2a.Post("/consent", h.A2A.RecordConsent)
	a2a.Get("/consent/check", h.A2A.CheckConsent)
	a2a.Post("/consent/:id/revoke", h.A2A.RevokeConsent)
	a2a.Get("/consent/user/:userId", h.A2A.ListUserConsents)

	// A2A Trust Scores (admin)
	a2a.Get("/trust-scores", h.A2A.ListAllTrustScores)

	// A2A Policy Evaluation
	a2a.Post("/policies/evaluate", h.A2A.EvaluatePolicy)

	// A2A Skill Registration (SDK compatibility)
	a2a.Post("/skills", h.A2A.RegisterSkill)

	// A2A Attestations
	a2a.Post("/attestations", h.A2A.AttestSkill)
	a2a.Get("/attestations/:agentId/:skillId", h.A2A.GetSkillAttestations)
	a2a.Get("/consensus/:agentId/:skillId", h.A2A.GetConsensusStatus)

	// A2A Security Policy Enforcement
	a2a.Post("/security/check", h.A2A.CheckSecurity)
	a2a.Get("/security/settings", h.A2A.GetSecuritySettings)
	a2a.Put("/security/settings", middleware.ManagerMiddleware(), h.A2A.UpdateSecuritySettings)

	// SDK Compatibility Routes (alternative paths that SDKs expect)
	a2a.Post("/sign", h.A2A.SignRequestAlt)
	a2a.Post("/verify", h.A2A.VerifyRequest)                                   // SDK verify request endpoint
	a2a.Get("/trust/:id", h.A2A.GetTrustScoreAlt)
	a2a.Put("/trust/:id", h.A2A.UpdateTrustScore)                              // SDK update trust score
	a2a.Post("/trust/:id/interaction", h.A2A.RecordInteraction)                // SDK record interaction
	a2a.Post("/discovery/route", h.A2A.RouteByIntentPost)
	a2a.Post("/discovery/capable", h.A2A.CapableOfPost)                        // Java SDK expects POST
	a2a.Get("/cards", h.A2A.ListAgentCards)                                    // SDK list agent cards
	a2a.Post("/cards", h.A2A.RegisterAgentCardAlt)                             // Java SDK expects POST /cards
	a2a.Get("/cards/:id", h.A2A.GetAgentCard)                                  // SDK calls /cards/:id instead of /agents/:id/card
	a2a.Put("/cards/:id", h.A2A.UpdateAgentCard)                               // SDK update agent card
	a2a.Post("/cards/:id/attestation", h.A2A.RefreshCardAttestation)           // Java SDK expects /cards/:id/attestation
	a2a.Get("/peers", h.A2A.ListPeerTrusts)                                    // SDK list peer trusts
	a2a.Delete("/skills/:id", h.A2A.DeleteSkill)                               // SDK delete skill
	a2a.Get("/security/violations", h.A2A.GetSecurityViolations)               // SDK get security violations
	a2a.Get("/agents/:id/attestations", h.A2A.GetAgentAttestations)            // SDK path: /agents/:id/attestations
	a2a.Get("/agents/:id/skills/:skillId/consensus", h.A2A.GetConsensusStatus) // SDK path variation

	// A2A Maintenance (admin only)
	a2aAdmin := v1.Group("/a2a/maintenance")
	a2aAdmin.Use(middleware.AuthMiddleware(jwtService))
	a2aAdmin.Use(middleware.AdminMiddleware())
	a2aAdmin.Post("/cleanup-nonces", h.A2A.CleanupExpiredNonces)
	a2aAdmin.Post("/refresh-cards", h.A2A.RefreshExpiredCards)
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
		content, err := os.ReadFile(filepath.Join("migrations", file))
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
	entries, err := os.ReadDir("migrations")
	if err != nil {
		// If migrations directory doesn't exist, return empty list (not an error)
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only include .up.sql files for forward migrations
		if strings.HasSuffix(entry.Name(), ".up.sql") ||
			(strings.HasSuffix(entry.Name(), ".sql") && !strings.Contains(entry.Name(), ".down.sql")) {
			migrations = append(migrations, entry.Name())
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

// startExpirationCleanupJob starts a background goroutine that periodically
// marks expired verification events as expired. This implements the
// "Time-Limited" JIT access feature documented in README.md.
//
// Expiration rules:
// - Pending verifications: expire after 1 hour (if not approved/denied)
// - Approved verifications: expire after 24 hours (access window closes)
//
// Returns a channel that should be closed to stop the cleanup job.
func startExpirationCleanupJob(db *sql.DB) chan struct{} {
	stopChan := make(chan struct{})
	ticker := time.NewTicker(5 * time.Minute) // Run every 5 minutes

	go func() {
		log.Println("🕐 Starting JIT access expiration cleanup job (runs every 5 minutes)")

		for {
			select {
			case <-ticker.C:
				if err := expireOldVerifications(db); err != nil {
					log.Printf("⚠️  Expiration cleanup error: %v", err)
				}
			case <-stopChan:
				ticker.Stop()
				log.Println("🛑 Stopping JIT access expiration cleanup job")
				return
			}
		}
	}()

	return stopChan
}

// expireOldVerifications marks old pending and approved verifications as expired
func expireOldVerifications(db *sql.DB) error {
	now := time.Now()

	// SQL query to update expired verifications
	// - Pending verifications older than 1 hour
	// - Approved verifications older than 24 hours
	query := `
		UPDATE verification_events
		SET
			result = 'expired',
			reason = CASE
				WHEN result = 'pending' THEN 'Pending verification expired after 1 hour (automatic)'
				WHEN result = 'verified' THEN 'Approved access expired after 24 hours (automatic)'
				ELSE 'Access expired automatically'
			END,
			metadata = COALESCE(metadata, '{}'::jsonb) || jsonb_build_object(
				'expiredAt', $1::text,
				'originalResult', result,
				'expirationReason', 'automatic_background_cleanup'
			),
			updated_at = NOW()
		WHERE
			result NOT IN ('expired', 'denied', 'failed')
			AND (
				(result = 'pending' AND created_at < $2)
				OR
				(result = 'verified' AND created_at < $3)
			)
		RETURNING id
	`

	pendingExpiry := now.Add(-1 * time.Hour)   // Pending expires after 1 hour
	approvedExpiry := now.Add(-24 * time.Hour) // Approved expires after 24 hours

	rows, err := db.Query(query, now.Format(time.RFC3339), pendingExpiry, approvedExpiry)
	if err != nil {
		return fmt.Errorf("expiration query failed: %w", err)
	}
	defer rows.Close()

	expiredCount := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		expiredCount++
	}

	if expiredCount > 0 {
		log.Printf("🕐 Expired %d verification(s) (pending >1h or approved >24h)", expiredCount)
	}

	return nil
}

// startRegistryBridgeJob starts a background goroutine that periodically
// aggregates anonymized attestation data and pushes it to the OpenA2A Registry.
// The bridge runs every 6 hours and is opt-in via REGISTRY_BRIDGE_ENABLED=true.
// Returns a channel that should be closed to stop the bridge job.
func startRegistryBridgeJob(bridgeService *application.RegistryBridgeService) chan struct{} {
	stopChan := make(chan struct{})
	ticker := time.NewTicker(6 * time.Hour)

	go func() {
		log.Println("Registry bridge enabled (runs every 6 hours)")

		for {
			select {
			case <-ticker.C:
				if err := bridgeService.AggregateAndPush(context.Background()); err != nil {
					log.Printf("Registry bridge error: %v", err)
				}
			case <-stopChan:
				ticker.Stop()
				log.Println("Registry bridge stopped")
				return
			}
		}
	}()

	return stopChan
}

// startCommunityIntelligenceJob starts a background goroutine that periodically
// collects anonymized trust factor distributions from opted-in organizations
// and pushes them to the OpenA2A Registry. Runs every 6 hours.
// Returns a channel that should be closed to stop the job.
func startCommunityIntelligenceJob(ciService *application.CommunityIntelligenceService) chan struct{} {
	stopChan := make(chan struct{})
	ticker := time.NewTicker(6 * time.Hour)

	go func() {
		log.Println("Community intelligence job enabled (runs every 6 hours)")

		for {
			select {
			case <-ticker.C:
				if err := ciService.CollectAndPushTelemetry(context.Background()); err != nil {
					log.Printf("Community intelligence job error: %v", err)
				}
			case <-stopChan:
				ticker.Stop()
				log.Println("Community intelligence job stopped")
				return
			}
		}
	}()

	return stopChan
}

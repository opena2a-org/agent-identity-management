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

	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/config"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
	"github.com/opena2a/identity/backend/internal/infrastructure/cache"
	"github.com/opena2a/identity/backend/internal/infrastructure/oauth"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
	"github.com/opena2a/identity/backend/internal/interfaces/http/handlers"
	"github.com/opena2a/identity/backend/internal/interfaces/http/middleware"
	"github.com/jmoiron/sqlx"
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
	repos := initRepositories(db)
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
	services := initServices(repos, cacheService, oauthRepo, oauthProviders)

	// Initialize handlers
	h := initHandlers(services, jwtService, legacyOAuthService)

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

	// API v1 routes
	v1 := app.Group("/api/v1")
	setupRoutes(v1, h, jwtService)

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
	Security          *repository.SecurityRepository
	Webhook           *repository.WebhookRepository
	VerificationEvent *repository.VerificationEventRepositorySimple
}

func initRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		User:              repository.NewUserRepository(db),
		Organization:      repository.NewOrganizationRepository(db),
		Agent:             repository.NewAgentRepository(db),
		APIKey:            repository.NewAPIKeyRepository(db),
		TrustScore:        repository.NewTrustScoreRepository(db),
		AuditLog:          repository.NewAuditLogRepository(db),
		Alert:             repository.NewAlertRepository(db),
		MCPServer:         repository.NewMCPServerRepository(db),
		Security:          repository.NewSecurityRepository(db),
		Webhook:           repository.NewWebhookRepository(db),
		VerificationEvent: repository.NewVerificationEventRepository(db),
	}
}

type Services struct {
	Auth              *application.AuthService
	Agent             *application.AgentService
	APIKey            *application.APIKeyService
	Trust             *application.TrustCalculator
	Audit             *application.AuditService
	Alert             *application.AlertService
	Compliance        *application.ComplianceService
	MCP               *application.MCPService
	Security          *application.SecurityService
	Webhook           *application.WebhookService
	VerificationEvent *application.VerificationEventService
	OAuth             *application.OAuthService
}

func initServices(repos *Repositories, cacheService *cache.RedisCache, oauthRepo *repository.OAuthRepositoryPostgres, oauthProviders map[domain.OAuthProvider]application.OAuthProvider) *Services {
	// Create services
	authService := application.NewAuthService(
		repos.User,
		repos.Organization,
	)

	auditService := application.NewAuditService(repos.AuditLog)

	trustCalculator := application.NewTrustCalculator(
		repos.TrustScore,
		repos.APIKey,
		repos.AuditLog,
	)

	agentService := application.NewAgentService(
		repos.Agent,
		trustCalculator,
		repos.TrustScore,
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

	mcpService := application.NewMCPService(
		repos.MCPServer,
		repos.VerificationEvent,
		repos.User,
	)

	securityService := application.NewSecurityService(
		repos.Security,
		repos.Agent,
	)

	webhookService := application.NewWebhookService(
		repos.Webhook,
	)

	verificationEventService := application.NewVerificationEventService(
		repos.VerificationEvent,
		repos.Agent,
	)

	oauthService := application.NewOAuthService(
		oauthRepo,
		repos.User,
		authService,
		auditService,
		oauthProviders,
	)

	return &Services{
		Auth:              authService,
		Agent:             agentService,
		APIKey:            apiKeyService,
		Trust:             trustCalculator,
		Audit:             auditService,
		Alert:             alertService,
		Compliance:        complianceService,
		MCP:               mcpService,
		Security:          securityService,
		Webhook:           webhookService,
		VerificationEvent: verificationEventService,
		OAuth:             oauthService,
	}
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
	Analytics         *handlers.AnalyticsHandler
	Webhook           *handlers.WebhookHandler
	VerificationEvent *handlers.VerificationEventHandler
	OAuth             *handlers.OAuthHandler
}

func initHandlers(services *Services, jwtService *auth.JWTService, oauthService *auth.OAuthService) *Handlers {
	return &Handlers{
		Auth: handlers.NewAuthHandler(
			services.Auth,
			oauthService,
			jwtService,
		),
		Agent: handlers.NewAgentHandler(
			services.Agent,
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
			services.Agent,
			services.MCP,
			services.Audit,
			services.Alert,
		),
		Compliance: handlers.NewComplianceHandler(
			services.Compliance,
			services.Audit,
		),
		MCP: handlers.NewMCPHandler(
			services.MCP,
			services.Audit,
		),
		Security: handlers.NewSecurityHandler(
			services.Security,
			services.Audit,
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

func setupRoutes(v1 fiber.Router, h *Handlers, jwtService *auth.JWTService) {
	// Auth routes (no authentication required)
	auth := v1.Group("/auth")
	auth.Post("/login/local", h.Auth.LocalLogin)      // Local email/password login
	auth.Get("/login/:provider", h.Auth.Login)        // OAuth login
	auth.Get("/callback/:provider", h.Auth.Callback)  // OAuth callback
	auth.Post("/logout", h.Auth.Logout)
	auth.Post("/change-password", middleware.AuthMiddleware(jwtService), h.Auth.ChangePassword) // Change password
	auth.Get("/me", middleware.AuthMiddleware(jwtService), h.Auth.Me)

	// Agents routes (authentication required)
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

	// API keys routes (authentication required)
	apiKeys := v1.Group("/api-keys")
	apiKeys.Use(middleware.AuthMiddleware(jwtService))
	apiKeys.Use(middleware.RateLimitMiddleware())
	apiKeys.Get("/", h.APIKey.ListAPIKeys)
	apiKeys.Post("/", middleware.MemberMiddleware(), h.APIKey.CreateAPIKey)
	apiKeys.Delete("/:id", middleware.MemberMiddleware(), h.APIKey.RevokeAPIKey)

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
	admin.Put("/users/:id/role", h.Admin.UpdateUserRole)
	admin.Delete("/users/:id", h.Admin.DeactivateUser)

	// Audit logs
	admin.Get("/audit-logs", h.Admin.GetAuditLogs)

	// Alerts
	admin.Get("/alerts", h.Admin.GetAlerts)
	admin.Post("/alerts/:id/acknowledge", h.Admin.AcknowledgeAlert)
	admin.Post("/alerts/:id/resolve", h.Admin.ResolveAlert)

	// Dashboard stats
	admin.Get("/dashboard/stats", h.Admin.GetDashboardStats)

	// Compliance routes (admin only)
	compliance := v1.Group("/compliance")
	compliance.Use(middleware.AuthMiddleware(jwtService))
	compliance.Use(middleware.AdminMiddleware())
	compliance.Use(middleware.StrictRateLimitMiddleware())
	compliance.Post("/reports/generate", h.Compliance.GenerateComplianceReport)
	compliance.Get("/status", h.Compliance.GetComplianceStatus)
	compliance.Get("/metrics", h.Compliance.GetComplianceMetrics)
	compliance.Get("/audit-log/export", h.Compliance.ExportAuditLog)
	compliance.Get("/access-review", h.Compliance.GetAccessReview)
	compliance.Get("/data-retention", h.Compliance.GetDataRetention)
	compliance.Post("/check", h.Compliance.RunComplianceCheck)
	// NEW: Additional compliance endpoints
	compliance.Get("/frameworks", h.Compliance.GetComplianceFrameworks)
	compliance.Get("/reports/:framework", h.Compliance.GetComplianceReportByFramework)
	compliance.Post("/scan/:framework", h.Compliance.RunComplianceScanByFramework)
	compliance.Get("/violations", h.Compliance.GetComplianceViolations)
	compliance.Post("/remediate/:violation_id", h.Compliance.RemediateViolation)

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

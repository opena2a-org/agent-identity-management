package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// Service Interfaces for Handlers
// ===========================
// These interfaces define the contracts for services used by handlers.
// Using interfaces enables dependency injection and testability with mocks.

// AgentServicer defines the methods from AgentService that handlers use
type AgentServicer interface {
	CreateAgent(ctx context.Context, req *application.CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error)
	GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error)
	GetAgentByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error)
	ListAgents(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error)
	UpdateAgent(ctx context.Context, id uuid.UUID, req *application.CreateAgentRequest, requestedBy uuid.UUID) (*domain.Agent, error)
	DeleteAgent(ctx context.Context, id uuid.UUID) error
	VerifyAgent(ctx context.Context, id uuid.UUID) error
	SuspendAgent(ctx context.Context, id uuid.UUID) error
	ReactivateAgent(ctx context.Context, id uuid.UUID) error
	RevokeAgent(ctx context.Context, id uuid.UUID) error
	RecalculateTrustScore(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error)
	UpdateTrustScore(ctx context.Context, agentID uuid.UUID, newScore float64) error
	RotateCredentials(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error)
	GetAgentCredentials(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error)
	UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string, authMethod string) error
	AddMCPServers(ctx context.Context, agentID uuid.UUID, mcpServerIdentifiers []string) (*domain.Agent, []string, error)
	RemoveMCPServer(ctx context.Context, agentID uuid.UUID, mcpServerIdentifier string) (*domain.Agent, error)
	VerifyCapability(ctx context.Context, agentID uuid.UUID, capability string, resource string, metadata map[string]interface{}, sourceIP string) (allowed bool, reason string, auditID uuid.UUID, err error)
	LogCapabilityResult(ctx context.Context, agentID uuid.UUID, auditID uuid.UUID, success bool, errorMsg string, result map[string]interface{}) error
	HasCapability(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error)
	UpdateLastActive(ctx context.Context, agentID uuid.UUID) error
	DetectMCPServersFromConfig(ctx context.Context, agentID uuid.UUID, req *application.DetectMCPServersRequest, mcpService *application.MCPService, orgID, userID uuid.UUID) (*application.DetectMCPServersResult, error)
	// PQC (Post-Quantum Cryptography) methods
	UpdateAgentPQCKey(ctx context.Context, agentID uuid.UUID, pqcPublicKey string, algorithm string, hybridMode bool) error
	RotateAgentPQCKey(ctx context.Context, agentID uuid.UUID, newPQCPublicKey string, algorithm string) error
	SetAgentHybridMode(ctx context.Context, agentID uuid.UUID, enabled bool) error
}

// MCPServicer defines the methods from MCPService that handlers use
type MCPServicer interface {
	CreateMCPServer(ctx context.Context, req *application.CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error)
	GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error)
	GetMCPServerByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error)
	ListMCPServers(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error)
	UpdateMCPServer(ctx context.Context, id uuid.UUID, req *application.UpdateMCPServerRequest) (*domain.MCPServer, error)
	DeleteMCPServer(ctx context.Context, id uuid.UUID) error
	VerifyMCPServer(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error
	AddPublicKey(ctx context.Context, serverID uuid.UUID, req *application.AddPublicKeyRequest) error
	GetVerificationStatus(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error)
	VerifyMCPCapability(ctx context.Context, mcpID uuid.UUID, capability string, resource string, targetService string, metadata map[string]interface{}) (allowed bool, reason string, auditID uuid.UUID, err error)
	GetConnectedAgents(ctx context.Context, mcpServerID uuid.UUID) ([]application.ConnectedAgent, error)
	GetConnectedAgentsCount(ctx context.Context, mcpServerID uuid.UUID) (int, error)
}

// AuditServicer defines the methods from AuditService that handlers use
type AuditServicer interface {
	LogAction(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error
	GetAgentActivity(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error)
	GetAuditLogs(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)
}

// CapabilityServicer defines the methods from CapabilityService that handlers use
type CapabilityServicer interface {
	VerifyAction(ctx context.Context, agentID uuid.UUID, requestedCapability string, signature []byte, payload []byte, sourceIP *string, metadata map[string]interface{}) (*application.VerificationResult, error)
	GrantCapability(ctx context.Context, agentID uuid.UUID, capabilityType string, scope map[string]interface{}, grantedBy *uuid.UUID, executionMode string) (*domain.AgentCapability, error)
	RevokeCapability(ctx context.Context, capabilityID uuid.UUID, revokedBy *uuid.UUID) error
	GetAgentCapabilities(ctx context.Context, agentID uuid.UUID, activeOnly bool) ([]*domain.AgentCapability, error)
	ListCapabilities(ctx context.Context, orgID uuid.UUID) ([]application.CapabilityDefinition, error)
	ListCapabilitiesWithMetadata(ctx context.Context, orgID uuid.UUID) (*application.ListCapabilitiesResponse, error)
	ValidateAndRegisterCapability(ctx context.Context, capability string, orgID uuid.UUID) error
	GetViolationsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetViolationsByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error)
	GetRecentViolations(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error)
}

// TagServicer defines the methods from TagService that handlers use
type TagServicer interface {
	GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error)
	GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error)
}

// APIKeyServicer defines the methods from APIKeyService that handlers use
type APIKeyServicer interface {
	GenerateAPIKey(ctx context.Context, agentID uuid.UUID, orgID uuid.UUID, userID uuid.UUID, name string, expirationDays int) (fullKey string, record *domain.APIKey, err error)
}

// AlertServicer defines the methods from AlertService that handlers use
type AlertServicer interface {
	CreateAlert(ctx context.Context, alert *domain.Alert) error
	GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
	GetAlerts(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error)
	CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error)
	CountBySeverity(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error)
}

// VerificationEventServicer defines the methods from VerificationEventService that handlers use
type VerificationEventServicer interface {
	LogVerificationEvent(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, protocol domain.VerificationProtocol, verificationType domain.VerificationType, status domain.VerificationEventStatus, durationMs int, initiatorType domain.InitiatorType, initiatorID *uuid.UUID, metadata map[string]interface{}) (*domain.VerificationEvent, error)
}

// MCPAttestationServicer defines the methods from MCPAttestationService that handlers use
type MCPAttestationServicer interface {
	GetMCPServersForAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.MCPServer, error)
}

// AuthServicer defines the methods from AuthService that handlers use
type AuthServicer interface {
	GetUsersByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	CountActiveUsers(ctx context.Context, orgID uuid.UUID, withinMinutes int) (int, error)
	UpdateUserRole(ctx context.Context, userID, orgID uuid.UUID, role domain.UserRole, adminID uuid.UUID) (*domain.User, error)
	DeactivateUser(ctx context.Context, userID, orgID, adminID uuid.UUID) error
}

// AdminServicer defines the methods from AdminService that handlers use
type AdminServicer interface {
	GetPendingUsers(ctx context.Context, adminOrgID uuid.UUID) ([]*domain.User, error)
	ApproveUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error
	RejectUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID, reason string) error
	ActivateUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error
	PermanentlyDeleteUser(ctx context.Context, userID, adminID, adminOrgID uuid.UUID) error
	GetOrganizationSettings(ctx context.Context, orgID uuid.UUID) (*domain.Organization, error)
	GetEnforcementSettings(ctx context.Context, orgID uuid.UUID) (*application.EnforcementSettings, error)
	UpdateEnforcementMode(ctx context.Context, orgID uuid.UUID, mode domain.EnforcementMode) error
}

// AlertServicerExtended defines extended methods from AlertService that handlers use
type AlertServicerExtended interface {
	GetAlerts(ctx context.Context, orgID uuid.UUID, severity, status string, limit, offset int) ([]*domain.Alert, int, error)
	GetAlertsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.Alert, error)
	CreateAlert(ctx context.Context, alert *domain.Alert) error
	AcknowledgeAlert(ctx context.Context, alertID, orgID, userID uuid.UUID) error
	ResolveAlert(ctx context.Context, alertID, orgID, userID uuid.UUID, resolution string) error
	BulkAcknowledgeAlerts(ctx context.Context, orgID, userID uuid.UUID) (int, error)
	CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error)
	CountBySeverity(ctx context.Context, orgID uuid.UUID, status string) (critical, high, warning, info int, err error)
}

// RegistrationServicer defines the methods from RegistrationService that handlers use
type RegistrationServicer interface {
	ListPendingRegistrationRequests(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.UserRegistrationRequest, int, error)
	ApproveRegistrationRequest(ctx context.Context, requestID, adminID, orgID uuid.UUID) (*domain.User, error)
	RejectRegistrationRequest(ctx context.Context, requestID, adminID uuid.UUID, reason string) error
}

// SecurityServicer defines the methods from SecurityService that handlers use
type SecurityServicer interface {
	CountOpenIncidents(ctx context.Context, orgID uuid.UUID) (int, error)
}

// SecurityServicerExtended extends SecurityServicer with additional methods for SecurityHandler
type SecurityServicerExtended interface {
	SecurityServicer
	GetThreats(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Threat, error)
	GetAnomalies(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.Anomaly, error)
	GetSecurityMetrics(ctx context.Context, orgID uuid.UUID) (*domain.SecurityMetrics, error)
}

// ComplianceServicer defines the methods from ComplianceService that handlers use
type ComplianceServicer interface {
	GetComplianceStatus(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	GetComplianceMetrics(ctx context.Context, orgID uuid.UUID, startDate, endDate time.Time, interval string) (interface{}, error)
	GetAccessReview(ctx context.Context, orgID uuid.UUID) (interface{}, error)
	ListEvidence(ctx context.Context, orgID uuid.UUID, framework string, limit, offset int) ([]*domain.ComplianceEvidence, error)
}

// ComplianceServicerExtended extends ComplianceServicer with additional methods for ComplianceHandler
type ComplianceServicerExtended interface {
	ComplianceServicer
	RunComplianceCheck(ctx context.Context, orgID uuid.UUID, checkType string) (interface{}, error)
	ExportComplianceReportFull(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time, userID uuid.UUID) (*domain.ComplianceExportReport, error)
	ExportToCSV(ctx context.Context, orgID uuid.UUID, framework string) (string, error)
	GetComplianceTrending(ctx context.Context, orgID uuid.UUID, framework string, startDate, endDate time.Time) (interface{}, error)
	RecordComplianceSnapshot(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework) (*domain.ComplianceSnapshot, error)
	CollectEvidence(ctx context.Context, orgID uuid.UUID, framework domain.ComplianceFramework, checkName string, userID uuid.UUID) (*domain.ComplianceEvidence, error)
	GetEvidenceForCheck(ctx context.Context, orgID uuid.UUID, checkName string) ([]*domain.ComplianceEvidence, error)
}

// ===========================
// MCPHandler Interfaces
// ===========================

// MCPCapabilityServicer defines the methods from MCPCapabilityService that handlers use
type MCPCapabilityServicer interface {
	GetCapabilities(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error)
	DetectCapabilities(ctx context.Context, mcpServerID uuid.UUID) error
}

// AgentRepositoryer defines the methods from AgentRepository that handlers use
type AgentRepositoryer interface {
	GetByID(id uuid.UUID) (*domain.Agent, error)
	GetByMCPServer(mcpServerID, orgID uuid.UUID) ([]*domain.Agent, error)
	GetByMCPServerName(mcpServerName string, orgID uuid.UUID) ([]*domain.Agent, error)
}

// VerificationEventRepositoryer defines the methods from VerificationEventRepository that handlers use
type VerificationEventRepositoryer interface {
	GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error)
}

// MCPServicerExtended extends MCPServicer with additional methods for MCPHandler
type MCPServicerExtended interface {
	MCPServicer
	GenerateVerificationChallenge(ctx context.Context, serverID uuid.UUID) (string, error)
}

// MCPAttestationServicerExtended extends MCPAttestationServicer for audit timeline
type MCPAttestationServicerExtended interface {
	MCPAttestationServicer
	GetMCPAttestations(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.AttestationWithAgentDetails, float64, time.Time, error)
}

// ===========================
// AnalyticsHandler Interfaces
// ===========================

// VerificationEventServicerExtended extends VerificationEventServicer with analytics methods
type VerificationEventServicerExtended interface {
	VerificationEventServicer
	GetLast24HoursStatistics(ctx context.Context, orgID uuid.UUID) (*domain.VerificationStatistics, error)
}

// SecurityServicerForAnalytics extends SecurityServicer with GetIncidents for AnalyticsHandler
type SecurityServicerForAnalytics interface {
	SecurityServicer
	GetIncidents(ctx context.Context, orgID uuid.UUID, status domain.IncidentStatus, limit, offset int) ([]*domain.SecurityIncident, error)
}

// ===========================
// VerificationHandler Interfaces
// ===========================

// AgentServicerForVerification extends AgentServicer with CreateSecurityAlert for VerificationHandler
type AgentServicerForVerification interface {
	AgentServicer
	CreateSecurityAlert(ctx context.Context, alert *domain.Alert) error
}

// AuditServicerForVerification extends AuditServicer with Log method for VerificationHandler
type AuditServicerForVerification interface {
	AuditServicer
	Log(ctx context.Context, entry *domain.AuditLog) error
}

// AlertServicerForVerification extends AlertServicer with DetectUnusualAccessPatterns for VerificationHandler
type AlertServicerForVerification interface {
	AlertServicer
	DetectUnusualAccessPatterns(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID) ([]*domain.Alert, error)
}

// VerificationEventServicerForVerification extends VerificationEventServicer with full verification methods
type VerificationEventServicerForVerification interface {
	VerificationEventServicer
	CreateVerificationEvent(ctx context.Context, req *application.CreateVerificationEventRequest) (*domain.VerificationEvent, error)
	GetVerificationEvent(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error)
	UpdateVerificationResult(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error
	SearchVerifications(ctx context.Context, orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error)
	UpdateExecutionStatus(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error
}

// OrganizationRepositoryer defines the methods from OrganizationRepository that handlers use
type OrganizationRepositoryer interface {
	GetByID(id uuid.UUID) (*domain.Organization, error)
}

// ===========================
// TrustScoreHandler Interfaces
// ===========================

// TrustCalculatorServicer defines the methods from TrustCalculator that handlers use
type TrustCalculatorServicer interface {
	CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error)
	GetLatestTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error)
	GetTrustScoreHistoryAuditTrail(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error)
}

// ===========================
// WebhookHandler Interfaces
// ===========================

// WebhookServicer defines the methods from WebhookService that handlers use
type WebhookServicer interface {
	CreateWebhook(ctx context.Context, req *application.CreateWebhookRequest, orgID, userID uuid.UUID) (*domain.Webhook, error)
	ListWebhooks(ctx context.Context, orgID uuid.UUID) ([]*domain.Webhook, error)
	GetWebhook(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	DeleteWebhook(ctx context.Context, id uuid.UUID) error
	UpdateWebhook(ctx context.Context, id uuid.UUID, req *application.CreateWebhookRequest) (*domain.Webhook, error)
	TestWebhook(ctx context.Context, id uuid.UUID) (*application.WebhookTestResult, error)
}

package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a/identity/backend/internal/domain"
)

// DataFlowService handles data flow visibility operations
type DataFlowService struct {
	repo          domain.DataFlowRepository
	policyCache   *PolicyCache
	classifyCache *ClassificationCache
}

// NewDataFlowService creates a new DataFlowService
func NewDataFlowService(repo domain.DataFlowRepository, alertService interface{}) *DataFlowService {
	svc := &DataFlowService{
		repo:          repo,
		policyCache:   NewPolicyCache(5 * time.Minute),
		classifyCache: NewClassificationCache(10 * time.Minute),
	}
	return svc
}

// ============================================================================
// POLICY CACHE - In-memory cache for fast policy evaluation
// ============================================================================

// PolicyCache caches policies per organization for fast evaluation
type PolicyCache struct {
	mu       sync.RWMutex
	policies map[uuid.UUID][]*domain.DataFlowPolicy // orgID -> policies
	expiry   map[uuid.UUID]time.Time
	ttl      time.Duration
}

// NewPolicyCache creates a new policy cache
func NewPolicyCache(ttl time.Duration) *PolicyCache {
	return &PolicyCache{
		policies: make(map[uuid.UUID][]*domain.DataFlowPolicy),
		expiry:   make(map[uuid.UUID]time.Time),
		ttl:      ttl,
	}
}

// Get retrieves cached policies for an organization
func (c *PolicyCache) Get(orgID uuid.UUID) ([]*domain.DataFlowPolicy, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if exp, ok := c.expiry[orgID]; ok && time.Now().Before(exp) {
		return c.policies[orgID], true
	}
	return nil, false
}

// Set caches policies for an organization
func (c *PolicyCache) Set(orgID uuid.UUID, policies []*domain.DataFlowPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.policies[orgID] = policies
	c.expiry[orgID] = time.Now().Add(c.ttl)
}

// Invalidate removes cached policies for an organization
func (c *PolicyCache) Invalidate(orgID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.policies, orgID)
	delete(c.expiry, orgID)
}

// ============================================================================
// CLASSIFICATION CACHE - Cache classification types for pattern matching
// ============================================================================

// ClassificationCache caches classification types for efficient lookup
type ClassificationCache struct {
	mu             sync.RWMutex
	types          []*domain.DataClassificationType
	compiledRegex  map[string]*regexp.Regexp // pattern -> compiled regex
	expiry         time.Time
	ttl            time.Duration
}

// NewClassificationCache creates a new classification cache
func NewClassificationCache(ttl time.Duration) *ClassificationCache {
	return &ClassificationCache{
		types:         nil,
		compiledRegex: make(map[string]*regexp.Regexp),
		ttl:           ttl,
	}
}

// GetTypes returns cached classification types
func (c *ClassificationCache) GetTypes() ([]*domain.DataClassificationType, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.types != nil && time.Now().Before(c.expiry) {
		return c.types, true
	}
	return nil, false
}

// SetTypes caches classification types
func (c *ClassificationCache) SetTypes(types []*domain.DataClassificationType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.types = types
	c.expiry = time.Now().Add(c.ttl)

	// Pre-compile all regex patterns for efficiency
	for _, t := range types {
		for _, pattern := range t.ColumnPatterns {
			if _, exists := c.compiledRegex[pattern]; !exists {
				if re, err := regexp.Compile("(?i)" + pattern); err == nil {
					c.compiledRegex[pattern] = re
				}
			}
		}
		for _, pattern := range t.ContentPatterns {
			if _, exists := c.compiledRegex[pattern]; !exists {
				if re, err := regexp.Compile(pattern); err == nil {
					c.compiledRegex[pattern] = re
				}
			}
		}
	}
}

// GetCompiledRegex returns a pre-compiled regex
func (c *ClassificationCache) GetCompiledRegex(pattern string) *regexp.Regexp {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compiledRegex[pattern]
}

// ============================================================================
// DATA FLOW OPERATIONS
// ============================================================================

// RecordDataFlowRequest is the request to record a data flow
type RecordDataFlowRequest struct {
	OrganizationID uuid.UUID
	AgentID        uuid.UUID
	AgentName      string

	// Source information
	SourceType       domain.DataSourceType
	SourceName       string
	SourceIdentifier string // For source lookup/creation
	SourceDetails    map[string]interface{}

	// Database source details (for classification)
	DatabaseType string
	SchemaName   string
	TableName    string
	ColumnName   string

	// Destination
	DestinationType     domain.DataFlowDestinationType
	DestinationName     string
	DestinationEndpoint string
	DestinationDetails  map[string]interface{}

	// Data metrics
	DataSizeBytes  int64
	RecordCount    int
	FieldCount     int
	FieldsDetected []string

	// Context
	RequestID           string
	SessionID           string
	UserID              string
	ClientIP            string
	VerificationEventID *uuid.UUID
	Capability          string

	// Optional: Explicit classification (Tier 1)
	ExplicitClassification *ExplicitClassification

	// For flow chaining
	FlowID       string
	ParentFlowID string
	HopNumber    int
}

// ExplicitClassification represents developer-provided classification (Tier 1)
type ExplicitClassification struct {
	Category   string  // PII, PHI, PCI
	Type       string  // ssn, credit_card, email
	Confidence float64 // Should be 1.0 for explicit
}

// RecordDataFlowResponse is the response from recording a data flow
type RecordDataFlowResponse struct {
	FlowID         uuid.UUID            `json:"flowId"`
	PolicyAction   domain.PolicyAction  `json:"policyAction"`
	PolicyReason   string               `json:"policyReason,omitempty"`
	Classification *ClassificationResult `json:"classification,omitempty"`
	Blocked        bool                 `json:"blocked"`
	RedactedFields []string             `json:"redactedFields,omitempty"`
}

// ClassificationResult contains the result of data classification
type ClassificationResult struct {
	Category   string                   `json:"category"`
	Type       string                   `json:"type"`
	Tier       domain.ClassificationTier `json:"tier"`
	Confidence float64                  `json:"confidence"`
}

// RecordDataFlow records a data flow event and evaluates policies
func (s *DataFlowService) RecordDataFlow(ctx context.Context, req *RecordDataFlowRequest) (*RecordDataFlowResponse, error) {
	// Generate flow ID if not provided
	flowID := req.FlowID
	if flowID == "" {
		flowID = uuid.New().String()
	}

	// 1. Classify the data (source-based classification)
	classification, sourceID, err := s.classifyData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// 2. Evaluate policies (cached for performance)
	policyResult, err := s.evaluatePolicy(ctx, req, classification)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	// 3. Create the data flow record
	now := time.Now()
	flow := &domain.DataFlow{
		ID:             uuid.New(),
		OrganizationID: req.OrganizationID,
		AgentID:        req.AgentID,
		AgentName:      req.AgentName,

		FlowID:       flowID,
		ParentFlowID: req.ParentFlowID,
		HopNumber:    req.HopNumber,

		SourceID:      sourceID,
		SourceType:    req.SourceType,
		SourceName:    req.SourceName,
		SourceDetails: req.SourceDetails,

		DestinationType:     req.DestinationType,
		DestinationName:     req.DestinationName,
		DestinationEndpoint: req.DestinationEndpoint,
		DestinationDetails:  req.DestinationDetails,

		DataSizeBytes:  req.DataSizeBytes,
		RecordCount:    req.RecordCount,
		FieldCount:     req.FieldCount,
		FieldsDetected: req.FieldsDetected,

		PolicyID:     policyResult.PolicyID,
		PolicyAction: policyResult.Action,
		PolicyReason: policyResult.Reason,

		RequestID:           req.RequestID,
		SessionID:           req.SessionID,
		UserID:              req.UserID,
		ClientIP:            req.ClientIP,
		VerificationEventID: req.VerificationEventID,
		Capability:          req.Capability,

		StartedAt: now,
		Status:    domain.DataFlowStatusInProgress,
		CreatedAt: now,
	}

	if classification != nil {
		flow.ClassificationCategory = classification.Category
		flow.ClassificationType = classification.Type
		flow.ClassificationTier = classification.Tier
		flow.ClassificationConfidence = classification.Confidence
	}

	// 4. Handle policy action
	response := &RecordDataFlowResponse{
		FlowID:         flow.ID,
		PolicyAction:   policyResult.Action,
		PolicyReason:   policyResult.Reason,
		Classification: classification,
		Blocked:        policyResult.Action == domain.PolicyActionBlock,
	}

	switch policyResult.Action {
	case domain.PolicyActionBlock:
		flow.Status = domain.DataFlowStatusBlocked
		flow.CompletedAt = &now
		// Create violation record
		if err := s.createViolation(ctx, flow, policyResult); err != nil {
			// Log but don't fail the request
			fmt.Printf("Failed to create violation: %v\n", err)
		}

	case domain.PolicyActionAlert:
		// Create alert but allow the flow
		if err := s.createFlowAlert(ctx, flow, policyResult); err != nil {
			fmt.Printf("Failed to create alert: %v\n", err)
		}

	case domain.PolicyActionRedact:
		// Mark fields for redaction (SDK handles actual redaction)
		response.RedactedFields = policyResult.RedactedFields
	}

	// 5. Save the flow record (async for performance)
	go func() {
		if err := s.repo.CreateFlow(flow); err != nil {
			fmt.Printf("Failed to save flow: %v\n", err)
		}
	}()

	return response, nil
}

// CompleteDataFlow marks a data flow as completed
func (s *DataFlowService) CompleteDataFlow(ctx context.Context, flowID uuid.UUID, durationMs int, errorMsg string) error {
	status := domain.DataFlowStatusCompleted
	if errorMsg != "" {
		status = domain.DataFlowStatusFailed
	}
	return s.repo.UpdateFlowStatus(flowID, status, time.Now(), durationMs)
}

// ============================================================================
// CLASSIFICATION ENGINE
// ============================================================================

// classifyData classifies data based on its source (the key innovation)
func (s *DataFlowService) classifyData(ctx context.Context, req *RecordDataFlowRequest) (*ClassificationResult, *uuid.UUID, error) {
	// Tier 1: Explicit classification from developer (highest confidence)
	if req.ExplicitClassification != nil {
		return &ClassificationResult{
			Category:   req.ExplicitClassification.Category,
			Type:       req.ExplicitClassification.Type,
			Tier:       domain.ClassificationTierExplicit,
			Confidence: 1.0,
		}, nil, nil
	}

	// Generate source identifier for lookup
	sourceIdentifier := s.generateSourceIdentifier(req)

	// Check if we already know this source
	existingSource, err := s.repo.GetSourceByIdentifier(req.OrganizationID, req.SourceType, sourceIdentifier)
	if err != nil {
		return nil, nil, err
	}

	if existingSource != nil && existingSource.ClassificationTypeID != nil {
		// Update last seen (async)
		go s.repo.UpdateSourceLastSeen(existingSource.ID)

		return &ClassificationResult{
			Category:   existingSource.ClassificationCategory,
			Type:       existingSource.ClassificationTypeName,
			Tier:       existingSource.ClassificationTier,
			Confidence: existingSource.ClassificationConfidence,
		}, &existingSource.ID, nil
	}

	// Tier 2: Schema inference (for database sources)
	if req.SourceType == domain.DataSourceTypeDatabase && req.ColumnName != "" {
		classification := s.classifyBySchema(ctx, req.TableName, req.ColumnName)
		if classification != nil {
			// Save the discovered source (async)
			sourceID := uuid.New()
			go s.saveDiscoveredSource(req, sourceIdentifier, classification, sourceID)
			return classification, &sourceID, nil
		}
	}

	// Tier 3: Heuristic inference (naming patterns)
	if req.SourceName != "" {
		classification := s.classifyByHeuristic(ctx, req.SourceName)
		if classification != nil {
			sourceID := uuid.New()
			go s.saveDiscoveredSource(req, sourceIdentifier, classification, sourceID)
			return classification, &sourceID, nil
		}
	}

	// Tier 4: Content analysis (fallback - use with caution)
	// Only run if we have field names to analyze
	if len(req.FieldsDetected) > 0 {
		classification := s.classifyByContent(ctx, req.FieldsDetected)
		if classification != nil {
			sourceID := uuid.New()
			go s.saveDiscoveredSource(req, sourceIdentifier, classification, sourceID)
			return classification, &sourceID, nil
		}
	}

	// No classification - data is considered public/unclassified
	return nil, nil, nil
}

// generateSourceIdentifier creates a unique identifier for a data source
func (s *DataFlowService) generateSourceIdentifier(req *RecordDataFlowRequest) string {
	var identifier string

	switch req.SourceType {
	case domain.DataSourceTypeDatabase:
		identifier = fmt.Sprintf("db:%s.%s.%s", req.SchemaName, req.TableName, req.ColumnName)
	case domain.DataSourceTypeAPI:
		identifier = fmt.Sprintf("api:%s", req.SourceName)
	default:
		identifier = fmt.Sprintf("%s:%s", req.SourceType, req.SourceName)
	}

	// Hash for consistent length and privacy
	hash := sha256.Sum256([]byte(identifier))
	return hex.EncodeToString(hash[:16]) // First 16 bytes
}

// classifyBySchema uses column/table names to infer classification (Tier 2)
func (s *DataFlowService) classifyBySchema(ctx context.Context, tableName, columnName string) *ClassificationResult {
	types, ok := s.classifyCache.GetTypes()
	if !ok {
		// Load and cache types
		var err error
		types, err = s.repo.GetClassificationTypes(nil)
		if err != nil {
			return nil
		}
		s.classifyCache.SetTypes(types)
	}

	columnLower := strings.ToLower(columnName)
	tableLower := strings.ToLower(tableName)

	for _, t := range types {
		// Check column patterns
		for _, pattern := range t.ColumnPatterns {
			re := s.classifyCache.GetCompiledRegex(pattern)
			if re != nil && re.MatchString(columnLower) {
				return &ClassificationResult{
					Category:   t.CategoryName,
					Type:       t.Name,
					Tier:       domain.ClassificationTierSchema,
					Confidence: 0.95, // High confidence for schema match
				}
			}
		}

		// Check table patterns
		for _, pattern := range t.TablePatterns {
			re := s.classifyCache.GetCompiledRegex(pattern)
			if re != nil && re.MatchString(tableLower) {
				return &ClassificationResult{
					Category:   t.CategoryName,
					Type:       t.Name,
					Tier:       domain.ClassificationTierSchema,
					Confidence: 0.85, // Slightly lower for table-level match
				}
			}
		}
	}

	return nil
}

// classifyByHeuristic uses naming patterns to infer classification (Tier 3)
func (s *DataFlowService) classifyByHeuristic(ctx context.Context, sourceName string) *ClassificationResult {
	nameLower := strings.ToLower(sourceName)

	// Quick heuristics for common patterns
	heuristics := []struct {
		patterns []string
		category string
		dataType string
	}{
		{[]string{"ssn", "social_security", "social-security"}, "PII", "ssn"},
		{[]string{"email", "e-mail", "e_mail"}, "PII", "email"},
		{[]string{"phone", "mobile", "telephone"}, "PII", "phone"},
		{[]string{"credit_card", "creditcard", "cc_number", "pan"}, "PCI", "credit_card"},
		{[]string{"cvv", "cvc", "security_code"}, "PCI", "cvv"},
		{[]string{"patient", "medical", "diagnosis", "prescription"}, "PHI", "medical_record_number"},
		{[]string{"insurance_id", "member_id", "policy_number"}, "PHI", "insurance_id"},
	}

	for _, h := range heuristics {
		for _, pattern := range h.patterns {
			if strings.Contains(nameLower, pattern) {
				return &ClassificationResult{
					Category:   h.category,
					Type:       h.dataType,
					Tier:       domain.ClassificationTierHeuristic,
					Confidence: 0.8,
				}
			}
		}
	}

	return nil
}

// classifyByContent analyzes field names for potential sensitive data (Tier 4)
func (s *DataFlowService) classifyByContent(ctx context.Context, fields []string) *ClassificationResult {
	types, ok := s.classifyCache.GetTypes()
	if !ok {
		types, _ = s.repo.GetClassificationTypes(nil)
		if types != nil {
			s.classifyCache.SetTypes(types)
		}
	}

	for _, field := range fields {
		fieldLower := strings.ToLower(field)
		for _, t := range types {
			for _, pattern := range t.ColumnPatterns {
				re := s.classifyCache.GetCompiledRegex(pattern)
				if re != nil && re.MatchString(fieldLower) {
					return &ClassificationResult{
						Category:   t.CategoryName,
						Type:       t.Name,
						Tier:       domain.ClassificationTierContent,
						Confidence: t.ContentPatternConfidence,
					}
				}
			}
		}
	}

	return nil
}

// saveDiscoveredSource saves a newly discovered data source
func (s *DataFlowService) saveDiscoveredSource(req *RecordDataFlowRequest, identifier string, classification *ClassificationResult, sourceID uuid.UUID) {
	// Get the classification type ID
	classType, err := s.repo.GetClassificationTypeByName(classification.Category, classification.Type)
	if err != nil || classType == nil {
		return
	}

	now := time.Now()
	source := &domain.DataSource{
		ID:               sourceID,
		OrganizationID:   req.OrganizationID,
		SourceType:       req.SourceType,
		SourceName:       req.SourceName,
		SourceIdentifier: identifier,
		DatabaseType:     req.DatabaseType,
		SchemaName:       req.SchemaName,
		TableName:        req.TableName,
		ColumnName:       req.ColumnName,

		ClassificationTier:       classification.Tier,
		ClassificationTypeID:     &classType.ID,
		ClassificationConfidence: classification.Confidence,

		IsActive:     true,
		DiscoveredAt: now,
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.repo.CreateSource(source)
}

// ============================================================================
// POLICY ENGINE
// ============================================================================

// PolicyResult contains the result of policy evaluation
type PolicyResult struct {
	PolicyID       *uuid.UUID
	PolicyName     string
	Action         domain.PolicyAction
	Reason         string
	AlertSeverity  string
	AlertMessage   string
	RedactedFields []string
}

// evaluatePolicy evaluates policies against a data flow (cached for performance)
func (s *DataFlowService) evaluatePolicy(ctx context.Context, req *RecordDataFlowRequest, classification *ClassificationResult) (*PolicyResult, error) {
	// Get cached policies or load from DB
	policies, ok := s.policyCache.Get(req.OrganizationID)
	if !ok {
		var err error
		policies, err = s.repo.GetEnabledPoliciesByPriority(req.OrganizationID)
		if err != nil {
			return nil, err
		}
		s.policyCache.Set(req.OrganizationID, policies)
	}

	// Evaluate policies in priority order (first match wins)
	for _, policy := range policies {
		if s.policyMatches(policy, req, classification) {
			return &PolicyResult{
				PolicyID:      &policy.ID,
				PolicyName:    policy.Name,
				Action:        policy.Action,
				Reason:        fmt.Sprintf("Matched policy: %s", policy.Name),
				AlertSeverity: policy.AlertSeverity,
				AlertMessage:  policy.AlertMessage,
			}, nil
		}
	}

	// No policy matched - default allow
	return &PolicyResult{
		Action: domain.PolicyActionAllow,
		Reason: "No matching policy",
	}, nil
}

// policyMatches checks if a policy matches the current data flow
func (s *DataFlowService) policyMatches(policy *domain.DataFlowPolicy, req *RecordDataFlowRequest, classification *ClassificationResult) bool {
	// Classification category match
	if len(policy.ClassificationCategories) > 0 {
		if classification == nil {
			return false
		}
		if !dataFlowContains(policy.ClassificationCategories, classification.Category) {
			return false
		}
	}

	// Classification type match
	if len(policy.ClassificationTypes) > 0 {
		if classification == nil {
			return false
		}
		if !dataFlowContains(policy.ClassificationTypes, classification.Type) {
			return false
		}
	}

	// Minimum classification tier
	if policy.MinClassificationTier != nil {
		if classification == nil {
			return false
		}
		if int(classification.Tier) < *policy.MinClassificationTier {
			return false
		}
	}

	// Source type match
	if len(policy.SourceTypes) > 0 {
		if !dataFlowContains(policy.SourceTypes, string(req.SourceType)) {
			return false
		}
	}

	// Destination type match
	if len(policy.DestinationTypes) > 0 {
		if !dataFlowContains(policy.DestinationTypes, string(req.DestinationType)) {
			return false
		}
	}

	// Blocked destinations
	if len(policy.BlockedDestinations) > 0 {
		for _, blocked := range policy.BlockedDestinations {
			if blocked == "*" || blocked == req.DestinationName || strings.HasPrefix(req.DestinationEndpoint, blocked) {
				return true // Match - this destination is blocked
			}
		}
	}

	// Allowed destinations (whitelist mode)
	if len(policy.AllowedDestinations) > 0 {
		allowed := false
		for _, dest := range policy.AllowedDestinations {
			if dest == req.DestinationName || strings.HasPrefix(req.DestinationEndpoint, dest) {
				allowed = true
				break
			}
		}
		if !allowed {
			return true // Match - destination not in whitelist
		}
	}

	// Agent filter
	if len(policy.AgentIDs) > 0 {
		found := false
		for _, id := range policy.AgentIDs {
			if id == req.AgentID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// createViolation creates a violation record when a policy blocks a flow
func (s *DataFlowService) createViolation(ctx context.Context, flow *domain.DataFlow, result *PolicyResult) error {
	violation := &domain.DataFlowViolation{
		ID:                     uuid.New(),
		OrganizationID:         flow.OrganizationID,
		DataFlowID:             flow.ID,
		PolicyID:               *result.PolicyID,
		AgentID:                flow.AgentID,
		ViolationType:          string(result.Action),
		Severity:               result.AlertSeverity,
		ClassificationCategory: flow.ClassificationCategory,
		ClassificationType:     flow.ClassificationType,
		SourceName:             flow.SourceName,
		DestinationName:        flow.DestinationName,
		DestinationEndpoint:    flow.DestinationEndpoint,
		ActionTaken:            string(result.Action),
		ActionDetails:          result.Reason,
		Status:                 domain.ViolationStatusOpen,
		CreatedAt:              time.Now(),
	}

	return s.repo.CreateViolation(violation)
}

// createFlowAlert creates an alert for policy violations that aren't blocked
func (s *DataFlowService) createFlowAlert(ctx context.Context, flow *domain.DataFlow, result *PolicyResult) error {
	// Alert integration handled through violation records
	// Future: Integrate with existing AlertService for real-time notifications
	return nil
}

// ============================================================================
// QUERY OPERATIONS
// ============================================================================

// GetDataFlows retrieves data flows with pagination
func (s *DataFlowService) GetDataFlows(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.DataFlow, int, error) {
	return s.repo.GetFlowsByOrganization(orgID, limit, offset)
}

// GetDataFlowsByAgent retrieves flows for a specific agent
func (s *DataFlowService) GetDataFlowsByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*domain.DataFlow, int, error) {
	return s.repo.GetFlowsByAgent(agentID, limit, offset)
}

// GetDataFlowsByClassification retrieves flows by classification
func (s *DataFlowService) GetDataFlowsByClassification(ctx context.Context, orgID uuid.UUID, category, dataType string, limit, offset int) ([]*domain.DataFlow, int, error) {
	return s.repo.GetFlowsByClassification(orgID, category, dataType, limit, offset)
}

// GetFlowChain retrieves all flows in a chain
func (s *DataFlowService) GetFlowChain(ctx context.Context, flowID string) ([]*domain.DataFlow, error) {
	return s.repo.GetFlowChain(flowID)
}

// GetRecentFlows retrieves recent flows
func (s *DataFlowService) GetRecentFlows(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.DataFlow, error) {
	return s.repo.GetRecentFlows(orgID, minutes)
}

// GetSummary returns a high-level data flow summary
func (s *DataFlowService) GetSummary(ctx context.Context, orgID uuid.UUID) (*domain.DataFlowSummary, error) {
	return s.repo.GetSummary(orgID)
}

// ============================================================================
// SOURCE OPERATIONS
// ============================================================================

// GetDataSources retrieves data sources for an organization
func (s *DataFlowService) GetDataSources(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.DataSource, int, error) {
	return s.repo.GetSourcesByOrganization(orgID, limit, offset)
}

// UpdateSourceClassification manually updates a source's classification (Tier 1)
func (s *DataFlowService) UpdateSourceClassification(ctx context.Context, sourceID uuid.UUID, typeID uuid.UUID, userID uuid.UUID) error {
	return s.repo.UpdateSourceClassification(sourceID, typeID, domain.ClassificationTierExplicit, 1.0)
}

// ============================================================================
// POLICY OPERATIONS
// ============================================================================

// GetPolicies retrieves all policies for an organization
func (s *DataFlowService) GetPolicies(ctx context.Context, orgID uuid.UUID) ([]*domain.DataFlowPolicy, error) {
	return s.repo.GetPoliciesByOrganization(orgID)
}

// CreatePolicy creates a new data flow policy
func (s *DataFlowService) CreatePolicy(ctx context.Context, policy *domain.DataFlowPolicy) error {
	policy.ID = uuid.New()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	if err := s.repo.CreatePolicy(policy); err != nil {
		return err
	}

	// Invalidate cache
	s.policyCache.Invalidate(policy.OrganizationID)
	return nil
}

// UpdatePolicy updates an existing policy
func (s *DataFlowService) UpdatePolicy(ctx context.Context, policy *domain.DataFlowPolicy) error {
	policy.UpdatedAt = time.Now()

	if err := s.repo.UpdatePolicy(policy); err != nil {
		return err
	}

	// Invalidate cache
	s.policyCache.Invalidate(policy.OrganizationID)
	return nil
}

// DeletePolicy deletes a policy
func (s *DataFlowService) DeletePolicy(ctx context.Context, policyID, orgID uuid.UUID) error {
	if err := s.repo.DeletePolicy(policyID); err != nil {
		return err
	}

	// Invalidate cache
	s.policyCache.Invalidate(orgID)
	return nil
}

// ============================================================================
// VIOLATION OPERATIONS
// ============================================================================

// GetViolations retrieves violations for an organization
func (s *DataFlowService) GetViolations(ctx context.Context, orgID uuid.UUID, status domain.ViolationStatus, limit, offset int) ([]*domain.DataFlowViolation, int, error) {
	return s.repo.GetViolationsByOrganization(orgID, status, limit, offset)
}

// ResolveViolation resolves a violation
func (s *DataFlowService) ResolveViolation(ctx context.Context, violationID uuid.UUID, status domain.ViolationStatus, resolvedBy uuid.UUID, notes string) error {
	return s.repo.UpdateViolationStatus(violationID, status, &resolvedBy, notes)
}

// ============================================================================
// CLASSIFICATION OPERATIONS
// ============================================================================

// GetClassificationCategories retrieves all classification categories
func (s *DataFlowService) GetClassificationCategories(ctx context.Context) ([]*domain.DataClassificationCategory, error) {
	return s.repo.GetClassificationCategories()
}

// GetClassificationTypes retrieves classification types
func (s *DataFlowService) GetClassificationTypes(ctx context.Context, categoryID *uuid.UUID) ([]*domain.DataClassificationType, error) {
	return s.repo.GetClassificationTypes(categoryID)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func dataFlowContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

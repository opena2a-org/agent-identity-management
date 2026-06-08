package domain

import (
	"fmt"
	"sort"
	"strings"
)

// PurposeVocabVersion is the version of the purpose vocabulary (categories and
// reserved taskScope namespaces) that validation is performed against. Every
// declared purpose records the vocab version it was issued under so a later
// vocabulary revision never silently re-interprets an old declaration.
const PurposeVocabVersion = "1"

// MaxPurposeStatementLength caps the free-text statement. The statement is
// attacker-controllable content judged only by a non-generative classifier; the
// cap denies prose-injection room and keeps the credential small.
const MaxPurposeStatementLength = 280

// Autonomy levels for a declared purpose.
const (
	AutonomySupervised  = "supervised"
	AutonomyHumanInLoop = "human_in_loop"
	AutonomyAutonomous  = "autonomous"
)

// Purpose breadth classes. Breadth is the expected diversity of capability
// namespaces a purpose touches; it is measured from capabilityJustification
// (see DeclaredBreadth), never claimed by a token name.
const (
	PurposeBreadthNarrow   = "narrow"
	PurposeBreadthModerate = "moderate"
	PurposeBreadthBroad    = "broad"
)

// Purpose sensitivity classes drive verifier-policy defaults and the minimum
// trust level a category is appropriate for.
const (
	PurposeSensitivityStandard  = "standard"
	PurposeSensitivityElevated  = "elevated"
	PurposeSensitivityRegulated = "regulated"
)

// PurposeCategoryDef describes a core purpose category: its breadth class and
// its sensitivity class. Mirrors the capability registry's CapabilityDefinition
// governance pattern.
type PurposeCategoryDef struct {
	Breadth     string
	Sensitivity string
}

// CorePurposeCategories is the closed core category vocabulary (vocab v1). There
// is deliberately no catch-all: an agent that spans many objectives declares
// agent-orchestration (broad) and pays the breadth cost. Custom categories are
// permitted under an org namespace (<org>.<name>) and always default to the
// broadest, most sensitive class until promoted, so custom is never a cheap way
// to claim a narrow profile.
var CorePurposeCategories = map[string]PurposeCategoryDef{
	"customer-support":     {PurposeBreadthNarrow, PurposeSensitivityStandard},
	"financial-operations": {PurposeBreadthModerate, PurposeSensitivityRegulated},
	"data-analysis":        {PurposeBreadthNarrow, PurposeSensitivityStandard},
	"data-engineering":     {PurposeBreadthModerate, PurposeSensitivityElevated},
	"software-development": {PurposeBreadthModerate, PurposeSensitivityStandard},
	"devops-automation":    {PurposeBreadthBroad, PurposeSensitivityElevated},
	"security-operations":  {PurposeBreadthBroad, PurposeSensitivityElevated},
	"content-generation":   {PurposeBreadthNarrow, PurposeSensitivityStandard},
	"research-assistant":   {PurposeBreadthModerate, PurposeSensitivityStandard},
	"sales-marketing":      {PurposeBreadthModerate, PurposeSensitivityElevated},
	"hr-people-ops":        {PurposeBreadthModerate, PurposeSensitivityRegulated},
	"legal-compliance":     {PurposeBreadthNarrow, PurposeSensitivityRegulated},
	"healthcare-clinical":  {PurposeBreadthModerate, PurposeSensitivityRegulated},
	"device-control":       {PurposeBreadthModerate, PurposeSensitivityElevated},
	"agent-orchestration":  {PurposeBreadthBroad, PurposeSensitivityElevated},
}

// ReservedTaskScopeNamespaces are the core-only taskScope namespaces (vocab v1).
// They are an intent axis, deliberately separate from capability namespaces
// (ReservedNamespaces, the access axis). A non-reserved namespace is a custom
// (org) taskScope.
var ReservedTaskScopeNamespaces = []string{
	"support", "billing", "accounting", "analytics", "dataops", "dev", "ci",
	"infra", "secops", "content", "research", "crm", "people", "legal",
	"clinical", "device", "orchestrate",
}

// dualUsePurposeCategories are legitimately dual-use (their behavior resembles an
// attacker's, or they have physical-world impact). They may not be declared with
// a narrow breadth and their capabilityJustification must enumerate every granted
// capability explicitly (no wildcard).
var dualUsePurposeCategories = map[string]bool{
	"security-operations": true,
	"device-control":      true,
}

// IsReservedTaskScopeNamespace reports whether a namespace is a reserved core
// taskScope namespace.
func IsReservedTaskScopeNamespace(namespace string) bool {
	for _, n := range ReservedTaskScopeNamespaces {
		if n == namespace {
			return true
		}
	}
	return false
}

// DeclaredPurpose is the publisher's structured declaration of what an agent is
// for. It mirrors the ATX declaredPurpose object (atx-spec core.md §1.5). It is
// an identity/attestation property and an offline detection signal — NEVER an
// authorization input. It is optional: a nil *DeclaredPurpose on an Agent means
// no purpose was declared, which degrades the detection signal but never fails
// verification or enforcement.
type DeclaredPurpose struct {
	// VocabVersion is the purpose vocabulary version this declaration targets.
	// Defaults to PurposeVocabVersion when empty.
	VocabVersion string `json:"vocabVersion,omitempty"`
	// Statement is human/audit-readable intent (<= MaxPurposeStatementLength).
	Statement string `json:"statement,omitempty"`
	// Category is a core category key (CorePurposeCategories) or an org-custom
	// "<org>.<name>" value.
	Category string `json:"category"`
	// TaskScopes are namespace:objective tokens at the objective level.
	TaskScopes []string `json:"taskScopes"`
	// CapabilityJustification maps each granted capability to the taskScope(s) it
	// serves. Keys must be a subset of the agent's granted Capabilities.
	CapabilityJustification map[string][]string `json:"capabilityJustification"`
	// Autonomy is an optional tolerance hint.
	Autonomy string `json:"autonomy,omitempty"`
	// DataScopes are the declared data domains the agent operates over.
	DataScopes []string `json:"dataScopes,omitempty"`
	// EgressScopes are the declared external destinations (hostnames/domains; no
	// paths, no secrets) the agent legitimately calls.
	EgressScopes []string `json:"egressScopes,omitempty"`
}

// PurposeError is a declared-purpose validation error.
type PurposeError struct {
	Field   string
	Message string
}

func (e *PurposeError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("declaredPurpose.%s: %s", e.Field, e.Message)
	}
	return "declaredPurpose: " + e.Message
}

// IsCustomPurposeCategory reports whether a category value is a well-formed
// org-custom category ("<org>.<name>", each segment lowercase alphanumeric with
// optional hyphens). Core categories are NOT custom.
func IsCustomPurposeCategory(category string) bool {
	if _, ok := CorePurposeCategories[category]; ok {
		return false
	}
	parts := strings.Split(category, ".")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if !isPurposeSegment(p) {
			return false
		}
	}
	return true
}

func isPurposeSegment(s string) bool {
	if len(s) == 0 || !isLowerLetter(rune(s[0])) {
		return false
	}
	for _, c := range s {
		if !isLowerAlphanumeric(c) && c != '-' {
			return false
		}
	}
	return true
}

// CategoryBreadthClass returns the breadth class for a category: the core class
// for a core category, or PurposeBreadthBroad for a custom category (custom
// defaults to the broadest class until CA promotion).
func CategoryBreadthClass(category string) string {
	if def, ok := CorePurposeCategories[category]; ok {
		return def.Breadth
	}
	return PurposeBreadthBroad
}

// maxNamespacesForBreadth returns the max distinct capability namespaces a
// breadth class tolerates before it is over-broad. -1 means unbounded.
func maxNamespacesForBreadth(breadth string) int {
	switch breadth {
	case PurposeBreadthNarrow:
		return 1
	case PurposeBreadthModerate:
		return 3
	default: // broad
		return -1
	}
}

// DeclaredBreadth is the measured breadth of a purpose: the count of distinct
// capability namespaces appearing across all capabilityJustification keys. This
// is the anti-gaming anchor (atx-spec §1.5.4) — breadth keys on the capability
// map, not on how taskScope tokens are named.
func (p *DeclaredPurpose) DeclaredBreadth() int {
	seen := map[string]struct{}{}
	for cap := range p.CapabilityJustification {
		ns, _, err := ParseCapability(cap)
		if err != nil {
			continue
		}
		seen[ns] = struct{}{}
	}
	return len(seen)
}

// Validate performs hard structural validation against the agent's granted
// capabilities. It rejects malformed declarations (bad grammar, oversized
// statement, justification of un-granted authority, malformed egress). It does
// NOT reject on coherence/breadth — those are issuance hygiene reported by
// CoherenceWarnings. grantedCapabilities is the agent's Capabilities slice.
func (p *DeclaredPurpose) Validate(grantedCapabilities []string) error {
	if p == nil {
		return nil
	}

	if p.VocabVersion != "" && p.VocabVersion != PurposeVocabVersion {
		return &PurposeError{Field: "vocabVersion", Message: fmt.Sprintf("unsupported vocab version %q (expected %q)", p.VocabVersion, PurposeVocabVersion)}
	}

	if len([]rune(p.Statement)) > MaxPurposeStatementLength {
		return &PurposeError{Field: "statement", Message: fmt.Sprintf("statement exceeds %d characters", MaxPurposeStatementLength)}
	}

	// category: core or well-formed custom.
	if p.Category == "" {
		return &PurposeError{Field: "category", Message: "category is required"}
	}
	if _, core := CorePurposeCategories[p.Category]; !core && !IsCustomPurposeCategory(p.Category) {
		return &PurposeError{Field: "category", Message: fmt.Sprintf("unknown category %q (not a core category and not a valid <org>.<name> custom category)", p.Category)}
	}

	// autonomy: optional enum.
	switch p.Autonomy {
	case "", AutonomySupervised, AutonomyHumanInLoop, AutonomyAutonomous:
	default:
		return &PurposeError{Field: "autonomy", Message: fmt.Sprintf("invalid autonomy %q", p.Autonomy)}
	}

	// taskScopes: required, valid grammar.
	if len(p.TaskScopes) == 0 {
		return &PurposeError{Field: "taskScopes", Message: "at least one taskScope is required"}
	}
	declared := map[string]struct{}{}
	for _, ts := range p.TaskScopes {
		if err := ValidateCapabilityFormat(ts); err != nil {
			return &PurposeError{Field: "taskScopes", Message: fmt.Sprintf("invalid taskScope %q: %s", ts, err.Error())}
		}
		declared[ts] = struct{}{}
	}

	// capabilityJustification: keys subset of granted capabilities, no wildcard,
	// values reference declared taskScopes.
	granted := map[string]struct{}{}
	for _, c := range grantedCapabilities {
		granted[c] = struct{}{}
	}
	for cap, scopes := range p.CapabilityJustification {
		if strings.Contains(cap, "*") {
			return &PurposeError{Field: "capabilityJustification", Message: fmt.Sprintf("wildcard capability %q is not allowed; enumerate each capability", cap)}
		}
		if _, ok := granted[cap]; !ok {
			return &PurposeError{Field: "capabilityJustification", Message: fmt.Sprintf("capability %q is justified but not granted to the agent", cap)}
		}
		for _, ts := range scopes {
			if _, ok := declared[ts]; !ok {
				return &PurposeError{Field: "capabilityJustification", Message: fmt.Sprintf("capability %q references taskScope %q which is not in taskScopes", cap, ts)}
			}
		}
	}

	// egressScopes: bare hostnames/domains only (no scheme, path, credentials).
	for _, e := range p.EgressScopes {
		if err := validateEgressTarget(e); err != nil {
			return &PurposeError{Field: "egressScopes", Message: err.Error()}
		}
	}

	// dual-use categories: must enumerate every granted capability explicitly.
	if dualUsePurposeCategories[p.Category] {
		for _, c := range grantedCapabilities {
			if _, ok := p.CapabilityJustification[c]; !ok {
				return &PurposeError{Field: "capabilityJustification", Message: fmt.Sprintf("dual-use category %q requires every granted capability to be justified explicitly; %q is not justified", p.Category, c)}
			}
		}
	}

	return nil
}

// validateEgressTarget enforces that an egress scope is a bare hostname/domain:
// no scheme, no path, no credentials, no whitespace, no port.
func validateEgressTarget(e string) error {
	if e == "" {
		return fmt.Errorf("egress target cannot be empty")
	}
	if strings.Contains(e, "://") {
		return fmt.Errorf("egress target %q must be a bare hostname, not a URL with a scheme", e)
	}
	for _, bad := range []string{"/", "@", " ", ":", "?", "#"} {
		if strings.Contains(e, bad) {
			return fmt.Errorf("egress target %q must be a bare hostname/domain (no %q)", e, bad)
		}
	}
	return nil
}

// CoherenceWarnings returns issuance-time hygiene warnings (atx-spec §1.5.4 /
// proposal §8.1). These are NOT a security control and NOT a rejection during the
// optional phase — they catch lazy incoherence (granted authority justified by no
// task, dead justifications, over-broad declarations). The caller logs them and,
// per the trust-level ramp, may later make them blocking at L>=3.
func (p *DeclaredPurpose) CoherenceWarnings(grantedCapabilities []string) []string {
	if p == nil {
		return nil
	}
	var warnings []string

	// Granted capabilities not justified by any declared task (unjustified reach).
	var unjustified []string
	for _, c := range grantedCapabilities {
		if _, ok := p.CapabilityJustification[c]; !ok {
			unjustified = append(unjustified, c)
		}
	}
	sort.Strings(unjustified)
	for _, c := range unjustified {
		warnings = append(warnings, fmt.Sprintf("capability %q is granted but justified by no declared task", c))
	}

	// Capabilities justified by an empty taskScope list (dead authority).
	var dead []string
	for cap, scopes := range p.CapabilityJustification {
		if len(scopes) == 0 {
			dead = append(dead, cap)
		}
	}
	sort.Strings(dead)
	for _, cap := range dead {
		warnings = append(warnings, fmt.Sprintf("capability %q is justified by no taskScope (dead authority)", cap))
	}

	// Over-broad: measured breadth exceeds the category's tolerated namespaces.
	max := maxNamespacesForBreadth(CategoryBreadthClass(p.Category))
	if max >= 0 {
		if b := p.DeclaredBreadth(); b > max {
			warnings = append(warnings, fmt.Sprintf("declared breadth %d exceeds the %q tier for category %q (max %d capability namespaces)", b, CategoryBreadthClass(p.Category), p.Category, max))
		}
	}

	return warnings
}

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ManifestSource identifies how a manifest snapshot was captured.
type ManifestSource string

const (
	// ManifestSourceWellKnown is the server's own /.well-known/mcp/capabilities declaration.
	// This is authoritative for "what the server claims to expose".
	ManifestSourceWellKnown ManifestSource = "well_known"
	// ManifestSourceAttestation is the tool set observed by attesting agents.
	ManifestSourceAttestation ManifestSource = "attestation"
)

// DriftSeverity grades a manifest change. Removing or changing a tool ranks above adding one,
// and any high-risk capability surface (exec/filesystem/network) escalates to high.
type DriftSeverity string

const (
	DriftSeverityLow    DriftSeverity = "low"
	DriftSeverityMedium DriftSeverity = "medium"
	DriftSeverityHigh   DriftSeverity = "high"
)

// MCPManifestBaseline is an append-only snapshot of an MCP server's tool manifest.
// The newest row for a server has IsCurrent=true and is the baseline future captures diff against.
type MCPManifestBaseline struct {
	ID            uuid.UUID      `json:"id"`
	MCPServerID   uuid.UUID      `json:"mcpServerId"`
	ManifestHash  string         `json:"manifestHash"`
	ToolCount     int            `json:"toolCount"`
	ResourceCount int            `json:"resourceCount"`
	PromptCount   int            `json:"promptCount"`
	CapturedFrom  ManifestSource `json:"capturedFrom"`
	CapturedAt    time.Time      `json:"capturedAt"`
	IsCurrent     bool           `json:"isCurrent"`
}

// MCPManifestDrift records a detected change between the previous baseline and a new capture.
type MCPManifestDrift struct {
	ID           uuid.UUID      `json:"id"`
	MCPServerID  uuid.UUID      `json:"mcpServerId"`
	PrevHash     string         `json:"prevHash"`
	NewHash      string         `json:"newHash"`
	AddedTools   []string       `json:"addedTools"`
	RemovedTools []string       `json:"removedTools"`
	ChangedTools []string       `json:"changedTools"`
	Severity     DriftSeverity  `json:"severity"`
	Source       ManifestSource `json:"source"`
	DetectedAt   time.Time      `json:"detectedAt"`
	Acknowledged bool           `json:"acknowledged"`
}

// ManifestEntry is the canonical per-capability unit used for hashing and diffing.
// SchemaHash makes a schema change detectable without storing the full schema in the manifest.
// It is persisted inside a baseline row so drift diffing does not depend on the mutable
// mcp_server_capabilities rows (which only ever reflect the current state).
type ManifestEntry struct {
	Type       MCPCapabilityType `json:"type"`
	Name       string            `json:"name"`
	SchemaHash string            `json:"schemaHash"`
}

// key uniquely identifies an entry across captures (type + name).
func (e ManifestEntry) key() string {
	return string(e.Type) + ":" + e.Name
}

// CanonicalSchemaHash returns a stable SHA-256 of a capability schema. JSON object key order is
// normalized (Go marshals map keys sorted) so cosmetic reordering or whitespace is not seen as drift;
// array order is preserved because it is semantically meaningful. Invalid/empty schemas hash their raw bytes.
func CanonicalSchemaHash(raw json.RawMessage) string {
	canonical := raw
	if len(raw) > 0 {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				canonical = b
			}
		}
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// BuildManifest converts the stored capabilities into a sorted canonical manifest, returning the
// entries (sorted by type then name), the manifest hash, and per-type counts. Only active
// capabilities are included.
func BuildManifest(caps []*MCPServerCapability) (entries []ManifestEntry, manifestHash string, tools, resources, prompts int) {
	for _, c := range caps {
		if c == nil || !c.IsActive {
			continue
		}
		entries = append(entries, ManifestEntry{
			Type:       c.CapabilityType,
			Name:       c.Name,
			SchemaHash: CanonicalSchemaHash(c.CapabilitySchema),
		})
	}

	manifestHash, tools, resources, prompts = HashManifestEntries(entries)
	return entries, manifestHash, tools, resources, prompts
}

// HashManifestEntries sorts a manifest entry set (by type then name), returns its canonical hash and
// per-type counts. Sorting makes the hash order-invariant; unit separators keep field boundaries
// unambiguous so distinct manifests cannot collide.
func HashManifestEntries(entries []ManifestEntry) (manifestHash string, tools, resources, prompts int) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return entries[i].Name < entries[j].Name
	})

	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(string(e.Type))
		sb.WriteByte(0x1f)
		sb.WriteString(e.Name)
		sb.WriteByte(0x1f)
		sb.WriteString(e.SchemaHash)
		sb.WriteByte(0x1e)
		switch e.Type {
		case MCPCapabilityTypeTool:
			tools++
		case MCPCapabilityTypeResource:
			resources++
		case MCPCapabilityTypePrompt:
			prompts++
		}
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:]), tools, resources, prompts
}

// BuildManifestFromNames builds a names-only manifest from agent-attested tool names. Attestations
// report tool names without schemas, so SchemaHash is left empty; diffing against this manifest is
// add/remove by name, not schema change. Names are deduplicated.
func BuildManifestFromNames(names []string) (entries []ManifestEntry, manifestHash string) {
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		entries = append(entries, ManifestEntry{Type: MCPCapabilityTypeTool, Name: n})
	}
	manifestHash, _, _, _ = HashManifestEntries(entries)
	return entries, manifestHash
}

// DiffManifests compares a previous and new manifest, returning capability keys that were added,
// removed, or changed (same key, different schema hash).
func DiffManifests(prev, next []ManifestEntry) (added, removed, changed []string) {
	prevByKey := make(map[string]ManifestEntry, len(prev))
	for _, e := range prev {
		prevByKey[e.key()] = e
	}
	nextByKey := make(map[string]ManifestEntry, len(next))
	for _, e := range next {
		nextByKey[e.key()] = e
	}

	for k, ne := range nextByKey {
		pe, ok := prevByKey[k]
		if !ok {
			added = append(added, k)
			continue
		}
		if pe.SchemaHash != ne.SchemaHash {
			changed = append(changed, k)
		}
	}
	for k := range prevByKey {
		if _, ok := nextByKey[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return added, removed, changed
}

// highRiskHints are capability-name substrings whose addition or change to a server's manifest
// warrants elevated scrutiny. Heuristic, intentionally conservative toward over-flagging.
var highRiskHints = []string{
	"exec", "shell", "command", "spawn", "eval",
	"file", "fs", "read_file", "write_file", "delete",
	"http", "fetch", "request", "network", "socket",
	"secret", "credential", "token", "env",
}

// ClassifyDriftSeverity grades a diff. Any added or changed capability matching a high-risk hint is
// high; removals or changes without high-risk hints are medium; additions alone are low.
func ClassifyDriftSeverity(added, removed, changed []string) DriftSeverity {
	for _, k := range append(append([]string{}, added...), changed...) {
		lower := strings.ToLower(k)
		for _, hint := range highRiskHints {
			if strings.Contains(lower, hint) {
				return DriftSeverityHigh
			}
		}
	}
	if len(removed) > 0 || len(changed) > 0 {
		return DriftSeverityMedium
	}
	return DriftSeverityLow
}

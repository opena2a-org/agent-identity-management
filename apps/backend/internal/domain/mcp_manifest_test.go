package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func cap(name string, t MCPCapabilityType, schema string) *MCPServerCapability {
	return &MCPServerCapability{
		ID:               uuid.New(),
		MCPServerID:      uuid.Nil,
		Name:             name,
		CapabilityType:   t,
		CapabilitySchema: json.RawMessage(schema),
		IsActive:         true,
	}
}

// Manifest hash must be stable regardless of capability ordering — otherwise a server reordering its
// tools/list would falsely register as drift.
func TestBuildManifest_OrderInvariant(t *testing.T) {
	a := []*MCPServerCapability{
		cap("get_weather", MCPCapabilityTypeTool, `{"type":"object"}`),
		cap("search_code", MCPCapabilityTypeTool, `{"type":"string"}`),
	}
	b := []*MCPServerCapability{
		cap("search_code", MCPCapabilityTypeTool, `{"type":"string"}`),
		cap("get_weather", MCPCapabilityTypeTool, `{"type":"object"}`),
	}
	_, hashA, _, _, _ := BuildManifest(a)
	_, hashB, _, _, _ := BuildManifest(b)
	if hashA != hashB {
		t.Fatalf("manifest hash must be order-invariant: %s != %s", hashA, hashB)
	}
}

// JSON key reordering and whitespace inside a schema must NOT count as drift.
func TestCanonicalSchemaHash_KeyOrderInvariant(t *testing.T) {
	h1 := CanonicalSchemaHash(json.RawMessage(`{"a":1,"b":2}`))
	h2 := CanonicalSchemaHash(json.RawMessage(`{ "b": 2, "a": 1 }`))
	if h1 != h2 {
		t.Fatalf("schema hash must ignore key order/whitespace: %s != %s", h1, h2)
	}
}

// A real schema change (different input shape for the same tool) MUST change the hash.
func TestBuildManifest_SchemaChangeDetected(t *testing.T) {
	before := []*MCPServerCapability{cap("run", MCPCapabilityTypeTool, `{"type":"object","properties":{"cmd":{"type":"string"}}}`)}
	after := []*MCPServerCapability{cap("run", MCPCapabilityTypeTool, `{"type":"object","properties":{"cmd":{"type":"string"},"cwd":{"type":"string"}}}`)}
	_, hb, _, _, _ := BuildManifest(before)
	_, ha, _, _, _ := BuildManifest(after)
	if hb == ha {
		t.Fatal("schema change must produce a different manifest hash")
	}
}

func TestDiffManifests_AddRemoveChange(t *testing.T) {
	prev, _, _, _, _ := BuildManifest([]*MCPServerCapability{
		cap("keep", MCPCapabilityTypeTool, `{"v":1}`),
		cap("gone", MCPCapabilityTypeTool, `{"v":1}`),
		cap("mutate", MCPCapabilityTypeTool, `{"v":1}`),
	})
	next, _, _, _, _ := BuildManifest([]*MCPServerCapability{
		cap("keep", MCPCapabilityTypeTool, `{"v":1}`),
		cap("mutate", MCPCapabilityTypeTool, `{"v":2}`),
		cap("brand_new", MCPCapabilityTypeTool, `{"v":1}`),
	})
	added, removed, changed := DiffManifests(prev, next)
	if len(added) != 1 || added[0] != "tool:brand_new" {
		t.Fatalf("added wrong: %v", added)
	}
	if len(removed) != 1 || removed[0] != "tool:gone" {
		t.Fatalf("removed wrong: %v", removed)
	}
	if len(changed) != 1 || changed[0] != "tool:mutate" {
		t.Fatalf("changed wrong: %v", changed)
	}
}

// Silent injection of a high-risk tool (the core threat) must escalate to high severity.
func TestClassifyDriftSeverity_HighRiskInjection(t *testing.T) {
	if got := ClassifyDriftSeverity([]string{"tool:exec_shell"}, nil, nil); got != DriftSeverityHigh {
		t.Fatalf("high-risk add must be high, got %s", got)
	}
	if got := ClassifyDriftSeverity([]string{"tool:list_items"}, nil, nil); got != DriftSeverityLow {
		t.Fatalf("benign add must be low, got %s", got)
	}
	if got := ClassifyDriftSeverity(nil, []string{"tool:list_items"}, nil); got != DriftSeverityMedium {
		t.Fatalf("removal must be medium, got %s", got)
	}
}

// Inactive capabilities are excluded from the manifest.
func TestBuildManifest_SkipsInactive(t *testing.T) {
	c := cap("dormant", MCPCapabilityTypeTool, `{}`)
	c.IsActive = false
	entries, _, tools, _, _ := BuildManifest([]*MCPServerCapability{c})
	if len(entries) != 0 || tools != 0 {
		t.Fatalf("inactive capability must be excluded: entries=%d tools=%d", len(entries), tools)
	}
}

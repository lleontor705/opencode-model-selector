package agentcatalog

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

type fakeRuntimeRegistry struct {
	agents []opencode.RuntimeAgent
	err    error
}

func (f fakeRuntimeRegistry) Agents(context.Context, string) ([]opencode.RuntimeAgent, error) {
	return f.agents, f.err
}

type fakeStaticCatalog struct {
	groups config.AgentGroups
	models map[string]resolvedModel
}

func (f fakeStaticCatalog) GetAgentGroups() config.AgentGroups { return f.groups }

func (f fakeStaticCatalog) ResolveEffectiveModel(name string) (string, config.ModelProvenance, bool) {
	model := f.models[name]
	return model.model, model.provenance, model.ok
}

func TestDiscoverRuntimeSuccessDoesNotMixStaticOnlyNames(t *testing.T) {
	discovery := Discovery{
		Runtime: fakeRuntimeRegistry{agents: []opencode.RuntimeAgent{{Name: "runtime", Mode: "primary"}}},
		Static: fakeStaticCatalog{
			groups: config.AgentGroups{Subagents: []string{"static-only"}},
			models: map[string]resolvedModel{
				"runtime": {model: "configured/runtime", provenance: config.ModelProvenanceInlineJSON, ok: true},
			},
		},
	}

	got := discovery.Discover(context.Background(), "project")
	want := Catalog{Buckets: Buckets{Primary: []AgentRecord{{
		Name: "runtime", Role: RolePrimary, Source: SourceRuntime, Status: StatusVisible,
		Model: "configured/runtime", Provenance: ProvenanceAgent,
	}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover() = %#v, want %#v", got, want)
	}
}

func TestDiscoverRuntimeFailuresUseStaticFallback(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{name: "startup", cause: errors.New("start failed")},
		{name: "timeout", cause: context.DeadlineExceeded},
		{name: "runtime HTTP", cause: errors.New("HTTP 503")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discovery := Discovery{
				Runtime: fakeRuntimeRegistry{err: tt.cause},
				Static: fakeStaticCatalog{
					groups: config.AgentGroups{
						Primary:   []string{"primary", "duplicate"},
						Subagents: []string{"subagent", "duplicate"},
						All:       []string{"missing", "all", "duplicate"},
					},
					models: map[string]resolvedModel{
						"primary":  {model: "global/model", provenance: config.ModelProvenanceGlobalTopLevel, ok: true},
						"subagent": {model: "project/model", provenance: config.ModelProvenanceProjectMarkdown, ok: true},
						"all":      {model: "markdown/model", provenance: config.ModelProvenanceGlobalMarkdown, ok: true},
					},
				},
			}

			got := discovery.Discover(context.Background(), "project")
			if !got.Degraded {
				t.Fatal("Discover().Degraded = false, want true")
			}
			if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != DiagnosticRuntimeDiscoveryFailed {
				t.Fatalf("Diagnostics = %#v, want one runtime failure", got.Diagnostics)
			}
			if !errors.Is(got.Diagnostics[0], tt.cause) {
				t.Fatalf("diagnostic does not preserve cause %v", tt.cause)
			}
			primary := recordNamed(t, got.Records(), "primary")
			if primary.Model != "global/model" || primary.Provenance != ProvenanceGlobal {
				t.Fatalf("primary fallback model = %#v", primary)
			}
			subagent := recordNamed(t, got.Records(), "subagent")
			if subagent.Model != "project/model" || subagent.Provenance != ProvenanceAgent {
				t.Fatalf("subagent fallback model = %#v", subagent)
			}
			if names(got.Buckets.All) != "all,missing" {
				t.Fatalf("All names = %q, want all,missing", names(got.Buckets.All))
			}
			all := recordNamed(t, got.Records(), "all")
			if all.Role != RoleAll || all.Status != StatusVisible || all.Model != "markdown/model" || all.Provenance != ProvenanceAgent {
				t.Fatalf("all fallback record = %#v", all)
			}
			missing := recordNamed(t, got.Records(), "missing")
			if missing.Role != RoleAll || missing.Status != StatusVisible || missing.Provenance != ProvenanceDefault {
				t.Fatalf("missing-mode fallback record = %#v", missing)
			}
			assertUniqueNames(t, got.Records())
			for _, record := range got.Records() {
				if record.Source != SourceConfig || record.Native || record.Hidden {
					t.Fatalf("fallback fabricated runtime metadata: %#v", record)
				}
			}
		})
	}
}

func TestDiscoverEmptyStaticFallbackRemainsDegraded(t *testing.T) {
	cause := errors.New("runtime unavailable")
	got := (Discovery{
		Runtime: fakeRuntimeRegistry{err: cause},
		Static:  fakeStaticCatalog{},
	}).Discover(context.Background(), "project")

	if !got.Degraded || len(got.Records()) != 0 || len(got.Diagnostics) != 1 {
		t.Fatalf("Discover() = %#v, want empty degraded catalog with one diagnostic", got)
	}
	if !errors.Is(got.Diagnostics[0], cause) {
		t.Fatalf("diagnostic does not preserve cause %v", cause)
	}
}

func names(records []AgentRecord) string {
	result := ""
	for _, record := range records {
		if result != "" {
			result += ","
		}
		result += record.Name
	}
	return result
}

func assertUniqueNames(t *testing.T, records []AgentRecord) {
	t.Helper()
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.Name] {
			t.Fatalf("duplicate name %q in %#v", record.Name, records)
		}
		seen[record.Name] = true
	}
}

func recordNamed(t *testing.T, records []AgentRecord, name string) AgentRecord {
	t.Helper()
	for _, record := range records {
		if record.Name == name {
			return record
		}
	}
	t.Fatalf("record %q not found in %#v", name, records)
	return AgentRecord{}
}

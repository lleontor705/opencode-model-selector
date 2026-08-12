package agentcatalog

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

type fakeModelResolver struct {
	models     map[string]resolvedModel
	staticMode map[string]string
	staticHide map[string]bool
}

type resolvedModel struct {
	model      string
	provenance config.ModelProvenance
	ok         bool
}

func (f fakeModelResolver) ResolveEffectiveModel(name string) (string, config.ModelProvenance, bool) {
	model := f.models[name]
	return model.model, model.provenance, model.ok
}

func boolPointer(value bool) *bool { return &value }

func TestComposeRuntimeAgents(t *testing.T) {
	runtime := []opencode.RuntimeAgent{
		{Name: "review", Mode: "subagent", Hidden: boolPointer(true)},
		{Name: "build", Mode: "primary", Native: true, Hidden: boolPointer(false)},
		{Name: "utility", Mode: "primary", Native: true, Hidden: boolPointer(true)},
		{Name: "general", Mode: "all"},
		{Name: "build", Mode: "subagent", Hidden: boolPointer(true)},
	}
	resolver := fakeModelResolver{models: map[string]resolvedModel{
		"review":  {model: "inline/model", provenance: config.ModelProvenanceInlineJSON, ok: true},
		"build":   {model: "project/model", provenance: config.ModelProvenanceProjectMarkdown, ok: true},
		"utility": {model: "global-md/model", provenance: config.ModelProvenanceGlobalMarkdown, ok: true},
		"general": {model: "global/model", provenance: config.ModelProvenanceGlobalTopLevel, ok: true},
	}}

	got := ComposeRuntimeAgents(runtime, resolver)
	want := Buckets{
		Primary: []AgentRecord{{
			Name: "build", Role: RolePrimary, Source: SourceRuntime, Native: true,
			Status: StatusVisible, Model: "project/model", Provenance: ProvenanceAgent,
		}},
		Subagent: []AgentRecord{{
			Name: "review", Role: RoleSubagent, Source: SourceRuntime, Hidden: true,
			Status: StatusHidden, Model: "inline/model", Provenance: ProvenanceAgent,
		}},
		All: []AgentRecord{{
			Name: "general", Role: RoleAll, Source: SourceRuntime, Status: StatusVisible,
			Model: "global/model", Provenance: ProvenanceGlobal,
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ComposeRuntimeAgents() = %#v, want %#v", got, want)
	}
}

func TestComposeRuntimeAgentsUsesOnlyRuntimeIdentityMetadata(t *testing.T) {
	runtime := []opencode.RuntimeAgent{{
		Name: "runtime-name", Mode: "primary", Native: true, Hidden: boolPointer(false),
	}}
	resolver := fakeModelResolver{
		models: map[string]resolvedModel{
			"runtime-name": {model: "configured/model", provenance: config.ModelProvenanceGlobalMarkdown, ok: true},
		},
		staticMode: map[string]string{"runtime-name": "subagent"},
		staticHide: map[string]bool{"runtime-name": true},
	}

	got := ComposeRuntimeAgents(runtime, resolver).Records()
	want := []AgentRecord{{
		Name: "runtime-name", Role: RolePrimary, Source: SourceRuntime, Native: true,
		Status: StatusVisible, Model: "configured/model", Provenance: ProvenanceAgent,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestComposeRuntimeAgentsMapsModelProvenance(t *testing.T) {
	tests := []struct {
		name       string
		configured config.ModelProvenance
		want       ModelProvenance
	}{
		{name: "inline JSON is agent-specific", configured: config.ModelProvenanceInlineJSON, want: ProvenanceAgent},
		{name: "project markdown is agent-specific", configured: config.ModelProvenanceProjectMarkdown, want: ProvenanceAgent},
		{name: "global markdown is agent-specific", configured: config.ModelProvenanceGlobalMarkdown, want: ProvenanceAgent},
		{name: "top-level model is global fallback", configured: config.ModelProvenanceGlobalTopLevel, want: ProvenanceGlobal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := fakeModelResolver{models: map[string]resolvedModel{
				"agent": {model: "resolved/model", provenance: tt.configured, ok: true},
			}}
			got := ComposeRuntimeAgents([]opencode.RuntimeAgent{{Name: "agent"}}, resolver).Records()
			if len(got) != 1 || got[0].Model != "resolved/model" || got[0].Provenance != tt.want {
				t.Fatalf("records = %#v, want model resolved/model with provenance %q", got, tt.want)
			}
		})
	}
}

func TestComposeRuntimeAgentsWithoutConfiguredModelUsesDefaultProvenance(t *testing.T) {
	got := ComposeRuntimeAgents(
		[]opencode.RuntimeAgent{{Name: "plain"}},
		fakeModelResolver{models: map[string]resolvedModel{}},
	).Records()
	want := []AgentRecord{{
		Name: "plain", Role: RoleAll, Source: SourceRuntime,
		Status: StatusVisible, Provenance: ProvenanceDefault,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestComposeRuntimeAgentsIsDeterministic(t *testing.T) {
	runtime := []opencode.RuntimeAgent{
		{Name: "zeta", Mode: "all"},
		{Name: "Alpha", Mode: "subagent"},
		{Name: "same", Mode: "subagent"},
		{Name: "same", Mode: "primary", Native: true},
	}
	resolver := fakeModelResolver{models: map[string]resolvedModel{
		"same": {model: "same/model", provenance: config.ModelProvenanceInlineJSON, ok: true},
	}}
	want := ComposeRuntimeAgents(runtime, resolver)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		shuffled := append([]opencode.RuntimeAgent(nil), runtime...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := ComposeRuntimeAgents(shuffled, resolver); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d produced %#v, want %#v", i, got, want)
		}
	}
}

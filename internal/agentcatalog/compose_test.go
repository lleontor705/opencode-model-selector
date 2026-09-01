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

// ---------------------------------------------------------------------------
// ComposeModels and Variant Tests (catalog-001)
// ---------------------------------------------------------------------------

type fakeConfigDataProvider struct {
	data map[string]interface{}
}

func (f fakeConfigDataProvider) Data() map[string]interface{} { return f.data }

type fakeVariantResolver struct {
	variants map[string][]opencode.VariantDescriptor
}

func (f fakeVariantResolver) ResolveModelVariants(provider, modelID string) []opencode.VariantDescriptor {
	return f.variants[provider+"/"+modelID]
}

func TestComposeModels_WithConfiguredVariants(t *testing.T) {
	cfgData := map[string]interface{}{
		"provider": map[string]interface{}{
			"anthropic": map[string]interface{}{
				"models": map[string]interface{}{
					"claude-sonnet-4-20250514": map[string]interface{}{
						"variants": map[string]interface{}{
							"medium": map[string]interface{}{"options": map[string]interface{}{"effort": "medium"}},
							"high":   map[string]interface{}{"options": map[string]interface{}{"effort": "high"}},
							"low":    map[string]interface{}{"options": map[string]interface{}{"effort": "low"}},
						},
					},
				},
			},
			"openai": map[string]interface{}{
				"options": map[string]interface{}{"max_tokens": 4096},
			},
		},
	}

	discovered := []opencode.Model{
		{Provider: "anthropic", ID: "claude-sonnet-4-20250514", FullName: "anthropic/claude-sonnet-4-20250514"},
		{Provider: "openai", ID: "gpt-4o", FullName: "openai/gpt-4o"},
	}

	// Test with ConfigDataProvider
	cdp := fakeConfigDataProvider{data: cfgData}
	composed := ComposeModels(discovered, cdp)
	if len(composed) != 2 {
		t.Fatalf("ComposeModels() returned %d models, want 2", len(composed))
	}

	// anthropic model checks
	anthropic := composed[0]
	if anthropic.FullName != "anthropic/claude-sonnet-4-20250514" {
		t.Fatalf("anthropic.FullName = %q, want anthropic/claude-sonnet-4-20250514", anthropic.FullName)
	}
	if len(anthropic.Variants) != 3 {
		t.Fatalf("anthropic.Variants length = %d, want 3", len(anthropic.Variants))
	}
	if anthropic.Variants[0].Name != "high" || anthropic.Variants[1].Name != "low" || anthropic.Variants[2].Name != "medium" {
		t.Fatalf("anthropic.Variants order = %#v, want high, low, medium", anthropic.Variants)
	}
	if !reflect.DeepEqual(anthropic.Variants[0].Options, map[string]interface{}{"effort": "high"}) {
		t.Fatalf("anthropic.Variants[0].Options = %#v, want effort:high", anthropic.Variants[0].Options)
	}

	// openai model checks (no variants declared)
	openai := composed[1]
	if openai.FullName != "openai/gpt-4o" {
		t.Fatalf("openai.FullName = %q, want openai/gpt-4o", openai.FullName)
	}
	if len(openai.Variants) != 0 {
		t.Fatalf("openai.Variants length = %d, want 0 (no synthetic entries)", len(openai.Variants))
	}

	// Test ComposeModelVariants alias
	composedAlias := ComposeModelVariants(discovered, cfgData)
	if !reflect.DeepEqual(composed, composedAlias) {
		t.Fatalf("ComposeModelVariants alias produced %#v, want %#v", composedAlias, composed)
	}
}

func TestComposeModels_WithVariantResolver(t *testing.T) {
	vr := fakeVariantResolver{
		variants: map[string][]opencode.VariantDescriptor{
			"custom/model": {
				{Name: "turbo", Options: map[string]interface{}{"fast": true}},
				{Name: "base"},
			},
		},
	}

	models := []opencode.Model{
		{Provider: "custom", ID: "model", FullName: "custom/model"},
	}

	composed := ComposeModels(models, vr)
	if len(composed) != 1 {
		t.Fatalf("composed length = %d, want 1", len(composed))
	}
	// Deterministically sorted: base, turbo
	if len(composed[0].Variants) != 2 || composed[0].Variants[0].Name != "base" || composed[0].Variants[1].Name != "turbo" {
		t.Fatalf("composed variants = %#v, want base, turbo", composed[0].Variants)
	}
}

func TestComposeModels_MissingOrMalformedYieldsNoSyntheticEntries(t *testing.T) {
	models := []opencode.Model{
		{Provider: "openai", ID: "gpt-4o", FullName: "openai/gpt-4o"},
	}

	for _, malformed := range []any{
		nil,
		map[string]interface{}{},
		"not-a-map",
		12345,
		map[string]interface{}{"provider": "malformed"},
		map[string]interface{}{"provider": map[string]interface{}{"openai": "malformed"}},
		map[string]interface{}{"provider": map[string]interface{}{"openai": map[string]interface{}{"models": "malformed"}}},
	} {
		composed := ComposeModels(models, malformed)
		if len(composed) != 1 {
			t.Fatalf("malformed %v returned %d models, want 1", malformed, len(composed))
		}
		if composed[0].FullName != "openai/gpt-4o" {
			t.Fatalf("FullName changed to %q", composed[0].FullName)
		}
		if len(composed[0].Variants) != 0 {
			t.Fatalf("malformed %v produced synthetic variants: %#v", malformed, composed[0].Variants)
		}
	}
}

func TestComposeModels_EmptySliceInput(t *testing.T) {
	composed := ComposeModels([]opencode.Model{}, nil)
	if composed == nil || len(composed) != 0 {
		t.Fatalf("ComposeModels([]) = %#v, want empty non-nil slice", composed)
	}
}

func TestComposeModels_PreservesFullIdentity(t *testing.T) {
	cfg := map[string]interface{}{
		"provider": map[string]interface{}{
			"anthropic": map[string]interface{}{
				"models": map[string]interface{}{
					"claude-sonnet-4-20250514": map[string]interface{}{
						"variants": map[string]interface{}{
							"high": map[string]interface{}{},
						},
					},
				},
			},
		},
	}
	models := []opencode.Model{
		{Provider: "anthropic", ID: "claude-sonnet-4-20250514", FullName: "anthropic/claude-sonnet-4-20250514"},
	}
	composed := ComposeModels(models, cfg)
	if composed[0].FullName != "anthropic/claude-sonnet-4-20250514" {
		t.Fatalf("FullName was mutated to %q", composed[0].FullName)
	}
	if composed[0].Provider != "anthropic" || composed[0].ID != "claude-sonnet-4-20250514" {
		t.Fatalf("Provider/ID mutated: %#v", composed[0])
	}
}

package agentcatalog

import (
	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// ModelResolver is the model-only configuration boundary used by runtime
// catalog composition. *config.Config satisfies this interface.
type ModelResolver interface {
	ResolveEffectiveModel(string) (string, config.ModelProvenance, bool)
}

// VariantResolver is the variant configuration boundary used by catalog
// composition to retrieve variant descriptors for a model.
type VariantResolver interface {
	ResolveModelVariants(provider, modelID string) []opencode.VariantDescriptor
}

// ConfigDataProvider is the configuration boundary for sources exposing raw
// configuration data maps (such as *config.Config).
type ConfigDataProvider interface {
	Data() map[string]interface{}
}

// ModelVariantResolver combines model resolution for agents and variant
// resolution for models.
type ModelVariantResolver interface {
	ModelResolver
	VariantResolver
}

// ComposeModels joins discovered models with configured variant descriptors
// from a resolver or raw configuration source. Full model identities are
// preserved, variants are sorted deterministically, and missing or malformed
// maps yield no synthetic entries.
func ComposeModels(models []opencode.Model, resolver any) []opencode.Model {
	if len(models) == 0 {
		return []opencode.Model{}
	}
	if resolver == nil {
		return opencode.JoinModelVariants(models, nil)
	}
	if vr, ok := resolver.(VariantResolver); ok {
		result := make([]opencode.Model, len(models))
		for i, m := range models {
			result[i] = opencode.Model{
				Provider: m.Provider,
				ID:       m.ID,
				FullName: m.FullName,
			}
			variants := vr.ResolveModelVariants(m.Provider, m.ID)
			if len(variants) > 0 {
				opencode.SortVariants(variants)
				result[i].Variants = variants
			}
		}
		return result
	}
	if cdp, ok := resolver.(ConfigDataProvider); ok {
		return opencode.JoinModelVariants(models, cdp.Data())
	}
	if data, ok := resolver.(map[string]interface{}); ok {
		return opencode.JoinModelVariants(models, data)
	}
	return opencode.JoinModelVariants(models, nil)
}

// ComposeModelVariants is an alias for ComposeModels.
func ComposeModelVariants(models []opencode.Model, resolver any) []opencode.Model {
	return ComposeModels(models, resolver)
}

// ComposeRuntimeAgents combines authoritative runtime identities with their
// effective configured models, then applies the catalog classification rules.
// Configuration cannot add identities or override runtime role, native, or
// hidden metadata through this model-only boundary.
func ComposeRuntimeAgents(runtime []opencode.RuntimeAgent, resolver ModelResolver) Buckets {
	records := make([]AgentRecord, 0, len(runtime))
	for _, agent := range runtime {
		record := AgentRecord{
			Name:       agent.Name,
			Role:       runtimeRole(agent.Mode),
			Source:     SourceRuntime,
			Native:     agent.Native,
			Hidden:     agent.Hidden != nil && *agent.Hidden,
			Provenance: ProvenanceDefault,
		}
		if resolver != nil {
			if model, provenance, ok := resolver.ResolveEffectiveModel(agent.Name); ok {
				record.Model = model
				record.Provenance = catalogProvenance(provenance)
			}
		}
		records = append(records, record)
	}
	return Classify(records)
}

func runtimeRole(mode string) Role {
	switch mode {
	case string(RolePrimary):
		return RolePrimary
	case string(RoleSubagent):
		return RoleSubagent
	default:
		return RoleAll
	}
}

func catalogProvenance(provenance config.ModelProvenance) ModelProvenance {
	switch provenance {
	case config.ModelProvenanceGlobalTopLevel:
		return ProvenanceGlobal
	case config.ModelProvenanceInlineJSON,
		config.ModelProvenanceProjectMarkdown,
		config.ModelProvenanceGlobalMarkdown:
		return ProvenanceAgent
	default:
		return ProvenanceDefault
	}
}

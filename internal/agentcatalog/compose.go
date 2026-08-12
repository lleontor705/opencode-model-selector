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

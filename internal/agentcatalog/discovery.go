package agentcatalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/lleontor705/opencode-model-selector/internal/config"
	"github.com/lleontor705/opencode-model-selector/internal/opencode"
)

// RuntimeRegistry is the injectable structured runtime discovery boundary.
// opencode.RuntimeAgentProbe satisfies this interface.
type RuntimeRegistry interface {
	Agents(context.Context, string) ([]opencode.RuntimeAgent, error)
}

var _ RuntimeRegistry = opencode.RuntimeAgentProbe{}

// StaticCatalog is the static identity and effective-model boundary used only
// when runtime discovery fails. *config.Config satisfies this interface.
type StaticCatalog interface {
	ModelResolver
	GetAgentGroups() config.AgentGroups
}

var _ StaticCatalog = (*config.Config)(nil)

// DiagnosticCode identifies a catalog diagnostic without requiring callers to
// parse presentation text.
type DiagnosticCode string

const DiagnosticRuntimeDiscoveryFailed DiagnosticCode = "runtime_discovery_failed"

// Diagnostic describes degraded discovery and retains the underlying cause.
type Diagnostic struct {
	Code    DiagnosticCode
	Message string
	Cause   error
}

func (d Diagnostic) Error() string {
	if d.Cause == nil {
		return d.Message
	}
	return fmt.Sprintf("%s: %v", d.Message, d.Cause)
}

// Unwrap exposes the original runtime failure to errors.Is/errors.As.
func (d Diagnostic) Unwrap() error { return d.Cause }

// Catalog is the discovery result consumed by catalog presentation and model
// selection. Runtime success is authoritative; Degraded means static fallback
// was used after a runtime failure.
type Catalog struct {
	Buckets     Buckets
	Diagnostics []Diagnostic
	Degraded    bool
}

// Records returns a copy of all catalog records in section order.
func (c Catalog) Records() []AgentRecord { return c.Buckets.Records() }

// Discovery coordinates authoritative runtime discovery and static fallback.
type Discovery struct {
	Runtime RuntimeRegistry
	Static  StaticCatalog
}

// Discover returns runtime identities when available. Any runtime error yields
// a deterministic static catalog and exactly one degraded diagnostic.
func (d Discovery) Discover(ctx context.Context, directory string) Catalog {
	runtimeAgents, err := runtimeAgents(ctx, directory, d.Runtime)
	if err == nil {
		return Catalog{Buckets: ComposeRuntimeAgents(runtimeAgents, d.Static)}
	}

	return Catalog{
		Buckets: composeStaticAgents(d.Static),
		Diagnostics: []Diagnostic{{
			Code:    DiagnosticRuntimeDiscoveryFailed,
			Message: "runtime agent discovery failed; using static catalog",
			Cause:   err,
		}},
		Degraded: true,
	}
}

func runtimeAgents(ctx context.Context, directory string, registry RuntimeRegistry) ([]opencode.RuntimeAgent, error) {
	if registry == nil {
		return nil, errors.New("runtime agent registry is required")
	}
	return registry.Agents(ctx, directory)
}

func composeStaticAgents(static StaticCatalog) Buckets {
	if static == nil {
		return Buckets{}
	}
	groups := static.GetAgentGroups()
	records := make([]AgentRecord, 0, len(groups.Primary)+len(groups.Subagents)+len(groups.All))
	seen := make(map[string]struct{}, cap(records))
	appendGroup := func(names []string, role Role) {
		for _, name := range names {
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			record := AgentRecord{
				Name:       name,
				Role:       role,
				Source:     SourceConfig,
				Provenance: ProvenanceDefault,
			}
			if model, provenance, ok := static.ResolveEffectiveModel(name); ok {
				record.Model = model
				record.Provenance = catalogProvenance(provenance)
			}
			records = append(records, record)
		}
	}
	appendGroup(groups.Primary, RolePrimary)
	appendGroup(groups.Subagents, RoleSubagent)
	appendGroup(groups.All, RoleAll)
	return Classify(records)
}

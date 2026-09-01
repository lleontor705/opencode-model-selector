// Package agentcatalog defines the adapter-independent agent catalog domain.
package agentcatalog

import (
	"slices"
	"strings"
)

// Role is the exclusive section in which an agent is presented.
type Role string

const (
	RolePrimary  Role = "primary"
	RoleSubagent Role = "subagent"
	RoleAll      Role = "all"
)

// Source identifies where an agent identity was discovered.
type Source string

const (
	SourceRuntime         Source = "runtime"
	SourceConfig          Source = "config"
	SourceGlobalMarkdown  Source = "global-markdown"
	SourceProjectMarkdown Source = "project-markdown"
	SourcePlugin          Source = "plugin"
)

// Status carries presentation state without exposing adapter-specific fields.
type Status string

const (
	StatusVisible Status = "visible"
	StatusHidden  Status = "hidden"
)

// ModelProvenance identifies the configuration layer that supplied Model.
type ModelProvenance string

const (
	ProvenanceDefault ModelProvenance = "default"
	ProvenanceGlobal  ModelProvenance = "global"
	ProvenanceAgent   ModelProvenance = "agent"
)

// AgentRecord is the unified, presentation-safe catalog record.
type AgentRecord struct {
	Name       string
	Role       Role
	Source     Source
	Native     bool
	Hidden     bool
	Status     Status
	Model      string
	Provenance ModelProvenance
}

// Buckets holds mutually exclusive role sections.
type Buckets struct {
	Primary  []AgentRecord
	Subagent []AgentRecord
	All      []AgentRecord
}

// Records returns a copy of all records in Primary, Subagent, All section order.
func (b Buckets) Records() []AgentRecord {
	records := make([]AgentRecord, 0, len(b.Primary)+len(b.Subagent)+len(b.All))
	records = append(records, b.Primary...)
	records = append(records, b.Subagent...)
	records = append(records, b.All...)
	return records
}

// Classify normalizes, filters, deduplicates, and orders catalog records.
func Classify(input []AgentRecord) Buckets {
	records := append([]AgentRecord(nil), input...)
	for i := range records {
		if records[i].Role == "" {
			records[i].Role = RoleAll
		}
		if records[i].Hidden {
			records[i].Status = StatusHidden
		} else {
			records[i].Status = StatusVisible
		}
	}

	// Sorting before exact-name dedup makes the winner independent of input order.
	slices.SortFunc(records, compareMetadata)
	seen := make(map[string]struct{}, len(records))
	var buckets Buckets
	for _, record := range records {
		if record.Native && record.Hidden {
			continue
		}
		if _, exists := seen[record.Name]; exists {
			continue
		}
		seen[record.Name] = struct{}{}
		switch record.Role {
		case RolePrimary:
			buckets.Primary = append(buckets.Primary, record)
		case RoleSubagent:
			buckets.Subagent = append(buckets.Subagent, record)
		default:
			record.Role = RoleAll
			buckets.All = append(buckets.All, record)
		}
	}

	slices.SortFunc(buckets.Primary, compareName)
	slices.SortFunc(buckets.Subagent, compareName)
	slices.SortFunc(buckets.All, compareName)
	return buckets
}

func compareName(a, b AgentRecord) int {
	if folded := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); folded != 0 {
		return folded
	}
	return strings.Compare(a.Name, b.Name)
}

func compareMetadata(a, b AgentRecord) int {
	for _, comparison := range []int{
		strings.Compare(a.Name, b.Name),
		strings.Compare(string(a.Role), string(b.Role)),
		strings.Compare(string(a.Source), string(b.Source)),
		compareBool(a.Native, b.Native),
		compareBool(a.Hidden, b.Hidden),
		strings.Compare(a.Model, b.Model),
		strings.Compare(string(a.Provenance), string(b.Provenance)),
	} {
		if comparison != 0 {
			return comparison
		}
	}
	return 0
}

func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}

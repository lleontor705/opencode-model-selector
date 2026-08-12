package agentcatalog

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		in   []AgentRecord
		want Buckets
	}{
		{
			name: "native hidden utility agents are excluded by properties",
			in: []AgentRecord{
				{Name: "compaction", Role: RolePrimary, Native: true, Hidden: true},
				{Name: "title", Role: RoleSubagent, Native: true, Hidden: true},
				{Name: "summary", Native: true, Hidden: true},
			},
			want: Buckets{},
		},
		{
			name: "visible native and hidden custom agents are retained",
			in: []AgentRecord{
				{Name: "build", Role: RolePrimary, Native: true},
				{Name: "plugin-secret", Role: RoleSubagent, Source: SourcePlugin, Hidden: true},
			},
			want: Buckets{
				Primary:  []AgentRecord{{Name: "build", Role: RolePrimary, Native: true, Status: StatusVisible}},
				Subagent: []AgentRecord{{Name: "plugin-secret", Role: RoleSubagent, Source: SourcePlugin, Hidden: true, Status: StatusHidden}},
			},
		},
		{
			name: "excluded native hidden duplicate cannot suppress custom record",
			in: []AgentRecord{
				{Name: "shared", Role: RolePrimary, Native: true, Hidden: true},
				{Name: "shared", Role: RoleSubagent, Source: SourcePlugin, Hidden: true},
			},
			want: Buckets{Subagent: []AgentRecord{{
				Name: "shared", Role: RoleSubagent, Source: SourcePlugin, Hidden: true, Status: StatusHidden,
			}}},
		},
		{
			name: "missing and explicit all modes share the exclusive all bucket",
			in: []AgentRecord{
				{Name: "missing"},
				{Name: "explicit", Role: RoleAll},
			},
			want: Buckets{All: []AgentRecord{
				{Name: "explicit", Role: RoleAll, Status: StatusVisible},
				{Name: "missing", Role: RoleAll, Status: StatusVisible},
			}},
		},
		{
			name: "exact duplicate names select the first stable metadata tuple",
			in: []AgentRecord{
				{Name: "same", Role: RoleSubagent, Source: SourceRuntime, Model: "z/model", Provenance: ProvenanceAgent},
				{Name: "same", Role: RolePrimary, Source: SourceRuntime, Model: "a/model", Provenance: ProvenanceGlobal},
			},
			want: Buckets{Primary: []AgentRecord{{
				Name: "same", Role: RolePrimary, Source: SourceRuntime, Status: StatusVisible,
				Model: "a/model", Provenance: ProvenanceGlobal,
			}}},
		},
		{
			name: "name ordering folds case then uses original as tie breaker",
			in: []AgentRecord{
				{Name: "beta", Role: RolePrimary},
				{Name: "alpha", Role: RoleAll},
				{Name: "ALPHA", Role: RolePrimary},
				{Name: "Alpha", Role: RolePrimary},
				{Name: "zeta", Role: RoleSubagent},
			},
			want: Buckets{
				Primary: []AgentRecord{
					{Name: "ALPHA", Role: RolePrimary, Status: StatusVisible},
					{Name: "Alpha", Role: RolePrimary, Status: StatusVisible},
					{Name: "beta", Role: RolePrimary, Status: StatusVisible},
				},
				Subagent: []AgentRecord{{Name: "zeta", Role: RoleSubagent, Status: StatusVisible}},
				All:      []AgentRecord{{Name: "alpha", Role: RoleAll, Status: StatusVisible}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Classify() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestClassifyIsStableForShuffledInput(t *testing.T) {
	input := []AgentRecord{
		{Name: "same", Role: RoleSubagent, Source: SourceRuntime, Model: "z/model"},
		{Name: "same", Role: RolePrimary, Source: SourceRuntime, Model: "a/model"},
		{Name: "bravo", Role: RoleAll},
		{Name: "Alpha", Role: RoleAll},
		{Name: "hidden-custom", Role: RoleSubagent, Hidden: true},
		{Name: "hidden-native", Role: RolePrimary, Hidden: true, Native: true},
	}
	want := Classify(input)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		shuffled := append([]AgentRecord(nil), input...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := Classify(shuffled); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d produced %#v, want %#v", i, got, want)
		}
	}
}

func TestBucketsRecordsUseSectionOrder(t *testing.T) {
	b := Classify([]AgentRecord{
		{Name: "all", Role: RoleAll},
		{Name: "sub", Role: RoleSubagent},
		{Name: "primary", Role: RolePrimary},
	})
	got := b.Records()
	want := []string{"primary", "sub", "all"}
	if len(got) != len(want) {
		t.Fatalf("Records() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("Records()[%d].Name = %q, want %q", i, got[i].Name, want[i])
		}
	}
}

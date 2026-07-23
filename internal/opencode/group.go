package opencode

import "sort"

// GroupByProvider groups a slice of Model values by their Provider key.
// Each provider's slice is sorted alphabetically by Model.ID.
//
// An empty (or nil) input slice returns an initialized empty map (never nil).
//
// Spec: REQ-OC-004
func GroupByProvider(models []Model) map[string][]Model {
	grouped := make(map[string][]Model)

	for _, m := range models {
		grouped[m.Provider] = append(grouped[m.Provider], m)
	}

	// Sort each provider's models alphabetically by ID
	for provider := range grouped {
		sort.Slice(grouped[provider], func(i, j int) bool {
			return grouped[provider][i].ID < grouped[provider][j].ID
		})
	}

	return grouped
}

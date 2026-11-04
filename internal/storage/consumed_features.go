package storage

import "sort"

// PolicyFeatureNames lists every feature name the embedded standard-field
// policy and describe catalog read. It is derived from the loaded data, so a
// gate added to the data appears here without a separate list.
func PolicyFeatureNames() []string {
	seen := map[string]bool{}
	for _, policy := range standardFieldPolicies {
		for _, feature := range policy.RequiredFeatures {
			seen[canonicalFeatureName(feature)] = true
		}
		for _, group := range policy.RequiredFeaturesAnyOf {
			for _, feature := range group {
				seen[canonicalFeatureName(feature)] = true
			}
		}
		for _, gate := range policy.gates {
			for _, feature := range gate.Features {
				seen[canonicalFeatureName(feature)] = true
			}
		}
	}
	for _, entry := range standardObjectCatalogData {
		for feature := range entry.FeatureFields {
			seen[canonicalFeatureName(feature)] = true
		}
	}
	for _, gate := range standardObjectFeatures {
		seen[canonicalFeatureName(gate.feature)] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

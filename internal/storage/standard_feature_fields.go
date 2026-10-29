package storage

import (
	"sort"

	"github.com/glade-sh/glade/internal/project"
)

func standardOptionalFieldEnabled(object, field, signature string) bool {
	policy := standardFieldPolicyFor(object)
	if policy == nil {
		return true
	}
	if gate, ok := standardPolicyLookup(policy.gates, field); ok {
		return standardFieldPolicyGateEnabled(gate, signature)
	}
	absent, _ := standardPolicyLookup(policy.absent, field)
	return !absent
}

func standardOptionalFieldsNeedRepair(definition ObjectDefinition, signature string) bool {
	policy := standardFieldPolicyFor(definition.APIName)
	if policy == nil {
		return false
	}
	if _, recorded := definition.Metadata[standardConfiguredPolicyFields]; !recorded {
		return true
	}
	if standardFieldPolicyHasRecordTypes(definition) {
		if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); !exists {
			return true
		}
	}
	for _, field := range policy.Present {
		if _, exists := ResolveFieldName(definition, "", field); !exists {
			return true
		}
	}
	for _, gate := range policy.gates {
		// An unknown shape retains existing catalog fields, but does not
		// synthesize fields belonging only to an unrequested feature profile.
		if !gate.Legacy && featureSignatureHas(signature, project.UnspecifiedStandardFieldShape) && !standardFieldPolicyGateEnabledForDefinition(gate, definition, signature) {
			continue
		}
		_, exists := ResolveFieldName(definition, "", gate.Field.Name)
		enabled := standardFieldPolicyGateEnabledForDefinition(gate, definition, signature) || standardFieldPolicyPreservesConfiguredField(definition, gate.Field.Name)
		if exists != enabled {
			return true
		}
	}
	for name := range definition.Fields {
		if !standardOptionalFieldEnabledForDefinition(definition, name, signature) {
			return true
		}
	}
	for _, relation := range definition.Relations {
		if !standardOptionalFieldEnabledForDefinition(definition, relation.Field, signature) {
			return true
		}
	}
	return false
}

func removeDisabledStandardOptionalFields(definition *ObjectDefinition, signature string) {
	applyStandardFieldPolicy(definition, signature)
	var relations []Relationship
	for i, relation := range definition.Relations {
		if !standardOptionalFieldEnabledForDefinition(*definition, relation.Field, signature) {
			if relations == nil {
				relations = append(make([]Relationship, 0, len(definition.Relations)), definition.Relations[:i]...)
			}
		} else if relations != nil {
			relations = append(relations, relation)
		}
	}
	if relations != nil {
		definition.Relations = relations
	}
}

// StandardFeatureObjectNames lists standard objects whose field overlays can
// change under this profile. Semantic analysis uses the same field policy even
// when the project contains no metadata for those objects.
func StandardFeatureObjectNames(features []string) []string {
	names := map[string]bool{}
	for name, entry := range standardObjectCatalogData {
		for feature := range entry.FeatureFields {
			if hasCanonicalFeature(features, feature) {
				names[name] = true
			}
		}
	}
	signature := canonicalFeatureSignature(features)
	for _, policy := range standardFieldPolicies {
		for _, gate := range policy.gates {
			if standardFieldPolicyGateEnabled(gate, signature) {
				names[gate.Object] = true
			}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// StandardObjectFieldRequiresFeature reports whether a standard field exists
// only when an org feature is enabled. Such a field is absent from a clean
// org's describe, so its absence is not a catalog gap.
func StandardObjectFieldRequiresFeature(objectName, fieldName string) bool {
	if policy := standardFieldPolicyFor(objectName); policy != nil {
		if _, gated := standardPolicyLookup(policy.gates, fieldName); gated {
			return true
		}
	}
	if canonical, known := ResolveKnownStandardObjectName(objectName); known {
		objectName = canonical
	}
	if entry, ok := standardObjectCatalogEntryFor(objectName); ok {
		for _, fields := range entry.FeatureFields {
			for key, field := range fields {
				if stringsEqualFold(key, fieldName) || stringsEqualFold(field.APIName, fieldName) {
					return true
				}
			}
		}
	}
	return false
}

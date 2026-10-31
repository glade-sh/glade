package storage

import (
	"sort"
	"strings"

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
	gatePresence, indexed := optionalGatePresence(definition, policy)
	for _, gate := range policy.gates {
		// An unknown shape retains existing catalog fields, but does not
		// synthesize fields belonging only to an unrequested feature profile.
		if !gate.Legacy && featureSignatureHas(signature, project.UnspecifiedStandardFieldShape) && !standardFieldPolicyGateEnabledForDefinition(gate, definition, signature) {
			continue
		}
		exists := gatePresence.has(gate.Field.Name)
		if !indexed {
			_, exists = ResolveFieldName(definition, "", gate.Field.Name)
		}
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

// optionalGatePresence builds only a per-call index, never a cached verdict.
// Simple ASCII names have the same existence result under ResolveFieldName as
// an ASCII case-folded exact key. Special names and non-ASCII keys fall back
// to the full resolver, including its namespace and location-component rules.
type optionalGateIndex struct {
	names   [64]string
	present uint64
	count   int
}

func (index *optionalGateIndex) has(name string) bool {
	for i := 0; i < index.count; i++ {
		if index.names[i] == name {
			return index.present&(uint64(1)<<i) != 0
		}
	}
	return false
}

func optionalGatePresence(definition ObjectDefinition, policy *standardFieldPolicyObject) (optionalGateIndex, bool) {
	var index optionalGateIndex
	if len(policy.gates) < 8 || len(policy.gates) > len(index.names) {
		return index, false
	}
	for _, gate := range policy.gates {
		name := gate.Field.Name
		if name == "" || strings.Contains(name, "__") || strings.EqualFold(name, "Id") || !relationshipNameASCII(name) {
			return index, false
		}
		index.names[index.count] = name
		index.count++
	}
	for name := range definition.Fields {
		if !relationshipNameASCII(name) {
			return index, false
		}
		if gate, ok := standardPolicyLookup(policy.gates, name); ok {
			for i := 0; i < index.count; i++ {
				if index.names[i] == gate.Field.Name {
					index.present |= uint64(1) << i
					break
				}
			}
		}
	}
	return index, true
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

package storage

import "sort"

var standardOptionalFieldFeatures = []struct{ object, field, feature string }{
	{"Account", "IsActive", "HealthCloud"},
	{"Contact", "IsPersonAccount", "PersonAccounts"},
	{"User", "DefaultCurrencyIsoCode", "MultiCurrency"},
	{"Opportunity", "SyncedQuoteId", "Quotes"},
}

func standardOptionalFieldEnabled(object, field, signature string) bool {
	for _, gate := range standardOptionalFieldFeatures {
		if stringsEqualFold(object, gate.object) && stringsEqualFold(field, gate.field) {
			return featureSignatureHas(signature, gate.feature)
		}
	}
	return true
}

func standardOptionalFieldsNeedRepair(definition ObjectDefinition, signature string) bool {
	matched := false
	for _, gate := range standardOptionalFieldFeatures {
		if !stringsEqualFold(definition.APIName, gate.object) {
			continue
		}
		matched = true
		_, exists := ResolveFieldName(definition, "", gate.field)
		if exists != featureSignatureHas(signature, gate.feature) {
			return true
		}
	}
	if !matched {
		return false
	}
	for _, relation := range definition.Relations {
		if !standardOptionalFieldEnabled(definition.APIName, relation.Field, signature) {
			return true
		}
	}
	return false
}

func removeDisabledStandardOptionalFields(definition *ObjectDefinition, signature string) {
	for _, gate := range standardOptionalFieldFeatures {
		if stringsEqualFold(definition.APIName, gate.object) && !featureSignatureHas(signature, gate.feature) {
			if name, exists := ResolveFieldName(*definition, "", gate.field); exists {
				delete(definition.Fields, name)
			}
		}
	}
	var relations []Relationship
	for i, relation := range definition.Relations {
		if !standardOptionalFieldEnabled(definition.APIName, relation.Field, signature) {
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
	for _, gate := range standardOptionalFieldFeatures {
		if hasCanonicalFeature(features, gate.feature) {
			names[gate.object] = true
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
	for _, gate := range standardOptionalFieldFeatures {
		if stringsEqualFold(objectName, gate.object) && stringsEqualFold(fieldName, gate.field) {
			return true
		}
	}
	return isStateAndCountryPicklistField(fieldName)
}

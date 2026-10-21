package storage

// The unmemoized standard overlay write check, with the shared field
// availability policy. It is the oracle for TestStandardOverlayCheckPlan*.

import "strings"

func legacyStandardFieldsOverlayNeedsRefresh(definition ObjectDefinition, featureSignature string) bool {
	if accountTeamReferencesNeedRepair(definition) || organizationFeatureFlagsNeedRepair(definition) || standardOptionalFieldsNeedRepair(definition, featureSignature) {
		return true
	}
	if name := standardGeneratedNumberField(definition.APIName); name != "" {
		if field, exists := definition.Fields[name]; exists && !field.AutoNumber {
			return true
		}
	}
	if stringsEqualFold(definition.APIName, "Account") {
		if _, exists := ResolveFieldName(definition, "", "DunsNumber"); !exists {
			return true
		}
	}
	if legacyStandardStubTimeFieldsNeedRepair(definition) {
		return true
	}
	if legacyStandardStubReferenceFieldsNeedRepair(definition, strings.Split(featureSignature, ",")) {
		return true
	}
	entry, ok := standardObjectCatalogEntryForName(definition.APIName)
	if !ok {
		return false
	}
	shallow := func(fields map[string]Field) bool {
		for name, standard := range fields {
			if standard.Type == "" || standard.Type == FieldAny || strings.EqualFold(strings.TrimSpace(standard.DisplayType), "ANY") {
				continue
			}
			existingName, exists := ResolveFieldName(definition, "", name)
			if !exists {
				continue
			}
			existing := definition.Fields[existingName]
			// Missing display metadata only needs a refresh when the overlay
			// has metadata to supply; otherwise enrichment is already complete.
			if existing.Type == "" || existing.Type == FieldAny ||
				(strings.TrimSpace(existing.DisplayType) == "" && strings.TrimSpace(standard.DisplayType) != "") {
				return true
			}
		}
		return false
	}
	missing := func(fields map[string]Field) bool {
		for name, standard := range fields {
			if len(standard.PicklistValues) == 0 {
				continue
			}
			existingName, exists := ResolveFieldName(definition, "", name)
			if !exists || len(definition.Fields[existingName].PicklistValues) != 0 {
				continue
			}
			return true
		}
		return false
	}
	if shallow(entry.Definition.Fields) || legacyShallowFieldsSlice(standardFieldsForObject(definition.APIName), shallow) {
		return true
	}
	if missing(entry.Definition.Fields) || legacyMissingFieldsSlice(standardFieldsForObject(definition.APIName), missing) {
		return true
	}
	for _, feature := range strings.Split(featureSignature, ",") {
		if feature != "" && missing(entry.FeatureFields[feature]) {
			return true
		}
	}
	return false
}

func legacyStandardStubReferenceFieldsNeedRepair(definition ObjectDefinition, features []string) bool {
	if _, hasStub := standardSObjectStubFieldsFor(definition.APIName); !hasStub {
		return false
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return false
	}
	for _, captured := range describe.Fields {
		if !strings.EqualFold(captured.Type, "reference") || (captured.DeprecatedAndHidden != nil && *captured.DeprecatedAndHidden) {
			continue
		}
		if !standardStubReferenceTargetsAvailable(captured.ReferenceTo) {
			continue
		}
		if !standardStubReferenceFieldEnabled(definition.APIName, captured.Name, features) {
			continue
		}
		name, exists := ResolveFieldName(definition, "", captured.Name)
		if !exists {
			return true
		}
		field := definition.Fields[name]
		if field.Type == FieldLocation || field.Type == "" || field.Type == FieldAny {
			return true
		}
		if field.Type != FieldReference {
			continue
		}
		for _, target := range captured.ReferenceTo {
			found := false
			for _, existingTarget := range field.ReferenceTo {
				if strings.EqualFold(existingTarget, target) {
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
		if captured.RelationshipName != "" && field.RelationshipName == "" {
			return true
		}
	}
	return false
}

func legacyShallowFieldsSlice(fields []Field, shallow func(map[string]Field) bool) bool {
	if len(fields) == 0 {
		return false
	}
	byName := make(map[string]Field, len(fields))
	for _, field := range fields {
		if field.APIName != "" {
			byName[field.APIName] = field
		}
	}
	return shallow(byName)
}

func legacyMissingFieldsSlice(fields []Field, missing func(map[string]Field) bool) bool {
	if len(fields) == 0 {
		return false
	}
	byName := make(map[string]Field, len(fields))
	for _, field := range fields {
		if field.APIName != "" {
			byName[field.APIName] = field
		}
	}
	return missing(byName)
}

func legacyStandardStubTimeFieldsNeedRepair(definition ObjectDefinition) bool {
	possible := false
	for _, field := range definition.Fields {
		if standardStubTimeReference(field) {
			possible = true
			break
		}
	}
	if !possible {
		return false
	}
	if _, ok := standardSObjectStubFieldsFor(definition.APIName); !ok {
		return false
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return false
	}
	for _, captured := range describe.Fields {
		if !strings.EqualFold(captured.Type, "time") {
			continue
		}
		name, exists := ResolveFieldName(definition, "", captured.Name)
		if exists && standardStubTimeReference(definition.Fields[name]) {
			return true
		}
	}
	return false
}

func legacyStandardReadOnlyFlagsNeedRepair(definition *ObjectDefinition) bool {
	if definition == nil || len(definition.Fields) == 0 {
		return false
	}
	readOnlyFields, ok := standardSObjectStubReadOnlyFieldsFor(definition.APIName)
	if !ok {
		return false
	}
	for _, name := range readOnlyFields {
		field, ok := definition.Fields[name]
		if ok && (field.Createable == nil || field.Updateable == nil) {
			return true
		}
	}
	return false
}

func legacyStandardObjectFieldsNeedWrite(definition ObjectDefinition, featureSignature string) bool {
	if masterDetailOwnerFieldsNeedRepair(definition) {
		return true
	}
	if _, ok := standardObjectCatalogEntryForName(definition.APIName); ok && !definition.EnableSearch {
		return true
	}
	if !standardFieldsOverlayApplied(definition, featureSignature) {
		return true
	}
	if legacyStandardFieldsOverlayNeedsRefresh(definition, featureSignature) {
		return true
	}
	return legacyStandardReadOnlyFlagsNeedRepair(&definition)
}

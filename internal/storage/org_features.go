package storage

import "strings"

// HasFeature reads the resolved org profile without exposing implementation
// flags as fields on the Salesforce Organization object.
func (o *OrgState) HasFeature(feature string) bool {
	if o == nil {
		return false
	}
	o.ensureFeatureProfile()
	return hasCanonicalFeature(o.Features, canonicalFeatureName(feature))
}

func (o *OrgState) setFeatureProfile(features []string) {
	signature := canonicalFeatureSignature(features)
	o.Features = []string{}
	if signature != "" {
		o.Features = strings.Split(signature, ",")
	}
}

func (o *OrgState) enableFeature(feature string) {
	if !o.HasFeature(feature) {
		o.Features = append(append([]string(nil), o.Features...), canonicalFeatureName(feature))
	}
}

var organizationFeatureFlagFields = []string{
	"IsMultiCurrencyEnabled", "IsChatterEnabled", "IsLightningExperienceEnabled",
	"IsSetPasswordInApiEnabled", "IsHealthCloudEnabled", "HasAnalyticsAdminPerms",
}

// Restore profiles written before org capabilities had their own storage field.
// Legacy overlay markers and synthetic flags are internal persistence formats.
func (o *OrgState) ensureFeatureProfile() {
	if o.Features == nil {
		o.restoreLegacyFeatureProfile()
	}
}

func (o *OrgState) restoreLegacyFeatureProfile() {
	features := append([]string(nil), o.Features...)
	for _, object := range o.Objects {
		features = append(features, strings.Split(object.Definition.Metadata[standardFieldsOverlayMarker], ",")...)
	}
	legacyFeatures := []string{"MultiCurrency", "Chatter", "LightningExperience", "EnableSetPasswordInApi", "HealthCloud", "AnalyticsAdminPerms"}
	if name, exists := ResolveObjectName(*o, "Organization"); exists {
		object := o.Objects[name]
		for _, record := range object.Records {
			for i, field := range organizationFeatureFlagFields {
				if value, ok := record.GetField(field); ok && value.Kind == ValueBoolean && value.Boolean {
					features = append(features, legacyFeatures[i])
				}
			}
		}
		if organizationFeatureFlagsNeedRepair(object.Definition) {
			definition, _ := EnsureMutableObjectDefinition(o, name)
			removeOrganizationFeatureFlagFields(definition)
		}
	}
	o.setFeatureProfile(features)
	clearOrganizationFeatureFlagRecords(o)
}

func organizationFeatureFlagsNeedRepair(definition ObjectDefinition) bool {
	if !stringsEqualFold(definition.APIName, "Organization") {
		return false
	}
	for _, field := range organizationFeatureFlagFields {
		if _, exists := ResolveFieldName(definition, "", field); exists {
			return true
		}
	}
	return false
}

func removeOrganizationFeatureFlagFields(definition *ObjectDefinition) {
	if !stringsEqualFold(definition.APIName, "Organization") {
		return
	}
	for _, field := range organizationFeatureFlagFields {
		if name, exists := ResolveFieldName(*definition, "", field); exists {
			delete(definition.Fields, name)
		}
	}
}

func clearOrganizationFeatureFlagRecords(org *OrgState) {
	object, _ := EnsureMutableObjectRecords(org, "Organization")
	if object == nil {
		return
	}
	for id, record := range object.Records {
		record = record.Clone()
		for field := range record.Fields {
			for _, flag := range organizationFeatureFlagFields {
				if stringsEqualFold(field, flag) {
					delete(record.Fields, field)
					break
				}
			}
		}
		object.Records[id] = record
	}
}

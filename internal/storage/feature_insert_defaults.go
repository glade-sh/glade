package storage

// featureInsertDefault records a standard picklist that Salesforce fills on
// insert when the org carries a feature set. Only isolated native captures
// belong here (docs/native-evidence/standard_recordtype_picklist_default).
type featureInsertDefault struct {
	object, field string
	features      []string
	value         string
}

var featureInsertDefaults = []featureInsertDefault{
	// UnitOfMeasure.Type is required and has no values in a plain Developer
	// org. HealthCloudAddOn alone (Enterprise) gives it the single value
	// Custom, defaulted on create; PublicSectorAccess alone gives it Period
	// (default) and Not Applicable. Both at API 62.0 and 67.0.
	{"UnitOfMeasure", "Type", []string{"HealthCloud"}, "Custom"},
	{"UnitOfMeasure", "Type", []string{"PublicSectorAccess"}, "Period"},
}

// FeatureInsertDefault returns the captured insert default for a standard
// field under the org's features. A field whose feature sets both match, or
// that no capture covers, has none (the combination is not captured).
func FeatureInsertDefault(org *OrgState, object, field string) (Value, bool) {
	// Insert defaults run for every field of every inserted record; most
	// objects have no entry, so reject them before any feature lookup.
	if org == nil || len(object) != len("UnitOfMeasure") || !stringsEqualFold(object, "UnitOfMeasure") {
		return Value{}, false
	}
	var found *featureInsertDefault
	for i := range featureInsertDefaults {
		entry := &featureInsertDefaults[i]
		if !stringsEqualFold(entry.object, object) || !stringsEqualFold(entry.field, field) {
			continue
		}
		enabled := true
		for _, feature := range entry.features {
			if !org.HasFeature(feature) {
				enabled = false
				break
			}
		}
		if !enabled {
			continue
		}
		if found != nil {
			return Value{}, false
		}
		found = entry
	}
	if found == nil {
		return Value{}, false
	}
	return StringValue(found.value), true
}

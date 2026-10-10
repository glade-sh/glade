package storage

import (
	"strings"
	"sync"
	"sync/atomic"
)

// standardOverlayCheckPlan memoizes the catalog side of the standard overlay
// write check for one object API name. Every input it holds comes from the
// embedded standard catalogs, so the plan never goes stale while those caches
// stay installed. The definition side is still read on every call: field maps
// are written in place across the runtime, so a verdict memo could not see
// those writes.
type standardOverlayCheckPlan struct {
	objectName           string
	catalogOK            bool
	isAccount            bool
	generatedNumberField string
	// hasStubFields marks objects with a generated stub overlay. Their
	// catalog time fields, which a stale stub overlay may still model as
	// references to Time, decode lazily on the first definition that holds
	// a Time reference, as the unmemoized check did.
	hasStubFields  bool
	stubTimeOnce   sync.Once
	stubTimeFields []string
	// shallowFields are catalog fields with a concrete type. A definition
	// field that lacks a type, or lacks display metadata the catalog has,
	// needs a refresh.
	shallowFields []standardOverlayShallowField
	// picklistFields are catalog fields with picklist values.
	picklistFields        []string
	featurePicklistFields map[string][]string
	readOnlyFields        []string
	readOnlyFieldsOK      bool
}

type standardOverlayShallowField struct {
	name       string
	hasDisplay bool
}

type standardOverlayCheckPlanSet struct {
	catalog *standardDescribeCatalogV2Cache
	raw     *standardDescribeCatalogV2RawCache
	plans   sync.Map
	builds  atomic.Uint64
}

var standardOverlayCheckPlans atomic.Pointer[standardOverlayCheckPlanSet]

// currentStandardOverlayCheckPlans returns the plan set for the installed
// catalog caches. Installing a different describe cache starts a new set.
func currentStandardOverlayCheckPlans() *standardOverlayCheckPlanSet {
	catalog, raw := standardDescribeCatalogV2ProductionCache, standardDescribeCatalogV2RawProductionCache
	for {
		current := standardOverlayCheckPlans.Load()
		if current != nil && current.catalog == catalog && current.raw == raw {
			return current
		}
		next := &standardOverlayCheckPlanSet{catalog: catalog, raw: raw}
		if standardOverlayCheckPlans.CompareAndSwap(current, next) {
			return next
		}
	}
}

func standardOverlayCheckPlanFor(objectName string) *standardOverlayCheckPlan {
	set := currentStandardOverlayCheckPlans()
	if plan, ok := set.plans.Load(objectName); ok {
		return plan.(*standardOverlayCheckPlan)
	}
	plan, _ := set.plans.LoadOrStore(objectName, buildStandardOverlayCheckPlan(objectName))
	set.builds.Add(1)
	return plan.(*standardOverlayCheckPlan)
}

func buildStandardOverlayCheckPlan(objectName string) *standardOverlayCheckPlan {
	plan := &standardOverlayCheckPlan{
		objectName:           objectName,
		isAccount:            stringsEqualFold(objectName, "Account"),
		generatedNumberField: standardGeneratedNumberField(objectName),
	}
	_, plan.hasStubFields = standardSObjectStubFieldsFor(objectName)
	plan.readOnlyFields, plan.readOnlyFieldsOK = standardSObjectStubReadOnlyFieldsFor(objectName)
	entry, ok := standardObjectCatalogEntryForName(objectName)
	if !ok {
		return plan
	}
	plan.catalogOK = true
	addShallow := func(fields map[string]Field) {
		for name, standard := range fields {
			if standard.Type == "" || standard.Type == FieldAny || strings.EqualFold(strings.TrimSpace(standard.DisplayType), "ANY") {
				continue
			}
			plan.shallowFields = append(plan.shallowFields, standardOverlayShallowField{
				name:       name,
				hasDisplay: strings.TrimSpace(standard.DisplayType) != "",
			})
		}
	}
	picklists := func(fields map[string]Field) []string {
		var out []string
		for name, standard := range fields {
			if len(standard.PicklistValues) != 0 {
				out = append(out, name)
			}
		}
		return out
	}
	extra := standardFieldsByName(standardFieldsForObject(objectName))
	addShallow(entry.Definition.Fields)
	addShallow(extra)
	plan.picklistFields = append(picklists(entry.Definition.Fields), picklists(extra)...)
	for feature, fields := range entry.FeatureFields {
		if names := picklists(fields); len(names) > 0 {
			if plan.featurePicklistFields == nil {
				plan.featurePicklistFields = make(map[string][]string)
			}
			plan.featurePicklistFields[feature] = names
		}
	}
	return plan
}

// standardFieldsByName keys a field slice by API name; a later duplicate
// replaces an earlier one.
func standardFieldsByName(fields []Field) map[string]Field {
	if len(fields) == 0 {
		return nil
	}
	byName := make(map[string]Field, len(fields))
	for _, field := range fields {
		if field.APIName != "" {
			byName[field.APIName] = field
		}
	}
	return byName
}

// standardStubTimeCatalogFields lists the describe catalog's time fields for
// an object that carries a generated stub overlay.
func standardStubTimeCatalogFields(objectName string) []string {
	describe, ok, err := lookupStandardDescribeCatalogV2(objectName)
	if err != nil || !ok {
		return nil
	}
	var out []string
	for _, captured := range describe.Fields {
		if strings.EqualFold(captured.Type, "time") {
			out = append(out, captured.Name)
		}
	}
	return out
}

func (plan *standardOverlayCheckPlan) needsRefresh(definition ObjectDefinition, featureSignature string) bool {
	if name := plan.generatedNumberField; name != "" {
		if field, exists := definition.Fields[name]; exists && !field.AutoNumber {
			return true
		}
	}
	if plan.isAccount {
		if _, exists := ResolveFieldName(definition, "", "DunsNumber"); !exists {
			return true
		}
	}
	if plan.stubTimeFieldsNeedRepair(definition) {
		return true
	}
	if !plan.catalogOK {
		return false
	}
	for _, standard := range plan.shallowFields {
		existingName, exists := ResolveFieldName(definition, "", standard.name)
		if !exists {
			continue
		}
		existing := definition.Fields[existingName]
		// Missing display metadata only needs a refresh when the overlay
		// has metadata to supply; otherwise enrichment is already complete.
		if existing.Type == "" || existing.Type == FieldAny ||
			(standard.hasDisplay && strings.TrimSpace(existing.DisplayType) == "") {
			return true
		}
	}
	if standardOverlayPicklistsMissing(definition, plan.picklistFields) {
		return true
	}
	if len(plan.featurePicklistFields) == 0 {
		return false
	}
	for featureSignature != "" {
		feature, rest, _ := strings.Cut(featureSignature, ",")
		featureSignature = rest
		if feature != "" && standardOverlayPicklistsMissing(definition, plan.featurePicklistFields[feature]) {
			return true
		}
	}
	return false
}

func (plan *standardOverlayCheckPlan) stubTimeFieldsNeedRepair(definition ObjectDefinition) bool {
	if !plan.hasStubFields {
		return false
	}
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
	plan.stubTimeOnce.Do(func() {
		plan.stubTimeFields = standardStubTimeCatalogFields(plan.objectName)
	})
	for _, name := range plan.stubTimeFields {
		existing, exists := ResolveFieldName(definition, "", name)
		if exists && standardStubTimeReference(definition.Fields[existing]) {
			return true
		}
	}
	return false
}

func standardOverlayPicklistsMissing(definition ObjectDefinition, names []string) bool {
	for _, name := range names {
		existingName, exists := ResolveFieldName(definition, "", name)
		if exists && !definition.Fields[existingName].PicklistValuesConfigured && len(definition.Fields[existingName].PicklistValues) == 0 {
			return true
		}
	}
	return false
}

func (plan *standardOverlayCheckPlan) readOnlyFlagsNeedRepair(definition *ObjectDefinition) bool {
	if definition == nil || len(definition.Fields) == 0 || !plan.readOnlyFieldsOK {
		return false
	}
	for _, name := range plan.readOnlyFields {
		field, ok := definition.Fields[name]
		if ok && (field.Createable == nil || field.Updateable == nil) {
			return true
		}
	}
	return false
}

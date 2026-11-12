package storage

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/project"
)

// The sidecar contains only captured field deltas and explicit legacy gates.
// Absence is limited to the recaptured historical candidates, rather than
// treating a partial REST describe as a complete standard schema.
//
//go:embed standard_field_policy.json
var standardFieldPolicyData []byte

// standardFieldSurfaceData extends the sidecar with closed constructor
// surfaces captured after it was built (Case, User) and with captured fields
// that the sidecar's surface lacks (WebStore.SupportedCurrencies). It is a
// separate file so a regenerated sidecar keeps them.
//
//go:embed standard_field_surfaces.json
var standardFieldSurfaceData []byte

type standardFieldPolicyGate struct {
	RecordTypes      bool `json:"record_types"`
	OverrideStubType bool `json:"override_stub_type"`
	Object           string
	Features         []string
	Field            standardDescribeField
	Legacy           bool
	// Alternatives are further feature sets that each enable the field alone,
	// from isolated captures added after the gate was built.
	Alternatives [][]string `json:"-"`
}

type standardFieldPolicyObject struct {
	ConstructorFields     []string   `json:"constructor_fields"`
	InputFields           []string   `json:"input_fields"`
	RequiredFeatures      []string   `json:"required_features"`
	RequiredFeaturesAnyOf [][]string `json:"required_features_any_of"`
	Present               []string
	// CapturedFields name policy fields whose native facets replace a field
	// that the stub overlay supplies with only a name and a type. A field that
	// the project itself configures is never replaced.
	CapturedFields           []string        `json:"captured_fields"`
	InsertForcedValues       map[string]bool `json:"insert_forced_values"`
	Fields                   []standardDescribeField
	Absent                   []string
	PlainDeveloperAbsent     []string          `json:"plain_developer_absent"`
	ConstructorDefaults      map[string]string `json:"constructor_defaults"`
	ConstructorHiddenRemoved []string          `json:"constructor_hidden_removed"`
	fields                   map[string]Field
	picklistFields           []Field
	absent                   map[string]bool
	// unspecifiedShapeFields are accepted only when the project declares no
	// scratch definition (an unknown org shape). featureEnabledFields lists
	// generated catalog fields that stay absent unless one feature group is on.
	unspecifiedShapeFields map[string]bool
	constructorFieldSet    map[string]bool
	featureEnabledFields   map[string][][]string
	plainDeveloperAbsent   map[string]bool
	constructorDefaults    map[string]string
	gates                  map[string]standardFieldPolicyGate
}

type standardFieldPolicyBundle struct {
	Version  int
	Objects  map[string]*standardFieldPolicyObject
	Gates    []standardFieldPolicyGate
	Sources  map[string]string
	Evidence struct {
		Profiles       map[string]map[string]StandardDescribeCatalogDelta `json:"profiles"`
		ProfileAliases map[string]map[string]string                       `json:"profile_aliases"`
	} `json:"evidence"`
}

var standardFieldPolicyCache struct {
	once     sync.Once
	policies map[string]*standardFieldPolicyObject
}

// standardFieldPolicies decodes the embedded policy sidecar on first use. The
// decode costs about a third of package init when it runs eagerly, which every
// process pays, including ones that never touch a standard object.
func standardFieldPolicies() map[string]*standardFieldPolicyObject {
	standardFieldPolicyCache.once.Do(func() {
		standardFieldPolicyCache.policies = loadStandardFieldPolicies()
	})
	return standardFieldPolicyCache.policies
}

func loadStandardFieldPolicies() map[string]*standardFieldPolicyObject {
	var bundle standardFieldPolicyBundle
	if err := json.Unmarshal(standardFieldPolicyData, &bundle); err != nil {
		panic(err)
	}
	if bundle.Version != 1 {
		panic("unsupported standard field policy version")
	}
	standardDescribeProfileCatalog = bundle.Evidence.Profiles
	standardDescribeProfileAliases = bundle.Evidence.ProfileAliases
	policies := make(map[string]*standardFieldPolicyObject, len(bundle.Objects))
	for object, policy := range bundle.Objects {
		policy.fields = map[string]Field{}
		policy.absent = map[string]bool{}
		policy.plainDeveloperAbsent = map[string]bool{}
		policy.constructorDefaults = map[string]string{}
		policy.gates = map[string]standardFieldPolicyGate{}
		for _, field := range policy.Fields {
			captured := describeField(field)
			policy.fields[strings.ToLower(field.Name)] = captured
			if len(captured.PicklistValues) != 0 {
				policy.picklistFields = append(policy.picklistFields, captured)
			}
		}
		for _, field := range policy.Absent {
			policy.absent[strings.ToLower(field)] = true
		}
		for _, field := range policy.PlainDeveloperAbsent {
			policy.plainDeveloperAbsent[strings.ToLower(field)] = true
		}
		for field, value := range policy.ConstructorDefaults {
			policy.constructorDefaults[strings.ToLower(field)] = value
		}
		policies[strings.ToLower(object)] = policy
	}
	for _, gate := range bundle.Gates {
		key := strings.ToLower(gate.Object)
		policy := policies[key]
		if policy == nil {
			policy = &standardFieldPolicyObject{fields: map[string]Field{}, absent: map[string]bool{}, gates: map[string]standardFieldPolicyGate{}}
			policies[key] = policy
		}
		policy.gates[strings.ToLower(gate.Field.Name)] = gate
	}
	applyStandardFieldSurfaces(policies)
	return policies
}

func applyStandardFieldSurfaces(policies map[string]*standardFieldPolicyObject) {
	var bundle struct {
		Version int
		Objects map[string]struct {
			ConstructorFields    []string              `json:"constructor_fields"`
			AddConstructorFields []string              `json:"add_constructor_fields"`
			GateAlternatives     map[string][][]string `json:"gate_alternatives"`
			UnspecifiedShape     []string              `json:"unspecified_shape_fields"`
			FeatureEnabled       map[string][][]string `json:"feature_enabled_fields"`
		}
	}
	if err := json.Unmarshal(standardFieldSurfaceData, &bundle); err != nil {
		panic(err)
	}
	if bundle.Version != 1 {
		panic("unsupported standard field surface version")
	}
	for object, surface := range bundle.Objects {
		key := strings.ToLower(object)
		policy := policies[key]
		if policy == nil {
			policy = &standardFieldPolicyObject{fields: map[string]Field{}, absent: map[string]bool{}, gates: map[string]standardFieldPolicyGate{}}
			policies[key] = policy
		}
		if len(surface.ConstructorFields) > 0 && len(policy.ConstructorFields) == 0 {
			policy.ConstructorFields = append([]string(nil), surface.ConstructorFields...)
		}
		if len(surface.UnspecifiedShape) > 0 && policy.unspecifiedShapeFields == nil {
			policy.unspecifiedShapeFields = map[string]bool{}
		}
		for _, field := range surface.UnspecifiedShape {
			policy.unspecifiedShapeFields[strings.ToLower(field)] = true
		}
		if len(surface.FeatureEnabled) > 0 && policy.featureEnabledFields == nil {
			policy.featureEnabledFields = map[string][][]string{}
		}
		for field, groups := range surface.FeatureEnabled {
			policy.featureEnabledFields[strings.ToLower(field)] = groups
		}
		for field, groups := range surface.GateAlternatives {
			if gate, ok := policy.gates[strings.ToLower(field)]; ok {
				gate.Alternatives = groups
				policy.gates[strings.ToLower(field)] = gate
			}
		}
		for _, name := range surface.AddConstructorFields {
			if len(policy.ConstructorFields) == 0 {
				continue
			}
			present := false
			for _, existing := range policy.ConstructorFields {
				if stringsEqualFold(existing, name) {
					present = true
					break
				}
			}
			if !present {
				policy.ConstructorFields = append(policy.ConstructorFields, name)
			}
		}
	}
}

func standardFieldPolicyFor(object string) *standardFieldPolicyObject {
	policy, _ := standardPolicyLookup(standardFieldPolicies(), object)
	return policy
}

// CapturedStandardConstructorDefault returns a native constructor-only value
// for the declared plain Developer shape without changing describe defaults.
func CapturedStandardConstructorDefault(definition ObjectDefinition, field Field) (Value, bool) {
	if !featureSignatureHas(definition.Metadata[standardFieldsOverlayMarker], project.PlainDeveloperObjectShape) || strings.TrimSpace(field.DefaultValue) != "" {
		return Value{}, false
	}
	policy := standardFieldPolicyFor(definition.APIName)
	if policy == nil {
		return Value{}, false
	}
	raw, ok := standardPolicyLookup(policy.constructorDefaults, field.APIName)
	if !ok {
		return Value{}, false
	}
	field.DefaultValue = raw
	return DefaultValueForField(field)
}

// CapturedStandardConstructorHiddenRemoved returns native hidden field names
// reported by stripping an original loaded record in a plain Developer org.
func CapturedStandardConstructorHiddenRemoved(definition ObjectDefinition) []string {
	if !featureSignatureHas(definition.Metadata[standardFieldsOverlayMarker], project.PlainDeveloperObjectShape) {
		return nil
	}
	if policy := standardFieldPolicyFor(definition.APIName); policy != nil {
		return policy.ConstructorHiddenRemoved
	}
	return nil
}

func standardFieldPolicyGateEnabled(gate standardFieldPolicyGate, signature string) bool {
	if standardFieldPolicyFeaturesPresent(gate.Features, signature) {
		return true
	}
	for _, group := range gate.Alternatives {
		if standardFieldPolicyFeaturesPresent(group, signature) {
			return true
		}
	}
	return false
}

func standardFieldPolicyFeaturesPresent(features []string, signature string) bool {
	for _, feature := range features {
		if !featureSignatureHas(signature, feature) {
			return false
		}
	}
	return true
}

func standardFieldPolicyHasRecordTypes(definition ObjectDefinition) bool {
	for _, recordType := range definition.RecordTypes {
		if recordType.DeveloperName != "" && !stringsEqualFold(recordType.DeveloperName, "Master") {
			return true
		}
	}
	return false
}

func standardFieldPolicyGateEnabledForDefinition(gate standardFieldPolicyGate, definition ObjectDefinition, signature string) bool {
	return (gate.RecordTypes && standardFieldPolicyHasRecordTypes(definition)) || standardFieldPolicyGateEnabled(gate, signature)
}

func standardOptionalFieldEnabledForDefinition(definition ObjectDefinition, field, signature string) bool {
	// Baseline absence describes objects with no non-Master record types. A
	// declared record type supplies its lookup independently of feature gates.
	if stringsEqualFold(field, "RecordTypeId") && standardFieldPolicyHasRecordTypes(definition) {
		return true
	}
	if policy := standardFieldPolicyFor(definition.APIName); policy != nil {
		if absent, _ := standardPolicyLookup(policy.plainDeveloperAbsent, field); absent && featureSignatureHas(signature, project.PlainDeveloperObjectShape) {
			return standardFieldPolicyPreservesConfiguredField(definition, field)
		}
		if gate, ok := standardPolicyLookup(policy.gates, field); ok {
			return (!gate.Legacy && featureSignatureHas(signature, project.UnspecifiedStandardFieldShape)) || standardFieldPolicyPreservesConfiguredField(definition, field) || standardFieldPolicyGateEnabledForDefinition(gate, definition, signature)
		}
		if absent, _ := standardPolicyLookup(policy.absent, field); absent {
			if groups, ok := standardPolicyLookup(policy.featureEnabledFields, field); ok {
				for _, group := range groups {
					if standardFieldPolicyFeaturesPresent(group, signature) {
						return true
					}
				}
			}
			return featureSignatureHas(signature, project.UnspecifiedStandardFieldShape) || standardFieldPolicyPreservesConfiguredField(definition, field)
		}
	}
	return true
}

// StandardFieldPolicyRemovesField reports whether the standard field policy
// already applied to definition (its overlay marker) removes field. Runtime
// helpers that merge platform fields into an existing definition skip such a
// field, so they agree with EnsureStandardObject instead of re-adding it.
func StandardFieldPolicyRemovesField(definition ObjectDefinition, field string) bool {
	signature, applied := definition.Metadata[standardFieldsOverlayMarker]
	return applied && !standardOptionalFieldEnabledForDefinition(definition, field, signature)
}

// StandardObjectRecordTypeFieldEnabled keeps describe conversions on the same
// generated-field policy as the effective storage definition.
func StandardObjectRecordTypeFieldEnabled(definition ObjectDefinition) bool {
	if standardFieldPolicyHasRecordTypes(definition) {
		return true
	}
	if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); exists {
		return true
	}
	return standardOptionalFieldEnabledForDefinition(definition, "RecordTypeId", definition.Metadata[standardFieldsOverlayMarker])
}

// Feature-generated record types leave with their profile. Project record
// types remain an independent source of RecordTypeId availability.
func removeDisabledStandardFeatureRecordTypes(definition *ObjectDefinition, features []string) {
	previous := definition.Metadata[standardFieldsOverlayMarker]
	if previous == "" || previous == canonicalFeatureSignature(features) {
		return
	}
	entry, ok := standardObjectCatalogEntryForName(definition.APIName)
	if !ok {
		return
	}
	removed := map[string]bool{}
	for feature, recordTypes := range entry.FeatureRecordTypes {
		if hasCanonicalFeature(features, feature) || !featureSignatureHas(previous, feature) {
			continue
		}
		for _, recordType := range recordTypes {
			if !stringsEqualFold(recordType.DeveloperName, "Master") {
				removed[strings.ToLower(recordType.DeveloperName)] = true
			}
		}
	}
	if len(removed) == 0 {
		return
	}
	recordTypes := make([]RecordTypeInfo, 0, len(definition.RecordTypes))
	for _, recordType := range definition.RecordTypes {
		if !removed[strings.ToLower(recordType.DeveloperName)] {
			recordTypes = append(recordTypes, recordType)
		}
	}
	definition.RecordTypes = recordTypes
}

// A gate opts into a more precise native storage type only for its captured
// field. Other catalog fields retain their established representation.
func standardPolicyGateDescribeField(gate standardFieldPolicyGate) Field {
	field := describeField(gate.Field)
	if gate.OverrideStubType && strings.EqualFold(gate.Field.Type, "multipicklist") {
		field.Type = FieldMultiPicklist
	}
	return field
}

func standardPolicyStubFieldNeedsRepair(definition ObjectDefinition, gate standardFieldPolicyGate, name string, field Field) bool {
	if !gate.OverrideStubType {
		return false
	}
	if standardFieldPolicyPreservesConfiguredField(definition, name) {
		return false
	}
	stubs, ok := standardSObjectStubFieldsFor(definition.APIName)
	if !ok {
		return false
	}
	stub, ok := stubs[gate.Field.Name]
	return ok && field.Type == stub.Type && field.DisplayType == stub.DisplayType
}

func policyCapturesField(policy *standardFieldPolicyObject, name string) bool {
	for _, captured := range policy.CapturedFields {
		if stringsEqualFold(captured, name) {
			return true
		}
	}
	return false
}

func applyStandardFieldPolicy(definition *ObjectDefinition, signature string) {
	policy := standardFieldPolicyFor(definition.APIName)
	if policy == nil {
		return
	}
	for _, field := range policy.fields {
		if name, exists := ResolveFieldName(*definition, "", field.APIName); !exists {
			definition.Fields[field.APIName] = cloneField(field)
		} else if policyCapturesField(policy, field.APIName) && !standardFieldPolicyPreservesConfiguredField(*definition, field.APIName) {
			definition.Fields[name] = cloneField(field)
		} else if len(field.PicklistValues) != 0 {
			existing := definition.Fields[name]
			if !existing.PicklistValuesConfigured && len(existing.PicklistValues) == 0 {
				existing.PicklistValues = append([]PicklistValue(nil), field.PicklistValues...)
				definition.Fields[name] = existing
			}
		}
	}
	for _, gate := range policy.gates {
		if gate.Legacy || !standardFieldPolicyGateEnabledForDefinition(gate, *definition, signature) {
			continue
		}
		field := standardPolicyGateDescribeField(gate)
		if name, exists := ResolveFieldName(*definition, "", field.APIName); exists {
			existing := definition.Fields[name]
			if standardPolicyStubFieldNeedsRepair(*definition, gate, name, existing) {
				existing.Type = field.Type
				existing.DisplayType = field.DisplayType
			}
			enrichStandardField(&existing, field)
			definition.Fields[name] = existing
		} else {
			definition.Fields[field.APIName] = field
		}
	}
	// Only the catalog's generated candidates are subject to removal. The
	// pre-overlay snapshot is authoritative for configured field provenance.
	removeGenerated := func(name string) {
		if !standardOptionalFieldEnabledForDefinition(*definition, name, signature) {
			if resolved, exists := ResolveFieldName(*definition, "", name); exists {
				delete(definition.Fields, resolved)
			}
		}
	}
	for name := range policy.absent {
		removeGenerated(name)
	}
	for name := range policy.plainDeveloperAbsent {
		removeGenerated(name)
	}
	for name := range policy.gates {
		removeGenerated(name)
	}
}

// StandardObjectPolicyGateFields returns the captured describe facets for the
// fields that the standard field policy gates behind an org feature. They
// carry the native type, so a consumer that holds a feature-independent field
// model can give an enabled gate field its real type instead of a guess from
// its name. Whether the field exists in a given org is still decided by the
// gate and the org's features. Legacy gates carry no facets and are skipped.
func StandardObjectPolicyGateFields(object string) []Field {
	policy := standardFieldPolicyFor(object)
	if policy == nil {
		return nil
	}
	var fields []Field
	for _, gate := range policy.gates {
		if gate.Legacy || gate.Field.Type == "" {
			continue
		}
		fields = append(fields, describeField(gate.Field))
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].APIName < fields[j].APIName })
	return fields
}

// StandardObjectConstructorAvailableForFeatures applies only observed object
// provisioning boundaries. Unknown objects retain their existing policy.
func StandardObjectConstructorAvailableForFeatures(object string, features []string) bool {
	if hasCanonicalFeature(features, project.UnspecifiedStandardFieldShape) {
		return true
	}
	policy := standardFieldPolicyFor(object)
	if policy == nil {
		return true
	}
	signature := canonicalFeatureSignature(features)
	for _, feature := range policy.RequiredFeatures {
		if !featureSignatureHas(signature, feature) {
			return false
		}
	}
	if len(policy.RequiredFeaturesAnyOf) > 0 {
		for _, features := range policy.RequiredFeaturesAnyOf {
			if standardFieldPolicyGateEnabled(standardFieldPolicyGate{Features: features}, signature) {
				return true
			}
		}
		return false
	}
	return true
}

// StandardObjectConstructorFieldAllowedForFeatures prevents source inference
// from inventing a disabled field. A closed constructor surface is installed
// only for an object whose full named map and constructor routes were captured.
func StandardObjectConstructorFieldAllowedForFeatures(object, field string, features []string, definition ObjectDefinition) bool {
	if _, exists := ResolveFieldName(definition, "", field); exists {
		return true
	}
	policy := standardFieldPolicyFor(object)
	if policy == nil {
		return true
	}
	signature := canonicalFeatureSignature(features)
	for _, input := range policy.InputFields {
		if stringsEqualFold(input, field) {
			return true
		}
	}
	if len(policy.ConstructorFields) == 0 {
		return standardOptionalFieldEnabled(object, field, signature)
	}
	for _, name := range policy.ConstructorFields {
		if stringsEqualFold(name, field) {
			return true
		}
	}
	// Names no captured configuration supplies stay open only while the org
	// shape is unknown (no scratch definition).
	if unspecified, _ := standardPolicyLookup(policy.unspecifiedShapeFields, field); unspecified && featureSignatureHas(signature, project.UnspecifiedStandardFieldShape) {
		return true
	}
	// An enabled gate can add a field outside the captured baseline surface.
	if gate, ok := standardPolicyLookup(policy.gates, field); ok {
		return standardFieldPolicyGateEnabledForDefinition(gate, definition, signature)
	}
	// Custom fields still require their project metadata in the semantic checker.
	return stringsHasSuffixFold(field, "__c") || stringsHasSuffixFold(field, "__r") || stringsHasSuffixFold(field, "__pc") || stringsHasSuffixFold(field, "__pr")
}

// StandardObjectUnsuppliedFieldRejected reports a standard field that no
// configured feature supplies: a name the surface file lists as open only for
// an unknown org shape (Case.Language, the Connection fields), or a field that
// a feature switches on (Case.Division) while that feature is off. Source
// inference and the query check use it so such a name is neither invented nor
// accepted, even when feature profiles made the schema object partial. An
// unspecified shape (no scratch definition) accepts both.
func StandardObjectUnsuppliedFieldRejected(object, field string, features []string) bool {
	policy := standardFieldPolicyFor(object)
	if policy == nil || strings.Contains(field, "__") {
		return false
	}
	unsupplied, _ := standardPolicyLookup(policy.unspecifiedShapeFields, field)
	groups, featureEnabled := standardPolicyLookup(policy.featureEnabledFields, field)
	if !unsupplied && !featureEnabled {
		return false
	}
	signature := canonicalFeatureSignature(features)
	if featureSignatureHas(signature, project.UnspecifiedStandardFieldShape) {
		return false
	}
	for _, group := range groups {
		if standardFieldPolicyFeaturesPresent(group, signature) {
			return false
		}
	}
	return true
}

// Policy keys are ASCII API names. Keep the case-insensitive lookup on the
// stack because overlay checks visit every field on every steady-state read.
func standardPolicyLookup[V any](entries map[string]V, name string) (V, bool) {
	var key [128]byte
	if len(name) > len(key) {
		value, ok := entries[strings.ToLower(name)]
		return value, ok
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 128 {
			value, ok := entries[strings.ToLower(name)]
			return value, ok
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		key[i] = c
	}
	value, ok := entries[string(key[:len(name)])]
	return value, ok
}

const standardConfiguredPolicyFields = "__glade_standard_configured_policy_fields"
const standardConfiguredPolicyFieldsLabelProvenance = "__glade_standard_configured_policy_fields_label_provenance"

// The native baseline catalog is a delta over project schema. Preserve fields
// supplied by that schema before adding the catalog's own compatibility fields.
func snapshotConfiguredStandardPolicyFields(definition *ObjectDefinition) {
	if standardFieldPolicyFor(definition.APIName) == nil && !standardAPI68CapturedNewObject(definition.APIName) {
		return
	}
	if _, recorded := definition.Metadata[standardConfiguredPolicyFields]; recorded {
		return
	}
	// Older persisted schemas have an overlay marker but no provenance. That
	// marker cannot distinguish configured fields from catalog additions. Keep
	// their existing fields rather than assigning an invented generated origin.
	// New overlays snapshot before generating anything, including an empty set.
	fields := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	if definition.Metadata == nil {
		definition.Metadata = map[string]string{}
	}
	// Only a pre-overlay snapshot proves label origin. Older snapshots could
	// have been created from an already-generated legacy overlay.
	if _, legacy := definition.Metadata[standardFieldsOverlayMarker]; !legacy {
		definition.Metadata[standardConfiguredPolicyFieldsLabelProvenance] = "true"
	}
	// Keep the empty snapshot too: a later conversion may synthesize fields,
	// but it must never turn them into configured fields on refresh.
	definition.Metadata[standardConfiguredPolicyFields] = strings.Join(fields, ",")
}

func standardFieldPolicyPreservesConfiguredField(definition ObjectDefinition, field string) bool {
	for remaining := definition.Metadata[standardConfiguredPolicyFields]; remaining != ""; {
		name, rest, _ := strings.Cut(remaining, ",")
		if stringsEqualFold(field, name) {
			return true
		}
		remaining = rest
	}
	return false
}

// Optional fields explicitly supplied by a project are not generated catalog
// placeholders. Their labels belong to that configuration even when a label
// happens to equal the API name.
func standardConfiguredOptionalField(definition ObjectDefinition, field string) bool {
	if definition.Metadata[standardConfiguredPolicyFieldsLabelProvenance] != "true" {
		return false
	}
	if !standardFieldPolicyPreservesConfiguredField(definition, field) {
		return false
	}
	policy := standardFieldPolicyFor(definition.APIName)
	if policy == nil {
		return false
	}
	_, gated := standardPolicyLookup(policy.gates, field)
	absent, _ := standardPolicyLookup(policy.absent, field)
	plainAbsent, _ := standardPolicyLookup(policy.plainDeveloperAbsent, field)
	return gated || absent || plainAbsent
}

// StandardObjectInsertInputFieldAllowed reports captured insertion inputs that
// are absent from the public describe catalog. It does not admit update fields.
func StandardObjectInsertInputFieldAllowed(object, field string) bool {
	if policy := standardFieldPolicyFor(object); policy != nil {
		for _, input := range policy.InputFields {
			if strings.EqualFold(input, field) {
				return true
			}
		}
	}
	return false
}

// StandardObjectInsertForcedValues returns captured Boolean values forced on insert.
// The returned map is a copy of immutable policy data.
func StandardObjectInsertForcedValues(object string) map[string]bool {
	policy := standardFieldPolicyFor(object)
	if policy == nil || len(policy.InsertForcedValues) == 0 {
		return nil
	}
	values := make(map[string]bool, len(policy.InsertForcedValues))
	for name, value := range policy.InsertForcedValues {
		values[name] = value
	}
	return values
}

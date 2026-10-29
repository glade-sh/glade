package storage

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// The sidecar contains only captured field deltas and explicit legacy gates.
// Absence is limited to the recaptured historical candidates, rather than
// treating a partial REST describe as a complete standard schema.
//
//go:embed standard_field_policy.json.gz
var standardFieldPolicyData []byte

type standardFieldPolicyGate struct {
	RecordTypes bool `json:"record_types"`
	Object      string
	Features    []string
	Field       standardDescribeField
	Legacy      bool
}

type standardFieldPolicyObject struct {
	ConstructorFields     []string   `json:"constructor_fields"`
	InputFields           []string   `json:"input_fields"`
	RequiredFeatures      []string   `json:"required_features"`
	RequiredFeaturesAnyOf [][]string `json:"required_features_any_of"`
	Present               []string
	Fields                []standardDescribeField
	Absent                []string
	fields                map[string]Field
	absent                map[string]bool
	gates                 map[string]standardFieldPolicyGate
}

type standardFieldPolicyBundle struct {
	Version  int
	Objects  map[string]*standardFieldPolicyObject
	Gates    []standardFieldPolicyGate
	Sources  map[string]string
	Evidence struct {
		Profiles map[string]map[string]StandardDescribeCatalogDelta `json:"profiles"`
	} `json:"evidence"`
}

var standardFieldPolicies = loadStandardFieldPolicies()

func loadStandardFieldPolicies() map[string]*standardFieldPolicyObject {
	stream, err := gzip.NewReader(bytes.NewReader(standardFieldPolicyData))
	if err != nil {
		panic(err)
	}
	defer stream.Close()
	var bundle standardFieldPolicyBundle
	if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
		panic(err)
	}
	if bundle.Version != 1 {
		panic("unsupported standard field policy version")
	}
	standardDescribeProfileCatalog = bundle.Evidence.Profiles
	policies := make(map[string]*standardFieldPolicyObject, len(bundle.Objects))
	for object, policy := range bundle.Objects {
		policy.fields = map[string]Field{}
		policy.absent = map[string]bool{}
		policy.gates = map[string]standardFieldPolicyGate{}
		for _, field := range policy.Fields {
			policy.fields[strings.ToLower(field.Name)] = describeField(field)
		}
		for _, field := range policy.Absent {
			policy.absent[strings.ToLower(field)] = true
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
	return policies
}

func standardFieldPolicyFor(object string) *standardFieldPolicyObject {
	policy, _ := standardPolicyLookup(standardFieldPolicies, object)
	return policy
}

func standardFieldPolicyGateEnabled(gate standardFieldPolicyGate, signature string) bool {
	for _, feature := range gate.Features {
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
		if gate, ok := standardPolicyLookup(policy.gates, field); ok {
			return (!gate.Legacy && featureSignatureHas(signature, project.UnspecifiedStandardFieldShape)) || standardFieldPolicyPreservesConfiguredField(definition, field) || standardFieldPolicyGateEnabledForDefinition(gate, definition, signature)
		}
		if absent, _ := standardPolicyLookup(policy.absent, field); absent {
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

func applyStandardFieldPolicy(definition *ObjectDefinition, signature string) {
	policy := standardFieldPolicyFor(definition.APIName)
	if policy == nil {
		return
	}
	for _, field := range policy.fields {
		if _, exists := ResolveFieldName(*definition, "", field.APIName); !exists {
			definition.Fields[field.APIName] = cloneField(field)
		}
	}
	for _, gate := range policy.gates {
		if gate.Legacy || !standardFieldPolicyGateEnabledForDefinition(gate, *definition, signature) {
			continue
		}
		field := describeField(gate.Field)
		if name, exists := ResolveFieldName(*definition, "", field.APIName); exists {
			existing := definition.Fields[name]
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
	for name := range policy.gates {
		removeGenerated(name)
	}
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
	// An enabled gate can add a field outside the captured baseline surface.
	if gate, ok := standardPolicyLookup(policy.gates, field); ok {
		return standardFieldPolicyGateEnabledForDefinition(gate, definition, signature)
	}
	// Custom fields still require their project metadata in the semantic checker.
	return stringsHasSuffixFold(field, "__c") || stringsHasSuffixFold(field, "__r") || stringsHasSuffixFold(field, "__pc") || stringsHasSuffixFold(field, "__pr")
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
	if standardFieldPolicyFor(definition.APIName) == nil {
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
	return gated || absent
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

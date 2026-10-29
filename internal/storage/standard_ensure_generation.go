package storage

import (
	"maps"
	"reflect"
	"strings"
)

const standardEnsureProofKey = "storage:standard-ensure-record-types"

type standardEnsureProof struct {
	fields      uintptr
	recordTypes []RecordTypeInfo
}

func recordTypeSeedSchemaReady(org *OrgState) bool {
	state, ok := org.Objects["RecordType"]
	if !ok || state.Definition.APIName == "" || state.Definition.KeyPrefix == "" {
		return false
	}
	for _, name := range []string{"Name", "DeveloperName", "NamespacePrefix", "SobjectType", "IsActive", "IsPersonType", "Description"} {
		found := false
		for key := range state.Definition.Fields {
			if strings.EqualFold(key, name) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func standardRecordTypeEnsureProven(org *OrgState, definition ObjectDefinition) bool {
	if !recordTypeSeedSchemaReady(org) || !recordTypeDefinitionSeeded(org, definition) {
		return false
	}
	generation := org.ensureRuntimeGeneration()
	generation.mu.Lock()
	defer generation.mu.Unlock()
	proofs, _ := generation.values[standardEnsureProofKey].(map[string]standardEnsureProof)
	proof, ok := proofs[definition.APIName]
	return ok && proof.fields == reflect.ValueOf(definition.Fields).Pointer() && reflect.DeepEqual(proof.recordTypes, definition.RecordTypes)
}

func recordTypeDefinitionSeeded(org *OrgState, definition ObjectDefinition) bool {
	// Match EnsureRecordTypeIDField's presence predicate. Custom objects do
	// not have a catalog plan that can detect this missing derived field. A
	// standard object whose field policy omits RecordTypeId (Master only, no
	// record-type evidence) never receives the field, so its absence is the
	// prepared state rather than a missing seed.
	if len(definition.RecordTypes) != 0 && StandardObjectRecordTypeFieldEnabled(definition) {
		if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); !exists {
			return false
		}
	}
	person, defaultPerson := false, false
	for _, info := range definition.RecordTypes {
		if strings.EqualFold(info.DeveloperName, "Master") {
			if info.ID != "012000000000000AAA" {
				return false
			}
			continue
		}
		if info.ID == "" || info.Name == "" {
			return false
		}
		row, ok := org.Objects["RecordType"].Records[info.ID]
		if !ok {
			return false
		}
		for _, field := range []string{"Name", "DeveloperName", "NamespacePrefix", "SobjectType", "IsActive", "IsPersonType", "Description"} {
			if _, ok := row.Fields[field]; !ok {
				return false
			}
		}
		if row.System.CreatedDate == "" || row.System.LastModifiedDate == "" || row.System.SystemModstamp == "" || row.System.CreatedByID == "" || row.System.LastModifiedByID == "" {
			return false
		}
		object := row.Fields["SobjectType"]
		developer := row.Fields["DeveloperName"]
		namespace := row.Fields["NamespacePrefix"]
		if object.String != definition.APIName || developer.String != info.DeveloperName || namespace.String != recordTypeNamespacePrefix(*org, info) {
			return false
		}
		if definition.APIName == "Account" && recordTypeIsPersonType("Account", info) {
			person = person || info.Active || info.Available
			defaultPerson = defaultPerson || info.Default
		}
	}
	return !person || defaultPerson
}

func rememberStandardRecordTypeEnsure(org *OrgState, definition ObjectDefinition) {
	if len(definition.RecordTypes) == 0 || !recordTypeSeedSchemaReady(org) {
		return
	}
	proof := standardEnsureProof{fields: reflect.ValueOf(definition.Fields).Pointer(), recordTypes: append([]RecordTypeInfo(nil), definition.RecordTypes...)}
	for i := range proof.recordTypes {
		proof.recordTypes[i].PicklistDefaults = maps.Clone(proof.recordTypes[i].PicklistDefaults)
	}
	generation := org.ensureRuntimeGeneration()
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if generation.values == nil {
		generation.values = make(map[string]any)
	}
	proofs, _ := generation.values[standardEnsureProofKey].(map[string]standardEnsureProof)
	if proofs == nil {
		proofs = make(map[string]standardEnsureProof)
		generation.values[standardEnsureProofKey] = proofs
	}
	proofs[definition.APIName] = proof
}

// PrimeStandardObjectEnsureNoops records prepared record-type definitions at
// an immutable runtime-template boundary. Later ensures may retain schema
// caches only while the same fields and complete record-type metadata remain.
func PrimeStandardObjectEnsureNoops(org *OrgState) {
	if org == nil || !recordTypeSeedSchemaReady(org) {
		return
	}
	for name, state := range org.Objects {
		if len(state.Definition.RecordTypes) != 0 && recordTypeDefinitionSeeded(org, state.Definition) && !standardObjectDefinitionNeedsWrite(state.Definition, name, standardEnsureFeatureSignature(org, state.Definition)) && !StandardObjectFieldsNeedWrite(state.Definition) {
			rememberStandardRecordTypeEnsure(org, state.Definition)
		}
	}
}

// standardEnsureFeatureSignature is the signature EnsureStandardObject would
// compare this definition's overlay against: the org's feature profile when
// one is set, otherwise the signature the definition's own marker recorded.
func standardEnsureFeatureSignature(org *OrgState, definition ObjectDefinition) string {
	if org != nil && org.Features != nil {
		return canonicalFeatureSignature(org.Features)
	}
	return canonicalFeatureSignature(strings.Split(definition.Metadata[standardFieldsOverlayMarker], ","))
}

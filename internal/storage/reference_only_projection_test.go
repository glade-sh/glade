package storage

import (
	"slices"
	"testing"
)

func TestReferenceOnlyTargetsKeepCatalogFieldProjection(t *testing.T) {
	previous := standardDescribeCatalogV2RawProductionCache
	targets := map[string][]string{
		"ActivationId": {"ActivationTarget"},
		"ActionId":     {"DataAction"},
		"MixedId":      {"Account", "BusinessUnit"},
	}
	standardDescribeCatalogV2RawProductionCache = newStandardDescribeCatalogV2RawCache(func(entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
		fields := []standardDescribeField{{Name: "UnavailableId", Type: "reference", ReferenceTo: []string{"Account", "RefOnlyMissingType"}}}
		for name, refs := range targets {
			fields = append(fields, standardDescribeField{Name: name, Type: "reference", ReferenceTo: refs})
		}
		return standardDescribeObject{Name: entry.Name, Fields: fields}, nil
	})
	t.Cleanup(func() { standardDescribeCatalogV2RawProductionCache = previous })
	definition := ObjectDefinition{APIName: "AssociatedLocation", Fields: map[string]Field{}}
	mergeStandardSObjectStubReferencesFromV2Describe(&definition, nil)
	for name, refs := range targets {
		if field := definition.Fields[name]; field.Type != FieldReference || !slices.Equal(field.ReferenceTo, refs) {
			t.Fatalf("reference-only field %s lost its catalog targets: %+v", name, field)
		}
	}
	if _, ok := definition.Fields["UnavailableId"]; ok {
		t.Fatal("unknown target admitted")
	}
	plan := buildStandardOverlayCheckPlan(definition.APIName)
	if plan.stubReferenceFieldsNeedRepair(definition, "") || legacyStandardStubReferenceFieldsNeedRepair(definition, nil) {
		t.Fatal("complete reference-only projection requests repair")
	}
	delete(definition.Fields, "MixedId")
	if !plan.stubReferenceFieldsNeedRepair(definition, "") || !legacyStandardStubReferenceFieldsNeedRepair(definition, nil) {
		t.Fatal("missing reference-only field does not request repair")
	}
}

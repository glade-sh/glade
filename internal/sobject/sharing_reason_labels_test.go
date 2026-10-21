package sobject

import (
	"maps"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestGeneratedShareRowCausePreservesAuthoredLabelAndLegacyFallback(t *testing.T) {
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{{
		Name:                "AuditArchive__c",
		SharingModel:        "ReadWrite",
		SharingReasons:      []string{"Authored_Audit__c", "Legacy_Audit__c"},
		SharingReasonLabels: map[string]string{"Authored_Audit__c": "Audit & review"},
	}}})
	describe, err := registry.Describe("AuditArchive__Share")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Authored_Audit__c": "Audit & review", "Legacy_Audit__c": "Legacy_Audit__c", "Manual": "Manual Sharing"}
	assertSharingReasonPicklistLabels(t, describe.Fields["RowCause"].PicklistValues, want)
	definition := ToObjectDefinition(describe)
	assertSharingReasonPicklistLabels(t, definition.Fields["RowCause"].PicklistValues, want)
	roundTrip := FromObjectDefinition(definition)
	assertSharingReasonPicklistLabels(t, roundTrip.Fields["RowCause"].PicklistValues, want)
}

func TestMergeSchemaObjectPreservesSharingReasonLabelsWithoutAliasing(t *testing.T) {
	base := schema.Object{Name: "AuditArchive__c", SharingReasons: []string{"Base_Audit__c", "Authored_Audit__c"}, SharingReasonLabels: map[string]string{
		"Base_Audit__c":     "Base audit",
		"Authored_Audit__c": "Earlier label",
	}}
	overlay := schema.Object{Name: "AuditArchive__c", SharingReasons: []string{"Authored_Audit__c", "Overlay_Audit__c"}, SharingReasonLabels: map[string]string{
		"Authored_Audit__c": "Audit & review",
		"Overlay_Audit__c":  "Overlay audit",
	}}
	baseLabels, overlayLabels := maps.Clone(base.SharingReasonLabels), maps.Clone(overlay.SharingReasonLabels)
	merged := mergeSchemaObject(base, overlay)
	if !reflect.DeepEqual(merged.SharingReasons, []string{"Base_Audit__c", "Authored_Audit__c", "Overlay_Audit__c"}) {
		t.Fatalf("merged sharing reason tokens = %#v", merged.SharingReasons)
	}
	want := map[string]string{"Base_Audit__c": "Base audit", "Authored_Audit__c": "Audit & review", "Overlay_Audit__c": "Overlay audit"}
	if !reflect.DeepEqual(merged.SharingReasonLabels, want) {
		t.Fatalf("merged labels = %#v, want %#v", merged.SharingReasonLabels, want)
	}
	merged.SharingReasonLabels["Base_Audit__c"] = "Changed merged base label"
	merged.SharingReasonLabels["Overlay_Audit__c"] = "Changed merged overlay label"
	if !reflect.DeepEqual(base.SharingReasonLabels, baseLabels) || !reflect.DeepEqual(overlay.SharingReasonLabels, overlayLabels) {
		t.Fatalf("merged label map aliases input: base=%#v overlay=%#v", base.SharingReasonLabels, overlay.SharingReasonLabels)
	}
}

// Alternate-case overlays follow the existing case-insensitive token merge;
// this checks local metadata precedence rather than claiming a native case row.
func TestMergeSchemaObjectSharingReasonLabelCaseControl(t *testing.T) {
	base := schema.Object{Name: "AuditArchive__c", SharingModel: "ReadWrite", SharingReasons: []string{"Authored_Audit__c"}, SharingReasonLabels: map[string]string{"Authored_Audit__c": "Base audit"}}
	overlay := schema.Object{Name: "AuditArchive__c", SharingReasons: []string{"authored_audit__c"}, SharingReasonLabels: map[string]string{"authored_audit__c": "Overlay audit"}}
	merged := mergeSchemaObject(base, overlay)
	if !reflect.DeepEqual(merged.SharingReasons, []string{"Authored_Audit__c"}) {
		t.Fatalf("merged token spelling changed: %#v", merged.SharingReasons)
	}
	if got := merged.SharingReasonLabel("Authored_Audit__c"); got != "Overlay audit" {
		t.Fatalf("canonical token overlay label = %q", got)
	}
	if len(merged.SharingReasonLabels) != 1 || merged.SharingReasonLabels["Authored_Audit__c"] != "Overlay audit" {
		t.Fatalf("labels not keyed by retained token: %#v", merged.SharingReasonLabels)
	}
	registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{merged}})
	describe, err := registry.Describe("AuditArchive__Share")
	if err != nil {
		t.Fatal(err)
	}
	assertSharingReasonPicklistLabels(t, describe.Fields["RowCause"].PicklistValues, map[string]string{"Authored_Audit__c": "Overlay audit", "Manual": "Manual Sharing"})
	if base.SharingReasonLabels["Authored_Audit__c"] != "Base audit" || overlay.SharingReasonLabels["authored_audit__c"] != "Overlay audit" {
		t.Fatal("alternate-case merge changed an input label map")
	}
}

func assertSharingReasonPicklistLabels(t *testing.T, values []storage.PicklistValue, want map[string]string) {
	t.Helper()
	found := map[string]bool{}
	for _, value := range values {
		label, checked := want[value.Value]
		if !checked {
			continue
		}
		if found[value.Value] || value.Label != label || !value.Active {
			t.Fatalf("RowCause entry = %#v, want active unique token %q with label %q", value, value.Value, label)
		}
		found[value.Value] = true
	}
	if len(found) != len(want) {
		t.Fatalf("RowCause tokens = %#v, want %#v", found, want)
	}
}

package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceGateUnstructuredErrorBoundary(t *testing.T) {
	// Identical prose is insufficient: only structured schema ownership allows
	// scoping. A previously handled error must not hide a new unknown error.
	scoped := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA003", Message: "lookup target missing", SchemaObject: "Parcel__c", File: "Parcel.object"}
	gate := &compileGate{dependencyErrors: []diagnostic.Diagnostic{scoped}}
	for _, route := range []string{"fileless_unknown", "metadata_unknown", "other_object"} {
		t.Run(route, func(t *testing.T) {
			unknown := scoped
			unknown.SchemaObject = ""
			if route == "fileless_unknown" {
				unknown.File = ""
			}
			if route == "other_object" {
				unknown.SchemaObject = "OtherParcel__c"
			}
			got := gate.withoutScopedDependencyErrors([]diagnostic.Diagnostic{scoped, unknown})
			if len(got) != 1 || got[0] != unknown {
				t.Fatalf("unrelated error lost: %v", got)
			}
			if route != "other_object" {
				if _, ok := gate.attributeDependencyDiagnostics(newCompileGateUnits(typesys.Index{}), got, newSourceCache()); ok {
					t.Fatal("unknown schema ownership was attributed")
				}
			}
		})
	}
}

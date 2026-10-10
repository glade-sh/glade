package sema

import "testing"

func TestMetadataRetrieveGenericAssignability(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name: "retrieve requires explicit narrowing",
			body: `List<Metadata.CustomMetadata> rows = Metadata.Operations.retrieve(
    Metadata.MetadataType.CustomMetadata, new List<String>{'Feature.Default'});`,
			wantCode: "GLADESEMA018",
		},
		{
			// The Apex Reference Guide's Operations.retrieve example keeps the
			// declared base list and casts the retrieved record before field access.
			name: "retrieve base list with record cast",
			body: `List<Metadata.Metadata> rows = Metadata.Operations.retrieve(
    Metadata.MetadataType.CustomMetadata, new List<String>{'Feature.Default'});
Metadata.CustomMetadata row = (Metadata.CustomMetadata) rows[0];
String fullName = row.fullName;
List<Metadata.CustomMetadataValue> values = row.values;`,
		},
		{
			name: "metadata list widening",
			body: `List<Metadata.CustomMetadata> customRows = new List<Metadata.CustomMetadata>();
List<Metadata.Metadata> rows = customRows;`,
		},
		{
			name: "metadata list narrowing requires cast",
			body: `List<Metadata.Metadata> rows = new List<Metadata.Metadata>();
List<Metadata.CustomMetadata> customRows = rows;`,
			wantCode: "GLADESEMA018",
		},
		{
			name: "metadata explicit list cast",
			body: `List<Metadata.Metadata> rows = new List<Metadata.CustomMetadata>();
List<Metadata.CustomMetadata> customRows = (List<Metadata.CustomMetadata>) rows;`,
		},
		{
			name: "metadata sibling lists stay incompatible",
			body: `List<Metadata.Layout> layouts = new List<Metadata.Layout>();
List<Metadata.CustomMetadata> customRows = layouts;`,
			wantCode: "GLADESEMA018",
		},
		{
			name: "user class list widening",
			body: `List<Child> children = new List<Child>();
List<Parent> parents = children;`,
		},
		{
			name: "user class list narrowing requires cast",
			body: `List<Parent> parents = new List<Parent>();
List<Child> children = parents;`,
			wantCode: "GLADESEMA018",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe {
  public virtual class Parent {}
  public class Child extends Parent {}
  public void run() {
` + tc.body + `
  }
}`,
			}, "67.0")
			if tc.wantCode == "" {
				if result.HasErrors() {
					t.Fatalf("valid generic assignment rejected: %#v", result.Diagnostics)
				}
				return
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.wantCode {
				t.Fatalf("diagnostics = %#v, want one %s narrowing diagnostic", result.Diagnostics, tc.wantCode)
			}
		})
	}
}

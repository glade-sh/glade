package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Packet70099B/SHA8d1d54a5f21defecc249db9c92e3c07ad9008ee392587d5616a73b9151203733.
// Schema.FieldSet catalog2201/member4; getNamespace clause197-199.
// Primary6429B/SHAbfdc882349fbbd7eb37274d96d034384e22e42260b25befaab5c86fccbc0739c.
// Clause260B/SHA4823ad02c6c05b717bf9b55f616d9a7ccc385ce321f633255500eda496582664.
// Defining-org origin is this controlled local construction/specification:
// each case creates fresh testDataOrg state and authors its one FieldSet record.
// No installed/package metadata or native provenance is inferred from names.
// Preserve both packet Apex bodies exactly, pkg control before empty focal.
// Native API67 unmanaged Account.FamilyBaseline.getNamespace() is null;
// retained capture /tmp/glade-baseline-schema-controls-org.json, row S01.
// The package namespace control remains unchanged.
func TestExecFieldSetNamespaceAPI67(t *testing.T) {
	cases := []struct {
		name         string
		orgNamespace string
		source       string
	}{
		{name: "orgOwnedNamespacedControl", orgNamespace: "pkg", source: `Schema.FieldSet fs = Schema.SObjectType.Account.fieldSets.getMap().get('Summary');
System.assert(fs != null);
String namespaceValue = fs.getNamespace();
System.assert(namespaceValue != null);
System.assert(namespaceValue.equals('pkg'));
`},
		{name: "orgOwnedWithoutNamespace", orgNamespace: "", source: `Schema.FieldSet fs = Schema.SObjectType.Account.fieldSets.getMap().get('Summary');
System.assert(fs != null);
String namespaceValue = fs.getNamespace();
System.assertEquals(null, namespaceValue);
`},
	}
	for _, tc := range cases {
		if !t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}

			// Fresh locally owned org and complete field-set input per subcase.
			org := testDataOrg()
			org.APIVersion = "67.0"
			org.Namespace = tc.orgNamespace
			account := org.Objects["Account"]
			account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true}
			account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Label: "Rating", Type: storage.FieldString, Required: false}
			org.Objects["Account"] = account
			org.Metadata.FieldSets = []storage.FieldSetMetadata{{
				ObjectName:  "Account",
				Namespace:   "",
				Name:        "Summary",
				Label:       "Account Summary",
				Description: "Summary fields",
				Fields: []storage.FieldSetMemberMetadata{
					{Field: "Name", Required: true},
					{Field: "Rating", Required: false},
				},
				File: "",
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		}) {
			// A failed binding/control must not permit the later focal.
			return
		}
	}
}

package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native dotted reads expose explicit null as false without changing raw data:
// commerce_extension_shapes/org.tsv:50,61,111,122 (WebStore),
// ../apextest/testdata/conformance/checkbox_dynamic_get_native.json's
// explicitNullConstructor/explicitNullPut rows (Contact, API62/67), and
// testdata/constructed_metadata_defaults/org.tsv:53-55,117-119 (metadata).
// The metadata case preserves false even when its schema default is true.
func TestCommerceBooleanNullNativeLinkedAndUnlinkedReads(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			object, field, defaultValue string
		}{
			{"WebStore", "OptionsAutoFacetingEnabled", ""},
			{"Contact", "DoNotCall", "false"},
			{"DefaultProbe__mdt", "Enabled__c", "true"},
		} {
			for _, linked := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/linked=%t", api, tc.object, linked), func(t *testing.T) {
					org := storage.NewOrgState()
					org.Objects[tc.object] = storage.ObjectState{Definition: storage.ObjectDefinition{
						APIName: tc.object,
						Fields:  map[string]storage.Field{tc.field: {APIName: tc.field, Type: storage.FieldBoolean, DefaultValue: tc.defaultValue}},
					}}
					machine := New(nil)
					machine.SetOrg(&org)
					machine.currentMethod.APIVersion = api
					if linked {
						if err := machine.RegisterClass(Class{Name: tc.object, PlatformType: true, SuperClass: "SObject", Fields: map[string]Field{
							tc.field: {Name: tc.field, Type: "Boolean", Access: "public"},
						}}); err != nil {
							t.Fatal(err)
						}
					}
					row := Object(tc.object)
					setExplicitSObjectField(&row, tc.field, Null)
					got, err := machine.lookupPath(row, []string{tc.field})
					if err != nil || got.Kind != ValueBool || got.Bool {
						t.Fatalf("dotted explicit-null read=%#v err=%v, want false", got, err)
					}
					if row.Fields[tc.field].Kind != ValueNull {
						t.Fatal("dotted read changed the stored null")
					}
					// Contact and metadata have native dynamic-null observations.
					// WebStore is a nonregression control for that unchanged path.
					got, handled, err := machine.callSObjectMember(row, "get", []Value{String(tc.field)})
					if err != nil || !handled || got.Kind != ValueNull {
						t.Fatalf("dynamic raw-null read=%#v handled=%t err=%v", got, handled, err)
					}
				})
			}
		}
	}
}

func TestCommerceBooleanNullSourcePropertyControl(t *testing.T) {
	// Compatibility controls for ordinary class fields, including names that
	// collide with schema objects. These are not additional native claims.
	for _, name := range []string{"OwnedBoolean", "OwnedBoolean__c", "OwnedBoolean__mdt"} {
		for _, dependency := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dependency=%t", name, dependency), func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{
					"value": {APIName: "value", Type: storage.FieldBoolean},
				}}}
				machine := New(nil)
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{Name: name, Dependency: dependency, Fields: map[string]Field{
					"value": {Name: "value", Type: "Boolean", Access: "public"},
				}}); err != nil {
					t.Fatal(err)
				}
				row := Object(name)
				row.classInstance = true
				row.projectClass = !dependency
				row.Fields["value"] = Null
				got, err := machine.lookupPath(row, []string{"value"})
				if err != nil || got.Kind != ValueNull {
					t.Fatalf("source property null changed: %#v err=%v", got, err)
				}
			})
		}
	}
}

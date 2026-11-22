package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local controls for uncaptured shapes; their expected results are pinned to
// origin/main rather than inferred from the owner-context Salesforce rows.
func TestNamespacedFieldMapUncapturedNeighbors(t *testing.T) {
	machine := describeShareUnprimedFor(true)
	machine.currentMethod = Method{APIVersion: "67.0", Namespace: "zzns", SourceContextBound: true}
	object := machine.Org.Objects["zzns__Gadget__c"]
	fields := machine.describeSObjectFieldMap("zzns__Gadget__c", object.Definition)
	for _, tc := range []struct {
		name, api, caller, key string
		want                   bool
	}{
		{"qualified", "67.0", "zzns", "zzns__Plain__c", true},
		{"decimal", "67.0", "zzns", "Size__c", true},
		{"different_api", "66.0", "zzns", "Plain__c", true},
		{"other_caller", "67.0", "other", "Plain__c", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine.currentMethod = Method{APIVersion: tc.api, Namespace: tc.caller, SourceContextBound: true}
			got, handled, err := machine.callMapValueMember("", fields, "containsKey", []Value{String(tc.key)}, &Result{})
			if err != nil || !handled || got.Kind != ValueBool || got.Bool != tc.want {
				t.Fatalf("containsKey(%q) = %+v, handled=%t, error=%v; want %t", tc.key, got, handled, err, tc.want)
			}
		})
	}
	t.Run("distinct_local_and_qualified_fields", func(t *testing.T) {
		org := describeShareNamespacedOrg()
		object := org.Objects["zzns__Gadget__c"]
		object.Definition.Fields["zzns__Plain__c"] = storage.Field{APIName: "zzns__Plain__c", Type: storage.FieldString}
		org.Objects["zzns__Gadget__c"] = object
		machine := New(nil)
		machine.SetOrg(&org)
		machine.currentMethod = Method{APIVersion: "67.0", Namespace: "zzns", SourceContextBound: true}
		fields := machine.describeSObjectFieldMap("zzns__Gadget__c", object.Definition)
		got, handled, err := machine.callMapValueMember("", fields, "containsKey", []Value{String("Plain__c")}, &Result{})
		if err != nil || !handled || got.Kind != ValueBool || !got.Bool {
			t.Fatalf("containsKey(local field) = %+v, handled=%t, error=%v; want true", got, handled, err)
		}
	})

}

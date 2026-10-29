package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These synthetic collisions check that the scan optimization preserves the
// existing field-map precedence. They are local controls, not native claims.
func TestFieldMapDirectFieldChildCollisionPrecedence(t *testing.T) {
	for _, tc := range []struct {
		field string
		keys  []string
	}{
		{"Name", []string{"Name", "name"}},
		{"Beacon__c", []string{"Beacon__c", "beacon__c", "pkg__Beacon__c", "pkg__beacon__c"}},
		{"pkg__Grove__c", []string{"pkg__Grove__c", "pkg__grove__c", "Grove__c", "grove__c"}},
	} {
		for _, method := range []string{"get", "containsKey"} {
			for _, key := range tc.keys {
				t.Run(tc.field+"/"+method+"/"+key, func(t *testing.T) {
					org := storage.NewOrgState()
					org.Namespace = "pkg"
					parent := candidatePlanObject("Parent__c")
					parent.Definition.Fields[tc.field] = storage.Field{APIName: tc.field, Type: storage.FieldString}
					org.Objects["Parent__c"] = parent
					org.Objects["Child__c"] = candidatePlanChild("Child__c", "Parent__c", tc.field, 0)
					org.Objects["Other__c"] = candidatePlanChild("Other__c", "Parent__c", "Orphans__r", 0)

					// Confirm the fixture really exercises both sides of the collision
					// without warming the machine used for the dispatch calls.
					probe := candidatePlanMachine(&org)
					if !probe.sObjectFieldMapKeyIsChildRelationship("Parent__c", key) {
						t.Fatalf("%q did not resolve to the synthetic child relationship", key)
					}
					machine := candidatePlanMachine(&org)
					fields := machine.describeSObjectFieldMap("Parent__c", parent.Definition)
					encoded, err := machine.resolvedMapLookupKey(fields, String(key))
					if err != nil || !machine.sObjectFieldMapDirectValueMatchesKey(fields, encoded, key) {
						t.Fatalf("%q is not a matching direct field key: %v", key, err)
					}
					for iteration := 0; iteration < 2; iteration++ {
						before := childRelationshipScanVisits.Load()
						got, handled, err := machine.callMapValueMember("", fields, method, []Value{String(key)}, nil)
						if err != nil || !handled {
							t.Fatalf("%s(%q): handled=%v err=%v", method, key, handled, err)
						}
						if method == "get" {
							canonical, ok := sObjectFieldMapCanonicalFieldName(got)
							if !ok || canonical != machine.describeFieldName(tc.field) {
								t.Fatalf("get(%q) = %v, want field %s", key, got, machine.describeFieldName(tc.field))
							}
						} else if got.Kind != ValueBool || !got.Bool {
							t.Fatalf("containsKey(%q) = %v, want true", key, got)
						}
						if scans := childRelationshipScanVisits.Load() - before; scans != 0 {
							t.Errorf("iteration %d: direct collision lookup scanned %d child candidates", iteration, scans)
						}
					}
					got, _, err := machine.callMapValueMember("", fields, method, []Value{String("Orphans__r")}, nil)
					if err != nil || (method == "get" && got.Kind != ValueNull) || (method == "containsKey" && (got.Kind != ValueBool || got.Bool)) {
						t.Fatalf("%s(child-only key) = %v, err %v", method, got, err)
					}
				})
			}
		}
	}
}

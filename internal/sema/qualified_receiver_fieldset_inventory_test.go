package sema

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFieldSetInventoryAuthorityBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, snapshot, receiver string
		known                    bool
	}{
		{"observed", `{"name":"Probe__c","fieldSetsComplete":true,"fieldSetNames":["main","mainlist"]}`, "SObjectType.Probe__c.FieldSets.MainList", true},
		{"absent_complete", `{"name":"Probe__c","fieldSetsComplete":true,"fieldSetNames":["main","mainlist"]}`, "SObjectType.Probe__c.FieldSets.Missing", false},
		{"absent_incomplete", `{"name":"Probe__c","fieldSetsComplete":false,"fieldSetNames":["main"]}`, "SObjectType.Probe__c.FieldSets.Missing", true},
		{"absent_partial", `{"name":"Probe__c","partial":true,"fieldSetsComplete":true,"fieldSetNames":["main"]}`, "SObjectType.Probe__c.FieldSets.Missing", true},
		{"opaque_dependency", `{"name":"vendor__Probe__c","fieldSetsComplete":true,"fieldSetNames":["main"]}`, "SObjectType.vendor__Probe__c.FieldSets.Missing", true},
		{"unknown_fieldset_namespace", `{"name":"Probe__c","fieldSetsComplete":true,"fieldSetNames":["main"]}`, "SObjectType.Probe__c.FieldSets.vendor__Other", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var object schema.Object
			if err := json.Unmarshal([]byte(tc.snapshot), &object); err != nil {
				t.Fatal(err)
			}
			idx := typesys.Index{Objects: []schema.Object{object}}
			a := NewAnalyzer()
			a.queryDeclaredObjects = []schema.Object{object}
			a.deps = map[string]bool{"vendor": true}
			a.opaqueDeps = map[string]bool{"vendor": true}
			if got := semaQualifiedCallReceiverKnown(a, "", tc.receiver, newIRSemaScope(nil), buildSemaTypeMemberView(idx)); got != tc.known {
				t.Fatalf("receiver known=%v, want %v", got, tc.known)
			}
		})
	}
}

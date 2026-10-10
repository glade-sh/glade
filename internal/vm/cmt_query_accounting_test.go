package vm

import (
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCMTQueryCountResolvesProjectedFieldMetadata(t *testing.T) {
	machine := New(nil)
	org := storage.OrgState{Namespace: "pkg", Objects: map[string]storage.ObjectState{
		"pkg__Config__mdt": {Definition: storage.ObjectDefinition{APIName: "pkg__Config__mdt", Fields: map[string]storage.Field{
			"pkg__Long__c":  {APIName: "pkg__Long__c", Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 131072},
			"pkg__Short__c": {APIName: "pkg__Short__c", Type: storage.FieldString, DisplayType: "TEXTAREA", Length: 255},
			"pkg__Text__c":  {APIName: "pkg__Text__c", Type: storage.FieldString, DisplayType: "STRING", Length: 1000},
		}}},
	}}
	machine.SetOrg(&org)
	for _, tc := range []struct {
		name, object string
		fields       []string
		want         bool
	}{
		{"namespaced long", "pkg__Config__mdt", []string{"pkg__Long__c"}, true},
		{"unqualified lower case", "config__mdt", []string{"long__c"}, true},
		{"short textarea", "Config__mdt", []string{"Short__c"}, false},
		{"ordinary string", "Config__mdt", []string{"Text__c"}, false},
		{"unknown field", "Config__mdt", []string{"Unknown__c"}, false},
		{"unselected long field", "Config__mdt", []string{"DeveloperName"}, false},
		{"ordinary object", "Account", []string{"Id"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := machine.soqlCountsQueryLimit(soql.Query{Object: tc.object, Fields: tc.fields}); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned standard describe catalog records UserRecordAccess.keyPrefix=null.
// Salesforce's DescribeSObjectResult.getKeyPrefix contract returns a value only
// for objects with a stable prefix (Apex Reference Guide, getKeyPrefix).
func TestDescribePreservesCapturedNullKeyPrefix(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
System.assertEquals(null, UserRecordAccess.SObjectType.getDescribe().getKeyPrefix());
System.assertEquals(null, UserRecordAccess.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).getKeyPrefix());
System.assertEquals(null, Schema.getGlobalDescribe().get('UserRecordAccess').getDescribe().getKeyPrefix());
System.assertEquals('001', Account.SObjectType.getDescribe().getKeyPrefix());
System.assertEquals('a01', PrefixSample__c.SObjectType.getDescribe().getKeyPrefix());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			for _, name := range []string{"UserRecordAccess", "Account"} {
				definition, ok := storage.StandardObjectDefinition(name)
				if !ok {
					t.Fatalf("missing standard object %s", name)
				}
				org.Objects[name] = storage.ObjectState{Definition: definition}
			}
			org.Objects["PrefixSample__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "PrefixSample__c", KeyPrefix: "a01",
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDescribeDoesNotTreatUnobservedEmptyPrefixAsNull(t *testing.T) {
	// This documentation supplement has an explicit empty string, not the
	// native null recorded for UserRecordAccess. Keep its local prefix fallback.
	const name = "AINaturalLangProcessRsltShare"
	prefix, known := storage.StandardObjectDescribeKeyPrefix(name)
	if !known || prefix == nil || *prefix != "" {
		t.Fatal("missing explicit-empty catalog control")
	}
	machine := New(nil)
	desc := machine.describeSObjectValue(name, storage.ObjectDefinition{APIName: name, KeyPrefix: "a01"})
	if got := desc.Fields["keyPrefix"]; got.Kind != ValueString || got.Text != "a01" {
		t.Fatalf("uncaptured prefix = %#v", got)
	}
}

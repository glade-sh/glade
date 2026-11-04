package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows testdata/conformance/case349_default_signal/org.tsv:8,14,17
// and the corresponding mobile rows observe a business default before insert.
func TestCase349DefaultBusinessSignalBeforeTrigger(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		for _, value := range []string{"   ", "false", "4155550123"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				org := testDataOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				object := org.Objects["Account"]
				object.Definition.RecordTypes = []storage.RecordTypeInfo{
					{ID: "012000000000001", DeveloperName: "Business", Name: "Business", Active: true, Available: true, Default: true},
					{ID: "012000000000002", DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true, PersonAccount: true},
				}
				org.Objects["Account"] = object
				machine := New(nil)
				machine.SetOrg(&org)
				records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), field: storage.StringValue(value)}}}
				machine.applySObjectFieldDefaults(records, "")
				machine.applyBeforeDMLDerivedFields(records)
				rt, ok := records[0].GetField("RecordTypeId")
				if !ok || !storage.IDsEqual(storageIDFromValue(rt), "012000000000001") {
					t.Errorf("business default: got %#v", rt)
				}
				person, ok := records[0].GetField("IsPersonAccount")
				if ok && (person.Kind != storage.ValueBoolean || person.Boolean) {
					t.Errorf("before IsPersonAccount: got %#v, want false", person)
				}
			})
		}
	}
}

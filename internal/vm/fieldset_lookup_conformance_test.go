package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecFieldSetDescribeNamespacedLookup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			org.Namespace = "gladetest1"
			for _, name := range []string{"CodexC163Capture", "CodexC163Second"} {
				org.Metadata.FieldSets = append(org.Metadata.FieldSets, storage.FieldSetMetadata{ObjectName: "Account", Name: name})
			}
			program, err := CompileAnonymousWithOptions(`
Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
System.assertEquals(2, sets.size());
System.assertEquals(2, sets.values().size());
System.assertEquals(2, sets.keySet().size());
System.assert(sets.containsKey('gladetest1__CodexC163Capture'));
System.assertNotEquals(null, sets.get('gladetest1__CodexC163Capture'));
System.assert(sets.containsKey('GLADETEST1__CODEXC163CAPTURE'));
System.assertNotEquals(null, sets.get('GLADETEST1__CODEXC163CAPTURE'));
System.assertEquals(false, sets.containsKey('CodexC163Capture'));
System.assertEquals(null, sets.get('CodexC163Capture'));
System.assertEquals(false, sets.containsKey('gladetest1__MissingCapture'));
System.assertEquals(null, sets.get('gladetest1__MissingCapture'));`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecFieldSetDescribeMapMutation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, body string
			readOnly   bool
		}{
			{"remove", `sets.remove('GLADETEST1__CODEXC163CAPTURE');`, true},
			{"put", `sets.put('gladetest1__Other', sets.get('gladetest1__CodexC163Capture'));`, true},
			{"putAll", `Map<String, Schema.FieldSet> additions = new Map<String, Schema.FieldSet>{'gladetest1__Other' => sets.get('gladetest1__CodexC163Capture')}; sets.putAll(additions);`, true},
			{"clearIsolation", `Map<String, Schema.FieldSet> second = Account.SObjectType.getDescribe().fieldSets.getMap();
sets.clear();
Map<String, Schema.FieldSet> fresh = Account.SObjectType.getDescribe().fieldSets.getMap();
System.assertEquals(0, sets.size());
System.assertEquals(false, sets.containsKey('gladetest1__CodexC163Capture'));
System.assertEquals(null, sets.get('gladetest1__CodexC163Capture'));
System.assertEquals(1, second.size());
System.assert(second.containsKey('gladetest1__CodexC163Capture'));
System.assertEquals(1, fresh.size());
System.assert(fresh.containsKey('gladetest1__CodexC163Capture'));`, false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				org.Namespace = "gladetest1"
				org.Metadata.FieldSets = []storage.FieldSetMetadata{{ObjectName: "Account", Name: "CodexC163Capture"}}
				program, err := CompileAnonymousWithOptions(`Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();`+tc.body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				_, err = machine.Execute(program)
				if tc.readOnly {
					if err == nil || !strings.Contains(err.Error(), "System.FinalException: Collection is read-only") {
						t.Fatalf("expected read-only FinalException, got %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecFieldSetDescribeKeyShape(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name, namespace string
			names, keys     []string
		}{
			{"namespaced", "samplepkg", []string{"CodexC163Capture", "CodexC163Second"}, []string{"samplepkg__codexc163capture", "samplepkg__codexc163second"}},
			{"unmanaged", "", []string{"CodexC163Capture"}, []string{"codexc163capture"}},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				org.Namespace = tc.namespace
				for _, name := range tc.names {
					org.Metadata.FieldSets = append(org.Metadata.FieldSets, storage.FieldSetMetadata{ObjectName: "Account", Name: name})
				}
				source := fmt.Sprintf(`Set<String> keys = Account.SObjectType.getDescribe().fieldSets.getMap().keySet();
System.assertEquals(%d, keys.size());`, len(tc.keys))
				for _, key := range tc.keys {
					source += fmt.Sprintf("\nSystem.assert(keys.contains('%s'));", key)
				}
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
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
}

package vm

import (
	"strings"
	"testing"
)

// NATIVE review_round12 (62.0, 67.0): an empty addAll on a field map's key
// set is a no-op returning false, also for a DEFERRED describe and a key set
// returned from a method (ke_addall_list, ke_addall_set, ke_addall_values,
// ke_def_*, ke_returned_addall); a non-empty addAll and add still fault
// (ke_addall_existing, ke_add_existing).
func TestDescribeKeySetEmptyAddAllIsNoOp(t *testing.T) {
	for _, row := range []struct{ source, err string }{
		{"Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); Integer n = k.size(); System.assertEquals(false, k.addAll(new List<String>())); System.assertEquals(n, k.size());", ""},
		{"Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); System.assertEquals(false, k.addAll(new Set<String>()));", ""},
		{"Set<String> k = Account.SObjectType.getDescribe(SObjectDescribeOptions.DEFERRED).fields.getMap().keySet(); System.assertEquals(false, k.addAll(new Map<String,String>().values()));", ""},
		{"Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); k.addAll(new List<String>{'id'});", "Collection is read-only"},
		{"Set<String> k = Schema.SObjectType.Account.fields.getMap().keySet(); k.add('id');", "Collection is read-only"},
	} {
		machine := describeShareUnprimedFor(false)
		program, err := CompileAnonymousWithOptions(row.source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = machine.Execute(program)
		switch {
		case row.err == "" && err != nil:
			t.Fatalf("%s: %v", row.source, err)
		case row.err != "" && (err == nil || !strings.Contains(err.Error(), row.err)):
			t.Fatalf("%s: err=%v, want %q", row.source, err, row.err)
		}
	}
}

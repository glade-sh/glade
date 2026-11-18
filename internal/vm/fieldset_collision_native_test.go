package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native: testdata/conformance/fieldset_namespace_collision/org.tsv:2-5.
func TestFieldSetNamespaceCollisionNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, check := range []struct{ name, source string }{
			{"size", `System.assertEquals(2, sets.size());`},
			{"values", `System.assertEquals(2, sets.values().size());
Set<String> labels = new Set<String>();
for (Schema.FieldSet item : sets.values()) labels.add(item.getLabel());
System.assert(labels.contains('Collision Probe 171'));
System.assert(labels.contains('Collision Local'));`},
			{"keys", `System.assertEquals(2, sets.keySet().size());
System.assert(sets.keySet().contains('firstpkg__collisionprobe171'));
System.assert(sets.keySet().contains('secondpkg__collisionprobe171'));
System.assertNotEquals(null, sets.get('firstpkg__CollisionProbe171'));
System.assertNotEquals(null, sets.get('SECONDPKG__COLLISIONPROBE171'));
System.assertEquals(null, sets.get('CollisionProbe171'));
System.assertEquals(null, sets.get('firstpkg__MissingLayout'));
System.assertEquals(null, sets.get('ghostns__CollisionProbe171'));`},
		} {
			t.Run(api+"/"+check.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				org.Metadata.FieldSets = []storage.FieldSetMetadata{
					{ObjectName: "Account", Namespace: "firstpkg", Name: "CollisionProbe171", Label: "Collision Probe 171"},
					{ObjectName: "Account", Namespace: "secondpkg", Name: "CollisionProbe171", Label: "Collision Local"},
				}
				source := `Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();` + "\n" + check.source
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

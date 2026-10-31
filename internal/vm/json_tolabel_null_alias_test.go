package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// boundary-fr-org.tsv:10,22,34,46 and boundary-de-org.tsv counterparts
// capture a null toLabel alias at APIs 62 and 67 on both execution routes.
// sobject_json3/org.tsv:34-35 captures its omission from both JSON overloads.
func TestJSONToLabelNullAliasReadAndSerialization(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account empty = new Account(Name='Empty'); insert empty;
Id emptyId = empty.Id;
Account row = Database.query('SELECT toLabel(Industry) result FROM Account WHERE Id = :emptyId');
System.assertEquals(null, row.get('result'));
Map<String,Object> normal = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(row));
Map<String,Object> suppressed = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(row, true));
System.assertEquals(false, normal.containsKey('result'));
System.assertEquals(false, suppressed.containsKey('result'));
Boolean rejected = false;
try { row.get('notSelected'); } catch (SObjectException e) { rejected = true; }
System.assertEquals(true, rejected);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Industry"] = storage.Field{APIName: "Industry", Type: storage.FieldPicklist}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

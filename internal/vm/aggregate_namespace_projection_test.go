package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestAggregateNamespaceProjectionGet(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
List<AggregateResult> rows = [SELECT pkg__First__c, COUNT(Id) total FROM Account GROUP BY First__c];
System.assertEquals(1, rows.size());
System.assertEquals('one', rows[0].get('pkg__First__c'));
System.assertEquals(1, rows[0].get('total'));
List<AggregateResult> other = Database.query('SELECT pkg__Second__c, COUNT(Id) total FROM Account GROUP BY Second__c');
System.assertEquals('two', other[0].get('pkg__Second__c'));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.Namespace = "pkg"
			account := org.Objects["Account"]
			account.Definition.Fields["First__c"] = storage.Field{APIName: "First__c", Type: storage.FieldString}
			account.Definition.Fields["Second__c"] = storage.Field{APIName: "Second__c", Type: storage.FieldString}
			account.Records["001000000000001"] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"First__c": storage.StringValue("one"), "Second__c": storage.StringValue("two")}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

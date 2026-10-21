package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestTimeFieldComparison(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			p, err := CompileAnonymousWithOptions(`
Time earlier=Time.newInstance(8,9,10,11); Time later=earlier.addMilliseconds(1);
Clock__c row=[SELECT Clock__c FROM Clock__c LIMIT 1];
System.assert(row.Clock__c < later, 'queried clock < later');
System.assert(row.Clock__c <= earlier, 'queried clock <= same clock');
System.assert(later > row.Clock__c, 'later > queried clock');
System.assert(earlier >= row.Clock__c, 'same clock >= queried clock');
System.assertEquals(earlier,row.Clock__c);
System.assertEquals(11,row.Clock__c.millisecond());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Clock__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Clock__c", KeyPrefix: "a0C", Fields: map[string]storage.Field{"Clock__c": {APIName: "Clock__c", Type: storage.FieldTime}}},
				Records:    map[storage.ID]storage.Record{"a0C000000000001": {ID: "a0C000000000001", Object: "Clock__c", Fields: map[string]storage.Value{"Clock__c": storage.StringValue("08:09:10.011Z")}}},
			}
			m := New(nil)
			m.SetOrg(&org)
			if _, err = m.Execute(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTimeFormulaComparison(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Clock__c row=[SELECT FormulaClock__c FROM Clock__c LIMIT 1];
Time earlier=Time.newInstance(8,9,10,11);Time later=earlier.addMilliseconds(1);
System.assert(row.FormulaClock__c < later);
System.assert(row.FormulaClock__c <= earlier);
System.assert(later > row.FormulaClock__c);
System.assert(earlier >= row.FormulaClock__c);
System.assertEquals(earlier,row.FormulaClock__c);
System.assertEquals(11,row.FormulaClock__c.millisecond());
System.assertEquals(earlier,(Time)row.get('FormulaClock__c'));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.Objects["Clock__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Clock__c", KeyPrefix: "a0C", Fields: map[string]storage.Field{
					"Clock__c":        {APIName: "Clock__c", Type: storage.FieldTime},
					"FormulaClock__c": {APIName: "FormulaClock__c", Type: storage.FieldCalculated, DisplayType: "TIME", Formula: "Clock__c"},
				}},
				Records: map[storage.ID]storage.Record{"a0C000000000001": {ID: "a0C000000000001", Object: "Clock__c", Fields: map[string]storage.Value{"Clock__c": storage.StringValue("08:09:10.011")}}},
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

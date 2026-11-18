package vm_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// The fixture includes matching and mixed source/transport versions and both
// native Apex routes. Only the configured restricted-picklist shape is modeled.
func TestRestrictedPicklistSourceVersionRows(t *testing.T) {
	raw, err := os.ReadFile("../soql/testdata/restricted_picklist_null_order/source_policy/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 73 {
		t.Fatalf("native clause rows %d, want 72", len(lines)-1)
	}
	for i, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 5 {
			t.Fatal(line)
		}
		t.Run(fmt.Sprintf("%s/%s/%s/%s", cells[0], cells[1], cells[2], cells[3]), func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = cells[1]
			def := storage.ObjectDefinition{APIName: "OrderingItem__c", Fields: map[string]storage.Field{
				"Id":      {APIName: "Id", Type: storage.FieldID},
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Rank__c": {APIName: "Rank__c", Type: storage.FieldPicklist, RestrictedPicklist: true, PicklistValues: []storage.PicklistValue{{Value: "Zulu"}, {Value: "Alpha"}, {Value: "Middle"}}},
			}}
			records := map[storage.ID]storage.Record{}
			for n, value := range []string{"Zulu", "Alpha", "Middle", ""} {
				id := storage.ID(fmt.Sprintf("a00%012d", n+1))
				fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("v%d", n)), "Id": storage.IDValue(id)}
				if value != "" {
					fields["Rank__c"] = storage.StringValue(value)
				}
				records[id] = storage.Record{ID: id, Object: def.APIName, Fields: fields}
			}
			org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: records}
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if cells[2] == "istest" {
				machine.EnableTestContext()
			}
			source := `List<String> names = new List<String>(); for (OrderingItem__c row : Database.query('SELECT Name FROM OrderingItem__c ORDER BY Rank__c ` + cells[3] + `, Id ASC')) names.add(row.Name); return String.join(names, ',');`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cells[0]})
			if err != nil {
				t.Fatal(err)
			}
			method := vm.Method{Name: "OrderingPolicy.run", ClassName: "OrderingPolicy", APIVersion: cells[0], ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "static"}, Program: program}
			if err := machine.RegisterClass(vm.Class{Name: "OrderingPolicy", Methods: map[string]vm.Method{"run": method}}); err != nil {
				t.Fatal(err)
			}
			caller, err := vm.CompileAnonymousWithOptions("System.debug(OrderingPolicy.run());", vm.CompileOptions{APIVersion: cells[1]})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(caller)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(result.Debug, "\n"); !strings.Contains(got, cells[4]) {
				t.Fatalf("org.tsv:%d native %s, local %s", i+2, cells[4], got)
			}
		})
	}
}

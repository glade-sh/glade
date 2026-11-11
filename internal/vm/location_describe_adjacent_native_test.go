package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native observations: location_describe_adjacent/org.tsv, APIs 62 and 67.
func TestLocationDescribeAdjacentNative(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "location_describe_adjacent", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 {
		t.Fatalf("native row count = %d, want 25", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []struct{ name, point, text string }{
			{"Account", "Second_Point__c", "Second_Text__c"},
			{"PlaceProbe__c", "Place_Point__c", "Place_Text__c"},
		} {
			t.Run(api+"/"+object.name, func(t *testing.T) {
				org := testDataOrg()
				org.APIVersion = api
				state, exists := org.Objects[object.name]
				if !exists {
					state = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object.name, Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
				}
				state.Definition.Fields[object.point] = storage.Field{APIName: object.point, Type: storage.FieldLocation, DisplayType: "LOCATION"}
				state.Definition.Fields[object.text] = storage.Field{APIName: object.text, Type: storage.FieldString}
				org.Objects[object.name] = state
				source := fmt.Sprintf(`Map<String, Schema.SObjectField> fields = Schema.getGlobalDescribe().get('%s').getDescribe().fields.getMap();`, object.name)
				selected := 0
				for _, row := range rows[1:] {
					if len(row) != 7 {
						t.Fatalf("invalid native row: %v", row)
					}
					if row[0] != api || row[2] != object.name {
						continue
					}
					selected++
					source += fmt.Sprintf("\nSystem.assertEquals(%s, fields.containsKey('%s'));", row[4], row[3])
					if row[4] == "false" {
						source += fmt.Sprintf("\nSystem.assertEquals(null, fields.get('%s'));", row[3])
						continue
					}
					source += fmt.Sprintf("\nSystem.assertEquals('%s', fields.get('%s').getDescribe().getName());", row[5], row[3])
					compound := "null"
					if row[6] != "null" {
						compound = "'" + row[6] + "'"
					}
					source += fmt.Sprintf("\nSystem.assertEquals(%s, fields.get('%s').getDescribe().getCompoundFieldName());", compound, row[3])
				}
				if selected != 6 {
					t.Fatalf("selected native rows = %d, want 6", selected)
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

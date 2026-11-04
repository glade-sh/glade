package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestCase349BusinessAccountFollowupNative compares the anonymous route with
// the reviewed source-API-specific Salesforce rows, including Id-based upsert.
func TestCase349BusinessAccountFollowupNative(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_followup/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 185 {
		t.Fatalf("got %d native rows", len(rows)-1)
	}
	for _, row := range rows[1:] {
		if row[6] != "anonymous-inline" {
			continue
		}
		row := row
		t.Run(row[5]+"/"+row[0], func(t *testing.T) {
			var observation struct {
				Result struct {
					Success bool `json:"success"`
					Errors  struct {
						Items []struct {
							Status string `json:"statusCode"`
						} `json:"items"`
					} `json:"errors"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(row[7]), &observation); err != nil {
				t.Fatal(err)
			}
			values := map[string]string{"NULL": "null", "SPACES": "'   '", "TAB_SPACES": "' \\t '", "FALSE_TEXT": "'false'", "NONEMPTY": "'2125550100'"}
			setup := "Account a = new Account(Name='Business', RecordTypeId='012000000000001');\n"
			if row[3] != "insert" {
				setup = "Account seed = new Account(Name='Business', RecordTypeId='012000000000001'); insert seed; Account a = new Account(Id=seed.Id);\n"
			}
			if row[2] != "OMITTED" {
				value, ok := values[row[2]]
				if !ok {
					t.Fatalf("unknown value %q", row[2])
				}
				setup += fmt.Sprintf("a.put('%s', %s);\n", row[1], value)
			}
			resultType := "Database.SaveResult"
			if row[3] == "upsert" {
				resultType = "Database.UpsertResult"
			}
			programText := setup + fmt.Sprintf("%s result = Database.%s(a, false);\nSystem.assertEquals(%t, result.isSuccess(), '%s');\n", resultType, row[3], observation.Result.Success, row[0])
			if !observation.Result.Success {
				programText += fmt.Sprintf("System.assertEquals('%s', String.valueOf(result.getErrors()[0].getStatusCode()));\n", observation.Result.Errors.Items[0].Status)
			}
			program, err := CompileAnonymousWithOptions(programText, CompileOptions{APIVersion: row[5]})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.APIVersion = row[5]
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{
				{ID: "012000000000001", DeveloperName: "Business", Name: "Business", Active: true, Available: true, Default: true},
				{ID: "012000000000002", DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true, PersonAccount: true},
			}
			org.Objects["Account"] = account
			org.Objects["RecordType"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]storage.Field{
					"Id": {APIName: "Id", Type: storage.FieldID},
				}},
				Records: map[storage.ID]storage.Record{
					"012000000000001": {Object: "RecordType", ID: "012000000000001"},
					"012000000000002": {Object: "RecordType", ID: "012000000000002"},
				},
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Errorf("%s: %v", strings.Join(row[:7], "/"), err)
			}
		})
	}
}

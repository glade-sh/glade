package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Even an entirely empty org uses the native deserialize field contracts without
// allocating an installed object map. The earlier route matrix pins the failure.
func TestColdStandardFieldValidationEmptyOrgDeserializeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		table, err := os.ReadFile("../../testdata/conformance/cold_standard_field_validation/native/api" + api[:2] + "/org.tsv")
		if err != nil {
			t.Fatal(err)
		}
		var expected map[string]string
		for _, line := range strings.Split(strings.TrimSpace(string(table)), "\n")[1:] {
			columns := strings.SplitN(line, "\t", 4)
			if len(columns) == 4 && columns[1] == "anonymous" && columns[2] == "deserialize" {
				if err := json.Unmarshal([]byte(columns[3]), &expected); err != nil {
					t.Fatal(err)
				}
			}
		}
		if len(expected) != 8 {
			t.Fatal("missing native deserialize observation")
		}
		for _, route := range []string{"deserialize", "deserializeStrict"} {
			for _, op := range []struct{ name, body string }{
				{"unknownGet", `row.get('DefinitelyMissing');`},
				{"unknownPut", `row.put('DefinitelyMissing','x');`},
				{"typedPut", `row.put('Quantity','not-a-number');`},
				{"readonlyPut", `row.put('CreatedDate',Datetime.newInstance(2026,1,2,0,0,0));`},
			} {
				t.Run(api+"/"+route+"/"+op.name, func(t *testing.T) {
					org := storage.OrgState{}
					machine := New(nil)
					machine.SetOrg(&org)
					body := `SObject row=(SObject)JSON.` + route + `('{"attributes":{"type":"Asset"},"Name":"probe"}',SObject.class);try {` + op.body + `System.debug('accepted');}catch(Exception issue){System.debug(issue.getTypeName()+': '+issue.getMessage());}`
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 || result.Debug[0] != expected[op.name] {
						t.Fatalf("native=%s; Glade=%v", expected[op.name], result.Debug)
					}
					if org.Objects != nil {
						t.Error("field validation allocated the org object map")
					}
				})
			}
		}
	}
}

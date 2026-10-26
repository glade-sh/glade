package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Even an entirely empty org uses the native deserialize relationship contracts without
// allocating an installed object map. The earlier route matrix pins the failure.
func TestColdStandardRelationshipValidationEmptyOrgDeserializeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		table, err := os.ReadFile("../../testdata/conformance/cold_standard_relationship_validation/native/api" + api[:2] + "/org.tsv")
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
		if len(expected) != 14 {
			t.Fatal("missing native deserialize observation")
		}
		for _, route := range []string{"deserialize", "deserializeStrict"} {
			for _, op := range []struct{ name, body string }{
				{"unknownPutSObject", `row.putSObject('DefinitelyMissing',new Account(Name='parent'));`},
				{"wrongTargetPutSObject", `row.putSObject('Account',new Contact(LastName='parent'));`},
				{"wrongTargetTokenPutSObject", `row.putSObject(Asset.AccountId,new Contact(LastName='parent'));`},
				{"unknownIsSet", `row.isSet('DefinitelyMissing');`},
				{"unknownGetSObject", `row.getSObject('DefinitelyMissing');`},
				{"unknownGetSObjects", `row.getSObjects('DefinitelyMissing');`},
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
						t.Error("relationship validation allocated the org object map")
					}
				})
			}
		}
	}
}

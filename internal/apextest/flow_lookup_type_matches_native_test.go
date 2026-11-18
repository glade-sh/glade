package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Positive matches and a non-filterable field control are recorded in
// testdata/native-capture/flow_type_matching/org.tsv at API 62 and 67.
func TestFlowLookupCapturedTypeMatches(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := strings.TrimSuffix(api, ".0")
			files := map[string]string{}
			methods := map[string]string{}
			numericSeed := `insert new List<Account>{new Account(Name='Flow Numeric Match A',AccountNumber='42'),new Account(Name='Flow Numeric Match B',AccountNumber='true')};`
			picklistSeed := `insert new List<Account>{new Account(Name='Flow Type Match A',Type='42'),new Account(Name='Flow Type Match B',Type='true')};`
			for _, tc := range []struct {
				name, seed, input, want string
				fault                   bool
			}{
				{"InAccountNumberNumber", numericSeed, "new List<Decimal>{42}", "'Flow Numeric Match A'", false},
				{"NotInAccountNumberNumber", numericSeed, "new List<Decimal>{42}", "'Flow Numeric Match B'", false},
				{"InAccountNumberBoolean", numericSeed, "new List<Boolean>{true}", "'Flow Numeric Match B'", false},
				{"NotInAccountNumberBoolean", numericSeed, "new List<Boolean>{true}", "'Flow Numeric Match A'", false},
				{"InPicklistNumber", picklistSeed, "new List<Decimal>{42}", "'Flow Type Match A'", false},
				{"NotInPicklistNumber", picklistSeed, "new List<Decimal>{42}", "'Flow Type Match B'", false},
				{"InPicklistBoolean", picklistSeed, "new List<Boolean>{true}", "'Flow Type Match B'", false},
				{"NotInPicklistBoolean", picklistSeed, "new List<Boolean>{true}", "'Flow Type Match A'", false},
				{"InDescriptionNumber", `insert new Account(Name='Flow Type Match A',Description='42');`, "new List<Decimal>{42}", "", true},
				{"NotInDescriptionBoolean", `insert new Account(Name='Flow Type Match A',Description='true');`, "new List<Boolean>{true}", "", true},
				{"InBooleanBoolean", `insert new Contact(LastName='Flow Type Match A');`, "new List<Boolean>{false}", "'Flow Type Match A'", false},
			} {
				name := "F" + version + tc.name
				path := filepath.Join("testdata", "native-capture", "flow_type_matching", "api"+version, name+".flow-meta.xml")
				metadata, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(metadata)
				call := `Flow.Interview f=Flow.Interview.createInterview('` + name + `',new Map<String,Object>{'values'=>` + tc.input + `}); f.start();`
				if tc.fault {
					methods[tc.name] = tc.seed + `Boolean caught=false; try {` + call + `} catch(Exception e) {caught=true; System.assertEquals('System.FlowException',e.getTypeName());} System.assertEquals(true,caught);`
				} else {
					methods[tc.name] = tc.seed + call + `System.assertEquals(` + tc.want + `,f.getVariableValue('outcome'));`
				}
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}

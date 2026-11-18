package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// The anonymous Salesforce observations are recorded in
// testdata/native-capture/pr163_flow_lookup_collection/org.tsv.
func TestFlowLookupCollectionCapturedProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			methods := map[string]string{}
			dir := "api62"
			if api == "67.0" {
				dir = "api67"
			}
			for _, op := range []string{"In", "NotIn"} {
				name := "Capture_Filter_" + op
				data, err := os.ReadFile(filepath.Join("testdata", "native-capture", "pr163_flow_lookup_collection", dir, name+".flow-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = string(data)
				for _, mode := range []string{"populated", "empty", "unassigned"} {
					input := "new Map<String,Object>()"
					if mode == "populated" {
						input = "new Map<String,Object>{'ids'=>new List<String>{String.valueOf(b.Id)}}"
					}
					if mode == "empty" {
						input = "new Map<String,Object>{'ids'=>new List<String>()}"
					}
					want := "null"
					if op == "In" && mode == "populated" {
						want = "'Capture Collection Seed B'"
					}
					if op == "NotIn" {
						want = "'Capture Collection Seed A'"
					}
					methods[op+"_"+mode] = `Account a = new Account(Name='Capture Collection Seed A'); Account b = new Account(Name='Capture Collection Seed B'); insert new List<Account>{a,b};` +
						`Flow.Interview flow=Flow.Interview.createInterview('` + name + `',` + input + `); flow.start(); System.assertEquals(` + want + `,flow.getVariableValue('outcome'));`
				}
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}

package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The projection and explicit-limit controls are independent of the candidate
// ordinal. Native describe traversal remains unanswered in the capture matrix.
func TestLookupOverflowCandidateProject(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "MELODY"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api, namespace))
				source := filepath.Join("testdata", "lookup_overflow")
				body, err := os.ReadFile(filepath.Join(source, "LookupOverflowRegression.cls"))
				if err != nil {
					t.Fatal(err)
				}
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "classes/LookupOverflowRegression.cls"), string(body))
				writeFile(t, filepath.Join(base, "classes/LookupOverflowRegression.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>46.0</apiVersion><status>Active</status></ApexClass>`)
				files, err := os.ReadDir(filepath.Join(source, "fields"))
				if err != nil {
					t.Fatal(err)
				}
				for _, file := range files {
					body, err := os.ReadFile(filepath.Join(source, "fields", file.Name()))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(base, "objects/Account/fields", file.Name()), string(body))
				}
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"LookupOverflowRegression"}, NoDiskCache: true})
				if summary := run.Summary(); summary.Total != 6 || summary.Passed != 6 {
					data, _ := json.Marshal(run)
					t.Fatalf("synthetic candidate regression: %s", data)
				}
			})
		}
	}
}

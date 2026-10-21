package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These owned named tests passed in Salesforce validation-only deployments.
// Wire Time text must preserve the queried clock's equality and ordering.
func TestTimeFieldWireNativeConformance(t *testing.T) {
	var data struct {
		Captures []struct {
			API, Route, Status, DeploymentID, ReceiptSHA256 string
			TestsCompleted, TestErrors                      int
			Files                                           map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/time_field_wire_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 3 {
		t.Fatal("missing Time field observations")
	}
	for _, capture := range data.Captures {
		t.Run(capture.API, func(t *testing.T) {
			if capture.Route != "isTest" || capture.Status != "Succeeded" || capture.TestsCompleted != 1 || capture.TestErrors != 0 || capture.DeploymentID == "" || len(capture.ReceiptSHA256) != 64 {
				t.Fatal("incomplete native observation")
			}
			root := t.TempDir()
			for path, source := range capture.Files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(path)), source)
			}
			index := loadTestIndex(t, root)
			// Class metadata binds the captured Apex source API. The local
			// REST endpoint is independent and uses supported API 67.
			report := Run(index, Options{SelectedClasses: []string{"OwnedClockReadTest"}, RuntimeRESTAPIVersion: "67.0", Parallelism: 1, NoDiskCache: true})
			if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				for _, suite := range report.Suites {
					for _, testCase := range suite.Cases {
						if testCase.Problem != nil {
							t.Logf("%s: %s", testCase.Problem.Type, testCase.Problem.Message)
						}
					}
				}
				t.Fatalf("Time field replay: %+v", summary)
			}
		})
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpaqueMethodAnnotationsNativeConformance(t *testing.T) {
	var data struct {
		ProjectAPI, TransportAPI, Context string
		Deployment                        struct {
			ID, Status                                 string
			Done, CheckOnly                            bool
			ComponentErrors, TestErrors, Passed, Total int
		}
		Rows []struct {
			Class, SourceAPI, Source, SourceSHA256, Method, Expected string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/opaque_method_annotations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	d := data.Deployment
	if data.ProjectAPI != "62.0" || data.TransportAPI != "62.0" || data.Context != "named test, unpackaged" || d.ID == "" || d.Status != "Succeeded" || !d.Done || !d.CheckOnly || d.ComponentErrors != 0 || d.TestErrors != 0 || d.Passed != 2 || d.Total != 2 || len(data.Rows) != 2 {
		t.Fatal("native observation is incomplete")
	}
	for i, row := range data.Rows {
		t.Run(row.SourceAPI, func(t *testing.T) {
			if row.SourceAPI != []string{"62.0", "67.0"}[i] || row.Class == "" || row.Method != "annotationControl" || row.Expected != "pass" {
				t.Fatal("native method or source API differs")
			}
			sum := sha256.Sum256([]byte(row.Source))
			if row.Source == "" || hex.EncodeToString(sum[:]) != row.SourceSHA256 {
				t.Fatal("native source digest differs")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
			path := filepath.Join(root, "force-app", "main", "default", "classes", row.Class+".cls")
			writeFile(t, path, row.Source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.SourceAPI+"</apiVersion><status>Active</status></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{
				SelectedClasses: []string{row.Class}, SelectedMethod: row.Method,
				// Metadata transport is capture provenance, not a local REST
				// endpoint version. Class metadata pins the executable API.
				NoDiskCache: true,
			})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("native method passed; local summary=%+v problem=%s", summary, firstRunProblem(run))
			}
		})
	}
}

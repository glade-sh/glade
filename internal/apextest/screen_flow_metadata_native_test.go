package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenFlowMetadataNativeConformance(t *testing.T) {
	var data struct {
		Captures []struct {
			API, Terminal               string
			ComponentErrors, TestErrors int
			Observed                    struct {
				ScreenRows                                   int
				ScreenProcessType, ScreenCreate, ScreenStart string
				ScreenTriggerType                            *string
			}
			Files map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/screen_flow_metadata_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 2 {
		t.Fatal("missing native Screen flow observations")
	}
	for _, capture := range data.Captures {
		t.Run(capture.API, func(t *testing.T) {
			o := capture.Observed
			if capture.Terminal != "observation_assertion" || capture.ComponentErrors != 0 || capture.TestErrors != 1 || o.ScreenRows != 1 || o.ScreenProcessType != "Flow" || o.ScreenTriggerType != nil || o.ScreenCreate != "returned" || o.ScreenStart != "System.FlowException" {
				t.Fatal("native Screen flow observation differs")
			}
			root := t.TempDir()
			for path, source := range capture.Files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(path)), source)
			}
			index := loadTestIndex(t, root)
			org := OrgFromIndex(index)
			if len(org.Metadata.Flows) != 0 || len(org.Metadata.FlowDeclarations) != 1 {
				t.Fatal("Screen metadata must remain separate from executable automation")
			}
			report := Run(index, Options{SelectedClasses: []string{"OwnedScreenCatalogTest"}, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
			if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						if row.Problem != nil {
							t.Logf("%s: %s", row.Problem.Type, row.Problem.Message)
						}
					}
				}
				t.Fatalf("Screen metadata replay: %+v", summary)
			}
		})
	}
}

func TestScreenFlowMetadataRefreshesRuntimeKey(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedScreenKey.cls"), `public class OwnedScreenKey { public static void run() {} }`)
	path := filepath.Join(root, "force-app/main/default/flows/OwnedScreenKey.flow-meta.xml")
	source := `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Owned Original</label><processType>Flow</processType><status>Active</status><screens><name>OwnedScreen</name></screens></Flow>`
	writeFile(t, path, source)
	index := loadTestIndex(t, root)
	first, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(source, "Owned Original", "Owned Changed", 1))
	next, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if first == next {
		t.Fatal("Screen metadata-only edit reused the earlier runtime key")
	}
}

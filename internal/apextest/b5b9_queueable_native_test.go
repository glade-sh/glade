package apextest

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// b5b9_feature_objects/org.tsv captures these independently executed named
// test methods in fresh Developer scratch orgs at 62.0 and 67.0. The sibling
// metadata and queueable are the same synthetic sources deployed natively.
func TestB5B9QueueableNativeTestRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/force-app/main/default/"
	for _, version := range []int{62, 67} {
		api := strconv.Itoa(version) + ".0"
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			class := "OwnedNativeTest" + strconv.Itoa(version)
			for _, name := range []string{"classes/OwnedQueue.cls", "classes/OwnedQueue.cls-meta.xml", "classes/" + class + ".cls", "classes/" + class + ".cls-meta.xml", "objects/OwnedFlag__mdt/OwnedFlag__mdt.object-meta.xml", "customMetadata/OwnedFlag.Example.md-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", name), string(data))
			}
			index := loadTestIndex(t, root)
			for _, method := range []string{"signature", "delayBoundary", "delayLower", "delayUpper", "depthBoundary", "depthNegative", "depthZero"} {
				t.Run(method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("native pass; Glade: %+v", run.Summary())
					}
				})
			}
		})
	}
}

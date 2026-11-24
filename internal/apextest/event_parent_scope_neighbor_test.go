package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// These synthetic neighbors use the captured payload and parent/share setup.
// Their expected parent count is the origin/main user-mode sharing result.
func TestEventParentPublicationAndVersionNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, classAPI, triggerAPI, route, want string
	}{
		{"captured62", "62.0", "62.0", "same", "parent1"},
		{"captured67", "67.0", "67.0", "same", "parent1"},
		{"publishedOutside", "62.0", "62.0", "outside", "parent0"},
		{"differentRunAs", "62.0", "62.0", "different", "parent0"},
		{"class67Trigger62", "67.0", "62.0", "same", "parent0"},
		{"class62Trigger67", "62.0", "67.0", "same", "parent0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join("testdata", "conformance", "event_trigger_parent_visibility", "project")
			err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				text := string(raw)
				if strings.HasSuffix(name, "ShareEventProbeTest.cls-meta.xml") {
					text = strings.ReplaceAll(text, "62.0", tc.classAPI)
				}
				if strings.HasSuffix(name, "EvtTrigger.trigger-meta.xml") {
					text = strings.ReplaceAll(text, "62.0", tc.triggerAPI)
				}
				if strings.HasSuffix(name, "ShareEventProbeTest.cls") && tc.route != "same" {
					text = strings.Replace(text, "String out;", "String out; Doc__c d; Integer direct;", 1)
					text = strings.Replace(text, "System.runAs(u) {", "System.runAs(u) {\nSystem.runAs(u) {", 1)
					text = strings.Replace(text, "return out;", "}\nreturn out;", 1)
					text = strings.Replace(text, "Doc__c d = new Doc__c", "d = new Doc__c", 1)
					text = strings.Replace(text, "Integer direct = [", "direct = [", 1)
					publish := "EventBus.publish(new Evt__e(DocId__c = String.valueOf(d.Id)));"
					if tc.route == "outside" {
						text = strings.Replace(text, publish, "}\n"+publish+"\nSystem.runAs(u) {", 1)
					} else {
						text = strings.Replace(text, publish, publish+"\n}\nSystem.runAs(u) {", 1)
					}
				}
				writeFile(t, filepath.Join(root, name), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			results := (&shareMdtProject{index: index, api: tc.classAPI}).runTestClass(t, "ShareEventProbeTest")
			for _, name := range []string{"ownerShare", "otherShare"} {
				if result, ok := results[name]; !ok || !strings.Contains(result, "trig="+tc.want+";") {
					t.Errorf("%s = %q; want %s", name, result, tc.want)
				}
			}
		})
	}
}

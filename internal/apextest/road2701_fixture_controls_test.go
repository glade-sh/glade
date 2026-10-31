package apextest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladecli"
	"github.com/glade-sh/glade/internal/testreport"
)

// The same project is reloaded after its metadata is removed and restored.
// Native observations: metadata_dependency_shapes/api{62,67}/{absent,complete}/org.tsv:2.
func TestRoad2701MetadataFixtureTransitions(t *testing.T) {
	apextest.InvalidateRuntimeCaches()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			write := func(path, body string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			write(filepath.Join(classes, "ControlTest.cls"), `@IsTest private class ControlTest { @IsTest static void check() { MeadowWords__mdt x = new MeadowWords__mdt(Words__c='oak,pine'); System.assertEquals('oak:pine:0',String.join(x.Words__c.split(','),':')+':'+MeadowWords__mdt.getAll().size()); } }`)
			write(filepath.Join(classes, "ControlTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			objectRoot := filepath.Join(root, "force-app/main/default/objects/MeadowWords__mdt")
			sourceRoot := filepath.Join("testdata/conformance/metadata_dependency_shapes/metadata/complete/objects/MeadowWords__mdt")
			for _, phase := range []string{"present", "absent", "present"} {
				if err := os.RemoveAll(objectRoot); err != nil {
					t.Fatal(err)
				}
				if phase == "present" {
					for _, name := range []string{"MeadowWords__mdt.object-meta.xml", "fields/Words__c.field-meta.xml"} {
						data, err := os.ReadFile(filepath.Join(sourceRoot, name))
						if err != nil {
							t.Fatal(err)
						}
						write(filepath.Join(objectRoot, name), string(data))
					}
				}
				for _, cache := range []string{"cold", "warm"} {
					args := []string{"test", "--project", root, "--no-serve", "--read-only", "--no-progress", "--json", "--parallelism", "1", "--no-parallel-methods"}
					if cache == "cold" {
						args = append(args, "--no-cache")
					}
					var out, errs bytes.Buffer
					code := gladecli.Run(context.Background(), args, &out, &errs)
					var envelope struct {
						Data testreport.Run `json:"data"`
					}
					if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
						t.Fatalf("%s/%s: %v: %s %s", phase, cache, err, out.String(), errs.String())
					}
					summary := envelope.Data.Summary()
					if phase == "present" && (code != 0 || summary.Passed != 1 || summary.Total != 1) {
						t.Fatalf("%s/%s: code=%d summary=%+v output=%s stderr=%s", phase, cache, code, summary, out.String(), errs.String())
					}
					if phase == "absent" && code == 0 && summary.OK() {
						t.Fatalf("%s/%s: missing metadata accepted: %s", phase, cache, out.String())
					}
					if phase == "absent" && !strings.Contains(out.String()+errs.String(), "MeadowWords__mdt") {
						t.Fatalf("%s/%s: missing type was not identified: %s %s", phase, cache, out.String(), errs.String())
					}
				}
			}
		})
	}
}

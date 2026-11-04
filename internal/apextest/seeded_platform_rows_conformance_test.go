package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSeededPlatformRowsNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "seeded_platform_rows", "SeededRows", 12, nil)
}

// The triage Apex requires MultiCurrency: native plain-org queries reject
// DatedConversionRate (org.tsv plain rows), rather than returning zero.
func TestSeededPlatformRowsTriageProjectColdWarm(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, schema := range []string{"standard", "custom"} {
			t.Run(api+"/"+schema, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [MultiCurrency]\n")
				if schema == "custom" {
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/Synthetic_Item__c/Synthetic_Item__c.object-meta.xml"), `<CustomObject><label>Synthetic Item</label><pluralLabel>Synthetic Items</pluralLabel><nameField><type>Text</type><label>Name</label></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				}
				source, err := os.ReadFile("testdata/conformance/seeded_platform_rows/TriageTest.cls")
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/classes/ProbeTest.cls")
				writeFile(t, path, string(source))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				index := loadTestIndex(t, root)
				for pass := 0; pass < 3; pass++ {
					run := Run(index, Options{SelectedClasses: []string{"ProbeTest"}, NoDiskCache: pass == 0, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("pass %d: summary=%+v suites=%+v", pass, summary, run.Suites)
					}
				}
			})
		}
	}
}

package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Preserve source-class hash dispatch when a class shadows a record name.
func TestShadowRecordNameHashProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "HashParent.cls"), `public virtual class HashParent {}`)
			writeFile(t, filepath.Join(classes, "Account.cls"), `public class Account extends HashParent { public Integer hashCode() { return 71; } }`)
			writeFile(t, filepath.Join(classes, "Contact.cls"), `public class Contact extends HashParent { public Integer hashCode() { return 72; } }`)
			writeFile(t, filepath.Join(classes, "ShadowHashProbeTest.cls"), `@IsTest private class ShadowHashProbeTest { @IsTest static void observe() { System.assertEquals(71, System.hashCode(new Account())); System.assertEquals(72, System.hashCode(new Contact())); } }`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"ShadowHashProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			for pass := 0; pass < 2; pass++ {
				if pass == 1 {
					ResetPerfCounters()
				}
				run := Run(index, opts)
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					t.Fatalf("run %d: %+v", pass, got)
				}
				if pass == 1 {
					if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
						t.Fatalf("warm project: %+v", phases)
					}
				}
			}
		})
	}
}

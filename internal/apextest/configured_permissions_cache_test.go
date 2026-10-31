package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Configured grants belong to each test run, not to a cached org template.
func TestConfiguredPermissionCacheTransitions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := configuredPermissionFixture(t, api)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ConfiguredCacheTest.cls"), `@IsTest private class ConfiguredCacheTest {
 @IsTest static void granted() {System.assertEquals(true, FeatureManagement.checkPermission('Synthetic_View'));}
 @IsTest static void absent() {System.assertEquals(false, FeatureManagement.checkPermission('Synthetic_View'));}
}`)
			for _, step := range []struct{ name, config, method string }{
				{"none", "test:\n  permissionSets: []\n", "absent"},
				{"set", "test:\n  permissionSets: [Synthetic_Access]\n", "granted"},
				{"group", "test:\n  permissionSetGroups: [Synthetic_Group]\n", "granted"},
				{"removed", "test:\n  permissionSets: []\n", "absent"},
			} {
				t.Run(step.name, func(t *testing.T) {
					writeFile(t, filepath.Join(root, "glade.yml"), step.config)
					run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ConfiguredCacheTest"}, SelectedMethod: step.method})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						data, _ := json.Marshal(run)
						t.Fatalf("configured grant transition: %s", data)
					}
				})
			}
		})
	}
}

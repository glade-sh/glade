package apextest

import (
	"path/filepath"
	"testing"
)

// Expected values are captured in native-capture/user_manage_users/permission_values/org.tsv.
func TestManagePermissionsCachedProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			InvalidateRuntimeCaches()
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "PermissionCache.cls"), `@IsTest private class PermissionCache {
    @IsTest static void capturedValues() {
     Profile admin = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'System Administrator' LIMIT 1];
     Profile standard = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'Standard User' LIMIT 1];
     PermissionSet adminSet = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :admin.Id LIMIT 1];
     PermissionSet standardSet = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :standard.Id LIMIT 1];
     System.assertEquals(true, admin.PermissionsManageUsers); System.assertEquals(true, admin.PermissionsManageInternalUsers);
     System.assertEquals(false, standard.PermissionsManageUsers); System.assertEquals(false, standard.PermissionsManageInternalUsers);
     System.assertEquals(true, adminSet.PermissionsManageUsers); System.assertEquals(true, adminSet.PermissionsManageInternalUsers);
     System.assertEquals(false, standardSet.PermissionsManageUsers); System.assertEquals(false, standardSet.PermissionsManageInternalUsers);
    }
   }`)
			writeFile(t, filepath.Join(classes, "PermissionCache.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for pass := 0; pass < 3; pass++ {
				if pass == 2 {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				run := Run(index, Options{SelectedClasses: []string{"PermissionCache"}, Parallelism: 1, PerfCounters: true})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("pass %d: %+v", pass, run.Suites)
				}
				phases := SnapshotPerfCounters().Phases
				if pass == 1 && phases.MemoryCacheHits != 1 {
					t.Fatalf("no memory cache hit: %+v", phases)
				}
				if pass == 2 && phases.DiskCacheHits != 1 {
					t.Fatalf("no disk cache hit: %+v", phases)
				}
			}
		})
	}
}

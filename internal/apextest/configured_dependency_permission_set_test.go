package apextest

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// test.permissionSets may name a loaded managed dependency's permission set,
// unprefixed or as ns__Name, as a subscriber org holds the installed package's
// sets (the assignment itself is the native-captured PermissionSetAssignment
// insert in the permission_set_dependency rows).
func TestConfiguredTestPermissionSetsAssignDependencySets(t *testing.T) {
	for _, setName := range []string{"Pkg_User", "basens__Pkg_User"} {
		t.Run(setName, func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join("testdata/conformance/permission_set_dependency/project")
			err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(src, path)
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			sub := filepath.Join(root, "sub")
			writeFile(t, filepath.Join(sub, "glade.yml"), "project:\n  managedPackageDependencies: [\"basens:../base\"]\ntest:\n  permissionSets: ["+setName+"]\n")
			writeFile(t, filepath.Join(sub, "force-app/main/default/classes/PsTest.cls"), `@IsTest
private class PsTest {
    @IsTest static void t() {
        System.assertEquals(1, [SELECT Id FROM PermissionSetAssignment WHERE PermissionSet.Name = 'Pkg_User' AND AssigneeId = :UserInfo.getUserId()].size(), 'assigned');
        System.assertEquals(true, FeatureManagement.checkPermission('Pkg_Run'), 'granted');
    }
}
`)
			if _, err := project.Load(sub); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, sub)
			report := RunContext(context.Background(), index, Options{NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					if string(c.Status) != "pass" {
						t.Fatalf("%s.%s: %s %+v", suite.Name, c.Name, c.Status, c.Problem)
					}
				}
			}
			if count != 1 {
				t.Fatalf("cases=%d", count)
			}
		})
	}
}

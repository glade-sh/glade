package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Two packages (and the project) may define a custom permission with the same
// DeveloperName. Their rows stay separate and each package's permission set
// grants only its own.
func TestManagedDependencyCustomPermissionsStaySeparatePerNamespace(t *testing.T) {
	root := t.TempDir()
	build := func(dir string) project.Project {
		cp := filepath.Join(root, dir, "Shared_Run.customPermission-meta.xml")
		ps := filepath.Join(root, dir, "Shared_User.permissionset-meta.xml")
		writeFile(t, cp, `<CustomPermission><label>Shared Run</label></CustomPermission>`)
		writeFile(t, ps, `<PermissionSet><label>Shared User</label><customPermissions><enabled>true</enabled><name>Shared_Run</name></customPermissions></PermissionSet>`)
		return project.Project{CustomPermissionFiles: []string{cp}, PermissionSetFiles: []string{ps}}
	}
	own := build("own")
	depA, depB := build("a"), build("b")
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	applyProjectCustomPermissionRecords(&org, own)
	p := project.Project{ManagedPackageDependencies: []project.ManagedPackageDependency{
		{Namespace: "pkga", Status: "loaded", SourceRoot: "a", Project: &depA},
		{Namespace: "pkgb", Status: "loaded", SourceRoot: "b", Project: &depB},
	}}
	applyManagedDependencyPermissionSetRecords(&org, p, map[string]permissionSetMetadataCacheEntry{})

	permissions := org.Objects["CustomPermission"]
	byNamespace := map[string]storage.ID{}
	for id, record := range permissions.Records {
		ns := record.Fields["NamespacePrefix"].String
		if _, dup := byNamespace[ns]; dup {
			t.Fatalf("two Shared_Run rows for namespace %q", ns)
		}
		byNamespace[ns] = id
	}
	if len(byNamespace) != 3 {
		t.Fatalf("want rows for the project and two packages, got %v", byNamespace)
	}
	for _, ns := range []string{"pkga", "pkgb"} {
		var setID storage.ID
		for id, record := range org.Objects["PermissionSet"].Records {
			if record.Fields["NamespacePrefix"].String == ns {
				setID = id
			}
		}
		if setID == "" {
			t.Fatalf("no permission set for %s", ns)
		}
		granted := 0
		for _, access := range org.Objects["SetupEntityAccess"].Records {
			parent, _ := access.GetField("ParentId")
			if !storageIDValueEqualsText(parent, string(setID)) {
				continue
			}
			granted++
			entity, _ := access.GetField("SetupEntityId")
			if !storageIDValueEqualsText(entity, string(byNamespace[ns])) {
				t.Fatalf("%s set grants %+v, want its own permission %s", ns, entity, byNamespace[ns])
			}
		}
		if granted != 1 {
			t.Fatalf("%s set has %d grants, want 1", ns, granted)
		}
	}
}

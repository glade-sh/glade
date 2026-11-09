package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const ternaryNamedArgumentProbe = `
public class TernaryNamedArgumentProbe {
	public Map<String, String> lookup = new Map<String, String>();
	public ObjectPermissions build(String key, Boolean flag) {
		return new ObjectPermissions(
			PermissionsRead = flag,
			SobjectType = (key == 'a' ? 'Account' : 'Contact'),
			PermissionsEdit = key != null && key == 'b' ? true : false);
	}
	public Integer count() {
		return Schema.SObjectType.ObjectPermissions.fields.getMap().size();
	}
}
`

func objectPermissionFieldNames(org storage.OrgState) []string {
	var names []string
	for name := range org.Objects["ObjectPermissions"].Definition.Fields {
		names = append(names, name)
	}
	return names
}

func hasFieldNamed(org storage.OrgState, want string) bool {
	for _, name := range objectPermissionFieldNames(org) {
		if strings.EqualFold(name, want) {
			return true
		}
	}
	return false
}

// A comparison or ternary inside a named constructor argument, and a method
// reached through fields, are not field references. They must not become
// synthetic fields of the standard object definition.
func TestOrgFromIndexDoesNotInferNamesFromNamedArgumentValuesOrFieldMapMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/TernaryNamedArgumentProbe.cls"), ternaryNamedArgumentProbe)
	org := orgFromIndex(loadTestIndex(t, root))
	for _, leaked := range []string{"key", "getMap", "flag"} {
		if hasFieldNamed(org, leaked) {
			t.Errorf("ObjectPermissions gained synthetic field %q; fields=%v", leaked, objectPermissionFieldNames(org))
		}
	}
	for _, real := range []string{"PermissionsRead", "SobjectType", "PermissionsEdit"} {
		if !hasFieldNamed(org, real) {
			t.Errorf("ObjectPermissions lost field %q", real)
		}
	}
}

// Two orgs built in one process share no definition state: names inferred for
// one project never reach another project's describe input.
func TestOrgFromIndexInferredNamesDoNotCrossOrgs(t *testing.T) {
	rootA := t.TempDir()
	writeFile(t, filepath.Join(rootA, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(rootA, "force-app/main/classes/TernaryNamedArgumentProbe.cls"), ternaryNamedArgumentProbe)
	rootB := t.TempDir()
	writeFile(t, filepath.Join(rootB, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(rootB, "force-app/main/classes/CleanProbe.cls"), `
public class CleanProbe {
	public static Object touch() {
		return new ObjectPermissions(PermissionsRead = true);
	}
}
`)
	orgA := orgFromIndex(loadTestIndex(t, rootA))
	orgB := orgFromIndex(loadTestIndex(t, rootB))
	for _, leaked := range []string{"key", "getMap"} {
		if hasFieldNamed(orgB, leaked) {
			t.Errorf("second org sees %q from the first; fields=%v", leaked, objectPermissionFieldNames(orgB))
		}
	}
	// Mutating one org's definition must not show in the other.
	stateA := orgA.Objects["ObjectPermissions"]
	stateA.Definition.Fields["OnlyInA"] = storage.Field{APIName: "OnlyInA"}
	if hasFieldNamed(orgB, "OnlyInA") {
		t.Fatalf("definition field map is shared between orgs")
	}
}

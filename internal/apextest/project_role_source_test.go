package apextest

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// RS01 records source-role identity, label and parent lookup at API 62/67/68.
func TestProjectRoleSourceHydration(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "absent metadata control"
		if present {
			name = "source role hierarchy"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/RoleSourceMarker.cls"), `public class RoleSourceMarker {}`)
			// Source ordering deliberately places the child before its parent.
			if present {
				writeFile(t, filepath.Join(root, "force-app/main/default/roles/ASeedChild.role-meta.xml"), `<Role xmlns="http://soap.sforce.com/2006/04/metadata"><name>Owned Child</name><parentRole>ZSeedParent</parentRole></Role>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/roles/ZSeedParent.role-meta.xml"), `<Role xmlns="http://soap.sforce.com/2006/04/metadata"><name>Owned Parent</name></Role>`)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			roles := map[string]storage.Record{}
			for _, r := range org.Objects["UserRole"].Records {
				roles[r.Fields["DeveloperName"].String] = r
			}
			if present {
				child, ok := roles["ASeedChild"]
				if !ok {
					t.Fatal("source child role was not hydrated")
				}
				parent, ok := roles["ZSeedParent"]
				if !ok {
					t.Fatal("source parent role was not hydrated")
				}
				if child.Fields["Name"].String != "Owned Child" || parent.Fields["Name"].String != "Owned Parent" || child.Fields["ParentRoleId"].ID != parent.ID {
					t.Fatalf("role labels/parent link: child=%+v parent=%+v", child, parent)
				}
				if len(roles) != 3 {
					t.Fatalf("source roles plus seed: got %d, want 3", len(roles))
				}
			} else if len(roles) != 1 {
				t.Fatalf("absent metadata fabricated roles: %d", len(roles))
			}
			// Metadata must never take over the role to which the seeded user belongs.
			if roles["CEO"].ID != "00E000000000001" || org.Objects["User"].Records[storage.DefaultUserID].Fields["UserRoleId"].ID != roles["CEO"].ID {
				t.Fatal("source role changed the seeded user's role")
			}
			second := orgFromIndex(index)
			if !reflect.DeepEqual(org.Objects["UserRole"].Records, second.Objects["UserRole"].Records) {
				t.Fatal("source role hydration is not deterministic")
			}
		})
	}
}

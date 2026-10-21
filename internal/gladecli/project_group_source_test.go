package gladecli

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestOrgStateFromIndexHydratesSourceGroupsAfterSeed(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "absent metadata control"
		if present {
			name = "declared group"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			if present {
				writeTestFile(t, filepath.Join(root, "force-app/main/default/groups/OwnedGroup.group-meta.xml"), `<Group><name>Owned Group Label</name><doesIncludeBosses>true</doesIncludeBosses></Group>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org, err := orgStateFromIndex(root, p, typesys.Index{Project: typesys.ProjectInfo{Root: p.Root, Namespace: p.Namespace}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, r := range org.Objects["Group"].Records {
				if r.Fields["DeveloperName"].String == "OwnedGroup" {
					found = true
					if r.Fields["Type"].String != "Regular" || !r.Fields["DoesIncludeBosses"].Boolean || r.Fields["Name"].String != "Owned Group Label" {
						t.Fatalf("source group collided with platform identity: %+v", r)
					}
				}
			}
			if found != present {
				t.Fatalf("source group present=%v, want %v", found, present)
			}
			if org.Objects["UserRole"].Records["00E000000000001"].Fields["DeveloperName"].String != "CEO" || org.Objects["User"].Records[storage.DefaultUserID].Fields["UserRoleId"].ID != "00E000000000001" {
				t.Fatal("source loading changed default user role")
			}
		})
	}
}

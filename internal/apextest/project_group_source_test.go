package apextest

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectGroupSourceHydration(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "absent group metadata control"
		if present {
			name = "declared group after role"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/GroupSourceMarker.cls"), `public class GroupSourceMarker {}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/roles/OwnedRole.role-meta.xml"), `<Role><name>Owned Role Label</name></Role>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Shield__c/Shield__c.object-meta.xml"), `<CustomObject><label>Shield</label><pluralLabel>Shields</pluralLabel><nameField><type>Text</type><label>Name</label></nameField><sharingModel>Private</sharingModel></CustomObject>`)
			// Owner-rule source is present, but this test neither materializes nor
			// asserts a grant. Rule evaluation remains a distinct runtime boundary.
			writeFile(t, filepath.Join(root, "force-app/main/default/sharingRules/Shield__c.sharingRules-meta.xml"), `<SharingRules><sharingOwnerRules><fullName>OwnedRule</fullName><accessLevel>Read</accessLevel><label>Owned Rule</label><sharedFrom><group>OwnedOrigin</group></sharedFrom><sharedTo><group>OwnedTarget</group></sharedTo></sharingOwnerRules></SharingRules>`)
			if present {
				writeFile(t, filepath.Join(root, "force-app/main/default/groups/OwnedOrigin.group-meta.xml"), `<Group><name>Owned Origin</name><doesIncludeBosses>true</doesIncludeBosses></Group>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/groups/OwnedTarget.group-meta.xml"), `<Group><name>Owned Target</name><doesIncludeBosses>false</doesIncludeBosses></Group>`)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			roleFound := false
			for _, r := range org.Objects["UserRole"].Records {
				if r.Fields["DeveloperName"].String == "OwnedRole" {
					roleFound = true
				}
			}
			if !roleFound {
				t.Fatal("existing role prerequisite was lost")
			}
			groups := org.Objects["Group"].Records
			if present {
				if len(groups) != 2 {
					t.Fatalf("source group count=%d, want 2", len(groups))
				}
				seen := map[string]bool{}
				for _, r := range groups {
					n := r.Fields["DeveloperName"].String
					seen[n] = true
					if r.Fields["Type"].String != "Regular" || r.Fields["Name"].String != "Owned "+map[string]string{"OwnedOrigin": "Origin", "OwnedTarget": "Target"}[n] || r.Fields["DoesIncludeBosses"].Boolean != (n == "OwnedOrigin") {
						t.Fatalf("source group shape: %+v", r)
					}
				}
				if !seen["OwnedOrigin"] || !seen["OwnedTarget"] {
					t.Fatal("source group identities missing")
				}
			} else if len(groups) != 0 {
				t.Fatalf("absent metadata fabricated groups: %d", len(groups))
			}
			if len(org.Objects["GroupMember"].Records) != 0 {
				t.Fatal("metadata fabricated membership")
			}
			if org.Objects["Shield__c"].Definition.SharingModel != "Private" {
				t.Fatal("group metadata changed private sharing")
			}
			if org.Objects["User"].Records[storage.DefaultUserID].Fields["UserRoleId"].ID != "00E000000000001" {
				t.Fatal("group metadata changed default user's role")
			}
			second := orgFromIndex(index)
			if !reflect.DeepEqual(org.Objects["Group"].Records, second.Objects["Group"].Records) || !reflect.DeepEqual(org.IDSequences, second.IDSequences) {
				t.Fatal("group metadata IDs/counters are not deterministic")
			}
		})
	}
}

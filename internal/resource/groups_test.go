package resource

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectGroupMetadataHydration(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/groups/OwnedOrigin.group-meta.xml"), `<Group><name>Owned Origin</name><doesIncludeBosses>true</doesIncludeBosses></Group>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/groups/OwnedTarget.group-meta.xml"), `<Group><name>Owned Target</name><doesIncludeBosses>false</doesIncludeBosses></Group>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.GroupFiles) != 2 {
		t.Fatalf("group discovery: %v", p.GroupFiles)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "GroupMember")
	state := org.Objects["Group"]
	state.Records["00G000000000001"] = storage.Record{ID: "00G000000000001", Object: "Group", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("ExistingQueue"), "Type": storage.StringValue("Queue")}}
	org.Objects["Group"] = state
	members := org.Objects["GroupMember"]
	members.Records["011000000000007"] = storage.Record{ID: "011000000000007", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue("00G000000000001"), "UserOrGroupId": storage.IDValue("005000000000001")}}
	org.Objects["GroupMember"] = members
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	org.Objects["Account"] = account
	org.IDSequences["Account"] = 91
	before := org.Clone()
	if err := ApplyProjectGroups(&org, p); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Group"].Records) != 3 {
		t.Fatal("source groups missing or unrelated queue lost")
	}
	for _, r := range org.Objects["Group"].Records {
		name := r.Fields["DeveloperName"].String
		if name == "ExistingQueue" {
			continue
		}
		if r.Fields["Type"].String != "Regular" || r.Fields["RelatedId"].Kind != storage.ValueNull || r.Fields["DoesSendEmailToMembers"].Boolean || r.Fields["DoesIncludeBosses"].Boolean != (name == "OwnedOrigin") {
			t.Fatalf("source group shape: %+v", r)
		}
	}
	for name, state := range before.Objects {
		if name != "Group" && !reflect.DeepEqual(state, org.Objects[name]) {
			t.Fatalf("group metadata changed unrelated object %s", name)
		}
	}
	if !reflect.DeepEqual(before.Objects["Group"].Records["00G000000000001"], org.Objects["Group"].Records["00G000000000001"]) || org.IDSequences["Account"] != 91 || org.IDSequences["UserRole"] != before.IDSequences["UserRole"] {
		t.Fatal("existing group or unrelated counters changed")
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences["Group"] = org.IDSequences["Group"]
	next, err := generator.Next("Group")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, occupied := storage.LookupRecordByID(org.Objects["Group"].Records, next); occupied {
		t.Fatal("source IDs collide with subsequent DML")
	}
	first := org.Clone()
	if err := ApplyProjectGroups(&org, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Objects, org.Objects) || !reflect.DeepEqual(first.IDSequences, org.IDSequences) {
		t.Fatal("group reload changed IDs, membership or counters")
	}
}

func TestProjectGroupMetadataAbsentControl(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	before := org.Clone()
	if err := ApplyProjectGroups(&org, project.Project{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatal("absent metadata changed setup")
	}
}

func TestProjectGroupMetadataPreservesExistingRegularIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "groups/Owned.group")
	writeFile(t, path, `<Group><name>Owned Label</name><doesIncludeBosses>true</doesIncludeBosses></Group>`)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	id := storage.ID("00G000000000008EAA")
	state := org.Objects["Group"]
	if _, _, occupied := storage.LookupRecordByID(state.Records, id); occupied {
		t.Fatal("existing regular group fixture ID is already occupied")
	}
	seeded := state.Clone()
	state.Records[id] = storage.Record{ID: id, Object: "Group", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Owned"), "Name": storage.StringValue("Previous"), "Type": storage.StringValue("Regular")}}
	org.Objects["Group"] = state
	before := org.Clone()
	if err := ApplyProjectGroups(&org, project.Project{GroupFiles: []string{path}}); err != nil {
		t.Fatal(err)
	}
	groups := org.Objects["Group"].Records
	if want := len(before.Objects["Group"].Records); len(groups) != want {
		t.Errorf("existing regular group identity changed: group count=%d, want %d; seeded=%d", len(groups), want, len(seeded.Records))
	}
	group, exists := groups[id]
	if !exists || group.ID != id || group.Fields["DeveloperName"].String != "Owned" || group.Fields["Name"].String != "Owned Label" || group.Fields["Type"].String != "Regular" || !group.Fields["DoesIncludeBosses"].Boolean {
		t.Errorf("existing regular group was replaced or not hydrated: %+v", group)
	}
	for seedID, seed := range seeded.Records {
		if !reflect.DeepEqual(seed, groups[seedID]) {
			t.Errorf("metadata changed seeded group %s", seedID)
		}
	}
	if !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Error("updating an existing regular group advanced ID counters")
	}
}

func TestProjectGroupMetadataUnsupportedShapesAreAtomic(t *testing.T) {
	for _, source := range []string{`<Group><name>Omitted</name></Group>`, `<Group><name>Members</name><doesIncludeBosses>true</doesIncludeBosses><groupMembers/></Group>`, `<Group><name>Queue Collision</name><doesIncludeBosses>false</doesIncludeBosses></Group>`} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "groups/Owned.group-meta.xml")
			writeFile(t, path, source)
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			state := org.Objects["Group"]
			state.Records["00G000000000001"] = storage.Record{ID: "00G000000000001", Object: "Group", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Owned"), "Type": storage.StringValue("Queue")}}
			org.Objects["Group"] = state
			before := org.Clone()
			if err := ApplyProjectGroups(&org, project.Project{GroupFiles: []string{path}}); err == nil {
				t.Fatal("unsupported shape was projected")
			}
			if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
				t.Fatal("unsupported shape partially changed setup")
			}
		})
	}
}

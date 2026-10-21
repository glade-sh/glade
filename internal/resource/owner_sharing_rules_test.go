package resource

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

const ownedReadRuleSource = `<SharingRules><sharingOwnerRules><fullName>OwnedReadGrant</fullName><accessLevel>Read</accessLevel><label>Owned Read Grant</label><description>Owned rule description</description><sharedTo><group>OwnedTarget</group></sharedTo><sharedFrom><group>OwnedOrigin</group></sharedFrom></sharingOwnerRules></SharingRules>`

func ownerSharingMetadataFixture(t *testing.T, source string) (storage.OrgState, project.Project) {
	t.Helper()
	root := t.TempDir()
	p := project.Project{}
	for _, name := range []string{"OwnedOrigin", "OwnedTarget"} {
		path := filepath.Join(root, "groups", name+".group-meta.xml")
		writeFile(t, path, `<Group><name>`+name+`</name><doesIncludeBosses>false</doesIncludeBosses></Group>`)
		p.GroupFiles = append(p.GroupFiles, path)
	}
	path := filepath.Join(root, "sharingRules/OwnedBox__c.sharingRules-meta.xml")
	writeFile(t, path, source)
	p.OwnerSharingRuleFiles = []string{path}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "GroupMember")
	org.Objects["OwnedBox__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "OwnedBox__c", SharingModel: "Private", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{"a00000000000001": {ID: "a00000000000001", Object: "OwnedBox__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Existing")}}}}
	org.IDSequences["OwnedBox__c"] = 41
	if err := ApplyProjectGroups(&org, p); err != nil {
		t.Fatal(err)
	}
	return org, p
}

func TestProjectOwnerSharingMetadataHydration(t *testing.T) {
	org, p := ownerSharingMetadataFixture(t, ownedReadRuleSource)
	before := org.Clone()
	if err := ApplyProjectOwnerSharingRules(&org, p); err != nil {
		t.Fatal(err)
	}
	want := []storage.OwnerSharingRule{{Name: "OwnedReadGrant", SourceGroup: "OwnedOrigin", TargetGroup: "OwnedTarget", AccessLevel: "Read"}}
	got := org.Objects["OwnedBox__c"]
	if !reflect.DeepEqual(got.Definition.OwnerSharingRules, want) {
		t.Fatalf("owner rule configuration: %+v", got.Definition.OwnerSharingRules)
	}
	definition := before.Objects["OwnedBox__c"].Definition
	definition.OwnerSharingRules = want
	if !reflect.DeepEqual(got.Definition, definition) || !reflect.DeepEqual(got.Records, before.Objects["OwnedBox__c"].Records) {
		t.Fatal("rule load changed object shape or records")
	}
	for name, state := range before.Objects {
		if name != "OwnedBox__c" && !reflect.DeepEqual(state, org.Objects[name]) {
			t.Fatalf("rule load changed unrelated object %s", name)
		}
	}
	if !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatal("rule load changed ID counters")
	}
	first := org.Clone()
	if err := ApplyProjectOwnerSharingRules(&org, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Objects, org.Objects) || !reflect.DeepEqual(first.IDSequences, org.IDSequences) {
		t.Fatal("reload changed identities or membership")
	}
	clone := got.Definition.Clone()
	clone.OwnerSharingRules[0].Name = "Changed"
	if got.Definition.OwnerSharingRules[0].Name != "OwnedReadGrant" {
		t.Fatal("definition clone aliases owner rules")
	}
}

func TestProjectOwnerSharingMetadataAbsentControl(t *testing.T) {
	org, _ := ownerSharingMetadataFixture(t, ownedReadRuleSource)
	before := org.Clone()
	if err := ApplyProjectOwnerSharingRules(&org, project.Project{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatal("absent metadata changed setup")
	}
}

func TestProjectOwnerSharingMetadataUnsupportedShapesAreAtomic(t *testing.T) {
	for _, source := range []string{
		`<SharingRules/>`,
		strings.Replace(ownedReadRuleSource, "Read</accessLevel>", "Edit</accessLevel>", 1),
		strings.Replace(ownedReadRuleSource, "<group>OwnedTarget</group>", "<role>OwnedTarget</role>", 1),
		strings.Replace(ownedReadRuleSource, "<group>OwnedTarget</group>", "<group>Absent</group>", 1),
		strings.Replace(ownedReadRuleSource, "</SharingRules>", "<sharingCriteriaRules/></SharingRules>", 1),
		strings.Replace(ownedReadRuleSource, "</sharingOwnerRules>", "<includeRecordsOwnedByAll>true</includeRecordsOwnedByAll></sharingOwnerRules>", 1),
		strings.Replace(ownedReadRuleSource, "</SharingRules>", strings.TrimSuffix(strings.TrimPrefix(ownedReadRuleSource, "<SharingRules>"), "</SharingRules>")+"</SharingRules>", 1),
	} {
		t.Run(source, func(t *testing.T) {
			org, p := ownerSharingMetadataFixture(t, source)
			before := org.Clone()
			if err := ApplyProjectOwnerSharingRules(&org, p); err == nil {
				t.Fatal("unsupported source was projected")
			}
			if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
				t.Fatal("unsupported source partially changed setup")
			}
		})
	}
	for _, change := range []string{"nonPrivate", "missingObject", "missingDeclaredGroup", "missingLoadedGroup", "queueTarget", "lateInvalidSource"} {
		t.Run(change, func(t *testing.T) {
			org, p := ownerSharingMetadataFixture(t, ownedReadRuleSource)
			switch change {
			case "nonPrivate":
				s := org.Objects["OwnedBox__c"]
				s.Definition.SharingModel = "ReadWrite"
				org.Objects["OwnedBox__c"] = s
			case "missingObject":
				delete(org.Objects, "OwnedBox__c")
			case "missingDeclaredGroup":
				p.GroupFiles = p.GroupFiles[:1]
			case "missingLoadedGroup":
				delete(org.Objects, "Group")
			case "queueTarget":
				s := org.Objects["Group"]
				for id, r := range s.Records {
					if r.Fields["DeveloperName"].String == "OwnedTarget" {
						r.Fields["Type"] = storage.StringValue("Queue")
						s.Records[id] = r
					}
				}
				org.Objects["Group"] = s
			case "lateInvalidSource":
				path := filepath.Join(t.TempDir(), "ZMissing__c.sharingRules-meta.xml")
				writeFile(t, path, ownedReadRuleSource)
				p.OwnerSharingRuleFiles = append(p.OwnerSharingRuleFiles, path)
			}
			before := org.Clone()
			if err := ApplyProjectOwnerSharingRules(&org, p); err == nil {
				t.Fatal("unsupported setup was projected")
			}
			if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
				t.Fatal("unsupported setup partially changed state")
			}
		})
	}
}

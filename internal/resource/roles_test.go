package resource

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectRoleMetadataHydration(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/roles/AChild.role-meta.xml"), `<Role xmlns="http://soap.sforce.com/2006/04/metadata"><name>Child Label</name><parentRole>ZParent</parentRole></Role>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/roles/ZParent.role-meta.xml"), `<Role xmlns="http://soap.sforce.com/2006/04/metadata"><name>Parent Label</name></Role>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.RoleFiles) != 2 {
		t.Fatalf("source role discovery: %v", p.RoleFiles)
	}
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	org.Objects["Account"] = account
	seed := org.Clone()
	if err := ApplyProjectRoles(&org, p); err != nil {
		t.Fatal(err)
	}
	byName := map[string]storage.Record{}
	for _, r := range org.Objects["UserRole"].Records {
		byName[r.Fields["DeveloperName"].String] = r
	}
	if len(byName) != 3 || byName["AChild"].Fields["Name"].String != "Child Label" || byName["ZParent"].Fields["Name"].String != "Parent Label" || byName["AChild"].Fields["ParentRoleId"].ID != byName["ZParent"].ID {
		t.Fatalf("source hierarchy: %+v", byName)
	}
	if byName["ZParent"].Fields["ParentRoleId"].Kind != storage.ValueNull {
		t.Fatal("root role parent must be null")
	}
	if !reflect.DeepEqual(seed.Objects["UserRole"].Records["00E000000000001"], org.Objects["UserRole"].Records["00E000000000001"]) {
		t.Fatal("existing unrelated role changed")
	}
	if !reflect.DeepEqual(seed.Objects["User"].Records, org.Objects["User"].Records) || org.Objects["Account"].Definition.SharingModel != "Private" {
		t.Fatal("role hydration changed users or private sharing")
	}
	generator := storage.NewStandardIDGenerator()
	generator.Sequences["UserRole"] = org.IDSequences["UserRole"]
	next, err := generator.Next("UserRole")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, occupied := storage.LookupRecordByID(org.Objects["UserRole"].Records, next); occupied {
		t.Fatal("source metadata did not reserve its IDs for later DML")
	}
	first := org.Clone()
	if err := ApplyProjectRoles(&org, p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Objects, org.Objects) || !reflect.DeepEqual(first.IDSequences, org.IDSequences) {
		t.Fatal("reloading source roles changed IDs or unrelated setup")
	}
}

func TestProjectRoleMetadataAbsentControl(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	before := org.Clone()
	if err := ApplyProjectRoles(&org, project.Project{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatal("absent role metadata changed setup")
	}
}

func TestProjectRoleMetadataResolvesExistingParentAndPreservesIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "roles/OwnedChild.role")
	writeFile(t, path, `<Role><name>Owned Child</name><parentRole>CEO</parentRole></Role>`)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	p := project.Project{RoleFiles: []string{path}}
	if err := ApplyProjectRoles(&org, p); err != nil {
		t.Fatal(err)
	}
	var child storage.Record
	for _, r := range org.Objects["UserRole"].Records {
		if r.Fields["DeveloperName"].String == "OwnedChild" {
			child = r
		}
	}
	if child.ID == "" || child.Fields["ParentRoleId"].ID != "00E000000000001" {
		t.Fatalf("existing parent resolution: %+v", child)
	}
	writeFile(t, path, `<Role><name>Renamed Label</name><parentRole>CEO</parentRole></Role>`)
	if err := ApplyProjectRoles(&org, p); err != nil {
		t.Fatal(err)
	}
	if org.Objects["UserRole"].Records[child.ID].Fields["Name"].String != "Renamed Label" || len(org.Objects["UserRole"].Records) != 2 {
		t.Fatal("label update lost developer identity")
	}
}

func TestProjectRoleMetadataFailureDoesNotPartiallyLoad(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "roles/OwnedChild.role-meta.xml")
	writeFile(t, path, `<Role><name>Owned Child</name><parentRole>UnavailableParent</parentRole></Role>`)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	before := org.Clone()
	if err := ApplyProjectRoles(&org, project.Project{RoleFiles: []string{path}}); err == nil {
		t.Fatal("unresolved parent was accepted")
	}
	if !reflect.DeepEqual(before.Objects, org.Objects) || !reflect.DeepEqual(before.IDSequences, org.IDSequences) {
		t.Fatal("failed role load changed setup")
	}
}

func TestProjectRoleMetadataProjectsDeclaredLowerAccess(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "roles/ProbeA.role-meta.xml")
	b := filepath.Join(root, "roles/ProbeB.role-meta.xml")
	c := filepath.Join(root, "roles/ProbeC.role-meta.xml")
	writeFile(t, a, `<Role><caseAccessLevel>Edit</caseAccessLevel><contactAccessLevel>Edit</contactAccessLevel><description>top</description><mayForecastManagerShare>false</mayForecastManagerShare><name>Probe A</name><opportunityAccessLevel>Edit</opportunityAccessLevel></Role>`)
	writeFile(t, b, `<Role><caseAccessLevel>Read</caseAccessLevel><contactAccessLevel>Edit</contactAccessLevel><mayForecastManagerShare>false</mayForecastManagerShare><name>Probe B</name><opportunityAccessLevel>Read</opportunityAccessLevel><parentRole>ProbeA</parentRole></Role>`)
	writeFile(t, c, `<Role><caseAccessLevel>None</caseAccessLevel><contactAccessLevel>Edit</contactAccessLevel><mayForecastManagerShare>false</mayForecastManagerShare><name>Probe C</name><opportunityAccessLevel>Edit</opportunityAccessLevel><parentRole>ProbeB</parentRole></Role>`)
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	if err := ApplyProjectRoles(&org, project.Project{RoleFiles: []string{a, b, c}}); err != nil {
		t.Fatal(err)
	}
	// Scratch-org rows (API 62, Case and Opportunity organization-wide defaults
	// Private): UserRole access fields equal the declared levels.
	want := map[string][3]string{
		"ProbeA": {"Edit", "Edit", "Edit"},
		"ProbeB": {"Read", "Edit", "Read"},
		"ProbeC": {"None", "Edit", "Edit"},
	}
	ids := map[string]storage.ID{}
	for id, record := range org.Objects["UserRole"].Records {
		ids[record.Fields["DeveloperName"].String] = id
	}
	for name, levels := range want {
		record := org.Objects["UserRole"].Records[ids[name]]
		got := [3]string{record.Fields["CaseAccessForAccountOwner"].String, record.Fields["ContactAccessForAccountOwner"].String, record.Fields["OpportunityAccessForAccountOwner"].String}
		if got != levels {
			t.Fatalf("%s access = %v, want %v", name, got, levels)
		}
	}
	if org.Objects["UserRole"].Records[ids["ProbeC"]].Fields["ParentRoleId"].ID != ids["ProbeB"] {
		t.Fatalf("ProbeC parent link missing")
	}
}

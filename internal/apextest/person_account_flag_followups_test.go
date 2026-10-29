package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native explicit-person rows in testdata/conformance/lookup_filter_followups/org.tsv
// preserve the selected person record type. The metadata provenance that enables
// that behavior must survive merging sources, regardless of their order.
func TestPersonAccountFlagProfileFollowups(t *testing.T) {
	for _, names := range personAccountFlagFollowupNames() {
		for _, personFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/personFirst=%v", names.name, personFirst), func(t *testing.T) {
				original := personAccountFlagFollowupRecordType(names.developer, names.label)
				org := personAccountFlagFollowupOrg(original)
				count := len(org.Objects["Account"].Definition.RecordTypes)
				root := t.TempDir()
				accountFile := filepath.Join(root, "Business.profile-meta.xml")
				personFile := filepath.Join(root, "Person.profile-meta.xml")
				writeFile(t, accountFile, personAccountFlagFollowupVisibility("Profile", "Account."+names.reference, true))
				writeFile(t, personFile, personAccountFlagFollowupVisibility("Profile", "PersonAccount."+names.reference, true))
				files := []string{accountFile, personFile}
				if personFirst {
					files[0], files[1] = files[1], files[0]
				}
				for _, file := range files {
					changed := applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: []string{file}})
					if changed != (file == personFile) {
						t.Fatalf("changed for %s = %v; want %v", filepath.Base(file), changed, file == personFile)
					}
				}
				personAccountFlagFollowupCheck(t, org, original, true, count)
				if applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: files}) {
					t.Fatal("reapplying visibility should not change the record type")
				}
				personAccountFlagFollowupCheck(t, org, original, true, count)
			})
		}
	}
}

func TestPersonAccountFlagPermissionSetAfterFoldFollowups(t *testing.T) {
	for _, names := range personAccountFlagFollowupNames() {
		t.Run(names.name, func(t *testing.T) {
			original := personAccountFlagFollowupRecordType(names.developer, names.label)
			org := personAccountFlagFollowupOrg(original)
			count := len(org.Objects["Account"].Definition.RecordTypes) + 1
			org.Objects["PersonAccount"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "PersonAccount", RecordTypes: []storage.RecordTypeInfo{{DeveloperName: "Unrelated", Name: "Unrelated", Active: true, Available: true}},
			}}
			foldPersonAccountObjectIntoAccount(&org)
			personAccountFlagFollowupCheck(t, org, original, false, count)
			file := filepath.Join(t.TempDir(), "Care.permissionset-meta.xml")
			writeFile(t, file, personAccountFlagFollowupVisibility("PermissionSet", "PersonAccount."+names.reference, true))
			p := project.Project{PermissionSetFiles: []string{file}}
			applyProjectPermissionSetRecords(&org, p, nil)
			personAccountFlagFollowupCheck(t, org, original, true, count)
			applyProjectPermissionSetRecords(&org, p, nil)
			personAccountFlagFollowupCheck(t, org, original, true, count)
		})
	}
}

func TestPersonAccountFlagFoldFollowups(t *testing.T) {
	for _, names := range personAccountFlagFollowupNames() {
		t.Run(names.name, func(t *testing.T) {
			original := personAccountFlagFollowupRecordType(names.developer, names.label)
			org := personAccountFlagFollowupOrg(original)
			count := len(org.Objects["Account"].Definition.RecordTypes)
			org.Objects["PersonAccount"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "PersonAccount", RecordTypes: []storage.RecordTypeInfo{{DeveloperName: names.reference, Name: "Incoming", Active: true, Available: true}},
			}}
			foldPersonAccountObjectIntoAccount(&org)
			personAccountFlagFollowupCheck(t, org, original, true, count)
			if _, exists := org.Objects["PersonAccount"]; exists {
				t.Fatal("PersonAccount metadata object survived folding")
			}
			foldPersonAccountObjectIntoAccount(&org)
			personAccountFlagFollowupCheck(t, org, original, true, count)
		})
	}
}

func TestPersonAccountFlagDeveloperIdentityFollowups(t *testing.T) {
	for _, name := range []string{"Care", "Member", "Provider"} {
		t.Run(name, func(t *testing.T) {
			business := personAccountFlagFollowupRecordType("Business", name)
			person := personAccountFlagFollowupRecordType(name, name+" Label")
			person.ID = "012000000000002"
			org := personAccountFlagFollowupOrg(business)
			account := org.Objects["Account"]
			account.Definition.RecordTypes = append(account.Definition.RecordTypes, person)
			org.Objects["Account"] = account
			org.Objects["PersonAccount"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "PersonAccount", RecordTypes: []storage.RecordTypeInfo{{DeveloperName: name, Name: name + " Label"}},
			}}
			foldPersonAccountObjectIntoAccount(&org)
			for _, rt := range org.Objects["Account"].Definition.RecordTypes {
				if rt.DeveloperName == "Business" && rt.PersonAccount {
					t.Fatal("label collision promoted a business record type")
				}
				if rt.DeveloperName == name && !rt.PersonAccount {
					t.Fatal("exact developer identity was not promoted")
				}
			}
		})
	}
}

func TestPersonAccountFlagVisibilityBoundariesFollowups(t *testing.T) {
	for _, source := range []string{"profile", "permissionSet", "profilePermissions"} {
		for _, tc := range []struct {
			name, object           string
			visible, initial, want bool
		}{
			{"business", "Account", true, false, false},
			{"invisible", "PersonAccount", false, false, false},
			{"visiblePerson", "PersonAccount", true, false, true},
			{"businessPreservesPerson", "Account", true, true, true},
			{"invisiblePreservesPerson", "PersonAccount", false, true, true},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				original := personAccountFlagFollowupRecordType("Care", "Care Label")
				original.PersonAccount = tc.initial
				org := personAccountFlagFollowupOrg(original)
				count := len(org.Objects["Account"].Definition.RecordTypes)
				file := filepath.Join(t.TempDir(), "Care.profile-meta.xml")
				root := "Profile"
				if source == "permissionSet" {
					file = filepath.Join(filepath.Dir(file), "Care.permissionset-meta.xml")
					root = "PermissionSet"
				}
				writeFile(t, file, personAccountFlagFollowupVisibility(root, tc.object+".Care", tc.visible))
				switch source {
				case "profile":
					if changed := applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: []string{file}}); changed != (tc.want && !tc.initial) {
						t.Fatalf("visibility changed = %v; want %v", changed, tc.want && !tc.initial)
					}
				case "permissionSet":
					applyProjectPermissionSetRecords(&org, project.Project{PermissionSetFiles: []string{file}}, nil)
				case "profilePermissions":
					storage.EnsureStandardObject(&org, "Profile")
					applyProjectProfileRecords(&org, project.Project{ProfileFiles: []string{file}}, nil)
				}
				personAccountFlagFollowupCheck(t, org, original, tc.want, count)
			})
		}
	}
}

func TestPersonAccountFlagSerializedFollowups(t *testing.T) {
	original := personAccountFlagFollowupRecordType("Care", "Care Label")
	org := personAccountFlagFollowupOrg(original)
	count := len(org.Objects["Account"].Definition.RecordTypes)
	root := t.TempDir()
	personFile := filepath.Join(root, "Person.profile-meta.xml")
	accountFile := filepath.Join(root, "Business.profile-meta.xml")
	writeFile(t, personFile, personAccountFlagFollowupVisibility("Profile", "PersonAccount.Care", true))
	writeFile(t, accountFile, personAccountFlagFollowupVisibility("Profile", "Account.Care", true))
	applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: []string{personFile}})
	personAccountFlagFollowupCheck(t, org, original, true, count)
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	personAccountFlagFollowupCheck(t, restored, original, true, count)
	if applyProjectProfileRecordTypes(&restored, project.Project{ProfileFiles: []string{accountFile}}) {
		t.Fatal("Account visibility changed restored person record type")
	}
	personAccountFlagFollowupCheck(t, restored, original, true, count)
}

// Identity boundaries are captured in person_record_type_identity/org.tsv.
func personAccountFlagFollowupNames() []struct{ name, developer, label, reference string } {
	return []struct{ name, developer, label, reference string }{
		{"exact", "Care", "Care Label", "Care"},
		{"memberExact", "Member", "Member Label", "Member"},
		{"providerExact", "Provider", "Provider Label", "Provider"},
	}
}

func personAccountFlagFollowupRecordType(developer, label string) storage.RecordTypeInfo {
	return storage.RecordTypeInfo{
		ID: "012000000000101AAA", DeveloperName: developer, Name: label,
		Active: true, Available: true, Default: true, Description: "Existing metadata",
		PicklistDefaults: map[string]string{"Rating": "Warm"},
	}
}

func personAccountFlagFollowupOrg(recordType storage.RecordTypeInfo) storage.OrgState {
	definition := storage.ObjectDefinition{APIName: "Account"}
	storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	definition.RecordTypes = append(definition.RecordTypes, recordType)
	return storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {Definition: definition},
	}}
}

func personAccountFlagFollowupVisibility(root, reference string, visible bool) string {
	return fmt.Sprintf(`<%s xmlns="http://soap.sforce.com/2006/04/metadata"><recordTypeVisibilities><default>false</default><recordType>%s</recordType><visible>%v</visible></recordTypeVisibilities></%s>`, root, reference, visible, root)
}

func personAccountFlagFollowupCheck(t *testing.T, org storage.OrgState, original storage.RecordTypeInfo, person bool, count int) {
	t.Helper()
	recordTypes := org.Objects["Account"].Definition.RecordTypes
	if len(recordTypes) != count {
		t.Fatalf("record types = %#v; want %d", recordTypes, count)
	}
	want := original
	want.PersonAccount = person
	for _, got := range recordTypes {
		if got.DeveloperName == original.DeveloperName {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("record type = %#v; want %#v", got, want)
			}
			return
		}
	}
	t.Fatalf("missing original record type: %#v", recordTypes)
}

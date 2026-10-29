package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// This preserves the pre-existing non-Account metadata projection. The owned
// person identity oracle applies to Account and PersonAccount, not these objects.
func TestNonAccountRecordTypeLegacyProjectionFollowups(t *testing.T) {
	for _, objectName := range []string{"pkg__Birch__c", "other__Cedar__c", "Oak__c"} {
		for _, route := range []string{"profile", "permissionSet", "profilePermissions"} {
			for _, existing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/existing=%v", objectName, route, existing), func(t *testing.T) {
					org := storage.OrgState{Objects: map[string]storage.ObjectState{
						objectName: {Definition: storage.ObjectDefinition{APIName: objectName}},
					}}
					if existing {
						state := org.Objects[objectName]
						state.Definition.RecordTypes = []storage.RecordTypeInfo{{DeveloperName: "Birch", Name: "Birch"}}
						org.Objects[objectName] = state
					}
					root, suffix := "Profile", ".profile-meta.xml"
					if route == "permissionSet" {
						root, suffix = "PermissionSet", ".permissionset-meta.xml"
					}
					file := filepath.Join(t.TempDir(), "Projection"+suffix)
					writeFile(t, file, personAccountFlagFollowupVisibility(root, objectName+".pkg__Birch", true))
					switch route {
					case "profile":
						applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: []string{file}})
					case "permissionSet":
						applyProjectPermissionSetRecords(&org, project.Project{PermissionSetFiles: []string{file}}, nil)
					case "profilePermissions":
						storage.EnsureStandardObject(&org, "Profile")
						applyProjectProfileRecords(&org, project.Project{ProfileFiles: []string{file}}, nil)
					}
					rts := org.Objects[objectName].Definition.RecordTypes
					if len(rts) != 1 || rts[0].DeveloperName != "Birch" {
						t.Fatalf("record types = %#v; want the legacy unqualified identity", rts)
					}
				})
			}
		}
	}
}

func TestNonAccountRecordTypeLegacyDefaultsFollowups(t *testing.T) {
	for _, objectName := range []string{"pkg__Birch__c", "other__Cedar__c", "Oak__c"} {
		file := filepath.Join(t.TempDir(), "Projection.profile-meta.xml")
		source := personAccountFlagFollowupVisibility("Profile", objectName+".pkg__Birch", true)
		writeFile(t, file, strings.Replace(source, "<default>false</default>", "<default>true</default>", 1))
		if got := loadProfileRecordTypeDefaults(file)[objectName]; got != "Birch" {
			t.Errorf("%s default = %q; want Birch", objectName, got)
		}
	}
}

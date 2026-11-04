package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Fresh project metadata establishes person family before synthetic rows are seeded.
// Native profile person controls: case349_profile_boundary/org.tsv:26-37,62-73.
func TestCase349FreshProfileFamilyBeforeSeeding38a(t *testing.T) {
	const fixture = "../../testdata/conformance/case349_profile_boundary/force-app/main/default"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			for _, p := range []string{"objects/Account/fields/ProbeKey__c.field-meta.xml", "objects/Account/recordTypes/Business_Default.recordType-meta.xml", "objects/PersonAccount/recordTypes/Provider.recordType-meta.xml", "profiles/Person Probe.profile-meta.xml"} {
				writeFile(t, filepath.Join(base, p), readFixtureFile(t, filepath.Join(fixture, p)))
			}
			// Account and PersonAccount metadata may refer to the same type.
			business := readFixtureFile(t, filepath.Join(fixture, "objects/Account/recordTypes/Business_Default.recordType-meta.xml"))
			writeFile(t, filepath.Join(base, "objects/Account/recordTypes/Provider.recordType-meta.xml"), strings.ReplaceAll(business, "Business_Default", "Provider"))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := OrgFromIndex(index)
			found := false
			for _, rt := range org.Objects["Account"].Definition.RecordTypes {
				if rt.DeveloperName == "Provider" {
					found = true
					if !rt.PersonAccount {
						t.Errorf("fresh profile lost person family: %#v", rt)
					}
					row, ok := org.Objects["RecordType"].Records[rt.ID]
					if !ok || !row.Fields["IsPersonType"].Boolean {
						t.Errorf("fresh seeded row lost person family: %#v", row)
					}
				}
			}
			if !found {
				t.Fatal("profile record type not loaded")
			}
		})
	}
}

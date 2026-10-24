package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// A project retrieved from an org with Person Accounts carries the
// PersonAccount object's record types. Without the scratch-definition feature
// the source still compiles name fields on Account; without that evidence the
// native rejection in person_account_without_feature still applies.
func TestPersonAccountRetrievedRecordTypeEnablesNameFields(t *testing.T) {
	const account = `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`
	const recordType = `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>Individual</label></RecordType>`
	const class = `public class PersonUse { public static Account build() { return new Account(FirstName = 'Ari', LastName = 'Lee'); } }`
	for _, tc := range []struct {
		name       string
		recordType bool
		wantErrors bool
	}{
		{"with retrieved record type", true, false},
		{"without evidence", false, true},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), account)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/PersonUse.cls"), class)
				if tc.recordType {
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml"), recordType)
				}
				result := sema.Analyze(loadTestIndex(t, root))
				if result.HasErrors() != tc.wantErrors {
					t.Fatalf("HasErrors = %v, want %v; diagnostics=%v", result.HasErrors(), tc.wantErrors, result.Diagnostics)
				}
			})
		}
	}
}

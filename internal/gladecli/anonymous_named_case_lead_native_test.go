package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnonymousNamedCaseLeadAccessNativeConformance(t *testing.T) {
	const base = "testdata/conformance/anonymous_named_case_lead_access"
	rows := readCriteriaTSV(t, filepath.Join(base, "org.tsv"))
	read := func(t *testing.T, name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, model := range []string{"ReadWriteTransfer", "ReadWrite"} {
		variant := "public_read_write"
		if model == "ReadWriteTransfer" {
			variant = "public_read_write_transfer"
		}
		t.Run(model, func(t *testing.T) {
			root := t.TempDir()
			project := func(api string) {
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			}
			project("67.0")
			writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites]\n")
			writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), read(t, "project-scratch-def.json"))
			for _, object := range []string{"Case", "Lead"} {
				rel := filepath.Join("objects", object, object+".object-meta.xml")
				source := filepath.Join(criteriaSharingFixtures, "external_user_case_lead_sharing", variant, rel)
				data, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default", rel), string(data))
			}

			setup := func(t *testing.T, db string) {
				t.Helper()
				for _, step := range []string{"setup-owner.apex", "setup-records.apex", "setup-readers.apex"} {
					runCriteriaExec(t, root, db, read(t, step)+"\nSystem.debug('OBS|setup');")
				}
				runCriteriaExec(t, root, db, anonymousNamedCaseLeadGuest)
			}
			db := filepath.Join(root, "state.sqlite")
			setup(t, db)

			for _, api := range []string{"62.0", "67.0"} {
				project(api)
				checked := 0
				for _, row := range rows {
					if row["model"] != model || row["api"] != api || row["route"] != "anonymous" {
						continue
					}
					predicate := map[string]string{
						"standard":  "Alias = 'rdstd'",
						"community": "Alias = 'rdcc'",
						"guest":     "UserType = 'Guest'",
					}[row["principal"]]
					if predicate == "" {
						t.Fatalf("unexpected native principal %q", row["principal"])
					}
					t.Run(api+"/"+row["principal"], func(t *testing.T) {
						source := strings.Replace(anonymousNamedCaseLeadProjection, "READER_PREDICATE", predicate, 1)
						if got := runCriteriaExec(t, root, db, source); got != row["access"] {
							t.Errorf("Glade=%q native=%q", got, row["access"])
						}
					})
					checked++
				}
				if checked != 3 {
					t.Fatalf("checked %d native rows for %s, want 3", checked, api)
				}
			}

			standard := strings.Replace(anonymousNamedCaseLeadProjection, "READER_PREDICATE", "Alias = 'rdstd'", 1)
			t.Run("extra-field", func(t *testing.T) {
				source := strings.Replace(standard, "HasEditAccess FROM", "HasEditAccess, HasDeleteAccess FROM", 1)
				if got := runCriteriaExec(t, root, db, source); got != "-|-|-" {
					t.Errorf("extra projection changed: %q", got)
				}
			})
			t.Run("api-68", func(t *testing.T) {
				project("68.0")
				if got := runCriteriaExec(t, root, db, standard); got != "-|-|-" {
					t.Errorf("API 68 projection changed: %q", got)
				}
			})
			t.Run("community-private-case", func(t *testing.T) {
				project("67.0")
				path := filepath.Join(root, "force-app/main/default/objects/Case/Case.object-meta.xml")
				metadata, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				private := strings.Replace(string(metadata), ">"+model+"<", ">Private<", 1)
				if private == string(metadata) {
					t.Fatal("missing internal Case sharing model")
				}
				writeTestFile(t, path, private)
				db := filepath.Join(root, "private.sqlite")
				setup(t, db)
				community := strings.Replace(anonymousNamedCaseLeadProjection, "READER_PREDICATE", "Alias = 'rdcc'", 1)
				if got := runCriteriaExec(t, root, db, community); got != "-|-|00" {
					t.Errorf("private Case projection changed: %q", got)
				}
			})
		})
	}
}

const anonymousNamedCaseLeadGuest = `
Id profileId = [SELECT Id FROM Profile WHERE UserType = 'Guest' LIMIT 1].Id;
insert new User(Alias='rdgst', Email='guest@example.com', EmailEncodingKey='UTF-8', LastName='Sharing Probe Guest',
    LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles',
    ProfileId=profileId, Username='rdgst.'+Crypto.getRandomLong()+'@example.com');
System.debug('OBS|setup');
`

const anonymousNamedCaseLeadProjection = `
User reader = [SELECT Id FROM User WHERE READER_PREDICATE LIMIT 1];
List<Id> records = new List<Id>{
    [SELECT Id FROM Case WHERE Subject='Sharing Probe Other' LIMIT 1].Id,
    [SELECT Id FROM Case WHERE Subject='Sharing Probe Contact Case' LIMIT 1].Id,
    [SELECT Id FROM Lead WHERE LastName='Sharing Probe Lead' LIMIT 1].Id
};
List<String> results = new List<String>();
for (Id recordId : records) {
    List<UserRecordAccess> access = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :reader.Id AND RecordId = :recordId];
    results.add(access.isEmpty() ? '-' : (access[0].HasReadAccess ? '1' : '0') + (access[0].HasEditAccess ? '1' : '0'));
}
System.debug('OBS|' + String.join(results, '|'));
`

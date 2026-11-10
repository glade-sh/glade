package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

const recordKeyPrefixFixture = "testdata/native_conformance/record_key_prefix"

func readRecordKeyPrefixRows(t *testing.T) [][]string {
	t.Helper()
	file, err := os.Open(filepath.Join(recordKeyPrefixFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,case,route,outcome" {
		t.Fatalf("native header = %v", rows[0])
	}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[3] != "Pass" {
			t.Fatalf("invalid native row: %v", row)
		}
	}
	return rows[1:]
}

func writeRecordKeyPrefixProject(t *testing.T, root, api string, classes ...string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	object, err := os.ReadFile(filepath.Join(recordKeyPrefixFixture, "project/objects/PrefixProbe__c/PrefixProbe__c.object-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/PrefixProbe__c/PrefixProbe__c.object-meta.xml"), string(object))
	for _, className := range classes {
		for _, ext := range []string{".cls", ".cls-meta.xml"} {
			body, err := os.ReadFile(filepath.Join(recordKeyPrefixFixture, "project/classes", className+ext))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", className+ext), string(body))
		}
	}
}

// The captured @IsTest rows run unchanged on the local test runner.
func TestRecordKeyPrefixNativeRowsLocalRunner(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range readRecordKeyPrefixRows(t) {
		api, method, route := row[0], row[1], row[2]
		if route != "IsTest" {
			continue
		}
		key := api + "/" + method
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			className := "PrefixCapture" + strings.TrimSuffix(api, ".0")
			writeRecordKeyPrefixProject(t, root, api, className)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{className}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("native test route: %+v", run.Summary())
			}
		})
	}
	if len(seen) != 9 {
		t.Fatalf("incomplete native matrix: %v", seen)
	}
}

// Imported records keep their source-org Ids, and a fixture org Id replaces
// the seeded one. The captured invariants (one Organization row whose Id is the
// org Id; a record Id's prefix is its object's describe key prefix and resolves
// to that object) must hold for imported data, not only for local inserts.
func TestImportedRecordsKeepNativeIdentityInvariants(t *testing.T) {
	const orgID = "00D000000000002EAA"
	const importedID = "a0Q000000000001EAA"
	for _, row := range readRecordKeyPrefixRows(t) {
		api := row[0]
		if row[2] != "IsTest" || row[1] != "customPrefixMatchesDescribe" {
			continue
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeRecordKeyPrefixProject(t, root, api)
			index := loadTestIndex(t, root)

			org := storage.NewOrgState()
			org.APIVersion = api
			// The local schema assigns its own prefix; the import carries the
			// source org's prefix in the record Id.
			org.Objects["PrefixProbe__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "PrefixProbe__c", Label: "Prefix Probe", KeyPrefix: "a0X", Fields: map[string]storage.Field{
					"Id":   {APIName: "Id", Type: storage.FieldID},
					"Name": {APIName: "Name", Type: storage.FieldString},
				}},
				Records: map[storage.ID]storage.Record{},
			}
			storage.EnsureDeterministicPlatformData(&org)
			fixture := storage.Fixture{
				Version: storage.FixtureVersion,
				Org:     storage.FixtureOrg{OrgID: orgID},
				Objects: []storage.FixtureObject{{Name: "PrefixProbe__c", Records: []storage.FixtureRecord{
					{ID: importedID, Fields: map[string]storage.Value{"Name": storage.StringValue("imported")}},
				}}},
			}
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			storage.EnsureDeterministicPlatformData(&org)

			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			checks := map[string]string{
				"organization": `
List<Organization> rows = [SELECT Id FROM Organization];
System.assertEquals(1, rows.size());
System.assertEquals(UserInfo.getOrganizationId(), rows[0].Id);
`,
				"keyPrefix": `
PrefixProbe__c probe = [SELECT Id FROM PrefixProbe__c WHERE Name = 'imported'];
System.assertEquals(PrefixProbe__c.SObjectType.getDescribe().getKeyPrefix(), String.valueOf(probe.Id).left(3));
System.assertEquals(PrefixProbe__c.SObjectType, probe.Id.getSObjectType());
System.assertEquals(PrefixProbe__c.SObjectType, Id.valueOf(String.valueOf(probe.Id)).getSObjectType());
`,
			}
			for _, name := range []string{"organization", "keyPrefix"} {
				t.Run(name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(checks[name], vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

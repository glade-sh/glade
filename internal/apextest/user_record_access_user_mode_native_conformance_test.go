package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
)

func TestUserRecordAccessUserModeNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/user_record_access_user_mode"
	var data struct {
		APIVersions []string `json:"apiVersions"`
		ClassName   string   `json:"className"`
		MethodName  string   `json:"methodName"`
		SourceFile  string   `json:"sourceFile"`
		Cases       []struct {
			ID       string `json:"id"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.ClassName != "RecordAccessModeProbe" || data.MethodName != "observe" || len(data.Cases) != 2 {
		t.Fatal("incomplete native case table")
	}
	source, err := os.ReadFile(filepath.Join(fixture, data.SourceFile))
	if err != nil {
		t.Fatal(err)
	}
	oracleFile, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(oracleFile)
	reader.Comma = '\t'
	oracleRows, readErr := reader.ReadAll()
	closeErr := oracleFile.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(oracleRows) != 5 || strings.Join(oracleRows[0], "|") != "source_api|case_id|route|observation" {
		t.Fatalf("native oracle rows = %d", len(oracleRows))
	}
	oracle := map[string]map[string]string{}
	for _, row := range oracleRows[1:] {
		if len(row) != 4 || row[2] != "istest" || row[3] == "" {
			t.Fatalf("invalid native oracle row: %v", row)
		}
		if oracle[row[0]] == nil {
			oracle[row[0]] = map[string]string{}
		}
		if _, duplicate := oracle[row[0]][row[1]]; duplicate {
			t.Fatalf("duplicate native oracle row: %v", row)
		}
		oracle[row[0]][row[1]] = row[3]
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, testCase := range data.Cases {
				if oracle[api][testCase.ID] != testCase.Expected || testCase.Expected != "ok|true|1" {
					t.Fatalf("invalid captured result for %s/%s: %q", api, testCase.ID, oracle[api][testCase.ID])
				}
			}
			fixtureData := userModeInaccessibleFixture{Metadata: map[string]string{}}
			index := userModeInaccessibleIndex(t, api, fixtureData, map[string]string{data.ClassName: string(source)})
			if index.HasErrors() {
				t.Fatalf("native class accepted; Glade diagnostics=%v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("native class accepted; Glade diagnostics=%v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			seedUserRecordAccessProbe(t, &org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			cases := Discover(index, Options{SelectedClasses: []string{data.ClassName}, SelectedMethod: data.MethodName})
			if len(cases) != 1 {
				t.Fatal("missing test method")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil {
				t.Fatal(methodErrors[key])
			}
			if programErrors[key] != nil {
				t.Fatal(programErrors[key])
			}
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			_, execErr := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
			if execErr != nil {
				t.Fatalf("native queries returned ok|true|1; Glade=%v", execErr)
			}
		})
	}
}

func seedUserRecordAccessProbe(t *testing.T, org *storage.OrgState) {
	t.Helper()
	var profile storage.ID
	for id, record := range org.Objects["Profile"].Records {
		if record.Fields["Name"].String == "Minimum Access - Salesforce" {
			profile = id
		}
	}
	if profile == "" {
		t.Fatal("missing restricted profile")
	}
	const actor storage.ID = "005000000000099"
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{
		{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{
			"Username":  storage.StringValue("synthetic.record.access.20261019@example.invalid"),
			"ProfileId": storage.IDValue(profile),
			"LastName":  storage.StringValue("Synthetic Probe"),
		}}}},
		{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: "0PS000000000099", Fields: map[string]storage.Value{
			"Name":  storage.StringValue("Record_Access_Probe"),
			"Label": storage.StringValue("Record Access Probe"),
		}}}},
		{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue("0PS000000000099"),
			"SObjectType":     storage.StringValue("Account"),
			"PermissionsRead": storage.BooleanValue(true),
		}}}},
		{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue(actor),
			"PermissionSetId": storage.IDValue("0PS000000000099"),
		}}}},
		{Name: "Account", Records: []storage.FixtureRecord{{ID: "001000000000098", Fields: map[string]storage.Value{
			"Name":    storage.StringValue("Synthetic record access target"),
			"OwnerId": storage.IDValue(storage.DefaultUserID),
		}}}},
	}
	if err := storage.ApplyFixture(org, fixture); err != nil {
		t.Fatal(err)
	}
	accounts := org.Objects["Account"]
	for id, record := range accounts.Records {
		if owner, ok := record.GetField("OwnerId"); ok {
			record.System.OwnerID = owner.ID
			accounts.Records[id] = record
		}
	}
	org.Objects["Account"] = accounts
}

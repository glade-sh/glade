package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Standard User running under System.runAs writes a custom field of a
// standard object (Account) that no permission grants. Native rows: explicit
// USER_MODE denies at API 62.0 and 67.0; the default mode denies only from
// source API 67.0; SYSTEM_MODE and a granted field always save. Denials are a
// statement DmlException or a failed SaveResult, never a SecurityException.
func TestStandardCustomFieldPolicyNativeConformance(t *testing.T) {
	runStandardCustomFieldPolicy(t, "standard_custom_field_policy", "PolicyProbe", 72)
}

func TestStandardRequiredCustomFieldNativeConformance(t *testing.T) {
	runStandardCustomFieldPolicy(t, "standard_required_custom_field", "RequiredProbe", 3)
}

func TestStandardCustomFieldScopedNativeConformance(t *testing.T) {
	runStandardCustomFieldPolicy(t, "standard_custom_field_scoped", "Probe", 15)
}

// The default test-runner administrator, without System.runAs, writes and
// queries a custom field of a standard object that no permission metadata
// grants (Probe_B__c). Native rows: API 62.0 saves and reads in system mode;
// API 67.0 fails every insert and update with the statement DmlException or a
// failed SaveResult, and a dynamic SELECT reports no such column.
//
// Only the API 62.0 rows are asserted. The API 67.0 rows stay parked: existing
// native routes (aggregate_namespace_projection, flow_partial_rows) write and
// query ungranted Account custom fields as the default administrator at 67.0
// and pass, and no fixture fact separates them from this capture. Rows for
// Probe_A__c (granted by an unassigned deployed set) and the rows whose
// in-test PermissionSetAssignment raises MIXED_DML_OPERATION are parked too.
func TestStandardCustomFieldAdminNativeConformance(t *testing.T) {
	runStandardCustomFieldPolicyRows(t, "standard_custom_field_admin", "FlsTest", 48, func(api, method, want string) bool {
		return api == "62.0" && (strings.HasPrefix(method, "aB_") || strings.HasPrefix(method, "pB_") && !strings.Contains(want, "MIXED_DML_OPERATION"))
	})
}

func runStandardCustomFieldPolicy(t *testing.T, name, prefix string, count int) {
	t.Helper()
	runStandardCustomFieldPolicyRows(t, name, prefix, count, func(string, string, string) bool { return true })
}

func runStandardCustomFieldPolicyRows(t *testing.T, name, prefix string, count int, asserted func(api, method, want string) bool) {
	t.Helper()
	family := filepath.Join("testdata/conformance", name)
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+count || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %d", len(rows))
	}
	root := t.TempDir()
	project := filepath.Join(family, "project")
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	for i, row := range rows[1:] {
		api, method, want := row[0], row[1], row[2]
		if !asserted(api, method, want) {
			continue
		}
		class := prefix + strings.TrimSuffix(api, ".0")
		line := i + 2
		t.Run(api+"/"+method, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing runner observation %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			_, got, ok := strings.Cut(message, " => ")
			if !ok {
				_, got, ok = strings.Cut(message, "Assertion Failed: ")
			}
			if !ok || strings.TrimSpace(got) != strings.TrimSpace(want) {
				t.Fatalf("NATIVE %s org.tsv:%d istest %s\nnative=%q\nglade =%q", method, line, api, want, message)
			}
		})
	}
}

// Shapes next to the captured administrator rows keep their earlier behavior:
// default-mode DML and dynamic SOQL at API 67.0 (parked, see above), the
// administrator's all-or-none and list USER_MODE inserts, a custom
// field of another standard object, Account in a Person Account org,
// a field that a deployed (unassigned) permission set grants, static SOQL, an
// explicit SYSTEM_MODE query and an API 62.0 explicit USER_MODE update all
// save or read as before, and a Standard User's ungranted Case custom field
// keeps its SecurityException, as do its scoped partial insert, a Standard
// User's ungranted Account Number field, explicit "upsert as user" and a
// scoped partial list insert. Only the captured shapes change.
func TestStandardCustomFieldAdminUncapturedNeighbors(t *testing.T) {
	project := filepath.Join("testdata/conformance/standard_custom_field_admin/project/force-app/main/default")
	field := func(name string) string {
		body, err := os.ReadFile(filepath.Join(project, "objects/Account/fields", name+".field-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	permissionSet, err := os.ReadFile(filepath.Join(project, "permissionsets/Probe_Access.permissionset-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, api, features, body string
	}{
		{"case_field", "67.0", "", `Case c = new Case(Subject='x'); c.put('Probe_C__c','v'); insert c; c.put('Probe_C__c','w'); update c; System.assertEquals('w', (String) Database.query('SELECT Probe_C__c FROM Case WHERE Id = \'' + c.Id + '\'')[0].get('Probe_C__c'));`},
		{"default_mode_admin", "67.0", "", `Account a = new Account(Name='x', Probe_B__c='v'); insert a; a.Probe_B__c='w'; update a; System.assertEquals(true, Database.insert(new Account(Name='y', Probe_B__c='v'), false).isSuccess()); System.assertEquals('w', (String) Database.query('SELECT Probe_B__c FROM Account WHERE Id = \'' + a.Id + '\'')[0].get('Probe_B__c'));`},
		{"admin_user_mode_all_or_none_and_list", "67.0", "", `Account a = new Account(Name='x', Probe_B__c='v'); System.assertEquals(true, Database.insert(a, true, AccessLevel.USER_MODE).isSuccess()); List<Database.SaveResult> rs = Database.insert(new List<Account>{new Account(Name='y', Probe_B__c='v')}, false, AccessLevel.USER_MODE); System.assertEquals(true, rs[0].isSuccess());`},
		{"standard_user_number_field", "67.0", ``, `Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User']; User u = new User(Alias='su3', Email='su@example.invalid', EmailEncodingKey='UTF-8', LastName='T', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='su3@example.invalid.glade'); insert u; String got = 'none'; System.runAs(u) { Account a = new Account(Name='x'); a.put('Probe_N__c', 5); try { insert a; } catch (Exception e) { got = e.getTypeName(); } } System.assertEquals('System.SecurityException', got);`},
		{"standard_user_explicit_upsert", "67.0", ``, `Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User']; User u = new User(Alias='su3', Email='su@example.invalid', EmailEncodingKey='UTF-8', LastName='T', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='su3@example.invalid.glade'); insert u; String got = 'none'; System.runAs(u) { Account a = new Account(Name='x', Probe_B__c='v'); try { upsert as user a; } catch (Exception e) { got = e.getTypeName(); } } System.assertEquals('System.SecurityException', got);`},
		{"scoped_standard_user_list", "67.0", `,"features":["ApexUserModeWithPermset"]`, `Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User']; User u = new User(Alias='su3', Email='su@example.invalid', EmailEncodingKey='UTF-8', LastName='T', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='su3@example.invalid.glade'); insert u; String got = 'none'; Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id; System.runAs(u) { try { Database.insert(new List<Account>{new Account(Name='x', Probe_B__c='v')}, false, AccessLevel.USER_MODE.withPermissionSetId(ps)); got = 'returned'; } catch (Exception e) { got = e.getTypeName(); } } System.assertEquals('System.SecurityException', got);`},
		{"person_account_org", "67.0", `,"features":["PersonAccounts"]`, `Account a = new Account(Name='x', Probe_B__c='v'); insert a; a.Probe_B__c='w'; update a; System.assertEquals('w', (String) Database.query('SELECT Probe_B__c FROM Account')[0].get('Probe_B__c'));`},
		{"deployed_grant", "67.0", "", `Account a = new Account(Name='x', Probe_A__c='v'); insert a; System.assertEquals('v', (String) Database.query('SELECT Probe_A__c FROM Account')[0].get('Probe_A__c'));`},
		{"static_and_system_query", "67.0", "", `Account a = new Account(Name='x'); insert a; System.assertEquals(1, [SELECT Probe_B__c FROM Account].size()); System.assertEquals(1, Database.query('SELECT Probe_B__c FROM Account', AccessLevel.SYSTEM_MODE).size());`},
		{"api62_user_mode_update", "62.0", "", `Account a = new Account(Name='x'); insert a; a.Probe_B__c='v'; System.assertEquals(true, Database.update(a, false, AccessLevel.USER_MODE).isSuccess());`},
		{"standard_user_case_field", "67.0", "", `Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User']; User u = new User(Alias='su1', Email='su@example.invalid', EmailEncodingKey='UTF-8', LastName='T', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='su1@example.invalid.glade'); insert u; String got = 'none'; System.runAs(u) { Case c = new Case(Subject='x'); c.put('Probe_C__c','v'); try { insert c; } catch (Exception e) { got = e.getTypeName(); } } System.assertEquals('System.SecurityException', got);`},
		{"scoped_standard_user_case_field", "67.0", `,"features":["ApexUserModeWithPermset"]`, `Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User']; User u = new User(Alias='su2', Email='su@example.invalid', EmailEncodingKey='UTF-8', LastName='T', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Los_Angeles', UserName='su2@example.invalid.glade'); insert u; Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id; String got = 'none'; System.runAs(u) { Case c = new Case(Subject='x'); c.put('Probe_C__c','v'); try { Database.insert(c, false, AccessLevel.USER_MODE.withPermissionSetId(ps)); got = 'returned'; } catch (Exception e) { got = e.getTypeName(); } } System.assertEquals('System.SecurityException', got);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, tc.api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"`+tc.features+`}`)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "objects/Account/fields/Probe_A__c.field-meta.xml"), field("Probe_A__c"))
			writeFile(t, filepath.Join(base, "objects/Account/fields/Probe_B__c.field-meta.xml"), field("Probe_B__c"))
			writeFile(t, filepath.Join(base, "objects/Case/fields/Probe_C__c.field-meta.xml"), strings.ReplaceAll(field("Probe_B__c"), "Probe_B", "Probe_C"))
			writeFile(t, filepath.Join(base, "objects/Account/fields/Probe_N__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Probe_N__c</fullName><label>Probe_N</label><precision>18</precision><scale>0</scale><type>Number</type><required>false</required></CustomField>`)
			writeFile(t, filepath.Join(base, "permissionsets/Probe_Access.permissionset-meta.xml"), string(permissionSet))
			writeFile(t, filepath.Join(base, "classes/NeighborProbe.cls"), "@IsTest private class NeighborProbe { @IsTest static void run() { "+tc.body+" } }")
			writeFile(t, filepath.Join(base, "classes/NeighborProbe.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, tc.api))
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"NeighborProbe"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				var problem any
				if len(run.Suites) == 1 && len(run.Suites[0].Cases) == 1 {
					problem = run.Suites[0].Cases[0].Problem
				}
				t.Fatalf("%+v %+v", summary, problem)
			}
		})
	}
}

package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDMLPolicyDuplicateAndAssignmentKeywordAndDatabaseRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"keyword", "database", "options", "record"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				fixture := filepath.Join("..", "apextest", "testdata", "conformance", "dml_policies")
				native, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(native), "duplicate_Alert_insert_"+route+"_") {
					t.Fatal("missing native route")
				}
				loaded, err := project.Load(filepath.Join(fixture, "project"))
				if err != nil {
					t.Fatal(err)
				}
				org := storage.OrgState{Objects: map[string]storage.ObjectState{}, APIVersion: api}
				for _, object := range []string{"Account", "Contact", "Case", "Lead", "Group", "QueueSobject", "User"} {
					storage.EnsureStandardObject(&org, object)
				}
				if err := resource.ApplyProject(&org, loaded); err != nil {
					t.Fatal(err)
				}
				installDMLPolicyAutomation(t, &org, loaded)
				source := `insert new Contact(LastName='Alert',Email='route@example.invalid');
Contact row=new Contact(LastName='Alert',Email='route@example.invalid');
Database.DMLOptions options=new Database.DMLOptions();options.DuplicateRuleHeader.allowSave=true;
String actual='ok';`
				expected := "DUPLICATES_DETECTED"
				switch route {
				case "options":
					source += `Database.SaveResult result=Database.insert(row,options);if(!result.isSuccess())actual=String.valueOf(result.getErrors()[0].getStatusCode());`
					expected = "ok"
				case "database":
					source += `Database.SaveResult result=Database.insert(row,false);if(!result.isSuccess())actual=String.valueOf(result.getErrors()[0].getStatusCode());`
				case "record":
					source += `row.setOptions(options);try{insert row;}catch(DmlException error){actual=String.valueOf(error.getDmlStatusCode(0));}`
				default:
					source += `try{insert row;}catch(DmlException error){actual=String.valueOf(error.getDmlStatusCode(0));}`
				}
				source += `System.assertEquals('` + expected + `',actual);`
				machine := New(nil)
				machine.SetOrg(&org)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDMLPolicyAssignmentFalseDatabaseRoute(t *testing.T) {
	program, err := CompileAnonymous(`Database.DMLOptions options=new Database.DMLOptions();options.assignmentRuleHeader.useDefaultRule=false;Database.SaveResult result=Database.insert(new Case(Subject='Stay',Status='New',Origin='Phone'),options);System.assertEquals(true,result.isSuccess());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Case")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDMLPolicyPendingCalloutAnonymousAndMockedRoutes(t *testing.T) {
	for _, context := range []string{"anonymous", "mocked"} {
		for _, operation := range []string{"keyword", "database", "failed"} {
			t.Run(context+"/"+operation, func(t *testing.T) {
				dml := `insert new Account(Name='Pending');`
				if operation == "database" {
					dml = `Database.insert(new Account(Name='Pending'),false);`
				}
				if operation == "failed" {
					dml = `Database.insert(new Account(),false);`
				}
				source := dml + `HttpRequest request=new HttpRequest();request.setEndpoint('https://example.com');request.setMethod('GET');Boolean blocked=false;try{new Http().send(request);}catch(CalloutException error){blocked=error.getMessage()=='You have uncommitted work pending. Please commit or rollback before calling out';}System.assertEquals(true,blocked);`
				program, err := CompileAnonymous(source)
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if context == "mocked" {
					machine.EnableTestContext()
					machine.testContext.HTTPMock = Object("SyntheticResponse")
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDMLPolicyCalloutNewExecutionClearsPendingWork(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.HTTPMock = Object("SyntheticResponse")
	first, err := CompileAnonymous(`insert new Account(Name='Committed');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(first); err != nil {
		t.Fatal(err)
	}
	next, err := CompileAnonymous(`HttpRequest request=new HttpRequest();request.setEndpoint('https://example.com');request.setMethod('GET');System.assertNotEquals(null,new Http().send(request));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(next); err != nil {
		t.Fatal(err)
	}
}

func TestDMLPolicyAssignmentCaseAndLeadInsertUpdateOptionsAndRecordRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	loaded, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		native, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, object := range []string{"Case", "Lead"} {
			for _, operation := range []string{"insert", "update"} {
				for _, route := range []string{"options", "record"} {
					t.Run(api+"/"+object+"/"+operation+"/"+route, func(t *testing.T) {
						key := "assignment_" + object + "_" + operation + "_" + route + "_default\ttrue|false"
						if !strings.Contains(string(native), key+"\n") {
							t.Fatal("missing native assignment row")
						}
						org := storage.OrgState{Objects: map[string]storage.ObjectState{}, APIVersion: api}
						for _, name := range []string{"Case", "Lead", "Group", "QueueSobject"} {
							storage.EnsureStandardObject(&org, name)
						}
						storage.EnsureDeterministicPlatformData(&org)
						if err := resource.ApplyProject(&org, loaded); err != nil {
							t.Fatal(err)
						}
						installDMLPolicyAutomation(t, &org, loaded)
						if err := resource.ApplyProjectQueues(&org, loaded); err != nil {
							t.Fatal(err)
						}
						input := `new Case(Subject='Before',Status='New',Origin='Phone')`
						field := "Subject"
						if object == "Lead" {
							input = `new Lead(LastName='Synthetic',Company='Before')`
							field = "Company"
						}
						source := `SObject row=` + input + `;`
						if operation == "update" {
							source += `insert row;`
						}
						source += `row.put('` + field + `','Route');Database.DMLOptions options=new Database.DMLOptions();options.assignmentRuleHeader.useDefaultRule=true;`
						if route == "record" {
							source += `row.setOptions(options);` + operation + ` row;`
						} else {
							source += `Database.SaveResult result=Database.` + operation + `(row,options);System.assertEquals(true,result.isSuccess());`
						}
						source += `Id rowId=(Id)row.get('Id');SObject stored=Database.query('SELECT OwnerId FROM ` + object + ` WHERE Id=:rowId');Id queueId=[SELECT Id FROM Group WHERE DeveloperName='SyntheticRouting' AND Type='Queue' LIMIT 1].Id;System.assertEquals(queueId,(Id)stored.get('OwnerId'));`
						machine := New(nil)
						machine.SetOrg(&org)
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func installDMLPolicyAutomation(t *testing.T, org *storage.OrgState, loaded project.Project) {
	t.Helper()
	index, err := automation.LoadProject(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Diagnostics) != 0 {
		t.Fatal(index.Diagnostics)
	}
	automation.ApplyToOrg(org, index)
}

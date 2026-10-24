package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Native mixed_dml_contexts plainrole/anonymous_role and runasrole rows
// establish the insert rejection and its runAs exception at API62/67.
func TestMixedDMLNativeStatementAndDatabaseInsertRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"statement_scalar", "statement_list", "database_scalar", "database_list", "database_all_or_none"} {
			for _, context := range []string{"anonymous", "test", "test_runas"} {
				t.Run(api+"/"+context+"/"+route, func(t *testing.T) {
					machine := mixedDMLRouteVM(api)
					if context != "anonymous" {
						machine.EnableTestContext()
					}
					insert := "insert actor;"
					switch route {
					case "statement_list":
						insert = "insert new List<User>{actor};"
					case "database_scalar":
						insert = "Database.insert(actor);"
					case "database_list":
						insert = "Database.insert(new List<User>{actor});"
					case "database_all_or_none":
						insert = "Database.insert(actor, true);"
					}
					if context == "test_runas" {
						insert = "System.runAs(new User(Id=UserInfo.getUserId())) {" + insert + "}"
					}
					source := `insert new Account(Name='Route customer');
User actor = new User(Alias='route', Email='route@example.invalid', EmailEncodingKey='UTF-8',
 LastName='Route', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='UTC',
 ProfileId=UserInfo.getProfileId(), Username='route@example.invalid', UserRoleId='00E000000000001');` + insert
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					if context == "test_runas" {
						if err != nil {
							t.Fatal(err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "MIXED_DML_OPERATION") || !strings.Contains(err.Error(), "User, original object: Account") {
						t.Fatalf("native mixed insert rejection missing: %v", err)
					}
					// Another anonymous request on the same VM is a distinct transaction.
					if context == "anonymous" {
						next, err := CompileAnonymousWithOptions(source[strings.Index(source, ";")+1:], CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(next); err != nil {
							t.Fatalf("next anonymous request: %v", err)
						}
						if machine.testContext != nil {
							t.Fatal("anonymous checking enabled Test APIs")
						}
					}
				})
			}
		}
	}
}

// Exercise actual DML and merge callers. Native mixed_dml_actions API62/67
// rows pin delete rejection through applyDML; Database record actions support
// lock/unlock/emptyRecycleBin rather than delete.
func TestMixedDMLSharedCheckerRecordActionAndMergeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"dml_insert", "dml_delete", "merge"} {
			t.Run(api+"/test/"+route, func(t *testing.T) {
				machine := mixedDMLRouteVM(api)
				machine.EnableTestContext()
				machine.testContext.SetupDML = true
				account := Object("Account")
				account.Fields["Id"] = String("001000000000001")
				account.Fields["Name"] = String("Route customer")
				var err error
				switch route {
				case "dml_insert":
					_, err = machine.applyDML("insert", account, true, "", dml.Options{}, &Result{})
				case "dml_delete":
					_, err = machine.applyDML("delete", account, true, "", dml.Options{}, &Result{})
				case "merge":
					duplicate := Object("Account")
					duplicate.Fields["Id"] = String("001000000000002")
					_, err = machine.executeDatabaseMerge([]Value{account, duplicate}, &Result{})
				}
				if err == nil || (!strings.Contains(err.Error(), "Mixed DML") && !strings.Contains(err.Error(), "MIXED_DML_OPERATION")) {
					t.Fatalf("shared checker %s lost rejection: %v", route, err)
				}
			})
		}
	}
}

func mixedDMLRouteVM(api string) *VM {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "UserRole")
	roles := org.Objects["UserRole"]
	roles.Records["00E000000000001"] = storage.Record{ID: "00E000000000001", Object: "UserRole", Fields: map[string]storage.Value{"Name": storage.StringValue("Route role")}}
	org.Objects["UserRole"] = roles
	accounts := org.Objects["Account"]
	for i := 1; i <= 2; i++ {
		id := storage.ID(fmt.Sprintf("00100000000000%d", i))
		accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Route customer")}}
	}
	org.Objects["Account"] = accounts
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: api}
	return machine
}

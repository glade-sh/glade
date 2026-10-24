package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are scope controls, not native parity claims for record actions.
// Only the anonymous extension is restricted; the older test-context rule stays.
func TestMixedDMLRecordActionAnonymousScopeAndTestContextRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, operation := range []string{"lock", "unlock", "emptyRecycleBin"} {
				for _, order := range []string{"setup_first", "record_action_first"} {
					t.Run(api+"/"+context+"/"+operation+"/"+order, func(t *testing.T) {
						machine := mixedDMLRouteVM(api)
						if context == "test" {
							machine.EnableTestContext()
						}
						setup := []storage.Record{{Object: "PermissionSet"}}
						account := []storage.Record{{Object: "Account"}}
						var err error
						if order == "setup_first" {
							if err := machine.checkMixedDML("insert", setup); err != nil {
								t.Fatal(err)
							}
							before := machine.mixedDML
							err = machine.checkMixedDML(operation, account)
							if machine.mixedDML != before {
								t.Fatal("record action changed setup transaction state")
							}
						} else {
							if err := machine.checkMixedDML(operation, account); err != nil {
								t.Fatal(err)
							}
							if context == "anonymous" && machine.mixedDML != (mixedDMLState{}) {
								t.Fatal("record action introduced anonymous mixed-DML state")
							}
							err = machine.checkMixedDML("insert", setup)
						}
						if context == "anonymous" {
							if err != nil {
								t.Fatalf("uncaptured anonymous record action entered mixed-DML rule: %v", err)
							}
						} else if err == nil || !strings.Contains(strings.ToLower(err.Error()), "mixed") {
							t.Fatalf("existing test-context boundary changed: %v", err)
						}
					})
				}
			}
		}
	}
}

// The VM supports additional Database dispatcher routes not exposed by sema.
// Exercise those here while the project twin covers accepted declarations.
func TestMixedDMLRecordActionAnonymousAndTestPublicDispatcherRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, action := range []string{"Database.lock", "Database.unlock", "Database.emptyRecycleBin", "Approval.lock", "Approval.unlock"} {
				for _, shape := range []string{"scalar", "list", "id", "ids"} {
					for _, mode := range []string{"default", "true", "false"} {
						t.Run(api+"/"+context+"/"+action+"/"+shape+"/"+mode, func(t *testing.T) {
							machine := mixedDMLRouteVM(api)
							seed, err := CompileAnonymousWithOptions(`Account live=[SELECT Id FROM Account LIMIT 1]; Approval.lock(live); Account deleted=new Account(Name='Deleted'); insert deleted; delete deleted;`, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := machine.Execute(seed); err != nil {
								t.Fatal(err)
							}
							if context == "test" {
								machine.EnableTestContext()
							}
							input := "row"
							switch shape {
							case "list":
								input = "new List<Account>{row}"
							case "id":
								input = "row.Id"
							case "ids":
								input = "new List<Id>{row.Id}"
							}
							if mode != "default" {
								input += "," + mode
							}
							query := "[SELECT Id FROM Account WHERE Name!='Deleted' LIMIT 1]"
							if action == "Database.emptyRecycleBin" {
								query = "[SELECT Id FROM Account WHERE Name='Deleted' LIMIT 1 ALL ROWS]"
							}
							source := fmt.Sprintf(`Account row=%s; insert new PermissionSet(Name='ScopeBefore',Label='Scope before'); %s(%s); insert new PermissionSet(Name='ScopeAfter',Label='Scope after');`, query, action, input)
							program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = machine.Execute(program)
							if context == "anonymous" {
								if err != nil {
									t.Fatalf("anonymous record-action scope: %v", err)
								}
							} else if err == nil || !strings.Contains(err.Error(), "Mixed DML") {
								t.Fatalf("test-context boundary: %v", err)
							}
						})
					}
				}
			}
		}
	}
}

// Native mixed_dml_actions API62/67 rows: insert_rejection, update_rejection,
// upsert_database_insert_partial, delete_statement_scalar,
// undelete_database_partial, and merge_statement_scalar (anonymous and test).
func TestMixedDMLCapturedOperationsRemainGuardedAnonymousAndTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, operation := range []string{"insert", "update", "upsert", "delete", "undelete", "merge"} {
				t.Run(api+"/"+context+"/"+operation, func(t *testing.T) {
					machine := mixedDMLRouteVM(api)
					if context == "test" {
						machine.EnableTestContext()
					}
					if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err != nil {
						t.Fatal(err)
					}
					before := machine.mixedDML
					err := machine.checkMixedDML(operation, []storage.Record{{Object: "Account"}})
					if err == nil || !strings.Contains(strings.ToLower(err.Error()), "mixed") {
						t.Fatalf("captured operation lost mixed-DML rejection: %v", err)
					}
					if machine.mixedDML != before {
						t.Fatal("rejected operation changed transaction state")
					}
				})
			}
		}
	}
}

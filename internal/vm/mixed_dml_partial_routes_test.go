package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// mixed_dml_actions native API62/67 rows cover both exception and result
// surfaces after PermissionSet insertion, and retain setup-only caller state.
func TestMixedDMLPartialStatementAndDatabaseRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, operation := range []string{"insert", "update", "upsert_insert", "upsert_update", "delete", "undelete", "merge_scalar", "merge_list"} {
				for _, allOrNone := range []bool{false, true} {
					surface := "database_partial_result"
					if allOrNone {
						surface = "statement_or_database_exception"
					}
					t.Run(api+"/"+context+"/"+operation+"/"+surface, func(t *testing.T) {
						machine := mixedDMLRouteVM(api)
						if context == "test" {
							machine.EnableTestContext()
						}
						if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err != nil {
							t.Fatal(err)
						}
						parent := machine.mixedDML
						account := Object("Account")
						account.Fields["Id"] = String("001000000000001")
						account.Fields["Name"] = String("Route customer")
						duplicate := Object("Account")
						duplicate.Fields["Id"] = String("001000000000002")
						duplicate.Fields["Name"] = String("Route customer")
						var results []dml.Result
						var merge Value
						var err error
						if strings.HasPrefix(operation, "merge_") {
							input := duplicate
							if operation == "merge_list" {
								input = List(duplicate)
							}
							merge, err = machine.executeDatabaseMerge([]Value{account, input, Bool(allOrNone)}, &Result{})
						} else {
							op := strings.TrimSuffix(strings.TrimSuffix(operation, "_insert"), "_update")
							if operation == "insert" || operation == "upsert_insert" {
								delete(account.Fields, "Id")
								delete(duplicate.Fields, "Id")
							}
							results, err = machine.applyDML(op, List(account, duplicate), allOrNone, "", dml.Options{}, &Result{})
						}
						if allOrNone {
							if err == nil || (!strings.Contains(err.Error(), "Mixed DML") && !strings.Contains(err.Error(), "MIXED_DML_OPERATION")) {
								t.Fatalf("native mixed-DML exception missing: %v", err)
							}
						} else {
							if err != nil {
								t.Fatalf("native returns failed row results; Glade threw: %v", err)
							}
							if strings.HasPrefix(operation, "merge_") {
								if operation == "merge_list" {
									if len(merge.List) != 1 {
										t.Fatalf("merge result count: %v", merge)
									}
									merge = merge.List[0]
								}
								errors := merge.Fields["errors"].List
								if merge.Fields["success"].Bool || len(errors) != 1 || errors[0].Fields["statusCode"].String() != "MIXED_DML_OPERATION" {
									t.Fatalf("native merge failure missing: %v", merge)
								}
							} else {
								if len(results) != 2 {
									t.Fatalf("row result count=%d, want 2", len(results))
								}
								for _, result := range results {
									if result.Success || result.StatusCode != "MIXED_DML_OPERATION" {
										t.Fatalf("native row failure missing: %+v", result)
									}
								}
							}
						}
						if machine.mixedDML != parent {
							t.Fatalf("failed operation changed transaction state: %+v", machine.mixedDML)
						}
						if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err != nil {
							t.Fatalf("next setup insert: %v", err)
						}
					})
				}
			}
		}
	}
}

// Native mixed_dml_partial_shapes insert_mixed_* rows retain the compatible
// PermissionSet row in either order and reject only the Account row.
func TestMixedDMLPartialMixedListAnonymousAndTestResultOrder(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, setupFirst := range []bool{false, true} {
				order := "data_first"
				if setupFirst {
					order = "setup_first"
				}
				t.Run(api+"/"+context+"/database_insert_partial/"+order, func(t *testing.T) {
					machine := mixedDMLRouteVM(api)
					storage.EnsureStandardObject(machine.Org, "PermissionSet")
					if context == "test" {
						machine.EnableTestContext()
					}
					if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err != nil {
						t.Fatal(err)
					}
					setup := Object("PermissionSet")
					setup.Fields["Name"] = String("PartialRoute")
					setup.Fields["Label"] = String("Partial route")
					account := Object("Account")
					account.Fields["Name"] = String("Rejected route customer")
					input := List(account, setup)
					setupIndex := 1
					if setupFirst {
						input = List(setup, account)
						setupIndex = 0
					}
					results, err := machine.applyDML("insert", input, false, "", dml.Options{}, &Result{})
					if err != nil {
						t.Fatalf("native returns success and failure rows; Glade threw: %v", err)
					}
					if len(results) != 2 || !results[setupIndex].Success || results[1-setupIndex].Success || results[1-setupIndex].StatusCode != "MIXED_DML_OPERATION" {
						t.Fatalf("native selective mixed-DML results missing: %+v", results)
					}
					if !machine.mixedDML.setup || machine.mixedDML.nonSetup {
						t.Fatalf("failed data row poisoned setup transaction: %+v", machine.mixedDML)
					}
				})
			}
		}
	}
}

// The native scalar rows also require the public Database dispatcher to render
// one typed result object rather than a list or a thrown DmlException.
func TestMixedDMLPartialDatabaseScalarResultRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "test"} {
			for _, operation := range []string{"insert", "update", "upsert", "delete", "undelete"} {
				t.Run(api+"/"+context+"/database_"+operation+"_scalar_partial", func(t *testing.T) {
					machine := mixedDMLRouteVM(api)
					if context == "test" {
						machine.EnableTestContext()
					}
					if err := machine.checkMixedDML("insert", []storage.Record{{Object: "PermissionSet"}}); err != nil {
						t.Fatal(err)
					}
					account := Object("Account")
					account.Fields["Name"] = String("Route customer")
					if operation != "insert" {
						account.Fields["Id"] = String("001000000000001")
					}
					result, err := machine.executeDatabaseDML(operation, []Value{account, Bool(false)}, &Result{})
					if err != nil {
						t.Fatalf("native scalar result route threw: %v", err)
					}
					errors := result.Fields["errors"].List
					if result.Kind != ValueObject || result.Type != databaseDMLResultType(operation) || result.Fields["success"].Bool || len(errors) != 1 || errors[0].Fields["statusCode"].String() != "MIXED_DML_OPERATION" {
						t.Fatalf("native typed scalar failure missing: %v", result)
					}
				})
			}
		}
	}
}

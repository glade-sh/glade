package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/vm"
)

// Restore the pre-extension anonymous scope through real project helpers.
// Named tests retain the existing rejection; neither route asserts native parity
// for lock/unlock/emptyRecycleBin mixed-DML behavior.
func TestMixedDMLRecordActionAnonymousHelperAndProjectTestScopeRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	fixture := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixture, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	helper := "public class RecordActionScope {\n"
	tests := "@IsTest private class RecordActionScopeTest { @TestSetup static void seed() {" + recordActionScopeSeed + "}\n"
	var routes []string
	for _, action := range []string{"Database.emptyRecycleBin", "Approval.lock", "Approval.unlock"} {
		for _, shape := range []string{"scalar", "list", "id", "ids"} {
			for _, mode := range []string{"default", "true", "false"} {
				// Keep project coverage on declarations accepted by sema; VM tests
				// cover the additional dispatcher overloads directly.
				if action == "Database.emptyRecycleBin" && (mode != "default" || shape == "id" || shape == "ids") {
					continue
				}
				route := strings.ReplaceAll(action, ".", "_") + "_" + shape + "_" + mode
				routes = append(routes, route)
				input := "row"
				switch shape {
				case "list":
					input = "new List<Account>{row}"
				case "id":
					input = "row.Id"
				case "ids":
					input = "new List<Id>{row.Id}"
				}
				args := input
				if mode != "default" {
					args += "," + mode
				}
				query := "[SELECT Id FROM Account WHERE Name='Live' LIMIT 1]"
				if action == "Database.emptyRecycleBin" {
					query = "[SELECT Id FROM Account WHERE Name='Deleted' LIMIT 1 ALL ROWS]"
				}
				resultType := action + "Result"
				switch action {
				case "Database.lock":
					resultType = "Database.LockResult"
				case "Database.unlock":
					resultType = "Database.UnlockResult"
				case "Database.emptyRecycleBin":
					resultType = "Database.EmptyRecycleBinResult"
				case "Approval.lock":
					resultType = "Approval.LockResult"
				case "Approval.unlock":
					resultType = "Approval.UnlockResult"
				}
				result := "observed"
				if shape == "list" || shape == "ids" {
					resultType += "[]"
					result += "[0]"
				}
				helper += fmt.Sprintf(`public static void %s(Boolean reject) {
 Account row=%s;
 insert new PermissionSet(Name='ScopeBefore',Label='Scope before');
 Boolean rejected=false;
 try { %s observed=%s(%s); System.assert(%s.isSuccess(), '%s/%s/%s'); }
 catch(DmlException e) { System.assert(e.getMessage().contains('Mixed DML') || e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage()); rejected=true; }
 System.assertEquals(reject,rejected,'%s/%s/%s mixed-DML scope');
 insert new PermissionSet(Name='ScopeAfter',Label='Scope after');
}
`, route, query, resultType, action, args, result, action, shape, mode, action, shape, mode)
				tests += "@IsTest static void " + route + "() { RecordActionScope." + route + "(true); }\n"
			}
		}
	}
	for name, body := range map[string]string{"RecordActionScope.cls": helper + "}", "RecordActionScopeTest.cls": tests + "}"} {
		if err := os.WriteFile(filepath.Join(fixture, "sources", name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := executionCaptureIndex(t, fixture, api)
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range routes {
				t.Run("anonymous_helper/"+route, func(t *testing.T) {
					org := apextest.OrgFromIndex(index)
					org.APIVersion = api
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
						t.Fatal(err)
					}
					for _, source := range []string{recordActionScopeSeed, "RecordActionScope." + route + "(false);"} {
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
			t.Run("project_test_existing_rejection", func(t *testing.T) {
				report := apextest.Run(index, apextest.Options{SelectedClasses: []string{"RecordActionScopeTest"}, NoDiskCache: true, Parallelism: 1})
				summary := report.Summary()
				if summary.Total != len(routes) || summary.Failed != 0 {
					t.Fatalf("test-context controls: %+v; %+v", summary, report)
				}
			})
		})
	}
}

const recordActionScopeSeed = `Account live=new Account(Name='Live'); insert live; Approval.lock(live);
Account deleted=new Account(Name='Deleted'); insert deleted; delete deleted;`

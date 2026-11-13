package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDuplicateRuleEvaluationNativeRoutes(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	const dir = "testdata/conformance/duplicate_rule_evaluation"
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, area := range []string{"account_overflow", "person_rules", "edit_gate", "sharing"} {
		body, err := os.ReadFile(filepath.Join(dir, area, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(body)), "\n")
		if lines[0] != "api\troute\trepeat\tgroup\tmethod\tobservation" {
			t.Fatal("invalid native header")
		}
		for _, line := range lines[1:] {
			f := strings.Split(line, "\t")
			if len(f) != 6 {
				t.Fatalf("invalid native row: %q", line)
			}
			// Committed seed indexing and overflow under sharing vary natively.
			// See account_overflow/RUN-FACTS.txt and sharing/RUN-FACTS.txt.
			if strings.HasPrefix(f[3], "committed") || f[4] == "overflow" {
				continue
			}
			if groups[f[3]][f[4]] == "" {
				t.Fatalf("missing input: %s/%s", f[3], f[4])
			}
			key := strings.Join([]string{f[0], f[1], f[3], f[4]}, "/")
			if old := rows[key]; old != "" && old != f[5] {
				t.Fatalf("inconsistent repeat: %s", key)
			}
			rows[key] = f[5]
		}
	}
	for _, group := range slices.Sorted(maps.Keys(groups)) {
		cases := groups[group]
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(group+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				sharing := strings.HasPrefix(group, "share")
				copyInputs := func(name string) {
					path := filepath.Join(dir, "inputs", name)
					if _, err := os.Stat(path); err == nil {
						if err := os.CopyFS(base, os.DirFS(path)); err != nil {
							t.Fatal(err)
						}
					}
				}
				copyInputs(group)
				helpers := []string{}
				switch {
				case group == "account":
					helpers = []string{"AccountBehavior", "AccountGateBehavior"}
					writeCapturedAssignments(t, root, "ExtAccess")
				case group == "overfull":
					helpers = []string{"OverfullBehavior", "OverfullWeakBehavior"}
				case strings.HasPrefix(group, "custom"):
					helpers = []string{"OverfullCustomBehavior"}
				case strings.HasPrefix(group, "pa"):
					helpers = []string{"PABehavior"}
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
					// Activation was verified before capture. First activation in Setup
					// cannot be represented by a matching-rule deployment.
					if group != "pa_off" {
						data, err := os.ReadFile(filepath.Join(dir, "inputs/pa_off/duplicateRules/PersonAccount.Standard_Person_Account_Duplicate_Rule.duplicateRule-meta.xml"))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(base, "duplicateRules/PersonAccount.Standard_Person_Account_Duplicate_Rule.duplicateRule-meta.xml"), strings.Replace(string(data), "<isActive>false</isActive>", "<isActive>true</isActive>", 1))
					}
					if group != "pa" {
						for _, sub := range []string{"objects", "permissionsets"} {
							if err := os.CopyFS(filepath.Join(base, sub), os.DirFS(filepath.Join(dir, "inputs/pa", sub))); err != nil {
								t.Fatal(err)
							}
						}
					}
					writeCapturedAssignments(t, root, "ExtAccess")
				case sharing:
					helpers = []string{"ShareBehavior", "ShareWith", "ShareWithout", "ShareInherited", "SharingReplay"}
				case strings.HasPrefix(group, "edit"):
					helpers = []string{"EditBehavior"}
					writeCapturedAssignments(t, root, "ExtAccessCL")
				default:
					t.Fatalf("unknown group %s", group)
				}

				const class = "ObservationTest"
				var testClass strings.Builder
				fmt.Fprintf(&testClass, "@IsTest private class %s {\n", class)
				expressions := map[string]string{}
				for _, method := range slices.Sorted(maps.Keys(cases)) {
					if sharing && method == "overflow" {
						continue
					} // sharing/RUN-FACTS.txt: unsettled indexing.
					call := cases[method]
					source := "String observation = " + call + "; System.assert(false, 'OBS|" + method + "|' + Test.isRunningTest() + '|' + observation.length() + '|' + observation + '|ENDOBS');"
					expressions[method] = source
					setup := ""
					if sharing {
						setup = "SharingReplay.prepare();"
					}
					fmt.Fprintf(&testClass, "@IsTest static void %s() { %s %s }\n", method, setup, source)
				}
				testClass.WriteString("}")
				for _, name := range append(helpers, class) {
					source := testClass.String()
					if name != class {
						data, err := os.ReadFile(filepath.Join(dir, name+".cls"))
						if err != nil {
							t.Fatal(err)
						}
						source = string(data)
					}
					writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
					writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				index := loadTestIndex(t, root)
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				check := func(t *testing.T, route, method, message string) {
					t.Helper()
					want := rows[strings.Join([]string{api, route, group, method}, "/")]
					if want == "" {
						t.Fatal("missing native row")
					}
					prefix := fmt.Sprintf("OBS|%s|%t|", method, route == "istest")
					_, frame, ok := strings.Cut(message, prefix)
					size, payload, sized := strings.Cut(frame, "|")
					got, ended := strings.CutSuffix(payload, "|ENDOBS")
					if !ok || !sized || !ended || size != strconv.Itoa(len(got)) {
						t.Fatalf("invalid observation frame: %s", message)
					}
					if got != want {
						t.Errorf("native=%s\nGlade=%s", want, got)
					}
				}
				for _, method := range slices.Sorted(maps.Keys(expressions)) {
					source := expressions[method]
					routes := []string{"anonymous"}
					if sharing {
						routes = append(routes, "anonuser")
					}
					for _, route := range routes {
						t.Run(route+"/"+method, func(t *testing.T) {
							machine := runner.newMachine()
							if sharing {
								setup, err := vm.CompileAnonymousWithOptions("SharingReplay.prepare(); ShareBehavior.committedFixtures();", vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								if _, err := machine.Execute(setup); err != nil {
									t.Fatal(err)
								}
								// Replay the committed criteria grant captured in sharing/RUN-FACTS.
								// Test-created criteria shares are deliberately absent.
								var actor storage.Record
								for _, user := range machine.Org.Objects["User"].Records {
									if user.Fields["LastName"].String == "Sharing Reader" {
										actor = user
									}
								}
								if actor.ID == "" {
									t.Fatal("missing sharing actor")
								}
								for _, row := range machine.Org.Objects["Contact"].Records {
									if row.Fields["Title"].String != "crit" {
										continue
									}
									fixture := storage.NewFixture()
									fixture.Objects = []storage.FixtureObject{{Name: "ContactShare", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ContactId": storage.IDValue(row.ID), "UserOrGroupId": storage.IDValue(actor.ID), "ContactAccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Rule")}}}}}
									if err := storage.ApplyFixture(machine.Org, fixture); err != nil {
										t.Fatal(err)
									}
								}
								if route == "anonuser" {
									machine.SetCurrentUser(actor)
								}
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = machine.Execute(program)
							var fault *vm.RuntimeError
							if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatalf("missing Apex observation: %v", err)
							}
							check(t, route, method, fault.ExceptionMessage())
						})
					}
				}
				options := Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1}
				selected := Discover(index, options)
				if len(selected) != len(expressions) {
					t.Fatalf("discovered %d of %d methods", len(selected), len(expressions))
				}
				report := RunCasesContext(context.Background(), index, options, selected)
				count := 0
				for _, suite := range report.Suites {
					for _, result := range suite.Cases {
						count++
						t.Run("istest/"+result.MethodName, func(t *testing.T) {
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing Apex observation: %+v", result.Problem)
							}
							check(t, "istest", result.MethodName, result.Problem.Message)
						})
					}
				}
				if count != len(expressions) {
					t.Fatalf("ran %d of %d methods", count, len(expressions))
				}
			})
		}
	}
}

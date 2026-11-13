package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFuzzySurnameNativeRoutes(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	const dir = "../dml/testdata/conformance/surname_keyboard"
	groups := map[string]map[string]string{}
	rows := map[string]string{}
	methodPattern := regexp.MustCompile(`public static String (m_[A-Za-z0-9_]+)\(\)`)
	for _, stage := range []string{"stage1", "stage1b", "stage2"} {
		body, err := os.ReadFile(filepath.Join(dir, stage+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		groups[stage] = map[string]string{}
		for _, m := range methodPattern.FindAllStringSubmatch(string(body), -1) {
			groups[stage][m[1]] = "SurnameBehavior." + m[1] + "()"
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			t.Fatal(line)
		}
		for stage, methods := range groups {
			if methods[f[4]] != "" {
				key := strings.Join([]string{f[0], f[1], stage, f[4]}, "/")
				if prev := rows[key]; prev != "" && prev != f[5] {
					t.Fatalf("conflicting repeats %s", key)
				}
				rows[key] = f[5]
			}
		}
	}
	for _, group := range slices.Sorted(maps.Keys(groups)) {
		cases := groups[group]
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(group+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				helpers := []string{"SurnameBehavior"}

				const class = "ObservationTest"
				var testClass strings.Builder
				fmt.Fprintf(&testClass, "@IsTest private class %s {\n", class)
				expressions := map[string]string{}
				for _, method := range slices.Sorted(maps.Keys(cases)) {
					call := cases[method]
					source := "String observation = " + call + "; System.assert(false, 'OBS|" + method + "|' + Test.isRunningTest() + '|' + observation.length() + '|' + observation + '|ENDOBS');"
					expressions[method] = source
					fmt.Fprintf(&testClass, "@IsTest static void %s() { %s }\n", method, source)
				}
				testClass.WriteString("}")
				for _, name := range append(helpers, class) {
					source := testClass.String()
					if name != class {
						data, err := os.ReadFile(filepath.Join(dir, group+".cls"))
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
					want = surnameExactExpected(t, method, want)
					if got != want {
						t.Errorf("native=%s\nGlade=%s", want, got)
					}
				}
				for _, method := range slices.Sorted(maps.Keys(expressions)) {
					source := expressions[method]
					t.Run("anonymous/"+method, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing Apex observation: %v", err)
						}
						check(t, "anonymous", method, fault.ExceptionMessage())
					})
				}
				options := Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1}
				selected := Discover(index, options)
				if len(selected) != len(cases) {
					t.Fatalf("discovered %d of %d methods", len(selected), len(cases))
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
				if count != len(cases) {
					t.Fatalf("ran %d of %d methods", count, len(cases))
				}
			})
		}
	}
}

// Whole payload equality pins integer and raw confidence, Find members, DML
// members, save count and allow-save. Only listed grade residuals are replaced.
func surnameExactExpected(t *testing.T, method, observation string) string {
	t.Helper()
	// Populated with the remaining ordinary-grade differences after the fix.
	return observation
}

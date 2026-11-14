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

	"github.com/glade-sh/glade/internal/vm"
)

const fuzzyPanelsDir = "testdata/conformance/duplicate_fuzzy_panels"

// fuzzyPanelPairs parses a FuzzyPanel observation: the value count, then one
// token per match of the probed rule, candidate and existing index in base 36
// followed by the confidence when it is not 100.
func fuzzyPanelPairs(t *testing.T, observation string) (int, map[string]string) {
	t.Helper()
	count, body, ok := strings.Cut(observation, ":")
	n, err := strconv.Atoi(count)
	if !ok || err != nil {
		t.Fatalf("invalid panel observation %q", observation)
	}
	pairs := map[string]string{}
	if body == "" {
		return n, pairs
	}
	for _, token := range strings.Split(body, ",") {
		if len(token) < 2 || strings.Contains(token[:2], "?") {
			t.Fatalf("invalid panel token %q", token)
		}
		confidence := "100"
		if len(token) > 2 {
			value, err := strconv.ParseFloat(token[2:], 64)
			if err != nil || value != float64(int(value)) {
				t.Fatalf("invalid panel confidence %q", token)
			}
			confidence = strconv.Itoa(int(value))
		}
		if _, dup := pairs[token[:2]]; dup {
			t.Fatalf("duplicate panel pair %q", token)
		}
		pairs[token[:2]] = confidence
	}
	return n, pairs
}

type fuzzyPanelResidual struct{ native, glade string }

// fuzzyPanelResiduals are the exact pairs where Glade differs from the native
// rows (a confidence, or "-" for no match). Both sides are pinned, so neither a
// changed fixture nor a changed scorer can shift a residual silently. A native
// nonmatch can be pinned only when the baseline scorer also matches it.
func fuzzyPanelResiduals(t *testing.T) map[string]fuzzyPanelResidual {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fuzzyPanelsDir, "residuals.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "method\tpair\tnative\tglade" {
		t.Fatal("invalid residual header")
	}
	out := map[string]fuzzyPanelResidual{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[2] == f[3] {
			t.Fatalf("invalid residual %q", line)
		}
		out[f[0]+"/"+f[1]] = fuzzyPanelResidual{f[2], f[3]}
	}
	return out
}

// NATIVE fuzzy matching panels testdata/conformance/duplicate_fuzzy_panels/org.tsv:2 anonymous 62.0
// NATIVE fuzzy matching panels testdata/conformance/duplicate_fuzzy_panels/org.tsv:8 istest 67.0
// Every pair of each panel's values, under custom matching rules with one
// Last Name, First Name, Title, Company Name or Street item, and one rule with
// three fuzzy items, through Datacloud.FindDuplicates on both Apex routes. The
// two native repeats agree; every confidence is compared exactly.
func TestDuplicateFuzzyPanelsNativeRoutes(t *testing.T) {
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(fuzzyPanelsDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(fuzzyPanelsDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\trepeat\tgroup\tmethod\tobservation" {
		t.Fatal("invalid native row header")
	}
	rows := map[string]string{}
	repeats := map[string]int{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 || groups[f[3]][f[4]] == "" {
			t.Fatalf("invalid row: %q", line)
		}
		key := strings.Join([]string{f[0], f[1], f[3], f[4]}, "/")
		if old, ok := rows[key]; ok && old != f[5] {
			t.Fatalf("native repeats differ: %s", key)
		}
		rows[key] = f[5]
		repeats[key]++
	}
	caseCount := 0
	for _, cases := range groups {
		caseCount += len(cases)
	}
	if caseCount == 0 || len(rows) != 4*caseCount {
		t.Fatalf("native matrix has %d observations for %d cases", len(rows), caseCount)
	}
	for key, n := range repeats {
		if n != 2 {
			t.Fatalf("%s: %d native repeats", key, n)
		}
	}
	residuals := fuzzyPanelResiduals(t)
	used := map[string]bool{}
	compare := func(t *testing.T, method, want, got string) {
		t.Helper()
		wn, native := fuzzyPanelPairs(t, want)
		gn, glade := fuzzyPanelPairs(t, got)
		if wn != gn {
			t.Fatalf("value count native=%d Glade=%d", wn, gn)
		}
		keys := map[string]bool{}
		for k := range native {
			keys[k] = true
		}
		for k := range glade {
			keys[k] = true
		}
		for _, pair := range slices.Sorted(maps.Keys(keys)) {
			n, g := native[pair], glade[pair]
			if n == "" {
				n = "-"
			}
			if g == "" {
				g = "-"
			}
			expected := n
			if r, ok := residuals[method+"/"+pair]; ok {
				used[method+"/"+pair] = true
				if r.native != n {
					t.Errorf("%s: native %s changed under residual pin %s", pair, n, r.native)
				}
				expected = r.glade
			}
			if g != expected {
				t.Errorf("%s %s: native=%s Glade=%s pinned=%s", method, pair, n, g, expected)
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
				for _, name := range []string{"matchingRules", "duplicateRules"} {
					if err := os.CopyFS(filepath.Join(base, name), os.DirFS(filepath.Join(fuzzyPanelsDir, name))); err != nil {
						t.Fatal(err)
					}
				}
				const class = "ObservationTest"
				var testClass strings.Builder
				fmt.Fprintf(&testClass, "@IsTest private class %s {\n", class)
				expressions := map[string]string{}
				for _, method := range slices.Sorted(maps.Keys(cases)) {
					source := "String observation = " + cases[method] + "; System.assert(false, 'OBS|" + method + "|' + Test.isRunningTest() + '|' + observation.length() + '|' + observation + '|ENDOBS');"
					expressions[method] = source
					fmt.Fprintf(&testClass, "@IsTest static void %s() { %s }\n", method, source)
				}
				testClass.WriteString("}")
				helper, err := os.ReadFile(filepath.Join(fuzzyPanelsDir, "FuzzyPanel.cls"))
				if err != nil {
					t.Fatal(err)
				}
				for name, source := range map[string]string{"FuzzyPanel": string(helper), class: testClass.String()} {
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
					compare(t, method, want, got)
				}
				for _, method := range slices.Sorted(maps.Keys(expressions)) {
					t.Run("anonymous/"+method, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(expressions[method], vm.CompileOptions{APIVersion: api})
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
	if !t.Failed() {
		for key := range residuals {
			if !used[key] {
				t.Errorf("unused residual %s", key)
			}
		}
	}
}

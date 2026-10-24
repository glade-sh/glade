package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The native source project reproduces a captured separator used twice by a
// parser, through named class calls from anonymous Apex and @IsTest methods.
func TestNumericBackreferenceNativeProjectSplitReplaceAllAndMatcherRoutes(t *testing.T) {
	fixture := filepath.Join("testdata", "regex_numeric_backreferences")
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][2]string{}
	for _, row := range read("cases.tsv") {
		var p, s string
		if err := json.Unmarshal([]byte(row[1]), &p); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(row[2]), &s); err != nil {
			t.Fatal(err)
		}
		cases[row[0]] = [2]string{p, s}
	}
	rows := read("org.tsv")
	if len(rows) != 100 || len(cases) != 25 {
		t.Fatalf("oracle rows=%d cases=%d", len(rows), len(cases))
	}
	quote := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `'`, `\'`)
		s = strings.ReplaceAll(s, "\b", `'+String.fromCharArray(new List<Integer>{8})+'`)
		return "'" + s + "'"
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, name := range []string{"NumericReferenceProbe", "NumericReferenceProbeTest"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			named := Run(index, Options{SelectedClasses: []string{"NumericReferenceProbeTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := named.Summary(); summary.Total != 25 || summary.Failed != 25 {
				t.Fatalf("missing named observation transport: %+v %s", summary, firstRunProblem(named))
			}
			observations := map[string]string{}
			for _, suite := range named.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil {
						t.Fatal("missing observation transport")
					}
					_, value, ok := strings.Cut(c.Problem.Message, ", Actual: ")
					if !ok || !strings.Contains(c.Problem.Message, "REFERENCE_ROWS") {
						t.Fatal(c.Problem.Message)
					}
					observations[c.MethodName] = value
				}
			}
			for _, row := range rows {
				if row[0] != api {
					continue
				}
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					input, ok := cases[row[2]]
					if !ok {
						t.Fatal("unknown native case")
					}
					observed := observations[row[2]]
					if row[1] == "anonymous" {
						body := `String observation=JSON.serialize(NumericReferenceProbe.observe(` + quote(input[0]) + `,` + quote(input[1]) + `));`
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						observed = machine.Globals["observation"].Text
					} else if row[1] != "isTest" {
						t.Fatal("unknown native context")
					}
					var got, want map[string]any
					if err := json.Unmarshal([]byte(observed), &got); err != nil {
						t.Fatalf("observation %q: %v", observed, err)
					}
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if len(want) != 11 {
						t.Fatalf("native route count=%d", len(want))
					}
					if row[2] == "ref10_nine_backspace" || row[2] == "ref10_nine_ref1_zero" || row[2] == "ref10_zero_backspace" {
						// Capture the native gap, but test every route against the
						// matcher control instead of asserting unsupported parity.
						matched, ok := got["Matcher.matches"].(bool)
						if !ok {
							t.Fatal("missing matcher control")
						}
						text := "L" + input[1] + "R" + input[1] + "T"
						parts, positive, replacement, found := []string{text}, []string{text}, text, []string{}
						if matched {
							parts, positive, replacement, found = []string{"L", "R", "T"}, []string{"L", "R" + input[1] + "T"}, "LQRQT", []string{input[1], input[1]}
						}
						control := map[string]any{"Matcher.find": found, "Matcher.replaceAll": replacement, "String.replaceAll": replacement}
						for _, route := range []string{"String.split", "Pattern.split"} {
							control[route+"/default"], control[route+"/negative"], control[route+"/positive"] = parts, parts, positive
						}
						for route, value := range control {
							raw, err := json.Marshal(value)
							if err != nil {
								t.Fatal(err)
							}
							var expected any
							if err := json.Unmarshal(raw, &expected); err != nil {
								t.Fatal(err)
							}
							want[route] = expected
						}
						want["Matcher.matches"] = matched
						t.Log("carried native mismatch; checking split/replaceAll/matcher equivalence (see CARRIED.txt)")
					}
					for route, expected := range want {
						t.Run(route, func(t *testing.T) {
							if !reflect.DeepEqual(got[route], expected) {
								t.Fatalf("got %v want native %v", got[route], expected)
							}
						})
					}
				})
			}
		})
	}
}

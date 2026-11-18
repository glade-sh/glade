package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type soslFidelityFixture struct {
	dir          string
	countQueries bool
}

type soslFidelityCase struct {
	id, source, setup, route, scope string
}

type soslFidelityKey struct {
	id, api, route string
}

type soslFidelityRow struct {
	observation string
	line        int
}

func (f soslFidelityFixture) read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.dir, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func (f soslFidelityFixture) table(t *testing.T, file string) [][]string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(f.read(t, file), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("%s contains no observations", file)
	}
	var rows [][]string
	for _, line := range lines[1:] {
		rows = append(rows, strings.Split(strings.TrimSuffix(line, "\r"), "\t"))
	}
	return rows
}

func (row soslFidelityRow) check(t *testing.T, got string) {
	t.Helper()
	if got != row.observation {
		t.Errorf("native org.tsv:%d: got %q, want %q", row.line, got, row.observation)
	}
}

func TestSOSLFidelityConformance(t *testing.T) {
	soslFidelityFixture{dir: "testdata/conformance/sosl_fidelity"}.replay(t)
}

func (f soslFidelityFixture) replay(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	var cases []soslFidelityCase
	known := map[string]string{}
	expectedRows := 0
	discovery := os.Getenv("GLADE_SOSL_DISCOVERY") == "1"
	for _, fields := range f.table(t, "cases.tsv") {
		if len(fields) < 3 || len(fields) > 5 || fields[0] == "" || known[fields[0]] != "" {
			t.Fatalf("invalid case: %q", fields)
		}
		scope := "owned"
		if len(fields) >= 4 {
			scope = fields[3]
		}
		route := "both"
		if len(fields) == 5 {
			route = fields[4]
		}
		switch route {
		case "both":
			expectedRows += 4
		case "istest":
			expectedRows += 2
		default:
			t.Fatalf("unknown route %q for %s", route, fields[0])
		}
		known[fields[0]] = route
		switch scope {
		case "owned", "indexed-discovery":
		case "discovery":
			if !discovery {
				continue
			}
		default:
			t.Fatalf("unknown scope %q for %s", scope, fields[0])
		}
		cases = append(cases, soslFidelityCase{fields[0], fields[1], fields[2], route, scope})
	}
	results := map[soslFidelityKey]soslFidelityRow{}
	for i, fields := range f.table(t, "org.tsv") {
		if len(fields) != 4 || known[fields[0]] == "" || fields[1] != "62.0" && fields[1] != "67.0" ||
			fields[2] != "anonymous" && fields[2] != "istest" || known[fields[0]] == "istest" && fields[2] != "istest" {
			t.Fatalf("invalid native org.tsv:%d: %q", i+2, fields)
		}
		key := soslFidelityKey{fields[0], fields[1], fields[2]}
		if _, ok := results[key]; ok {
			t.Fatalf("duplicate native row: %v", key)
		}
		results[key] = soslFidelityRow{fields[3], i + 2}
	}
	if len(cases) == 0 {
		t.Fatal("no SOSL cases selected")
	}
	if len(results) != expectedRows {
		t.Fatalf("incomplete native matrix: got %d rows, want %d", len(results), expectedRows)
	}

	seed := f.read(t, "setup.apex")
	setups := map[string]string{"none": "", "set": f.read(t, "set.apex"), "clear": "Test.setFixedSearchResults(new List<Id>());"}
	if !f.countQueries {
		setups["clear"] = setups["set"] + "\n" + setups["clear"]
		setups["reverse"] = f.read(t, "reverse.apex")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var class strings.Builder
			const name = "SearchFidelityTest"
			fmt.Fprintf(&class, "@IsTest private class %s {\n", name)
			for _, c := range cases {
				setup, ok := setups[c.setup]
				if !ok {
					t.Fatalf("unknown setup %q for %s", c.setup, c.id)
				}
				fmt.Fprintf(&class, "@IsTest static void %s(){\n%s\n%s\n%s\nSystem.assert(false,'OBS|'+obs+'|END');\n}\n",
					c.id, seed, setup, f.observation(f.read(t, c.source)))
			}
			class.WriteString("}\n")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
			writeFile(t, path, class.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			t.Run("istest", func(t *testing.T) {
				report := Run(index, Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
				if len(report.Suites) != 1 || len(report.Suites[0].Cases) != len(cases) {
					t.Fatalf("incomplete test execution: %+v", report.Summary())
				}
				observed := map[string]testreport.Case{}
				for _, entry := range report.Suites[0].Cases {
					observed[entry.MethodName] = entry
				}
				for _, c := range cases {
					t.Run(c.id, func(t *testing.T) {
						entry := observed[c.id]
						if entry.Problem == nil {
							t.Fatalf("test did not report its observation: %+v", entry)
						}
						got := "UNCATCHED|" + entry.Problem.Type + "|" + entry.Problem.Message
						if entry.Problem.Type == "System.AssertException" {
							got = extractSOSLObservation(t, entry.Problem.Message, "Assertion Failed: OBS|")
						}
						results[soslFidelityKey{c.id, api, "istest"}].check(t, got)
					})
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				// Search the records from a previously committed transaction. Use a
				// fresh Tooling VM for each row, including its governor counters.
				org := orgFromIndex(index)
				seedVM := vm.New(nil)
				seedVM.SetOrg(&org)
				if _, err := runSOSLAnonymous(t, seedVM, index, api, seed); err != nil {
					t.Fatalf("anonymous seed: %v", err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: seedVM.Org})
				for _, c := range cases {
					if c.route == "istest" || c.scope == "indexed-discovery" && !discovery {
						continue
					}

					t.Run(c.id, func(t *testing.T) {
						path := strings.TrimSuffix(c.source, ".apex") + ".anonymous.apex"
						body, err := os.ReadFile(filepath.Join(f.dir, path))
						if os.IsNotExist(err) {
							body = []byte(f.read(t, c.source))
						} else if err != nil {
							t.Fatal(err)
						}
						result, err := runSOSLAnonymous(t, runner.newMachine(), index, api, f.observation(string(body))+"\nSystem.debug('OBS|'+obs+'|END');")
						var got string
						if err != nil {
							var runtimeErr *vm.RuntimeError
							if !errors.As(err, &runtimeErr) {
								t.Fatal(err)
							}
							got = "UNCATCHED|" + runtimeErr.Type + "|" + runtimeErr.ExceptionMessage()
						} else {
							if len(result.Debug) != 1 {
								t.Fatalf("expected one observation: %q", result.Debug)
							}
							got = extractSOSLObservation(t, result.Debug[0], "OBS|")
						}
						results[soslFidelityKey{c.id, api, "anonymous"}].check(t, got)
					})
				}
			})
		})
	}
}

func extractSOSLObservation(t *testing.T, line, prefix string) string {
	t.Helper()
	value, ok := strings.CutPrefix(line, prefix)
	value, end := strings.CutSuffix(value, "|END")
	if !ok || !end {
		t.Fatalf("missing observation: %q", line)
	}
	return value
}

func (f soslFidelityFixture) observation(source string) string {
	body := "String obs=''; try{\n" + source + "\n}catch(Exception e){obs=e.getTypeName()+'|'+e.getMessage();}"
	if f.countQueries {
		return "Integer q0=Limits.getSoslQueries();\n" + body + "\nobs+='|'+(Limits.getSoslQueries()-q0);"
	}
	return body
}

func runSOSLAnonymous(t *testing.T, machine *vm.VM, index typesys.Index, api, source string) (vm.Result, error) {
	t.Helper()
	if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
		t.Fatal(analysis.Diagnostics)
	}
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, source, api)})
	if err != nil {
		t.Fatal(err)
	}
	machine.SetToolingExecuteAnonymous(true)
	machine.SetLimitMode(vm.LimitModeStrict)
	return machine.Execute(program)
}

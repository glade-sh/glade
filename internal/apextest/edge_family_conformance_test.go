package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// edgeFamily replays one native capture family: cases.tsv names the rows,
// sources/ holds the synthetic Apex, seeds/ the committed data, force-app/
// the metadata templates ("__A__" is the API major, "__API__" the version) and
// org.tsv the observations Salesforce returned for each id, API and route.
type edgeFamily struct {
	dir string
	// skip lists rows whose observation Glade cannot reproduce yet; each entry
	// states why. A skipped row is still checked for presence in org.tsv.
	skip map[string]string
}

type edgeCase struct {
	id, seed, route, post string
}

type edgeKey struct{ id, api, route string }

type edgeRow struct {
	observation string
	line        int
}

func (f edgeFamily) read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.dir, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func (f edgeFamily) source(t *testing.T, id, route string) string {
	t.Helper()
	for _, name := range []string{id + "." + route + ".apex", id + ".apex"} {
		if body, err := os.ReadFile(filepath.Join(f.dir, "sources", name)); err == nil {
			return string(body)
		}
	}
	t.Fatalf("no %s source for %s", route, id)
	return ""
}

func (f edgeFamily) cases(t *testing.T) []edgeCase {
	t.Helper()
	var out []edgeCase
	seen := map[string]bool{}
	for i, line := range strings.Split(strings.TrimSuffix(f.read(t, "cases.tsv"), "\n"), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 5 || seen[fields[0]] {
			t.Fatalf("invalid case line %d: %q", i+1, line)
		}
		seen[fields[0]] = true
		switch fields[3] {
		case "anonymous", "istest", "both":
		default:
			t.Fatalf("unknown route %q for %s", fields[3], fields[0])
		}
		out = append(out, edgeCase{id: fields[0], seed: fields[2], route: fields[3], post: fields[4]})
	}
	return out
}

func (f edgeFamily) rows(t *testing.T, cases []edgeCase, apis []string) map[edgeKey]edgeRow {
	t.Helper()
	rows := map[edgeKey]edgeRow{}
	for i, line := range strings.Split(strings.TrimSuffix(f.read(t, "org.tsv"), "\n"), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			t.Fatalf("invalid native org.tsv:%d", i+1)
		}
		key := edgeKey{fields[0], fields[1], fields[2]}
		if _, dup := rows[key]; dup {
			t.Fatalf("duplicate native row %v", key)
		}
		rows[key] = edgeRow{fields[3], i + 1}
	}
	want := 0
	for _, c := range cases {
		for _, api := range apis {
			for _, route := range []string{"anonymous", "istest"} {
				if c.route != "both" && c.route != route {
					continue
				}
				want++
				if _, ok := rows[edgeKey{c.id, api, route}]; !ok {
					t.Fatalf("missing native row %s %s %s", c.id, api, route)
				}
			}
		}
	}
	if len(rows) != want {
		t.Fatalf("native matrix has %d rows, want %d", len(rows), want)
	}
	return rows
}

func (f edgeFamily) writeProject(t *testing.T, root, api string, cases []edgeCase) string {
	t.Helper()
	maj := strings.TrimSuffix(api, ".0")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	templates := filepath.Join(f.dir, "force-app")
	if _, err := os.Stat(templates); err == nil {
		err := filepath.Walk(templates, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(templates, path)
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := strings.NewReplacer("__A__", maj, "__API__", api).Replace(string(body))
			writeFile(t, filepath.Join(root, "force-app", strings.ReplaceAll(rel, "__A__", maj)), text)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	const name = "EdgeFamilyTest"
	var class strings.Builder
	fmt.Fprintf(&class, "@IsTest private class %s {\n", name)
	count := 0
	for _, c := range cases {
		if c.route == "anonymous" {
			continue
		}
		seed := ""
		if c.seed != "-" && c.seed != "none" {
			seed = f.read(t, "seeds/"+c.seed+".apex")
		}
		body := strings.ReplaceAll(f.source(t, c.id, "istest"), "__A__", maj)
		fmt.Fprintf(&class, "@IsTest static void %s() {\n%s\nString obs='';\ntry {\n%s\n} catch (Exception e) { obs = e.getTypeName() + '|' + e.getMessage(); }\nSystem.assert(false, 'OBS|' + obs + '|END');\n}\n",
			c.id, strings.ReplaceAll(seed, "__A__", maj), body)
		count++
	}
	class.WriteString("}\n")
	if count > 0 {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, class.String())
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return name
}

var edgeOrgID = regexp.MustCompile(`00D[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?`)

var edgeRepeat = regexp.MustCompile(`^// repeat: (\d+)`)

func (f edgeFamily) replay(t *testing.T) {
	releaseRuntimeCachesAfterTest(t)
	cases := f.cases(t)
	apis := []string{"62.0", "67.0"}
	rows := f.rows(t, cases, apis)
	check := func(t *testing.T, id, api, route, got string) {
		t.Helper()
		got = edgeOrgID.ReplaceAllString(got, "00DREDACTED")
		row := rows[edgeKey{id, api, route}]
		if reason, ok := f.skip[id+"/"+route]; ok {
			t.Skipf("known gap: %s (native org.tsv:%d %q, Glade %q)", reason, row.line, row.observation, got)
		}
		if strings.HasPrefix(row.observation, "COMPILEERR|") {
			// Only the rejection is compared; compiler wording is not claimed.
			if !strings.HasPrefix(got, "COMPILEERR|") {
				t.Errorf("native org.tsv:%d %s %s: want a compile error, got %q", row.line, route, api, got)
			}
			return
		}
		if got != row.observation {
			t.Errorf("native org.tsv:%d %s %s: got %q, want %q", row.line, route, api, got, row.observation)
		}
	}
	for _, api := range apis {
		t.Run(api, func(t *testing.T) {
			maj := strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			name := f.writeProject(t, root, api, cases)
			index := loadTestIndex(t, root)
			hasIstest, hasAnon := false, false
			for _, c := range cases {
				hasIstest = hasIstest || c.route != "anonymous"
				hasAnon = hasAnon || c.route != "istest"
			}
			if hasIstest {
				t.Run("istest", func(t *testing.T) {
					report := Run(index, Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
					observed := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, entry := range suite.Cases {
							observed[entry.MethodName] = entry
						}
					}
					for _, c := range cases {
						if c.route == "anonymous" {
							continue
						}
						t.Run(c.id, func(t *testing.T) {
							entry, ok := observed[c.id]
							if !ok || entry.Problem == nil {
								t.Fatalf("test did not report an observation: %+v", entry)
							}
							got := "UNCATCHED|" + entry.Problem.Type + "|" + entry.Problem.Message
							if entry.Problem.Type == "System.AssertException" {
								got = extractSOSLObservation(t, entry.Problem.Message, "Assertion Failed: OBS|")
							}
							check(t, c.id, api, "istest", got)
						})
					}
				})
			}
			if !hasAnon {
				return
			}
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				seedVM := vm.New(nil)
				seedVM.SetOrg(&org)
				seeded := map[string]bool{}
				for _, c := range cases {
					if c.route == "istest" || c.seed == "-" || c.seed == "none" || seeded[c.seed] {
						continue
					}
					seeded[c.seed] = true
					body := f.read(t, "seeds/"+c.seed+".apex")
					runs := 1
					if m := edgeRepeat.FindStringSubmatch(body); m != nil {
						runs, _ = strconv.Atoi(m[1])
					}
					for i := 0; i < runs; i++ {
						source := strings.NewReplacer("__N__", strconv.Itoa(i), "__A__", maj).Replace(body)
						if _, err := runEdgeAnonymous(t, seedVM, index, api, source, vm.LimitModePermissive); err != nil {
							t.Fatalf("seed %s: %v", c.seed, err)
						}
					}
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: seedVM.Org})
				for _, c := range cases {
					if c.route == "istest" {
						continue
					}
					t.Run(c.id, func(t *testing.T) {
						source := "String obs='';\ntry {\n" + strings.ReplaceAll(f.source(t, c.id, "anonymous"), "__A__", maj) +
							"\n} catch (Exception e) { obs = e.getTypeName() + '|' + e.getMessage(); }\nSystem.debug('OBS|' + obs + '|END');\n"
						machine := runner.newMachine()
						result, err := runEdgeAnonymous(t, machine, index, api, source, vm.LimitModeStrict)
						var got string
						var compileErr *edgeCompileError
						var runtimeErr *vm.RuntimeError
						switch {
						case errors.As(err, &compileErr):
							got = "COMPILEERR|" + compileErr.message
						case errors.As(err, &runtimeErr):
							got = "UNCATCHED|" + runtimeErr.Type + "|" + runtimeErr.ExceptionMessage()
						case err != nil:
							t.Fatal(err)
						default:
							for _, line := range result.Debug {
								if value, ok := strings.CutPrefix(line, "OBS|"); ok {
									got = strings.TrimSuffix(value, "|END")
								}
							}
							if got == "" {
								t.Fatalf("no observation in debug: %q", result.Debug)
							}
						}
						if c.post != "-" && c.post != "" {
							t.Skipf("asynchronous observation; checked by the family's async test")
						}
						check(t, c.id, api, "anonymous", got)
					})
				}
			})
		})
	}
}

type edgeCompileError struct{ message string }

func (e *edgeCompileError) Error() string { return e.message }

func runEdgeAnonymous(t *testing.T, machine *vm.VM, index typesys.Index, api, source string, mode vm.LimitMode) (vm.Result, error) {
	t.Helper()
	if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
		return vm.Result{}, &edgeCompileError{fmt.Sprint(analysis.Diagnostics)}
	}
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, source, api)})
	if err != nil {
		return vm.Result{}, &edgeCompileError{err.Error()}
	}
	machine.SetToolingExecuteAnonymous(true)
	machine.SetLimitMode(mode)
	return machine.Execute(program)
}

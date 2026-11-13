package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Each row in org.tsv identifies the API, route and projected department labels.
// With fixed IDs inside a test, the seed names still need not match the term.
func TestSOSLNameTokenConformance(t *testing.T) {
	const dir = "testdata/conformance/sosl_name_tokens"
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	readTable := func(file string, columns int) [][]string {
		t.Helper()
		lines := strings.Split(strings.TrimSuffix(read(file), "\n"), "\n")
		var result [][]string
		for _, line := range lines[1:] {
			parts := strings.Split(line, "\t")
			if len(parts) != columns {
				t.Fatalf("malformed %s row %q", file, line)
			}
			result = append(result, parts)
		}
		return result
	}
	type key struct{ id, api, route string }
	type row struct {
		want string
		line int
	}
	rows := map[key]row{}
	for i, fields := range readTable("org.tsv", 4) {
		k := key{fields[0], fields[1], fields[2]}
		if _, ok := rows[k]; ok {
			t.Fatalf("duplicate native row %+v", k)
		}
		rows[k] = row{fields[3], i + 2}
	}
	queries := readTable("queries.tsv", 3)
	if len(queries) != 27 || len(rows) != 4*len(queries) {
		t.Fatal("incomplete native token matrix")
	}
	seed := read("seed.apex")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			const className = "SearchTermsTest"
			var class strings.Builder
			class.WriteString("@IsTest private class " + className + " {\n")
			type probe struct {
				id, body, policy string
				row              row
			}
			var anon []probe
			for _, q := range queries {
				id, body := q[0], q[2]
				body = "String obs; try{" + body + "}catch(Exception e){obs=e.getTypeName()+'|'+e.getMessage();}"
				for _, route := range []string{"istest", "anonymous"} {
					k := key{id, api, route}
					r, ok := rows[k]
					if !ok {
						t.Fatalf("missing native row %+v", k)
					}
					check := fmt.Sprintf("System.assertEquals(%s,obs,'org.tsv:%d %s');", conformanceApexString(r.want), r.line, id)
					if route == "istest" {
						fmt.Fprintf(&class, "@IsTest static void %s(){%s\n Test.setFixedSearchResults(new List<Id>{f.Id,b.Id,a.Id});\n%s\n%s}\n", id, seed, body, check)
					} else {
						anon = append(anon, probe{id, body + "\nSystem.debug(obs);", q[1], r})
					}
				}
			}
			class.WriteString("}")
			path := filepath.Join(root, "force-app/main/default/classes", className+".cls")
			writeFile(t, path, class.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			t.Run("istest", func(t *testing.T) {
				report := Run(index, Options{SelectedClasses: []string{className}, NoDiskCache: true, Parallelism: 1})
				if sum := report.Summary(); sum.Passed != len(queries) || sum.Total != len(queries) {
					t.Fatalf("test results: %+v", report.Suites)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				org := orgFromIndex(index)
				machine := vm.New(nil)
				machine.SetOrg(&org)
				machine.SetToolingExecuteAnonymous(true)
				execute := func(machine *vm.VM, body string) (vm.Result, error) {
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						return vm.Result{}, err
					}
					return machine.Execute(program)
				}
				if _, err := execute(machine, seed); err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: machine.Org})
				for _, p := range anon {
					t.Run(p.id, func(t *testing.T) {
						m := runner.newMachine()
						m.SetToolingExecuteAnonymous(true)
						result, err := execute(m, p.body)
						got := ""
						if err != nil {
							got = err.Error()
						} else if len(result.Debug) == 1 {
							got = result.Debug[0]
						} else {
							t.Fatalf("missing observation: %v", result.Debug)
						}
						if got != p.row.want {
							message := fmt.Sprintf("native org.tsv:%d: got %q, want %q", p.row.line, got, p.row.want)
							switch p.policy {
							case "assert":
								t.Error(message)
							case "observe":
								t.Log("open observation:", message)
							default:
								t.Fatalf("unknown comparison %q", p.policy)
							}
						}
					})
				}
				// Local agreement across entry points. The native rows used Search.query.
				for _, tc := range []struct{ id, term, scope string }{
					{"middleQuestion", "B?ta", "NAME"}, {"phoneLocal", "5551287", "PHONE"},
				} {
					suffix := " IN " + tc.scope + " FIELDS RETURNING Contact(Id,Department ORDER BY Department ASC)"
					query := conformanceApexString("FIND {" + tc.term + "}" + suffix)
					want := rows[key{tc.id, api, "anonymous"}].want
					for _, call := range []struct{ name, loop string }{
						{"inline", "List<List<SObject>> hits=[FIND '" + tc.term + "'" + suffix + "]; for(SObject rec:hits[0]){labels.add((String)rec.get('Department'));}"},
						{"querySystem", "for(SObject rec:Search.query(" + query + ",AccessLevel.SYSTEM_MODE)[0]){labels.add((String)rec.get('Department'));}"},
						{"find", "for(Search.SearchResult hit:Search.find(" + query + ").get('Contact')){labels.add((String)hit.getSObject().get('Department'));}"},
						{"findSystem", "for(Search.SearchResult hit:Search.find(" + query + ",AccessLevel.SYSTEM_MODE).get('Contact')){labels.add((String)hit.getSObject().get('Department'));}"},
					} {
						t.Run("localEntryPoints/"+tc.id+"/"+call.name, func(t *testing.T) {
							m := runner.newMachine()
							m.SetToolingExecuteAnonymous(true)
							result, err := execute(m, "List<String> labels=new List<String>();"+call.loop+"System.debug(String.join(labels,','));")
							if err != nil || len(result.Debug) != 1 || result.Debug[0] != want {
								t.Fatalf("%s got %v, %v; want %q", call.name, result.Debug, err, want)
							}
						})
					}
				}
			})
		})
	}
}

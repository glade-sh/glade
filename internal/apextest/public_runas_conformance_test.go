package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native sources and diagnostics stay in the matrix. Only observation transport
// is replaced with exact String.equals assertions; both routes use real shadows.
func TestPublicRunAsOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID, APIVersion, Route, Revision, Name, Source, Code, Expected string
		Compile                                                       bool
		Companions                                                    map[string]string
	}
	type evidence struct {
		OrgTSV, CompileTSV, RuntimeSource string
		Companions                        map[string]string
	}
	type revision struct {
		NativeEvidence map[string]map[string]evidence
	}
	var data struct {
		APIVersions, Routes []string
		NativeEvidence      map[string]map[string]evidence
		Revisions           map[string]revision
		Rows                []nativeRow
	}
	raw, err := os.ReadFile("testdata/conformance/public_runas.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	// r1 captured R001-R007/C001-C007; r2 adds R008-R019/C008-C019 with
	// per-row companions on both routes and APIs.
	r2 := data.Revisions["r2"]
	if len(data.Rows) != 152 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		len(data.Revisions) != 1 {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.Route + "/" + row.APIVersion + "/" + row.ID
		if seen[key] || row.Code == "" || row.Expected == "" || row.Expected == "?" {
			t.Fatalf("invalid native row: %s", key)
		}
		seen[key] = true
		ev := data.NativeEvidence[row.Route][row.APIVersion]
		if row.Revision == "r2" {
			ev = r2.NativeEvidence[row.Route][row.APIVersion]
			compiled := "compiled"
			if row.Compile {
				compiled = row.Expected
			}
			if !strings.Contains(ev.CompileTSV, row.ID+"\t"+compiled+"\n") || row.Source == "" {
				t.Fatalf("missing r2 provenance: %s", key)
			}
			if row.Route == "isTest" && len(row.Companions) == 0 {
				t.Fatalf("missing r2 companions: %s", key)
			}
		} else if row.Revision != "" {
			t.Fatalf("unknown revision: %s", key)
		}
		native := ev.OrgTSV
		if row.Route == "isTest" && row.Compile {
			native = ev.CompileTSV
		}
		if !strings.Contains(native, row.ID+"\t"+row.Expected+"\n") {
			t.Fatalf("row differs from native TSV: %s", key)
		}
	}
	observation := func(items []diagnostic.Diagnostic, withLine bool) string {
		messages := []string{}
		for _, d := range items {
			if d.Severity != diagnostic.Error {
				continue
			}
			text := d.NativeMessage
			if text == "" {
				text = d.Message
			}
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			if d.NativeLine != nil {
				line = *d.NativeLine
			}
			if withLine && line != 0 {
				text = fmt.Sprintf("line %d: %s", line, text)
			}
			// Named analysis checks the same call through IR and source paths.
			// Preserve every distinct diagnostic; repeated identical text is one
			// native compiler observation, not a different rejection.
			duplicate := false
			for _, previous := range messages {
				if previous == text {
					duplicate = true
				}
			}
			if !duplicate {
				messages = append(messages, text)
			}
		}
		if len(messages) == 0 {
			return "compiled"
		}
		return "COMPILE_ERROR\t" + strings.Join(messages, "; ")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			build := func(sources map[string]string) typesys.Index {
				t.Helper()
				root := t.TempDir()
				names := []string{}
				for name := range sources {
					names = append(names, name)
				}
				sort.Strings(names)
				paths := []string{}
				for _, name := range names {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, sources[name])
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatalf("native source parser: %#v", index.Diagnostics)
				}
				return index
			}
			ev := data.NativeEvidence["isTest"][api]
			if len(ev.Companions) != 3 {
				t.Fatal("missing captured companions")
			}
			// Each captured row ran with only its own top-level companions.
			// Rows that share an identical companion set run as one project.
			companionsFor := func(row nativeRow) map[string]string {
				if row.Revision == "r2" {
					return row.Companions
				}
				return ev.Companions
			}
			companionKey := func(companions map[string]string) string {
				names := []string{}
				for name, source := range companions {
					names = append(names, name+"="+source)
				}
				sort.Strings(names)
				return strings.Join(names, "|")
			}
			// Block-local declarations become anonymous declaration context; the
			// masked block keeps every line position for native line numbers.
			anonymous := func(source string) (typesys.Index, string, string) {
				t.Helper()
				parsed := apexast.NewParser().ParseSource("PublicRunAsAnonymous.cls", source)
				if (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
					t.Fatalf("anonymous parser: %#v", parsed.Diagnostics)
				}
				declarations := map[string]string{}
				masked := []byte(source)
				for _, decl := range parsed.Declarations {
					if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
						continue
					}
					start, end := decl.Range.Start.Offset, decl.Range.End.Offset
					padding := []byte(source[:start])
					for i := range padding {
						if padding[i] != '\n' && padding[i] != '\r' {
							padding[i] = ' '
						}
					}
					declarations[decl.Name] = string(padding) + source[start:end]
					for i := start; i < end; i++ {
						if masked[i] != '\n' && masked[i] != '\r' {
							masked[i] = ' '
						}
					}
				}
				index := sema.WithAnonymousDeclarationContext(build(declarations))
				analysis := sema.AnalyzeAnonymousDeclarations(index)
				if !analysis.HasErrors() {
					analysis = sema.AnalyzeAnonymous(index, string(masked), api)
				}
				return index, string(masked), observation(analysis.Diagnostics, true)
			}
			namedGroups := map[string]map[string]string{}
			namedRuntime := map[string]nativeRow{}
			for _, route := range data.Routes {
				matched := 0
				for n := 1; n <= 19; n++ {
					for _, prefix := range []string{"R", "C"} {
						if !seen[fmt.Sprintf("%s/%s/%s%03d", route, api, prefix, n)] {
							t.Fatal("missing route/API row")
						}
					}
				}
				for _, row := range data.Rows {
					if row.Route != route || row.APIVersion != api {
						continue
					}
					if t.Run(route+"/"+row.ID, func(t *testing.T) {
						if route == "isTest" {
							sources := map[string]string{row.Name: row.Source}
							for name, source := range companionsFor(row) {
								sources[name] = source
							}
							index := build(sources)
							actual := observation(sema.Analyze(index).Diagnostics, false)
							expected := "compiled"
							if row.Compile {
								expected = row.Expected
							}
							if actual != expected {
								t.Fatalf("native <%s>, actual <%s>", expected, actual)
							}
							if row.Compile {
								return
							}
							terminal := "System.assert(false, 'P|" + row.ID + "|' + corpusObserved);"
							if strings.Count(row.Source, terminal) != 1 {
								t.Fatal("missing captured terminal observation")
							}
							assertion := "System.assert(" + conformanceApexString(row.Expected) + ".equals(corpusObserved), '" + row.ID + " actual <'+corpusObserved+'>');"
							group := companionKey(companionsFor(row))
							if namedGroups[group] == nil {
								namedGroups[group] = map[string]string{}
								for name, source := range companionsFor(row) {
									namedGroups[group][name] = source
								}
							}
							namedGroups[group][row.Name] = strings.Replace(row.Source, terminal, assertion, 1)
							namedRuntime[row.Name] = row
							return
						}
						source := row.Source
						if row.Revision == "r2" && !row.Compile {
							// r2 submitted each row alone with exception transport. Keep the exact
							// source and replace only the terminal transport throw with an assertion.
							if _, _, actual := anonymous(source); actual != "compiled" {
								t.Fatalf("native <compiled>, actual <%s>", actual)
							}
							transport := "throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));"
							if strings.Count(source, transport) != 1 || !strings.Contains(source, "pq.out('"+row.ID+"',r);") || !strings.Contains(source, row.Code) {
								t.Fatal("missing captured r2 transport")
							}
							observed := "String.join(pq.rows, '; ')"
							source = strings.Replace(source, transport, "System.assert("+conformanceApexString("P|"+row.ID+"|"+row.Expected)+".equals("+observed+"), '"+row.ID+" actual <'+"+observed+"+'>');", 1)
						} else if !row.Compile {
							// The retained runtime source lists every independently submitted row.
							// Keep its exact helper/declarations and select the one captured body.
							lines := strings.Split(data.NativeEvidence[route][api].RuntimeSource, "\n")
							body := ""
							for _, line := range lines {
								if strings.Contains(line, "pq.out('"+row.ID+"', r)") {
									body = line
								}
							}
							if body == "" || !strings.Contains(body, row.Code) {
								t.Fatal("missing captured runtime body")
							}
							source = strings.Join(lines[:8], "\n") + "\n" + body + "\n"
							source = strings.Replace(source, "System.debug(LoggingLevel.ERROR, 'P|' + id + '|' + String.valueOf(v));", "System.assert("+conformanceApexString(row.Expected)+".equals(String.valueOf(v)), id+' actual <'+String.valueOf(v)+'>');", 1)
							source = strings.Replace(source, "System.debug(LoggingLevel.ERROR, 'P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage());", "System.assert("+conformanceApexString(row.Expected)+".equals('EXC|' + e.getTypeName() + '|' + e.getMessage()), id+' actual <'+e.getMessage()+'>');", 1)
						}
						index, masked, actual := anonymous(source)
						expected := "compiled"
						if row.Compile {
							expected = row.Expected
						}
						if actual != expected {
							t.Fatalf("native <%s>, actual <%s>", expected, actual)
						}
						if row.Compile {
							return
						}
						program, err := vm.CompileAnonymousWithOptions(masked, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						if _, err = runner.execute(program); err != nil {
							t.Fatal(err)
						}
					}) {
						matched++
					}
				}
				t.Logf("%s/%s compiler and anonymous runtime matches: %d/38", route, api, matched)
			}
			results := map[string]bool{}
			total := 0
			for _, sources := range namedGroups {
				index := build(sources)
				if result := sema.Analyze(index); result.HasErrors() {
					t.Fatalf("named assertion semantics: %#v", result.Diagnostics)
				}
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				total += run.Summary().Total
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						row, ok := namedRuntime[result.ClassName]
						if !ok || results[result.ClassName] || result.MethodName != row.ID || sources[result.ClassName] == "" {
							t.Fatalf("unexpected named result: %#v", result)
						}
						results[result.ClassName] = true
						if result.Status != testreport.StatusPass || result.Problem != nil {
							t.Errorf("%s native <%s>: %#v", row.ID, row.Expected, result)
						}
					}
				}
			}
			if len(namedGroups) != 5 || len(namedRuntime) != 19 || len(results) != 19 || total != 19 {
				t.Fatalf("named runtime count: groups=%d sources=%d results=%d total=%d", len(namedGroups), len(namedRuntime), len(results), total)
			}
		})
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/hex"
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

// Blocker repro rows (Batchable Iterable scope, QueryLocator as
// Iterable<Object>, RecordType.SobjectType, Iterator<T> arguments), the B41 H1
// rows with their anonymous twins (H1X*-A), and the area M1
// Iterator/SObjectType boundary rows. Sources are the deployed bytes; only the
// terminal observation transport is replaced for local execution.
func TestIterableAssignabilityOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID, APIVersion, Route, Behavior, Role, Twin, Capture, TestClass string
		Source, ExpectedCompilation, ExpectedRuntime                    string
		Sources, SourceSha256                                           map[string]string
		NativeDiagnostic                                                *struct {
			Class string
			Line  int
		}
		// Top-level classes deployed to the org before an anonymous row runs.
		OrgClasses map[string]string
		// A carried difference: logged, not asserted (doc-1).
		Carry *struct{ Owner, Reason string }
	}
	var data struct {
		APIVersions, Routes []string
		Captures            map[string]string
		AbsentRoutes        []struct {
			Route, Reason string
			Rows          []string
		}
		// Native rows left out of the export, each with its carry reason and owner.
		ParkedRows []struct {
			Route, Owner, Reason string
			Rows                 []string
		}
		Rows []nativeRow
	}
	raw, err := os.ReadFile("testdata/conformance/iterable_assignability.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	// Capture provenance stays in glade-tools; rows keep the native row ids.
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || len(data.Rows) != 274 ||
		len(data.Captures) != 5 || data.Captures["b41-h1-test"] == "" || data.Captures["blocker-repros"] == "" || data.Captures["area-M1"] == "" || data.Captures["b41-h1"] == "" ||
		data.Captures["b41-h1-anon"] == "" {
		t.Fatal("incomplete native matrix")
	}
	if len(data.AbsentRoutes) != 1 || data.AbsentRoutes[0].Route != "anonymous" || len(data.AbsentRoutes[0].Rows) != 19 ||
		!strings.Contains(data.AbsentRoutes[0].Reason, "captured only as named check-only deploys") {
		t.Fatal("missing anonymous route boundary for the blocker repro rows")
	}
	if len(data.ParkedRows) != 0 {
		t.Fatal("unexpected parked iterable rows")
	}
	absent := map[string]bool{}
	for _, id := range data.AbsentRoutes[0].Rows {
		absent[id] = true
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.Route + "/" + row.APIVersion + "/" + row.ID
		if row.Carry != nil {
			t.Fatalf("unexpected carried iterable row: %s", key)
		}
		capture := "area-M1"
		if absent[row.ID] {
			capture = "blocker-repros"
		}
		if strings.HasPrefix(row.ID, "H1") {
			// Each b41-h1 row is one route; anonymous ids end in -A.
			capture = "b41-h1"
			if strings.HasPrefix(row.ID, "H1X") {
				capture = "b41-h1-anon"
			}
			if strings.HasPrefix(row.ID, "H1T") {
				capture = "b41-h1-test"
			}
			if (row.Route == "anonymous") != strings.HasSuffix(row.ID, "-A") {
				t.Fatalf("b41-h1 route mismatch: %s", key)
			}
		}
		if len(row.OrgClasses) > 0 && row.Route != "anonymous" {
			t.Fatalf("org classes outside the anonymous route: %s", key)
		}
		if seen[key] || row.Capture != capture || (absent[row.ID] && row.Route != "isTest") || row.ExpectedCompilation == "" ||
			(row.ExpectedCompilation == "compiled") == (row.ExpectedRuntime == "") {
			t.Fatalf("invalid native row: %s", key)
		}
		seen[key] = true
		sources := row.Sources
		if row.Route == "anonymous" {
			sources = map[string]string{"anonymous": row.Source}
			for name, code := range row.OrgClasses {
				sources[name] = code
			}
		}
		if len(sources) == 0 || len(sources) != len(row.SourceSha256) {
			t.Fatalf("missing native sources: %s", key)
		}
		for name, code := range sources {
			digest := sha256.Sum256([]byte(code))
			if hex.EncodeToString(digest[:]) != row.SourceSha256[name] {
				t.Fatalf("captured source digest changed: %s/%s", key, name)
			}
		}
	}
	for _, api := range data.APIVersions {
		for id := range absent {
			if !seen["isTest/"+api+"/"+id] {
				t.Fatalf("missing blocker row %s@%s", id, api)
			}
		}
		for _, id := range strings.Fields("I01 I101 I02 I102 I03 I103 S16 S116") {
			for _, route := range data.Routes {
				if !seen[route+"/"+api+"/"+id] {
					t.Fatalf("missing area M1 row %s/%s@%s", route, id, api)
				}
			}
		}
		// D35: the anonymous twins of the named cast (PA01/PA03/PA04), token
		// (PB05-PB07) and QueryLocator return (H1K2T1/H1K2T2) rows.
		for _, id := range strings.Fields("H1XC1-A H1XC2-A H1XC3-A H1XT5-A H1XT6-A H1XT7-A H1XK1-A H1XK2-A") {
			if !seen["anonymous/"+api+"/"+id] {
				t.Fatalf("missing anonymous twin %s@%s", id, api)
			}
		}
		// The @IsTest twins H1TX<k> of H1X<k>-A, and both routes of each H1TB boundary.
		for key := range seen {
			route, rest, _ := strings.Cut(key, "/")
			rowAPI, id, _ := strings.Cut(rest, "/")
			if rowAPI != api || route != "isTest" {
				continue
			}
			twin := ""
			if strings.HasPrefix(id, "H1TX") {
				twin = "H1X" + strings.TrimPrefix(id, "H1TX") + "-A"
			} else if strings.HasPrefix(id, "H1TB") {
				twin = id + "-A"
			}
			if twin != "" && !seen["anonymous/"+api+"/"+twin] {
				t.Fatalf("missing route twin %s for %s@%s", twin, id, api)
			}
		}

	}

	observation := func(items []diagnostic.Diagnostic, withLine bool) string {
		var messages []string
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
			// The IR and source checks can report one native rejection twice.
			duplicate := false
			for _, previous := range messages {
				duplicate = duplicate || previous == text
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
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			build := func(t *testing.T, sources map[string]string) typesys.Index {
				t.Helper()
				root := t.TempDir()
				var names, paths []string
				for name := range sources {
					names = append(names, name)
				}
				sort.Strings(names)
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
			// Anonymous block-local classes become declaration context; the
			// masked block keeps line positions for native line numbers.
			anonymous := func(t *testing.T, source string, orgClasses map[string]string) (typesys.Index, string, string) {
				t.Helper()
				parsed := apexast.NewParser().ParseSource("IterableAssignabilityAnonymous.cls", source)
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
				index := sema.WithAnonymousDeclarationContext(build(t, declarations))
				if len(orgClasses) > 0 {
					// Only the block's own declarations are transient; org
					// classes stay ordinary top-level classes.
					all := map[string]string{}
					for name, code := range declarations {
						all[name] = code
					}
					for name, code := range orgClasses {
						all[name] = code
					}
					index = build(t, all)
					index.Types = append([]typesys.TypeSymbol(nil), index.Types...)
					for i := range index.Types {
						if _, org := orgClasses[index.Types[i].Name]; !org && !index.Types[i].Dependency {
							index.Types[i].Modifiers = append(append([]string(nil), index.Types[i].Modifiers...), vm.AnonymousClassModifier)
						}
					}
				}
				analysis := sema.AnalyzeAnonymousDeclarations(index)
				if !analysis.HasErrors() {
					analysis = sema.AnalyzeAnonymous(index, string(masked), api)
				}
				return index, string(masked), observation(analysis.Diagnostics, true)
			}
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				t.Run(row.Route+"/"+row.ID, func(t *testing.T) {
					if row.Route == "anonymous" {
						if _, _, actual := anonymous(t, row.Source, row.OrgClasses); actual != row.ExpectedCompilation {
							t.Fatalf("native <%s>, actual <%s>", row.ExpectedCompilation, actual)
						}
						if row.ExpectedRuntime == "" {
							return
						}
						// area-M1 used the exception transport; b41-h1 ends with a tagged assertion.
						transport, observed, expected := "throw new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));", "String.join(pq.rows, '; ')", "P|"+row.ID+"|"+row.ExpectedRuntime
						if strings.HasPrefix(row.Capture, "b41-h1") {
							// The native message concatenates observed, so a null reads as 'null'.
							transport, observed, expected = "System.assert(false, 'P|"+row.ID+"|' + observed);", "('' + observed)", row.ExpectedRuntime
						} else if !strings.Contains(row.Source, "pq.out('"+row.ID+"',r);") {
							t.Fatal("missing captured anonymous observation")
						}
						if strings.Count(row.Source, transport) != 1 {
							t.Fatal("missing captured anonymous transport")
						}
						source := strings.Replace(row.Source, transport, "System.assert("+conformanceApexString(expected)+".equals("+observed+"), '"+row.ID+" actual <'+"+observed+"+'>');", 1)
						index, masked, actual := anonymous(t, source, row.OrgClasses)
						if actual != "compiled" {
							t.Fatalf("local assertion semantics: %s", actual)
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
						return
					}
					analysis := sema.Analyze(build(t, row.Sources))
					if actual := observation(analysis.Diagnostics, false); actual != row.ExpectedCompilation {
						t.Fatalf("native <%s>, actual <%s>: %#v", row.ExpectedCompilation, actual, analysis.Diagnostics)
					}
					if row.NativeDiagnostic != nil {
						matched := false
						for _, d := range analysis.Diagnostics {
							matched = matched || (d.Severity == diagnostic.Error && strings.EqualFold(strings.TrimSuffix(filepath.Base(d.File), ".cls"), row.NativeDiagnostic.Class) && d.Range != nil && d.Range.Start.Line == row.NativeDiagnostic.Line)
						}
						if !matched {
							t.Fatalf("native rejection is %s line %d: %#v", row.NativeDiagnostic.Class, row.NativeDiagnostic.Line, analysis.Diagnostics)
						}
					}
					if row.ExpectedRuntime == "" {
						return
					}
					terminal, observed := "System.assert(false, 'P|"+row.ID+"|' + observed);", "('' + observed)"
					if row.Capture == "area-M1" {
						terminal = "System.assert(false,'P|" + row.ID + "|'+observed);"
					}
					test := row.Sources[row.TestClass]
					if strings.Count(test, terminal) != 1 {
						t.Fatal("missing captured terminal observation")
					}
					sources := map[string]string{}
					for name, code := range row.Sources {
						sources[name] = code
					}
					sources[row.TestClass] = strings.Replace(test, terminal, "System.assert("+conformanceApexString(row.ExpectedRuntime)+".equals("+observed+"), '"+row.ID+" actual <'+"+observed+"+'>');", 1)
					index := build(t, sources)
					if result := sema.Analyze(index); result.HasErrors() {
						t.Fatalf("local assertion semantics: %#v", result.Diagnostics)
					}
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 {
						t.Fatalf("captured @IsTest method missing: %#v", run.Suites)
					}
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							if !strings.EqualFold(result.ClassName, row.TestClass) || result.Status != testreport.StatusPass || result.Problem != nil {
								problem := ""
								if result.Problem != nil {
									problem = result.Problem.Message
								}
								t.Errorf("%s native <%s>: %s %s %s", row.ID, row.ExpectedRuntime, result.ClassName, result.Status, problem)
							}
						}
					}
				})
			}
		})
	}
}

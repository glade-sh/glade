package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type collectionIncrementSource struct {
	Name, Code, SHA256 string
}

// Compile rows measure deployment only. Runtime rows independently measure
// anonymous and @IsTest values; compilation never supplies a runtime answer.
// Revision r4 rows record either the compiler rejection or
// "compiled" and the stored values, on both routes.
func TestCollectionIncrementOrgConformance(t *testing.T) {
	type source = collectionIncrementSource
	type observation struct {
		Expected string
		Sources  []source
	}
	var matrix struct {
		APIVersions []string
		Cases       []struct {
			ID, Tier, Route, Revision string
			ByAPI                     map[string]observation
		}
	}
	raw, err := os.ReadFile("testdata/conformance/collection_increment.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	if strings.Join(matrix.APIVersions, ",") != "62.0,67.0" || len(matrix.Cases) != 356 {
		t.Fatal("incomplete native matrix")
	}
	compileText := func(items []diagnostic.Diagnostic) string {
		for _, item := range items {
			if item.Severity == diagnostic.Error {
				message := item.Message
				if item.NativeMessage != "" {
					message = item.NativeMessage
				}
				return "COMPILE_ERROR\t" + message
			}
		}
		return "compiled"
	}
	// r4: the anonymous route has R4-01..R4-43 and twins C4-01..C4-43; the
	// @IsTest route has this representative subset of pairs.
	r4Rows := map[string][]int{"r4": nil, "named-r4": {1, 8, 10, 13, 16, 22, 26, 29, 34, 35, 39, 40, 43}}
	for n := 1; n <= 43; n++ {
		r4Rows["r4"] = append(r4Rows["r4"], n)
	}
	for _, api := range matrix.APIVersions {
		t.Run(api, func(t *testing.T) {
			build := func(sources map[string]string, anonymous bool) typesys.Index {
				t.Helper()
				root := t.TempDir()
				var paths []string
				for name, code := range sources {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, code)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
				if anonymous {
					index = sema.WithAnonymousDeclarationContext(index)
				}
				return index
			}
			seen := map[string]bool{}
			counts := map[string]int{}
			for _, row := range matrix.Cases {
				key := row.Tier + "/" + row.ID
				if seen[key] || len(row.ByAPI) != 2 {
					t.Fatalf("duplicate or incomplete row: %s", row.ID)
				}
				seen[key] = true
				counts[row.Tier]++
				t.Run(row.Tier+"/"+row.ID, func(t *testing.T) {
					captured := row.ByAPI[api]
					_, r4 := r4Rows[row.Tier]
					if r4 != (row.Revision == "r4") {
						t.Fatal("r4 rows need the r4 revision")
					}
					if len(captured.Sources) == 0 || captured.Expected == "" || captured.Expected == "?" {
						t.Fatal("missing native source or answer")
					}
					for _, unit := range captured.Sources {
						actual := sha256.Sum256([]byte(unit.Code))
						if hex.EncodeToString(actual[:]) != unit.SHA256 {
							t.Fatal("captured source digest changed")
						}
					}
					if r4 {
						if row.Route != map[string]string{"r4": "anonymous", "named-r4": "isTest"}[row.Tier] {
							t.Fatal("unexpected native route")
						}
						collectionIncrementR4(t, api, row.ID, row.Route, captured.Expected, captured.Sources, build, compileText)
						return
					}
					if row.Tier == "compile" || row.Tier == "compile-final" || row.Tier == "runtime-named" {
						namedRuntime := row.Tier == "runtime-named"
						if (!namedRuntime && row.Route != "named") || (namedRuntime && row.Route != "isTest") {
							t.Fatal("compile capture must use the named deployment route")
						}
						sources := map[string]string{}
						terminalAssertions := 0
						for _, unit := range captured.Sources {
							code := unit.Code
							if namedRuntime {
								// Retain the captured @IsTest method and helper classes.
								// Replace only its terminal observation transport.
								terminal := "System.assert(false, 'P|" + row.ID + "|' + pq.value);"
								terminalAssertions += strings.Count(code, terminal)
								code = strings.ReplaceAll(code, terminal, "System.assertEquals("+conformanceApexString(captured.Expected)+",pq.value);")
							}
							sources[unit.Name] = code
						}
						if namedRuntime && terminalAssertions != 1 {
							t.Fatal("missing or repeated native @IsTest observation transport")
						}
						index := build(sources, false)
						items := index.Diagnostics
						if !index.HasErrors() {
							items = sema.Analyze(index).Diagnostics
						}
						if !namedRuntime {
							if actual := compileText(items); actual != captured.Expected {
								t.Fatalf("expected <%s> actual <%s>: %v", captured.Expected, actual, items)
							}
							return // Uncalled methods: never execute a compile-only row.
						}
						if actual := compileText(items); actual != "compiled" {
							t.Fatalf("expected <%s> actual <%s>: %v", captured.Expected, actual, items)
						}
						namedCases := Discover(index, Options{})
						if len(namedCases) != 1 || namedCases[0].MethodName != row.ID {
							t.Fatalf("expected one captured @IsTest method: %v", namedCases)
						}
						methods, methodErrors := compileTestMethodsForIndex(&index, namedCases)
						programs, programErrors := compileTestInvokePrograms(namedCases)
						key := testCaseKey(namedCases[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], namedCases[0].ClassName); err != nil {
							t.Fatal(err)
						}
						return
					}
					if row.Tier != "runtime" || row.Route != "anonymous" || len(captured.Sources) != 1 {
						t.Fatal("unexpected native route")
					}
					// Replace only the capture transport with an exact assertion.
					// Keep every operand, result use and exception handler unchanged.
					code, transport, found := strings.Cut(captured.Sources[0].Code, "class FamilyProbeTransportException")
					if !found || !strings.Contains(transport, "GLADE_FAMILY_ROWS|") {
						t.Fatal("missing captured transport")
					}
					code += "System.assertEquals(1,pq.rows.size());\nSystem.assertEquals(" + conformanceApexString("P|"+row.ID+"|"+captured.Expected) + ",pq.rows[0]);\n"
					declarations, body := apexMetadataDeclarations(t, code)
					index := build(declarations, true)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, api)})
					if err != nil {
						t.Fatal(err)
					}
					runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			for tier, numbers := range r4Rows {
				if counts[tier] != 2*len(numbers) {
					t.Fatalf("%s count: %d", tier, counts[tier])
				}
				for _, n := range numbers {
					if !seen[fmt.Sprintf("%s/R4-%02d", tier, n)] || !seen[fmt.Sprintf("%s/C4-%02d", tier, n)] {
						t.Fatal("missing native r4 row")
					}
				}
			}
			for tier, bounds := range map[string][2]int{"compile": {1, 176}, "compile-final": {177, 196}, "runtime": {1, 24}, "runtime-named": {1, 24}} {
				if counts[tier] != bounds[1]-bounds[0]+1 {
					t.Fatalf("%s count: %d", tier, counts[tier])
				}
				prefix := "C"
				if strings.HasPrefix(tier, "runtime") {
					prefix = "R"
				}
				for n := bounds[0]; n <= bounds[1]; n++ {
					if !seen[tier+"/"+fmt.Sprintf("%s%03d", prefix, n)] {
						t.Fatal("missing native row")
					}
				}
			}
		})
	}
}

// collectionIncrementR4 checks one r4 row. A rejected row must give the native
// compiler message. A compiled row must compile and store exactly the native
// values; only its capture transport is replaced by an exact assertion.
func collectionIncrementR4(t *testing.T, api, id, route, expected string, units []collectionIncrementSource, build func(map[string]string, bool) typesys.Index, compileText func([]diagnostic.Diagnostic) string) {
	t.Helper()
	marker := strings.ReplaceAll(id, "-", "")
	values, compiled := strings.CutPrefix(expected, "compiled\t")
	if !compiled && !strings.HasPrefix(expected, "COMPILE_ERROR\t") {
		t.Fatalf("unexpected native answer: %q", expected)
	}
	want := expected
	if compiled {
		want = "compiled"
	}
	switch route {
	case "isTest":
		sources := map[string]string{}
		terminal := "System.assert(false, 'P|" + marker + "|' + pq.value);"
		terminals := 0
		for _, unit := range units {
			terminals += strings.Count(unit.Code, terminal)
			sources[unit.Name] = strings.ReplaceAll(unit.Code, terminal, "System.assertEquals("+conformanceApexString(values)+",pq.value);")
		}
		if terminals != 1 {
			t.Fatal("missing or repeated native @IsTest observation transport")
		}
		index := build(sources, false)
		items := index.Diagnostics
		if !index.HasErrors() {
			items = sema.Analyze(index).Diagnostics
		}
		if actual := compileText(items); actual != want {
			t.Fatalf("expected <%s> actual <%s>: %v", want, actual, items)
		}
		if !compiled {
			return
		}
		namedCases := Discover(index, Options{})
		if len(namedCases) != 1 || namedCases[0].MethodName != marker {
			t.Fatalf("expected one captured @IsTest method: %v", namedCases)
		}
		methods, methodErrors := compileTestMethodsForIndex(&index, namedCases)
		programs, programErrors := compileTestInvokePrograms(namedCases)
		key := testCaseKey(namedCases[0])
		if methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		if _, err := machine.ExecuteInClass(programs[key], namedCases[0].ClassName); err != nil {
			t.Fatal(err)
		}
	case "anonymous":
		if len(units) != 1 {
			t.Fatal("unexpected native route")
		}
		code, transport, found := strings.Cut(units[0].Code, "class FamilyProbeTransportException")
		if !found || !strings.Contains(transport, "GLADE_FAMILY_ROWS|") {
			t.Fatal("missing captured transport")
		}
		if compiled {
			code += "System.assertEquals(1,pq.rows.size());\nSystem.assertEquals(" + conformanceApexString("P|"+marker+"|"+values) + ",pq.rows[0]);\n"
		}
		declarations, body := apexMetadataDeclarations(t, code)
		index := build(declarations, true)
		items := append([]diagnostic.Diagnostic(nil), index.Diagnostics...)
		if !index.HasErrors() {
			items = append(items, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
			items = append(items, sema.AnalyzeAnonymous(index, body, api).Diagnostics...)
		}
		if actual := compileText(items); actual != want {
			t.Fatalf("expected <%s> actual <%s>: %v", want, actual, items)
		}
		if !compiled {
			return
		}
		program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, api)})
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
		if _, err := runner.execute(program); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unexpected native route")
	}
}

// TestCollectionIncrementRuntimeBoundary pins the runtime to the statements a
// captured rule approves for the element or field type, operator and route.
// Each uncaptured twin keeps the name-path compile error, comments included;
// each captured one lowers and stores. These are local preservation checks,
// not Salesforce observations.
func TestCollectionIncrementRuntimeBoundary(t *testing.T) {
	const box = "public class Box { public Decimal amount = 2; public Long count = 2; }\n"
	const accounts = "Map<String,Account> m = new Map<String,Account>{'one' => new Account(AnnualRevenue = 2)};\nString key = 'one';\nAccount acc = new Account(Name = 'one');\n"
	const boxes = "Map<String,Box> m = new Map<String,Box>{'one' => new Box()};\n"
	const longs = "List<Long> l = new List<Long>{2L};\n"
	named := []struct{ body, check string }{
		{accounts + "++m.get('one').AnnualRevenue;", "System.assert(m.get('one').AnnualRevenue == 3);"}, // R4-16
		{longs + "--l[0];", "System.assertEquals(1L, l[0]);"},                                           // R4-43
		{boxes + "++m.get('one').count;", "System.assertEquals(3L, m.get('one').count);"},               // R4-29
		{accounts + "--m.get('one').AnnualRevenue;", ""},                                                // R4-04 is anonymous-only
		{accounts + "--/*c*/m.get('one').AnnualRevenue;", ""},
		{accounts + "++m.get(key).AnnualRevenue;", ""},      // R4-07 is anonymous-only
		{accounts + "++m.get(acc.Name).AnnualRevenue;", ""}, // R4-11 is anonymous-only
		{longs + "++l[0];", ""}, // R4-43 holds only --
		{longs + "++/*c*/l[0];", ""},
		{boxes + "++m.get('one').amount;", ""}, // R4-29 holds Long, Double and String fields
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range named {
			body := "\n" + row.body + "\n" + row.check + "\n"
			source := "@IsTest private class Probe {\n@IsTest static void check() {" + body + "}\n}\n"
			root := t.TempDir()
			paths := []string{filepath.Join(root, "Probe.cls"), filepath.Join(root, "Box.cls")}
			writeFile(t, paths[0], source)
			writeFile(t, paths[1], box)
			for _, path := range paths {
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
			cases := Discover(index, Options{})
			if len(cases) != 1 {
				t.Fatalf("%s %s: %v", api, row.body, cases)
			}
			key := testCaseKey(cases[0])
			methods, errs := compileTestMethodsForIndex(&index, cases)
			extracted, err := extractMethodBody(source, cases[0].BodyRange)
			if err != nil {
				t.Fatal(err)
			}
			_, namePath := vm.CompileAnonymous(extracted)
			if namePath == nil {
				t.Fatalf("%s %s: the name path lowered it", api, row.body)
			}
			if row.check == "" {
				if errs[key] == nil || errs[key].Error() != namePath.Error() {
					t.Fatalf("%s %s: got %v, want the name-path error %v", api, row.body, errs[key], namePath)
				}
				continue
			}
			programs, programErrs := compileTestInvokePrograms(cases)
			if errs[key] != nil || programErrs[key] != nil {
				t.Fatalf("%s %s: %v %v", api, row.body, errs[key], programErrs[key])
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
				t.Fatalf("%s %s: %v", api, row.body, err)
			}
		}
		// Anonymous: R4-42 lowers; a Date element is uncaptured, so sema rejects
		// the body and the runtime keeps the name-path error.
		for body, lowered := range map[string]bool{longs + "++l[0];\nSystem.assertEquals(3L, l[0]);": true, "List<Date> l = new List<Date>{Date.today()};\n++l[0];": false} {
			approved := sema.ApprovedAnonymousPrefixStatements(typesys.Index{}, body, api)
			analysis := sema.AnalyzeAnonymous(typesys.Index{}, body, api)
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: approved})
			_, namePath := vm.CompileAnonymous(body)
			if !lowered {
				if !analysis.HasErrors() || err == nil || err.Error() != namePath.Error() {
					t.Fatalf("%s anonymous %q: %v %v", api, body, analysis.Diagnostics, err)
				}
				continue
			}
			if analysis.HasErrors() || err != nil {
				t.Fatalf("%s anonymous %q: %v %v", api, body, analysis.Diagnostics, err)
			}
			org := orgFromIndex(typesys.Index{})
			org.APIVersion = api
			runner := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Fatalf("%s anonymous %q: %v", api, body, err)
			}
		}
	}
}

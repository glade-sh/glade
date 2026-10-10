package apextest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native API endpoints include the distinct anonymous and granted @IsTest
// routes. The exported metadata is the entire credential-free CI fixture.
func TestSecurityAccessOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, NamedCode     string
		Compile, NativeNull     bool
		Expected, NamedExpected map[string]string
		NamedProvenance         map[string]string
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Declarations, Metadata map[string]string
		Cases, Controls        []familyCase
		NamespaceControls      []securityNamespaceControl
		NestedControls         []securityNamespaceControl
		Remaining              []struct{ ID, Owner, Reason string }
		ProvenanceCounts       map[string]struct {
			AnonymousNative, NamedNative, NamedAnonymousBackedCompiler, AnonymousControls int
		}
	}
	raw, err := os.ReadFile("testdata/conformance/security_access.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 265 || len(data.Controls) != 45 || len(data.NamespaceControls) != 8 || len(data.NestedControls) != 8 || len(data.Remaining) != 0 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d family rows + %d anonymous controls + %d carried at %v", len(data.Cases), len(data.Controls), len(data.Remaining), data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if tc.ID == "" || seen[tc.ID] {
			t.Fatalf("invalid/repeated row: %#v", tc)
		}
		seen[tc.ID] = true
		for _, api := range data.APIVersions {
			if tc.Expected[api] == "" || tc.Expected[api] == "?" || tc.NamedExpected[api] == "" || tc.NamedExpected[api] == "?" {
				t.Fatalf("uncaptured %s at %s", tc.ID, api)
			}
		}
	}
	for _, tc := range data.Controls {
		if tc.ID == "" || seen[tc.ID] || len(tc.NamedExpected) != 0 {
			t.Fatalf("invalid/repeated anonymous control: %#v", tc)
		}
		seen[tc.ID] = true
		for _, api := range data.APIVersions {
			if tc.Expected[api] == "" || tc.Expected[api] == "?" {
				t.Fatalf("uncaptured control %s at %s", tc.ID, api)
			}
		}
	}
	for _, api := range data.APIVersions {
		nativeNamed, anonymousBacked := 0, 0
		for _, tc := range data.Cases {
			switch tc.NamedProvenance[api] {
			case "native-named":
				nativeNamed++
			case "anonymous-backed-compiler":
				if tc.NamedExpected[api] != tc.Expected[api] || !strings.HasPrefix(tc.Expected[api], "COMPILE_ERROR\tline ") {
					t.Fatalf("invalid anonymous-backed compiler check %s at %s", tc.ID, api)
				}
				anonymousBacked++
			default:
				t.Fatalf("missing/unknown named provenance %s at %s: %q", tc.ID, api, tc.NamedProvenance[api])
			}
		}
		counts := data.ProvenanceCounts[api]
		if counts.AnonymousNative != len(data.Cases) || counts.AnonymousControls != len(data.Controls) || counts.NamedNative != nativeNamed || counts.NamedAnonymousBackedCompiler != anonymousBacked {
			t.Fatalf("provenance counts at %s: %#v, observed named %d native + %d anonymous-backed", api, counts, nativeNamed, anonymousBacked)
		}
		t.Logf("API %s provenance: %d native anonymous rows + %d anonymous controls; %d native named results + %d anonymous-backed named compiler checks", api, len(data.Cases), len(data.Controls), nativeNamed, anonymousBacked)
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || strings.Contains(row.Owner, "A28") || row.Reason == "" {
			t.Fatalf("invalid carry: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  if(expectedNull){System.assert(v==null,id+' expected raw null actual <'+observedText+'>');return;}
  if(expectedText=='null'){System.assert(v!=null,id+' expected String null, actual raw null');}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	nativeLinePattern := regexp.MustCompile(`^COMPILE_ERROR\tline ([0-9]+): `)
	nativeLine := func(expected string) int {
		if match := nativeLinePattern.FindStringSubmatch(expected); match != nil {
			line, _ := strconv.Atoi(match[1])
			return line
		}
		return 0
	}
	compileText := func(result sema.Result) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			message := item.Message
			if item.NativeMessage != "" {
				message = item.NativeMessage
			}
			observed := fmt.Sprintf("line %d: %s", line, strings.ReplaceAll(message, "\n", " "))
			if len(observed) > 300 {
				observed = observed[:300]
			}
			return "COMPILE_ERROR\t" + observed
		}
		return "ACCEPTED"
	}
	body := func(tc familyCase, expected string, named bool) string {
		code := tc.Code
		if named && tc.NamedCode != "" {
			code = tc.NamedCode
		}
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + code
		}
		return prefix + "Object r; try {" + code + "}catch(Exception e){r='EXC|'+e.getTypeName()+'|'+e.getMessage();} pq.out('" + tc.ID + "',r);"
	}
	rejectedSource := func(tc familyCase, expected string, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class SecurityAccessRejected" + tc.ID + " {\n@IsTest static void observed(){\n"
		}
		prefix += "P pq=new P();\n"
		line := strings.Count(prefix, "\n") + 1
		if nativeLine(expected) < line {
			t.Fatalf("native rejection line precedes body: %s %q", tc.ID, expected)
		}
		prefix += strings.Repeat("\n", nativeLine(expected)-line)
		if !tc.Compile {
			prefix += "Object r; "
		}
		prefix += tc.Code
		if named {
			prefix += "\n}\n}\n"
		}
		return prefix
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "security access", api)
			t.Run("namespace-shadow-controls", func(t *testing.T) {
				checkSecurityNamespaceControls(t, api, data.NamespaceControls, 4, counts)
			})
			t.Run("nested-type-shadow-controls", func(t *testing.T) {
				checkSecurityNamespaceControls(t, api, data.NestedControls, 8, counts)
			})
			root := t.TempDir()
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, path), source)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			p.SourceAPIVersion = api
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
			}
			var names, paths []string
			for name := range declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				pp := p
				pp.ApexFiles = append([]string{}, paths...)
				if extra != "" {
					pp.ApexFiles = append(pp.ApexFiles, extra)
				}
				return typesys.Build(pp, schema)
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			// Match exec's transient declaration path: no metadata sidecars,
			// with the anonymous source API retained by the captured index.
			anonymousRoot := t.TempDir()
			anonymousProject := p
			anonymousProject.ApexFiles = nil
			for _, name := range names {
				path := filepath.Join(anonymousRoot, name+".cls")
				writeFile(t, path, declarations[name])
				anonymousProject.ApexFiles = append(anonymousProject.ApexFiles, path)
			}
			anonymousIndex := sema.WithAnonymousDeclarationContext(typesys.Build(anonymousProject, schema))
			if analysis := sema.AnalyzeAnonymousDeclarations(anonymousIndex); analysis.HasErrors() {
				t.Fatalf("anonymous helper semantics: %v", analysis.Diagnostics)
			}
			anonymousRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(anonymousIndex, nil)
			if err != nil {
				t.Fatal(err)
			}
			anonymousRunner := newConformanceRunner(t, anonymousIndex, conformanceRunnerOptions{RequestRuntime: &anonymousRuntime, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class SecurityAccessProbe {\n")
			executable := 0
			rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
			for _, tc := range data.Cases {
				expected := tc.NamedExpected[api]
				if nativeLine(expected) > 0 {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, expected, true))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SecurityAccessProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedCases := Discover(namedIndex, Options{})
			if len(namedCases) != executable {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(methods)
			counters := newRunPerfCounters(false)
			namedByID := make(map[string]TestCase, len(namedCases))
			for _, row := range namedCases {
				id := strings.TrimPrefix(row.MethodName, "observed")
				if row.ClassName != "SecurityAccessProbe" || namedByID[id].MethodName != "" {
					t.Fatalf("unexpected/repeated named row: %#v", row)
				}
				namedByID[id] = row
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) {
						expected := tc.Expected[api]
						source := body(tc, expected, false)
						if nativeLine(expected) > 0 {
							source = rejectedSource(tc, expected, false)
						} else if tc.Compile {
							// C005/C006: exec extracts inline declarations before lowering
							// the body. Their captured implementation is already linked
							// once above; keep the row's remaining bytes and offsets intact.
							masked := []byte(source)
							for _, declaration := range apexast.NewParser().ParseSource("__glade_anonymous.cls", source).Declarations {
								if declaration.Kind != apexast.DeclarationClass {
									continue
								}
								if data.Declarations[declaration.Name] == "" {
									t.Fatalf("unlinked captured declaration %s", declaration.Name)
								}
								for i := declaration.Range.Start.Offset; i < declaration.Range.End.Offset; i++ {
									if masked[i] != '\n' && masked[i] != '\r' {
										masked[i] = ' '
									}
								}
							}
							source = string(masked)
						}
						analysis := sema.AnalyzeAnonymous(anonymousIndex, source, api)
						if nativeLine(expected) > 0 {
							if observed := compileText(analysis); observed != expected {
								t.Fatalf("expected <%s> actual <%s>", expected, observed)
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := anonymousRunner.execute(program, func(machine *vm.VM) { machine.SetTraceEnabled(true) }); err != nil {
							t.Fatal(err)
						}
					})
					if tc.NamedExpected[api] == "" {
						return // Additional controls were captured only through anonymous Apex.
					}
					namedKind := "exact"
					if tc.NamedProvenance[api] == "anonymous-backed-compiler" {
						namedKind = ""
					}
					counts.run(t, "isTest", "@IsTest", namedKind, func(t *testing.T) {
						expected := tc.NamedExpected[api]
						if tc.NamedProvenance[api] == "anonymous-backed-compiler" {
							t.Log("anonymous-backed compiler assertion; native named observation not captured")
						}
						if nativeLine(expected) > 0 {
							path := filepath.Join(root, "SecurityAccessRejected"+tc.ID+".cls")
							writeFile(t, path, rejectedSource(tc, expected, true))
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							if observed := compileText(sema.Analyze(buildIndex(path))); observed != expected {
								t.Fatalf("expected <%s> actual <%s>", expected, observed)
							}
							return
						}
						row := namedByID[tc.ID]
						key := testCaseKey(row)
						seed := namedRunner.org.CloneRuntimeOrg()
						initializeTestOrg(&seed)
						result := runCase(context.Background(), row, methods[key], runtimeMethods[testMethodSourceKey(row.ClassName, row.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
						if result.Status != testreport.StatusPass {
							t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
						}
					})
				})
			}
		})
	}
}

// N048-N055 are independently captured anonymous and named sources. The
// System-bound sources reject at identifier declaration; their good twins run.
// N056-N063 reject anonymous nesting but accept named nested enums. Retain
// independently captured diagnostics, runtime results and source bytes.
type securityNamespaceControl struct {
	ID, Code, NamedCode, NamedClass          string
	Expected, NamedExpected, NamedProvenance map[string]string
}

func checkSecurityNamespaceControls(t *testing.T, api string, rows []securityNamespaceControl, wantNamedRuntime int, counts *conformanceCounts) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	helperPath := filepath.Join(root, "force-app/main/default/classes/P.cls")
	writeFile(t, helperPath, `public class P {
 public String expectedText;
 public void out(String id,Object value){
  String observedText=''+String.valueOf(value);
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`)
	writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	baseIndex := loadTestIndex(t, root)
	baseRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(baseIndex, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{RequestRuntime: &baseRuntime})
	seen := map[string]bool{}
	var namedPaths []string
	for _, row := range rows {
		if row.ID == "" || seen[row.ID] || row.NamedClass == "" || row.Code == "" || row.NamedCode == "" {
			t.Fatalf("invalid namespace control: %#v", row)
		}
		seen[row.ID] = true
		for _, expected := range []string{row.Expected[api], row.NamedExpected[api]} {
			if expected == "" || expected == "?" || expected == "MISSING" {
				t.Fatalf("uncaptured namespace control %s at %s", row.ID, api)
			}
		}
		if row.NamedProvenance[api] == "native-named-runtime" {
			// Replace only the transport assertion. The observed method and all
			// helper/parameter declarations remain the captured source verbatim.
			marker := "System.assert(false,'A28NS|" + row.ID + "|'+String.valueOf(value)+'|A28NSEND');"
			if strings.Count(row.NamedCode, marker) != 1 {
				t.Fatalf("missing/repeated named transport marker %s", row.ID)
			}
			assertion := "String observedText=''+String.valueOf(value); String expectedText=" + conformanceApexString(row.NamedExpected[api]) + "; System.assert(expectedText.equals(observedText),'" + row.ID + " expected <'+expectedText+'> actual <'+observedText+'>');"
			path := filepath.Join(root, "force-app/main/default/classes", row.NamedClass+".cls")
			writeFile(t, path, strings.Replace(row.NamedCode, marker, assertion, 1))
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedPaths = append(namedPaths, path)
		} else if row.NamedProvenance[api] != "native-named-deployment-rejection" {
			t.Fatalf("unknown namespace provenance %s: %q", row.ID, row.NamedProvenance[api])
		}
	}
	if len(namedPaths) != wantNamedRuntime {
		t.Fatalf("native named runtime controls: %d, want %d", len(namedPaths), wantNamedRuntime)
	}
	namedIndex := loadTestIndex(t, root)
	if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
		t.Fatalf("named namespace helpers: %#v", analysis.Diagnostics)
	}
	namedOrg := orgFromIndex(namedIndex)
	namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &namedOrg})
	namedCases := Discover(namedIndex, Options{})
	if len(namedCases) != len(namedPaths) {
		t.Fatalf("namespace named discovery: %d, want %d", len(namedCases), len(namedPaths))
	}
	methods, methodErrors := compileTestMethods(namedCases)
	programs, programErrors := compileTestInvokePrograms(namedCases)
	runtimeMethods := indexTestRuntimeMethods(methods)
	counters := newRunPerfCounters(false)
	namedByClass := map[string]TestCase{}
	for _, tc := range namedCases {
		namedByClass[tc.ClassName] = tc
	}
	// Native rejections must retain the complete diagnostic and its measured
	// position. Read the declaration parser used by the product compile gate;
	// do not route invalid declarations into the runtime/security fallback.
	checkRejection := func(source, expected string, named bool) {
		t.Helper()
		file := apexast.NewParser().ParseSource("NamespaceRejected.cls", source)
		for _, item := range file.Diagnostics {
			if item.Code != "APEXPARSE002" || item.Severity != diagnostic.Error {
				continue
			}
			if item.Range == nil {
				t.Fatal("reserved identifier diagnostic has no range")
			}
			observed := fmt.Sprintf("COMPILE_ERROR\tline %d: %s", item.Range.Start.Line, item.Message)
			if named {
				observed = fmt.Sprintf("COMPILE_ERROR\tline %d, column %d: %s", item.Range.Start.Line, item.Range.Start.Column, item.Message)
			}
			if observed != expected {
				t.Fatalf("expected <%s> actual <%s>", expected, observed)
			}
			return
		}
		if !named {
			// N056-N063: preserve the exception transport's five prelude
			// lines and analyze the declarations in the implicit anonymous
			// enclosing type. Ordinary nesting diagnostics precede execution.
			rowRoot := t.TempDir()
			pp := project.Project{Root: rowRoot, SourceAPIVersion: api}
			for _, decl := range file.Declarations {
				if decl.Kind != apexast.DeclarationClass {
					continue
				}
				prefix := strings.Map(func(r rune) rune {
					if r == '\n' || r == '\r' {
						return r
					}
					return ' '
				}, source[:decl.Range.Start.Offset])
				path := filepath.Join(rowRoot, decl.Name+".cls")
				writeFile(t, path, prefix+source[decl.Range.Start.Offset:decl.Range.End.Offset])
				pp.ApexFiles = append(pp.ApexFiles, path)
			}
			index := sema.WithAnonymousDeclarationContext(typesys.Build(pp, gladeschema.Schema{}))
			analysis := sema.AnalyzeAnonymousDeclarations(index)
			for _, item := range analysis.Diagnostics {
				if item.Severity != diagnostic.Error {
					continue
				}
				line := 0
				if item.Range != nil {
					line = item.Range.Start.Line
				}
				if item.NativeLine != nil {
					line = *item.NativeLine
				}
				message := item.Message
				if item.NativeMessage != "" {
					message = item.NativeMessage
				}
				observed := fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
				if observed != expected {
					t.Fatalf("expected <%s> actual <%s>", expected, observed)
				}
				return
			}
		}
		t.Fatalf("expected <%s>, no native declaration diagnostic", expected)
	}
	for _, row := range rows {
		t.Run(row.ID, func(t *testing.T) {
			counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) {
				expected := row.Expected[api]
				if strings.HasPrefix(expected, "COMPILE_ERROR\t") {
					// probe.compile_script adds five prelude lines at exception transport.
					checkRejection(strings.Repeat("\n", 5)+row.Code, expected, false)
					return
				}
				rowRoot := t.TempDir()
				pp := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{helperPath}}
				masked := []byte(row.Code)
				for _, decl := range apexast.NewParser().ParseSource("AnonymousNamespace.cls", row.Code).Declarations {
					if decl.Kind != apexast.DeclarationClass {
						continue
					}
					path := filepath.Join(rowRoot, decl.Name+".cls")
					writeFile(t, path, row.Code[decl.Range.Start.Offset:decl.Range.End.Offset])
					pp.ApexFiles = append(pp.ApexFiles, path)
					for i := decl.Range.Start.Offset; i < decl.Range.End.Offset; i++ {
						if masked[i] != '\n' && masked[i] != '\r' {
							masked[i] = ' '
						}
					}
				}
				index := sema.WithAnonymousDeclarationContext(typesys.Build(pp, gladeschema.Schema{}))
				if index.HasErrors() {
					t.Fatalf("anonymous declarations: %#v", index.Diagnostics)
				}
				if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
					t.Fatalf("anonymous declaration semantics: %#v", analysis.Diagnostics)
				}
				source := "P pq=new P(); pq.expectedText=" + conformanceApexString(expected) + ";\n" + string(masked)
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatalf("anonymous namespace semantics: %#v", analysis.Diagnostics)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime})
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
			counts.run(t, "isTest", "@IsTest", "exact", func(t *testing.T) {
				expected := row.NamedExpected[api]
				if row.NamedProvenance[api] == "native-named-deployment-rejection" {
					checkRejection(row.NamedCode, expected, true)
					return
				}
				tc := namedByClass[row.NamedClass]
				key := testCaseKey(tc)
				seed := namedRunner.newOrg()
				initializeTestOrg(&seed)
				result := runCase(context.Background(), tc, methods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
				if result.Status != testreport.StatusPass {
					t.Fatalf("named namespace result: status %s problem %v", result.Status, result.Problem)
				}
			})
		})
	}
	t.Logf("%d native anonymous name controls; %d native named runtime controls + %d native named deployment rejections; no anonymous-backed named expectations", len(rows), len(namedPaths), len(rows)-len(namedPaths))
}

// N039-N047 captured these expressions through anonymous Apex at both APIs.
// The named-scope cases are compiler regressions, not new native named results.
func TestSecurityClassLiteralCompilation(t *testing.T) {
	t.Parallel()
	cases := []struct{ id, body string }{
		{"N039", "Type t=AccessLevel.class; r=t!=null;"},
		{"N040", "Type t=AccessType.class; r=t!=null;"},
		{"N041", "Type t=A28Access__c.class; r=t!=null;"},
		{"N042", "Type t=System.AccessLevel.class; r=t!=null;"},
		{"N043", "Type t=System.AccessType.class; r=t!=null;"},
		{"N044", "Type t=Schema.A28Access__c.class; r=t!=null;"},
		{"N045", "r=AccessLevel.class.getName()!=null;"},
		{"N046", "r=AccessType.class.getName()!=null;"},
		{"N047", "r=A28Access__c.class.getName()!=null;"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.id, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "SecurityClassLiteralProbe.cls")
					writeFile(t, path, "public class SecurityClassLiteralProbe { public static Object run(){Object r; "+tc.body+" return r;} }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, gladeschema.Schema{
						Objects: []gladeschema.Object{{Name: "A28Access__c", Fields: []gladeschema.Field{{Name: "Name", Type: "Text"}}}},
					})
					if index.HasErrors() {
						t.Fatalf("parser: %#v", index.Diagnostics)
					}
					for name, result := range map[string]sema.Result{
						"anonymous": sema.AnalyzeAnonymous(index, "Object r; "+tc.body, api),
						"named":     sema.Analyze(index),
					} {
						t.Run(name, func(t *testing.T) {
							if result.HasErrors() {
								t.Fatalf("class literal rejected: %#v", result.Diagnostics)
							}
						})
					}
				})
			}
		})
	}
}

// These scope regressions preserve canonical behavior outside the captured
// R209-R211 permission metadata, N001-N003/N009-N011 inserts and generated
// R072/R213-R241 shares. They are not additional native observations.
func TestSecurityAccessSelfReviewBoundaries(t *testing.T) {
	t.Run("permission-metadata-without-name-based-assignments", func(t *testing.T) {
		var data struct{ Metadata map[string]string }
		raw, err := os.ReadFile("testdata/conformance/security_access.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		var source string
		for path, content := range data.Metadata {
			if strings.HasSuffix(path, "/A28OracleAccess.permissionset-meta.xml") {
				source = content
			}
		}
		if source == "" {
			t.Fatal("missing captured permission metadata")
		}
		for _, name := range []string{"A28OracleAccess", "UnassignedGuestMetadata"} {
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, name+".permissionset-meta.xml")
				writeFile(t, path, source)
				org := storage.NewOrgState()
				ApplyProjectPermissionSets(&org, project.Project{Root: root, PermissionSetFiles: []string{path}})
				if len(org.Objects["PermissionSet"].Records) != 1 || len(org.Objects["ObjectPermissions"].Records) != 1 || len(org.Objects["FieldPermissions"].Records) != 1 {
					t.Fatal("explicit permission metadata was not seeded")
				}
				if len(org.Objects["User"].Records) != 0 || len(org.Objects["PermissionSetAssignment"].Records) != 0 {
					t.Fatal("metadata name created a user or permission assignment")
				}
				for _, record := range org.Objects["FieldPermissions"].Records {
					if !record.Fields["PermissionsRead"].Boolean || !record.Fields["PermissionsEdit"].Boolean {
						t.Fatal("captured field permissions changed")
					}
				}
			})
		}
	})
	t.Run("name-defaults-require-declared-metadata", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Incomplete__c"] = storage.ObjectState{
			Definition: storage.ObjectDefinition{
				APIName: "Incomplete__c", KeyPrefix: "a0p",
				Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString, Required: true}},
			},
			Records: make(map[storage.ID]storage.Record),
		}
		engine := dml.NewEngine(&org)
		result := engine.Insert([]storage.Record{{Object: "Incomplete__c"}})
		if len(result) != 1 || result[0].Success || result[0].StatusCode != "REQUIRED_FIELD_MISSING" {
			t.Fatalf("incomplete metadata acquired omitted Name defaults from its spelling: %#v", result)
		}
		// Canonical normalization turns empty text into explicit null. Preserve
		// that existing contract rather than treating it as a new default.
		result = engine.Insert([]storage.Record{{Object: "Incomplete__c", Fields: map[string]storage.Value{"Name": storage.StringValue("")}}})
		if len(result) != 1 || !result[0].Success {
			t.Fatalf("canonical explicit-null normalization changed: %#v", result)
		}
		standard := sobject.BuildDescribeRegistry(gladeschema.Schema{Objects: []gladeschema.Object{{Name: "Account", NameField: gladeschema.NameField{Type: "Text"}}}})
		definition := sobject.ToObjectDefinition(standard.Objects["Account"])
		if definition.Metadata["nameFieldType"] != "" {
			t.Fatal("standard catalog object acquired custom Name defaults")
		}
		org.Objects["Account"] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
		result = engine.Insert([]storage.Record{{Object: "Account"}})
		if len(result) != 1 || result[0].Success || result[0].StatusCode != "REQUIRED_FIELD_MISSING" {
			t.Fatalf("N004 standard Name validation changed: %#v", result)
		}
	})
	t.Run("row-cause-members-on-ordinary-declared-types", func(t *testing.T) {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(api, func(t *testing.T) {
				root := t.TempDir()
				files := []string{filepath.Join(root, "A28Holder.cls"), filepath.Join(root, "A28Ordinary.cls")}
				writeFile(t, files[0], "public class A28Holder {public String Visible='visible';}")
				writeFile(t, files[1], "public class A28Ordinary {public static A28Holder RowCause=new A28Holder();public static Object run(){return A28Ordinary.RowCause.Visible;}}")
				for _, file := range files {
					writeFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{Objects: []gladeschema.Object{{Name: "A28Ordinary__c", NameField: gladeschema.NameField{Type: "Text"}}}})
				if index.HasErrors() {
					t.Fatalf("parser: %#v", index.Diagnostics)
				}
				for scope, result := range map[string]sema.Result{
					"anonymous": sema.AnalyzeAnonymous(index, "Object r=A28Ordinary.RowCause.Visible;", api),
					"named":     sema.Analyze(index),
				} {
					if result.HasErrors() {
						t.Fatalf("%s ordinary members acquired C016 platform diagnostics: %#v", scope, result.Diagnostics)
					}
				}
			})
		}
	})
	t.Run("name-insert-default-does-not-weaken-workflow-updates", func(t *testing.T) {
		registry := sobject.BuildDescribeRegistry(gladeschema.Schema{Objects: []gladeschema.Object{{
			Name: "A28Access__c", NameField: gladeschema.NameField{Type: "Text"},
			Fields: []gladeschema.Field{{Name: "Payload__c", Type: "Text"}},
		}}})
		org := storage.NewOrgState()
		org.Objects["A28Access__c"] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(registry.Objects["A28Access__c"]),
			Records:    make(map[storage.ID]storage.Record),
		}
		engine := dml.NewEngine(&org)
		inserted := engine.Insert([]storage.Record{{Object: "A28Access__c", Fields: map[string]storage.Value{"Payload__c": storage.StringValue("keep")}}})
		if len(inserted) != 1 || !inserted[0].Success {
			t.Fatalf("captured insert default: %#v", inserted)
		}
		id := inserted[0].ID
		before := org.Objects["A28Access__c"].Records[id].Fields["Name"].String
		if len(before) != 15 || before != string(id)[:15] {
			t.Fatalf("captured Name fallback = %q, Id %q", before, id)
		}
		state := org.Objects["A28Access__c"]
		state.Definition.WorkflowRules = []storage.WorkflowRule{{
			Name: "ClearName", Active: true,
			Criteria:     []storage.WorkflowCriteriaItem{{Field: "Payload__c", Operation: "equals", Value: "clear"}},
			FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Clear", Field: "Name", LiteralValue: ""}},
		}}
		org.Objects["A28Access__c"] = state
		updated := engine.Update([]storage.Record{{Object: "A28Access__c", ID: id, Fields: map[string]storage.Value{"Payload__c": storage.StringValue("clear")}}})
		if len(updated) != 1 || updated[0].Success || updated[0].StatusCode != "REQUIRED_FIELD_MISSING" {
			t.Fatalf("canonical workflow validation weakened: %#v", updated)
		}
		if got := org.Objects["A28Access__c"].Records[id].Fields["Name"].String; got != before {
			t.Fatalf("failed workflow persisted Name %q, want %q", got, before)
		}
	})
	t.Run("share-rules-require-generated-schema-provenance", func(t *testing.T) {
		registry := sobject.BuildDescribeRegistry(gladeschema.Schema{Objects: []gladeschema.Object{
			{Name: "A28Access__c", EnableSharing: true, SharingModel: "Private"},
			{Name: "Ordinary__Share", Fields: []gladeschema.Field{{Name: "RowCause", Type: "Text"}, {Name: "AccessLevel", Type: "Text"}}},
		}})
		org := storage.NewOrgState()
		for name, describe := range registry.Objects {
			org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: make(map[storage.ID]storage.Record)}
		}
		generated := org.Objects["A28Access__Share"].Definition
		if generated.Metadata["kind"] != "generatedShare" || generated.Metadata["parentObject"] != "A28Access__c" || generated.Fields["RowCause"].Updateable == nil || *generated.Fields["RowCause"].Updateable {
			t.Fatalf("generated share provenance/writeability: %#v", generated)
		}
		ordinary := org.Objects["Ordinary__Share"].Definition
		if ordinary.Metadata["kind"] == "generatedShare" || ordinary.Fields["RowCause"].Updateable != nil && !*ordinary.Fields["RowCause"].Updateable {
			t.Fatal("ordinary metadata acquired share semantics from its name")
		}
		engine := dml.NewEngine(&org)
		inserted := engine.Insert([]storage.Record{{Object: "Ordinary__Share", Fields: map[string]storage.Value{"RowCause": storage.StringValue("Owner"), "AccessLevel": storage.StringValue("All")}}})
		if len(inserted) != 1 || !inserted[0].Success {
			t.Fatalf("ordinary fields acquired share constraints: %#v", inserted)
		}
		updated := engine.Update([]storage.Record{{Object: "Ordinary__Share", ID: inserted[0].ID, Fields: map[string]storage.Value{"RowCause": storage.StringValue("changed")}}})
		if len(updated) != 1 || !updated[0].Success {
			t.Fatalf("ordinary RowCause became read-only: %#v", updated)
		}
		invalid := engine.Insert([]storage.Record{{Object: "A28Access__Share", Fields: map[string]storage.Value{"AccessLevel": storage.StringValue("Read")}}})
		if len(invalid) != 1 || invalid[0].Success || invalid[0].StatusCode != "FIELD_INTEGRITY_EXCEPTION" {
			t.Fatalf("captured generated-share constraints changed: %#v", invalid)
		}
	})
}

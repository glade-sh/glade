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
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Captured Salesforce rows are exact-text assertions at both API endpoints.
// Empty controls use Id=null; nonempty controls seed their own exported records.
func TestSOSLOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		ParserDiagnostic    bool
		ResultData          bool
		AttachedOrg         bool
	}
	var data struct {
		APIVersions                  []string `json:"apiVersions"`
		Cases, Controls              []familyCase
		ShadowControls               []familyCase
		Declarations                 map[string]string
		DeclarationAPIVersions       map[string]string `json:"declarationApiVersions"`
		ShadowDeclarations           map[string]string
		ShadowDeclarationAPIVersions map[string]string `json:"shadowDeclarationApiVersions"`
		ResultPrelude                string
		UnavailableObjects           []string
		Remaining                    []struct{ ID, Owner, Reason, Expected string }
		HostedLimitations            []string
	}
	raw, err := os.ReadFile("testdata/conformance/sosl.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases)+len(data.Remaining) != 277 || len(data.Controls) != 70 || len(data.ShadowControls) != 8 {
		t.Fatalf("oracle rows: %d asserted + %d carried; %d controls + %d shadow controls", len(data.Cases), len(data.Remaining), len(data.Controls), len(data.ShadowControls))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle APIs: %v", data.APIVersions)
	}
	seen := map[string]bool{}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A35" || row.Reason == "" || row.Expected == "" || row.Expected == "?" {
			t.Fatalf("invalid carry: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s; native expected <%s>", row.ID, row.Owner, row.Reason, row.Expected)
	}
	for _, limitation := range data.HostedLimitations {
		t.Logf("hosted limitation: %s", limitation)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	for _, row := range append(append([]familyCase{}, rows...), data.ShadowControls...) {
		if row.ID == "" || seen[row.ID] || row.Expected == "?" {
			t.Fatalf("invalid assertion: %#v", row)
		}
		seen[row.ID] = true
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
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	rowCode := func(row familyCase) string {
		if row.Compile {
			return row.Code
		}
		return "try {Object r; " + row.Code + " pq.out('" + row.ID + "',r);}catch(Exception e){pq.err('" + row.ID + "',e);}"
	}
	body := func(row familyCase) string {
		if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
			return "P pq=new P();\n\n\n\n\n" + rowCode(row)
		}
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(row.Expected) + ";\n"
		if row.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if row.ResultData {
			prefix += data.ResultPrelude + "\n"
		}
		return prefix + rowCode(row)
	}
	compileText := func(result sema.Result, compiledRow bool) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			problem := fmt.Sprintf("line %d: %s", line, item.Message)
			if compiledRow {
				// probe.py's separate compile capture replaces newlines and
				// retains the first 300 characters of the native problem.
				problem = strings.ReplaceAll(problem, "\n", " ")
				if chars := []rune(problem); len(chars) > 300 {
					problem = string(chars[:300])
				}
			}
			return "COMPILE_ERROR\t" + problem
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SOSL", api)
			root := t.TempDir()
			helperPath := filepath.Join(root, "P.cls")
			writeFile(t, helperPath, helper)
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			declarationNames := make([]string, 0, len(data.Declarations))
			for name := range data.Declarations {
				declarationNames = append(declarationNames, name)
			}
			sort.Strings(declarationNames)
			files := []string{helperPath}
			for _, name := range declarationNames {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, data.Declarations[name])
				version := data.DeclarationAPIVersions[name]
				if version == "" {
					version = api
				}
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
				files = append(files, path)
			}
			buildIndex := func(extra string) typesys.Index {
				selectedFiles := append([]string{}, files...)
				if extra != "" {
					selectedFiles = append(selectedFiles, extra)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: selectedFiles}, gladeschema.Schema{})
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
			// Rejection rows use the captured org's unavailable object set;
			// they do not establish hosted feature acceptance or effects.
			for _, name := range data.UnavailableObjects {
				delete(org.Objects, name)
			}
			// The CLI's anonymous route may have no attached org. Exercise its
			// standard-metadata fallback; @IsTest uses the exported local schema.
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			// Result and metadata rejection controls clone the linked base with
			// an attached org. Other empty controls exercise the nil-org route.
			resultRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class SOSLProbe {\n")
			executable := 0
			for _, row := range rows {
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SOSLProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			// Link once per API, then clone the shared runner for every @IsTest row.
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"SOSLProbe"}})
			if len(namedCases) != executable {
				t.Fatalf("named methods: %d expected %d", len(namedCases), executable)
			}
			methods, methodErrors := compileTestMethods(namedCases)
			invocations, invocationErrors := compileTestInvokePrograms(namedCases)
			namedRows := map[string]TestCase{}
			for _, row := range namedCases {
				key := testCaseKey(row)
				if methodErrors[key] != nil || invocationErrors[key] != nil {
					t.Fatalf("%s compilation: %v %v", key, methodErrors[key], invocationErrors[key])
				}
				id := strings.TrimPrefix(row.MethodName, "observed")
				if _, exists := namedRows[id]; exists {
					t.Fatalf("repeated named row %s", id)
				}
				namedRows[id] = row
			}
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						source := body(row)
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
							if row.ParserDiagnostic {
								// CLI declaration preparation can reject at this
								// entry point before semantic analysis runs.
								parsed := apexast.NewParser().ParseSource("__glade_anonymous.cls", "class SOSLParserProbe{} "+source)
								if observed := compileText(sema.Result{Diagnostics: parsed.Diagnostics}, row.Compile); observed != row.Expected {
									t.Fatalf("parser expected <%s> actual <%s>", row.Expected, observed)
								}
							}
							if observed := compileText(analysis, row.Compile); observed != row.Expected {
								t.Fatalf("expected <%s> actual <%s>", row.Expected, observed)
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
						rowRunner := runner
						if row.ResultData || row.AttachedOrg {
							rowRunner = resultRunner
						}
						if _, err := rowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
							name := "SOSLRejected" + row.ID
							source := "@IsTest private class " + name + " {\n@IsTest static void observed(){\n\n\nP pq=new P();\n" + rowCode(row) + "\n}\n}\n"
							path := filepath.Join(root, name+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							rowIndex := buildIndex(path)
							if row.ParserDiagnostic {
								if observed := compileText(sema.Result{Diagnostics: rowIndex.Diagnostics}, row.Compile); observed != row.Expected {
									t.Fatalf("parser expected <%s> actual <%s>", row.Expected, observed)
								}
							}
							analysis := sema.Analyze(rowIndex)
							if observed := compileText(analysis, row.Compile); observed != row.Expected {
								t.Fatalf("expected <%s> actual <%s>", row.Expected, observed)
							}
							return
						}
						tc, ok := namedRows[row.ID]
						if !ok {
							t.Fatalf("missing named row %s", row.ID)
						}
						key := testCaseKey(tc)
						machine := namedRunner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(invocations[key], tc.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}

			// S001-S008 have a source-owned Search namespace. Keep this index
			// separate so the platform constructor/signature controls above
			// continue to resolve their captured platform symbols.
			shadowFiles := []string{helperPath}
			shadowNames := make([]string, 0, len(data.ShadowDeclarations))
			for name := range data.ShadowDeclarations {
				shadowNames = append(shadowNames, name)
			}
			sort.Strings(shadowNames)
			for _, name := range shadowNames {
				path := filepath.Join(root, "shadow", name+".cls")
				writeFile(t, path, data.ShadowDeclarations[name])
				version := data.ShadowDeclarationAPIVersions[name]
				if version == "" {
					version = api
				}
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
				shadowFiles = append(shadowFiles, path)
			}
			shadowIndex := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: shadowFiles}, gladeschema.Schema{})
			if shadowIndex.HasErrors() {
				t.Fatalf("shadow parser: %v", shadowIndex.Diagnostics)
			}
			if analysis := sema.Analyze(shadowIndex); analysis.HasErrors() {
				t.Fatalf("shadow semantics: %v", analysis.Diagnostics)
			}
			shadowRunner := newConformanceRunner(t, shadowIndex, conformanceRunnerOptions{Base: runner, LinkProject: true})
			var shadowSource strings.Builder
			shadowSource.WriteString("@IsTest private class SOSLShadowProbe {\n")
			for _, row := range data.ShadowControls {
				fmt.Fprintf(&shadowSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row))
			}
			shadowPath := filepath.Join(root, "shadow", "SOSLShadowProbe.cls")
			writeFile(t, shadowPath, shadowSource.String()+"}\n")
			writeFile(t, shadowPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			shadowNamedIndex := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: append(shadowFiles, shadowPath)}, gladeschema.Schema{})
			if shadowNamedIndex.HasErrors() {
				t.Fatalf("shadow named parser: %v", shadowNamedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(shadowNamedIndex); analysis.HasErrors() {
				t.Fatalf("shadow named semantics: %v", analysis.Diagnostics)
			}
			shadowNamedRunner := newConformanceRunner(t, shadowNamedIndex, conformanceRunnerOptions{Base: shadowRunner, LinkProject: true})
			shadowCases := Discover(shadowNamedIndex, Options{SelectedClasses: []string{"SOSLShadowProbe"}})
			if len(shadowCases) != len(data.ShadowControls) {
				t.Fatalf("shadow named methods: %d expected %d", len(shadowCases), len(data.ShadowControls))
			}
			shadowMethods, shadowMethodErrors := compileTestMethods(shadowCases)
			shadowInvocations, shadowInvocationErrors := compileTestInvokePrograms(shadowCases)
			shadowRows := map[string]TestCase{}
			for _, tc := range shadowCases {
				key := testCaseKey(tc)
				if shadowMethodErrors[key] != nil || shadowInvocationErrors[key] != nil {
					t.Fatalf("%s shadow compilation: %v %v", key, shadowMethodErrors[key], shadowInvocationErrors[key])
				}
				shadowRows[strings.TrimPrefix(tc.MethodName, "observed")] = tc
			}
			for _, row := range data.ShadowControls {
				t.Run(row.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						source := body(row)
						if analysis := sema.AnalyzeAnonymous(shadowIndex, source, api); analysis.HasErrors() {
							t.Fatalf("source Search rejected: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := shadowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						tc, ok := shadowRows[row.ID]
						if !ok {
							t.Fatalf("missing shadow named row %s", row.ID)
						}
						key := testCaseKey(tc)
						machine := shadowNamedRunner.newMachine()
						if err := machine.RegisterMethod(shadowMethods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(shadowInvocations[key], tc.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}

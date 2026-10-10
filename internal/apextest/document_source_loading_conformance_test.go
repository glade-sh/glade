package apextest

import (
	"crypto/sha256"
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
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type documentSourceNamedCapture struct {
	Directory string
	SHA256    map[string]string
}

func readDocumentSourceCapture(t *testing.T, capture documentSourceNamedCapture, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/conformance", capture.Directory, name))
	if err != nil {
		t.Fatal(err)
	}
	if expected := capture.SHA256[name]; expected == "" || fmt.Sprintf("%x", sha256.Sum256(data)) != expected {
		t.Fatalf("native capture hash mismatch for %s", name)
	}
	return string(data)
}

// The exported observations retain each native API's exact metadata names and
// row bodies. Only captured routes run; anonymous answers do not stand in for
// native @IsTest observations.
func TestDocumentSourceLoadingOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Route, Code, Expected string
		Compile                   bool
	}
	var data struct {
		Captures []struct {
			APIVersion, Project    string
			Cases, PageParamsCases []familyCase
			Named, PageParamsNamed documentSourceNamedCapture
			PageParamsAnonymous    documentSourceNamedCapture
		}
	}
	raw, err := os.ReadFile("testdata/conformance/document_source_loading.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 2 || data.Captures[0].APIVersion != "62.0" || data.Captures[1].APIVersion != "67.0" {
		t.Fatal("expected independent API 62.0 and 67.0 captures")
	}
	compileObservation := func(result sema.Result) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			message := item.Message
			if item.NativeMessage != "" {
				message = item.NativeMessage
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			if item.NativeLine != nil {
				line = *item.NativeLine
			}
			return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
		}
		return "compiled"
	}
	for _, capture := range data.Captures {
		t.Run(capture.APIVersion, func(t *testing.T) {
			if len(capture.Cases) != 7 {
				t.Fatalf("captured anonymous rows = %d, want 7", len(capture.Cases))
			}
			if len(capture.PageParamsCases) != 3 {
				t.Fatal("expected independent page-parameter R011/R012/C001 observations")
			}
			p, err := project.Load(filepath.Join("testdata/conformance", capture.Project))
			if err != nil {
				t.Fatal(err)
			}
			if p.SourceAPIVersion != capture.APIVersion {
				t.Fatalf("source API = %q, capture API = %q", p.SourceAPIVersion, capture.APIVersion)
			}
			// The anonymous compiler row uses the probe's pq output helper. Its
			// body is irrelevant to compilation; retain the row text and line six.
			helperPath := filepath.Join(t.TempDir(), "P.cls")
			writeFile(t, helperPath, "public class P { public void out(String id,Object value) {} }")
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+capture.APIVersion+"</apiVersion></ApexClass>")
			p.ApexFiles = append(p.ApexFiles, helperPath)
			s, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			// Exercise the ordinary project-to-org path so a Document load
			// failure cannot be hidden by manually seeding the subsequent page.
			org := orgFromIndex(index)
			org.APIVersion = capture.APIVersion
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			registerVisualforcePages(runner.base, visualforcePageNames(index))
			runAnonymous := func(t *testing.T, cases []familyCase, required []string) {
				seen := make(map[string]bool)
				for _, row := range cases {
					if row.ID == "" || seen[row.ID] || row.Route != "anonymous" || row.Code == "" || row.Expected == "" || row.Expected == "?" {
						t.Fatalf("invalid captured row: %#v", row)
					}
					seen[row.ID] = true
					t.Run(row.Route+"/"+row.ID, func(t *testing.T) {
						if row.Compile {
							source := "\n\n\n\nP pq=new P();\n" + row.Code
							observed := compileObservation(sema.AnalyzeAnonymous(index, source, capture.APIVersion))
							if observed != row.Expected {
								t.Fatalf("native expected <%s>, compiler observed <%s>", row.Expected, observed)
							}
							return
						}
						if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
							t.Fatal("runtime row has a compiler observation")
						}
						source := "String observedText;\ntry {Object r;\n" + row.Code +
							"\nobservedText=String.valueOf(r);\n} catch(Exception e) {observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();}\n" +
							"String expectedText=" + conformanceApexString(row.Expected) + ";\n" +
							"System.assert(expectedText.equals(observedText),'expected <'+expectedText+'> actual <'+observedText+'>');"
						if result := sema.AnalyzeAnonymous(index, source, capture.APIVersion); result.HasErrors() {
							t.Fatal(result.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: capture.APIVersion})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
				}
				for _, id := range required {
					if !seen[id] {
						t.Fatalf("missing captured row %s", id)
					}
				}
			}
			runAnonymous(t, capture.Cases, []string{"R001", "R002", "R003", "R004", "R005", "R006", "C001"})
			t.Run("pageParams", func(t *testing.T) {
				if len(capture.PageParamsAnonymous.SHA256) != 3 {
					t.Fatal("expected three original anonymous capture files")
				}
				rows := strings.Split(strings.TrimSuffix(readDocumentSourceCapture(t, capture.PageParamsAnonymous, "org.tsv"), "\n"), "\n")
				if len(rows) != len(capture.PageParamsCases) {
					t.Fatal("anonymous native row count changed")
				}
				runtimeSource := readDocumentSourceCapture(t, capture.PageParamsAnonymous, "runtime.apex")
				compileSource := readDocumentSourceCapture(t, capture.PageParamsAnonymous, "compile/C001.apex")
				for i, row := range capture.PageParamsCases {
					source := runtimeSource
					if row.Compile {
						source = compileSource
					}
					if rows[i] != row.ID+"\t"+row.Expected || !strings.Contains(source, row.Code) {
						t.Fatalf("page-parameter row does not match original native source/answer: %s", row.ID)
					}
				}
				runAnonymous(t, capture.PageParamsCases, []string{"R011", "R012", "C001"})
			})
			t.Run("isTest", func(t *testing.T) {
				documentSourceNamedConformance(t, capture.APIVersion, capture.Project, capture.Named, []string{"R001", "R002", "R003", "R004", "R005", "R006", "R007", "R008", "R009", "R010", "S001", "S002"})
			})
			t.Run("pageParamsIsTest", func(t *testing.T) {
				documentSourceNamedConformance(t, capture.APIVersion, capture.Project, capture.PageParamsNamed, []string{"R011", "R012"})
			})
		})
	}
}

func documentSourceNamedConformance(t *testing.T, api, projectPath string, capture documentSourceNamedCapture, ids []string) {
	t.Helper()
	read := func(name string) string {
		t.Helper()
		return readDocumentSourceCapture(t, capture, name)
	}
	rows := func(name string, expected []string) map[string]string {
		t.Helper()
		lines := strings.Split(strings.TrimSuffix(read(name), "\n"), "\n")
		if len(lines) != len(expected) {
			t.Fatalf("%s has %d native rows, want %d", name, len(lines), len(expected))
		}
		observations := make(map[string]string)
		for i, line := range lines {
			id, value, ok := strings.Cut(line, "\t")
			if !ok || id != expected[i] || value == "" || value == "?" || value == "MISSING" {
				t.Fatalf("invalid native observation in %s: %q", name, line)
			}
			observations[id] = value
		}
		return observations
	}
	nativeRows := rows("org.tsv", ids)
	compiled := rows("compile.tsv", append(append([]string(nil), ids...), "C001"))
	var sources []struct {
		ID, Name, Method, Route, Source string
	}
	if err := json.Unmarshal([]byte(read("source-manifest.json")), &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != len(ids)+1 || len(capture.SHA256) != len(ids)+6 {
		t.Fatalf("named capture has %d sources and %d hashes, want %d and %d", len(sources), len(capture.SHA256), len(ids)+1, len(ids)+6)
	}
	var nativeResults []struct {
		FullName, MethodName, Outcome, Message string
	}
	if err := json.Unmarshal([]byte(read("native-test-results.json")), &nativeResults); err != nil {
		t.Fatal(err)
	}
	if len(nativeResults) != len(ids) {
		t.Fatalf("native test results = %d, want %d", len(nativeResults), len(ids))
	}
	nativeMessages := make(map[string]string)
	for _, result := range nativeResults {
		if result.Outcome != "Fail" || result.Message != "System.AssertException: Assertion Failed: P|"+result.MethodName+"|"+nativeRows[result.MethodName] || nativeMessages[result.FullName] != "" {
			t.Fatalf("invalid native terminal assertion: %+v", result)
		}
		nativeMessages[result.FullName] = result.Message
	}
	p, err := project.Load(filepath.Join("testdata/conformance", projectPath))
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceAPIVersion != api {
		t.Fatalf("named project API = %q, want %q", p.SourceAPIVersion, api)
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	classRoot := t.TempDir()
	want := make(map[string]string)
	classNames := make([]string, 0, len(ids))
	compilerPath := ""
	for i, source := range sources {
		id, route := "C001", "compile"
		if i < len(ids) {
			id, route = ids[i], "runtime"
		}
		if source.ID != id || source.Name == "" || source.Source != id+".cls" || source.Route != route || (route == "runtime" && source.Method != id) || (route == "compile" && source.Method != "") {
			t.Fatalf("invalid native source manifest row: %+v", source)
		}
		path := filepath.Join(classRoot, source.Name+".cls")
		writeFile(t, path, read(source.Source))
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
		if route == "compile" {
			compilerPath = path
			continue
		}
		fullName := source.Name + "." + source.Method
		if compiled[id] != "compiled" || nativeMessages[fullName] == "" || want[fullName] != "" {
			t.Fatalf("missing native compiler or test observation for %s", fullName)
		}
		want[fullName] = nativeMessages[fullName]
		classNames = append(classNames, source.Name)
		p.ApexFiles = append(p.ApexFiles, path)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	// Run the original captured test methods with the ordinary runner. Its
	// isolation and SeeAllData handling determine Document visibility.
	// Project and class metadata select the source API; the independent REST
	// runtime context retains its default.
	run := Run(index, Options{SelectedClasses: classNames, Parallelism: 1, NoDiskCache: true})
	seen := make(map[string]bool)
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			fullName := result.ClassName + "." + result.MethodName
			message, exists := want[fullName]
			if !exists || seen[fullName] {
				t.Fatalf("unexpected or duplicate named result: %+v", result)
			}
			seen[fullName] = true
			t.Run(result.MethodName, func(t *testing.T) {
				if result.Problem == nil {
					t.Fatalf("native terminal assertion did not execute: %+v", result)
				}
				if result.Status != testreport.StatusFail || result.Reason != testreport.ReasonAssertion {
					t.Fatalf("native terminal assertion did not execute: status=%s reason=%s problem=%+v", result.Status, result.Reason, *result.Problem)
				}
				if got := result.Problem.Type + ": " + result.Problem.Message; got != message {
					t.Fatalf("named observation <%s>, native <%s>", got, message)
				}
			})
		}
	}
	if len(seen) != len(ids) {
		t.Fatalf("executed %d named rows, want %d", len(seen), len(ids))
	}
	t.Run("C001", func(t *testing.T) {
		var nativeErrors []struct {
			ErrorCode, Message string
			Fields             []string
		}
		if err := json.Unmarshal([]byte(read("C001.compiler.json")), &nativeErrors); err != nil {
			t.Fatal(err)
		}
		if len(nativeErrors) != 1 || nativeErrors[0].ErrorCode != "UNKNOWN_EXCEPTION" || len(nativeErrors[0].Fields) != 0 || compiled["C001"] != "COMPILE_ERROR\t"+nativeErrors[0].Message {
			t.Fatalf("invalid independent named compiler observation: %+v", nativeErrors)
		}
		compilerProject := p
		compilerProject.ApexFiles = append(append([]string(nil), p.ApexFiles...), compilerPath)
		compilerIndex := typesys.Build(compilerProject, s)
		if compilerIndex.HasErrors() {
			t.Fatal(compilerIndex.Diagnostics)
		}
		observed := "compiled"
		var messages []string
		for _, item := range sema.Analyze(compilerIndex).Diagnostics {
			if item.Severity == diagnostic.Error {
				message := item.Message
				if item.NativeMessage != "" {
					message = item.NativeMessage
				}
				messages = append(messages, message)
			}
		}
		if len(messages) != 0 {
			observed = "COMPILE_ERROR\t" + strings.Join(messages, "\n")
		}
		if observed != compiled["C001"] {
			t.Fatalf("named compiler observed <%s>, native <%s>", observed, compiled["C001"])
		}
	})
}

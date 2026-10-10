package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCorpusRTDatetimeConformance(t *testing.T) {
	type familyCase struct {
		ID                 string                       `json:"id"`
		Code               string                       `json:"code"`
		Compile            bool                         `json:"compile"`
		ExpectedByRouteAPI map[string]map[string]string `json:"expectedByRouteAPI"`
	}
	var data struct {
		APIVersions             []string     `json:"apiVersions"`
		IntermediateAPIVersions []string     `json:"intermediateAPIVersions"`
		Cases                   []familyCase `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/conformance/corpus_rt_datetime.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.IntermediateAPIVersions, ",") != "63.0,64.0,65.0,66.0" || len(data.Cases) != 47 {
		t.Fatalf("oracle matrix: APIs %v, rows %d", data.APIVersions, len(data.Cases))
	}
	boundaryRows := make(map[string]bool)
	for _, id := range strings.Fields("D021 D022 D023 D024 D044 D045 D046 D047") {
		boundaryRows[id] = true
	}
	seen := make(map[string]bool, len(data.Cases))
	observations := 0
	for _, row := range data.Cases {
		if row.ID == "" || seen[row.ID] || row.Code == "" || row.Compile {
			t.Fatalf("invalid or repeated runtime row: %#v", row)
		}
		seen[row.ID] = true
		if len(row.ExpectedByRouteAPI) != 2 {
			t.Fatalf("%s: expected both native routes", row.ID)
		}
		for _, route := range []string{"anonymous", "isTest"} {
			byAPI := row.ExpectedByRouteAPI[route]
			versions := append([]string{}, data.APIVersions...)
			if route == "isTest" && boundaryRows[row.ID] {
				versions = append(versions, data.IntermediateAPIVersions...)
			}
			if len(byAPI) != len(versions) {
				t.Fatalf("%s/%s: expected %d independently captured API observations", row.ID, route, len(versions))
			}
			for _, api := range versions {
				expected, exists := byAPI[api]
				if !exists || expected == "" || expected == "?" || expected == "MISSING" || strings.HasPrefix(expected, "COMPILE_ERROR") {
					t.Fatalf("%s/%s/%s: missing native runtime observation", row.ID, route, api)
				}
				observations++
			}
		}
	}
	for n := 1; n <= 47; n++ {
		if id := fmt.Sprintf("D%03d", n); !seen[id] {
			t.Fatalf("missing native row: %s", id)
		}
	}
	if observations != 220 {
		t.Fatalf("native observations: %d, expected 220", observations)
	}
	body := func(row familyCase, route, api string) string {
		return "Object r;\ntry {\n" + row.Code + "\n" +
			"} catch (Exception e) { r='EXC|'+e.getTypeName()+'|'+e.getMessage(); }\n" +
			"String observedText=String.valueOf(r);\n" +
			"String expectedText=" + conformanceApexString(row.ExpectedByRouteAPI[route][api]) + ";\n" +
			"System.assert(expectedText.equals(observedText),'" + row.ID + " expected <'+expectedText+'> actual <'+observedText+'>');\n"
	}
	versions := append(append([]string{}, data.APIVersions...), data.IntermediateAPIVersions...)
	for _, api := range versions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "date and time", api, "corpus-datetime")
			endpoint := api == "62.0" || api == "67.0"
			rows := data.Cases
			if !endpoint {
				rows = nil
				for _, row := range data.Cases {
					if boundaryRows[row.ID] {
						rows = append(rows, row)
					}
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			// Standard Event definitions supply the fields used by every row.
			metadata := gladeschema.Schema{Objects: []gladeschema.Object{
				{Name: "Event", Fields: []gladeschema.Field{{Name: "Id", Type: "Id"}}},
			}}
			buildIndex := func(classPath string) typesys.Index {
				var files []string
				if classPath != "" {
					files = append(files, classPath)
				}
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, metadata)
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("anonymous index: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("anonymous index semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})

			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class CorpusRTDatetimeProbe {\n")
			for _, row := range rows {
				fmt.Fprintf(&namedSource, "@IsTest static void %s(){\n%s}\n", row.ID, body(row, "isTest", api))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "CorpusRTDatetimeProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"CorpusRTDatetimeProbe"}})
			if len(namedCases) != len(rows) {
				t.Fatalf("named discovery: %d, expected %d", len(namedCases), len(rows))
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			namedRows := make(map[string]TestCase, len(namedCases))
			for _, row := range namedCases {
				key := testCaseKey(row)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation %s: %v %v", row.MethodName, methodErrors[key], programErrors[key])
				}
				if _, exists := namedRows[row.MethodName]; exists || !seen[row.MethodName] {
					t.Fatalf("unexpected or repeated named row: %s", row.MethodName)
				}
				namedRows[row.MethodName] = row
			}
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					if endpoint {
						counts.Run(t, "anonymous", row.ID, "anonymous", func(t *testing.T) {
							source := body(row, "anonymous", api)
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := anonymousRunner.execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
					counts.Run(t, "isTest", row.ID, "isTest", func(t *testing.T) {
						namedRow, exists := namedRows[row.ID]
						if !exists {
							t.Fatalf("missing named row: %s", row.ID)
						}
						key := testCaseKey(namedRow)
						// The shared runner clones the seed org for each execution.
						machine := namedRunner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], namedRow.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}

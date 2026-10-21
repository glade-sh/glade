package apextest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCollectionExpressionsOrgConformance(t *testing.T) {
	type cell struct {
		API, Context, Status string
		Native               map[string]any
	}
	var data struct {
		APIs  []string
		Cases []struct {
			ID, Methods, Observation string
			Cells                    []cell
		}
		Carries []struct {
			ID, Reason string
			Cells      []cell
		}
	}
	raw, err := os.ReadFile("testdata/conformance/collection_expressions/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,67.0,68.0" || len(data.Cases) != 16 || len(data.Carries) != 1 {
		t.Fatalf("native fixture shape: APIs=%v cases=%d carries=%d", data.APIs, len(data.Cases), len(data.Carries))
	}
	seen := map[string]bool{}
	validateCell := func(id string, item cell) string {
		name := id + "/" + item.API + "/" + item.Context
		if seen[name] || !strings.Contains(",62.0,67.0,68.0,", ","+item.API+",") || (item.Context != "anonymous" && item.Context != "istest") {
			t.Fatalf("unexpected/repeated native cell %s", name)
		}
		seen[name] = true
		return name
	}
	accepted, rejected, carried := 0, 0, 0
	for _, tc := range data.Cases {
		if tc.ID == "" || len(tc.Cells) != 6 {
			t.Fatalf("invalid semantic case %s: cells=%d", tc.ID, len(tc.Cells))
		}
		for _, item := range tc.Cells {
			name := validateCell(tc.ID, item)
			if item.Status == "OBSERVED" {
				accepted++
			} else if item.Status == "COMPILE-REJECTED" {
				rejected++
			} else {
				t.Fatalf("unexpected native status %s: %s", name, item.Status)
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				helperPath := filepath.Join(root, "ExpressionProbe.cls")
				helper := "public class ExpressionProbe { List<Object> values=new List<Object>(); " + tc.Methods + " public Map<String,Object> observe(){Map<String,Object> observed=new Map<String,Object>();" + tc.Observation + "\nobserved.put('size',values.size());return observed;}}"
				writeFile(t, helperPath, helper)
				writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+item.API+"</apiVersion></ApexClass>")
				files := []string{helperPath}
				caller := "ExpressionProbe probe=new ExpressionProbe(); Map<String,Object> observed=probe.observe();\n"
				if item.Status == "OBSERVED" {
					keys := make([]string, 0, len(item.Native))
					for key := range item.Native {
						keys = append(keys, key)
					}
					sort.Strings(keys)
					caller += fmt.Sprintf("System.assertEquals(%d,observed.size(),'observed field count');\n", len(keys))
					for _, key := range keys {
						expected := collectionExpressionLiteral(t, item.Native[key])
						caller += "System.assert(observed.containsKey(" + conformanceApexString(key) + "));\n"
						caller += "System.assertEquals(" + expected + ",observed.get(" + conformanceApexString(key) + ")," + conformanceApexString(key) + ");\n"
					}
				}
				if item.Context == "istest" {
					path := filepath.Join(root, "ExpressionTest.cls")
					writeFile(t, path, "@IsTest private class ExpressionTest {@IsTest static void observed(){"+caller+"}}")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+item.API+"</apiVersion></ApexClass>")
					files = append(files, path)
				}
				index := typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: item.API}, schema.Schema{})
				analysis := sema.Analyze(index)
				compileRejected := index.HasErrors() || analysis.HasErrors()
				if item.Status == "COMPILE-REJECTED" {
					if !compileRejected {
						t.Fatal("native compilation rejected this expression, but local compilation accepted it")
					}
					return
				}
				if compileRejected {
					t.Fatalf("native compilation accepted this expression: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
				}
				if item.Context == "istest" {
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 || summary.CompileErrors != 0 {
						t.Fatalf("named execution: %#v: %s", summary, firstRunProblem(run))
					}
					return
				}
				if analysis := sema.AnalyzeAnonymous(index, caller, item.API); analysis.HasErrors() {
					t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(caller, vm.CompileOptions{APIVersion: item.API})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = item.API
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	for _, carry := range data.Carries {
		if carry.ID != "P01" || strings.TrimSpace(carry.Reason) == "" || len(carry.Cells) != 6 {
			t.Fatalf("invalid configuration carry: %#v", carry)
		}
		for _, item := range carry.Cells {
			name := validateCell(carry.ID, item)
			carried++
			t.Run(name, func(t *testing.T) {
				if _, ok := item.Native["values"].([]any); !ok {
					t.Fatal("configuration carry lost its native picklist evidence")
				}
				t.Logf("Configuration carry: %s", carry.Reason)
			})
		}
	}
	if accepted != 54 || rejected != 42 || carried != 6 || len(seen) != 102 {
		t.Fatalf("native cell coverage accepted=%d rejected=%d configuration-carry=%d unique=%d", accepted, rejected, carried, len(seen))
	}
}

func collectionExpressionLiteral(t *testing.T, value any) string {
	t.Helper()
	switch value := value.(type) {
	case bool:
		return fmt.Sprint(value)
	case float64:
		if value != math.Trunc(value) {
			t.Fatalf("native observation is not an integer: %v", value)
		}
		return fmt.Sprintf("%.0f", value)
	case string:
		return conformanceApexString(value)
	default:
		t.Fatalf("unexpected native observation type %T", value)
		return ""
	}
}

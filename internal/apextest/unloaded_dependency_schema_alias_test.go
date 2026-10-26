package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestUnloadedDependencySchemaAliasKnownTypeUnitRoute(t *testing.T) {
	index := typesys.Index{
		Objects: []schema.Object{{Name: "CedarAlias__c"}, {Name: "cedar__Record__c"}},
		Types:   []typesys.TypeSymbol{{Name: "CedarHelper"}},
	}
	for _, cell := range []struct {
		name  string
		known bool
	}{
		{"Schema.Account", true}, {"sChEmA.aCcOuNt", true}, {"Account", true},
		{"Schema.CedarAlias__c", true}, {"sChEmA.cEdArAlIaS__c", true}, {"CedarAlias__c", true},
		{"Schema.cedar__Record__c", true}, {"System.String", true},
		{"Schema.CedarAbsent__c", false}, {"Vendor.Account", false},
		{"Schema.Schema.Account", false}, {"Schema.CedarHelper", false}, {"CedarHelper", true},
	} {
		t.Run(cell.name+"/known_type_unit", func(t *testing.T) {
			if got := dependencyTypeKnown(index, cell.name); got != cell.known {
				t.Fatalf("known type %q=%v, want %v", cell.name, got, cell.known)
			}
		})
	}
}

func TestUnloadedDependencySchemaAliasExplanationAnonymousUnitRoute(t *testing.T) {
	index := typesys.Index{
		Objects:      []schema.Object{{Name: "CedarAlias__c"}},
		Dependencies: []typesys.DependencyInfo{{Namespace: "cedar-optional", Diagnostics: []diagnostic.Diagnostic{{Code: "dependency_missing"}}}},
	}
	for _, name := range []string{"Schema.Account", "Schema.CedarAlias__c"} {
		for _, route := range []string{"anonymous_compile_error", "anonymous_runtime_error"} {
			t.Run(name+"/"+route, func(t *testing.T) {
				var original error = fmt.Errorf(`method "observe" calls unknown method "%s.absentMember"`, name)
				if route == "anonymous_runtime_error" {
					original = &vm.RuntimeError{Type: "UnsupportedFeature", Message: fmt.Sprintf(`unsupported call "%s.absentMember"`, name), Stack: []vm.StackFrame{{Symbol: "observe", Line: 1, Column: 1}}}
				}
				if got := ExplainDependencyError(index, name+".absentMember();", original); got != original {
					t.Fatalf("known object member error changed: original=%v got=%v", original, got)
				}
			})
		}
	}
}

func TestUnloadedDependencySchemaAliasNamedTestRunnerLSPNativeRoutes(t *testing.T) {
	dir := filepath.Join("testdata/conformance/unloaded_dependency/schema_alias")
	var cases []struct{ ID, Source, Outcome string }
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	object, err := os.ReadFile(filepath.Join(dir, "sources/CedarAlias__c.object-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		f, err := os.Open(filepath.Join(dir, "api"+api[:2], "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil || len(rows) != 1+2*len(cases) {
			t.Fatalf("native matrix: rows=%d error=%v", len(rows), err)
		}
		native := map[string][]string{}
		for _, row := range rows[1:] {
			if len(row) != 5 || row[1] != api || row[4] == "" {
				t.Fatalf("invalid native row: %v", row)
			}
			native[row[0]+"/"+row[2]] = row
		}
		for _, cell := range cases {
			for _, route := range []string{"named_test_runner_lsp", "trigger_helper_test_runner_lsp"} {
				t.Run(api+"/"+cell.ID+"/"+route, func(t *testing.T) {
					for _, invocation := range []string{"anonymous", "istest"} {
						if row := native[cell.ID+"/"+invocation]; len(row) != 5 || row[3] != cell.Outcome {
							t.Fatalf("native %s: %v", invocation, row)
						}
					}
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"cedar-optional"}]}]}`)
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/CedarAlias__c/CedarAlias__c.object-meta.xml"), string(object))
					writeClass := func(name, source string) {
						path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					source := cell.Source
					if strings.HasPrefix(route, "trigger_helper") {
						writeClass("CedarSchemaHelper", "public class CedarSchemaHelper { public static void apply() { "+source+" } }")
						path := filepath.Join(root, "force-app/main/default/triggers/CedarSchemaGate.trigger")
						writeFile(t, path, "trigger CedarSchemaGate on Account (before insert) { CedarSchemaHelper.apply(); }")
						writeFile(t, path+"-meta.xml", "<ApexTrigger><apiVersion>"+api+"</apiVersion></ApexTrigger>")
						source = "insert new Account(Name='cedar');"
					}
					writeClass("CedarSchemaObserve", "@IsTest private class CedarSchemaObserve { @IsTest static void observe() { "+source+" } }")
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("missing observer: %+v", run.Summary())
					}
					observed := run.Suites[0].Cases[0]
					if cell.Outcome == "accepted" {
						if observed.Status != testreport.StatusPass || !run.Summary().OK() {
							t.Fatalf("native assertions passed: %+v problem=%+v", observed, observed.Problem)
						}
					} else if observed.Status == testreport.StatusPass || observed.Problem == nil || observed.Problem.Type == "dependency_missing" || !strings.Contains(observed.Problem.Message, "absentMember") || strings.Contains(observed.Problem.Message, "cedar-optional") {
						t.Fatalf("known Schema object relabeled: %+v problem=%+v", observed, observed.Problem)
					}
					for _, diag := range lsp.TestDiagnostics(run) {
						if strings.Contains(diag.Message, "dependency_missing") {
							t.Errorf("known Schema object relabeled in LSP: %+v", diag)
						}
					}
				})
			}
		}
	}
}

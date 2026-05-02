package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestAnalyzeResolvesMemberTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "List<Thing__c>"},
				},
			},
		},
		Objects: []schema.Object{{Name: "Thing__c"}},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeUnknownMemberType(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}}},
				},
			},
		},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA002" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func TestAnalyzeMethodParameterTypes(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Hello",
				File: "Hello.cls",
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationMethod,
						Name: "run",
						Parameters: []apexast.Parameter{
							{Name: "accounts", Type: "List<Account>"},
							{Name: "missing", Type: "MissingType", Range: diagnostic.Range{Start: diagnostic.Position{Line: 2, Column: 20}}},
						},
					},
				},
			},
		},
		Objects: []schema.Object{{Name: "Account"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "OAERSEMA004" {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

func TestAnalyzeProjectNamespaceQualifiedTypes(t *testing.T) {
	index := typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "Helper",
				File: "Helper.cls",
			},
			{
				Kind: apexast.DeclarationClass,
				Name: "UsesHelper",
				File: "UsesHelper.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "run", Type: "pkg.Helper"},
					{
						Kind: apexast.DeclarationMethod,
						Name: "withParam",
						Parameters: []apexast.Parameter{
							{Name: "helper", Type: "pkg.Helper"},
						},
					},
				},
			},
		},
	}

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeVisibilityBaseline(t *testing.T) {
	index := typesys.Index{
		Types: []typesys.TypeSymbol{
			{
				Kind:      apexast.DeclarationClass,
				Name:      "Both",
				File:      "Both.cls",
				Modifiers: []string{"public", "global"},
			},
			{
				Kind: apexast.DeclarationInterface,
				Name: "IWorker",
				File: "IWorker.cls",
				Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "hidden", Modifiers: []string{"private"}},
				},
			},
		},
	}

	result := Analyze(index)
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diag := range result.Diagnostics {
		if diag.Code != "OAERSEMA005" {
			t.Fatalf("diagnostic = %#v", diag)
		}
	}
}

func TestAnalyzeMethodBodyBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void work() {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  Integer field;
  public void run(String input) {
    Integer count = 1;
    Helper h = new Helper();
    h.work();
    field = count;
  }
  public void callRun() {
    run('x');
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
}

func TestAnalyzeMethodBodyDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    MissingType item;
    MissingCtor built = new MissingCtor();
    missingValue = 1;
    missingCall();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "Hello.cls")}}, schema.Schema{})

	result := Analyze(index)
	codes := map[string]bool{}
	for _, diag := range result.Diagnostics {
		codes[diag.Code] = true
	}
	for _, code := range []string{"OAERSEMA006", "OAERSEMA007", "OAERSEMA008"} {
		if !codes[code] {
			t.Fatalf("missing %s in diagnostics: %#v", code, result.Diagnostics)
		}
	}
}

func TestAnalyzeMethodCallOverloadBaseline(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "Helper.cls"), `
public class Helper {
  public void pick(Integer value) {}
  public void pick(String value) {}
}
`)
	writeSemaFile(t, filepath.Join(root, "Hello.cls"), `
public class Hello {
  public void run() {
    Helper h = new Helper();
    h.pick(1);
    h.pick('one');
    h.pick(true);
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{
		filepath.Join(root, "Helper.cls"),
		filepath.Join(root, "Hello.cls"),
	}}, schema.Schema{})

	result := Analyze(index)
	var got bool
	for _, diag := range result.Diagnostics {
		if diag.Code == "OAERSEMA009" {
			got = true
		}
	}
	if !got {
		t.Fatalf("expected OAERSEMA009: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerObject(t *testing.T) {
	index := typesys.Index{
		Triggers: []typesys.TriggerSymbol{{Name: "ThingTrigger", ObjectName: "Missing__c", File: "Thing.trigger"}},
	}

	result := Analyze(index)
	if !result.HasErrors() {
		t.Fatalf("expected diagnostic: %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Code != "OAERSEMA001" {
		t.Fatalf("diagnostic = %#v", result.Diagnostics[0])
	}
}

func writeSemaFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTypeNames(t *testing.T) {
	got := extractTypeNames("Map<String,List<Account>>")
	want := []string{"String", "Account"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v", got)
		}
	}
}

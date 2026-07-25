package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeTriggerProject(t *testing.T, files map[string]string, objects ...schema.Object) Result {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, contents := range files {
		path := filepath.Join(root, name)
		writeSemaFile(t, path, contents)
		paths = append(paths, path)
	}
	index, artifacts := typesys.BuildWithArtifacts(
		project.Project{Root: root, ApexFiles: paths},
		schema.Schema{Objects: objects},
	)
	return AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:                    true,
		SuppressPerformanceDiagnostics: true,
		BuildArtifacts:                 &artifacts,
	})
}

func triggerDiagnosticMatching(result Result, substring string) bool {
	for _, diag := range result.Diagnostics {
		if strings.Contains(strings.ToLower(diag.Message), strings.ToLower(substring)) {
			return true
		}
	}
	return false
}

func TestAnalyzeTriggerBodyRejectsIncompatibleAssignment(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert) {
  Integer value = 'wrong';
}
`,
	}, schema.Object{Name: "Account"})

	if !result.HasErrors() {
		t.Fatalf("expected trigger body assignment error, got %#v", result.Diagnostics)
	}
	if !triggerDiagnosticMatching(result, `trigger "AccountTrigger" initializes Integer local "value" with String`) {
		t.Fatalf("expected diagnostic naming the trigger and local, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerBodyRejectsUnknownCallAndLocalType(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountHandler.cls": `
public class AccountHandler {
  public static void beforeInsert(List<Account> records) {}
}
`,
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert) {
  AccountHandler.missingMethod(Trigger.new);
}
`,
	}, schema.Object{Name: "Account"})
	if !triggerDiagnosticMatching(result, "AccountHandler.missingMethod") {
		t.Fatalf("expected unknown call diagnostic, got %#v", result.Diagnostics)
	}

	localResult := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert) {
  MissingType__c record = null;
}
`,
	}, schema.Object{Name: "Account"})
	if !triggerDiagnosticMatching(localResult, "MissingType__c") {
		t.Fatalf("expected unknown local type diagnostic, got %#v", localResult.Diagnostics)
	}
}

func TestAnalyzeTriggerEventRejectsDuplicateEvents(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert, before insert) {
}
`,
	}, schema.Object{Name: "Account"})

	if !triggerDiagnosticMatching(result, "duplicate") {
		t.Fatalf("expected duplicate trigger event diagnostic, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerEventAcceptsDistinctEvents(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert, after insert, before update, after update) {
}
`,
	}, schema.Object{Name: "Account"})

	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for distinct trigger events: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerObjectRejectsNonTriggerableObject(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"ApexClassTrigger.trigger": `
trigger ApexClassTrigger on ApexClass (before insert) {
}
`,
	})

	if !triggerDiagnosticMatching(result, "does not support triggers") {
		t.Fatalf("expected non-triggerable object diagnostic, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerObjectAcceptsContentVersionDelete(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"ContentVersionTrigger.trigger": `
trigger ContentVersionTrigger on ContentVersion (before delete, after delete) {
}
`,
	})

	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for ContentVersion delete trigger: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerObjectAcceptsCustomObjectWithoutDescribeEvidence(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"ThingTrigger.trigger": `
trigger ThingTrigger on Thing__c (before insert) {
}
`,
	}, schema.Object{Name: "Thing__c"})

	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for custom object trigger: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerBodyAcceptsTriggerContextAndHandlerCalls(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountHandler.cls": `
public class AccountHandler {
  public static void beforeInsert(List<Account> records) {}
  public static void afterUpdate(Map<Id, Account> previous) {}
}
`,
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert, after update) {
  if (Trigger.isBefore && Trigger.isInsert) {
    AccountHandler.beforeInsert(Trigger.new);
  }
  if (Trigger.isAfter && Trigger.isUpdate) {
    AccountHandler.afterUpdate(Trigger.oldMap);
    for (Account record : Trigger.new) {
      Account previous = Trigger.oldMap.get(record.Id);
      if (previous.Name != record.Name) {
        System.debug(record.Name);
      }
    }
  }
}
`,
	}, schema.Object{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}})

	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for valid trigger body: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerBodyTypesContextForDeclaredObject(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert) {
  Integer bad = Trigger.new;
}
`,
	}, schema.Object{Name: "Account"})

	if !triggerDiagnosticMatching(result, "List<Account>") {
		t.Fatalf("expected Trigger.new to type as List<Account>, got %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerBodyAcceptsStaticTriggerLocal(t *testing.T) {
	t.Parallel()
	result := analyzeTriggerProject(t, map[string]string{
		"AccountTrigger.trigger": `
trigger AccountTrigger on Account (before insert) {
  static Integer processed = 0;
  processed = processed + Trigger.new.size();
}
`,
	}, schema.Object{Name: "Account"})

	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics for static trigger local: %#v", result.Diagnostics)
	}
}

func TestAnalyzeTriggerBodyIgnoresDependencyTriggers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	depRoot := filepath.Join(root, "dep")
	consumerRoot := filepath.Join(root, "consumer")
	for _, dir := range []string{depRoot, consumerRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	depTrigger := filepath.Join(depRoot, "DepTrigger.trigger")
	writeSemaFile(t, depTrigger, `
trigger DepTrigger on Account (before insert) {
  Integer value = 'wrong';
}
`)
	writeSemaFile(t, filepath.Join(consumerRoot, "Consumer.cls"), `
public class Consumer {
  public void run() {}
}
`)
	depProject := project.Project{
		Root:      depRoot,
		Namespace: "pkgx",
		ApexFiles: []string{depTrigger},
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root: consumerRoot,
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "pkgx",
			SourceRoot: depRoot,
			Project:    &depProject,
			Status:     "loaded",
		}},
		ApexFiles: []string{filepath.Join(consumerRoot, "Consumer.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	result := AnalyzeWithOptions(index, AnalyzeOptions{
		Diagnostics:                    true,
		SuppressPerformanceDiagnostics: true,
		BuildArtifacts:                 &artifacts,
	})

	for _, diag := range result.Diagnostics {
		if strings.Contains(diag.File, depRoot) {
			t.Fatalf("dependency trigger diagnostic leaked: %#v", result.Diagnostics)
		}
	}
}

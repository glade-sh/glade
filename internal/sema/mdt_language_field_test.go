package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rows (scratch org, API 62.0, 67.0, 68.0): a custom metadata type
// exposes Language beside DeveloperName, MasterLabel, NamespacePrefix,
// QualifiedApiName and Label. SELECT, WHERE and ORDER BY on Language compiled
// and ran (value en_US). GROUP BY Language failed to compile (not groupable).
func TestQueryAcceptsCustomMetadataLanguageField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "RuleReader.cls")
	writeSemaFile(t, classPath, `
public class RuleReader {
  public static List<Rule__mdt> all() {
    return [SELECT DeveloperName, Language, MasterLabel, NamespacePrefix, QualifiedApiName, Label FROM Rule__mdt WHERE Language = 'en_US' ORDER BY Language];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Rule__mdt"}},
	})
	result := Analyze(index)
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_QUERY_FIELD" {
			t.Fatalf("Language rejected on a custom metadata type: %#v", diag)
		}
	}
}

func TestQueryRejectsGroupingByCustomMetadataLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	classPath := filepath.Join(root, "RuleCounter.cls")
	writeSemaFile(t, classPath, `
public class RuleCounter {
  public static List<AggregateResult> counts() {
    return [SELECT Language lang, COUNT(Id) c FROM Rule__mdt GROUP BY Language];
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, schema.Schema{
		Objects: []schema.Object{{Name: "Rule__mdt"}},
	})
	if !hasDiagnosticCode(Analyze(index).Diagnostics, "GLADESEMA_QUERY_CAPABILITY") {
		t.Fatal("GROUP BY Language compiled; native rejected it as not groupable")
	}
}

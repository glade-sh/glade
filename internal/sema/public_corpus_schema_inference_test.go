package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPublicCorpusCustomSettingNameRemainsStringWhenInitializedFromDatetimeFormat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "CustomSettingName.cls"), `
public class CustomSettingName {
  public String currentONSName { get; set; }
  public void run() {
    Opportunity_Naming_Settings__c settings = new Opportunity_Naming_Settings__c(
      Name = System.now().format()
    );
    currentONSName = settings.Name;
  }
}
`)
	index := typesys.Build(project.Project{
		Root:      root,
		ApexFiles: []string{filepath.Join(root, "CustomSettingName.cls")},
	}, schema.Schema{Objects: []schema.Object{{Name: "Opportunity_Naming_Settings__c"}}})
	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "currentONSName")
}

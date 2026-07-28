package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeMergesPartialConsumerObjectWithCompleteDependencyObject(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "DependencySchemaConsumer.cls")
	writeSemaFile(t, classPath, `
public class DependencySchemaConsumer {
  public void run(dep__Event__c record, Decimal amount) {
    record.dep__UnitPrice__c = amount;
    Boolean open = record.dep__StartsAt__c > Datetime.now();
    record.local__DisplayLabel__c = 'label';
  }
}
`)

	index := typesys.Build(project.Project{
		Root:      root,
		Namespace: "local",
		ApexFiles: []string{classPath},
	}, schema.Schema{Objects: []schema.Object{
		{
			Name:    "dep__Event__c",
			Partial: true,
			Fields: []schema.Field{
				{Name: "local__DisplayLabel__c", Type: "Text"},
				{Name: "dep__UnitPrice__c", Type: "Integer"},
				{Name: "dep__StartsAt__c", Type: "Date"},
			},
		},
		{
			Name:  "dep__Event__c",
			Label: "Event",
			Fields: []schema.Field{
				{Name: "dep__UnitPrice__c", Type: "Currency"},
				{Name: "dep__StartsAt__c", Type: "Datetime"},
			},
		},
	}})

	result := Analyze(index)
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "UnitPrice")
	assertNoDiagnosticContaining(t, result, "GLADESEMA019", "ordering operator")
	assertNoDiagnosticContaining(t, result, "GLADESEMA018", "DisplayLabel")
}

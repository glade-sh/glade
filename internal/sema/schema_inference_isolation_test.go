package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Source inference fills placeholder fields into the analysis view only. The
// caller's index also builds the runtime org, where an inferred Id-typed
// RecordTypeId would replace the RecordType lookup and skip record-type
// ownership checks. A single-object index must be isolated like a larger one.
func TestProjectReferencedSchemaInferenceLeavesCallerIndexUntouched(t *testing.T) {
	t.Parallel()
	for _, objects := range [][]string{{"Ledger__c"}, {"Ledger__c", "Other__c"}} {
		t.Run(strings.Join(objects, "+"), func(t *testing.T) {
			root := t.TempDir()
			writeSemaFile(t, filepath.Join(root, "SeedsLedger.cls"), `
public class SeedsLedger {
  public static void seed(Id typeId) {
    insert new Ledger__c(Name = 'Ledger', RecordTypeId = typeId);
  }
}
`)
			declared := make([]schema.Object, 0, len(objects))
			for _, name := range objects {
				declared = append(declared, schema.Object{Name: name, RecordTypes: []schema.RecordType{{DeveloperName: "Local", Label: "Local", Active: true}}})
			}
			index := typesys.Build(project.Project{
				Root:      root,
				ApexFiles: []string{filepath.Join(root, "SeedsLedger.cls")},
			}, schema.Schema{Objects: declared})
			enriched := enrichIndexWithProjectReferencedSchemaFields(index)
			inferred := false
			for _, object := range enriched.Objects {
				if strings.EqualFold(object.Name, "Ledger__c") && hasQuerySchemaField(object.Fields, "RecordTypeId") {
					inferred = true
				}
			}
			if !inferred {
				t.Fatalf("analysis view did not infer the referenced field: %#v", enriched.Objects)
			}
			for _, object := range index.Objects {
				if len(object.Fields) != 0 {
					t.Fatalf("inference wrote into the caller's index: %s %#v", object.Name, object.Fields)
				}
			}
		})
	}
}

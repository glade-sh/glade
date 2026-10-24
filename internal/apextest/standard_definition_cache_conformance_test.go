package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardDefinitionCacheAnonymousNamedAndTestRunnerNativeRoutes(t *testing.T) {
	// Reuse the deployed API 62/67 fixture and all six native rows. A caller's
	// local schema changes must not leak into subsequent project runtimes.
	for _, name := range []string{"Account", "Contact"} {
		definition, ok := storage.StandardObjectDefinition(name)
		if !ok {
			t.Fatalf("missing %s catalog definition", name)
		}
		for key, field := range definition.Fields {
			if field.Createable != nil {
				*field.Createable = false
			}
			if field.Nillable != nil {
				*field.Nillable = true
			}
			if len(field.ReferenceTo) != 0 {
				field.ReferenceTo[0] = "MissingCatalogTarget"
			}
			definition.Fields[key] = storage.Field{APIName: key, Type: storage.FieldInteger}
		}
	}
	runRuntimeSchemaIsolationNativeConformance(t)
}

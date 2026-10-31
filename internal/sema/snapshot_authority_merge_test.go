package sema

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Completeness evidence must belong to one captured object, not be assembled
// from unrelated inputs. Native missing-field results: standard_field_access/
// org.tsv:18,22; the merge policy is a local schema-generation boundary.
func TestSnapshotAssertionDoesNotCombineUnrelatedProvenance(t *testing.T) {
	for _, object := range []string{"Account", "Contact"} {
		t.Run(object, func(t *testing.T) {
			for _, tc := range []struct {
				name, objects string
				closed        bool
			}{
				{name: "split-untrusted", objects: fmt.Sprintf(`[{"name":%q,"completeSnapshot":true},{"name":%q,"fieldsComplete":true}]`, object, object)},
				{name: "same-captured-object", objects: fmt.Sprintf(`[{"name":%q,"completeSnapshot":true,"fieldsComplete":true},{"name":%q,"partial":true}]`, object, object), closed: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var index typesys.Index
					if err := json.Unmarshal([]byte(`{"objects":`+tc.objects+`}`), &index); err != nil {
						t.Fatal(err)
					}
					// Empty anonymous indexes skip source enrichment; apply the same
					// object merge directly so both paths use the intended fixture.
					index.Objects = semaMergeEquivalentSchemaObjects(index.Objects)
					result := AnalyzeAnonymous(index, "Object x = new "+object+"().Missing__c;", "67.0")
					if result.HasErrors() != tc.closed {
						t.Fatalf("closed=%t diagnostics=%v", tc.closed, result.Diagnostics)
					}
				})
			}
		})
	}
}

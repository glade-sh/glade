package sema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native missing-field results: standard_field_access/org.tsv:2,6,18,22.
// Completeness assertions and the fallback for old snapshots are local input
// contracts, not Salesforce observations about an arbitrary snapshot.
func TestStandardSnapshotExplicitCompletenessRoutes(t *testing.T) {
	cases := []struct {
		name         string
		assertion    any
		legacyMarker bool
		partial      bool
		closed       bool
	}{
		{name: "legacy"}, {name: "old-inferred-marker", legacyMarker: true}, {name: "explicit-false", assertion: false},
		{name: "explicit-true", assertion: true, closed: true},
		{name: "contradictory-partial", assertion: true, partial: true},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Account", "Contact"} {
			for _, tc := range cases {
				tc := tc
				t.Run(api+"/"+object+"/"+tc.name, func(t *testing.T) {
					root := t.TempDir()
					source := filepath.Join(root, "Probe.cls")
					body := "Object x = new " + object + "().Missing__c;"
					writeSemaFile(t, source, "@IsTest public class Probe { @IsTest static void run() { "+body+" } }")
					entry := map[string]any{"name": object, "fields": []any{map[string]any{"name": "Known__c", "type": "Text"}}}
					if tc.assertion != nil {
						entry["fieldsComplete"] = tc.assertion
					}
					if tc.legacyMarker {
						entry["completeSnapshot"] = true
					}
					if tc.partial {
						entry["partial"] = true
					}
					raw, err := json.Marshal(map[string]any{"objects": []any{entry}})
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "snapshot.json")
					if err := os.WriteFile(path, raw, 0600); err != nil {
						t.Fatal(err)
					}
					sum := sha256.Sum256(raw)
					p := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{source}, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)}
					writeSemaFile(t, filepath.Join(root, "glade.yml"), fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %x\n", sum))
					loaded, err := schema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, loaded)
					serialized, err := json.Marshal(index)
					if err != nil {
						t.Fatal(err)
					}
					var restored typesys.Index
					if err := json.Unmarshal(serialized, &restored); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(filepath.Join(root, "glade.yml")); err != nil {
						t.Fatal(err)
					}
					for _, route := range []string{"anonymous", "named"} {
						for _, idx := range []typesys.Index{index, restored} {
							for _, phase := range []string{"cold", "warm"} {
								var result Result
								if route == "anonymous" {
									result = AnalyzeAnonymous(idx, body, api)
								} else {
									result = Analyze(idx)
								}
								if result.HasErrors() != tc.closed {
									t.Errorf("%s %s closed=%t diagnostics=%v", route, phase, tc.closed, result.Diagnostics)
								}
							}
						}
					}
					known := AnalyzeAnonymous(index, "Object x = new "+object+"().Known__c;", api)
					if known.HasErrors() {
						t.Errorf("known field rejected: %v", known.Diagnostics)
					}
					if tc.legacyMarker {
						var oldIndex typesys.Index
						oldJSON := fmt.Sprintf(`{"project":{"root":%q},"objects":[{"name":%q,"completeSnapshot":true}]}`, root, object)
						if err := json.Unmarshal([]byte(oldJSON), &oldIndex); err != nil {
							t.Fatal(err)
						}
						if result := AnalyzeAnonymous(oldIndex, body, api); result.HasErrors() {
							t.Errorf("old inferred index must remain open: %v", result.Diagnostics)
						}
					}
					if tc.closed && !strings.Contains(string(serialized), `"completeSnapshot":true`) {
						t.Errorf("missing explicit assertion from serialized index: %s", serialized)
					}
				})
			}
		}
	}
}

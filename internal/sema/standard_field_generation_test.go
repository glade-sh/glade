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

// The result belongs to the indexed schema generation, even if the snapshot
// binding changes after that generation was created. The native missing/known
// field outcomes are in standard_field_access/org.tsv:2,6,14,16,18,22,30,32.
func TestStandardFieldSchemaGenerationStability(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Account", "Contact"} {
			t.Run(api+"/"+object, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "FieldGeneration.cls")
				writeSemaFile(t, file, "public class FieldGeneration { public static void run() { Object x = new "+object+"().Missing__c; } }")
				p := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}
				// An object metadata overlay may be marked non-partial without being an
				// org snapshot; it must remain open after a snapshot appears on disk.
				openShape := schema.Schema{Objects: []schema.Object{{Name: object}}}
				openIndex := typesys.Build(p, openShape)
				check := func(label string, idx typesys.Index, closed bool) {
					t.Helper()
					resetAnonymousSetupCache()
					for _, route := range []string{"anonymous", "named"} {
						for _, phase := range []string{"cold", "warm"} {
							var result Result
							if route == "anonymous" {
								result = AnalyzeAnonymous(idx, "Object x = new "+object+"().Missing__c;", api)
							} else {
								result = Analyze(idx)
							}
							if result.HasErrors() != closed {
								t.Errorf("%s %s %s: closed=%t diagnostics=%v", label, route, phase, closed, result.Diagnostics)
							}
						}
					}
				}
				check("open before binding", openIndex, false)
				shape := schema.Schema{Objects: []schema.Object{{Name: object, CompleteSnapshot: true, Fields: []schema.Field{{Name: "Known__c", Type: "Text"}}}}}
				raw, err := json.Marshal(shape)
				if err != nil {
					t.Fatal(err)
				}
				raw = []byte(strings.ReplaceAll(string(raw), `"completeSnapshot"`, `"fieldsComplete"`))
				path := filepath.Join(root, "snapshot.json")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(raw)
				hash := fmt.Sprintf("%x", digest)
				cfg := filepath.Join(root, "glade.yml")
				writeSemaFile(t, cfg, fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %s\n", hash))
				check("old open after binding", openIndex, false)
				p.SchemaSnapshot = path
				p.SchemaSnapshotSHA256 = hash
				loaded, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				closedIndex := typesys.Build(p, loaded)
				check("new closed", closedIndex, true)
				known := AnalyzeAnonymous(closedIndex, "Object x = new "+object+"().Known__c;", api)
				if known.HasErrors() {
					t.Fatalf("known field rejected: %v", known.Diagnostics)
				}
				// Persisted index restoration must preserve the same generation. Old
				// representations without provenance must conservatively remain open.
				encoded, err := json.Marshal(closedIndex)
				if err != nil {
					t.Fatal(err)
				}
				var restored typesys.Index
				if err := json.Unmarshal(encoded, &restored); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(cfg); err != nil {
					t.Fatal(err)
				}
				check("closed after removing binding", closedIndex, true)
				check("restored after removing binding", restored, true)
				if strings.Contains(string(encoded), "Known__c") == false {
					t.Fatal("schema not serialized")
				}
				if err := os.WriteFile(path, []byte(`{"objects":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, cfg, fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %s\n", hash))
				check("closed after invalidating snapshot", closedIndex, true)
				check("old open after invalidating snapshot", openIndex, false)
				replacement, err := json.Marshal(schema.Schema{Objects: []schema.Object{{Name: object, CompleteSnapshot: true, Fields: []schema.Field{{Name: "Other__c", Type: "Text"}}}}})
				if err != nil {
					t.Fatal(err)
				}
				replacement = []byte(strings.ReplaceAll(string(replacement), `"completeSnapshot"`, `"fieldsComplete"`))
				if err := os.WriteFile(path, replacement, 0600); err != nil {
					t.Fatal(err)
				}
				replacementDigest := sha256.Sum256(replacement)
				writeSemaFile(t, cfg, fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %x\n", replacementDigest))
				check("closed after replacing snapshot", closedIndex, true)
				check("old open after replacing snapshot", openIndex, false)
				if result := AnalyzeAnonymous(closedIndex, "Object x = new "+object+"().Known__c;", api); result.HasErrors() {
					t.Errorf("replacement changed the old indexed fields: %v", result.Diagnostics)
				}
			})
		}
	}
}

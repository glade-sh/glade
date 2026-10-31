package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A reused server index must not borrow completeness from a later snapshot.
// Native rejection/control rows: standard_field_access/org.tsv:18,22,30,32.
func TestStandardFieldServerSchemaGeneration(t *testing.T) {
	for _, object := range []string{"Account", "Contact"} {
		t.Run(object, func(t *testing.T) {
			root := t.TempDir()
			org := testOrg()
			handler := New(&org)
			p := project.Project{Root: root}
			shape := schema.Schema{Objects: []schema.Object{{Name: object}}}
			openIndex := typesys.Build(p, shape)
			handler.Index = &openIndex
			snapshot := schema.Schema{Objects: []schema.Object{{Name: object, CompleteSnapshot: true, Fields: []schema.Field{{Name: "Known__c", Type: "Text"}}}}}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			raw = []byte(strings.ReplaceAll(string(raw), `"completeSnapshot"`, `"fieldsComplete"`))
			path := filepath.Join(root, "schema.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			hash := fmt.Sprintf("%x", digest)
			cfg := filepath.Join(root, "glade.yml")
			if err := os.WriteFile(cfg, []byte(fmt.Sprintf("project:\n  schemaSnapshot: schema.json\n  schemaSnapshotSHA256: %s\n", hash)), 0600); err != nil {
				t.Fatal(err)
			}
			call := func(field string) bool {
				t.Helper()
				body, err := json.Marshal(map[string]string{"anonymousBody": "Object x = new " + object + "()." + field + ";"})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(body))))
				var response struct {
					Compiled bool `json:"compiled"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("response %s: %v", rec.Body.String(), err)
				}
				return response.Compiled
			}
			for i := 0; i < 2; i++ {
				if !call("Known__c") {
					t.Error("old open index rejected after adding snapshot")
				}
			}
			p.SchemaSnapshot = path
			p.SchemaSnapshotSHA256 = hash
			loaded, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			closedIndex := typesys.Build(p, loaded)
			handler.Index = &closedIndex
			if call("Missing__c") {
				t.Error("complete captured schema accepted unknown field")
			}
			if err := os.Remove(cfg); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if call("Missing__c") {
					t.Error("closed index reopened after removing snapshot binding")
				}
			}
		})
	}
}

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

// Native rejection rows are in standard_field_access/org.tsv:18,22. Snapshot
// completeness and reuse of a server index are local input boundaries.
func TestStandardSnapshotAssertionServer(t *testing.T) {
	for _, object := range []string{"Account", "Contact"} {
		for _, tc := range []struct {
			name            string
			marker          any
			partial, closed bool
		}{
			{name: "legacy"}, {name: "false", marker: false}, {name: "true", marker: true, closed: true}, {name: "partial", marker: true, partial: true},
		} {
			tc := tc
			t.Run(object+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				entry := map[string]any{"name": object}
				if tc.marker != nil {
					entry["fieldsComplete"] = tc.marker
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
				p := project.Project{Root: root, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)}
				loaded, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, loaded)
				org := testOrg()
				handler := New(&org)
				handler.Index = &index
				body, err := json.Marshal(map[string]string{"anonymousBody": "Object x = new " + object + "().Missing__c;"})
				if err != nil {
					t.Fatal(err)
				}
				for _, phase := range []string{"cold", "warm"} {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(body))))
					var response struct {
						Compiled bool `json:"compiled"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatalf("%s response=%s: %v", phase, rec.Body.String(), err)
					}
					if response.Compiled == tc.closed {
						t.Errorf("%s closed=%t response=%s", phase, tc.closed, rec.Body.String())
					}
				}
			})
		}
	}
}

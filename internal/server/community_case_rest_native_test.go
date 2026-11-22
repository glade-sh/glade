package server

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// NATIVE testdata/community_case_rest/org.tsv:2-3, authenticated Apex REST.
func TestCommunityCaseRESTNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "community_case_rest")
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeServerTestFile(t, filepath.Join(root, rel), string(data))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, sch)
			if a := sema.Analyze(idx); idx.HasErrors() || a.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", idx.Diagnostics, a.Diagnostics)
			}
			org := communityCaseCompileOrg(row[1])
			host := New(&org)
			host.SetProjectIndex(idx)
			version := strings.TrimSuffix(row[1], ".0")
			req := httptest.NewRequest(http.MethodPost, "/services/apexrest/RouteCaseRest"+version+"/", strings.NewReader("{}"))
			req.Header.Set("X-GLADE-User-Id", "005000000000002")
			rec := httptest.NewRecorder()
			host.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
			}
			var got string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got != row[3] {
				t.Fatalf("Glade=%q native=%q", got, row[3])
			}
		})
	}
}

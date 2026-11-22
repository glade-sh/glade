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
	"github.com/glade-sh/glade/internal/storage"
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
	if len(rows) != 7 {
		t.Fatalf("got %d rows", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		variants := []string{"native"}
		if row[0] == "case_rest" {
			variants = append(variants, "uncaptured-no-api", "uncaptured-no-origin", "uncaptured-api68")
		}
		for _, variant := range variants {
			variant := variant
			t.Run(row[0]+"/"+row[1]+"/"+variant, func(t *testing.T) {
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
					if strings.HasSuffix(path, ".cls-meta.xml") && variant == "uncaptured-api68" {
						data = []byte(strings.ReplaceAll(string(data), "<apiVersion>"+row[1]+"</apiVersion>", "<apiVersion>68.0</apiVersion>"))
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
				if variant == "uncaptured-no-api" {
					profiles := org.Objects["Profile"]
					p := profiles.Records["00e000000000002"]
					p.Fields["PermissionsApiEnabled"] = storage.BooleanValue(false)
					profiles.Records[p.ID] = p
					org.Objects["Profile"] = profiles
				}
				if variant == "uncaptured-no-origin" {
					fields := org.Objects["FieldPermissions"]
					delete(fields.Records, "0FP000000000002")
					org.Objects["FieldPermissions"] = fields
				}
				host := New(&org)
				host.SetProjectIndex(idx)
				version := strings.TrimSuffix(row[1], ".0")
				suffix := map[string]string{"case_rest": "", "case_constructed": "Constructed", "case_sharing": "Sharing"}[row[0]]
				req := httptest.NewRequest(http.MethodPost, "/services/apexrest/RouteCaseRest"+version+suffix+"/", strings.NewReader("{}"))
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
				want := row[3]
				if variant == "uncaptured-no-api" || variant == "uncaptured-no-origin" || variant == "uncaptured-api68" {
					want = "describe=true,true,true,true|insert=ok|update=ok|delete=ok"
				}
				if got != want {
					t.Fatalf("Glade=%q want=%q", got, want)
				}
			})
		}
	}
}

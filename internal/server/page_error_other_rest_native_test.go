package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestOtherMissingPDFRestNative(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "page_error_rest_controls")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want struct {
				HTTP int `json:"http"`
				Body any `json:"body"`
			}
			found := false
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) == 4 && row[0] == api && row[1] == "rest" && row[2] == "other" {
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("missing native REST observation")
			}
			root := t.TempDir()
			src := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, path)
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
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureDeterministicPlatformData(&org)
			host := newLightningTestServer(t, &org, source)
			host.SetProjectIndex(idx)
			version := strings.TrimSuffix(api, ".0")
			for _, phase := range []string{"cold", "warm"} {
				rec := httptest.NewRecorder()
				host.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/apexrest/page-control-other-"+version+"/", nil))
				var got any
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("%s: %v body=%s", phase, err, rec.Body.String())
				}
				if rec.Code != want.HTTP || !reflect.DeepEqual(got, want.Body) {
					t.Fatalf("%s native HTTP %d body=%v; Glade HTTP %d body=%v", phase, want.HTTP, want.Body, rec.Code, got)
				}
			}
		})
	}
}

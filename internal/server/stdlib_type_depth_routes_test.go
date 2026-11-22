package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replays captured anonymous rows through the Tooling API host.
func TestStdlibTypeDepthToolingRoute(t *testing.T) {
	const api = "67.0"
	fixtures := []string{"stdlib_type_depth", "stdlib_type_depth_neighbors", "stdlib_comparable_interface", "stdlib_comparable_inheritance"}
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	for _, fixture := range fixtures {
		dir := filepath.Join("../apextest/testdata/conformance", fixture, "force-app/main/default/classes")
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes", entry.Name()), strings.ReplaceAll(string(body), "__API__", api))
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = api
	srv := newLightningTestServer(t, &org, metadata)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	seen := 0
	for _, fixture := range fixtures {
		dir := filepath.Join("../apextest/testdata/conformance", fixture)
		f, err := os.Open(filepath.Join(dir, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			if row[1] != api || row[2] != "anonymous" {
				continue
			}
			seen++
			t.Run(row[0], func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(dir, row[0]+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				payload, err := json.Marshal(map[string]string{"anonymousBody": string(body)})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
				var got struct {
					Compiled         bool    `json:"compiled"`
					Success          bool    `json:"success"`
					ExceptionMessage *string `json:"exceptionMessage"`
					Logs             string  `json:"logs"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("response %q: %v", rec.Body.String(), err)
				}
				wantSuccess := row[4] == "True"
				if rec.Code != http.StatusOK || !got.Compiled || got.Success != wantSuccess {
					t.Fatalf("native %q: response %d %s", row[5], rec.Code, rec.Body.String())
				}
				if !wantSuccess && (got.ExceptionMessage == nil || !strings.Contains(*got.ExceptionMessage, row[5])) {
					t.Fatalf("native %q: response %s", row[5], rec.Body.String())
				}
				if wantSuccess && row[5] != "ok" && !strings.Contains(got.Logs, row[5]) {
					t.Fatalf("native %q: logs %q", row[5], got.Logs)
				}
			})
		}
	}
	if seen != 14 {
		t.Fatalf("selected %d native rows, want 14", seen)
	}
}

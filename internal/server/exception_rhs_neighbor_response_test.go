package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The uncaptured standard-exception response remains the origin/main output.
func TestExceptionRHSAnonymousNeighborResponse(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	dir := filepath.Join(root, "force-app/main/default/classes")
	writeServerTestFile(t, filepath.Join(dir, "Caller.cls"), `public class Caller { public static void fail() { throw new IllegalArgumentException('neighbor'); } }`)
	writeServerTestFile(t, filepath.Join(dir, "Caller.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`)
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
	srv := newLightningTestServer(t, &org, metadata)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(`{"anonymousBody":"Caller.fail();"}`)))
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"compiled": true, "success": false, "line": float64(1), "column": float64(1), "logs": "", "exceptionStackTrace": nil, "compileProblem": nil, "exceptionMessage": "IllegalArgumentException: neighbor"}
	if rec.Code != http.StatusOK || len(got) != len(want) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %#v, want %#v; body=%s", key, got[key], value, rec.Body.String())
		}
	}
}

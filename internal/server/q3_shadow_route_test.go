package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native shadowed-root observations: q3share/org.tsv:16-19.
func TestQ3ShadowedRootsServer(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for _, ext := range []string{".cls", ".cls-meta.xml"} {
		b, err := os.ReadFile("../apextest/testdata/conformance/q3share/shadow/classes/Q3Shadow67" + ext)
		if err != nil {
			t.Fatal(err)
		}
		writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Q3Shadow67"+ext), string(b))
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	org := storage.NewOrgState()
	host := New(&org)
	host.SetProjectRuntime(index, nil, nil)
	for _, api := range []string{"62.0", "67.0"} {
		for run := 0; run < 2; run++ {
			body, err := json.Marshal(map[string]string{"anonymousBody": "Q3Shadow67.run();"})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			host.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(body))))
			if api == "62.0" {
				assertSalesforceError(t, rec, http.StatusNotFound, "NOT_FOUND", "unsupported API version v62.0")
				continue
			}
			var got struct {
				Compiled, Success bool
				ExceptionMessage  string `json:"exceptionMessage"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			// Tooling anonymous execution links the project classes, so the
			// fixture reaches the shadowed-root expression and completes.
			if rec.Code != http.StatusOK || !got.Compiled || !got.Success || got.ExceptionMessage != "" {
				t.Fatalf("run=%d status=%d body=%s", run, rec.Code, rec.Body.String())
			}
		}
	}
}

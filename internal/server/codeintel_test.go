package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestToolingGladeCodeIntelEndpoints(t *testing.T) {
	org := testOrg()
	handler := NewWithSource(&org, SourceMetadata{Project: project.Project{Root: t.TempDir()}})
	index := testCodeIntelIndex(t)
	handler.SetProjectIndex(index)

	tests := []struct {
		name    string
		path    string
		wantMin int
	}{
		{name: "symbols", path: serverTestDataPath + "/tooling/glade/symbols", wantMin: 2},
		{name: "definition", path: serverTestDataPath + "/tooling/glade/definition?symbol=Account.Name", wantMin: 1},
		{name: "references", path: serverTestDataPath + "/tooling/glade/references?symbol=Account.Name", wantMin: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			var payload struct {
				TotalSize int               `json:"totalSize"`
				Done      bool              `json:"done"`
				Records   []json.RawMessage `json:"records"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
			}
			if !payload.Done || payload.TotalSize < tc.wantMin || len(payload.Records) != payload.TotalSize {
				t.Fatalf("payload = %#v body=%s", payload, rec.Body.String())
			}
		})
	}
}

func TestToolingGladeCodeIntelRequiresGet(t *testing.T) {
	org := testOrg()
	handler := NewWithSource(&org, testSourceMetadata(t))
	handler.SetProjectIndex(typesys.Index{})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, serverTestDataPath+"/tooling/glade/symbols", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func testCodeIntelIndex(t *testing.T) typesys.Index {
	t.Helper()
	root := t.TempDir()
	apexPath := filepath.Join(root, "Reader.cls")
	source := "public class Reader { void read(){ for (Account acct : [SELECT Name FROM Account]) { System.debug(acct.Name); } } }"
	if err := os.WriteFile(apexPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write Reader.cls: %v", err)
	}
	return typesys.Build(project.Project{Root: root, ApexFiles: []string{apexPath}}, schema.Schema{
		Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Name", Type: "Text"},
			},
		}},
	})
}

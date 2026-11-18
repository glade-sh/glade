package server

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Namespace spelling does not override the actual record ID's object.
func TestRecordNavigationNamespaceNative(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "pr163_namespaced_navigation", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"namespace":"own","packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	const object = "own__ProbeNavC163__c"
	writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Custom_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata"><sobjectType>`+object+`</sobjectType><type>RecordPage</type></FlexiPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "own"
	id := storage.ID("a17000000000001AAA")
	org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, KeyPrefix: "a17"}, Records: map[storage.ID]storage.Record{id: {ID: id, Object: object}}}
	handler := newLightningTestServer(t, &org, SourceMetadata{Project: p})
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0], func(t *testing.T) {
			requested := strings.ReplaceAll(row[0], "$NS", "own")
			want := strings.ReplaceAll(row[4], "$NS", "own")
			req := httptest.NewRequest(http.MethodGet, "/lwc/preview/record/"+requested+"/"+string(id), nil)
			shell, redirect, diags, err := handler.resolveLWCShellRequest(req, []string{"record", requested, string(id)})
			if err != nil || shell.Context.ObjectAPIName != want {
				t.Fatalf("object %q, want %q; error %v; diagnostics %#v", shell.Context.ObjectAPIName, want, err, diags)
			}
			expected := ""
			if requested != want {
				expected = "/lwc/preview/record/" + want + "/" + string(id)
			}
			if redirect != expected {
				t.Fatalf("redirect %q, want %q", redirect, expected)
			}
			if expected != "" {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != http.StatusFound || response.Header().Get("Location") != expected {
					t.Fatalf("HTTP redirect %d %q, want %q", response.Code, response.Header().Get("Location"), expected)
				}
			}
		})
	}
}

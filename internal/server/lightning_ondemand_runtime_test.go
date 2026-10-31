package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestLightningApexOnDemandRuntime(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/RowController.cls"), `public class RowController { @AuraEnabled public static String read() { return 'row'; } }`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			host := newLightningTestServer(t, &org, source)
			host.SetProjectRuntime(typesys.Build(p, sch), nil, nil)
			for i := 0; i < 2; i++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/RowController/read", strings.NewReader(`{}`))
				host.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("request %d: status %d: %s", i, rec.Code, rec.Body.String())
				}
				var result lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Error != nil || result.Data != "row" {
					t.Fatalf("request %d: result %#v", i, result)
				}
			}
		})
	}
}

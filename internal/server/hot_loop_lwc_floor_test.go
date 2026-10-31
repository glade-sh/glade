package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Bundle API version is independent of the invoked Apex class source version.
func TestHotLoopLightningFloorRoutes(t *testing.T) {
	controller, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "hot_loop_routes", "LoopShapes.cls"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls"), string(controller))
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	bundle := filepath.Join(root, "force-app/main/default/lwc/loopHost")
	writeServerTestFile(t, filepath.Join(bundle, "loopHost.js"), `import run from '@salesforce/apex/LoopShapes.run'; export default run;`)
	writeServerTestFile(t, filepath.Join(bundle, "loopHost.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.APIVersion = "62.0"
	handler := newLightningTestServer(t, &org, source)
	handler.SetProjectIndex(typesys.Build(p, schema))
	if handler.runtimeErr != nil {
		t.Fatal(handler.runtimeErr)
	}
	const want = "300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;"
	for _, route := range []struct{ name, path, body string }{
		{"wire", "/lightning/wire/apex", `{"className":"LoopShapes","method":"run","params":{}}`},
		{"imperative", "/lightning/apex/LoopShapes/run", `{}`},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, state := range []string{"cold", "warm"} {
				t.Run(state, func(t *testing.T) {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
						t.Fatalf("status=%d response=%+v", rec.Code, out)
					}
				})
			}
		})
	}
}

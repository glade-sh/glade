package server

import (
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

// The page floor invokes an Apex controller at the Apex source floor.
func TestHotLoopVisualforceFloorRoute(t *testing.T) {
	controller, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "hot_loop_routes", "LoopShapes.cls"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls"), string(controller))
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LoopHost.page"), `<apex:page controller="LoopShapes" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LoopHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Loop host</label></ApexPage>`)
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
	configureVisualforceTestPrincipal(t, handler)
	handler.SetProjectIndex(typesys.Build(p, schema))
	if handler.runtimeErr != nil {
		t.Fatal(handler.runtimeErr)
	}
	const want = "300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;"
	for _, state := range []string{"cold", "warm"} {
		t.Run(state, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/LoopHost", nil))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ">"+want+"<") {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

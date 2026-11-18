package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFlowLoopCommittedAnonymousServer(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_loop_relationship"
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"RouteProbe", "RouteProbeTest"} {
		source, err := os.ReadFile(filepath.Join(fixture, "sources/classes", name+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeServerTestFile(t, path, string(source))
		writeServerTestFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	flow, err := os.ReadFile(filepath.Join(fixture, "metadata/api67/flows/RouteLoop.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/flows/RouteLoop.flow-meta.xml"), string(flow))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	shape, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, shape)
	flows, err := automation.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := apextest.OrgFromIndex(index)
	automation.ApplyToOrg(&org, flows)
	handler := New(&org)
	handler.SetProjectIndex(index)
	if handler.runtimeErr != nil {
		t.Fatal(handler.runtimeErr)
	}
	execute := func(source string) map[string]any {
		t.Helper()
		body, err := json.Marshal(map[string]string{"anonymousBody": source})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(body))))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	first := execute(`System.debug('V036OBS|' + JSON.serialize(RouteProbe.observe()));`)
	if first["success"] != true || !strings.Contains(first["logs"].(string), `"parent":"RouteParent"`) {
		t.Fatalf("native committed observation; Glade=%v", first)
	}
	failed := execute(`insert new Account(Name='RouteLoopTrigger'); insert new Account();`)
	if failed["success"] != false || !strings.Contains(failed["exceptionMessage"].(string), "REQUIRED_FIELD_MISSING") {
		t.Fatalf("native required-field rejection; Glade=%v", failed)
	}
	last := execute(`System.debug('V036COUNT|' + [SELECT COUNT() FROM Account WHERE Name = 'RouteLoopTrigger']);`)
	if last["success"] != true || !strings.Contains(last["logs"].(string), "V036COUNT|4") {
		t.Fatalf("native committed count 4; Glade=%v", last)
	}
}

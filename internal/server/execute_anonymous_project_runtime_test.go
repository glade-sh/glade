package server

import (
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

// The tooling executeAnonymous route runs anonymous Apex against the project's
// classes in the org namespace, as glade exec does. Native rows:
// testdata/conformance/namespace_anonymous_owner (Type.forName returns the
// namespace-qualified name; project classes are callable unqualified).
func TestToolingExecuteAnonymousUsesProjectRuntimeAndNamespace(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"gladens004","sourceApiVersion":"67.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Greeter.cls"), `public class Greeter { public static String hi() { return 'hi'; } }`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Greeter.cls-meta.xml"), `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`)
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
	org.Namespace = "gladens004"
	srv := newLightningTestServer(t, &org, source)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	rec := httptest.NewRecorder()
	body := `{"anonymousBody":"System.debug('OBS|' + Greeter.hi() + '|' + String.valueOf(Type.forName('Greeter')));"}`
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(body)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "OBS|hi|gladens004.Greeter") {
		t.Fatalf("logs lack project class result and namespace: %s", rec.Body.String())
	}
}

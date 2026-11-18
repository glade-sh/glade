package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native positive locals: namespace_unlocked_subscriber/installed/org.tsv.
// The captured provider is indexed; an independent unavailable alias does
// not supply membership. The host invokes an unrelated safe method.
func TestUnlockedSubscriberLightningAndVisualforceRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"v036-unlocked"}]}]}`)
			provider, err := os.ReadFile(filepath.Join("..", "..", "testdata", "conformance", "namespace_unlocked_subscriber", "provider", "package", "main", "default", "classes", "V036PkgOuter.cls"))
			if err != nil {
				t.Fatal(err)
			}
			providerPath := filepath.Join(root, "force-app/main/default/classes/V036PkgOuter.cls")
			writeServerTestFile(t, providerPath, string(provider))
			writeServerTestFile(t, providerPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			path := filepath.Join(root, "force-app/main/default/classes/UnlockedHostV036.cls")
			writeServerTestFile(t, path, `public class UnlockedHostV036 {
    @AuraEnabled public static Integer ping() {
        if (false) { V036PkgOuter.Box localBox; V036PkgOuter.Shape localShape; }
        return 7;
    }
    public Integer getResult() { return ping(); }
}`)
			writeServerTestFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/UnlockedHostV036.page"), `<apex:page controller="UnlockedHostV036" showHeader="false" sidebar="false"><apex:outputText value="{!result}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/UnlockedHostV036.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Synthetic host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			s, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			for _, d := range sema.Analyze(index).Diagnostics {
				if d.Severity == diagnostic.Error && strings.Contains(d.Message, "V036PkgOuter") {
					t.Fatalf("indexed package member rejected: %s", d.Message)
				}
			}
			org := apextest.OrgFromIndex(index)
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(index)
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					response := httptest.NewRecorder()
					request := httptest.NewRequest(http.MethodPost, "/lightning/apex/UnlockedHostV036/ping", strings.NewReader(`{}`))
					request.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(response, request)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil || response.Code != http.StatusOK || out.Error != nil || fmt.Sprint(out.Data) != "7" {
						t.Fatalf("LWC status=%d response=%s err=%v", response.Code, response.Body.String(), err)
					}
					response = httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apex/UnlockedHostV036", nil))
					if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), ">7<") {
						t.Fatalf("VF status=%d response=%s", response.Code, response.Body.String())
					}
				})
			}
		})
	}
}

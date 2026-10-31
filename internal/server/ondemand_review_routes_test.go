package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// These are supplemental host-route controls for the existing shared-runtime
// regression; they preserve the local USER_MODE and wire contracts.
func TestVisualforceOnDemandLookupSearchAuthorization(t *testing.T) {
	for _, deniedField := range []struct{ object, id string }{
		{"ObjectPermissions", "110000000000097"},
		{"FieldPermissions", "120000000000097"},
	} {
		t.Run(deniedField.object, func(t *testing.T) {
			apextest.InvalidateRuntimeCaches()
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			const name = "VF Popup & <owned>"
			host := newVisualforceLookupRouteServer(t, vfAccessOwnerID, name)
			sch, err := schema.LoadProject(host.Source.Project)
			if err != nil {
				t.Fatal(err)
			}
			host.SetProjectRuntime(typesys.Build(host.Source.Project, sch), nil, nil)
			if host.Index == nil || host.runtime != nil {
				t.Fatal("expected on-demand project runtime")
			}
			const path = "/_ui/common/data/LookupPage?lkfm=ownedForm&lknm=ownedForm:owned&lktp=005&lksrch=vf%20POPUP"
			for i, allowed := range []bool{false, true, false, true} {
				permissions := host.Org.Objects[deniedField.object]
				grant := permissions.Records[storage.ID(deniedField.id)]
				grant.Fields["PermissionsRead"] = storage.BooleanValue(allowed)
				permissions.Records[grant.ID] = grant
				host.Org.Objects[deniedField.object] = permissions
				response := requestVisualforceLookupRoute(host, path, "")
				body := response.Body.String()
				if allowed {
					if response.Code != http.StatusOK || !strings.Contains(body, html.EscapeString(name)) || !strings.Contains(body, string(vfAccessOwnerID)) || !strings.Contains(body, "gladeVisualforceSelectLookup") || strings.Contains(body, name) {
						t.Fatalf("request %d allowed: status=%d body=%q", i, response.Code, body)
					}
				} else if response.Code != http.StatusNotFound || strings.Contains(body, name) || strings.Contains(body, html.EscapeString(name)) || strings.Contains(body, string(vfAccessOwnerID)) {
					t.Fatalf("request %d denied: status=%d body=%q", i, response.Code, body)
				}
			}
		})
	}
}

func TestLightningWireApexOnDemandRuntime(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			apextest.InvalidateRuntimeCaches()
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/WireRowController.cls"), `public class WireRowController { private static Integer calls = 0; @AuraEnabled(cacheable=true) public static String read(String value) { calls++; return value + ':' + calls; } }`)
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
			if host.Index == nil || host.runtime != nil {
				t.Fatal("expected on-demand project runtime")
			}
			for i, value := range []string{"cold", "warm", "cold"} {
				body := `{"className":"WireRowController","method":"read","params":{"value":"` + value + `"},"cacheable":true}`
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(body))
				host.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("request %d: status=%d body=%s", i, rec.Code, rec.Body.String())
				}
				var result lwcbrowser.WireResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Error != nil || result.Data != value+":1" {
					t.Fatalf("request %d: result %#v", i, result)
				}
			}
		})
	}
}

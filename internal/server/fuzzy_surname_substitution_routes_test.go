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

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The HTTP and controller hosts replay the captured Find and DML helper. These
// are local route twins of surname_substitution/org.tsv, not native host rows.
func TestFuzzySurnameSubstitutionHostRoutes(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	const dir = "../dml/testdata/conformance/surname_substitution"
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			t.Fatal(line)
		}
		if f[1] == "anonymous" && f[2] == "1" {
			rows[f[0]+"/"+f[4]] = f[5]
		}
	}
	helper, err := os.ReadFile(filepath.Join(dir, "panel.cls"))
	if err != nil {
		t.Fatal(err)
	}
	controller := strings.Replace(string(helper), "class SurnameBehavior", "class AjaxController", 1)
	controller = strings.ReplaceAll(controller, "public static String m_", "@AuraEnabled @RemoteAction public static String m_")
	// Every position, neighbor geometry and length, both objects and directions;
	// also cover far keys and each missing-corroboration control.
	methods := []string{
		"m_J_Contact_first_row_fwd", "m_J_Lead_middle_vertical_rev",
		"m_J_Contact_final_diagonal_fwd", "m_J_Lead_first_vertical_fwd",
		"m_J_Contact_middle_diagonal_rev", "m_J_Lead_final_row_rev",
		"m_J_Contact_first_far_fwd", "m_J_Lead_middle_far_rev",
		"m_K_Contact_noemail_fwd", "m_K_Lead_diffemail_rev",
		"m_K_Contact_diffname_rev", "m_K_Lead_phone_fwd",
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range methods {
			for _, route := range []string{"tooling", "lightning", "visualforce"} {
				t.Run(api+"/"+route+"/"+method, func(t *testing.T) {
					want := rows[api+"/"+method]
					if want == "" {
						t.Fatal("missing native row")
					}
					root := t.TempDir()
					writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					base := filepath.Join(root, "force-app/main/default")
					writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls"), controller)
					writeServerTestFile(t, filepath.Join(base, "classes/AjaxController.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					writeServerTestFile(t, filepath.Join(base, "pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
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
					org.APIVersion = api
					org.StandardDuplicateRules = true
					storage.EnsureDeterministicPlatformData(&org)
					srv := newLightningTestServer(t, &org, source)
					configureVisualforceTestPrincipal(t, srv)
					srv.SetProjectIndex(typesys.Build(p, schema))
					if srv.runtimeErr != nil {
						t.Fatal(srv.runtimeErr)
					}
					switch route {
					case "tooling":
						source := "System.assertEquals('" + want + "', AjaxController." + method + "());"
						data, err := json.Marshal(map[string]string{"anonymousBody": source})
						if err != nil {
							t.Fatal(err)
						}
						rec := httptest.NewRecorder()
						srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(data))))
						if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
					case "lightning":
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/"+method, strings.NewReader(`{}`))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if out.Error != nil || out.Data != want {
							t.Fatalf("response=%+v want=%s", out, want)
						}
					case "visualforce":
						data := `[{"action":"AjaxController","method":"` + method + `","data":[],"type":"rpc","tid":1}]`
						out := postVisualforceRemotingWithViewState(t, srv, data)
						if len(out) != 1 || !out[0].Status || out[0].Result != want {
							t.Fatalf("response=%+v want=%s", out, want)
						}
					}
				})
			}
		}
	}
}

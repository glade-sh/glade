package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The synthetic native rows cover the metadata and runtime boundaries exercised
// through the same project loader by VF, LWC, and Tooling hosts.
func TestProcessBuilderReviewAdditionalHosts(t *testing.T) {
	scenarios := []struct {
		fixture, method string
		parents         []string
	}{
		{"process_builder_review_multiprocess", "multiprocess", nil},
		{"process_builder_review_reverse", "multiprocess", nil},
		{"process_builder_review_fourreverse", "fourflows", nil},
		{"process_builder_review_usersingle", "userwrapped", []string{"process_builder_review_mixed", "process_builder_review_wrapped"}},
		{"process_builder_review_html", "lookuphtml", []string{"process_builder_review_lookup"}},
		{"process_builder_review_afteroutputs", "lookuphtml", []string{"process_builder_review_lookup", "process_builder_review_html"}},
		{"process_builder_review_afteroutputs", "lookupmissing", []string{"process_builder_review_lookup", "process_builder_review_html"}},
		{"process_builder_review_afteroutputs", "lookupreject", []string{"process_builder_review_lookup", "process_builder_review_html"}},
		{"process_builder_review_afterprojection", "lookuphtml", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs"}},
		{"process_builder_review_afterprojection", "lookupmissing", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs"}},
		{"process_builder_review_afterprojection", "lookupreject", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs"}},
		{"process_builder_review_afterbinding", "lookuphtml", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs", "process_builder_review_afterprojection"}},
		{"process_builder_review_afterbinding", "lookupmissing", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs", "process_builder_review_afterprojection"}},
		{"process_builder_review_afterbinding", "lookupreject", []string{"process_builder_review_lookup", "process_builder_review_html", "process_builder_review_afteroutputs", "process_builder_review_afterprojection"}},
		{"process_builder_review_record", "recordflow", nil},
		{"process_builder_review_prior", "priorformula", []string{"process_builder_review_record"}},
		{"process_builder_review_format", "formatformula", nil},
		{"process_builder_review_wrapped", "userwrapped", []string{"process_builder_review_mixed"}},
		{"process_builder_review_lookup", "lookupflow", nil},
		{"process_builder_review_phone", "phoneflow", []string{"process_builder_review_record"}},
		{"process_builder_review_personprior", "personprior", []string{"process_builder_review_record", "process_builder_review_prior"}},
		{"process_builder_review_afteraccept", "fourcomplex", []string{"process_builder_review_aftercomplex"}},
		{"process_builder_review_afteraccept", "fourcomplexalt", []string{"process_builder_review_aftercomplex"}},
		{"process_builder_review_afteruser", "fourcomplex", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept"}},
		{"process_builder_review_afteruser", "fourcomplexalt", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept"}},
		{"process_builder_review_afterbulk", "alertbulk", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser"}},
		{"process_builder_review_afterbulk", "alertreverse", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser"}},
		{"process_builder_review_afterbulk", "alertpartial", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser"}},
		{"process_builder_review_afterprior", "priorflow", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser"}},
		{"process_builder_review_afterprior2", "priorflow", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior"}},
		{"process_builder_review_afterprior2", "priorflowinsert", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior"}},
		{"process_builder_review_aftercase", "valid", nil},
		{"process_builder_review_aftercase", "nullperson", nil},
		{"process_builder_review_aftercaseerrors", "triple", []string{"process_builder_review_aftercase"}},
		{"process_builder_review_aftercasenogroup", "valid", []string{"process_builder_review_aftercase"}},
		{"process_builder_review_aftercaseformula", "owner", []string{"process_builder_review_aftercase"}},
		{"process_builder_review_aftercaseformula", "changed", []string{"process_builder_review_aftercase"}},
		{"process_builder_review_aftercaseformula", "nullperson", []string{"process_builder_review_aftercase"}},
		{"process_builder_review_aftercasecalc", "owner", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula"}},
		{"process_builder_review_aftercasecalc", "changed", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula"}},
		{"process_builder_review_aftercasecalc", "nullperson", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula"}},
		{"process_builder_review_afternullqueue", "noqueue", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula"}},
		{"process_builder_review_afternullqueue", "nonmatch", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula"}},
		{"process_builder_review_afterqueueerror", "noqueue", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula", "process_builder_review_afternullqueue"}},
		{"process_builder_review_afterqueueerror", "nonmatch", []string{"process_builder_review_aftercase", "process_builder_review_aftercaseformula", "process_builder_review_afternullqueue"}},
		{"process_builder_review_afterperson", "branchprobe", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch"}},
		{"process_builder_review_afterperson", "branchreject", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch"}},
		{"process_builder_review_aftersharebulk", "sharepair", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"}},
		{"process_builder_review_afterreplace", "replacement", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"}},
		{"process_builder_review_afterreplace", "branchreject", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"}},
		{"process_builder_review_afterreplace", "sharepair", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"}},
		{"process_builder_review_afterdatefirst", "branchdates", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson", "process_builder_review_afterreplace"}},
		{"process_builder_review_afterdatefirst", "branchreject", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson", "process_builder_review_afterreplace"}},
		{"process_builder_review_afterdatefirst", "sharepair", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson", "process_builder_review_afterreplace"}},
		{"process_builder_review_afterarg", "priorflowinsert", []string{"process_builder_review_aftercomplex", "process_builder_review_afteraccept", "process_builder_review_afteruser", "process_builder_review_afterprior", "process_builder_review_afterprior2", "process_builder_review_afterbranch", "process_builder_review_afterperson"}},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.fixture+"_"+scenario.method, func(t *testing.T) {
			base := filepath.Join("..", "apextest", "testdata", "conformance")
			fixture := filepath.Join(base, scenario.fixture)
			rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			native := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
				f := strings.Split(line, "\t")
				if len(f) != 4 {
					t.Fatalf("bad row: %q", line)
				}
				if f[1] == "anonymous" && f[2] == scenario.method {
					native[f[0]] = f[3]
				}
			}
			for _, api := range []string{"62.0", "67.0"} {
				for _, route := range []string{"vf", "lwc", "server"} {
					if api == "62.0" && route == "server" {
						continue
					} // Tooling endpoint starts at API 65.
					t.Run(api+"/"+route, func(t *testing.T) {
						root := t.TempDir()
						if err := os.CopyFS(root, os.DirFS(filepath.Join(base, "process_builder_shapes", "project"))); err != nil {
							t.Fatal(err)
						}
						layers := append([]string{"process_builder_review"}, scenario.parents...)
						layers = append(layers, scenario.fixture)
						for _, layer := range layers {
							overlay := filepath.Join(base, layer, "project")
							if err := filepath.WalkDir(overlay, func(path string, e fs.DirEntry, err error) error {
								if err != nil {
									return err
								}
								if e.IsDir() {
									return nil
								}
								rel, err := filepath.Rel(overlay, path)
								if err != nil {
									return err
								}
								data, err := os.ReadFile(path)
								if err != nil {
									return err
								}
								if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0755); err != nil {
									return err
								}
								return os.WriteFile(filepath.Join(root, rel), data, 0644)
							}); err != nil {
								t.Fatal(err)
							}
						}
						class := "FernCases" + strings.TrimSuffix(api, ".0")
						if strings.HasPrefix(scenario.fixture, "process_builder_review_aftercase") || scenario.fixture == "process_builder_review_afternullqueue" || scenario.fixture == "process_builder_review_afterqueueerror" {
							class = "FernCaseCases" + strings.TrimSuffix(api, ".0")
							writeServerTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Fern_Access, Fern_CaseAccess]\n")
						}
						writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/PBHost.cls"), fmt.Sprintf("public class PBHost { @AuraEnabled @RemoteAction public static String observe() { return %s.%s(); } }", class, scenario.method))
						writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/PBHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="PBHost"></apex:page>`)
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
						index := typesys.Build(p, schema)
						org, err := apextest.OrgFromIndexWithConfiguredPermissions(index)
						if err != nil {
							t.Fatal(err)
						}
						if scenario.fixture == "process_builder_review_afterperson" || scenario.fixture == "process_builder_review_aftersharebulk" || scenario.fixture == "process_builder_review_afterarg" || scenario.fixture == "process_builder_review_afterreplace" || scenario.fixture == "process_builder_review_afterdatefirst" {
							users := org.Objects["User"]
							syntheticID := storage.ID("005000000000099AAA")
							users.Records[syntheticID] = storage.Record{ID: syntheticID, Object: "User", Fields: map[string]storage.Value{"Alias": storage.StringValue("pbfake"), "Username": storage.StringValue("synthetic.runner@example.invalid"), "IsActive": storage.BooleanValue(true)}}
							if scenario.fixture == "process_builder_review_afterreplace" || scenario.fixture == "process_builder_review_afterdatefirst" {
								secondID := storage.ID("005000000000098AAA")
								users.Records[secondID] = storage.Record{ID: secondID, Object: "User", Fields: map[string]storage.Value{"Alias": storage.StringValue("pbthird"), "Username": storage.StringValue("synthetic.third@example.invalid"), "IsActive": storage.BooleanValue(true)}}
							}
							org.Objects["User"] = users
						}
						srv := newLightningTestServer(t, &org, source)
						configureVisualforceTestPrincipal(t, srv)
						srv.SetProjectIndex(index)
						if srv.runtimeErr != nil {
							t.Fatal(srv.runtimeErr)
						}
						want := native[api]
						if want == "" {
							t.Fatal("missing native row")
						}
						// Native assigns a fresh error ID on each anonymous execution.
						// Compare the captured message with only that ID removed.
						errorID := regexp.MustCompile(` Error ID: [0-9-]+(?: \(-?[0-9]+\))?`)
						if scenario.fixture == "process_builder_review_afterqueueerror" {
							want = errorID.ReplaceAllString(want, "")
						}
						switch route {
						case "vf":
							response := postVisualforceRemotingWithViewState(t, srv, `[{"action":"PBHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
							if len(response) == 1 && scenario.fixture == "process_builder_review_afterqueueerror" {
								if result, ok := response[0].Result.(string); ok {
									response[0].Result = errorID.ReplaceAllString(result, "")
								}
							}
							if len(response) != 1 || !response[0].Status || response[0].Result != want {
								t.Fatalf("remoting got %+v; want %q", response, want)
							}
						case "lwc":
							rec := httptest.NewRecorder()
							req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"PBHost","method":"observe","params":{}}`))
							req.Header.Set("Content-Type", "application/json")
							srv.ServeHTTP(rec, req)
							var out struct {
								Data  any `json:"data"`
								Error any `json:"error"`
							}
							err := json.Unmarshal(rec.Body.Bytes(), &out)
							if scenario.fixture == "process_builder_review_afterqueueerror" {
								if result, ok := out.Data.(string); ok {
									out.Data = errorID.ReplaceAllString(result, "")
								}
							}
							if err != nil || rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
								t.Fatalf("wire status=%d body=%s want=%q err=%v", rec.Code, rec.Body.String(), want, err)
							}
						case "server":
							call := class + "." + scenario.method + "()"
							if scenario.fixture == "process_builder_review_afterqueueerror" {
								call += ".replaceAll(' Error ID: [0-9-]+', '')"
							}
							q := url.Values{"anonymousBody": {"System.assertEquals('" + strings.ReplaceAll(want, "'", "\\'") + "', " + call + ");"}}
							rec := httptest.NewRecorder()
							srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v"+api+"/tooling/executeAnonymous?"+q.Encode(), nil))
							var out struct {
								Compiled         bool   `json:"compiled"`
								Success          bool   `json:"success"`
								ExceptionMessage string `json:"exceptionMessage"`
								CompileProblem   string `json:"compileProblem"`
							}
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusOK || !out.Compiled || !out.Success {
								t.Fatalf("tooling status=%d result=%+v err=%v", rec.Code, out, err)
							}
						}
					})
				}
			}

		})
	}
}

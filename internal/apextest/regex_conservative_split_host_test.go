package apextest_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// These host adapters replay the same observe call and nine results captured
// through anonymous and @IsTest at API 62/67 in org.tsv (see ORG-RUN.txt).
// They check the local host boundary, without claiming a native browser capture.
func TestConservativeSplitNativeVisualforceAndLWCHostRoutes(t *testing.T) {
	fixture := filepath.Join("testdata", "regex_conservative_split")
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma, r.LazyQuotes = '\t', true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := map[string][2]string{}
	for _, row := range read("cases.tsv") {
		var input [2]string
		for i := range input {
			if err := json.Unmarshal([]byte(row[i+1]), &input[i]); err != nil {
				t.Fatal(err)
			}
		}
		cases[row[0]] = input
	}
	rows := read("org.tsv")
	if len(rows) != 52 || len(cases) != 13 {
		t.Fatalf("oracle rows=%d cases=%d", len(rows), len(cases))
	}
	write := func(root, name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			apextest.InvalidateRuntimeCaches()
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			write(root, "sfdx-project.json", fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, name := range []string{"ConservativeSplitProbe", "ConservativeSplitProbeTest", "SplitHostController"} {
				write(root, "force-app/main/default/classes/"+name+".cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			}
			write(root, "force-app/main/default/classes/SplitHostController.cls", `public class SplitHostController {
 @AuraEnabled(cacheable=true) public static String observe(String source, String separator) {
  return JSON.serialize(ConservativeSplitProbe.observe(source, separator));
 }
 public String getObservation() {
  Map<String,String> params=ApexPages.currentPage().getParameters();
  return observe(params.get('source'), params.get('separator'));
 }
}`)
			write(root, "force-app/main/default/pages/SplitHost.page", `<apex:page controller="SplitHostController" showHeader="false" sidebar="false"><pre id="split-observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write(root, "force-app/main/default/pages/SplitHost.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Split host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			const userID storage.ID = "005000000000091"
			storage.EnsureStandardObject(&org, "User")
			users := org.Objects["User"]
			users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
			org.Objects["User"] = users
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = userID
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, row := range rows {
				if row[0] != api {
					continue
				}
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					input := cases[row[2]]
					var want map[string]any
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if row[2] == "octal_class_400" || row[2] == "octal_class_777" {
						// Same carried local syntax gap as the named/anonymous test.
						for key := range want {
							if key == "Matcher.find" {
								want[key] = false
							} else {
								want[key] = []any{"L" + input[1] + "R" + input[1] + "T"}
							}
						}
					}
					params := map[string]string{"source": input[0], "separator": input[1]}
					for _, route := range []string{"Visualforce", "LWC/imperative", "LWC/wire"} {
						t.Run(route, func(t *testing.T) {
							path, body := "/apex/SplitHost?"+url.Values{"source": {input[0]}, "separator": {input[1]}}.Encode(), []byte(nil)
							method := http.MethodGet
							if route != "Visualforce" {
								method, path = http.MethodPost, "/lightning/apex/SplitHostController/observe"
								body, err = json.Marshal(params)
								if route == "LWC/wire" {
									path = "/lightning/wire/apex"
									body, err = json.Marshal(map[string]any{"className": "SplitHostController", "method": "observe", "params": params})
								}
								if err != nil {
									t.Fatal(err)
								}
							}
							rec := httptest.NewRecorder()
							req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
							req.Header.Set("Content-Type", "application/json")
							handler.ServeHTTP(rec, req)
							if rec.Code != http.StatusOK {
								t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
							}
							var observed string
							if route == "Visualforce" {
								_, after, ok := strings.Cut(rec.Body.String(), `<pre id="split-observation">`)
								value, _, closed := strings.Cut(after, "</pre>")
								if !ok || !closed {
									t.Fatalf("missing rendered observation: %s", rec.Body.String())
								}
								observed = html.UnescapeString(value)
							} else {
								var response lwcbrowser.WireResponse
								if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
									t.Fatal(err)
								}
								if response.Error != nil {
									t.Fatalf("bridge error: %+v", response.Error)
								}
								var ok bool
								observed, ok = response.Data.(string)
								if !ok {
									t.Fatalf("bridge observation=%#v", response.Data)
								}
							}
							var got map[string]any
							if err := json.Unmarshal([]byte(observed), &got); err != nil {
								t.Fatalf("observation %q: %v", observed, err)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("host=%v want %s oracle=%v", got, row[1], want)
							}
						})
					}
				})
			}
		})
	}
}

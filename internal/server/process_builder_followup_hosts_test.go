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
	"strings"
	"testing"
)

// The native shared-target rows are process_builder_followup/org.tsv:6,14.
// These hosts transport the same Apex project method and record-triggered process.
func TestProcessBuilderSharedTargetHosts(t *testing.T) {
	base := filepath.Join("..", "apextest", "testdata", "conformance")
	fixture := filepath.Join(base, "process_builder_followup")
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
		if f[1] == "anonymous" && f[2] == "aba" {
			native[f[0]] = f[3]
		}
	}
	pbHostTempEnv(t)
	hosts := newPBHostFixtures(t)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"vf", "lwc", "server"} {
			if api == "62.0" && route == "server" {
				continue
			} // Tooling endpoint starts at API 65.
			t.Run(api+"/"+route, func(t *testing.T) {
				class := "FernCases" + strings.TrimSuffix(api, ".0")
				srv := hosts.server(t, api, func(root string) {
					if err := os.CopyFS(root, os.DirFS(filepath.Join(base, "process_builder_shapes", "project"))); err != nil {
						t.Fatal(err)
					}
					overlay := filepath.Join(fixture, "project")
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
						return os.WriteFile(filepath.Join(root, rel), data, 0644)
					}); err != nil {
						t.Fatal(err)
					}
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/PBHost.cls"), fmt.Sprintf("public class PBHost { @AuraEnabled @RemoteAction public static String observe() { return %s.aba(); } }", class))
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/PBHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="PBHost"></apex:page>`)
				}, nil)
				want := native[api]
				if want == "" {
					t.Fatal("missing native row")
				}
				switch route {
				case "vf":
					response := postVisualforceRemotingWithViewState(t, srv, `[{"action":"PBHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
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
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
						t.Fatalf("wire status=%d body=%s want=%q err=%v", rec.Code, rec.Body.String(), want, err)
					}
				case "server":
					q := url.Values{"anonymousBody": {"System.assertEquals('" + strings.ReplaceAll(want, "'", "\\'") + "', " + class + ".aba());"}}
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
}

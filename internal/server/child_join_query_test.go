package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The server route reuses the captured query; the HTTP response is a local contract.
func TestChildJoinServerRoute(t *testing.T) {
	fixture := "../soql/testdata/child_join_conjunction"
	source, err := os.ReadFile(filepath.Join(fixture, "combined.apex"))
	if err != nil {
		t.Fatal(err)
	}
	query, err := os.ReadFile(filepath.Join(fixture, "query.soql"))
	if err != nil {
		t.Fatal(err)
	}
	seed, _, ok := strings.Cut(string(source), "String result;")
	if !ok {
		t.Fatal("capture seed not found")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "force-app"), 0755); err != nil {
				t.Fatal(err)
			}
			config := fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)
			if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(config), 0644); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			machine := vm.New(nil)
			machine.SetOrg(&org)
			program, err := vm.CompileAnonymousWithOptions(seed, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			h := server.New(&org)
			restAPI, versionErr := storage.ResolveRESTAPIVersion(api)
			if versionErr != nil {
				// The server only accepts the REST versions in storage.SupportedRESTAPIVersions.
				unsupported := httptest.NewRecorder()
				h.ServeHTTP(unsupported, httptest.NewRequest(http.MethodGet, "/services/data/v"+api+"/query?q="+url.QueryEscape(strings.TrimSpace(string(query))), nil))
				if unsupported.Code != http.StatusNotFound {
					t.Fatalf("unsupported REST version: %d", unsupported.Code)
				}
				restAPI = storage.DefaultRESTAPIVersion
			}
			req := httptest.NewRequest(http.MethodGet, "/services/data/v"+restAPI+"/query?q="+url.QueryEscape(strings.TrimSpace(string(query))), nil)
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("query status %d: %s", resp.Code, resp.Body.String())
			}
			var got struct {
				TotalSize int `json:"totalSize"`
				Records   []struct {
					Contacts struct {
						TotalSize int `json:"totalSize"`
					} `json:"Contacts"`
				} `json:"records"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.TotalSize != 1 || len(got.Records) != 1 || got.Records[0].Contacts.TotalSize != 1 {
				t.Fatalf("unexpected result: %s", resp.Body.String())
			}
		})
	}
}

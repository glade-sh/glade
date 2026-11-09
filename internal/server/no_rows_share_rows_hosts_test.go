package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Host reachability twins of no_rows_share_rows/org.tsv:2-3.
func TestNoRowsShareRowsHostRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/no_rows_share_rows/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		api := row[0]
		var native struct {
			FirstTarget  int `json:"firstTarget"`
			SecondTarget int `json:"secondTarget"`
		}
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatal(err)
		}
		want := native.FirstTarget + native.SecondTarget
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceDir := "../apextest/testdata/conformance/no_rows_visibility/project"
			err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(sourceDir, path)
				if err != nil {
					return err
				}
				writeServerTestFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="ShareHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ShareHost.cls"), `public without sharing class ShareHost { @AuraEnabled @RemoteAction public static Integer ruleCount() { return [SELECT COUNT() FROM PrivateProbe__Share WHERE RowCause='Rule']; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ShareHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			registry := sobject.BuildDescribeRegistry(schema)
			for name, describe := range registry.Objects {
				org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe), Records: map[storage.ID]storage.Record{}}
			}
			storage.EnsureDeterministicPlatformData(&org)
			if err := resource.ApplyProjectGroups(&org, p); err != nil {
				t.Fatal(err)
			}
			if err := resource.ApplyProjectOwnerSharingRules(&org, p); err != nil {
				t.Fatal(err)
			}
			groups := map[string]storage.ID{}
			for id, g := range org.Objects["Group"].Records {
				groups[g.Fields["DeveloperName"].String] = id
			}
			storage.EnsureStandardObject(&org, "GroupMember")
			state := org.Objects["GroupMember"]
			state.Records = map[storage.ID]storage.Record{}
			owners := []storage.ID{"005000000000701", "005000000000702"}
			for i, origin := range []string{"ProbeOrigin", "ProbeOriginTwo"} {
				id := storage.ID(fmt.Sprintf("01100000000070%d", i+1))
				state.Records[id] = storage.Record{ID: id, Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue(groups[origin]), "UserOrGroupId": storage.IDValue(owners[i])}}
			}
			org.Objects["GroupMember"] = state
			engine := dml.NewEngine(&org)
			for _, owner := range owners {
				engine.UserID = owner
				result := engine.Insert([]storage.Record{{Object: "PrivateProbe__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Host row")}}})
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("insert: %+v", result)
				}
			}
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"ShareHost","method":"ruleCount","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || fmt.Sprint(responses[0].Result) != fmt.Sprint(want) {
					t.Fatalf("remoting: %+v want=%d", responses, want)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"ShareHost","method":"ruleCount","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != nil || fmt.Sprint(body.Data) != fmt.Sprint(want) {
					t.Fatalf("wire: %s want=%d", rec.Body.String(), want)
				}
			})
		})
	}
}

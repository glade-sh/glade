package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Supplemental host twins of no_rows_subscriber/org.tsv:2-5.
func TestNoRowsSubscriberHostRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/no_rows_subscriber/org.tsv")
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
		if row[1] != "anonymous" {
			continue
		}
		api := row[0]
		var observation struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(row[2]), &observation); err != nil {
			t.Fatal(err)
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls"), `public class AjaxController { @AuraEnabled @RemoteAction public static String subscriber() { return [SELECT Name FROM EventBusSubscriber WHERE Topic='SyntheticEvent__e' AND Type='ApexTrigger' LIMIT 1].Name; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			storage.EnsureStandardObject(&org, "EventBusSubscriber")
			state := org.Objects["EventBusSubscriber"]
			if state.Records == nil {
				state.Records = map[storage.ID]storage.Record{}
			}
			id := storage.ID("0Cd000000000001")
			state.Records[id] = storage.Record{ID: id, Object: "EventBusSubscriber", MetadataSourceUnit: "glade:event-subscriber:01q000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("SyntheticEventTrigger"), "Topic": storage.StringValue("SyntheticEvent__e"), "Type": storage.StringValue("ApexTrigger")}}
			org.Objects["EventBusSubscriber"] = state
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"subscriber","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != observation.Name {
					t.Fatalf("remoting: %+v", responses)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"AjaxController","method":"subscriber","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != nil || body.Data != observation.Name {
					t.Fatalf("wire: %s", rec.Body.String())
				}
			})
		})
	}
}

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

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Host twins of no_rows_visibility/org.tsv:3,7,10,14,18-21 and no_rows_same_group/org.tsv:2-3.
func TestNoRowsVisibilityHostRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "no_rows_visibility")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	sameFixture := filepath.Join("..", "apextest", "testdata", "conformance", "no_rows_same_group")
	sameFile, err := os.Open(filepath.Join(sameFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer sameFile.Close()
	sameReader := csv.NewReader(sameFile)
	sameReader.Comma, sameReader.LazyQuotes = '\t', true
	sameRows, err := sameReader.ReadAll()
	if err != nil || len(sameRows) != 3 {
		t.Fatalf("invalid same-group observations: %v", err)
	}
	rows = append(rows, sameRows[1:]...)
	for _, row := range rows[1:] {
		if row[1] != "hierarchy" && row[1] != "sharingRule" && row[1] != "controlledDetail" && row[1] != "controlledHierarchy" && row[1] != "sameGroup" {
			continue
		}
		api, kind := row[0], row[1]
		var observed map[string]any
		if err := json.Unmarshal([]byte(row[2]), &observed); err != nil {
			t.Fatal(err)
		}
		t.Run(api+"/"+kind, func(t *testing.T) {
			root := t.TempDir()
			sourceDir := filepath.Join(fixture, "project")
			err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(sourceDir, path)
				if err != nil {
					return err
				}
				writeServerTestFile(t, filepath.Join(root, rel), string(data))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if kind == "sameGroup" {
				rel := filepath.Join("force-app", "main", "default", "sharingRules", "PrivateProbe__c.sharingRules-meta.xml")
				body, err := os.ReadFile(filepath.Join(sameFixture, "project-overrides", rel))
				if err != nil {
					t.Fatal(err)
				}
				writeServerTestFile(t, filepath.Join(root, rel), string(body))
			}
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="VisibilityHost"></apex:page>`)
			queryObject := "PrivateProbe__c"
			if kind == "controlledDetail" || kind == "controlledHierarchy" {
				queryObject = "DetailProbe__c"
			}
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/VisibilityHost.cls"), `public with sharing class VisibilityHost { @AuraEnabled @RemoteAction public static Integer countRows() { return [SELECT COUNT() FROM `+queryObject+` WHERE Name='Host Row']; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/VisibilityHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			if err := resource.ApplyProjectRoles(&org, p); err != nil {
				t.Fatal(err)
			}
			if err := resource.ApplyProjectGroups(&org, p); err != nil {
				t.Fatal(err)
			}
			if kind == "sharingRule" || kind == "controlledDetail" || kind == "sameGroup" {
				if err := resource.ApplyProjectOwnerSharingRules(&org, p); err != nil {
					t.Fatal(err)
				}
			} else {
				resource.ApplyHierarchyAccessObjects(&org, []string{"PrivateProbe__c"})
			}
			var parentRole, childRole, origin, target storage.ID
			for id, record := range org.Objects["UserRole"].Records {
				switch record.Fields["DeveloperName"].String {
				case "ProbeParent":
					parentRole = id
				case "ProbeChild":
					childRole = id
				}
			}
			for id, record := range org.Objects["Group"].Records {
				switch record.Fields["DeveloperName"].String {
				case "ProbeOrigin":
					origin = id
				case "ProbeTarget":
					target = id
				}
			}
			if parentRole == "" || childRole == "" || origin == "" || target == "" {
				t.Fatal("missing synthetic roles or groups")
			}
			if kind == "sameGroup" {
				target = origin
			}
			const owner storage.ID = "005000000000701"
			const viewer storage.ID = "005000000000702"
			const other storage.ID = "005000000000703"
			addUser(&org, owner, "owner@example.invalid", "owner@example.invalid", "Owner")
			addUser(&org, viewer, "viewer@example.invalid", "viewer@example.invalid", "Viewer")
			addUser(&org, other, "other@example.invalid", "other@example.invalid", "Other")
			users := org.Objects["User"]
			u := users.Records[owner]
			u.Fields["UserRoleId"] = storage.IDValue(childRole)
			users.Records[owner] = u
			u = users.Records[viewer]
			u.Fields["UserRoleId"] = storage.IDValue(parentRole)
			users.Records[viewer] = u
			org.Objects["User"] = users
			storage.EnsureStandardObject(&org, "GroupMember")
			members := org.Objects["GroupMember"]
			members.Records = map[storage.ID]storage.Record{"011000000000701": {ID: "011000000000701", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue(origin), "UserOrGroupId": storage.IDValue(owner)}}, "011000000000702": {ID: "011000000000702", Object: "GroupMember", Fields: map[string]storage.Value{"GroupId": storage.IDValue(target), "UserOrGroupId": storage.IDValue(viewer)}}}
			org.Objects["GroupMember"] = members
			object := org.Objects["PrivateProbe__c"]
			id := storage.ID("a00000000000701")
			object.Records[id] = storage.Record{ID: id, Object: "PrivateProbe__c", System: storage.SystemFields{OwnerID: owner}, Fields: map[string]storage.Value{"Name": storage.StringValue("Host Row")}}
			org.Objects["PrivateProbe__c"] = object
			detail := org.Objects["DetailProbe__c"]
			detailID := storage.ID("a01000000000701")
			detail.Records[detailID] = storage.Record{ID: detailID, Object: "DetailProbe__c", System: storage.SystemFields{OwnerID: owner}, Fields: map[string]storage.Value{"Name": storage.StringValue("Host Row"), "Parent__c": storage.IDValue(id)}}
			org.Objects["DetailProbe__c"] = detail
			srv := newLightningTestServer(t, &org, source)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			grantedKey, deniedKey := "parent", "unrelated"
			switch kind {
			case "sharingRule", "sameGroup":
				grantedKey, deniedKey = "recipient", "other"
			case "controlledDetail":
				grantedKey, deniedKey = "after", "other"
			case "controlledHierarchy":
				grantedKey = "ancestor"
			}
			for _, principal := range []struct {
				name string
				id   storage.ID
				want int
			}{{"granted", viewer, int(observed[grantedKey].(float64))}, {"other", other, int(observed[deniedKey].(float64))}} {
				t.Run(principal.name, func(t *testing.T) {
					t.Run("vf", func(t *testing.T) {
						srv.VisualforceHTMLUserID = principal.id
						responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"VisibilityHost","method":"countRows","data":[],"type":"rpc","tid":3}]`)
						if len(responses) != 1 || !responses[0].Status || fmt.Sprint(responses[0].Result) != fmt.Sprint(principal.want) {
							t.Fatalf("remoting: %+v want=%d", responses, principal.want)
						}
					})
					t.Run("lwc", func(t *testing.T) {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"VisibilityHost","method":"countRows","params":{}}`))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-GLADE-User-Id", string(principal.id))
						srv.ServeHTTP(rec, req)
						var body struct {
							Data  any `json:"data"`
							Error any `json:"error"`
						}
						if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != nil || fmt.Sprint(body.Data) != fmt.Sprint(principal.want) {
							t.Fatalf("wire: %s want=%d", rec.Body.String(), principal.want)
						}
					})
				})
			}
		})
	}
}

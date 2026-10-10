package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

const (
	vfAccessOwnerID  storage.ID = "005000000000011"
	vfAccessReaderID storage.ID = "005000000000012"
	vfAccessNoObject storage.ID = "005000000000013"
	vfAccessNoName   storage.ID = "005000000000014"

	vfAccessOwnerProfile  storage.ID = "00e000000000011"
	vfAccessReaderProfile storage.ID = "00e000000000012"
	vfAccessNoObjectProf  storage.ID = "00e000000000013"
	vfAccessNoNameProfile storage.ID = "00e000000000014"

	vfAccessOwnerRecord storage.ID = "001000000000011"
	vfAccessPrivateRow  storage.ID = "001000000000012"
	vfAccessSharedRow   storage.ID = "001000000000013"
	vfAccessObjectRow   storage.ID = "001000000000014"
	vfAccessNoNameRow   storage.ID = "001000000000015"
	vfAccessUnknownUser storage.ID = "005000000000099"
)

func TestVisualforceHTMLStandardControllerRecordAuthorizationAPI67(t *testing.T) {
	t.Run("owner can read the StandardController record", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessOwnerID)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, "")
		assertVisualforceHTMLRecordRendered(t, rec, "VF_OWNER_NAME_CANARY")
	})

	t.Run("different owner cannot read the StandardController record", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessReaderID)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, "")
		assertVisualforceHTMLRecordNotRendered(t, rec, vfAccessOwnerRecord, "VF_OWNER_NAME_CANARY")
	})

	t.Run("explicit AccountShare grants read access", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessReaderID)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessSharedRow, "")
		assertVisualforceHTMLRecordRendered(t, rec, "VF_SHARED_NAME_CANARY")
	})

	t.Run("missing object read permission denies the record", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessNoObject)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessObjectRow, "")
		assertVisualforceHTMLRecordNotRendered(t, rec, vfAccessObjectRow, "VF_OBJECT_NAME_CANARY")
	})

	t.Run("missing Name field read permission hides Name", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessNoName)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessNoNameRow, "")
		if strings.Contains(rec.Body.String(), "VF_NO_NAME_CANARY") {
			t.Fatalf("HTML exposed Account.Name without field read permission: %s", rec.Body.String())
		}
	})

	t.Run("missing configured principal fails closed despite header", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, "")
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, string(vfAccessOwnerID))
		assertVisualforceHTMLPrincipalRejected(t, rec, vfAccessOwnerRecord, "VF_OWNER_NAME_CANARY")
	})

	t.Run("unknown configured principal fails closed despite header", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessUnknownUser)
		rec := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, string(vfAccessOwnerID))
		assertVisualforceHTMLPrincipalRejected(t, rec, vfAccessOwnerRecord, "VF_OWNER_NAME_CANARY")
	})

	t.Run("sequential requests cannot switch principal or retain record data", func(t *testing.T) {
		srv := newVisualforceHTMLRecordAccessServer(t, vfAccessReaderID)
		shared := requestVisualforceHTMLRecord(t, srv, vfAccessSharedRow, string(vfAccessOwnerID))
		assertVisualforceHTMLRecordRendered(t, shared, "VF_SHARED_NAME_CANARY")

		private := requestVisualforceHTMLRecord(t, srv, vfAccessPrivateRow, string(vfAccessOwnerID))
		assertVisualforceHTMLRecordNotRendered(t, private, vfAccessPrivateRow, "VF_PRIVATE_NAME_CANARY")
		if strings.Contains(private.Body.String(), "VF_SHARED_NAME_CANARY") {
			t.Fatalf("HTML retained the prior request's record value: %s", private.Body.String())
		}
	})
}

func TestVisualforceHTMLRejectsConflictingStoredUserIDAPI67(t *testing.T) {
	for name, mutate := range map[string]func(*storage.Record){
		"direct field":  func(user *storage.Record) { user.Fields["Id"] = storage.IDValue(vfAccessUnknownUser) },
		"dotted field":  func(user *storage.Record) { user.Fields["Id.value"] = storage.StringValue(string(vfAccessUnknownUser)) },
		"null metadata": func(user *storage.Record) { user.ExplicitNulls = map[string]bool{"Id": true} },
	} {
		t.Run(name, func(t *testing.T) {
			srv := newVisualforceHTMLRecordAccessServer(t, vfAccessOwnerID)
			users := srv.Org.Objects["User"]
			stored := users.Records[vfAccessOwnerID]
			mutate(&stored)
			users.Records[vfAccessOwnerID] = stored
			srv.Org.Objects["User"] = users
			if user, err := srv.visualforceHTMLExecutionUser(); err == nil {
				t.Fatalf("accepted conflicting Org.User identity: %#v", user)
			}
		})
	}
}

func TestVisualforceRemotingUsesConfiguredStoredPrincipalAPI67(t *testing.T) {
	t.Setenv("GLADE_VISUALFORCE_HTML_USER_ID", string(vfAccessOwnerID))
	srv := newVisualforceFixtureServer(t, "Remote.page", `<apex:page controller="AjaxController"></apex:page>`, `public class AjaxController {
  @RemoteAction
  public static String executionUserId() {
    return UserInfo.getUserId();
  }
}`)
	addUser(srv.Org, vfAccessOwnerID, "vf-owner@example.test", "vf-owner@example.test", "VF Owner")
	addUser(srv.Org, vfAccessReaderID, "vf-reader@example.test", "vf-reader@example.test", "VF Reader")

	body := remotingEnvelopeWithViewState(t, srv, `[{"action":"AjaxController","method":"executionUserId","data":[],"type":"rpc","tid":11}]`)
	req := httptest.NewRequest(http.MethodPost, "/apex/Remote/remoting", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GLADE-User-Id", string(vfAccessReaderID))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var responses []visualforce.RemotingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("remoting response json: %v body=%s", err, rec.Body.String())
	}
	// Apex String coercion expands this all-uppercase 15-character fixture ID.
	if len(responses) != 1 || !responses[0].Status || responses[0].Result != string(vfAccessOwnerID)+"AAA" {
		t.Fatalf("response = %#v; wanted configured principal %q despite request header %q", responses, vfAccessOwnerID, vfAccessReaderID)
	}
}

func TestVisualforceHTMLMissingPrincipalRejectedBeforeExpressionDiagnosticAPI67(t *testing.T) {
	srv := newVisualforceHTMLRecordAccessServer(t, "")
	pagePath := filepath.Join(srv.Source.Project.Root, "force-app/main/default/pages/RecordAccess.page")
	writeServerTestFile(t, pagePath, `<apex:page standardController="Account"><div>{!1 + }</div></apex:page>`)

	rec := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, string(vfAccessOwnerID))
	if rec.Code < http.StatusBadRequest {
		t.Fatalf("missing principal returned status %d with body %q; wanted fail-closed error before diagnostic rendering", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Visualforce Error") || strings.Contains(rec.Body.String(), "VF_OWNER_NAME_CANARY") {
		t.Fatalf("missing principal reached diagnostic or record rendering: %s", rec.Body.String())
	}
}

func newVisualforceHTMLRecordAccessServer(t *testing.T, configuredPrincipal storage.ID) *Server {
	t.Helper()
	t.Setenv("GLADE_VISUALFORCE_HTML_USER_ID", string(configuredPrincipal))

	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	account.Records = map[storage.ID]storage.Record{
		vfAccessOwnerRecord: {
			ID: vfAccessOwnerRecord, Object: "Account",
			System: storage.SystemFields{OwnerID: vfAccessOwnerID},
			Fields: map[string]storage.Value{"Name": storage.StringValue("VF_OWNER_NAME_CANARY")},
		},
		vfAccessPrivateRow: {
			ID: vfAccessPrivateRow, Object: "Account",
			System: storage.SystemFields{OwnerID: vfAccessOwnerID},
			Fields: map[string]storage.Value{"Name": storage.StringValue("VF_PRIVATE_NAME_CANARY")},
		},
		vfAccessSharedRow: {
			ID: vfAccessSharedRow, Object: "Account",
			System: storage.SystemFields{OwnerID: vfAccessOwnerID},
			Fields: map[string]storage.Value{"Name": storage.StringValue("VF_SHARED_NAME_CANARY")},
		},
		vfAccessObjectRow: {
			ID: vfAccessObjectRow, Object: "Account",
			System: storage.SystemFields{OwnerID: vfAccessNoObject},
			Fields: map[string]storage.Value{"Name": storage.StringValue("VF_OBJECT_NAME_CANARY")},
		},
		vfAccessNoNameRow: {
			ID: vfAccessNoNameRow, Object: "Account",
			System: storage.SystemFields{OwnerID: vfAccessNoName},
			Fields: map[string]storage.Value{"Name": storage.StringValue("VF_NO_NAME_CANARY")},
		},
	}
	org.Objects["Account"] = account

	addVisualforceHTMLAccessUser(&org, vfAccessOwnerID, vfAccessOwnerProfile, "owner@example.test")
	addVisualforceHTMLAccessUser(&org, vfAccessReaderID, vfAccessReaderProfile, "reader@example.test")
	addVisualforceHTMLAccessUser(&org, vfAccessNoObject, vfAccessNoObjectProf, "no-object@example.test")
	addVisualforceHTMLAccessUser(&org, vfAccessNoName, vfAccessNoNameProfile, "no-name@example.test")

	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		vfAccessOwnerProfile:  visualforceHTMLTestProfile(vfAccessOwnerProfile, "VF Read Owner"),
		vfAccessReaderProfile: visualforceHTMLTestProfile(vfAccessReaderProfile, "VF Read User"),
		vfAccessNoObjectProf:  visualforceHTMLTestProfile(vfAccessNoObjectProf, "VF No Account Read"),
		vfAccessNoNameProfile: visualforceHTMLTestProfile(vfAccessNoNameProfile, "VF No Name Read"),
	}}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"110000000000011": visualforceHTMLObjectPermission("110000000000011", vfAccessOwnerProfile, true),
		"110000000000012": visualforceHTMLObjectPermission("110000000000012", vfAccessReaderProfile, true),
		"110000000000013": visualforceHTMLObjectPermission("110000000000013", vfAccessNoObjectProf, false),
		"110000000000014": visualforceHTMLObjectPermission("110000000000014", vfAccessNoNameProfile, true),
	}}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"120000000000011": visualforceHTMLNamePermission("120000000000011", vfAccessOwnerProfile, true),
		"120000000000012": visualforceHTMLNamePermission("120000000000012", vfAccessReaderProfile, true),
		"120000000000013": visualforceHTMLNamePermission("120000000000013", vfAccessNoObjectProf, true),
		"120000000000014": visualforceHTMLNamePermission("120000000000014", vfAccessNoNameProfile, false),
	}}
	shares := org.Objects["AccountShare"]
	shares.Records = map[storage.ID]storage.Record{
		"130000000000013": {
			ID: "130000000000013", Object: "AccountShare",
			Fields: map[string]storage.Value{
				"AccountId":          storage.IDValue(vfAccessSharedRow),
				"UserOrGroupId":      storage.IDValue(vfAccessReaderID),
				"AccountAccessLevel": storage.StringValue("Read"),
			},
		},
	}
	org.Objects["AccountShare"] = shares

	source := testSourceMetadata(t)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/RecordAccess.page"), `<apex:page standardController="Account"><apex:outputField value="{!Account.Name}"/></apex:page>`)
	writeServerTestFile(t, filepath.Join(source.Project.Root, "force-app/main/default/pages/RecordAccess.page-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Record Access</label></ApexPage>`)
	reloaded, err := project.Load(source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	source, err = NewSourceMetadataFromProject(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	index, err := visualforce.LoadProject(source.Project)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := index.Page("RecordAccess")
	if !ok || page.APIVersion != "67.0" || page.StandardController != "Account" {
		t.Fatalf("fixture page profile = found:%v API:%q controller:%q; want API 67.0 Account StandardController", ok, page.APIVersion, page.StandardController)
	}

	srv := newLightningTestServer(t, &org, source)
	return srv
}

func addVisualforceHTMLAccessUser(org *storage.OrgState, id, profileID storage.ID, username string) {
	addUser(org, id, username, username, "VF Access Test User")
	users := org.Objects["User"]
	user := users.Records[id]
	user.Fields["ProfileId"] = storage.IDValue(profileID)
	users.Records[id] = user
	org.Objects["User"] = users
}

func visualforceHTMLTestProfile(id storage.ID, name string) storage.Record {
	return storage.Record{ID: id, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
}

func visualforceHTMLObjectPermission(id string, profileID storage.ID, canRead bool) storage.Record {
	return storage.Record{ID: storage.ID(id), Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Account"),
		"PermissionsRead": storage.BooleanValue(canRead),
	}}
}

func visualforceHTMLNamePermission(id string, profileID storage.ID, canRead bool) storage.Record {
	return storage.Record{ID: storage.ID(id), Object: "FieldPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Account"),
		"Field": storage.StringValue("Account.Name"), "PermissionsRead": storage.BooleanValue(canRead),
	}}
}

func requestVisualforceHTMLRecord(t *testing.T, srv *Server, recordID storage.ID, requestUserID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/apex/RecordAccess?id="+string(recordID), nil)
	if requestUserID != "" {
		req.Header.Set("X-GLADE-User-Id", requestUserID)
	}
	srv.ServeHTTP(rec, req)
	return rec
}

func assertVisualforceHTMLRecordRendered(t *testing.T, rec *httptest.ResponseRecorder, name string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q; want rendered record", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), name) {
		t.Fatalf("HTML missing allowed Account.Name %q: %s", name, rec.Body.String())
	}
}

func assertVisualforceHTMLRecordNotRendered(t *testing.T, rec *httptest.ResponseRecorder, id storage.ID, name string) {
	t.Helper()
	body := rec.Body.String()
	if strings.Contains(body, string(id)) || strings.Contains(body, name) {
		t.Fatalf("HTML exposed denied record id or Account.Name: %s", body)
	}
}

func assertVisualforceHTMLPrincipalRejected(t *testing.T, rec *httptest.ResponseRecorder, id storage.ID, name string) {
	t.Helper()
	if rec.Code < http.StatusBadRequest {
		t.Fatalf("status = %d; missing/unknown process principal must fail closed", rec.Code)
	}
	assertVisualforceHTMLRecordNotRendered(t, rec, id, name)
}

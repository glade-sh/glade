package server

import (
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

// VF157.3: visualforce-contract:67.0:rendering-pdf-profiles:output-field:field-format.
// Retained pages_compref_outputField.md SHA256 06d1caa501b6b202fe2a78860c0ba93f7c7810968a0f05e502caf275ef398816.
// Local API67 initial HTML Account standard-controller GET with a readable
// OwnerId/User target. /record/User/{id} is a local destination, not a claim
// about Salesforce's lookup href or its full record-detail UI.
func TestVisualforceLookupRecordDestinationAPI67(t *testing.T) {
	const targetName = `VF_ROUTE_OWNER_&<probe>`
	path := "/record/User/" + string(vfAccessOwnerID)
	// vfAccessOwnerID contains only digits, so its valid 18-character checksum is AAA.
	alternateWidthID := storage.ID(string(vfAccessOwnerID) + "AAA")
	alternateWidthPath := "/record/User/" + string(alternateWidthID)

	t.Run("readable User target shows ID and escaped available Name", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		page := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, "")
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), html.EscapeString(targetName)) ||
			strings.Contains(page.Body.String(), targetName) {
			t.Fatalf("API67 lookup source profile failed: status=%d body=%q", page.Code, page.Body.String())
		}
		rec := requestVisualforceLookupRoute(srv, path, "")
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") ||
			!strings.Contains(rec.Body.String(), string(vfAccessOwnerID)) ||
			!strings.Contains(rec.Body.String(), html.EscapeString(targetName)) ||
			strings.Contains(rec.Body.String(), targetName) {
			t.Fatalf("authorized local destination status=%d contentType=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	})

	t.Run("valid 18-character URL resolves stored 15-character User", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		rec := requestVisualforceLookupRoute(srv, alternateWidthPath, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), string(vfAccessOwnerID)) ||
			!strings.Contains(rec.Body.String(), html.EscapeString(targetName)) ||
			strings.Contains(rec.Body.String(), targetName) {
			t.Fatalf("alternate-width destination status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed 18-character checksum does not resolve stored User", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		malformed := storage.ID(string(vfAccessOwnerID) + "AAB")
		rec := requestVisualforceLookupRoute(srv, "/record/User/"+string(malformed), "")
		assertVisualforceLookupRouteHidden(t, rec, vfAccessOwnerID, targetName)
	})

	t.Run("18-character checksum reflects uppercase source characters", func(t *testing.T) {
		if !validVisualforceLookupID("005Abc000000011IAA") || validVisualforceLookupID("005Abc000000011AAA") {
			t.Fatal("lookup ID checksum ignored uppercase source characters")
		}
	})

	t.Run("missing target is controlled and does not reveal record data", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		missing := storage.ID("005000000000098")
		rec := requestVisualforceLookupRoute(srv, "/record/User/"+string(missing), "")
		assertVisualforceLookupRouteHidden(t, rec, missing, targetName)
	})

	t.Run("unshared User target is indistinguishable from missing", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessReaderID, targetName)
		users := srv.Org.Objects["User"]
		users.Definition.SharingModel = "Private"
		srv.Org.Objects["User"] = users
		rec := requestVisualforceLookupRoute(srv, path, string(vfAccessOwnerID))
		assertVisualforceLookupRouteHidden(t, rec, vfAccessOwnerID, targetName)
	})

	t.Run("User object denial hides target", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		permissions := srv.Org.Objects["ObjectPermissions"]
		grant := permissions.Records["110000000000097"]
		grant.Fields["PermissionsRead"] = storage.BooleanValue(false)
		permissions.Records[grant.ID] = grant
		srv.Org.Objects["ObjectPermissions"] = permissions
		rec := requestVisualforceLookupRoute(srv, path, "")
		assertVisualforceLookupRouteHidden(t, rec, vfAccessOwnerID, targetName)
	})

	t.Run("User Name denial shows only authorized ID", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		permissions := srv.Org.Objects["FieldPermissions"]
		grant := permissions.Records["120000000000097"]
		grant.Fields["PermissionsRead"] = storage.BooleanValue(false)
		permissions.Records[grant.ID] = grant
		srv.Org.Objects["FieldPermissions"] = permissions
		rec := requestVisualforceLookupRoute(srv, path, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), string(vfAccessOwnerID)) ||
			strings.Contains(rec.Body.String(), targetName) || strings.Contains(rec.Body.String(), html.EscapeString(targetName)) {
			t.Fatalf("Name-denied destination status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("User Name denial on valid 18-character URL shows only authorized ID", func(t *testing.T) {
		srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
		permissions := srv.Org.Objects["FieldPermissions"]
		grant := permissions.Records["120000000000097"]
		grant.Fields["PermissionsRead"] = storage.BooleanValue(false)
		permissions.Records[grant.ID] = grant
		srv.Org.Objects["FieldPermissions"] = permissions
		rec := requestVisualforceLookupRoute(srv, alternateWidthPath, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), string(vfAccessOwnerID)) ||
			strings.Contains(rec.Body.String(), targetName) || strings.Contains(rec.Body.String(), html.EscapeString(targetName)) {
			t.Fatalf("Name-denied alternate-width destination status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	for _, route := range []string{
		"/record/Account/" + string(vfAccessOwnerID),
		"/record/User/not-an-id",
		path + "/extra",
	} {
		t.Run("malformed or other-object destination "+route, func(t *testing.T) {
			srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
			rec := requestVisualforceLookupRoute(srv, route, "")
			assertVisualforceLookupRouteHidden(t, rec, vfAccessOwnerID, targetName)
		})
	}

	for _, test := range []struct {
		name      string
		principal storage.ID
	}{
		{"missing configured principal", ""},
		{"unknown configured principal", vfAccessUnknownUser},
	} {
		t.Run(test.name+" ignores request-user header", func(t *testing.T) {
			srv := newVisualforceLookupRouteServer(t, test.principal, targetName)
			rec := requestVisualforceLookupRoute(srv, path, string(vfAccessOwnerID))
			if rec.Code < http.StatusBadRequest || strings.Contains(rec.Body.String(), string(vfAccessOwnerID)) ||
				strings.Contains(rec.Body.String(), targetName) || strings.Contains(rec.Body.String(), html.EscapeString(targetName)) {
				t.Fatalf("untrusted request identity reached destination: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestVisualforceLookupOutputFieldAnchorDestinationAPI67(t *testing.T) {
	const targetName = `VF_ROUTE_OWNER_&<probe>`
	srv := newVisualforceLookupRouteServer(t, vfAccessOwnerID, targetName)
	page := requestVisualforceHTMLRecord(t, srv, vfAccessOwnerRecord, "")
	pageHasEscapedName := strings.Contains(page.Body.String(), html.EscapeString(targetName))
	pageHasRawName := strings.Contains(page.Body.String(), targetName)
	if page.Code != http.StatusOK || !pageHasEscapedName || pageHasRawName {
		t.Fatalf("authorized outputField page status=%d escapedName=%t rawName=%t", page.Code, pageHasEscapedName, pageHasRawName)
	}
	href := visualforceLookupOutputAnchor(t, page.Body.String())
	if want := "/record/User/" + string(vfAccessOwnerID); href != want {
		t.Fatalf("outputField href=%q, want authorized local destination %q", href, want)
	}

	visible := requestVisualforceLookupRoute(srv, href, "")
	visibleHTML := strings.HasPrefix(visible.Header().Get("Content-Type"), "text/html")
	visibleID := strings.Contains(visible.Body.String(), string(vfAccessOwnerID))
	visibleEscapedName := strings.Contains(visible.Body.String(), html.EscapeString(targetName))
	visibleRawName := strings.Contains(visible.Body.String(), targetName)
	if visible.Code != http.StatusOK || !visibleHTML || !visibleID || !visibleEscapedName || visibleRawName {
		t.Fatalf("followed authorized lookup status=%d html=%t id=%t escapedName=%t rawName=%t",
			visible.Code, visibleHTML, visibleID, visibleEscapedName, visibleRawName)
	}

	deniedServer := newVisualforceLookupRouteServer(t, vfAccessReaderID, targetName)
	users := deniedServer.Org.Objects["User"]
	users.Definition.SharingModel = "Private"
	deniedServer.Org.Objects["User"] = users
	denied := requestVisualforceLookupRoute(deniedServer, href, string(vfAccessOwnerID))
	assertVisualforceLookupRouteHidden(t, denied, vfAccessOwnerID, targetName)
}

func visualforceLookupOutputAnchor(t *testing.T, pageHTML string) string {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		t.Fatal(err)
	}
	var hrefs []string
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "/record/User/") {
					hrefs = append(hrefs, attr.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if len(hrefs) != 1 {
		t.Fatalf("outputField local lookup anchors=%d, want one", len(hrefs))
	}
	return hrefs[0]
}

func newVisualforceLookupRouteServer(t *testing.T, principal storage.ID, targetName string) *Server {
	t.Helper()
	srv := newVisualforceHTMLRecordAccessServer(t, principal)
	account := srv.Org.Objects["Account"]
	account.Definition.Fields["OwnerId"] = storage.Field{
		APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner",
	}
	row := account.Records[vfAccessOwnerRecord]
	row.Fields["OwnerId"] = storage.IDValue(vfAccessOwnerID)
	account.Records[row.ID] = row
	srv.Org.Objects["Account"] = account

	users := srv.Org.Objects["User"]
	users.Definition.APIName = "User"
	users.Definition.KeyPrefix = "005"
	users.Definition.SharingModel = "PublicReadOnly"
	if users.Definition.Fields == nil {
		users.Definition.Fields = make(map[string]storage.Field)
	}
	users.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString}
	target := users.Records[vfAccessOwnerID]
	target.System.OwnerID = vfAccessOwnerID
	target.Fields["Name"] = storage.StringValue(targetName)
	users.Records[vfAccessOwnerID] = target
	srv.Org.Objects["User"] = users

	profiles := srv.Org.Objects["Profile"]
	for _, id := range []storage.ID{vfAccessOwnerProfile, vfAccessReaderProfile} {
		profile := profiles.Records[id]
		profile.Fields["Name"] = storage.StringValue("Minimum Access - Salesforce")
		profiles.Records[id] = profile
	}
	srv.Org.Objects["Profile"] = profiles

	objects := srv.Org.Objects["ObjectPermissions"]
	objects.Records["110000000000097"] = visualforceLookupPermission("110000000000097", vfAccessOwnerProfile, "User", "", true)
	objects.Records["110000000000098"] = visualforceLookupPermission("110000000000098", vfAccessReaderProfile, "User", "", true)
	srv.Org.Objects["ObjectPermissions"] = objects
	fields := srv.Org.Objects["FieldPermissions"]
	fields.Records["120000000000097"] = visualforceLookupPermission("120000000000097", vfAccessOwnerProfile, "User", "Name", true)
	fields.Records["120000000000098"] = visualforceLookupPermission("120000000000098", vfAccessReaderProfile, "User", "Name", true)
	srv.Org.Objects["FieldPermissions"] = fields

	pageFile := filepath.Join(srv.Source.Project.Root, "force-app/main/default/pages/RecordAccess.page")
	writeServerTestFile(t, pageFile, `<apex:page standardController="Account"><apex:outputField value="{!Account.OwnerId}"/></apex:page>`)
	p, err := project.Load(srv.Source.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	index, err := visualforce.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := index.Page("RecordAccess")
	if !ok || page.APIVersion != "67.0" || page.StandardController != "Account" {
		t.Fatalf("lookup source profile found=%t API=%q controller=%q", ok, page.APIVersion, page.StandardController)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	srv.Source = source
	return srv
}

func visualforceLookupPermission(id string, profile storage.ID, object, field string, read bool) storage.Record {
	permissionObject := "ObjectPermissions"
	values := map[string]storage.Value{
		"ParentId": storage.IDValue(profile), "SObjectType": storage.StringValue(object),
		"PermissionsRead": storage.BooleanValue(read),
	}
	if field != "" {
		permissionObject = "FieldPermissions"
		values["Field"] = storage.StringValue(object + "." + field)
	}
	return storage.Record{ID: storage.ID(id), Object: permissionObject, Fields: values}
}

func requestVisualforceLookupRoute(srv *Server, path, requestUserID string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if requestUserID != "" {
		req.Header.Set("X-GLADE-User-Id", requestUserID)
	}
	srv.ServeHTTP(rec, req)
	return rec
}

func assertVisualforceLookupRouteHidden(t *testing.T, rec *httptest.ResponseRecorder, id storage.ID, name string) {
	t.Helper()
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), string(id)) ||
		strings.Contains(rec.Body.String(), name) || strings.Contains(rec.Body.String(), html.EscapeString(name)) {
		t.Fatalf("missing or unauthorized target leaked: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

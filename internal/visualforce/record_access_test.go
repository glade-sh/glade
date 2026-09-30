package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceStandardControllerInitialRenderHidesUnsharedRecordAPI67(t *testing.T) {
	const (
		pageName  = "RecordAccess"
		accountID = storage.ID("001000000000001AAA")
		ownerID   = storage.ID("005000000000002AAA")
	)

	readableFixture := newRecordAccessRenderFixture(t, `<apex:page standardController="Account"><apex:outputText value="{!Account.Name}"/></apex:page>`, accessTestUserID)
	readableHTML := readableFixture.render(t, pageName, accountID)
	if !strings.Contains(readableHTML, "VF_READABLE_NAME_CANARY") {
		t.Fatalf("positive control did not render the configured user's Account.Name: %s", readableHTML)
	}

	fixture := newRecordAccessRenderFixture(t, `<apex:page standardController="Account"><apex:outputText value="{!Account.Name}"/></apex:page>`, ownerID)
	html := fixture.render(t, pageName, accountID)
	if strings.Contains(html, "VF_READABLE_NAME_CANARY") {
		t.Fatalf("initial StandardController render exposed an unshared Account.Name: %s", html)
	}
}

func TestVisualforceOutputFieldDoesNotFallbackPastNameProjectionAPI67(t *testing.T) {
	const (
		pageName  = "RecordAccess"
		accountID = storage.ID("001000000000001AAA")
	)

	fixture := newRecordAccessRenderFixture(t, `<apex:page standardController="Account"><apex:outputText value="{!Account.Name}"/><apex:outputField value="{!Account.Secret__c}"/></apex:page>`, accessTestUserID)
	html := fixture.render(t, pageName, accountID)
	if !strings.Contains(html, "VF_READABLE_NAME_CANARY") {
		t.Fatalf("readable Name projection missing from HTML: %s", html)
	}
	if strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("outputField bypassed the Name/Id projection and exposed FLS-denied Secret__c: %s", html)
	}
}

func TestVisualforceUserGlobalUsesConfiguredExecutionUserAPI67(t *testing.T) {
	const (
		preferredUserID  = storage.ID("005-local-user")
		defaultUserID    = storage.ID("005000000000001")
		configuredID     = storage.ID("005000000000099AAA")
		preferredProfID  = storage.ID("00e000000000001AAA")
		defaultProfID    = storage.ID("00e000000000002AAA")
		configuredProfID = storage.ID("00e000000000099AAA")
	)

	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		preferredUserID: {ID: preferredUserID, Object: "User", Fields: map[string]storage.Value{
			"Username": storage.StringValue("preferred@example.test"), "ProfileId": storage.IDValue(preferredProfID),
		}},
		defaultUserID: {ID: defaultUserID, Object: "User", Fields: map[string]storage.Value{
			"Username": storage.StringValue("default@example.test"), "ProfileId": storage.IDValue(defaultProfID),
		}},
		configuredID: {ID: configuredID, Object: "User", Fields: map[string]storage.Value{
			"Username": storage.StringValue("configured@example.test"), "ProfileId": storage.IDValue(configuredProfID),
		}},
	}}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(org.Objects["User"].Records[configuredID])

	got, err := RenderExpressionTemplate("{!$User.Id}|{!$User.Username}|{!$Profile.Id}", &ExpressionContext{VM: machine})
	if err != nil {
		t.Fatal(err)
	}
	want := string(configuredID) + "|configured@example.test|" + string(configuredProfID)
	if got != want {
		t.Fatalf("$User global = %q, want configured execution user %q", got, want)
	}
}

func TestVisualforceUnsupportedRecordAccessFailsClosedAPI67(t *testing.T) {
	fixture := newRecordAccessRenderFixture(t, `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/></apex:form></apex:page>`, accessTestUserID)
	result, err := RenderPage(PageRenderRequest{
		Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
		PageName: "RecordAccess", PageURL: "/apex/RecordAccess?id=001000000000001AAA",
	})
	if err == nil && result.Error == nil {
		t.Fatalf("unsupported editable field rendered instead of failing closed: %s", result.HTML)
	}
	if strings.Contains(result.HTML, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("unsupported editable field leaked restricted data: %s", result.HTML)
	}
}

func TestVisualforceSetControllerOnlyRendersAuthorizedProjectionAPI67(t *testing.T) {
	markup := `<apex:page standardController="Account" recordSetVar="rows"><apex:repeat value="{!rows}" var="row"><apex:outputText value="{!row.Name}"/><apex:outputText value="{!row.Secret__c}"/></apex:repeat></apex:page>`
	readable := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	html := readable.render(t, "RecordAccess", "001000000000001AAA")
	if !strings.Contains(html, "VF_READABLE_NAME_CANARY") || strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("set projection omitted readable Name or exposed denied Secret__c: %s", html)
	}
	private := newRecordAccessRenderFixture(t, markup, "005000000000002AAA")
	html = private.render(t, "RecordAccess", "001000000000001AAA")
	if strings.Contains(html, "VF_READABLE_NAME_CANARY") || strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("set projection exposed private row: %s", html)
	}
}

const (
	accessTestUserID    = storage.ID("005000000000001AAA")
	accessTestProfileID = storage.ID("00e000000000001AAA")
)

type recordAccessRenderFixture struct {
	project project.Project
	index   Index
	org     storage.OrgState
	machine *vm.VM
}

func newRecordAccessRenderFixture(t *testing.T, markup string, ownerID storage.ID) recordAccessRenderFixture {
	t.Helper()
	const pageName = "RecordAccess"
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	pagePath := filepath.Join(root, "force-app/main/default/pages/"+pageName+".page")
	writeFile(t, pagePath, markup)
	writeFile(t, pagePath+"-meta.xml", `<?xml version="1.0" encoding="UTF-8"?><ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Record Access 67</label></ApexPage>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := index.Page(pageName)
	if !ok || page.APIVersion != "67.0" || page.StandardController != "Account" {
		t.Fatalf("fixture page profile = found:%t API:%q controller:%q; want API 67.0 Account StandardController", ok, page.APIVersion, page.StandardController)
	}

	user := storage.Record{ID: accessTestUserID, Object: "User", Fields: map[string]storage.Value{
		"ProfileId": storage.IDValue(accessTestProfileID),
		"Username":  storage.StringValue("vf-render-user@example.test"),
		"UserType":  storage.StringValue("Standard"),
	}}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account", KeyPrefix: "001", SharingModel: "Private",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"Secret__c": {APIName: "Secret__c", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001AAA": {
				ID: "001000000000001AAA", Object: "Account",
				System: storage.SystemFields{OwnerID: ownerID},
				Fields: map[string]storage.Value{
					"Name":      storage.StringValue("VF_READABLE_NAME_CANARY"),
					"Secret__c": storage.StringValue("VF_SECRET_FIELD_CANARY"),
				},
			},
		},
	}
	users := map[storage.ID]storage.Record{
		accessTestUserID: user,
	}
	if ownerID != accessTestUserID {
		users[ownerID] = storage.Record{ID: ownerID, Object: "User"}
	}
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "User", KeyPrefix: "005"},
		Records:    users,
	}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		accessTestProfileID: {ID: accessTestProfileID, Object: "Profile", Fields: map[string]storage.Value{
			"Name": storage.StringValue("VF Render Access User"),
		}},
	}}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{
			"ParentId": accessTestProfileIDValue(), "SObjectType": storage.StringValue("Account"),
			"PermissionsRead": storage.BooleanValue(true),
		}},
	}}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"120000000000001": {ID: "120000000000001", Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": accessTestProfileIDValue(), "SObjectType": storage.StringValue("Account"),
			"Field": storage.StringValue("Account.Name"), "PermissionsRead": storage.BooleanValue(true),
		}},
		"120000000000002": {ID: "120000000000002", Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": accessTestProfileIDValue(), "SObjectType": storage.StringValue("Account"),
			"Field": storage.StringValue("Account.Secret__c"), "PermissionsRead": storage.BooleanValue(false),
		}},
	}}
	org.Objects["AccountShare"] = storage.ObjectState{Records: map[storage.ID]storage.Record{}}

	machine := vm.New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(user)
	return recordAccessRenderFixture{project: p, index: index, org: org, machine: machine}
}

func accessTestProfileIDValue() storage.Value {
	return storage.IDValue(accessTestProfileID)
}

func (fixture recordAccessRenderFixture) render(t *testing.T, pageName string, recordID storage.ID) string {
	t.Helper()
	result, err := RenderPage(PageRenderRequest{
		Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
		PageName: pageName, PageURL: "/apex/" + pageName + "?id=" + string(recordID),
	})
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("RenderPage result error: %v", result.Error)
	}
	return result.HTML
}

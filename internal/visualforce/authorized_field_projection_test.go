package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestVisualforceAuthorizedScalarFieldRenderingAPI67(t *testing.T) {
	const recordID = storage.ID("001000000000001AAA")
	for _, test := range []struct {
		name   string
		markup string
		want   string
	}{
		{"outputField", `<apex:page standardController="Account"><apex:outputField value="{!Account.Secret__c}"/></apex:page>`, "VF_SECRET_FIELD_CANARY"},
		{"inputField", `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/></apex:form></apex:page>`, `value="VF_SECRET_FIELD_CANARY"`},
		{"outputText", `<apex:page standardController="Account"><apex:outputText value="{!Account.Secret__c}"/></apex:page>`, "VF_SECRET_FIELD_CANARY"},
		{"text expression", `<apex:page standardController="Account">{!Account.Secret__c}</apex:page>`, "VF_SECRET_FIELD_CANARY"},
		{"nested expression", `<apex:page standardController="Account"><apex:outputText value="{!UPPER(Account.Secret__c)}"/></apex:page>`, "VF_SECRET_FIELD_CANARY"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRecordAccessRenderFixture(t, test.markup, accessTestUserID)
			permissions := fixture.org.Objects["FieldPermissions"]
			permission := permissions.Records["120000000000002"]
			permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
			permissions.Records["120000000000002"] = permission
			fixture.org.Objects["FieldPermissions"] = permissions
			result, err := RenderPage(PageRenderRequest{
				Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
				PageName: "RecordAccess", PageURL: "/apex/RecordAccess?id=" + string(recordID),
			})
			if err != nil || result.Error != nil || !strings.Contains(result.HTML, test.want) {
				t.Fatalf("authorized field missing in %s: err=%v resultErr=%v html=%q", test.name, err, result.Error, result.HTML)
			}
		})
	}
}

func TestVisualforceDeniedScalarExpressionStaysBlankAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:outputText value="{!Account.Name}"/><apex:outputText value="{!Account.Secret__c}"/></apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if !strings.Contains(html, "VF_READABLE_NAME_CANARY") || strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("authorized Name or denied Secret__c expression projection wrong: %s", html)
	}
}

func TestVisualforceGrantedFieldCannotReadPrivateRowAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:outputField value="{!Account.Secret__c}"/></apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, "005000000000002AAA")
	permissions := fixture.org.Objects["FieldPermissions"]
	permission := permissions.Records["120000000000002"]
	permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
	permissions.Records["120000000000002"] = permission
	fixture.org.Objects["FieldPermissions"] = permissions
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("granted field exposed private row: %s", html)
	}
}

func TestVisualforceDeniedFieldDoesNotSuppressGrantedSiblingAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:outputField value="{!Account.Public__c}"/><apex:outputField value="{!Account.Secret__c}"/></apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	account := fixture.org.Objects["Account"]
	account.Definition.Fields["Public__c"] = storage.Field{APIName: "Public__c", Type: storage.FieldString}
	row := account.Records["001000000000001AAA"]
	row.Fields["Public__c"] = storage.StringValue("VF_PUBLIC_FIELD_CANARY")
	account.Records[row.ID] = row
	fixture.org.Objects["Account"] = account
	permissions := fixture.org.Objects["FieldPermissions"]
	permissions.Records["120000000000003"] = storage.Record{ID: "120000000000003", Object: "FieldPermissions", Fields: map[string]storage.Value{
		"ParentId": accessTestProfileIDValue(), "SObjectType": storage.StringValue("Account"),
		"Field": storage.StringValue("Account.Public__c"), "PermissionsRead": storage.BooleanValue(true),
	}}
	fixture.org.Objects["FieldPermissions"] = permissions
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if !strings.Contains(html, "VF_PUBLIC_FIELD_CANARY") || strings.Contains(html, "VF_SECRET_FIELD_CANARY") {
		t.Fatalf("mixed field permissions did not isolate granted sibling: %s", html)
	}
}

func TestVisualforceAuthorizedInputFieldRerendersSubmittedValueAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/><apex:outputField value="{!Account.Secret__c}"/><apex:outputText value="{!Account.Secret__c}"/></apex:form></apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	permissions := fixture.org.Objects["FieldPermissions"]
	permission := permissions.Records["120000000000002"]
	permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
	permissions.Records["120000000000002"] = permission
	fixture.org.Objects["FieldPermissions"] = permissions
	result, err := RenderPage(PageRenderRequest{
		Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
		PageName: "RecordAccess", PageURL: "/apex/RecordAccess?id=001000000000001AAA",
		FormValues: map[string]string{"Account.Secret__c": "VF_SUBMITTED_FIELD_CANARY"},
	})
	if err != nil || result.Error != nil {
		t.Fatalf("POST rerender failed: err=%v resultErr=%v", err, result.Error)
	}
	if strings.Contains(result.HTML, "VF_SECRET_FIELD_CANARY") || strings.Count(result.HTML, "VF_SUBMITTED_FIELD_CANARY") != 3 {
		t.Fatalf("field components did not retain the submitted value: %s", result.HTML)
	}
}

func TestVisualforceAuthorizedInputFieldWithoutVisibleRowStaysBlankAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/><apex:outputField value="{!Account.Secret__c}"/></apex:form></apex:page>`
	for _, test := range []struct {
		name  string
		owner storage.ID
		id    string
	}{
		{"missing", accessTestUserID, "001000000000002AAA"},
		{"unshared", "005000000000002AAA", "001000000000001AAA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRecordAccessRenderFixture(t, markup, test.owner)
			permissions := fixture.org.Objects["FieldPermissions"]
			permission := permissions.Records["120000000000002"]
			permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
			permissions.Records["120000000000002"] = permission
			fixture.org.Objects["FieldPermissions"] = permissions
			result, err := RenderPage(PageRenderRequest{
				Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
				PageName: "RecordAccess", PageURL: "/apex/RecordAccess?id=" + test.id,
				FormValues: map[string]string{"Account.Secret__c": "VF_SUBMITTED_FIELD_CANARY"},
			})
			if err != nil || result.Error != nil {
				t.Fatalf("absent-row render err=%v resultErr=%v", err, result.Error)
			}
			if !strings.Contains(result.HTML, `name="Account.Secret__c"`) || !strings.Contains(result.HTML, `value=""`) ||
				strings.Contains(result.HTML, "VF_SECRET_FIELD_CANARY") || strings.Contains(result.HTML, "VF_SUBMITTED_FIELD_CANARY") {
				t.Fatalf("absent row exposed a stored or submitted value: %s", result.HTML)
			}
		})
	}
}

func TestVisualforceDeniedInputFieldCannotUseMissingRowAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/></apex:form></apex:page>`
	for _, name := range []string{"FLS denied", "object denied", "forged user"} {
		t.Run(name, func(t *testing.T) {
			fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
			switch name {
			case "object denied":
				permissions := fixture.org.Objects["ObjectPermissions"]
				permission := permissions.Records["110000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["110000000000001"] = permission
				fixture.org.Objects["ObjectPermissions"] = permissions
			case "forged user":
				fixture.machine.SetCurrentUser(storage.Record{ID: "005000000000999AAA", Object: "User"})
			}
			result, err := RenderPage(PageRenderRequest{
				Project: fixture.project, VFIndex: fixture.index, Org: &fixture.org, Machine: fixture.machine,
				PageName: "RecordAccess", PageURL: "/apex/RecordAccess?id=001000000000002AAA",
			})
			if err == nil && result.Error == nil {
				t.Fatalf("%s admitted denied input field: %s", name, result.HTML)
			}
			if strings.Contains(result.HTML, "VF_SECRET_FIELD_CANARY") {
				t.Fatalf("%s leaked stored field: %s", name, result.HTML)
			}
		})
	}
}

func TestVisualforceNamespacedStandardControllerFieldAliasAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Thing__c"><apex:form><apex:inputField value="{!Thing__c.Secret__c}"/></apex:form><apex:outputField value="{!Thing__c.Secret__c}"/><apex:outputText value="{!Thing__c.Secret__c}"/>{!Thing__c.Secret__c}</apex:page>`
	fixture := newRecordAccessRenderFixture(t, `<apex:page standardController="Account"/>`, accessTestUserID)
	fixture.project.Namespace = "pkg"
	page, _ := fixture.index.Page("RecordAccess")
	writeFile(t, page.File, markup)
	var err error
	fixture.index, err = LoadProject(fixture.project)
	if err != nil {
		t.Fatal(err)
	}
	fixture.org.Namespace = "pkg"
	account := fixture.org.Objects["Account"]
	delete(fixture.org.Objects, "Account")
	account.Definition.APIName = "pkg__Thing__c"
	for id, row := range account.Records {
		row.Object = "pkg__Thing__c"
		account.Records[id] = row
	}
	fixture.org.Objects["pkg__Thing__c"] = account
	objectPermissions := fixture.org.Objects["ObjectPermissions"]
	permission := objectPermissions.Records["110000000000001"]
	permission.Fields["SObjectType"] = storage.StringValue("pkg__Thing__c")
	objectPermissions.Records["110000000000001"] = permission
	fixture.org.Objects["ObjectPermissions"] = objectPermissions
	fieldPermissions := fixture.org.Objects["FieldPermissions"]
	for id, permission := range fieldPermissions.Records {
		permission.Fields["SObjectType"] = storage.StringValue("pkg__Thing__c")
		field := permission.Fields["Field"].String
		permission.Fields["Field"] = storage.StringValue(strings.Replace(field, "Account.", "pkg__Thing__c.", 1))
		if strings.HasSuffix(field, ".Secret__c") {
			permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
		}
		fieldPermissions.Records[id] = permission
	}
	fixture.org.Objects["FieldPermissions"] = fieldPermissions
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if strings.Count(html, "VF_SECRET_FIELD_CANARY") != 4 {
		t.Fatalf("namespaced standard-controller alias did not render all authorized bindings: %s", html)
	}
}

func TestVisualforceAuthorizedFieldRetainsNumberMetadataAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:form><apex:inputField value="{!Account.Secret__c}"/><apex:outputField value="{!Account.Secret__c}"/></apex:form></apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	account := fixture.org.Objects["Account"]
	account.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldInteger, Label: "Authorized Number"}
	row := account.Records["001000000000001AAA"]
	row.Fields["Secret__c"] = storage.IntegerValue(42)
	account.Records[row.ID] = row
	fixture.org.Objects["Account"] = account
	permissions := fixture.org.Objects["FieldPermissions"]
	permission := permissions.Records["120000000000002"]
	permission.Fields["PermissionsRead"] = storage.BooleanValue(true)
	permissions.Records["120000000000002"] = permission
	fixture.org.Objects["FieldPermissions"] = permissions
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if !strings.Contains(html, `type="number"`) || !strings.Contains(html, `value="42"`) ||
		!strings.Contains(html, `step="1"`) || !strings.Contains(html, `>42</span>`) {
		t.Fatalf("authorized numeric field lost metadata formatting: %s", html)
	}
}

func TestVisualforceUnsupportedRelationshipPathDoesNotReadRawRecordAPI67(t *testing.T) {
	const markup = `<apex:page standardController="Account"><apex:outputField value="{!Account.Owner.Name}"/><apex:outputText value="{!Account.Owner.Name}"/>{!Account.Owner.Name}</apex:page>`
	fixture := newRecordAccessRenderFixture(t, markup, accessTestUserID)
	account := fixture.org.Objects["Account"]
	row := account.Records["001000000000001AAA"]
	row.Fields["Owner.Name"] = storage.StringValue("VF_RELATIONSHIP_CANARY")
	row.ParentRelationships = map[string]storage.Record{"Owner": {
		ID: accessTestUserID, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("VF_RELATIONSHIP_CANARY")},
	}}
	account.Records[row.ID] = row
	fixture.org.Objects["Account"] = account
	html := fixture.render(t, "RecordAccess", "001000000000001AAA")
	if strings.Contains(html, "VF_RELATIONSHIP_CANARY") {
		t.Fatalf("unsupported relationship field leaked raw record data: %s", html)
	}
}

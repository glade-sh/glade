package visualforce

import (
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

const (
	fieldRenderingTestUserID          storage.ID = "005000000000091AAA"
	fieldRenderingTestProfileID       storage.ID = "00e000000000091AAA"
	fieldRenderingAccountPermissionID storage.ID = "110000000000091"
	fieldRenderingNamePermissionID    storage.ID = "120000000000000001"
)

func authorizeFieldRenderingFixture(t *testing.T, org *storage.OrgState, machine *vm.VM, fields ...string) {
	t.Helper()
	user := storage.Record{ID: fieldRenderingTestUserID, Object: "User", Fields: map[string]storage.Value{
		"ProfileId": storage.IDValue(fieldRenderingTestProfileID),
		"Username":  storage.StringValue("vf-field-render-user@example.test"),
		"UserType":  storage.StringValue("Standard"),
	}}
	users := org.Objects["User"]
	users.Definition.APIName = "User"
	users.Definition.KeyPrefix = "005"
	if users.Records == nil {
		users.Records = make(map[storage.ID]storage.Record)
	}
	users.Records[fieldRenderingTestUserID] = user
	org.Objects["User"] = users

	profiles := org.Objects["Profile"]
	if profiles.Records == nil {
		profiles.Records = make(map[storage.ID]storage.Record)
	}
	profiles.Records[fieldRenderingTestProfileID] = storage.Record{
		ID: fieldRenderingTestProfileID, Object: "Profile",
		Fields: map[string]storage.Value{
			"Name":                     storage.StringValue("Minimum Access - Salesforce"),
			"PermissionsViewAllData":   storage.BooleanValue(false),
			"PermissionsModifyAllData": storage.BooleanValue(false),
		},
	}
	org.Objects["Profile"] = profiles

	objectPermissions := org.Objects["ObjectPermissions"]
	if objectPermissions.Records == nil {
		objectPermissions.Records = make(map[storage.ID]storage.Record)
	}
	for id, permission := range objectPermissions.Records {
		parent, _ := permission.GetField("ParentId")
		if parent.Kind == storage.ValueID && storage.IDsEqual(parent.ID, fieldRenderingTestProfileID) {
			delete(objectPermissions.Records, id)
		}
	}
	objectPermissions.Records[fieldRenderingAccountPermissionID] = storage.Record{
		ID: fieldRenderingAccountPermissionID, Object: "ObjectPermissions",
		Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue(fieldRenderingTestProfileID),
			"SObjectType":     storage.StringValue("Account"),
			"PermissionsRead": storage.BooleanValue(true),
		},
	}
	org.Objects["ObjectPermissions"] = objectPermissions

	fieldPermissions := org.Objects["FieldPermissions"]
	if fieldPermissions.Records == nil {
		fieldPermissions.Records = make(map[storage.ID]storage.Record)
	}
	for id, permission := range fieldPermissions.Records {
		parent, _ := permission.GetField("ParentId")
		if parent.Kind == storage.ValueID && storage.IDsEqual(parent.ID, fieldRenderingTestProfileID) {
			delete(fieldPermissions.Records, id)
		}
	}
	readFields := append([]string{"Name"}, fields...)
	seen := make(map[string]struct{}, len(readFields))
	for index, field := range readFields {
		key := strings.ToLower(field)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		permissionID := storage.ID(fmt.Sprintf("120000000000%06d", index+1))
		fieldPermissions.Records[permissionID] = storage.Record{
			ID: permissionID, Object: "FieldPermissions",
			Fields: map[string]storage.Value{
				"ParentId":        storage.IDValue(fieldRenderingTestProfileID),
				"SObjectType":     storage.StringValue("Account"),
				"Field":           storage.StringValue("Account." + field),
				"PermissionsRead": storage.BooleanValue(true),
			},
		}
	}
	org.Objects["FieldPermissions"] = fieldPermissions

	account := org.Objects["Account"]
	account.Definition.SharingModel = "PublicReadOnly"
	org.Objects["Account"] = account
	machine.SetCurrentUser(user)
}

func authorizeFieldRenderingWriteFixture(t *testing.T, org *storage.OrgState) {
	t.Helper()
	objectPermissions := org.Objects["ObjectPermissions"]
	accountGrant := objectPermissions.Records[fieldRenderingAccountPermissionID]
	accountGrant.Fields["PermissionsCreate"] = storage.BooleanValue(true)
	accountGrant.Fields["PermissionsEdit"] = storage.BooleanValue(true)
	accountGrant.Fields["PermissionsDelete"] = storage.BooleanValue(true)
	objectPermissions.Records[fieldRenderingAccountPermissionID] = accountGrant
	org.Objects["ObjectPermissions"] = objectPermissions

	fieldPermissions := org.Objects["FieldPermissions"]
	nameGrant := fieldPermissions.Records[fieldRenderingNamePermissionID]
	nameGrant.Fields["PermissionsEdit"] = storage.BooleanValue(true)
	fieldPermissions.Records[fieldRenderingNamePermissionID] = nameGrant
	org.Objects["FieldPermissions"] = fieldPermissions

	account := org.Objects["Account"]
	for id, record := range account.Records {
		record.System.OwnerID = fieldRenderingTestUserID
		account.Records[id] = record
	}
	org.Objects["Account"] = account
}

func authorizeFieldRenderingOwnerNameFixture(t *testing.T, org *storage.OrgState, owner storage.Record) {
	t.Helper()
	users := org.Objects["User"]
	if users.Definition.Fields == nil {
		users.Definition.Fields = make(map[string]storage.Field)
	}
	if _, ok := users.Definition.Fields["Name"]; !ok {
		users.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Full Name", Type: storage.FieldString}
	}
	users.Records[owner.ID] = owner
	org.Objects["User"] = users

	objectPermissions := org.Objects["ObjectPermissions"]
	objectPermissions.Records["110000000000092"] = storage.Record{
		ID: "110000000000092", Object: "ObjectPermissions",
		Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue(fieldRenderingTestProfileID),
			"SObjectType":     storage.StringValue("User"),
			"PermissionsRead": storage.BooleanValue(true),
		},
	}
	org.Objects["ObjectPermissions"] = objectPermissions

	fieldPermissions := org.Objects["FieldPermissions"]
	fieldPermissions.Records["120000000000000099"] = storage.Record{
		ID: "120000000000000099", Object: "FieldPermissions",
		Fields: map[string]storage.Value{
			"ParentId":        storage.IDValue(fieldRenderingTestProfileID),
			"SObjectType":     storage.StringValue("User"),
			"Field":           storage.StringValue("User.Name"),
			"PermissionsRead": storage.BooleanValue(true),
		},
	}
	org.Objects["FieldPermissions"] = fieldPermissions
}

func TestRenderInputFieldUsesLocalSchemaAndRecordValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"), `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:outputField value="{!Account.Name}"/>
    <apex:inputField id="industry" value="{!Account.Industry}"/>
    <apex:inputField id="rating" value="{!Account.Rating}"/>
  </apex:form>
</apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":     {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			"Industry": {APIName: "Industry", Label: "Industry", Type: storage.FieldString},
			"Rating": {
				APIName: "Rating",
				Label:   "Rating",
				Type:    storage.FieldPicklist,
				PicklistValues: []storage.PicklistValue{
					{Value: "Hot", Label: "Hot", Active: true},
					{Value: "Warm", Label: "Warm", Active: true},
				},
			},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":     storage.StringValue("Acme"),
					"Industry": storage.StringValue("Manufacturing"),
					"Rating":   storage.StringValue("Hot"),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, machine, "Industry", "Rating")

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields?id=001000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`Acme`,
		`id="j_id0:f:industry"`,
		`name="Account.Industry"`,
		`value="Manufacturing"`,
		`<option value="Hot" selected="selected">Hot</option>`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestRenderInputFieldUsesDisplayMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"), `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:inputField value="{!Account.Description}"/>
    <apex:inputField value="{!Account.Email__c}"/>
    <apex:inputField value="{!Account.Website}"/>
    <apex:inputField value="{!Account.Phone}"/>
    <apex:inputField value="{!Account.AnnualRevenue}"/>
    <apex:inputField value="{!Account.Locked__c}"/>
  </apex:form>
</apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":          {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			"Description":   {APIName: "Description", Label: "Description", Type: storage.FieldString, DisplayType: "TEXTAREA", Required: true},
			"Email__c":      {APIName: "Email__c", Label: "Email", Type: storage.FieldString, DisplayType: "EMAIL"},
			"Website":       {APIName: "Website", Label: "Website", Type: storage.FieldString, DisplayType: "URL"},
			"Phone":         {APIName: "Phone", Label: "Phone", Type: storage.FieldString, DisplayType: "PHONE"},
			"AnnualRevenue": {APIName: "AnnualRevenue", Label: "Annual Revenue", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Scale: 2},
			"Locked__c":     {APIName: "Locked__c", Label: "Locked", Type: storage.FieldString, DisplayType: "STRING", Updateable: storage.BoolFlag(false)},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":          storage.StringValue("Metadata Fixture"),
					"Description":   storage.StringValue("Line 1\n<Line 2>"),
					"Email__c":      storage.StringValue("ada@example.test"),
					"Website":       storage.StringValue("https://example.test"),
					"Phone":         storage.StringValue("+1 555 0100"),
					"AnnualRevenue": storage.DecimalValue("1234.50"),
					"Locked__c":     storage.StringValue("fixed"),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(
		t, &org, machine,
		"Description", "Email__c", "Website", "Phone", "AnnualRevenue", "Locked__c",
	)

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields?id=001000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<textarea class="inputField" name="Account.Description" id="Description" required="required">Line 1`,
		`&lt;Line 2&gt;</textarea>`,
		`type="email" class="inputField" name="Account.Email__c" id="Email__c" value="ada@example.test"`,
		`type="url" class="inputField" name="Account.Website" id="Website" value="https://example.test"`,
		`type="tel" class="inputField" name="Account.Phone" id="Phone" value="+1 555 0100"`,
		`type="number" class="inputField" name="Account.AnnualRevenue" id="AnnualRevenue" value="1234.50" step="0.01"`,
		`type="text" class="inputField" name="Account.Locked__c" id="Locked__c" value="fixed" readonly="readonly"`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestRenderInputFieldUsesMultiPicklistAndCheckboxMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"), `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:inputField id="segments" value="{!Account.Segments__c}"/>
    <apex:inputField id="active" value="{!Account.Active__c}"/>
  </apex:form>
</apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			"Segments__c": {
				APIName:     "Segments__c",
				Label:       "Segments",
				Type:        storage.FieldMultiPicklist,
				DisplayType: "MULTIPICKLIST",
				PicklistValues: []storage.PicklistValue{
					{Value: "Retail", Label: "Retail", Active: true},
					{Value: "Services", Label: "Services", Active: true},
					{Value: "Dormant", Label: "Dormant", Active: true},
				},
			},
			"Active__c": {APIName: "Active__c", Label: "Active", Type: storage.FieldBoolean, DisplayType: "BOOLEAN"},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":        storage.StringValue("MultiPicklist Fixture"),
					"Segments__c": storage.StringValue("Retail;Services"),
					"Active__c":   storage.BooleanValue(true),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, machine, "Segments__c", "Active__c")

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields?id=001000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<select class="inputField" name="Account.Segments__c" id="j_id0:f:segments" multiple="multiple">`,
		`<option value="Retail" selected="selected">Retail</option>`,
		`<option value="Services" selected="selected">Services</option>`,
		`<option value="Dormant">Dormant</option>`,
		`<input type="hidden" name="Account.Active__c" value="false" />`,
		`<input type="checkbox" class="inputField" name="Account.Active__c" id="j_id0:f:active" value="true" checked="checked" />`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestRenderOutputFieldUsesParentRelationshipDisplayName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"), `<apex:page standardController="Account">
  <apex:outputField value="{!Account.OwnerId}"/>
</apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":    {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			"OwnerId": {APIName: "OwnerId", Label: "Owner", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				System: storage.SystemFields{OwnerID: "005000000000001AAA"},
				Fields: map[string]storage.Value{
					"Name":    storage.StringValue("Relationship Fixture"),
					"OwnerId": storage.IDValue("005000000000001AAA"),
				},
				ParentRelationships: map[string]storage.Record{
					"Owner": {
						ID:     "005000000000001AAA",
						Object: "User",
						Fields: map[string]storage.Value{
							"Name": storage.StringValue("Ada Owner"),
						},
					},
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, machine, "OwnerId")
	owner := org.Objects["Account"].Records["001000000000001"].ParentRelationships["Owner"]
	authorizeFieldRenderingOwnerNameFixture(t, &org, owner)

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields?id=001000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := nethtml.Parse(strings.NewReader(result.HTML))
	if err != nil {
		t.Fatal(err)
	}
	links := fieldPresentationNodes(doc, func(n *nethtml.Node) bool { return n.Data == "a" })
	if len(links) != 1 {
		t.Fatalf("owner display links=%d, want one", len(links))
	}
	link := links[0]
	if link.Parent == nil || link.Parent.Data != "span" {
		t.Fatal("owner link is not inside its outputField span")
	}
	wantHref := "/record/User/" + string(owner.ID)
	linkText := fieldPresentationText(link)
	spanText := fieldPresentationText(link.Parent)
	if fieldPresentationAttr(link, "href") != wantHref || linkText != "Ada Owner" || spanText != "Ada Owner" ||
		strings.Contains(spanText, string(owner.ID)) {
		t.Fatalf("owner outputField authorizedHref=%t linkName=%t spanName=%t idVisible=%t",
			fieldPresentationAttr(link, "href") == wantHref, linkText == "Ada Owner", spanText == "Ada Owner",
			strings.Contains(spanText, string(owner.ID)))
	}
}

func TestRenderOwnerOutputFieldRequiresAuthorizedUserName(t *testing.T) {
	const (
		accountID = storage.ID("001000000000001AAA")
		ownerID   = storage.ID("005000000000001AAA")
		ownerName = "Authorized & <Owner>"
	)
	for _, test := range []struct {
		name string
		deny string
		want string
	}{
		{"readable target", "", ownerName},
		{"User object denied", "object", string(ownerID)},
		{"User Name denied", "field", string(ownerID)},
		{"target missing", "missing", string(ownerID)},
		{"target unshared", "sharing", string(ownerID)},
		{"source Account unshared", "source", ""},
		{"metadata excludes User", "metadata", string(ownerID)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"),
				`<apex:page standardController="Account"><apex:outputField value="{!Account.OwnerId}"/></apex:page>`)
			project, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index, err := LoadProject(project)
			if err != nil {
				t.Fatal(err)
			}
			owner := storage.Record{ID: ownerID, Object: "User", System: storage.SystemFields{OwnerID: ownerID},
				Fields: map[string]storage.Value{"Name": storage.StringValue(ownerName)}}
			org := storage.NewOrgState()
			org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
					"Name":    {APIName: "Name", Type: storage.FieldString},
					"OwnerId": {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
				}}, Records: map[storage.ID]storage.Record{accountID: {
				ID: accountID, Object: "Account", Fields: map[string]storage.Value{
					"Name": storage.StringValue("Source"), "OwnerId": storage.IDValue(ownerID),
					"Owner.Name": storage.StringValue("RAW_FLATTENED_OWNER_CANARY"),
				}, ParentRelationships: map[string]storage.Record{"Owner": {
					ID: ownerID, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("RAW_PARENT_OWNER_CANARY")},
				}},
			}}}
			machine := vm.New(nil)
			machine.SetOrg(&org)
			authorizeFieldRenderingFixture(t, &org, machine, "OwnerId")
			authorizeFieldRenderingOwnerNameFixture(t, &org, owner)
			users := org.Objects["User"]
			users.Definition.SharingModel = "PublicReadOnly"
			org.Objects["User"] = users
			switch test.deny {
			case "object":
				permissions := org.Objects["ObjectPermissions"]
				grant := permissions.Records["110000000000092"]
				grant.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records[grant.ID] = grant
				org.Objects["ObjectPermissions"] = permissions
			case "field":
				permissions := org.Objects["FieldPermissions"]
				grant := permissions.Records["120000000000000099"]
				grant.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records[grant.ID] = grant
				org.Objects["FieldPermissions"] = permissions
			case "missing":
				users := org.Objects["User"]
				delete(users.Records, ownerID)
				org.Objects["User"] = users
			case "sharing":
				users := org.Objects["User"]
				users.Definition.SharingModel = "Private"
				org.Objects["User"] = users
			case "source":
				accounts := org.Objects["Account"]
				accounts.Definition.SharingModel = "Private"
				row := accounts.Records[accountID]
				row.System.OwnerID = ownerID
				accounts.Records[accountID] = row
				org.Objects["Account"] = accounts
			case "metadata":
				accounts := org.Objects["Account"]
				field := accounts.Definition.Fields["OwnerId"]
				field.ReferenceTo = []string{"Group"}
				accounts.Definition.Fields["OwnerId"] = field
				org.Objects["Account"] = accounts
			}
			result, err := RenderPage(PageRenderRequest{Project: project, VFIndex: index, Org: &org,
				Machine: machine, PageName: "Fields", PageURL: "/apex/Fields?id=" + string(accountID)})
			if err != nil || result.Error != nil {
				t.Fatalf("owner render err=%v resultErr=%v", err, result.Error)
			}
			rawParent := strings.Contains(result.HTML, "RAW_PARENT_OWNER_CANARY")
			rawFlattened := strings.Contains(result.HTML, "RAW_FLATTENED_OWNER_CANARY")
			unescapedName := strings.Contains(result.HTML, "<Owner>")
			if rawParent || rawFlattened || unescapedName {
				t.Fatalf("owner render rawParent=%t rawFlattened=%t unescapedName=%t", rawParent, rawFlattened, unescapedName)
			}
			const linkStart = `<a href=`
			if test.deny == "" {
				wantLink := `<a href="/record/User/` + string(ownerID) + `">` + html.EscapeString(ownerName) + `</a>`
				if count := strings.Count(result.HTML, linkStart); count != 1 || !strings.Contains(result.HTML, wantLink) {
					t.Fatalf("readable owner links=%d exactAuthorizedLink=%t; want one", count, strings.Contains(result.HTML, wantLink))
				}
			} else if strings.Contains(result.HTML, linkStart) {
				t.Fatal("denied or unavailable owner rendered a link")
			}
			if test.want != "" && !strings.Contains(result.HTML, html.EscapeString(test.want)) {
				t.Fatal("owner display missing expected escaped text")
			}
			if test.deny != "" && strings.Contains(result.HTML, "Authorized") {
				t.Fatal("denied owner name leaked")
			}
			if test.deny == "source" && strings.Contains(result.HTML, string(ownerID)) {
				t.Fatal("unshared source OwnerId leaked")
			}
		})
	}
}

func TestRenderInputFieldDoesNotUseArbitraryRecordWhenPageIDMisses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Fields.page"), `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:outputField value="{!Account.Name}"/>
    <apex:inputField id="industry" value="{!Account.Industry}"/>
  </apex:form>
</apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name":     {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
			"Industry": {APIName: "Industry", Label: "Industry", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":     storage.StringValue("Acme"),
					"Industry": storage.StringValue("Manufacturing"),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, machine, "Industry")

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields?id=001000000000002",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, notWant := range []string{`Acme`, `value="Manufacturing"`} {
		if strings.Contains(result.HTML, notWant) {
			t.Fatalf("html should not include arbitrary record value %q: %s", notWant, result.HTML)
		}
	}
	if !strings.Contains(result.HTML, `id="j_id0:f:industry"`) || !strings.Contains(result.HTML, `name="Account.Industry"`) || !strings.Contains(result.HTML, `value=""`) {
		t.Fatalf("html missing empty input value for unresolved page id: %s", result.HTML)
	}

	result, err = RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Fields",
		PageURL:  "/apex/Fields",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, notWant := range []string{`Acme`, `value="Manufacturing"`} {
		if strings.Contains(result.HTML, notWant) {
			t.Fatalf("html without id should not include arbitrary record value %q: %s", notWant, result.HTML)
		}
	}
}

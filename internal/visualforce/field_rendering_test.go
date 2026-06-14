package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

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
		`id="industry"`,
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
			"OwnerId": {APIName: "OwnerId", Label: "Owner", Type: storage.FieldReference, DisplayType: "REFERENCE", ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
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
	if !strings.Contains(result.HTML, `Ada Owner`) {
		t.Fatalf("html missing relationship display name: %s", result.HTML)
	}
	if strings.Contains(result.HTML, `005000000000001AAA`) {
		t.Fatalf("html should prefer relationship display name over id: %s", result.HTML)
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
	if !strings.Contains(result.HTML, `id="industry"`) || !strings.Contains(result.HTML, `name="Account.Industry"`) || !strings.Contains(result.HTML, `value=""`) {
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

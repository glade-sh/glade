package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderInputFieldRequiredAttributeUsesPageRenderer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/RequiredFields.page"), `<apex:page standardController="Account">
  <apex:form id="f">
    <apex:inputField id="requiredEmpty" value="{!Account.RequiredEmpty__c}" required="true"/>
    <apex:inputField id="requiredPresent" value="{!Account.RequiredPresent__c}" required="true"/>
    <apex:inputField id="requiredTrueExpression" value="{!Account.RequiredTrueExpression__c}" required="{!true}"/>
    <apex:inputField id="requiredFalseExpression" value="{!Account.RequiredFalseExpression__c}" required="{!false}"/>
    <apex:inputField id="requiredTextarea" value="{!Account.RequiredTextarea__c}" required="true"/>
    <apex:inputField id="requiredPicklist" value="{!Account.RequiredPicklist__c}" required="true"/>
    <apex:inputField id="requiredCheckbox" value="{!Account.RequiredCheckbox__c}" required="true"/>
    <apex:inputField id="schemaRequiredCheckbox" value="{!Account.SchemaRequiredCheckbox__c}" required="false"/>
    <apex:inputField id="omitted" value="{!Account.Omitted__c}"/>
    <apex:inputField id="explicitFalse" value="{!Account.ExplicitFalse__c}" required="false"/>
    <apex:inputField id="standardName" value="{!Account.Name}" required="false"/>
    <apex:inputField id="metadataRequired" value="{!Account.MetadataRequired__c}"/>
  </apex:form>
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ThrowingRequired.page"), `<apex:page standardController="Account">
  <apex:form>
    <apex:inputField value="{!Account.RequiredEmpty__c}" required="{!$Action.Widget.save}"/>
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

	optional := storage.Field{
		Type:     storage.FieldString,
		Nillable: storage.BoolFlag(true),
	}
	required := storage.Field{
		Type:     storage.FieldString,
		Required: true,
		Nillable: storage.BoolFlag(false),
	}
	id := storage.ID("001000000000001")
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":                       {APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Nillable: storage.BoolFlag(false)},
				"RequiredEmpty__c":           {APIName: "RequiredEmpty__c", Label: "Required Empty", Type: optional.Type, Nillable: optional.Nillable},
				"RequiredPresent__c":         {APIName: "RequiredPresent__c", Label: "Required Present", Type: optional.Type, Nillable: optional.Nillable},
				"RequiredTrueExpression__c":  {APIName: "RequiredTrueExpression__c", Label: "Required True Expression", Type: optional.Type, Nillable: optional.Nillable},
				"RequiredFalseExpression__c": {APIName: "RequiredFalseExpression__c", Label: "Required False Expression", Type: optional.Type, Nillable: optional.Nillable},
				"RequiredTextarea__c":        {APIName: "RequiredTextarea__c", Label: "Required Textarea", Type: storage.FieldString, DisplayType: "TEXTAREA", Nillable: storage.BoolFlag(true)},
				"RequiredPicklist__c":        {APIName: "RequiredPicklist__c", Label: "Required Picklist", Type: storage.FieldPicklist, DisplayType: "PICKLIST", Nillable: storage.BoolFlag(true), PicklistValues: []storage.PicklistValue{{Value: "North", Label: "North", Active: true}}},
				"RequiredCheckbox__c":        {APIName: "RequiredCheckbox__c", Label: "Required Checkbox", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", Nillable: storage.BoolFlag(true)},
				"SchemaRequiredCheckbox__c":  {APIName: "SchemaRequiredCheckbox__c", Label: "Schema Required Checkbox", Type: storage.FieldBoolean, DisplayType: "BOOLEAN", Required: true, Nillable: storage.BoolFlag(false)},
				"Omitted__c":                 {APIName: "Omitted__c", Label: "Omitted", Type: optional.Type, Nillable: optional.Nillable},
				"ExplicitFalse__c":           {APIName: "ExplicitFalse__c", Label: "Explicit False", Type: optional.Type, Nillable: optional.Nillable},
				"MetadataRequired__c":        {APIName: "MetadataRequired__c", Label: "Metadata Required", Type: required.Type, Required: required.Required, Nillable: required.Nillable},
			},
		},
		Records: map[storage.ID]storage.Record{
			id: {
				ID:     id,
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":                       storage.StringValue("Acme"),
					"RequiredEmpty__c":           storage.StringValue(""),
					"RequiredPresent__c":         storage.StringValue("present"),
					"RequiredTrueExpression__c":  storage.StringValue(""),
					"RequiredFalseExpression__c": storage.StringValue(""),
					"RequiredTextarea__c":        storage.StringValue("notes"),
					"RequiredPicklist__c":        storage.StringValue("North"),
					"RequiredCheckbox__c":        storage.BooleanValue(false),
					"SchemaRequiredCheckbox__c":  storage.BooleanValue(false),
					"Omitted__c":                 storage.StringValue(""),
					"ExplicitFalse__c":           storage.StringValue(""),
					"MetadataRequired__c":        storage.StringValue("kept"),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, machine,
		"RequiredEmpty__c", "RequiredPresent__c", "RequiredTrueExpression__c", "RequiredFalseExpression__c",
		"RequiredTextarea__c", "RequiredPicklist__c", "RequiredCheckbox__c", "SchemaRequiredCheckbox__c",
		"Omitted__c", "ExplicitFalse__c", "MetadataRequired__c",
	)
	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "RequiredFields",
		PageURL:  "/apex/RequiredFields?id=001000000000001",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		field         string
		value         string
		wantValueAttr bool
		wantRequired  bool
	}{
		{field: "RequiredEmpty__c", value: "", wantValueAttr: true, wantRequired: true},
		{field: "RequiredPresent__c", value: "present", wantValueAttr: true, wantRequired: true},
		{field: "RequiredTrueExpression__c", value: "", wantValueAttr: true, wantRequired: true},
		{field: "RequiredFalseExpression__c", value: "", wantValueAttr: true, wantRequired: false},
		{field: "RequiredTextarea__c", value: "notes", wantRequired: true},
		{field: "RequiredPicklist__c", wantRequired: true},
		{field: "RequiredCheckbox__c", value: "true", wantValueAttr: true, wantRequired: true},
		{field: "SchemaRequiredCheckbox__c", value: "true", wantValueAttr: true, wantRequired: false},
		{field: "Omitted__c", value: "", wantValueAttr: true, wantRequired: false},
		{field: "ExplicitFalse__c", value: "", wantValueAttr: true, wantRequired: false},
		// Standard-object Name remains required even when the attribute is false.
		{field: "Name", value: "Acme", wantValueAttr: true, wantRequired: true},
		// Preserve the existing schema-metadata-required rendering behavior.
		{field: "MetadataRequired__c", value: "kept", wantValueAttr: true, wantRequired: true},
	} {
		tag := inputFieldRequiredTestTag(t, result.HTML, "Account."+tc.field)
		if tc.wantValueAttr && !strings.Contains(tag, `value="`+tc.value+`"`) {
			t.Errorf("input %s = %s, want value %q", tc.field, tag, tc.value)
		}
		gotRequired := strings.Contains(tag, `required="required"`)
		if gotRequired != tc.wantRequired {
			t.Errorf("input %s required = %v, want %v: %s", tc.field, gotRequired, tc.wantRequired, tag)
		}
	}
	if !strings.Contains(result.HTML, `>notes</textarea>`) {
		t.Errorf("required textarea lost its current value: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, `<option value="North" selected="selected">North</option>`) {
		t.Errorf("required picklist lost its selected option: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, `<input type="hidden" name="Account.RequiredCheckbox__c" value="false" />`) {
		t.Errorf("required checkbox changed its hidden false input: %s", result.HTML)
	}
	for _, field := range []string{"RequiredCheckbox__c", "SchemaRequiredCheckbox__c"} {
		if tag := inputFieldRequiredTestTag(t, result.HTML, "Account."+field); strings.Contains(tag, `checked="checked"`) {
			t.Errorf("false checkbox %s rendered checked: %s", field, tag)
		}
	}
	throwingMachine := vm.New(nil)
	throwingMachine.SetOrg(&org)
	authorizeFieldRenderingFixture(t, &org, throwingMachine, "RequiredEmpty__c")
	if _, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  throwingMachine,
		PageName: "ThrowingRequired",
		PageURL:  "/apex/ThrowingRequired?id=001000000000001",
	}); err == nil {
		t.Fatal("RenderPage succeeded with an unsupported required expression; expected the expression error to propagate")
	} else if !strings.Contains(err.Error(), "$Action") || !strings.Contains(err.Error(), "unsupported Visualforce global") {
		t.Fatalf("RenderPage error = %v, want the required-expression error to propagate", err)
	}
}

func inputFieldRequiredTestTag(t *testing.T, renderedHTML, inputName string) string {
	t.Helper()
	marker := `name="` + inputName + `"`
	for offset := 0; offset < len(renderedHTML); {
		relativeNameAt := strings.Index(renderedHTML[offset:], marker)
		if relativeNameAt < 0 {
			break
		}
		nameAt := offset + relativeNameAt
		start := strings.LastIndex(renderedHTML[:nameAt], "<")
		endOffset := strings.Index(renderedHTML[nameAt:], ">")
		if start >= 0 && endOffset >= 0 {
			tag := renderedHTML[start : nameAt+endOffset+1]
			if strings.Contains(tag, `class="inputField"`) {
				return tag
			}
		}
		offset = nameAt + len(marker)
	}
	t.Fatalf("rendered HTML has no visible field control named %q: %s", inputName, renderedHTML)
	return ""
}

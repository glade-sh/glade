package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// This is a point-source family at the fixture's declared API 67.0. It does
// not claim an API-version interval or Salesforce runtime parity.
func TestVisualforceAssignmentContractFamily(t *testing.T) {
	t.Run("component-attribute-assignTo-setter-and-type-match", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "AttributeHost", `<apex:page><c:AttributeCard caption="component-value"/></apex:page>`, map[string]string{
			"AttributeCard": `<apex:component controller="AttributeComponentController">
  <apex:attribute name="caption" type="String" assignTo="{!caption}" required="true"/>
  <apex:outputText value="{!caption}"/>
</apex:component>`,
		})
		machine := testRunner(t)
		fields := assignmentAccessorFields(t, "AttributeComponentController", "caption", "String", "_caption")
		if err := machine.RegisterClass(vm.Class{Name: "AttributeComponentController", Fields: fields}); err != nil {
			t.Fatal(err)
		}

		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "AttributeHost"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Error != nil {
			t.Fatalf("render returned error: %v", result.Error)
		}
		if !strings.Contains(result.HTML, "component-value") {
			t.Fatalf("component attribute value did not reach its controller property: %s", result.HTML)
		}
	})

	t.Run("component-attribute-assignTo-type-mismatch-is-rejected", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "AttributeTypeHost", `<apex:page><c:AttributeCard caption="7"/></apex:page>`, map[string]string{
			"AttributeCard": `<apex:component controller="AttributeTypeController">
  <apex:attribute name="caption" type="Integer" assignTo="{!caption}"/>
  <apex:outputText value="{!caption}"/>
</apex:component>`,
		})
		machine := testRunner(t)
		fields := assignmentAccessorFields(t, "AttributeTypeController", "caption", "String", "_caption")
		if err := machine.RegisterClass(vm.Class{Name: "AttributeTypeController", Fields: fields}); err != nil {
			t.Fatal(err)
		}

		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "AttributeTypeHost"})
		requireAssignmentRenderFailure(t, result, err)
	})

	t.Run("required-value-and-default-false", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "OptionalAttributeHost", `<apex:page><c:OptionalCard/></apex:page>`, map[string]string{
			"OptionalCard": `<apex:component><apex:attribute name="caption" type="String"/><apex:outputText value="optional"/></apex:component>`,
		})
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: testRunner(t), PageName: "OptionalAttributeHost"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Error != nil {
			t.Fatalf("omitted attribute defaulted to required: %v", result.Error)
		}

		requiredProject, requiredIndex := assignmentContractPage(t, "RequiredAttributeHost", `<apex:page><c:RequiredCard/></apex:page>`, map[string]string{
			"RequiredCard": `<apex:component><apex:attribute name="caption" type="String" required="true"/><apex:outputText value="required"/></apex:component>`,
		})
		required, requiredErr := RenderPage(PageRenderRequest{Project: requiredProject, VFIndex: requiredIndex, Machine: testRunner(t), PageName: "RequiredAttributeHost"})
		requireAssignmentRenderFailure(t, required, requiredErr)

		emptyProject, emptyIndex := assignmentContractPage(t, "EmptyRequiredAttributeHost", `<apex:page><c:RequiredCard caption=""/></apex:page>`, map[string]string{
			"RequiredCard": `<apex:component><apex:attribute name="caption" type="String" required="true"/><apex:outputText value="provided"/></apex:component>`,
		})
		empty, emptyErr := RenderPage(PageRenderRequest{Project: emptyProject, VFIndex: emptyIndex, Machine: testRunner(t), PageName: "EmptyRequiredAttributeHost"})
		if emptyErr != nil || empty.Error != nil {
			t.Fatalf("supplied empty attribute was treated as omitted: err=%v result=%#v", emptyErr, empty.Error)
		}
	})

	t.Run("automatic-get-set-property-uses-declared-field-metadata", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "AutoPropertyHost", `<apex:page><c:BoundCard caption="auto-value"/></apex:page>`, map[string]string{
			"BoundCard": `<apex:component controller="AutoPropertyController"><apex:attribute name="caption" type="String" assignTo="{!caption}"/><apex:outputText value="{!caption}"/></apex:component>`,
		})
		machine := testRunner(t)
		if err := machine.RegisterClass(vm.Class{Name: "AutoPropertyController", Fields: map[string]vm.Field{
			"caption": {Name: "caption", Type: "String", Property: true, HasGetter: true, HasSetter: true, InitialValue: vm.String("")},
		}}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "AutoPropertyHost"})
		if err != nil || result.Error != nil || !strings.Contains(result.HTML, "auto-value") {
			t.Fatalf("automatic get/set property assignment failed: err=%v result=%#v html=%s", err, result.Error, result.HTML)
		}
	})

	t.Run("automatic-getter-custom-setter-property-is-preserved", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "MixedPropertyHost", `<apex:page><c:BoundCard caption="mixed-value"/></apex:page>`, map[string]string{
			"BoundCard": `<apex:component controller="MixedPropertyController"><apex:attribute name="caption" type="String" assignTo="{!caption}"/><apex:outputText value="{!caption}"/></apex:component>`,
		})
		// attachPropertyAccessors represents get; with no Getter program, while
		// the custom setter has a body. Its recursive write uses VM assignment.
		setterProgram, err := vm.CompileAnonymous(`this.caption = value;`)
		if err != nil {
			t.Fatal(err)
		}
		setter := vm.Method{Name: "MixedPropertyController.caption.set", ClassName: "MixedPropertyController", Params: []vm.Param{{Name: "value", Type: "String"}}, Program: setterProgram}
		machine := testRunner(t)
		if err := machine.RegisterClass(vm.Class{Name: "MixedPropertyController", Fields: map[string]vm.Field{
			"caption": {Name: "caption", Type: "String", Property: true, HasGetter: true, HasSetter: true, Setter: &setter, InitialValue: vm.String("")},
		}}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "MixedPropertyHost"})
		if err != nil || result.Error != nil || !strings.Contains(result.HTML, "mixed-value") {
			t.Fatalf("mixed automatic getter/custom setter assignment failed: err=%v result=%#v html=%s", err, result.Error, result.HTML)
		}
	})

	t.Run("getter-only-unknown-and-dotted-targets-reject", func(t *testing.T) {
		getterProgram, err := vm.CompileAnonymous(`return this._caption;`)
		if err != nil {
			t.Fatal(err)
		}
		getter := vm.Method{Name: "AssignmentController.getCaption", ClassName: "AssignmentController", ReturnType: "String", Program: getterProgram}
		writable := assignmentAccessorFields(t, "AssignmentController", "caption", "String", "_caption")
		for _, tc := range []struct {
			name   string
			target string
			fields map[string]vm.Field
		}{
			{name: "getter-only", target: "{!caption}", fields: map[string]vm.Field{
				"_caption": {Name: "_caption", Type: "String", InitialValue: vm.String("")},
				"caption":  {Name: "caption", Type: "String", Property: true, Getter: &getter},
			}},
			{name: "unknown-property", target: "{!missing}", fields: map[string]vm.Field{}},
			{name: "dotted-target-is-not-truncated", target: "{!controller.caption}", fields: writable},
		} {
			t.Run(tc.name, func(t *testing.T) {
				pageName := "InvalidAssignToHost"
				p, idx := assignmentContractPage(t, pageName, `<apex:page><c:BoundCard caption="value"/></apex:page>`, map[string]string{
					"BoundCard": `<apex:component controller="AssignmentController"><apex:attribute name="caption" type="String" assignTo="` + tc.target + `"/><apex:outputText value="ok"/></apex:component>`,
				})
				machine := testRunner(t)
				if err := machine.RegisterClass(vm.Class{Name: "AssignmentController", Fields: tc.fields}); err != nil {
					t.Fatal(err)
				}
				result, renderErr := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: pageName})
				requireAssignmentRenderFailure(t, result, renderErr)
			})
		}
	})

	t.Run("throwing-property-setter-propagates", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "ThrowingSetterHost", `<apex:page><c:BoundCard caption="value"/></apex:page>`, map[string]string{
			"BoundCard": `<apex:component controller="ThrowingSetterController"><apex:attribute name="caption" type="String" assignTo="{!caption}"/><apex:outputText value="ok"/></apex:component>`,
		})
		fields := assignmentAccessorFields(t, "ThrowingSetterController", "caption", "String", "_caption")
		throwProgram, err := vm.CompileAnonymous(`throw new VisualforceException('setter rejected');`)
		if err != nil {
			t.Fatal(err)
		}
		property := fields["caption"]
		setter := *property.Setter
		setter.Program = throwProgram
		property.Setter = &setter
		fields["caption"] = property
		machine := testRunner(t)
		if err := machine.RegisterClass(vm.Class{Name: "ThrowingSetterController", Fields: fields}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "ThrowingSetterHost"})
		requireAssignmentRenderFailure(t, result, err)
	})

	t.Run("param-assignTo-uses-submitted-name-and-controller-setter", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "ParamHost", `<apex:page controller="ParamController">
  <apex:form><apex:commandLink action="{!capture}" value="Capture"><apex:param name="incoming" value="default" assignTo="{!selected}"/><apex:param name="incomingCount" value="0" assignTo="{!selectedCount}"/><apex:param name="incomingEnabled" value="false" assignTo="{!selectedEnabled}"/></apex:commandLink></apex:form>
  <apex:outputText value="{!selected}"/>
  <apex:outputText value="{!selectedCount}"/>
  <apex:outputText value="{!selectedEnabled}"/>
</apex:page>`, nil)
		machine := testRunner(t)
		fields := make(map[string]vm.Field)
		for name, field := range assignmentAccessorFields(t, "ParamController", "selected", "String", "_selected") {
			fields[name] = field
		}
		for name, field := range assignmentAccessorFields(t, "ParamController", "selectedCount", "Integer", "_selectedCount") {
			fields[name] = field
		}
		for name, field := range assignmentAccessorFields(t, "ParamController", "selectedEnabled", "Boolean", "_selectedEnabled") {
			fields[name] = field
		}
		capture, err := vm.CompileAnonymous(`return null;`)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterClass(vm.Class{
			Name:   "ParamController",
			Fields: fields,
			Methods: map[string]vm.Method{
				"capture": {Name: "ParamController.capture", ClassName: "ParamController", ReturnType: "PageReference", Program: capture},
			},
		}); err != nil {
			t.Fatal(err)
		}

		result, err := RenderPage(PageRenderRequest{
			Project: p, VFIndex: idx, Machine: machine, PageName: "ParamHost", Action: "{!capture}",
			FormValues: map[string]string{"incoming": "param-value", "incomingCount": "7", "incomingEnabled": "true"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Error != nil {
			t.Fatalf("render returned error: %v", result.Error)
		}
		if !strings.Contains(result.HTML, "param-value") || !strings.Contains(result.HTML, "7") || !strings.Contains(result.HTML, "true") {
			t.Fatalf("submitted apex:param value did not reach assignTo property: %s", result.HTML)
		}
	})

	t.Run("ordinary-form-field-binding-is-preserved", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "OrdinaryFormHost", `<apex:page controller="OrdinaryFormController"><apex:form><apex:inputText value="{!caption}"/></apex:form><apex:outputText value="{!caption}"/></apex:page>`, nil)
		machine := testRunner(t)
		if err := machine.RegisterClass(vm.Class{Name: "OrdinaryFormController", Fields: map[string]vm.Field{
			"caption": {Name: "caption", Type: "String", InitialValue: vm.String("before")},
		}}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "OrdinaryFormHost", FormValues: map[string]string{"caption": "ordinary-form-value"}})
		if err != nil || result.Error != nil {
			t.Fatalf("ordinary form binding failed: err=%v result=%#v", err, result.Error)
		}
		if !strings.Contains(result.HTML, "ordinary-form-value") {
			t.Fatalf("ordinary form field was not bound: %s", result.HTML)
		}
	})

	t.Run("unsubmitted-param-does-not-suppress-ordinary-target-input", func(t *testing.T) {
		p, idx := assignmentContractPage(t, "OmittedParamHost", `<apex:page controller="OmittedParamController"><apex:form><apex:inputText value="{!caption}"/><apex:commandLink action="{!capture}"><apex:param name="incoming" value="default" assignTo="{!caption}"/></apex:commandLink></apex:form><apex:outputText value="{!caption}"/></apex:page>`, nil)
		capture, err := vm.CompileAnonymous(`return null;`)
		if err != nil {
			t.Fatal(err)
		}
		machine := testRunner(t)
		if err := machine.RegisterClass(vm.Class{Name: "OmittedParamController", Fields: map[string]vm.Field{
			"caption": {Name: "caption", Type: "String", InitialValue: vm.String("before")},
		}, Methods: map[string]vm.Method{
			"capture": {Name: "OmittedParamController.capture", ClassName: "OmittedParamController", ReturnType: "PageReference", Program: capture},
		}}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "OmittedParamHost", Action: "{!capture}", FormValues: map[string]string{"caption": "ordinary-value"}})
		if err != nil || result.Error != nil || !strings.Contains(result.HTML, "ordinary-value") {
			t.Fatalf("unsubmitted param suppressed ordinary input: err=%v result=%#v html=%s", err, result.Error, result.HTML)
		}
	})
}

func assignmentAccessorFields(t *testing.T, className, propertyName, typeName, backingName string) map[string]vm.Field {
	t.Helper()
	getterProgram, err := vm.CompileAnonymous(`return this.` + backingName + `;`)
	if err != nil {
		t.Fatal(err)
	}
	setterProgram, err := vm.CompileAnonymous(`this.` + backingName + ` = value;`)
	if err != nil {
		t.Fatal(err)
	}
	getter := vm.Method{Name: className + ".get" + propertyName, ClassName: className, ReturnType: typeName, Program: getterProgram}
	setter := vm.Method{Name: className + ".set" + propertyName, ClassName: className, Params: []vm.Param{{Name: "value", Type: typeName}}, Program: setterProgram}
	return map[string]vm.Field{
		backingName:  {Name: backingName, Type: typeName, InitialValue: assignmentInitialValue(typeName)},
		propertyName: {Name: propertyName, Type: typeName, Property: true, Getter: &getter, Setter: &setter},
	}
}

func assignmentInitialValue(typeName string) vm.Value {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "boolean":
		return vm.Bool(false)
	case "integer", "long":
		return vm.Int(0)
	case "decimal", "double":
		return vm.Decimal(0)
	default:
		return vm.String("")
	}
}

func assignmentContractPage(t *testing.T, pageName, pageMarkup string, components map[string]string) (project.Project, Index) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages", pageName+".page"), pageMarkup)
	for name, markup := range components {
		writeFile(t, filepath.Join(root, "force-app/main/default/components", name+".component"), markup)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, idx
}

func requireAssignmentRenderFailure(t *testing.T, result PageRenderResult, err error) {
	t.Helper()
	if err == nil && result.Error == nil {
		t.Fatalf("render succeeded but the source-defined assignment contract requires rejection: %s", result.HTML)
	}
}

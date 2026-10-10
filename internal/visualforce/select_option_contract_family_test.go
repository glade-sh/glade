package visualforce

import (
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceSelectOptionConstructorAndAccessorState(t *testing.T) {
	program, err := vm.CompileAnonymous(`
SelectOption pair = new SelectOption('initial-value', 'Initial label');
System.assertEquals('initial-value', pair.getValue());
System.assertEquals('Initial label', pair.getLabel());

SelectOption option = new SelectOption('constructor-value', 'Constructor label', true);
System.assertEquals(true, option.getDisabled());
option.setValue('mutated-value');
option.setLabel('Mutated label');
option.setDisabled(false);
option.setEscapeItem(false);
System.assertEquals('mutated-value', option.getValue());
System.assertEquals('Mutated label', option.getLabel());
System.assertEquals(false, option.getDisabled());
System.assertEquals(false, option.getEscapeItem());
option.setDisabled(true);
option.setEscapeItem(true);
System.assertEquals(true, option.getDisabled());
System.assertEquals(true, option.getEscapeItem());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestVisualforceSelectOptionControllerCollectionRenderAndSubmit(t *testing.T) {
	const controllerName = "SelectOptionFamilyController"
	optionsProgram, err := vm.CompileAnonymous(`
SelectOption escapedChoice = new SelectOption('constructor-value', 'Constructor label', true);
escapedChoice.setValue('mutated<&-value');
escapedChoice.setLabel('Mutated <label> & text');
escapedChoice.setDisabled(true);
escapedChoice.setEscapeItem(true);
SelectOption submittedChoice = new SelectOption('submitted<&-value', '<em>Unescaped label</em>', false);
submittedChoice.setEscapeItem(false);
return new List<SelectOption>{escapedChoice, submittedChoice};
`)
	if err != nil {
		t.Fatal(err)
	}
	actionProgram, err := vm.CompileAnonymous(`this.received = 'action-saw:' + this.selectedList + ':' + this.selectedRadio + ':' + this.selectedCheckboxes; return null;`)
	if err != nil {
		t.Fatal(err)
	}
	optionsGetter := vm.Method{
		Name:       controllerName + ".getOptions",
		ClassName:  controllerName,
		ReturnType: "List<SelectOption>",
		Program:    optionsProgram,
	}
	submit := vm.Method{
		Name:       controllerName + ".submit",
		ClassName:  controllerName,
		ReturnType: "PageReference",
		Program:    actionProgram,
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: controllerName,
		Fields: map[string]vm.Field{
			"selectedList":       {Name: "selectedList", Type: "String", InitialValue: vm.String("")},
			"selectedRadio":      {Name: "selectedRadio", Type: "String", InitialValue: vm.String("")},
			"selectedCheckboxes": {Name: "selectedCheckboxes", Type: "String", InitialValue: vm.String("")},
			"received":           {Name: "received", Type: "String"},
			"options":            {Name: "options", Type: "List<SelectOption>", Property: true, Getter: &optionsGetter},
		},
		Methods: map[string]vm.Method{"submit": submit},
	}); err != nil {
		t.Fatal(err)
	}
	markup := `<apex:page controller="` + controllerName + `"><apex:form><apex:selectList id="selectedList" value="{!selectedList}" size="1"><apex:selectOptions value="{!options}"/></apex:selectList><apex:selectRadio id="selectedRadio" value="{!selectedRadio}"><apex:selectOptions value="{!options}"/></apex:selectRadio><apex:selectCheckboxes id="selectedCheckboxes" value="{!selectedCheckboxes}"><apex:selectOptions value="{!options}"/></apex:selectCheckboxes><apex:commandButton action="{!submit}" value="Submit"/></apex:form><apex:outputText value="{!received}"/></apex:page>`
	project, index := expressionContractPage(t, "SelectOptionFamily", markup)
	result, err := RenderPage(PageRenderRequest{
		Project: project, VFIndex: index, Machine: machine, PageName: "SelectOptionFamily",
		PageURL: "/apex/SelectOptionFamily", FormValues: map[string]string{
			"selectedList": "submitted<&-value", "selectedRadio": "submitted<&-value", "selectedCheckboxes": "submitted<&-value",
		},
		Action: "{!submit}",
	})
	if err != nil {
		t.Fatal(err)
	}
	options := regexp.MustCompile(`(?s)<option\b[^>]*>.*?</option>`).FindAllString(result.HTML, -1)
	if len(options) != 2 {
		t.Fatalf("selectList rendered %d collection options, want the two supplied choices: %s", len(options), result.HTML)
	}
	radioInputs := regexp.MustCompile(`<input\b[^>]*type="radio"[^>]*>`).FindAllString(result.HTML, -1)
	checkboxInputs := regexp.MustCompile(`<input\b[^>]*type="checkbox"[^>]*>`).FindAllString(result.HTML, -1)
	if len(radioInputs) != 2 || len(checkboxInputs) != 2 {
		t.Fatalf("rendered %d radio and %d checkbox choices, want two each: %s", len(radioInputs), len(checkboxInputs), result.HTML)
	}
	disabledAttr := regexp.MustCompile(`\sdisabled(?:\s|=|>)`)
	disabledOption := selectOptionElementWithValue(options, `mutated&lt;&amp;-value`)
	if disabledOption == "" || !disabledAttr.MatchString(strings.SplitN(disabledOption, ">", 2)[0]+">") || !strings.Contains(disabledOption, `Mutated &lt;label&gt; &amp; text`) {
		t.Fatalf("mutated disabled list option was not visible with escaped label content: %s", disabledOption)
	}
	submittedOption := selectOptionElementWithValue(options, `submitted&lt;&amp;-value`)
	if submittedOption == "" || !strings.Contains(submittedOption, `selected="selected"`) || disabledAttr.MatchString(strings.SplitN(submittedOption, ">", 2)[0]+">") {
		t.Fatalf("submitted enabled list option state is wrong: %s", submittedOption)
	}
	if !strings.Contains(submittedOption, `<em>Unescaped label</em>`) || strings.Contains(submittedOption, `&lt;em&gt;Unescaped label&lt;/em&gt;`) {
		t.Fatalf("escapeItem=false did not render list item content as written: %s", submittedOption)
	}
	disabledRadio := selectOptionElementWithValue(radioInputs, `mutated&lt;&amp;-value`)
	submittedRadio := selectOptionElementWithValue(radioInputs, `submitted&lt;&amp;-value`)
	disabledCheckbox := selectOptionElementWithValue(checkboxInputs, `mutated&lt;&amp;-value`)
	submittedCheckbox := selectOptionElementWithValue(checkboxInputs, `submitted&lt;&amp;-value`)
	for _, control := range []struct {
		name, disabled, submitted string
	}{
		{name: "radio", disabled: disabledRadio, submitted: submittedRadio},
		{name: "checkbox", disabled: disabledCheckbox, submitted: submittedCheckbox},
	} {
		if control.disabled == "" || !disabledAttr.MatchString(control.disabled) {
			t.Errorf("%s disabled option state missing: %s", control.name, control.disabled)
		}
		if control.submitted == "" || !strings.Contains(control.submitted, `checked="checked"`) || disabledAttr.MatchString(control.submitted) {
			t.Errorf("%s submitted option state is wrong: %s", control.name, control.submitted)
		}
	}
	if strings.Count(result.HTML, `value="mutated&lt;&amp;-value"`) != 3 || strings.Count(result.HTML, `value="submitted&lt;&amp;-value"`) != 3 {
		t.Fatalf("option values were not attribute-escaped in all three selection components: %s", result.HTML)
	}
	if strings.Count(result.HTML, `Mutated &lt;label&gt; &amp; text`) != 3 || strings.Count(result.HTML, `<em>Unescaped label</em>`) != 3 {
		t.Fatalf("escapeItem state was not applied in all three selection components: %s", result.HTML)
	}
	if !strings.Contains(result.HTML, "action-saw:submitted&lt;&amp;-value:submitted&lt;&amp;-value:submitted&lt;&amp;-value") {
		t.Fatalf("submitted option value did not reach the controller action for every selection component: %s", result.HTML)
	}
}

func selectOptionElementWithValue(elements []string, value string) string {
	for _, element := range elements {
		if strings.Contains(element, `value="`+value+`"`) {
			return element
		}
	}
	return ""
}

func TestVisualforceSelectOptionParentControlsPreserved(t *testing.T) {
	parents := []string{"selectCheckboxes", "selectRadio", "selectList"}
	for _, child := range []string{"selectOption", "selectOptions"} {
		for _, parent := range parents {
			t.Run(child+"-under-"+parent, func(t *testing.T) {
				childMarkup := `<apex:selectOption itemValue="one" itemLabel="One"/>`
				if child == "selectOptions" {
					childMarkup = `<apex:selectOptions value="{!choices}"/>`
				}
				markup := `<apex:page><apex:` + parent + ` value="{!selected}">` + childMarkup + `</apex:` + parent + `></apex:page>`
				expressionContractPage(t, "SelectOptionParent", markup)
			})
		}
		t.Run(child+"-outside-select-parent", func(t *testing.T) {
			childMarkup := `<apex:selectOption itemValue="one" itemLabel="One"/>`
			if child == "selectOptions" {
				childMarkup = `<apex:selectOptions value="{!choices}"/>`
			}
			markup := `<apex:page><apex:outputPanel>` + childMarkup + `</apex:outputPanel></apex:page>`
			if _, _, err := loadStructureContractFixture(t, markup, ""); err == nil {
				t.Fatal("LoadProject accepted select child outside its documented select parents")
			}
		})
	}
}
